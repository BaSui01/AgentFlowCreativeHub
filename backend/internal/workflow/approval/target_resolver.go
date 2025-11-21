package approval

import (
	"context"
	"fmt"
	"strings"

	"backend/internal/logger"
	"backend/internal/tenant"
	workflowpkg "backend/internal/workflow"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TargetResolver 负责根据审批请求解析可用的通知目标
type TargetResolver interface {
	Resolve(ctx context.Context, approval *workflowpkg.ApprovalRequest) (*ResolvedTargets, error)
	Settings(ctx context.Context, tenantID string) *tenant.ApprovalSettings
}

// ResolvedTargets 包含每个渠道最终可用的目标清单
type ResolvedTargets struct {
	Channels      map[string][]string
	FallbackOrder []string
}

// ConfigTargetResolver 基于租户配置与请求上下文解析通知目标
type ConfigTargetResolver struct {
	db        *gorm.DB
	configSvc tenant.TenantConfigService
	logger    *zap.Logger
}

// NewConfigTargetResolver 创建解析器
func NewConfigTargetResolver(db *gorm.DB, configSvc tenant.TenantConfigService) *ConfigTargetResolver {
	return &ConfigTargetResolver{
		db:        db,
		configSvc: configSvc,
		logger:    logger.Get(),
	}
}

// Resolve 综合审批请求与租户配置，输出各渠道目标
func (r *ConfigTargetResolver) Resolve(ctx context.Context, approval *workflowpkg.ApprovalRequest) (*ResolvedTargets, error) {
	if approval == nil {
		return nil, fmt.Errorf("approval 请求不能为空")
	}

	settings := r.loadApprovalSettings(ctx, approval.TenantID)
	channels := dedupStrings(mergeStringSlices(approval.NotifyChannels, settings.DefaultChannels))
	if len(channels) == 0 {
		channels = []string{"websocket", "email"}
	}
	result := &ResolvedTargets{
		Channels:      make(map[string][]string, len(channels)),
		FallbackOrder: settings.ChannelFallbackOrder,
	}
	if len(result.FallbackOrder) == 0 {
		result.FallbackOrder = []string{"websocket", "email", "webhook"}
	}

	for _, channel := range channels {
		targets := r.collectChannelTargets(ctx, approval, channel, settings)
		if len(targets) > 0 {
			result.Channels[channel] = dedupStrings(targets)
		}
	}

	return result, nil
}

// Settings 返回指定租户的审批配置，主要用于速率限制
func (r *ConfigTargetResolver) Settings(ctx context.Context, tenantID string) *tenant.ApprovalSettings {
	return r.loadApprovalSettings(ctx, tenantID)
}

func (r *ConfigTargetResolver) collectChannelTargets(ctx context.Context, approval *workflowpkg.ApprovalRequest, channel string, settings *tenant.ApprovalSettings) []string {
	sources := make([]string, 0, 4)
	if approval.NotifyTargets != nil {
		sources = append(sources, approval.NotifyTargets[channel]...)
	}
	if settings != nil && settings.NotificationTargets != nil {
		sources = append(sources, settings.NotificationTargets[channel]...)
	}

	var resolved []string
	for _, spec := range sources {
		values := r.expandTargetSpec(ctx, approval, channel, spec)
		resolved = append(resolved, values...)
	}

	if len(resolved) == 0 {
		fallback := r.defaultTargetForChannel(ctx, approval, channel)
		if fallback != "" {
			resolved = append(resolved, fallback)
		}
	}
	return resolved
}

func (r *ConfigTargetResolver) expandTargetSpec(ctx context.Context, approval *workflowpkg.ApprovalRequest, channel, spec string) []string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return []string{spec}
	}
	kind := strings.ToLower(parts[0])
	value := strings.TrimSpace(parts[1])
	if value == "" {
		return nil
	}

	switch kind {
	case "user":
		if channel == "email" {
			if email := r.lookupUserEmail(ctx, approval.TenantID, value); email != "" {
				return []string{email}
			}
			return nil
		}
		return []string{value}
	case "email":
		return []string{value}
	case "webhook":
		return []string{value}
	case "role":
		return r.targetsByRole(ctx, approval.TenantID, value, channel)
	default:
		return []string{spec}
	}
}

func (r *ConfigTargetResolver) defaultTargetForChannel(ctx context.Context, approval *workflowpkg.ApprovalRequest, channel string) string {
	switch channel {
	case "websocket":
		return approval.RequestedBy
	case "email":
		return r.lookupUserEmail(ctx, approval.TenantID, approval.RequestedBy)
	case "webhook":
		return ""
	default:
		return ""
	}
}

func (r *ConfigTargetResolver) lookupUserEmail(ctx context.Context, tenantID, userID string) string {
	if tenantID == "" || userID == "" {
		return ""
	}
	var user tenant.User
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", userID, tenantID).
		Select("email").First(&user).Error
	if err != nil {
		if r.logger != nil {
			r.logger.Debug("查询用户邮箱失败", zap.String("tenantId", tenantID), zap.String("userId", userID), zap.Error(err))
		}
		return ""
	}
	return strings.TrimSpace(user.Email)
}

func (r *ConfigTargetResolver) targetsByRole(ctx context.Context, tenantID, roleCode, channel string) []string {
	if tenantID == "" || roleCode == "" {
		return nil
	}
	var rows []struct {
		UserID string
		Email  string
	}
	query := r.db.WithContext(ctx).
		Table("user_roles AS ur").
		Select("u.id AS user_id, u.email").
		Joins("JOIN users u ON ur.user_id = u.id").
		Joins("JOIN roles r ON ur.role_id = r.id").
		Where("ur.tenant_id = ? AND (r.code = ? OR r.name = ?)", tenantID, roleCode, roleCode)
	if err := query.Find(&rows).Error; err != nil {
		if r.logger != nil {
			r.logger.Debug("查询角色成员失败", zap.String("tenantId", tenantID), zap.String("role", roleCode), zap.Error(err))
		}
		return nil
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if channel == "email" {
			if strings.TrimSpace(row.Email) != "" {
				values = append(values, strings.TrimSpace(row.Email))
			}
		} else {
			values = append(values, row.UserID)
		}
	}
	return values
}

func (r *ConfigTargetResolver) loadApprovalSettings(ctx context.Context, tenantID string) *tenant.ApprovalSettings {
	if r.configSvc == nil || tenantID == "" {
		return defaultApprovalSettings()
	}
	tCtx := tenant.TenantContext{TenantID: tenantID}
	cfgCtx := tenant.WithTenantContext(ctx, tCtx)
	cfg, err := r.configSvc.GetConfig(cfgCtx)
	if err != nil || cfg == nil || cfg.ApprovalSettings == nil {
		return defaultApprovalSettings()
	}
	return cfg.ApprovalSettings
}

func defaultApprovalSettings() *tenant.ApprovalSettings {
	return &tenant.ApprovalSettings{
		DefaultChannels:      []string{"websocket", "email"},
		ChannelFallbackOrder: []string{"websocket", "email", "webhook"},
		NotificationTargets:  map[string][]string{},
	}
}

func mergeStringSlices(values ...[]string) []string {
	var merged []string
	for _, slice := range values {
		if len(slice) == 0 {
			continue
		}
		merged = append(merged, slice...)
	}
	return merged
}

func dedupStrings(items []string) []string {
	set := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		val := strings.TrimSpace(item)
		if val == "" {
			continue
		}
		if _, ok := set[val]; ok {
			continue
		}
		set[val] = struct{}{}
		result = append(result, val)
	}
	return result
}
