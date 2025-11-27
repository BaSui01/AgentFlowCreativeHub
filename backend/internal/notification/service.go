package notification

import (
	"context"
	"fmt"
	"time"

	"backend/internal/common"
	"backend/internal/tenant"

	"gorm.io/gorm"
)

// notificationConfigService 通知配置服务实现
type notificationConfigService struct {
	*common.BaseService
	db *gorm.DB
}

// NewNotificationConfigService 创建通知配置服务
func NewNotificationConfigService(db *gorm.DB) NotificationConfigServiceInterface {
	return &notificationConfigService{
		BaseService: common.NewBaseService(db),
		db:          db,
	}
}

// getUserIDFromContext 从context中获取当前用户ID
func (s *notificationConfigService) getUserIDFromContext(ctx context.Context) string {
	tc, ok := tenant.FromContext(ctx)
	if !ok {
		return "" // 未找到租户上下文
	}
	return tc.UserID
}

// ============ 用户偏好管理 ============

// GetUserPreferences 获取用户通知偏好（不存在时返回默认配置）
func (s *notificationConfigService) GetUserPreferences(ctx context.Context, tenantID uint, userID string) (*NotificationPreferences, error) {
	var prefs NotificationPreferences
	
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&prefs).Error
	
	if err == gorm.ErrRecordNotFound {
		// 返回默认配置
		return s.getDefaultPreferences(tenantID, userID), nil
	}
	
	if err != nil {
		return nil, fmt.Errorf("查询用户偏好失败: %w", err)
	}
	
	return &prefs, nil
}

// UpdateUserPreferences 更新用户通知偏好
func (s *notificationConfigService) UpdateUserPreferences(ctx context.Context, tenantID uint, userID string, prefs *NotificationPreferences) error {
	// 设置租户和用户信息
	prefs.TenantID = tenantID
	prefs.UserID = userID
	prefs.UpdatedBy = s.getUserIDFromContext(ctx)
	prefs.UpdatedAt = time.Now()
	
	// 检查是否存在
	var existing NotificationPreferences
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&existing).Error
	
	if err == gorm.ErrRecordNotFound {
		// 不存在，创建新记录
		prefs.CreatedBy = prefs.UpdatedBy
		prefs.CreatedAt = time.Now()
		return s.db.WithContext(ctx).Create(prefs).Error
	}
	
	if err != nil {
		return fmt.Errorf("查询用户偏好失败: %w", err)
	}
	
	// 存在，更新记录
	prefs.ID = existing.ID
	prefs.CreatedAt = existing.CreatedAt
	prefs.CreatedBy = existing.CreatedBy
	
	return s.db.WithContext(ctx).Save(prefs).Error
}

// ResetUserPreferences 重置用户偏好为默认值
func (s *notificationConfigService) ResetUserPreferences(ctx context.Context, tenantID uint, userID string) error {
	defaultPrefs := s.getDefaultPreferences(tenantID, userID)
	defaultPrefs.UpdatedBy = s.getUserIDFromContext(ctx)
	
	return s.UpdateUserPreferences(ctx, tenantID, userID, defaultPrefs)
}

// getDefaultPreferences 获取默认偏好配置
func (s *notificationConfigService) getDefaultPreferences(tenantID uint, userID string) *NotificationPreferences {
	return &NotificationPreferences{
		TenantID:        tenantID,
		UserID:          userID,
		EnabledChannels: []NotificationChannel{ChannelWebSocket, ChannelEmail},
		CategoryPreferences: CategoryPreferences{
			CategoryApproval: true,
			CategoryWorkflow: true,
			CategorySystem:   true,
			CategorySecurity: true,
			CategoryAgent:    true,
			CategoryQuota:    true,
		},
		EmailFrequency:    FrequencyRealtime,
		DigestEnabled:     false,
		QuietHoursEnabled: false,
		Timezone:          "UTC",
		Language:          "zh-CN",
		FilterRules:       NotificationFilters{OnlyRelevant: false},
	}
}

// ============ 渠道管理 ============

// EnableChannel 启用指定通知渠道
func (s *notificationConfigService) EnableChannel(ctx context.Context, tenantID uint, userID string, channel NotificationChannel) error {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	
	// 检查是否已启用
	for _, ch := range prefs.EnabledChannels {
		if ch == channel {
			return nil // 已启用，无需重复添加
		}
	}
	
	// 添加渠道
	prefs.EnabledChannels = append(prefs.EnabledChannels, channel)
	
	return s.UpdateUserPreferences(ctx, tenantID, userID, prefs)
}

