package auth

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============ 角色层级与权限继承 ============

// RoleLevel 角色层级定义
type RoleLevel int

const (
	RoleLevelGuest   RoleLevel = 0   // 访客
	RoleLevelUser    RoleLevel = 10  // 普通用户
	RoleLevelMember  RoleLevel = 20  // 成员
	RoleLevelEditor  RoleLevel = 30  // 编辑者
	RoleLevelManager RoleLevel = 40  // 管理者
	RoleLevelAdmin   RoleLevel = 50  // 管理员
	RoleLevelOwner   RoleLevel = 100 // 所有者
)

// Role 角色定义
type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Level       RoleLevel `json:"level"`
	ParentID    string    `json:"parent_id,omitempty"` // 父角色ID，用于继承
	Permissions []string  `json:"permissions"`         // 该角色直接拥有的权限标识
	Description string    `json:"description,omitempty"`
}

// RoleHierarchy 角色层级管理器
type RoleHierarchy struct {
	roles    map[string]*Role // roleID -> Role
	children map[string][]string // parentID -> childIDs
	mu       sync.RWMutex
}

// NewRoleHierarchy 创建角色层级管理器
func NewRoleHierarchy() *RoleHierarchy {
	rh := &RoleHierarchy{
		roles:    make(map[string]*Role),
		children: make(map[string][]string),
	}
	// 初始化默认角色层级
	rh.initDefaultRoles()
	return rh
}

// initDefaultRoles 初始化默认角色
func (rh *RoleHierarchy) initDefaultRoles() {
	defaultRoles := []*Role{
		{ID: "guest", Name: "访客", Level: RoleLevelGuest, Permissions: []string{"read"}},
		{ID: "user", Name: "用户", Level: RoleLevelUser, ParentID: "guest", Permissions: []string{"write"}},
		{ID: "member", Name: "成员", Level: RoleLevelMember, ParentID: "user", Permissions: []string{"execute"}},
		{ID: "editor", Name: "编辑者", Level: RoleLevelEditor, ParentID: "member", Permissions: []string{"delete"}},
		{ID: "manager", Name: "管理者", Level: RoleLevelManager, ParentID: "editor", Permissions: []string{"manage"}},
		{ID: "admin", Name: "管理员", Level: RoleLevelAdmin, ParentID: "manager", Permissions: []string{"admin"}},
		{ID: "owner", Name: "所有者", Level: RoleLevelOwner, ParentID: "admin", Permissions: []string{"*"}},
	}

	for _, role := range defaultRoles {
		rh.RegisterRole(role)
	}
}

// RegisterRole 注册角色
func (rh *RoleHierarchy) RegisterRole(role *Role) {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	rh.roles[role.ID] = role
	if role.ParentID != "" {
		rh.children[role.ParentID] = append(rh.children[role.ParentID], role.ID)
	}
}

// GetRole 获取角色
func (rh *RoleHierarchy) GetRole(roleID string) (*Role, bool) {
	rh.mu.RLock()
	defer rh.mu.RUnlock()
	role, ok := rh.roles[roleID]
	return role, ok
}

// GetInheritanceChain 获取角色继承链（从当前角色到根角色）
func (rh *RoleHierarchy) GetInheritanceChain(roleID string) []*Role {
	rh.mu.RLock()
	defer rh.mu.RUnlock()

	var chain []*Role
	visited := make(map[string]bool) // 防止循环

	currentID := roleID
	for currentID != "" && !visited[currentID] {
		visited[currentID] = true
		if role, ok := rh.roles[currentID]; ok {
			chain = append(chain, role)
			currentID = role.ParentID
		} else {
			break
		}
	}

	return chain
}

// GetInheritedPermissions 获取角色继承的所有权限（累积）
func (rh *RoleHierarchy) GetInheritedPermissions(roleID string) []string {
	chain := rh.GetInheritanceChain(roleID)

	permSet := make(map[string]bool)
	for _, role := range chain {
		for _, perm := range role.Permissions {
			permSet[perm] = true
		}
	}

	permissions := make([]string, 0, len(permSet))
	for perm := range permSet {
		permissions = append(permissions, perm)
	}
	sort.Strings(permissions)
	return permissions
}

