package tenant

import (
	"context"
	"errors"
	"strings"
)

// TenantConfigService provides read and update operations for tenant-level
// configuration, with caching and audit logging.
type TenantConfigService interface {
	GetConfig(ctx context.Context) (*TenantConfig, error)
	UpdateConfig(ctx context.Context, params UpdateTenantConfigParams) (*TenantConfig, error)
}

// UpdateTenantConfigParams contains fields that can be updated for a tenant
// configuration. Zero values mean "keep existing".
type UpdateTenantConfigParams struct {
	DisplayName      *string
	Description      *string
	LogoURL          *string
	Language         *string
	TimeZone         *string
	FeatureFlags     *map[string]bool
	ApprovalSettings *ApprovalSettings
}

type tenantConfigService struct {
	repo  TenantConfigRepository
	cache TenantConfigCache
	audit AuditLogger
}

// NewTenantConfigService constructs a TenantConfigService using the given
// repository, cache and audit logger.
func NewTenantConfigService(repo TenantConfigRepository, cache TenantConfigCache, audit AuditLogger) TenantConfigService {
	return &tenantConfigService{repo: repo, cache: cache, audit: audit}
}

// defaultConfig builds an in-memory default config when none exists.
func defaultConfig(tenantID string) *TenantConfig {
	return &TenantConfig{
		TenantID:     tenantID,
		DisplayName:  "",
		Description:  "",
		LogoURL:      "",
		Language:     "zh-CN",
		TimeZone:     "Asia/Shanghai",
		FeatureFlags: map[string]bool{},
		ApprovalSettings: &ApprovalSettings{
			DefaultChannels:      []string{"websocket", "email"},
			NotificationTargets:  map[string][]string{},
			ChannelFallbackOrder: []string{"websocket", "email", "webhook"},
			OfflineBufferSize:    50,
			OfflineTTLSeconds:    3600,
			ResendLimit:          5,
		},
	}
}

func (s *tenantConfigService) GetConfig(ctx context.Context) (*TenantConfig, error) {
	tc, ok := FromContext(ctx)
	if !ok || tc.TenantID == "" {
		return nil, ErrForbidden
	}

	// 先查缓存
	if s.cache != nil {
		if cfg, ok := s.cache.Get(ctx); ok {
			return cfg, nil
		}
	}

	cfg, err := s.repo.GetByTenantID(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			cfg = defaultConfig(tc.TenantID)
			// 将默认配置写入持久层，避免每次都构造
			if err := s.repo.Upsert(ctx, cfg); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if cfg.ApprovalSettings == nil {
		cfg.ApprovalSettings = normalizeApprovalSettings(ApprovalSettings{})
	}

	if s.cache != nil {
		s.cache.Set(ctx, cfg)
	}
	return cfg, nil
}

func (s *tenantConfigService) UpdateConfig(ctx context.Context, params UpdateTenantConfigParams) (*TenantConfig, error) {
	tc, ok := FromContext(ctx)
	if !ok || tc.TenantID == "" {
		return nil, ErrForbidden
	}

	cfg, err := s.repo.GetByTenantID(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			cfg = defaultConfig(tc.TenantID)
		} else {
			return nil, err
		}
	}

	if params.DisplayName != nil {
		cfg.DisplayName = strings.TrimSpace(*params.DisplayName)
	}
	if params.Description != nil {
		cfg.Description = strings.TrimSpace(*params.Description)
	}
	if params.LogoURL != nil {
		cfg.LogoURL = strings.TrimSpace(*params.LogoURL)
	}
	if params.Language != nil && strings.TrimSpace(*params.Language) != "" {
		cfg.Language = strings.TrimSpace(*params.Language)
	}
	if params.TimeZone != nil && strings.TrimSpace(*params.TimeZone) != "" {
		cfg.TimeZone = strings.TrimSpace(*params.TimeZone)
	}
	if params.FeatureFlags != nil {
		cfg.FeatureFlags = *params.FeatureFlags
	}
	if params.ApprovalSettings != nil {
		cfg.ApprovalSettings = normalizeApprovalSettings(*params.ApprovalSettings)
	}
	if cfg.ApprovalSettings == nil {
		cfg.ApprovalSettings = normalizeApprovalSettings(ApprovalSettings{})
	}

	if err := s.repo.Upsert(ctx, cfg); err != nil {
		return nil, err
	}

	if s.cache != nil {
		// 直接覆盖缓存即可
		s.cache.Set(ctx, cfg)
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "tenant.config.update", "tenant_config", map[string]any{
			"tenantId": tc.TenantID,
		})
	}

	return cfg, nil
}

func normalizeApprovalSettings(input ApprovalSettings) *ApprovalSettings {
	settings := input
	if len(settings.DefaultChannels) == 0 {
		settings.DefaultChannels = []string{"websocket", "email"}
	}
	if len(settings.ChannelFallbackOrder) == 0 {
		settings.ChannelFallbackOrder = []string{"websocket", "email", "webhook"}
	}
	if settings.NotificationTargets == nil {
		settings.NotificationTargets = make(map[string][]string)
	}
	if settings.OfflineBufferSize <= 0 {
		settings.OfflineBufferSize = 50
	}
	if settings.OfflineTTLSeconds <= 0 {
		settings.OfflineTTLSeconds = 3600
	}
	if settings.ResendLimit <= 0 {
		settings.ResendLimit = 5
	}
	return &settings
}
