package tenant

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/internal/common"

	"gorm.io/gorm"
)

// ============================================================================
// 错误定义
// ============================================================================

var (
	// ErrQuotaExceeded 配额超限错误
	ErrQuotaExceeded = errors.New("quota exceeded")

	// ErrQuotaNotFound 配额不存在错误
	ErrQuotaNotFound = errors.New("quota not found")

	// ErrInvalidQuotaOperation 无效的配额操作
	ErrInvalidQuotaOperation = errors.New("invalid quota operation")
)

// ============================================================================
// 资源类型定义
// ============================================================================

// ResourceType 资源类型枚举
type ResourceType string

const (
	ResourceTypeUsers          ResourceType = "users"
	ResourceTypeStorage        ResourceType = "storage"
	ResourceTypeWorkflows      ResourceType = "workflows"
	ResourceTypeKnowledgeBases ResourceType = "knowledge_bases"
	ResourceTypeTokens         ResourceType = "tokens"
	ResourceTypeAPICalls       ResourceType = "api_calls"
)

// ============================================================================
// 服务接口定义
// ============================================================================

// QuotaService 配额服务接口
type QuotaService interface {
	// GetQuota 获取租户配额信息
	GetQuota(ctx context.Context, tenantID string) (*TenantQuota, error)

	// CreateQuota 创建租户配额（通常在创建租户时调用）
	CreateQuota(ctx context.Context, tenantID string, tier string) (*TenantQuota, error)

	// UpdateQuotaLimits 更新配额限制（通常由管理员或套餐升级时调用）
	UpdateQuotaLimits(ctx context.Context, tenantID string, limits map[ResourceType]int64) error

	// CheckLimit 检查是否超过配额限制
	CheckLimit(ctx context.Context, tenantID string, resourceType ResourceType) (bool, error)

	// IncrementUsage 增加资源使用量
	IncrementUsage(ctx context.Context, tenantID string, resourceType ResourceType, delta int64) error

	// DecrementUsage 减少资源使用量
	DecrementUsage(ctx context.Context, tenantID string, resourceType ResourceType, delta int64) error

	// SetUsage 设置资源使用量（直接设置）
	SetUsage(ctx context.Context, tenantID string, resourceType ResourceType, value int64) error

	// GetUsageStats 获取资源使用统计
	GetUsageStats(ctx context.Context, tenantID string) ([]common.UsageStats, error)

	// ResetPeriodicalUsage 重置周期性配额（如每月Token、每日API调用）
	ResetPeriodicalUsage(ctx context.Context, tenantID string) error

	// IsQuotaAvailable 检查配额是否可用（检查+预留）
	IsQuotaAvailable(ctx context.Context, tenantID string, resourceType ResourceType, required int64) (bool, error)

	// ReserveQuota 预留配额（用于长时间操作）
	ReserveQuota(ctx context.Context, tenantID string, resourceType ResourceType, amount int64) error

	// ReleaseQuota 释放预留的配额
	ReleaseQuota(ctx context.Context, tenantID string, resourceType ResourceType, amount int64) error
}

// ============================================================================
// 服务实现
// ============================================================================

// quotaService 配额服务实现
type quotaService struct {
	*common.BaseService
	quotaRepo TenantQuotaRepository
	ids       IDGenerator
	audit     AuditLogger
}

// NewQuotaService 创建配额服务实例
func NewQuotaService(
	db *gorm.DB,
	quotaRepo TenantQuotaRepository,
	ids IDGenerator,
	audit AuditLogger,
) QuotaService {
	return &quotaService{
		BaseService: common.NewBaseService(db),
		quotaRepo:   quotaRepo,
		ids:         ids,
		audit:       audit,
	}
}

// GetQuota 获取租户配额信息
func (s *quotaService) GetQuota(ctx context.Context, tenantID string) (*TenantQuota, error) {
	quota, err := s.quotaRepo.FindByTenantID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuotaNotFound
		}
		return nil, fmt.Errorf("failed to get quota: %w", err)
	}

	return quota, nil
}

