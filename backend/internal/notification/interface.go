package notification

import (
	"context"
)

// NotificationConfigServiceInterface 通知配置服务接口
type NotificationConfigServiceInterface interface {
	// ============ 用户偏好管理 ============
	
	// GetUserPreferences 获取用户通知偏好（不存在时返回默认配置）
	GetUserPreferences(ctx context.Context, tenantID uint, userID string) (*NotificationPreferences, error)
	
	// UpdateUserPreferences 更新用户通知偏好
	UpdateUserPreferences(ctx context.Context, tenantID uint, userID string, prefs *NotificationPreferences) error
	
	// ResetUserPreferences 重置用户偏好为默认值
	ResetUserPreferences(ctx context.Context, tenantID uint, userID string) error
	
	// ============ 渠道管理 ============
	
	// EnableChannel 启用指定通知渠道
	EnableChannel(ctx context.Context, tenantID uint, userID string, channel NotificationChannel) error
	
	// DisableChannel 禁用指定通知渠道
	DisableChannel(ctx context.Context, tenantID uint, userID string, channel NotificationChannel) error
	
	// IsChannelEnabled 检查渠道是否启用
	IsChannelEnabled(ctx context.Context, tenantID uint, userID string, channel NotificationChannel) (bool, error)
	
	// GetEnabledChannels 获取用户启用的所有渠道
	GetEnabledChannels(ctx context.Context, tenantID uint, userID string) ([]NotificationChannel, error)
	
	// ============ 分类订阅管理 ============
	
	// SubscribeCategory 订阅指定类别的通知
	SubscribeCategory(ctx context.Context, tenantID uint, userID string, category NotificationCategory) error
	
	// UnsubscribeCategory 取消订阅指定类别的通知
	UnsubscribeCategory(ctx context.Context, tenantID uint, userID string, category NotificationCategory) error
	
	// IsCategorySubscribed 检查是否订阅了指定类别
	IsCategorySubscribed(ctx context.Context, tenantID uint, userID string, category NotificationCategory) (bool, error)
	
	// ============ 静音时段管理 ============
	
	// SetQuietHours 设置静音时段
	SetQuietHours(ctx context.Context, tenantID uint, userID string, start, end string, enabled bool) error
	
	// IsInQuietHours 检查当前是否在静音时段
	IsInQuietHours(ctx context.Context, tenantID uint, userID string) (bool, error)
	
	// ============ 租户配置管理 ============
	
	// GetTenantConfig 获取租户通知配置
	GetTenantConfig(ctx context.Context, tenantID uint) (*TenantNotificationConfig, error)
	
	// UpdateTenantConfig 更新租户通知配置
	UpdateTenantConfig(ctx context.Context, tenantID uint, config *TenantNotificationConfig) error
	
	// GetAvailableChannels 获取租户可用的通知渠道
	GetAvailableChannels(ctx context.Context, tenantID uint) ([]NotificationChannel, error)
	
	// ============ 通知历史 ============
	
	// RecordNotification 记录通知历史
	RecordNotification(ctx context.Context, history *NotificationHistory) error
	
	// GetNotificationHistory 获取用户通知历史
	GetNotificationHistory(ctx context.Context, tenantID uint, userID string, limit int) ([]*NotificationHistory, error)
	
	// GetFailedNotifications 获取发送失败的通知（用于重试）
	GetFailedNotifications(ctx context.Context, tenantID uint, maxAttempts int) ([]*NotificationHistory, error)
	
	// UpdateNotificationStatus 更新通知状态
	UpdateNotificationStatus(ctx context.Context, notificationID uint, status string, failReason string) error
	
	// ============ 批量操作 ============
	
	// BatchUpdatePreferences 批量更新多个用户的偏好
	BatchUpdatePreferences(ctx context.Context, tenantID uint, userIDs []string, updates map[string]any) error
	
	// GetUsersByChannel 获取启用了指定渠道的所有用户
	GetUsersByChannel(ctx context.Context, tenantID uint, channel NotificationChannel) ([]string, error)
	
	// GetUsersByCategory 获取订阅了指定类别的所有用户
	GetUsersByCategory(ctx context.Context, tenantID uint, category NotificationCategory) ([]string, error)
	
	// ============ 统计分析 ============
	
	// GetNotificationStats 获取通知统计（发送量、成功率等）
	GetNotificationStats(ctx context.Context, tenantID uint, startTime, endTime string) (map[string]any, error)
	
	// CleanupOldNotifications 清理过期的通知历史
	CleanupOldNotifications(ctx context.Context, tenantID uint, retentionDays int) (int64, error)
}

// NotificationConfigRepositoryInterface 通知配置Repository接口（可选）
type NotificationConfigRepositoryInterface interface {
	// FindPreferencesByUser 查询用户偏好
	FindPreferencesByUser(ctx context.Context, tenantID uint, userID string) (*NotificationPreferences, error)
	
	// CreatePreferences 创建用户偏好
	CreatePreferences(ctx context.Context, prefs *NotificationPreferences) error
	
	// UpdatePreferences 更新用户偏好
	UpdatePreferences(ctx context.Context, prefs *NotificationPreferences) error
	
	// FindTenantConfig 查询租户配置
	FindTenantConfig(ctx context.Context, tenantID uint) (*TenantNotificationConfig, error)
	
	// CreateTenantConfig 创建租户配置
	CreateTenantConfig(ctx context.Context, config *TenantNotificationConfig) error
	
	// UpdateTenantConfig 更新租户配置
	UpdateTenantConfig(ctx context.Context, config *TenantNotificationConfig) error
	
	// CreateHistory 创建通知历史
	CreateHistory(ctx context.Context, history *NotificationHistory) error
	
	// FindHistoryByUser 查询用户通知历史
	FindHistoryByUser(ctx context.Context, tenantID uint, userID string, limit int) ([]*NotificationHistory, error)
	
	// FindFailedNotifications 查询失败的通知
	FindFailedNotifications(ctx context.Context, tenantID uint, maxAttempts int) ([]*NotificationHistory, error)
	
	// UpdateHistoryStatus 更新通知历史状态
	UpdateHistoryStatus(ctx context.Context, historyID uint, status string, failReason string) error
	
	// DeleteOldHistory 删除过期历史
	DeleteOldHistory(ctx context.Context, tenantID uint, beforeDate string) (int64, error)
}
