package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ToolPermissionService 工具权限控制服务
type ToolPermissionService struct {
	store   ToolPermissionStore
	cache   map[string]*ToolPermissionSet
	cacheMu sync.RWMutex
}

// ToolPermissionStore 工具权限存储接口
type ToolPermissionStore interface {
	GetUserToolPermissions(ctx context.Context, userID string) (*ToolPermissionSet, error)
	GetRoleToolPermissions(ctx context.Context, roleID string) (*ToolPermissionSet, error)
	SetUserToolPermissions(ctx context.Context, userID string, perms *ToolPermissionSet) error
	GetUserRoles(ctx context.Context, userID string) ([]string, error)
}

// ToolPermissionSet 工具权限集
type ToolPermissionSet struct {
	UserID          string           `json:"user_id,omitempty"`
	RoleID          string           `json:"role_id,omitempty"`
	AllowedTools    []string         `json:"allowed_tools,omitempty"`    // 允许的工具列表
	DeniedTools     []string         `json:"denied_tools,omitempty"`     // 禁止的工具列表
	ToolLimits      map[string]Limit `json:"tool_limits,omitempty"`      // 工具使用限制
	DailyQuota      int              `json:"daily_quota,omitempty"`      // 每日总调用限额
	MonthlyQuota    int              `json:"monthly_quota,omitempty"`    // 每月总调用限额
	AllowAll        bool             `json:"allow_all"`                  // 允许所有工具
	UpdatedAt       time.Time        `json:"updated_at"`
}

// Limit 使用限制
type Limit struct {
	MaxCallsPerDay    int           `json:"max_calls_per_day,omitempty"`
	MaxCallsPerHour   int           `json:"max_calls_per_hour,omitempty"`
	MaxExecutionTime  time.Duration `json:"max_execution_time,omitempty"`
	MaxInputSize      int64         `json:"max_input_size,omitempty"`
	MaxOutputSize     int64         `json:"max_output_size,omitempty"`
	AllowedParams     []string      `json:"allowed_params,omitempty"`   // 限制可用参数
	DeniedParams      []string      `json:"denied_params,omitempty"`    // 禁止的参数
	RequireApproval   bool          `json:"require_approval,omitempty"` // 需要审批
}

// UsageCounter 使用计数器
type UsageCounter struct {
	mu       sync.RWMutex
	daily    map[string]map[string]int // userID -> toolName -> count
	hourly   map[string]map[string]int
	lastDay  string
	lastHour string
}

var (
	ErrToolNotAllowed     = errors.New("tool not allowed")
	ErrQuotaExceeded      = errors.New("quota exceeded")
	ErrParamNotAllowed    = errors.New("parameter not allowed")
	ErrApprovalRequired   = errors.New("approval required")
)

// NewToolPermissionService 创建工具权限服务
func NewToolPermissionService(store ToolPermissionStore) *ToolPermissionService {
	return &ToolPermissionService{
		store: store,
		cache: make(map[string]*ToolPermissionSet),
	}
}

// CheckToolPermission 检查工具使用权限
func (s *ToolPermissionService) CheckToolPermission(ctx context.Context, userID, toolName string) error {
	perms, err := s.getUserEffectivePermissions(ctx, userID)
	if err != nil {
		return err
	}

	// 检查是否允许
	if !s.isToolAllowed(perms, toolName) {
		return fmt.Errorf("%w: %s", ErrToolNotAllowed, toolName)
	}

	return nil
}

// CheckToolUsage 检查工具使用（包含限制）
func (s *ToolPermissionService) CheckToolUsage(ctx context.Context, req *ToolUsageRequest) error {
	perms, err := s.getUserEffectivePermissions(ctx, req.UserID)
	if err != nil {
		return err
	}

	// 检查工具是否允许
	if !s.isToolAllowed(perms, req.ToolName) {
		return fmt.Errorf("%w: %s", ErrToolNotAllowed, req.ToolName)
	}

	// 检查工具特定限制
	if limit, ok := perms.ToolLimits[req.ToolName]; ok {
		if err := s.checkLimit(&limit, req); err != nil {
			return err
		}
	}

	return nil
}

// ToolUsageRequest 工具使用请求
type ToolUsageRequest struct {
	UserID     string
	ToolName   string
	Params     map[string]any
	InputSize  int64
	DailyCount int
	HourlyCount int
}

func (s *ToolPermissionService) getUserEffectivePermissions(ctx context.Context, userID string) (*ToolPermissionSet, error) {
	// 检查缓存
	s.cacheMu.RLock()
	cached, ok := s.cache[userID]
	s.cacheMu.RUnlock()
	if ok {
		return cached, nil
	}

	// 获取用户权限
	userPerms, err := s.store.GetUserToolPermissions(ctx, userID)
	if err != nil {
		userPerms = &ToolPermissionSet{}
	}

	// 获取角色权限
	roles, _ := s.store.GetUserRoles(ctx, userID)
	for _, roleID := range roles {
		rolePerms, err := s.store.GetRoleToolPermissions(ctx, roleID)
		if err != nil {
			continue
		}
		userPerms = s.mergePermissions(userPerms, rolePerms)
	}

	// 缓存
	s.cacheMu.Lock()
	s.cache[userID] = userPerms
	s.cacheMu.Unlock()

	return userPerms, nil
}

