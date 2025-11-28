package auth

import (
	"context"
	"testing"
	"time"
)

// mockPermissionStore 模拟权限存储
type mockPermissionStore struct {
	userPerms map[string][]*ResourcePermission
	rolePerms map[string][]*ResourcePermission
	userRoles map[string][]string
}

func newMockPermissionStore() *mockPermissionStore {
	return &mockPermissionStore{
		userPerms: make(map[string][]*ResourcePermission),
		rolePerms: make(map[string][]*ResourcePermission),
		userRoles: make(map[string][]string),
	}
}

func (m *mockPermissionStore) GetUserPermissions(ctx context.Context, userID string) ([]*ResourcePermission, error) {
	return m.userPerms[userID], nil
}

func (m *mockPermissionStore) GetRolePermissions(ctx context.Context, roleID string) ([]*ResourcePermission, error) {
	return m.rolePerms[roleID], nil
}

func (m *mockPermissionStore) SetPermission(ctx context.Context, perm *ResourcePermission) error {
	if perm.PrincipalType == "user" {
		m.userPerms[perm.PrincipalID] = append(m.userPerms[perm.PrincipalID], perm)
	} else {
		m.rolePerms[perm.PrincipalID] = append(m.rolePerms[perm.PrincipalID], perm)
	}
	return nil
}

func (m *mockPermissionStore) DeletePermission(ctx context.Context, id string) error {
	return nil
}

func (m *mockPermissionStore) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	return m.userRoles[userID], nil
}

// ========== 角色层级测试 ==========

func TestRoleHierarchy_DefaultRoles(t *testing.T) {
	rh := NewRoleHierarchy()

	// 测试默认角色是否存在
	expectedRoles := []string{"guest", "user", "member", "editor", "manager", "admin", "owner"}
	for _, roleID := range expectedRoles {
		role, ok := rh.GetRole(roleID)
		if !ok {
			t.Errorf("默认角色 %s 不存在", roleID)
			continue
		}
		if role.ID != roleID {
			t.Errorf("角色ID不匹配: got %s, want %s", role.ID, roleID)
		}
	}
}

func TestRoleHierarchy_InheritanceChain(t *testing.T) {
	rh := NewRoleHierarchy()

	// 测试继承链：admin -> manager -> editor -> member -> user -> guest
	chain := rh.GetInheritanceChain("admin")

	expectedChain := []string{"admin", "manager", "editor", "member", "user", "guest"}
	if len(chain) != len(expectedChain) {
		t.Errorf("继承链长度不匹配: got %d, want %d", len(chain), len(expectedChain))
	}

	for i, role := range chain {
		if i >= len(expectedChain) {
			break
		}
		if role.ID != expectedChain[i] {
			t.Errorf("继承链位置 %d: got %s, want %s", i, role.ID, expectedChain[i])
		}
	}
}

func TestRoleHierarchy_InheritedPermissions(t *testing.T) {
	rh := NewRoleHierarchy()

	// 测试 admin 继承的权限（应包含所有父角色的权限）
	perms := rh.GetInheritedPermissions("admin")

	expectedPerms := map[string]bool{
		"read":    true,
		"write":   true,
		"execute": true,
		"delete":  true,
		"manage":  true,
		"admin":   true,
	}

	for _, perm := range perms {
		if !expectedPerms[perm] {
			t.Errorf("意外的权限: %s", perm)
		}
		delete(expectedPerms, perm)
	}

	for perm := range expectedPerms {
		t.Errorf("缺少权限: %s", perm)
	}
}

func TestRoleHierarchy_IsRoleHigherOrEqual(t *testing.T) {
	rh := NewRoleHierarchy()

	tests := []struct {
		roleA    string
		roleB    string
		expected bool
	}{
		{"admin", "user", true},
		{"admin", "admin", true},
		{"user", "admin", false},
		{"manager", "editor", true},
		{"guest", "owner", false},
		{"owner", "guest", true},
	}

	for _, tt := range tests {
		result := rh.IsRoleHigherOrEqual(tt.roleA, tt.roleB)
		if result != tt.expected {
			t.Errorf("IsRoleHigherOrEqual(%s, %s): got %v, want %v",
				tt.roleA, tt.roleB, result, tt.expected)
		}
	}
}

func TestRoleHierarchy_IsAncestor(t *testing.T) {
	rh := NewRoleHierarchy()

	tests := []struct {
		roleID     string
		ancestorID string
		expected   bool
	}{
		{"admin", "guest", true},    // guest 是 admin 的祖先
		{"admin", "manager", true},  // manager 是 admin 的父角色
		{"admin", "admin", true},    // 自己也在继承链中
		{"guest", "admin", false},   // admin 不是 guest 的祖先
		{"user", "member", false},   // member 不是 user 的祖先
	}

	for _, tt := range tests {
		result := rh.IsAncestor(tt.roleID, tt.ancestorID)
		if result != tt.expected {
			t.Errorf("IsAncestor(%s, %s): got %v, want %v",
				tt.roleID, tt.ancestorID, result, tt.expected)
		}
	}
}

