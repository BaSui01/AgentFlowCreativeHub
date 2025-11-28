package notifications

import (
	"time"
)

// ========== 用户偏好 ==========

// UpdatePreferencesRequest 更新通知偏好请求
type UpdatePreferencesRequest struct {
	EnabledChannels     []string               `json:"enabledChannels"`
	CategoryPreferences map[string]interface{} `json:"categoryPreferences"`
	EmailAddress        *string                `json:"emailAddress"`
	EmailFrequency      *string                `json:"emailFrequency"`
	DigestEnabled       *bool                  `json:"digestEnabled"`
	QuietHoursStart     *string                `json:"quietHoursStart"`
	QuietHoursEnd       *string                `json:"quietHoursEnd"`
	QuietHoursEnabled   *bool                  `json:"quietHoursEnabled"`
	Timezone            *string                `json:"timezone"`
	Language            *string                `json:"language"`
}

// SetQuietHoursRequest 设置静音时段请求
type SetQuietHoursRequest struct {
	Start   string `json:"start" binding:"required"`
	End     string `json:"end" binding:"required"`
	Enabled bool   `json:"enabled"`
}

// ========== 消息 ==========

// UnreadCountResponse 未读数量响应
type UnreadCountResponse struct {
	Total      int            `json:"total"`
	ByCategory map[string]int `json:"byCategory,omitempty"`
}

// ========== 租户配置 ==========

// UpdateTenantConfigRequest 更新租户通知配置请求
type UpdateTenantConfigRequest struct {
	AvailableChannels       []string `json:"availableChannels"`
	DefaultFromEmail        *string  `json:"defaultFromEmail"`
	DefaultFromName         *string  `json:"defaultFromName"`
	SMTPHost                *string  `json:"smtpHost"`
	SMTPPort                *int     `json:"smtpPort"`
	SMTPUsername            *string  `json:"smtpUsername"`
	SMTPPassword            *string  `json:"smtpPassword"`
	UseTLS                  *bool    `json:"useTls"`
	DefaultWebhookURL       *string  `json:"defaultWebhookUrl"`
	RateLimitEnabled        *bool    `json:"rateLimitEnabled"`
	MaxNotificationsPerHour *int     `json:"maxNotificationsPerHour"`
	RetentionDays           *int     `json:"retentionDays"`
}

// ========== 通知历史 ==========

// NotificationHistoryResponse 通知历史响应
type NotificationHistoryResponse struct {
	ID          string                 `json:"id"`
	Channel     string                 `json:"channel"`
	Category    string                 `json:"category"`
	Subject     string                 `json:"subject"`
	Status      string                 `json:"status"`
	SentAt      *time.Time             `json:"sentAt"`
	RelatedType string                 `json:"relatedType,omitempty"`
	RelatedID   string                 `json:"relatedId,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	Data        map[string]interface{} `json:"data,omitempty"`
}