// IsRoleHigherOrEqual 检查 roleA 是否高于或等于 roleB
func (rh *RoleHierarchy) IsRoleHigherOrEqual(roleA, roleB string) bool {
	rh.mu.RLock()
	defer rh.mu.RUnlock()

	a, okA := rh.roles[roleA]
	b, okB := rh.roles[roleB]

	if !okA || !okB {
		return false
	}

	return a.Level >= b.Level
}

// IsAncestor 检查 ancestorID 是否是 roleID 的祖先角色
func (rh *RoleHierarchy) IsAncestor(roleID, ancestorID string) bool {
	chain := rh.GetInheritanceChain(roleID)
	for _, role := range chain {
		if role.ID == ancestorID {
			return true
		}
	}
	return false
}

// GetAllDescendants 获取角色的所有后代角色
func (rh *RoleHierarchy) GetAllDescendants(roleID string) []string {
	rh.mu.RLock()
	defer rh.mu.RUnlock()

	var descendants []string
	visited := make(map[string]bool)

	var collect func(id string)
	collect = func(id string) {
		for _, childID := range rh.children[id] {
			if !visited[childID] {
				visited[childID] = true
				descendants = append(descendants, childID)
				collect(childID)
			}
		}
	}

	collect(roleID)
	return descendants
}

// PermissionService 细粒度权限控制服务
type PermissionService struct {
	store         PermissionStore
	cache         map[string]*CachedPermission
	cacheMu       sync.RWMutex
	cacheTTL      time.Duration
	roleHierarchy *RoleHierarchy // 角色层级管理器
}

// PermissionStore 权限存储接口
type PermissionStore interface {
	GetUserPermissions(ctx context.Context, userID string) ([]*ResourcePermission, error)
	GetRolePermissions(ctx context.Context, roleID string) ([]*ResourcePermission, error)
	SetPermission(ctx context.Context, perm *ResourcePermission) error
	DeletePermission(ctx context.Context, id string) error
	GetUserRoles(ctx context.Context, userID string) ([]string, error)
}

// ResourcePermission 资源权限
type ResourcePermission struct {
	ID           string         `json:"id"`
	PrincipalID  string         `json:"principal_id"`   // 用户或角色 ID
	PrincipalType string        `json:"principal_type"` // user / role
	ResourceType string         `json:"resource_type"`  // agent / workflow / kb / workspace
	ResourceID   string         `json:"resource_id"`    // 具体资源 ID 或 * 表示所有
	Actions      []string       `json:"actions"`        // read / write / delete / execute / manage
	Conditions   []Condition    `json:"conditions,omitempty"` // 条件限制
	Effect       Effect         `json:"effect"`         // allow / deny
	Priority     int            `json:"priority"`       // 优先级（deny 通常更高）
	GrantedBy    string         `json:"granted_by"`
	GrantedAt    time.Time      `json:"granted_at"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"`
}

// Effect 权限效果
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Condition 条件
type Condition struct {
	Field    string `json:"field"`    // ip / time / tenant
	Operator string `json:"operator"` // eq / ne / in / not_in / gt / lt
	Value    any    `json:"value"`
}

// CachedPermission 缓存的权限
type CachedPermission struct {
	Permissions []*ResourcePermission
	CachedAt    time.Time
}

// PermissionRequest 权限检查请求
type PermissionRequest struct {
	UserID       string
	ResourceType string
	ResourceID   string
	Action       string
	Context      map[string]any // IP、时间等上下文
}

var (
	ErrAccessDenied = errors.New("access denied")
)

// NewPermissionService 创建权限服务
func NewPermissionService(store PermissionStore, cacheTTL time.Duration) *PermissionService {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &PermissionService{
		store:         store,
		cache:         make(map[string]*CachedPermission),
		cacheTTL:      cacheTTL,
		roleHierarchy: NewRoleHierarchy(),
	}
}

// NewPermissionServiceWithHierarchy 使用自定义角色层级创建权限服务
func NewPermissionServiceWithHierarchy(store PermissionStore, cacheTTL time.Duration, rh *RoleHierarchy) *PermissionService {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	if rh == nil {
		rh = NewRoleHierarchy()
	}
	return &PermissionService{
		store:         store,
		cache:         make(map[string]*CachedPermission),
		cacheTTL:      cacheTTL,
		roleHierarchy: rh,
	}
}

// GetRoleHierarchy 获取角色层级管理器
func (s *PermissionService) GetRoleHierarchy() *RoleHierarchy {
	return s.roleHierarchy
}