// CreateQuota 创建租户配额
func (s *quotaService) CreateQuota(ctx context.Context, tenantID string, tier string) (*TenantQuota, error) {
	// 检查配额是否已存在
	existing, err := s.quotaRepo.FindByTenantID(ctx, tenantID)
	if err == nil && existing != nil {
		return existing, nil // 已存在，直接返回
	}

	// 生成ID
	id, err := s.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate quota ID: %w", err)
	}

	// 根据套餐设置默认配额
	quota := &TenantQuota{
		ID:       id,
		TenantID: tenantID,
	}

	// 设置配额限制（根据套餐）
	s.applyTierLimits(quota, tier)

	// 设置重置时间
	now := time.Now()
	quota.TokenQuotaResetAt = now.AddDate(0, 1, 0) // 下个月
	quota.APIQuotaResetAt = now.AddDate(0, 0, 1)   // 明天

	// 保存到数据库
	if err := s.quotaRepo.Insert(ctx, quota); err != nil {
		return nil, fmt.Errorf("failed to create quota: %w", err)
	}

	// 审计日志
	if s.audit != nil {
		s.audit.LogAction(ctx, TenantContext{TenantID: tenantID}, "quota.created", "quota", map[string]any{
			"tier": tier,
		})
	}

	return quota, nil
}

// applyTierLimits 根据套餐设置配额限制
func (s *quotaService) applyTierLimits(quota *TenantQuota, tier string) {
	switch tier {
	case "free":
		quota.MaxUsers = 10
		quota.MaxStorageMB = 1024          // 1GB
		quota.MaxWorkflows = 10
		quota.MaxKnowledgeBases = 2
		quota.MaxTokensPerMonth = 100000   // 10万Token
		quota.MaxAPICallsPerDay = 1000

	case "basic":
		quota.MaxUsers = 50
		quota.MaxStorageMB = 10240         // 10GB
		quota.MaxWorkflows = 100
		quota.MaxKnowledgeBases = 10
		quota.MaxTokensPerMonth = 1000000  // 100万Token
		quota.MaxAPICallsPerDay = 10000

	case "pro":
		quota.MaxUsers = 200
		quota.MaxStorageMB = 51200         // 50GB
		quota.MaxWorkflows = 500
		quota.MaxKnowledgeBases = 50
		quota.MaxTokensPerMonth = 10000000 // 1000万Token
		quota.MaxAPICallsPerDay = 100000

	case "enterprise":
		quota.MaxUsers = -1                 // 无限制
		quota.MaxStorageMB = -1
		quota.MaxWorkflows = -1
		quota.MaxKnowledgeBases = -1
		quota.MaxTokensPerMonth = -1
		quota.MaxAPICallsPerDay = -1

	default:
		// 默认使用free套餐配额
		s.applyTierLimits(quota, "free")
	}
}

// UpdateQuotaLimits 更新配额限制
func (s *quotaService) UpdateQuotaLimits(ctx context.Context, tenantID string, limits map[ResourceType]int64) error {
	quota, err := s.GetQuota(ctx, tenantID)
	if err != nil {
		return err
	}

	// 更新配额限制
	for resourceType, limit := range limits {
		switch resourceType {
		case ResourceTypeUsers:
			quota.MaxUsers = int(limit)
		case ResourceTypeStorage:
			quota.MaxStorageMB = int(limit)
		case ResourceTypeWorkflows:
			quota.MaxWorkflows = int(limit)
		case ResourceTypeKnowledgeBases:
			quota.MaxKnowledgeBases = int(limit)
		case ResourceTypeTokens:
			quota.MaxTokensPerMonth = limit
		case ResourceTypeAPICalls:
			quota.MaxAPICallsPerDay = int(limit)
		}
	}

	// 保存到数据库
	if err := s.quotaRepo.Update(ctx, quota); err != nil {
		return fmt.Errorf("failed to update quota limits: %w", err)
	}

	// 审计日志
	if s.audit != nil {
		s.audit.LogAction(ctx, TenantContext{TenantID: tenantID}, "quota.limits_updated", "quota", limits)
	}

	return nil
}