func (s *ToolPermissionService) isToolAllowed(perms *ToolPermissionSet, toolName string) bool {
	// 检查黑名单
	for _, denied := range perms.DeniedTools {
		if denied == toolName || denied == "*" {
			return false
		}
	}

	// 允许所有
	if perms.AllowAll {
		return true
	}

	// 检查白名单
	if len(perms.AllowedTools) == 0 {
		return true // 无白名单限制
	}

	for _, allowed := range perms.AllowedTools {
		if allowed == toolName || allowed == "*" {
			return true
		}
	}

	return false
}

func (s *ToolPermissionService) checkLimit(limit *Limit, req *ToolUsageRequest) error {
	// 检查每日限额
	if limit.MaxCallsPerDay > 0 && req.DailyCount >= limit.MaxCallsPerDay {
		return fmt.Errorf("%w: daily limit %d", ErrQuotaExceeded, limit.MaxCallsPerDay)
	}

	// 检查每小时限额
	if limit.MaxCallsPerHour > 0 && req.HourlyCount >= limit.MaxCallsPerHour {
		return fmt.Errorf("%w: hourly limit %d", ErrQuotaExceeded, limit.MaxCallsPerHour)
	}

	// 检查输入大小
	if limit.MaxInputSize > 0 && req.InputSize > limit.MaxInputSize {
		return fmt.Errorf("%w: input size %d > %d", ErrQuotaExceeded, req.InputSize, limit.MaxInputSize)
	}

	// 检查参数白名单
	if len(limit.AllowedParams) > 0 {
		for param := range req.Params {
			if !contains(limit.AllowedParams, param) {
				return fmt.Errorf("%w: %s", ErrParamNotAllowed, param)
			}
		}
	}

	// 检查参数黑名单
	if len(limit.DeniedParams) > 0 {
		for param := range req.Params {
			if contains(limit.DeniedParams, param) {
				return fmt.Errorf("%w: %s", ErrParamNotAllowed, param)
			}
		}
	}

	// 检查是否需要审批
	if limit.RequireApproval {
		return ErrApprovalRequired
	}

	return nil
}

func (s *ToolPermissionService) mergePermissions(base, other *ToolPermissionSet) *ToolPermissionSet {
	if other == nil {
		return base
	}

	// 合并允许列表
	base.AllowedTools = append(base.AllowedTools, other.AllowedTools...)
	
	// 合并禁止列表（禁止优先）
	base.DeniedTools = append(base.DeniedTools, other.DeniedTools...)

	// 合并限制（取更严格的）
	if base.ToolLimits == nil {
		base.ToolLimits = make(map[string]Limit)
	}
	for tool, limit := range other.ToolLimits {
		if existing, ok := base.ToolLimits[tool]; ok {
			base.ToolLimits[tool] = s.stricterLimit(existing, limit)
		} else {
			base.ToolLimits[tool] = limit
		}
	}

	// AllowAll 只有都为 true 才允许
	base.AllowAll = base.AllowAll && other.AllowAll

	return base
}

func (s *ToolPermissionService) stricterLimit(a, b Limit) Limit {
	result := a

	if b.MaxCallsPerDay > 0 && (a.MaxCallsPerDay == 0 || b.MaxCallsPerDay < a.MaxCallsPerDay) {
		result.MaxCallsPerDay = b.MaxCallsPerDay
	}
	if b.MaxCallsPerHour > 0 && (a.MaxCallsPerHour == 0 || b.MaxCallsPerHour < a.MaxCallsPerHour) {
		result.MaxCallsPerHour = b.MaxCallsPerHour
	}
	if b.MaxInputSize > 0 && (a.MaxInputSize == 0 || b.MaxInputSize < a.MaxInputSize) {
		result.MaxInputSize = b.MaxInputSize
	}

	result.RequireApproval = a.RequireApproval || b.RequireApproval

	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SetUserToolPermissions 设置用户工具权限
func (s *ToolPermissionService) SetUserToolPermissions(ctx context.Context, userID string, perms *ToolPermissionSet) error {
	perms.UserID = userID
	perms.UpdatedAt = time.Now()

	err := s.store.SetUserToolPermissions(ctx, userID, perms)
	if err != nil {
		return err
	}

	// 清除缓存
	s.cacheMu.Lock()
	delete(s.cache, userID)
	s.cacheMu.Unlock()

	return nil
}

// GetUserToolPermissions 获取用户工具权限
func (s *ToolPermissionService) GetUserToolPermissions(ctx context.Context, userID string) (*ToolPermissionSet, error) {
	return s.getUserEffectivePermissions(ctx, userID)
}

// ListAllowedTools 列出用户允许使用的工具
func (s *ToolPermissionService) ListAllowedTools(ctx context.Context, userID string, allTools []string) ([]string, error) {
	perms, err := s.getUserEffectivePermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	allowed := make([]string, 0)
	for _, tool := range allTools {
		if s.isToolAllowed(perms, tool) {
			allowed = append(allowed, tool)
		}
	}

	return allowed, nil
}

// InvalidateCache 清除缓存
func (s *ToolPermissionService) InvalidateCache(userID string) {
	s.cacheMu.Lock()
	delete(s.cache, userID)
	s.cacheMu.Unlock()
}