// CheckPermission 检查权限
func (s *PermissionService) CheckPermission(ctx context.Context, req *PermissionRequest) error {
	decision := s.Evaluate(ctx, req)
	if !decision.Allowed {
		return fmt.Errorf("%w: %s", ErrAccessDenied, decision.Reason)
	}
	return nil
}

// PermissionDecision 权限决策
type PermissionDecision struct {
	Allowed    bool   `json:"allowed"`
	Reason     string `json:"reason"`
	MatchedBy  string `json:"matched_by,omitempty"` // 匹配的权限 ID
}

// Evaluate 评估权限
func (s *PermissionService) Evaluate(ctx context.Context, req *PermissionRequest) *PermissionDecision {
	// 获取用户权限
	perms, err := s.getUserEffectivePermissions(ctx, req.UserID)
	if err != nil {
		return &PermissionDecision{
			Allowed: false,
			Reason:  "failed to get permissions",
		}
	}

	// 过滤相关权限
	relevant := s.filterRelevantPermissions(perms, req)

	// 按优先级排序（deny 优先）
	s.sortByPriority(relevant)

	// 评估
	for _, perm := range relevant {
		// 检查过期
		if perm.ExpiresAt != nil && perm.ExpiresAt.Before(time.Now()) {
			continue
		}

		// 检查条件
		if !s.checkConditions(perm.Conditions, req.Context) {
			continue
		}

		// 检查操作
		if !s.matchAction(perm.Actions, req.Action) {
			continue
		}

		// 返回决策
		if perm.Effect == EffectDeny {
			return &PermissionDecision{
				Allowed:   false,
				Reason:    "explicitly denied",
				MatchedBy: perm.ID,
			}
		}

		return &PermissionDecision{
			Allowed:   true,
			Reason:    "allowed by permission",
			MatchedBy: perm.ID,
		}
	}

	// 默认拒绝
	return &PermissionDecision{
		Allowed: false,
		Reason:  "no matching permission",
	}
}

func (s *PermissionService) getUserEffectivePermissions(ctx context.Context, userID string) ([]*ResourcePermission, error) {
	// 检查缓存
	s.cacheMu.RLock()
	cached, ok := s.cache[userID]
	if ok && time.Since(cached.CachedAt) < s.cacheTTL {
		s.cacheMu.RUnlock()
		return cached.Permissions, nil
	}
	s.cacheMu.RUnlock()

	// 获取用户直接权限
	userPerms, err := s.store.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 获取用户角色
	roles, err := s.store.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 收集所有角色（包括继承的角色）
	allRoleIDs := s.collectInheritedRoles(roles)

	// 获取所有角色的权限（包括继承的）
	for _, roleID := range allRoleIDs {
		rolePerms, err := s.store.GetRolePermissions(ctx, roleID)
		if err != nil {
			continue
		}
		userPerms = append(userPerms, rolePerms...)

		// 添加角色层级定义的基础权限
		if s.roleHierarchy != nil {
			inheritedActions := s.roleHierarchy.GetInheritedPermissions(roleID)
			if len(inheritedActions) > 0 {
				// 为继承的权限创建通配符资源权限
				inheritedPerm := &ResourcePermission{
					ID:            fmt.Sprintf("inherited_%s_%d", roleID, time.Now().UnixNano()),
					PrincipalID:   roleID,
					PrincipalType: "role",
					ResourceType:  "*",
					ResourceID:    "*",
					Actions:       inheritedActions,
					Effect:        EffectAllow,
					Priority:      -1, // 低优先级，可被具体权限覆盖
				}
				userPerms = append(userPerms, inheritedPerm)
			}
		}
	}

	// 缓存
	s.cacheMu.Lock()
	s.cache[userID] = &CachedPermission{
		Permissions: userPerms,
		CachedAt:    time.Now(),
	}
	s.cacheMu.Unlock()

	return userPerms, nil
}

// collectInheritedRoles 收集角色及其继承链上的所有角色
func (s *PermissionService) collectInheritedRoles(roles []string) []string {
	if s.roleHierarchy == nil {
		return roles
	}

	roleSet := make(map[string]bool)

	for _, roleID := range roles {
		// 获取角色的继承链
		chain := s.roleHierarchy.GetInheritanceChain(roleID)
		for _, role := range chain {
			roleSet[role.ID] = true
		}
		// 也包含原始角色（即使不在层级管理器中）
		roleSet[roleID] = true
	}

	result := make([]string, 0, len(roleSet))
	for roleID := range roleSet {
		result = append(result, roleID)
	}
	return result
}