func TestRoleHierarchy_GetAllDescendants(t *testing.T) {
	rh := NewRoleHierarchy()

	// guest 的后代应该包含 user, member, editor, manager, admin, owner
	descendants := rh.GetAllDescendants("guest")

	expected := map[string]bool{
		"user":    true,
		"member":  true,
		"editor":  true,
		"manager": true,
		"admin":   true,
		"owner":   true,
	}

	if len(descendants) != len(expected) {
		t.Errorf("后代数量不匹配: got %d, want %d", len(descendants), len(expected))
	}

	for _, desc := range descendants {
		if !expected[desc] {
			t.Errorf("意外的后代角色: %s", desc)
		}
	}
}

func TestRoleHierarchy_RegisterCustomRole(t *testing.T) {
	rh := NewRoleHierarchy()

	// 注册自定义角色
	customRole := &Role{
		ID:          "super_editor",
		Name:        "超级编辑",
		Level:       35, // 介于 editor(30) 和 manager(40) 之间
		ParentID:    "editor",
		Permissions: []string{"publish", "review"},
	}
	rh.RegisterRole(customRole)

	// 验证角色已注册
	role, ok := rh.GetRole("super_editor")
	if !ok {
		t.Fatal("自定义角色未注册成功")
	}

	if role.Level != 35 {
		t.Errorf("角色级别不匹配: got %d, want 35", role.Level)
	}

	// 验证继承链
	chain := rh.GetInheritanceChain("super_editor")
	if len(chain) < 2 || chain[0].ID != "super_editor" || chain[1].ID != "editor" {
		t.Errorf("自定义角色继承链不正确")
	}

	// 验证继承的权限
	perms := rh.GetInheritedPermissions("super_editor")
	permSet := make(map[string]bool)
	for _, p := range perms {
		permSet[p] = true
	}

	if !permSet["publish"] || !permSet["review"] {
		t.Error("自定义角色的直接权限缺失")
	}
	if !permSet["read"] || !permSet["write"] || !permSet["delete"] {
		t.Error("继承的权限缺失")
	}
}

// ========== 权限服务继承测试 ==========

func TestPermissionService_InheritedRolePermissions(t *testing.T) {
	store := newMockPermissionStore()
	store.userRoles["user1"] = []string{"editor"}

	svc := NewPermissionService(store, 5*time.Minute)
	ctx := context.Background()

	// editor 应该继承 member, user, guest 的权限
	perms, err := svc.ListUserPermissions(ctx, "user1")
	if err != nil {
		t.Fatalf("ListUserPermissions 失败: %v", err)
	}

	// 收集所有继承的动作（可能分布在多个权限条目中）
	allActions := make(map[string]bool)
	var foundInherited bool
	for _, perm := range perms {
		if perm.ResourceType == "*" && perm.ResourceID == "*" {
			foundInherited = true
			for _, a := range perm.Actions {
				allActions[a] = true
			}
		}
	}

	if !foundInherited {
		t.Error("未找到继承的权限条目")
		return
	}

	// editor 应该有 read, write, execute, delete（从继承链累积）
	expectedActions := []string{"read", "write", "execute", "delete"}
	for _, ea := range expectedActions {
		if !allActions[ea] {
			t.Errorf("缺少继承的动作: %s (已有: %v)", ea, allActions)
		}
	}
}

func TestPermissionService_AccumulatedPermissions(t *testing.T) {
	store := newMockPermissionStore()
	store.userRoles["user1"] = []string{"manager"}
	store.userPerms["user1"] = []*ResourcePermission{
		{
			ID:            "perm1",
			PrincipalID:   "user1",
			PrincipalType: "user",
			ResourceType:  "agent",
			ResourceID:    "agent_123",
			Actions:       []string{"custom_action"},
			Effect:        EffectAllow,
		},
	}

	svc := NewPermissionService(store, 5*time.Minute)
	ctx := context.Background()

	accumulated, err := svc.GetUserAccumulatedPermissions(ctx, "user1")
	if err != nil {
		t.Fatalf("GetUserAccumulatedPermissions 失败: %v", err)
	}

	// 检查角色
	if len(accumulated.Roles) != 1 || accumulated.Roles[0] != "manager" {
		t.Errorf("角色不匹配: %v", accumulated.Roles)
	}

	// 检查继承的角色（应包含 manager 及其所有父角色）
	inheritedSet := make(map[string]bool)
	for _, r := range accumulated.InheritedRoles {
		inheritedSet[r] = true
	}
	expected := []string{"manager", "editor", "member", "user", "guest"}
	for _, e := range expected {
		if !inheritedSet[e] {
			t.Errorf("继承角色中缺少: %s", e)
		}
	}

	// 检查最高角色
	if accumulated.HighestRole != "manager" {
		t.Errorf("最高角色不匹配: got %s, want manager", accumulated.HighestRole)
	}

	if accumulated.HighestLevel != RoleLevelManager {
		t.Errorf("最高级别不匹配: got %d, want %d", accumulated.HighestLevel, RoleLevelManager)
	}
}

