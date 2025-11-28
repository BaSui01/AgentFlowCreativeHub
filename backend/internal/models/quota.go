package models

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrModelQuotaExceeded  = errors.New("模型配额已用尽")
	ErrModelQuotaNotFound  = errors.New("模型配额不存在")
	ErrInsufficientBalance = errors.New("余额不足")
)

// ModelQuota 模型配额
type ModelQuota struct {
	ID        string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string `json:"tenantId" gorm:"type:uuid;not null;index"`
	ModelID   string `json:"modelId" gorm:"type:uuid;index"`          // 空表示租户级别配额
	
	// 配额限制
	MaxTokensPerDay    int64 `json:"maxTokensPerDay" gorm:"default:-1"`     // -1 表示无限制
	MaxTokensPerMonth  int64 `json:"maxTokensPerMonth" gorm:"default:-1"`
	MaxCallsPerDay     int64 `json:"maxCallsPerDay" gorm:"default:-1"`
	MaxCallsPerMonth   int64 `json:"maxCallsPerMonth" gorm:"default:-1"`
	MaxCostPerDay      float64 `json:"maxCostPerDay" gorm:"type:decimal(10,4);default:-1"`
	MaxCostPerMonth    float64 `json:"maxCostPerMonth" gorm:"type:decimal(10,4);default:-1"`
	
	// 当前使用量
	UsedTokensToday    int64   `json:"usedTokensToday" gorm:"default:0"`
	UsedTokensThisMonth int64  `json:"usedTokensThisMonth" gorm:"default:0"`
	UsedCallsToday     int64   `json:"usedCallsToday" gorm:"default:0"`
	UsedCallsThisMonth int64   `json:"usedCallsThisMonth" gorm:"default:0"`
	UsedCostToday      float64 `json:"usedCostToday" gorm:"type:decimal(10,4);default:0"`
	UsedCostThisMonth  float64 `json:"usedCostThisMonth" gorm:"type:decimal(10,4);default:0"`
	
	// 重置时间
	DayResetAt   time.Time `json:"dayResetAt" gorm:"not null"`
	MonthResetAt time.Time `json:"monthResetAt" gorm:"not null"`
	
	// 预警阈值（百分比）
	WarningThreshold int `json:"warningThreshold" gorm:"default:80"` // 默认 80%
	
	// 状态
	IsActive  bool      `json:"isActive" gorm:"default:true"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ModelQuota) TableName() string {
	return "model_quotas"
}

// ModelQuotaService 模型配额服务
type ModelQuotaService struct {
	db *gorm.DB
	mu sync.RWMutex
}

// NewModelQuotaService 创建模型配额服务
func NewModelQuotaService(db *gorm.DB) *ModelQuotaService {
	return &ModelQuotaService{db: db}
}

// AutoMigrate 自动迁移表结构
func (s *ModelQuotaService) AutoMigrate() error {
	return s.db.AutoMigrate(&ModelQuota{})
}

// CreateQuota 创建模型配额
func (s *ModelQuotaService) CreateQuota(ctx context.Context, tenantID, modelID string, limits *QuotaLimits) (*ModelQuota, error) {
	now := time.Now()
	quota := &ModelQuota{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		ModelID:          modelID,
		MaxTokensPerDay:  limits.MaxTokensPerDay,
		MaxTokensPerMonth: limits.MaxTokensPerMonth,
		MaxCallsPerDay:   limits.MaxCallsPerDay,
		MaxCallsPerMonth: limits.MaxCallsPerMonth,
		MaxCostPerDay:    limits.MaxCostPerDay,
		MaxCostPerMonth:  limits.MaxCostPerMonth,
		WarningThreshold: 80,
		DayResetAt:       now.Add(24 * time.Hour).Truncate(24 * time.Hour),
		MonthResetAt:     time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()),
		IsActive:         true,
	}
	
	if err := s.db.WithContext(ctx).Create(quota).Error; err != nil {
		return nil, err
	}
	return quota, nil
}

// QuotaLimits 配额限制
type QuotaLimits struct {
	MaxTokensPerDay   int64   `json:"maxTokensPerDay"`
	MaxTokensPerMonth int64   `json:"maxTokensPerMonth"`
	MaxCallsPerDay    int64   `json:"maxCallsPerDay"`
	MaxCallsPerMonth  int64   `json:"maxCallsPerMonth"`
	MaxCostPerDay     float64 `json:"maxCostPerDay"`
	MaxCostPerMonth   float64 `json:"maxCostPerMonth"`
}

// GetQuota 获取配额
func (s *ModelQuotaService) GetQuota(ctx context.Context, tenantID, modelID string) (*ModelQuota, error) {
	var quota ModelQuota
	
	// 先查找模型级别配额
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND model_id = ?", tenantID, modelID).
		First(&quota).Error
	
	if err == nil {
		s.checkAndResetQuota(&quota)
		return &quota, nil
	}
	
	// 如果没有模型级别配额，查找租户级别配额
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = s.db.WithContext(ctx).
			Where("tenant_id = ? AND (model_id = '' OR model_id IS NULL)", tenantID).
			First(&quota).Error
		if err == nil {
			s.checkAndResetQuota(&quota)
			return &quota, nil
		}
	}
	
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelQuotaNotFound
	}
	return nil, err
}

// CheckQuota 检查配额是否充足
func (s *ModelQuotaService) CheckQuota(ctx context.Context, tenantID, modelID string, tokens int, cost float64) error {
	quota, err := s.GetQuota(ctx, tenantID, modelID)
	if err != nil {
		if errors.Is(err, ErrModelQuotaNotFound) {
			return nil // 没有配额限制
		}
		return err
	}
	
	if !quota.IsActive {
		return ErrModelQuotaExceeded
	}
	
	// 检查 Token 限制
	if quota.MaxTokensPerDay > 0 && quota.UsedTokensToday+int64(tokens) > quota.MaxTokensPerDay {
		return fmt.Errorf("%w: 每日 Token 限额已达", ErrModelQuotaExceeded)
	}
	if quota.MaxTokensPerMonth > 0 && quota.UsedTokensThisMonth+int64(tokens) > quota.MaxTokensPerMonth {
		return fmt.Errorf("%w: 每月 Token 限额已达", ErrModelQuotaExceeded)
	}
	
	// 检查调用次数限制
	if quota.MaxCallsPerDay > 0 && quota.UsedCallsToday+1 > quota.MaxCallsPerDay {
		return fmt.Errorf("%w: 每日调用次数已达上限", ErrModelQuotaExceeded)
	}
	if quota.MaxCallsPerMonth > 0 && quota.UsedCallsThisMonth+1 > quota.MaxCallsPerMonth {
		return fmt.Errorf("%w: 每月调用次数已达上限", ErrModelQuotaExceeded)
	}
	
	// 检查成本限制
	if quota.MaxCostPerDay > 0 && quota.UsedCostToday+cost > quota.MaxCostPerDay {
		return fmt.Errorf("%w: 每日成本限额已达", ErrModelQuotaExceeded)
	}
	if quota.MaxCostPerMonth > 0 && quota.UsedCostThisMonth+cost > quota.MaxCostPerMonth {
		return fmt.Errorf("%w: 每月成本限额已达", ErrModelQuotaExceeded)
	}
	
	return nil
}

// ConsumeQuota 消耗配额
func (s *ModelQuotaService) ConsumeQuota(ctx context.Context, tenantID, modelID string, tokens int, cost float64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var quota ModelQuota
		
		// 先尝试模型级别配额
		err := tx.Where("tenant_id = ? AND model_id = ?", tenantID, modelID).
			First(&quota).Error
		
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 尝试租户级别配额
			err = tx.Where("tenant_id = ? AND (model_id = '' OR model_id IS NULL)", tenantID).
				First(&quota).Error
		}
		
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 没有配额记录，不做限制
			}
			return err
		}
		
		// 检查并重置
		s.checkAndResetQuota(&quota)
		
		// 更新使用量
		quota.UsedTokensToday += int64(tokens)
		quota.UsedTokensThisMonth += int64(tokens)
		quota.UsedCallsToday++
		quota.UsedCallsThisMonth++
		quota.UsedCostToday += cost
		quota.UsedCostThisMonth += cost
		
		return tx.Save(&quota).Error
	})
}

// GetUsageStats 获取使用统计
func (s *ModelQuotaService) GetUsageStats(ctx context.Context, tenantID string) ([]QuotaUsageStats, error) {
	var quotas []ModelQuota
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&quotas).Error; err != nil {
		return nil, err
	}
	
	var stats []QuotaUsageStats
	for _, q := range quotas {
		s.checkAndResetQuota(&q)
		
		stat := QuotaUsageStats{
			ModelID:           q.ModelID,
			TokensUsedToday:   q.UsedTokensToday,
			TokensLimitToday:  q.MaxTokensPerDay,
			TokensUsedMonth:   q.UsedTokensThisMonth,
			TokensLimitMonth:  q.MaxTokensPerMonth,
			CallsUsedToday:    q.UsedCallsToday,
			CallsLimitToday:   q.MaxCallsPerDay,
			CallsUsedMonth:    q.UsedCallsThisMonth,
			CallsLimitMonth:   q.MaxCallsPerMonth,
			CostUsedToday:     q.UsedCostToday,
			CostLimitToday:    q.MaxCostPerDay,
			CostUsedMonth:     q.UsedCostThisMonth,
			CostLimitMonth:    q.MaxCostPerMonth,
		}
		
		// 计算使用百分比
		if q.MaxTokensPerMonth > 0 {
			stat.TokensUsagePercent = float64(q.UsedTokensThisMonth) / float64(q.MaxTokensPerMonth) * 100
		}
		if q.MaxCostPerMonth > 0 {
			stat.CostUsagePercent = q.UsedCostThisMonth / q.MaxCostPerMonth * 100
		}
		
		// 检查是否达到预警
		stat.IsWarning = stat.TokensUsagePercent >= float64(q.WarningThreshold) ||
			stat.CostUsagePercent >= float64(q.WarningThreshold)
		
		stats = append(stats, stat)
	}
	
	return stats, nil
}

// QuotaUsageStats 配额使用统计
type QuotaUsageStats struct {
	ModelID            string  `json:"modelId"`
	TokensUsedToday    int64   `json:"tokensUsedToday"`
	TokensLimitToday   int64   `json:"tokensLimitToday"`
	TokensUsedMonth    int64   `json:"tokensUsedMonth"`
	TokensLimitMonth   int64   `json:"tokensLimitMonth"`
	TokensUsagePercent float64 `json:"tokensUsagePercent"`
	CallsUsedToday     int64   `json:"callsUsedToday"`
	CallsLimitToday    int64   `json:"callsLimitToday"`
	CallsUsedMonth     int64   `json:"callsUsedMonth"`
	CallsLimitMonth    int64   `json:"callsLimitMonth"`
	CostUsedToday      float64 `json:"costUsedToday"`
	CostLimitToday     float64 `json:"costLimitToday"`
	CostUsedMonth      float64 `json:"costUsedMonth"`
	CostLimitMonth     float64 `json:"costLimitMonth"`
	CostUsagePercent   float64 `json:"costUsagePercent"`
	IsWarning          bool    `json:"isWarning"`
}

// UpdateQuotaLimits 更新配额限制
func (s *ModelQuotaService) UpdateQuotaLimits(ctx context.Context, tenantID, modelID string, limits *QuotaLimits) error {
	updates := map[string]interface{}{}
	
	if limits.MaxTokensPerDay >= 0 {
		updates["max_tokens_per_day"] = limits.MaxTokensPerDay
	}
	if limits.MaxTokensPerMonth >= 0 {
		updates["max_tokens_per_month"] = limits.MaxTokensPerMonth
	}
	if limits.MaxCallsPerDay >= 0 {
		updates["max_calls_per_day"] = limits.MaxCallsPerDay
	}
	if limits.MaxCallsPerMonth >= 0 {
		updates["max_calls_per_month"] = limits.MaxCallsPerMonth
	}
	if limits.MaxCostPerDay >= 0 {
		updates["max_cost_per_day"] = limits.MaxCostPerDay
	}
	if limits.MaxCostPerMonth >= 0 {
		updates["max_cost_per_month"] = limits.MaxCostPerMonth
	}
	
	return s.db.WithContext(ctx).Model(&ModelQuota{}).
		Where("tenant_id = ? AND model_id = ?", tenantID, modelID).
		Updates(updates).Error
}

// checkAndResetQuota 检查并重置配额
func (s *ModelQuotaService) checkAndResetQuota(quota *ModelQuota) {
	now := time.Now()
	needUpdate := false
	
	// 检查日重置
	if now.After(quota.DayResetAt) {
		quota.UsedTokensToday = 0
		quota.UsedCallsToday = 0
		quota.UsedCostToday = 0
		quota.DayResetAt = now.Add(24 * time.Hour).Truncate(24 * time.Hour)
		needUpdate = true
	}
	
	// 检查月重置
	if now.After(quota.MonthResetAt) {
		quota.UsedTokensThisMonth = 0
		quota.UsedCallsThisMonth = 0
		quota.UsedCostThisMonth = 0
		quota.MonthResetAt = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
		needUpdate = true
	}
	
	if needUpdate {
		s.db.Save(quota)
	}
}

// ListQuotas 获取配额列表
func (s *ModelQuotaService) ListQuotas(ctx context.Context, tenantID, modelID string) ([]*ModelQuota, error) {
	query := s.db.WithContext(ctx).Model(&ModelQuota{}).Where("tenant_id = ?", tenantID)
	if modelID != "" {
		query = query.Where("model_id = ?", modelID)
	}

	var quotas []*ModelQuota
	if err := query.Order("created_at DESC").Find(&quotas).Error; err != nil {
		return nil, fmt.Errorf("查询配额列表失败: %w", err)
	}

	return quotas, nil
}

// GetQuotaByID 根据ID获取配额
func (s *ModelQuotaService) GetQuotaByID(ctx context.Context, tenantID, quotaID string) (*ModelQuota, error) {
	var quota ModelQuota
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", quotaID, tenantID).
		First(&quota).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelQuotaNotFound
		}
		return nil, fmt.Errorf("查询配额失败: %w", err)
	}
	return &quota, nil
}

// UpdateQuota 更新配额
func (s *ModelQuotaService) UpdateQuota(ctx context.Context, tenantID, quotaID string, limits *QuotaLimits) (*ModelQuota, error) {
	var quota ModelQuota
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", quotaID, tenantID).
		First(&quota).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelQuotaNotFound
		}
		return nil, fmt.Errorf("查询配额失败: %w", err)
	}

	updates := map[string]interface{}{
		"updated_at": time.Now().UTC(),
	}

	if limits.MaxTokensPerDay >= 0 {
		updates["max_tokens_per_day"] = limits.MaxTokensPerDay
	}
	if limits.MaxTokensPerMonth >= 0 {
		updates["max_tokens_per_month"] = limits.MaxTokensPerMonth
	}
	if limits.MaxCallsPerDay >= 0 {
		updates["max_calls_per_day"] = limits.MaxCallsPerDay
	}
	if limits.MaxCallsPerMonth >= 0 {
		updates["max_calls_per_month"] = limits.MaxCallsPerMonth
	}
	if limits.MaxCostPerDay >= 0 {
		updates["max_cost_per_day"] = limits.MaxCostPerDay
	}
	if limits.MaxCostPerMonth >= 0 {
		updates["max_cost_per_month"] = limits.MaxCostPerMonth
	}

	if err := s.db.WithContext(ctx).Model(&quota).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新配额失败: %w", err)
	}

	// 重新查询
	if err := s.db.WithContext(ctx).First(&quota, "id = ?", quotaID).Error; err != nil {
		return nil, err
	}

	return &quota, nil
}

// DeleteQuota 删除配额
func (s *ModelQuotaService) DeleteQuota(ctx context.Context, tenantID, quotaID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", quotaID, tenantID).
		Delete(&ModelQuota{})

	if result.Error != nil {
		return fmt.Errorf("删除配额失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrModelQuotaNotFound
	}

	return nil
}

// EstimateCost 预估成本
func (s *ModelQuotaService) EstimateCost(ctx context.Context, modelID string, promptTokens, completionTokens int) (float64, error) {
	var model Model
	if err := s.db.WithContext(ctx).Where("id = ?", modelID).First(&model).Error; err != nil {
		return 0, err
	}
	
	promptCost := float64(promptTokens) / 1000 * model.InputCostPer1K
	completionCost := float64(completionTokens) / 1000 * model.OutputCostPer1K
	
	return promptCost + completionCost, nil
}