// GetUserAccumulatedPermissions 获取用户累积的所有权限（包括继承）
func (s *PermissionService) GetUserAccumulatedPermissions(ctx context.Context, userID string) (*AccumulatedPermissions, error) {
	perms, err := s.getUserEffectivePermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	roles, err := s.store.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 收集所有动作
	actionSet := make(map[string]bool)
	resourcePerms := make(map[string]map[string][]string) // resourceType -> resourceID -> actions

	for _, perm := range perms {
		if perm.Effect != EffectAllow {
			continue
		}

		for _, action := range perm.Actions {
			actionSet[action] = true
		}

		if resourcePerms[perm.ResourceType] == nil {
			resourcePerms[perm.ResourceType] = make(map[string][]string)
		}
		resourcePerms[perm.ResourceType][perm.ResourceID] = append(
			resourcePerms[perm.ResourceType][perm.ResourceID],
			perm.Actions...,
		)
	}

	actions := make([]string, 0, len(actionSet))
	for action := range actionSet {
		actions = append(actions, action)
	}
	sort.Strings(actions)

	// 计算最高角色层级
	var highestLevel RoleLevel
	var highestRole string
	if s.roleHierarchy != nil {
		for _, roleID := range roles {
			if role, ok := s.roleHierarchy.GetRole(roleID); ok {
				if role.Level > highestLevel {
					highestLevel = role.Level
					highestRole = role.ID
				}
			}
		}
	}

	return &AccumulatedPermissions{
		UserID:          userID,
		Roles:           roles,
		InheritedRoles:  s.collectInheritedRoles(roles),
		AllActions:      actions,
		ResourcePerms:   resourcePerms,
		HighestRole:     highestRole,
		HighestLevel:    highestLevel,
		TotalPermCount:  len(perms),
	}, nil
}

// AccumulatedPermissions 累积的权限信息
type AccumulatedPermissions struct {
	UserID          string                         `json:"user_id"`
	Roles           []string                       `json:"roles"`
	InheritedRoles  []string                       `json:"inherited_roles"`
	AllActions      []string                       `json:"all_actions"`
	ResourcePerms   map[string]map[string][]string `json:"resource_perms"`
	HighestRole     string                         `json:"highest_role"`
	HighestLevel    RoleLevel                      `json:"highest_level"`
	TotalPermCount  int                            `json:"total_perm_count"`
}

// HasRoleLevel 检查用户是否具有指定级别或更高的角色
func (s *PermissionService) HasRoleLevel(ctx context.Context, userID string, requiredLevel RoleLevel) (bool, error) {
	roles, err := s.store.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	if s.roleHierarchy == nil {
		return false, nil
	}

	for _, roleID := range roles {
		if role, ok := s.roleHierarchy.GetRole(roleID); ok {
			if role.Level >= requiredLevel {
				return true, nil
			}
		}
	}

	return false, nil
}

// CheckRoleHierarchy 检查用户是否可以管理目标角色
func (s *PermissionService) CheckRoleHierarchy(ctx context.Context, userID, targetRoleID string) (bool, error) {
	roles, err := s.store.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	if s.roleHierarchy == nil {
		return false, nil
	}

	targetRole, ok := s.roleHierarchy.GetRole(targetRoleID)
	if !ok {
		return false, nil
	}

	for _, roleID := range roles {
		if role, ok := s.roleHierarchy.GetRole(roleID); ok {
			// 用户角色级别必须高于目标角色才能管理
			if role.Level > targetRole.Level {
				return true, nil
			}
		}
	}

	return false, nil
}

func (s *PermissionService) filterRelevantPermissions(perms []*ResourcePermission, req *PermissionRequest) []*ResourcePermission {
	relevant := make([]*ResourcePermission, 0)
	for _, perm := range perms {
		// 匹配资源类型
		if perm.ResourceType != req.ResourceType && perm.ResourceType != "*" {
			continue
		}

		// 匹配资源 ID
		if perm.ResourceID != req.ResourceID && perm.ResourceID != "*" {
			// 支持前缀匹配 (e.g., "kb_*")
			if !strings.HasSuffix(perm.ResourceID, "*") {
				continue
			}
			prefix := strings.TrimSuffix(perm.ResourceID, "*")
			if !strings.HasPrefix(req.ResourceID, prefix) {
				continue
			}
		}

		relevant = append(relevant, perm)
	}
	return relevant
}