// DisableChannel 禁用指定通知渠道
func (s *notificationConfigService) DisableChannel(ctx context.Context, tenantID uint, userID string, channel NotificationChannel) error {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	
	// 移除渠道
	newChannels := make([]NotificationChannel, 0)
	for _, ch := range prefs.EnabledChannels {
		if ch != channel {
			newChannels = append(newChannels, ch)
		}
	}
	
	prefs.EnabledChannels = newChannels
	
	return s.UpdateUserPreferences(ctx, tenantID, userID, prefs)
}

// IsChannelEnabled 检查渠道是否启用
func (s *notificationConfigService) IsChannelEnabled(ctx context.Context, tenantID uint, userID string, channel NotificationChannel) (bool, error) {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return false, err
	}
	
	for _, ch := range prefs.EnabledChannels {
		if ch == channel {
			return true, nil
		}
	}
	
	return false, nil
}

// GetEnabledChannels 获取用户启用的所有渠道
func (s *notificationConfigService) GetEnabledChannels(ctx context.Context, tenantID uint, userID string) ([]NotificationChannel, error) {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	
	return prefs.EnabledChannels, nil
}

// ============ 分类订阅管理 ============

// SubscribeCategory 订阅指定类别的通知
func (s *notificationConfigService) SubscribeCategory(ctx context.Context, tenantID uint, userID string, category NotificationCategory) error {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	
	if prefs.CategoryPreferences == nil {
		prefs.CategoryPreferences = make(CategoryPreferences)
	}
	
	prefs.CategoryPreferences[category] = true
	
	return s.UpdateUserPreferences(ctx, tenantID, userID, prefs)
}

// UnsubscribeCategory 取消订阅指定类别的通知
func (s *notificationConfigService) UnsubscribeCategory(ctx context.Context, tenantID uint, userID string, category NotificationCategory) error {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	
	if prefs.CategoryPreferences == nil {
		prefs.CategoryPreferences = make(CategoryPreferences)
	}
	
	prefs.CategoryPreferences[category] = false
	
	return s.UpdateUserPreferences(ctx, tenantID, userID, prefs)
}

// IsCategorySubscribed 检查是否订阅了指定类别
func (s *notificationConfigService) IsCategorySubscribed(ctx context.Context, tenantID uint, userID string, category NotificationCategory) (bool, error) {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return false, err
	}
	
	if prefs.CategoryPreferences == nil {
		return true, nil // 默认订阅所有类别
	}
	
	subscribed, exists := prefs.CategoryPreferences[category]
	if !exists {
		return true, nil // 未设置，默认订阅
	}
	
	return subscribed, nil
}

// ============ 静音时段管理 ============

// SetQuietHours 设置静音时段
func (s *notificationConfigService) SetQuietHours(ctx context.Context, tenantID uint, userID string, start, end string, enabled bool) error {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	
	prefs.QuietHoursStart = &start
	prefs.QuietHoursEnd = &end
	prefs.QuietHoursEnabled = enabled
	
	return s.UpdateUserPreferences(ctx, tenantID, userID, prefs)
}

// IsInQuietHours 检查当前是否在静音时段
func (s *notificationConfigService) IsInQuietHours(ctx context.Context, tenantID uint, userID string) (bool, error) {
	prefs, err := s.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		return false, err
	}
	
	// 未启用静音时段
	if !prefs.QuietHoursEnabled || prefs.QuietHoursStart == nil || prefs.QuietHoursEnd == nil {
		return false, nil
	}
	
	// 解析用户时区的当前时间
	loc, err := time.LoadLocation(prefs.Timezone)
	if err != nil {
		loc = time.UTC
	}
	
	now := time.Now().In(loc)
	currentTime := now.Format("15:04")
	
	start := *prefs.QuietHoursStart
	end := *prefs.QuietHoursEnd
	
	// 处理跨天情况（如 22:00-08:00）
	if start < end {
		// 同一天内（如 08:00-22:00）
		return currentTime >= start && currentTime <= end, nil
	} else {
		// 跨天（如 22:00-08:00）
		return currentTime >= start || currentTime <= end, nil
	}
}