// CheckLimit 检查是否超过配额限制
func (s *quotaService) CheckLimit(ctx context.Context, tenantID string, resourceType ResourceType) (bool, error) {
	quota, err := s.GetQuota(ctx, tenantID)
	if err != nil {
		return false, err
	}

	// 检查是否需要重置周期性配额
	s.checkAndResetPeriodicalQuota(ctx, quota)

	var current, limit int64

	switch resourceType {
	case ResourceTypeUsers:
		current = int64(quota.UsedUsers)
		limit = int64(quota.MaxUsers)
	case ResourceTypeStorage:
		current = int64(quota.UsedStorageMB)
		limit = int64(quota.MaxStorageMB)
	case ResourceTypeWorkflows:
		current = int64(quota.UsedWorkflows)
		limit = int64(quota.MaxWorkflows)
	case ResourceTypeKnowledgeBases:
		current = int64(quota.UsedKnowledgeBases)
		limit = int64(quota.MaxKnowledgeBases)
	case ResourceTypeTokens:
		current = quota.UsedTokensThisMonth
		limit = quota.MaxTokensPerMonth
	case ResourceTypeAPICalls:
		current = int64(quota.UsedAPICallsToday)
		limit = int64(quota.MaxAPICallsPerDay)
	default:
		return false, ErrInvalidQuotaOperation
	}

	// -1 表示无限制
	if limit == -1 {
		return false, nil
	}

	return current >= limit, nil
}

// IncrementUsage 增加资源使用量
func (s *quotaService) IncrementUsage(ctx context.Context, tenantID string, resourceType ResourceType, delta int64) error {
	if delta <= 0 {
		return ErrInvalidQuotaOperation
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		// 使用悲观锁（FOR UPDATE）避免并发问题
		quota, err := s.quotaRepo.FindByTenantIDForUpdate(ctx, tx, tenantID)
		if err != nil {
			return fmt.Errorf("failed to lock quota: %w", err)
		}

		// 检查配额是否超限
		var current, limit int64
		switch resourceType {
		case ResourceTypeUsers:
			current = int64(quota.UsedUsers)
			limit = int64(quota.MaxUsers)
		case ResourceTypeStorage:
			current = int64(quota.UsedStorageMB)
			limit = int64(quota.MaxStorageMB)
		case ResourceTypeWorkflows:
			current = int64(quota.UsedWorkflows)
			limit = int64(quota.MaxWorkflows)
		case ResourceTypeKnowledgeBases:
			current = int64(quota.UsedKnowledgeBases)
			limit = int64(quota.MaxKnowledgeBases)
		case ResourceTypeTokens:
			current = quota.UsedTokensThisMonth
			limit = quota.MaxTokensPerMonth
		case ResourceTypeAPICalls:
			current = int64(quota.UsedAPICallsToday)
			limit = int64(quota.MaxAPICallsPerDay)
		default:
			return ErrInvalidQuotaOperation
		}

		// 检查是否超限（-1表示无限制）
		if limit != -1 && current+delta > limit {
			return ErrQuotaExceeded
		}

		// 更新使用量
		switch resourceType {
		case ResourceTypeUsers:
			quota.UsedUsers += int(delta)
		case ResourceTypeStorage:
			quota.UsedStorageMB += int(delta)
		case ResourceTypeWorkflows:
			quota.UsedWorkflows += int(delta)
		case ResourceTypeKnowledgeBases:
			quota.UsedKnowledgeBases += int(delta)
		case ResourceTypeTokens:
			quota.UsedTokensThisMonth += delta
		case ResourceTypeAPICalls:
			quota.UsedAPICallsToday += int(delta)
		}

		// 保存到数据库
		if err := tx.Save(quota).Error; err != nil {
			return fmt.Errorf("failed to update quota usage: %w", err)
		}

		return nil
	})
}

