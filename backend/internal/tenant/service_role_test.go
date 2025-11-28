package tenant

import (
	"context"
	"errors"
	"testing"
)

type fakeRoleRepository struct {
	items map[string]*Role
}

func newFakeRoleRepository() *fakeRoleRepository {
	return &fakeRoleRepository{items: make(map[string]*Role)}
}

func (r *fakeRoleRepository) Insert(_ context.Context, role *Role) error {
	if _, exists := r.items[role.ID]; exists {
		return errors.New("duplicate role")
	}
	r.items[role.ID] = role
	return nil
}

func (r *fakeRoleRepository) GetByID(_ context.Context, id string) (*Role, error) {
	if role, ok := r.items[id]; ok {
		return role, nil
	}
	return nil, ErrNotFound
}

func (r *fakeRoleRepository) GetByName(_ context.Context, name string) (*Role, error) {
	for _, role := range r.items {
		if role.Name == name {
			return role, nil
		}
	}
	return nil, ErrNotFound
}

func (r *fakeRoleRepository) Update(_ context.Context, role *Role) error {
	r.items[role.ID] = role
	return nil
}

func (r *fakeRoleRepository) Delete(_ context.Context, id string) error {
	delete(r.items, id)
	return nil
}

func (r *fakeRoleRepository) List(_ context.Context) ([]*Role, error) {
	var roles []*Role
	for _, role := range r.items {
		roles = append(roles, role)
	}
	return roles, nil
}

type fakeUserRoleRepository struct {
	roles map[string][]string
}

func newFakeUserRoleRepository() *fakeUserRoleRepository {
	return &fakeUserRoleRepository{roles: make(map[string][]string)}
}

func (r *fakeUserRoleRepository) AssignRoleToUser(context.Context, string, string) error {
	return nil
}

func (r *fakeUserRoleRepository) RemoveRolesByRole(context.Context, string) error {
	return nil
}

func (r *fakeUserRoleRepository) ReplaceUserRoles(context.Context, string, []string) error {
	return nil
}

func (r *fakeUserRoleRepository) ListRoleIDsByUser(_ context.Context, userID string) ([]string, error) {
	if len(r.roles[userID]) == 0 {
		return nil, nil
	}
	return append([]string(nil), r.roles[userID]...), nil
}

type fakeRolePermissionRepository struct {
	lastRoleID string
	lastPerms  []string
}

func (r *fakeRolePermissionRepository) ReplaceRolePermissions(_ context.Context, roleID string, permissionIDs []string) error {
	r.lastRoleID = roleID
	r.lastPerms = append([]string(nil), permissionIDs...)
	return nil
}

func (r *fakeRolePermissionRepository) ListPermissionIDsByRoles(_ context.Context, roleIDs []string) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, id := range roleIDs {
		if id == r.lastRoleID {
			result[id] = append([]string(nil), r.lastPerms...)
		} else {
			result[id] = nil
		}
	}
	return result, nil
}

type fakePermissionRepository struct{}

func (fakePermissionRepository) ListByTenant(context.Context) ([]*Permission, error) {
	return nil, nil
}

func (fakePermissionRepository) GetPermissionsForUser(context.Context, string) ([]*Permission, error) {
	return nil, nil
}

func newRoleServiceForTest(ids []string) (RoleService, *fakeRoleRepository, *fakeRolePermissionRepository, *fakeUserRoleRepository) {
	roleRepo := newFakeRoleRepository()
	rolePermRepo := &fakeRolePermissionRepository{}
	userRoleRepo := newFakeUserRoleRepository()
	service := NewRoleService(
		roleRepo,
		userRoleRepo,
		rolePermRepo,
		fakePermissionRepository{},
		&sequenceIDGenerator{values: ids},
		&fakeAuditLogger{},
	)
	return service, roleRepo, rolePermRepo, userRoleRepo
}

func TestRoleServiceCreateRoleRequiresContext(t *testing.T) {
	service, _, _, _ := newRoleServiceForTest([]string{"role-1"})
	_, err := service.CreateRole(context.Background(), CreateRoleParams{Name: "editor"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("缺少上下文时应返回 ErrForbidden, got %v", err)
	}
}

func TestRoleServiceCreateRoleRejectsDuplicateName(t *testing.T) {
	service, repo, _, _ := newRoleServiceForTest([]string{"role-1"})
	repo.items["role-existing"] = &Role{ID: "role-existing", Name: "editor"}
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "tenant-1", UserID: "admin", IsSystemAdmin: true})
	_, err := service.CreateRole(ctx, CreateRoleParams{Name: "editor"})
	if !errors.Is(err, ErrRoleNameExists) {
		t.Fatalf("重复名称应返回 ErrRoleNameExists, got %v", err)
	}
}

func TestRoleServiceCreateRoleAssignsPermissions(t *testing.T) {
	service, repo, rolePerm, _ := newRoleServiceForTest([]string{"role-new"})
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "tenant-1", UserID: "admin", IsSystemAdmin: true})
	role, err := service.CreateRole(ctx, CreateRoleParams{Name: "reviewer", Description: "审核", PermissionIDs: []string{"perm-1", "perm-2"}})
	if err != nil {
		t.Fatalf("CreateRole 失败: %v", err)
	}
	if role.ID != "role-new" {
		t.Fatalf("role id 不符: %s", role.ID)
	}
	if rolePerm.lastRoleID != role.ID || len(rolePerm.lastPerms) != 2 {
		t.Fatalf("权限未绑定: %+v", rolePerm.lastPerms)
	}
	if _, ok := repo.items[role.ID]; !ok {
		t.Fatalf("角色未写入仓储")
	}
}

func TestRoleServiceUpdateRoleChangesNameAndPermissions(t *testing.T) {
	service, repo, rolePerm, _ := newRoleServiceForTest([]string{"role-1"})
	repo.items["role-1"] = &Role{ID: "role-1", Name: "old", Description: ""}
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "tenant-1", UserID: "admin", IsSystemAdmin: true})
	updated, err := service.UpdateRole(ctx, "role-1", UpdateRoleParams{Name: "new", Description: "desc", PermissionIDs: []string{"perm-9"}})
	if err != nil {
		t.Fatalf("UpdateRole 失败: %v", err)
	}
	if updated.Name != "new" || updated.Description != "desc" {
		t.Fatalf("角色字段未更新: %#v", updated)
	}
	if rolePerm.lastRoleID != "role-1" || len(rolePerm.lastPerms) != 1 || rolePerm.lastPerms[0] != "perm-9" {
		t.Fatalf("权限更新未执行: %+v", rolePerm.lastPerms)
	}
}

func TestRoleServiceListUserRoles(t *testing.T) {
	service, _, _, userRepo := newRoleServiceForTest([]string{"role-1"})
	userRepo.roles["user-1"] = []string{"role-a", "role-b"}
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "tenant-1", UserID: "admin"})
	roles, err := service.ListUserRoles(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListUserRoles 返回错误: %v", err)
	}
	if len(roles) != 2 || roles[0] != "role-a" {
		t.Fatalf("返回角色不正确: %#v", roles)
	}
}