// ============ 租户配置管理 ============

// GetTenantConfig 获取租户通知配置
func (s *notificationConfigService) GetTenantConfig(ctx context.Context, tenantID uint) (*TenantNotificationConfig, error) {
	var config TenantNotificationConfig
	
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&config).Error
	
	if err == gorm.ErrRecordNotFound {
		// 返回默认配置
		return s.getDefaultTenantConfig(tenantID), nil
	}
	
	if err != nil {
		return nil, fmt.Errorf("查询租户配置失败: %w", err)
	}
	
	return &config, nil
}

// UpdateTenantConfig 更新租户通知配置
func (s *notificationConfigService) UpdateTenantConfig(ctx context.Context, tenantID uint, config *TenantNotificationConfig) error {
	config.TenantID = tenantID
	config.UpdatedAt = time.Now()
	
	// 检查是否存在
	var existing TenantNotificationConfig
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&existing).Error
	
	if err == gorm.ErrRecordNotFound {
		// 不存在，创建新记录
		config.CreatedAt = time.Now()
		return s.db.WithContext(ctx).Create(config).Error
	}
	
	if err != nil {
		return fmt.Errorf("查询租户配置失败: %w", err)
	}
	
	// 存在，更新记录
	config.ID = existing.ID
	config.CreatedAt = existing.CreatedAt
	
	return s.db.WithContext(ctx).Save(config).Error
}

// GetAvailableChannels 获取租户可用的通知渠道
func (s *notificationConfigService) GetAvailableChannels(ctx context.Context, tenantID uint) ([]NotificationChannel, error) {
	config, err := s.GetTenantConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	
	return config.AvailableChannels, nil
}

// getDefaultTenantConfig 获取默认租户配置
func (s *notificationConfigService) getDefaultTenantConfig(tenantID uint) *TenantNotificationConfig {
	return &TenantNotificationConfig{
		TenantID:                 tenantID,
		AvailableChannels:        []NotificationChannel{ChannelWebSocket, ChannelEmail, ChannelWebhook},
		RateLimitEnabled:         true,
		MaxNotificationsPerHour:  100,
		RetentionDays:            30,
		UseTLS:                   true,
	}
}

// ============ 通知历史 ============

// RecordNotification 记录通知历史
func (s *notificationConfigService) RecordNotification(ctx context.Context, history *NotificationHistory) error {
	history.CreatedAt = time.Now()
	history.UpdatedAt = time.Now()
	
	if history.Status == "" {
		history.Status = "pending"
	}
	
	return s.db.WithContext(ctx).Create(history).Error
}

// GetNotificationHistory 获取用户通知历史
func (s *notificationConfigService) GetNotificationHistory(ctx context.Context, tenantID uint, userID string, limit int) ([]*NotificationHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	
	var history []*NotificationHistory
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&history).Error
	
	if err != nil {
		return nil, fmt.Errorf("查询通知历史失败: %w", err)
	}
	
	return history, nil
}

// GetFailedNotifications 获取发送失败的通知（用于重试）
func (s *notificationConfigService) GetFailedNotifications(ctx context.Context, tenantID uint, maxAttempts int) ([]*NotificationHistory, error) {
	var failed []*NotificationHistory
	
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ? AND attempts < ?", tenantID, "failed", maxAttempts).
		Order("created_at ASC").
		Limit(100).
		Find(&failed).Error
	
	if err != nil {
		return nil, fmt.Errorf("查询失败通知失败: %w", err)
	}
	
	return failed, nil
}

// UpdateNotificationStatus 更新通知状态
func (s *notificationConfigService) UpdateNotificationStatus(ctx context.Context, notificationID uint, status string, failReason string) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}
	
	if status == "sent" {
		now := time.Now()
		updates["sent_at"] = &now
	}
	
	if failReason != "" {
		updates["failed_reason"] = failReason
		updates["attempts"] = gorm.Expr("attempts + 1")
	}
	
	return s.db.WithContext(ctx).
		Model(&NotificationHistory{}).
		Where("id = ?", notificationID).
		Updates(updates).Error
}

// ============ 批量操作 ============