// DecrementUsage 减少资源使用量
func (s *quotaService) DecrementUsage(ctx context.Context, tenantID string, resourceType ResourceType, delta int64) error {
	if delta <= 0 {
		return ErrInvalidQuotaOperation
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		quota, err := s.quotaRepo.FindByTenantIDForUpdate(ctx, tx, tenantID)
		if err != nil {
			return fmt.Errorf("failed to lock quota: %w", err)
		}

		// 更新使用量（确保不会为负数）
		switch resourceType {
		case ResourceTypeUsers:
			quota.UsedUsers = max(0, quota.UsedUsers-int(delta))
		case ResourceTypeStorage:
			quota.UsedStorageMB = max(0, quota.UsedStorageMB-int(delta))
		case ResourceTypeWorkflows:
			quota.UsedWorkflows = max(0, quota.UsedWorkflows-int(delta))
		case ResourceTypeKnowledgeBases:
			quota.UsedKnowledgeBases = max(0, quota.UsedKnowledgeBases-int(delta))
		case ResourceTypeTokens:
			if quota.UsedTokensThisMonth > delta {
				quota.UsedTokensThisMonth -= delta
			} else {
				quota.UsedTokensThisMonth = 0
			}
		case ResourceTypeAPICalls:
			quota.UsedAPICallsToday = max(0, quota.UsedAPICallsToday-int(delta))
		default:
			return ErrInvalidQuotaOperation
		}

		// 保存到数据库
		if err := tx.Save(quota).Error; err != nil {
			return fmt.Errorf("failed to update quota usage: %w", err)
		}

		return nil
	})
}

// SetUsage 设置资源使用量
func (s *quotaService) SetUsage(ctx context.Context, tenantID string, resourceType ResourceType, value int64) error {
	if value < 0 {
		return ErrInvalidQuotaOperation
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		quota, err := s.quotaRepo.FindByTenantIDForUpdate(ctx, tx, tenantID)
		if err != nil {
			return fmt.Errorf("failed to lock quota: %w", err)
		}

		// 设置使用量
		switch resourceType {
		case ResourceTypeUsers:
			quota.UsedUsers = int(value)
		case ResourceTypeStorage:
			quota.UsedStorageMB = int(value)
		case ResourceTypeWorkflows:
			quota.UsedWorkflows = int(value)
		case ResourceTypeKnowledgeBases:
			quota.UsedKnowledgeBases = int(value)
		case ResourceTypeTokens:
			quota.UsedTokensThisMonth = value
		case ResourceTypeAPICalls:
			quota.UsedAPICallsToday = int(value)
		default:
			return ErrInvalidQuotaOperation
		}

		// 保存到数据库
		if err := tx.Save(quota).Error; err != nil {
			return fmt.Errorf("failed to set quota usage: %w", err)
		}

		return nil
	})
}

// GetUsageStats 获取资源使用统计
func (s *quotaService) GetUsageStats(ctx context.Context, tenantID string) ([]common.UsageStats, error) {
	quota, err := s.GetQuota(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	stats := []common.UsageStats{
		{
			ResourceType: string(ResourceTypeUsers),
			Used:         int64(quota.UsedUsers),
			Limit:        int64(quota.MaxUsers),
			UpdatedAt:    now,
		},
		{
			ResourceType: string(ResourceTypeStorage),
			Used:         int64(quota.UsedStorageMB),
			Limit:        int64(quota.MaxStorageMB),
			UpdatedAt:    now,
		},
		{
			ResourceType: string(ResourceTypeWorkflows),
			Used:         int64(quota.UsedWorkflows),
			Limit:        int64(quota.MaxWorkflows),
			UpdatedAt:    now,
		},
		{
			ResourceType: string(ResourceTypeKnowledgeBases),
			Used:         int64(quota.UsedKnowledgeBases),
			Limit:        int64(quota.MaxKnowledgeBases),
			UpdatedAt:    now,
		},
		{
			ResourceType: string(ResourceTypeTokens),
			Used:         quota.UsedTokensThisMonth,
			Limit:        quota.MaxTokensPerMonth,
			UpdatedAt:    now,
		},
		{
			ResourceType: string(ResourceTypeAPICalls),
			Used:         int64(quota.UsedAPICallsToday),
			Limit:        int64(quota.MaxAPICallsPerDay),
			UpdatedAt:    now,
		},
	}

	// 计算使用率
	for i := range stats {
		stats[i].CalculatePercentage()
	}

	return stats, nil
}

// ResetPeriodicalUsage 重置周期性配额
func (s *quotaService) ResetPeriodicalUsage(ctx context.Context, tenantID string) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		quota, err := s.quotaRepo.FindByTenantIDForUpdate(ctx, tx, tenantID)
		if err != nil {
			return fmt.Errorf("failed to lock quota: %w", err)
		}

		now := time.Now()

		// 重置月度Token配额
		if now.After(quota.TokenQuotaResetAt) {
			quota.UsedTokensThisMonth = 0
			quota.TokenQuotaResetAt = now.AddDate(0, 1, 0)
		}

		// 重置每日API调用配额
		if now.After(quota.APIQuotaResetAt) {
			quota.UsedAPICallsToday = 0
			quota.APIQuotaResetAt = now.AddDate(0, 0, 1)
		}

		// 保存到数据库
		if err := tx.Save(quota).Error; err != nil {
			return fmt.Errorf("failed to reset quota: %w", err)
		}

		return nil
	})
}

