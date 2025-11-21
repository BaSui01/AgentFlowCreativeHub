package tenant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrRoleNameExists = errors.New("tenant: role name already exists in tenant")
)

// RoleService encapsulates role and permission management within a tenant.
type RoleService interface {
	CreateRole(ctx context.Context, params CreateRoleParams) (*Role, error)
	UpdateRole(ctx context.Context, id string, params UpdateRoleParams) (*Role, error)
	DeleteRole(ctx context.Context, id string) error
	AssignRoleToUser(ctx context.Context, userID, roleID string) error
	UpdateRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error
	GetUserPermissions(ctx context.Context, userID string) ([]*Permission, error)
	ListPermissions(ctx context.Context) ([]*Permission, error)
}

type CreateRoleParams struct {
	Name          string
	Description   string
	PermissionIDs []string
}

type UpdateRoleParams struct {
	Name          string
	Description   string
	PermissionIDs []string
}

type roleService struct {
	roles     RoleRepository
	userRoles UserRoleRepository
	rolePerms RolePermissionRepository
	perms     PermissionRepository // Added dependency
	ids       IDGenerator
	audit     AuditLogger
}

// NewRoleService constructs a RoleService from repositories and utilities.
func NewRoleService(
	roles RoleRepository,
	userRoles UserRoleRepository,
	rolePerms RolePermissionRepository,
	perms PermissionRepository, // Added dependency
	ids IDGenerator,
	audit AuditLogger,
) RoleService {
	return &roleService{
		roles:     roles,
		userRoles: userRoles,
		rolePerms: rolePerms,
		perms:     perms,
		ids:       ids,
		audit:     audit,
	}
}

func (s *roleService) CreateRole(ctx context.Context, params CreateRoleParams) (*Role, error) {
	tc, ok := FromContext(ctx)
	if !ok {
		return nil, ErrForbidden
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, errors.New("tenant: role name is required")
	}

	if existing, err := s.roles.GetByName(ctx, params.Name); err == nil && existing != nil {
		return nil, ErrRoleNameExists
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	id, err := s.ids.NewID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	role := &Role{
		ID:          id,
		Name:        params.Name,
		Description: params.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.roles.Insert(ctx, role); err != nil {
		return nil, err
	}

	// Assign permissions if provided
	if len(params.PermissionIDs) > 0 {
		if err := s.rolePerms.ReplaceRolePermissions(ctx, role.ID, params.PermissionIDs); err != nil {
			return nil, fmt.Errorf("failed to assign permissions: %w", err)
		}
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "role.create", "role", map[string]any{
			"roleId":      role.ID,
			"roleName":    role.Name,
			"permissions": params.PermissionIDs,
		})
	}

	return role, nil
}

func (s *roleService) UpdateRole(ctx context.Context, id string, params UpdateRoleParams) (*Role, error) {
	tc, ok := FromContext(ctx)
	if !ok {
		return nil, ErrForbidden
	}

	role, err := s.roles.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Name != "" && params.Name != role.Name {
		if existing, err := s.roles.GetByName(ctx, params.Name); err == nil && existing != nil && existing.ID != role.ID {
			return nil, ErrRoleNameExists
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		role.Name = params.Name
	}
	if params.Description != "" {
		role.Description = params.Description
	}
	role.UpdatedAt = time.Now().UTC()

	if err := s.roles.Update(ctx, role); err != nil {
		return nil, err
	}

	if params.PermissionIDs != nil {
		if err := s.rolePerms.ReplaceRolePermissions(ctx, role.ID, params.PermissionIDs); err != nil {
			return nil, fmt.Errorf("failed to update permissions: %w", err)
		}
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "role.update", "role", map[string]any{
			"roleId":      role.ID,
			"roleName":    role.Name,
			"permissions": params.PermissionIDs,
		})
	}

	return role, nil
}

func (s *roleService) DeleteRole(ctx context.Context, id string) error {
	tc, ok := FromContext(ctx)
	if !ok {
		return ErrForbidden
	}

	// 先解除用户与角色绑定
	if err := s.userRoles.RemoveRolesByRole(ctx, id); err != nil {
		return err
	}
	// 再清空角色权限
	if err := s.rolePerms.ReplaceRolePermissions(ctx, id, nil); err != nil {
		return err
	}
	// 最后删除角色记录
	if err := s.roles.Delete(ctx, id); err != nil {
		return err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "role.delete", "role", map[string]any{
			"roleId": id,
		})
	}
	return nil
}

func (s *roleService) AssignRoleToUser(ctx context.Context, userID, roleID string) error {
	tc, ok := FromContext(ctx)
	if !ok {
		return ErrForbidden
	}

	if err := s.userRoles.AssignRoleToUser(ctx, userID, roleID); err != nil {
		return err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "role.assign", "user_role", map[string]any{
			"userId": userID,
			"roleId": roleID,
		})
	}
	return nil
}

func (s *roleService) UpdateRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	tc, ok := FromContext(ctx)
	if !ok {
		return ErrForbidden
	}

	if err := s.rolePerms.ReplaceRolePermissions(ctx, roleID, permissionIDs); err != nil {
		return err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "role.permissions.update", "role_permission", map[string]any{
			"roleId":        roleID,
			"permissionIds": permissionIDs,
		})
	}
	return nil
}

func (s *roleService) GetUserPermissions(ctx context.Context, userID string) ([]*Permission, error) {
	// 验证 TenantContext 是否存在
	if _, ok := FromContext(ctx); !ok {
		return nil, ErrForbidden
	}
	return s.perms.GetPermissionsForUser(ctx, userID)
}

func (s *roleService) ListPermissions(ctx context.Context) ([]*Permission, error) {
	if _, ok := FromContext(ctx); !ok {
		return nil, ErrForbidden
	}
	return s.perms.ListByTenant(ctx)
}