// BatchUpdatePreferences 批量更新多个用户的偏好
func (s *notificationConfigService) BatchUpdatePreferences(ctx context.Context, tenantID uint, userIDs []string, updates map[string]any) error {
	if len(userIDs) == 0 {
		return nil
	}
	
	updates["updated_at"] = time.Now()
	updates["updated_by"] = s.getUserIDFromContext(ctx)
	
	return s.db.WithContext(ctx).
		Model(&NotificationPreferences{}).
		Where("tenant_id = ? AND user_id IN ?", tenantID, userIDs).
		Updates(updates).Error
}

// GetUsersByChannel 获取启用了指定渠道的所有用户
func (s *notificationConfigService) GetUsersByChannel(ctx context.Context, tenantID uint, channel NotificationChannel) ([]string, error) {
	var userIDs []string
	
	err := s.db.WithContext(ctx).
		Model(&NotificationPreferences{}).
		Where("tenant_id = ? AND enabled_channels @> ?", tenantID, fmt.Sprintf("[\"%s\"]", channel)).
		Pluck("user_id", &userIDs).Error
	
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	
	return userIDs, nil
}

// GetUsersByCategory 获取订阅了指定类别的所有用户
func (s *notificationConfigService) GetUsersByCategory(ctx context.Context, tenantID uint, category NotificationCategory) ([]string, error) {
	var userIDs []string
	
	// 查询订阅了该类别的用户 + 未设置偏好的用户（默认订阅所有）
	err := s.db.WithContext(ctx).
		Model(&NotificationPreferences{}).
		Where("tenant_id = ? AND (category_preferences->>? = 'true' OR category_preferences->>? IS NULL)", 
			tenantID, string(category), string(category)).
		Pluck("user_id", &userIDs).Error
	
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	
	return userIDs, nil
}

// ============ 统计分析 ============

// GetNotificationStats 获取通知统计（发送量、成功率等）
func (s *notificationConfigService) GetNotificationStats(ctx context.Context, tenantID uint, startTime, endTime string) (map[string]any, error) {
	stats := make(map[string]any)
	
	// 总发送量
	var total int64
	s.db.WithContext(ctx).
		Model(&NotificationHistory{}).
		Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startTime, endTime).
		Count(&total)
	stats["total"] = total
	
	// 成功数量
	var success int64
	s.db.WithContext(ctx).
		Model(&NotificationHistory{}).
		Where("tenant_id = ? AND status = ? AND created_at BETWEEN ? AND ?", tenantID, "sent", startTime, endTime).
		Count(&success)
	stats["success"] = success
	
	// 失败数量
	var failed int64
	s.db.WithContext(ctx).
		Model(&NotificationHistory{}).
		Where("tenant_id = ? AND status = ? AND created_at BETWEEN ? AND ?", tenantID, "failed", startTime, endTime).
		Count(&failed)
	stats["failed"] = failed
	
	// 成功率
	if total > 0 {
		stats["success_rate"] = float64(success) / float64(total) * 100
	} else {
		stats["success_rate"] = 0.0
	}
	
	// 按渠道统计
	var channelStats []struct {
		Channel NotificationChannel `json:"channel"`
		Count   int64               `json:"count"`
	}
	s.db.WithContext(ctx).
		Model(&NotificationHistory{}).
		Select("channel, COUNT(*) as count").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startTime, endTime).
		Group("channel").
		Find(&channelStats)
	stats["by_channel"] = channelStats
	
	// 按类别统计
	var categoryStats []struct {
		Category NotificationCategory `json:"category"`
		Count    int64                `json:"count"`
	}
	s.db.WithContext(ctx).
		Model(&NotificationHistory{}).
		Select("category, COUNT(*) as count").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startTime, endTime).
		Group("category").
		Find(&categoryStats)
	stats["by_category"] = categoryStats
	
	return stats, nil
}

// CleanupOldNotifications 清理过期的通知历史
func (s *notificationConfigService) CleanupOldNotifications(ctx context.Context, tenantID uint, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30 // 默认保留30天
	}
	
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
	
	result := s.db.WithContext(ctx).
		Where("tenant_id = ? AND created_at < ?", tenantID, cutoffDate).
		Delete(&NotificationHistory{})
	
	if result.Error != nil {
		return 0, fmt.Errorf("清理通知历史失败: %w", result.Error)
	}
	
	return result.RowsAffected, nil
}