// checkAndResetPeriodicalQuota 检查并重置周期性配额
func (s *quotaService) checkAndResetPeriodicalQuota(ctx context.Context, quota *TenantQuota) {
	now := time.Now()
	needUpdate := false

	// 检查月度Token配额
	if now.After(quota.TokenQuotaResetAt) {
		quota.UsedTokensThisMonth = 0
		quota.TokenQuotaResetAt = now.AddDate(0, 1, 0)
		needUpdate = true
	}

	// 检查每日API调用配额
	if now.After(quota.APIQuotaResetAt) {
		quota.UsedAPICallsToday = 0
		quota.APIQuotaResetAt = now.AddDate(0, 0, 1)
		needUpdate = true
	}

	if needUpdate {
		// 异步更新（不阻塞当前操作）
		go func() {
			_ = s.quotaRepo.Update(context.Background(), quota)
		}()
	}
}

// IsQuotaAvailable 检查配额是否可用
func (s *quotaService) IsQuotaAvailable(ctx context.Context, tenantID string, resourceType ResourceType, required int64) (bool, error) {
	if required <= 0 {
		return false, ErrInvalidQuotaOperation
	}

	quota, err := s.GetQuota(ctx, tenantID)
	if err != nil {
		return false, err
	}

	var current, limit int64

	switch resourceType {
	case ResourceTypeUsers:
		current = int64(quota.UsedUsers)
		limit = int64(quota.MaxUsers)
	case ResourceTypeStorage:
		current = int64(quota.UsedStorageMB)
		limit = int64(quota.MaxStorageMB)
	case ResourceTypeWorkflows:
		current = int64(quota.UsedWorkflows)
		limit = int64(quota.MaxWorkflows)
	case ResourceTypeKnowledgeBases:
		current = int64(quota.UsedKnowledgeBases)
		limit = int64(quota.MaxKnowledgeBases)
	case ResourceTypeTokens:
		current = quota.UsedTokensThisMonth
		limit = quota.MaxTokensPerMonth
	case ResourceTypeAPICalls:
		current = int64(quota.UsedAPICallsToday)
		limit = int64(quota.MaxAPICallsPerDay)
	default:
		return false, ErrInvalidQuotaOperation
	}

	// -1 表示无限制
	if limit == -1 {
		return true, nil
	}

	return current+required <= limit, nil
}

// ReserveQuota 预留配额
func (s *quotaService) ReserveQuota(ctx context.Context, tenantID string, resourceType ResourceType, amount int64) error {
	// 预留配额实际上就是增加使用量
	return s.IncrementUsage(ctx, tenantID, resourceType, amount)
}

// ReleaseQuota 释放预留的配额
func (s *quotaService) ReleaseQuota(ctx context.Context, tenantID string, resourceType ResourceType, amount int64) error {
	// 释放配额实际上就是减少使用量
	return s.DecrementUsage(ctx, tenantID, resourceType, amount)
}

// max 返回两个int的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