func TestPermissionService_HasRoleLevel(t *testing.T) {
	store := newMockPermissionStore()
	store.userRoles["user1"] = []string{"editor"}
	store.userRoles["user2"] = []string{"admin"}
	store.userRoles["user3"] = []string{"guest"}

	svc := NewPermissionService(store, 5*time.Minute)
	ctx := context.Background()

	tests := []struct {
		userID   string
		level    RoleLevel
		expected bool
	}{
		{"user1", RoleLevelEditor, true},
		{"user1", RoleLevelManager, false},
		{"user2", RoleLevelAdmin, true},
		{"user2", RoleLevelOwner, false},
		{"user3", RoleLevelGuest, true},
		{"user3", RoleLevelUser, false},
	}

	for _, tt := range tests {
		result, err := svc.HasRoleLevel(ctx, tt.userID, tt.level)
		if err != nil {
			t.Errorf("HasRoleLevel(%s, %d) 失败: %v", tt.userID, tt.level, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("HasRoleLevel(%s, %d): got %v, want %v",
				tt.userID, tt.level, result, tt.expected)
		}
	}
}

func TestPermissionService_CheckRoleHierarchy(t *testing.T) {
	store := newMockPermissionStore()
	store.userRoles["admin_user"] = []string{"admin"}
	store.userRoles["manager_user"] = []string{"manager"}
	store.userRoles["editor_user"] = []string{"editor"}

	svc := NewPermissionService(store, 5*time.Minute)
	ctx := context.Background()

	tests := []struct {
		userID       string
		targetRoleID string
		canManage    bool
	}{
		{"admin_user", "manager", true},   // admin 可以管理 manager
		{"admin_user", "editor", true},    // admin 可以管理 editor
		{"manager_user", "editor", true},  // manager 可以管理 editor
		{"manager_user", "admin", false},  // manager 不能管理 admin
		{"editor_user", "manager", false}, // editor 不能管理 manager
		{"admin_user", "admin", false},    // admin 不能管理同级 admin
	}

	for _, tt := range tests {
		result, err := svc.CheckRoleHierarchy(ctx, tt.userID, tt.targetRoleID)
		if err != nil {
			t.Errorf("CheckRoleHierarchy(%s, %s) 失败: %v",
				tt.userID, tt.targetRoleID, err)
			continue
		}
		if result != tt.canManage {
			t.Errorf("CheckRoleHierarchy(%s, %s): got %v, want %v",
				tt.userID, tt.targetRoleID, result, tt.canManage)
		}
	}
}

func TestPermissionService_MultipleRoles(t *testing.T) {
	store := newMockPermissionStore()
	// 用户同时拥有多个角色
	store.userRoles["multi_role_user"] = []string{"editor", "guest"}

	svc := NewPermissionService(store, 5*time.Minute)
	ctx := context.Background()

	accumulated, err := svc.GetUserAccumulatedPermissions(ctx, "multi_role_user")
	if err != nil {
		t.Fatalf("GetUserAccumulatedPermissions 失败: %v", err)
	}

	// 最高角色应该是 editor
	if accumulated.HighestRole != "editor" {
		t.Errorf("最高角色不匹配: got %s, want editor", accumulated.HighestRole)
	}

	// 应该有 editor 的所有权限
	actionSet := make(map[string]bool)
	for _, a := range accumulated.AllActions {
		actionSet[a] = true
	}

	expectedActions := []string{"read", "write", "execute", "delete"}
	for _, ea := range expectedActions {
		if !actionSet[ea] {
			t.Errorf("多角色用户缺少动作: %s", ea)
		}
	}
}

// ========== 循环引用防护测试 ==========

func TestRoleHierarchy_CycleProtection(t *testing.T) {
	rh := &RoleHierarchy{
		roles:    make(map[string]*Role),
		children: make(map[string][]string),
	}

	// 创建循环引用: A -> B -> C -> A
	rh.roles["roleA"] = &Role{ID: "roleA", ParentID: "roleC", Permissions: []string{"permA"}}
	rh.roles["roleB"] = &Role{ID: "roleB", ParentID: "roleA", Permissions: []string{"permB"}}
	rh.roles["roleC"] = &Role{ID: "roleC", ParentID: "roleB", Permissions: []string{"permC"}}

	// 获取继承链不应该无限循环
	chain := rh.GetInheritanceChain("roleA")

	// 应该检测到循环并停止
	if len(chain) > 3 {
		t.Errorf("循环检测失败，继承链过长: %d", len(chain))
	}
}