func (s *PermissionService) sortByPriority(perms []*ResourcePermission) {
	// 简单排序：deny 优先，然后按 priority
	for i := 0; i < len(perms)-1; i++ {
		for j := i + 1; j < len(perms); j++ {
			swap := false
			// deny 优先于 allow
			if perms[j].Effect == EffectDeny && perms[i].Effect == EffectAllow {
				swap = true
			} else if perms[i].Effect == perms[j].Effect && perms[j].Priority > perms[i].Priority {
				swap = true
			}
			if swap {
				perms[i], perms[j] = perms[j], perms[i]
			}
		}
	}
}

func (s *PermissionService) matchAction(allowed []string, requested string) bool {
	for _, a := range allowed {
		if a == "*" || a == requested {
			return true
		}
	}
	return false
}

func (s *PermissionService) checkConditions(conditions []Condition, ctx map[string]any) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, cond := range conditions {
		value, ok := ctx[cond.Field]
		if !ok {
			return false // 缺少上下文值
		}

		if !s.evaluateCondition(cond, value) {
			return false
		}
	}

	return true
}

func (s *PermissionService) evaluateCondition(cond Condition, value any) bool {
	switch cond.Operator {
	case "eq", "==":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", cond.Value)
	case "ne", "!=":
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", cond.Value)
	case "in":
		if arr, ok := cond.Value.([]any); ok {
			for _, v := range arr {
				if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", v) {
					return true
				}
			}
		}
		return false
	case "not_in":
		if arr, ok := cond.Value.([]any); ok {
			for _, v := range arr {
				if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", v) {
					return false
				}
			}
		}
		return true
	default:
		return true
	}
}

// GrantPermission 授予权限
func (s *PermissionService) GrantPermission(ctx context.Context, perm *ResourcePermission) error {
	if perm.ID == "" {
		perm.ID = fmt.Sprintf("perm_%d", time.Now().UnixNano())
	}
	perm.GrantedAt = time.Now()

	err := s.store.SetPermission(ctx, perm)
	if err != nil {
		return err
	}

	// 清除缓存
	s.invalidateCache(perm.PrincipalID)
	return nil
}

// RevokePermission 撤销权限
func (s *PermissionService) RevokePermission(ctx context.Context, permID string) error {
	return s.store.DeletePermission(ctx, permID)
}

// ListUserPermissions 列出用户权限
func (s *PermissionService) ListUserPermissions(ctx context.Context, userID string) ([]*ResourcePermission, error) {
	return s.getUserEffectivePermissions(ctx, userID)
}

func (s *PermissionService) invalidateCache(userID string) {
	s.cacheMu.Lock()
	delete(s.cache, userID)
	s.cacheMu.Unlock()
}

// InvalidateAllCache 清除所有缓存
func (s *PermissionService) InvalidateAllCache() {
	s.cacheMu.Lock()
	s.cache = make(map[string]*CachedPermission)
	s.cacheMu.Unlock()
}

// ========== 便捷方法 ==========

// CanRead 检查读权限
func (s *PermissionService) CanRead(ctx context.Context, userID, resourceType, resourceID string) bool {
	err := s.CheckPermission(ctx, &PermissionRequest{
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       "read",
	})
	return err == nil
}

// CanWrite 检查写权限
func (s *PermissionService) CanWrite(ctx context.Context, userID, resourceType, resourceID string) bool {
	err := s.CheckPermission(ctx, &PermissionRequest{
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       "write",
	})
	return err == nil
}

// CanDelete 检查删除权限
func (s *PermissionService) CanDelete(ctx context.Context, userID, resourceType, resourceID string) bool {
	err := s.CheckPermission(ctx, &PermissionRequest{
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       "delete",
	})
	return err == nil
}

// CanExecute 检查执行权限
func (s *PermissionService) CanExecute(ctx context.Context, userID, resourceType, resourceID string) bool {
	err := s.CheckPermission(ctx, &PermissionRequest{
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       "execute",
	})
	return err == nil
}

// CanManage 检查管理权限
func (s *PermissionService) CanManage(ctx context.Context, userID, resourceType, resourceID string) bool {
	err := s.CheckPermission(ctx, &PermissionRequest{
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       "manage",
	})
	return err == nil
}
