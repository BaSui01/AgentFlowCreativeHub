package notification

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// NotificationChannel 通知渠道类型
type NotificationChannel string

const (
	ChannelEmail     NotificationChannel = "email"
	ChannelWebhook   NotificationChannel = "webhook"
	ChannelWebSocket NotificationChannel = "websocket"
	ChannelSMS       NotificationChannel = "sms" // 预留
	ChannelPush      NotificationChannel = "push" // 预留
)

// NotificationCategory 通知类别
type NotificationCategory string

const (
	CategoryApproval NotificationCategory = "approval"      // 审批通知
	CategoryWorkflow NotificationCategory = "workflow"      // 工作流通知
	CategorySystem   NotificationCategory = "system"        // 系统通知
	CategorySecurity NotificationCategory = "security"      // 安全通知
	CategoryAgent    NotificationCategory = "agent"         // Agent通知
	CategoryQuota    NotificationCategory = "quota"         // 配额通知
)

// EmailFrequency 邮件摘要频率
type EmailFrequency string

const (
	FrequencyRealtime EmailFrequency = "realtime" // 实时
	FrequencyHourly   EmailFrequency = "hourly"   // 每小时
	FrequencyDaily    EmailFrequency = "daily"    // 每日
	FrequencyWeekly   EmailFrequency = "weekly"   // 每周
	FrequencyNever    EmailFrequency = "never"    // 不发送
)

// NotificationPreferences 用户通知偏好设置
type NotificationPreferences struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TenantID  uint      `json:"tenantId" gorm:"not null;index:idx_tenant_user,priority:1"`
	UserID    string    `json:"userId" gorm:"type:varchar(255);not null;index:idx_tenant_user,priority:2;index"`
	
	// 渠道启用状态
	EnabledChannels ChannelList `json:"enabledChannels" gorm:"type:jsonb;default:'[\"websocket\",\"email\"]'"`
	
	// 分类订阅设置（按类别开关通知）
	CategoryPreferences CategoryPreferences `json:"categoryPreferences" gorm:"type:jsonb;default:'{}'"`
	
	// 邮件偏好
	EmailAddress       string         `json:"emailAddress" gorm:"type:varchar(255)"`
	EmailFrequency     EmailFrequency `json:"emailFrequency" gorm:"type:varchar(20);default:'realtime'"`
	DigestEnabled      bool           `json:"digestEnabled" gorm:"default:false"` // 是否启用邮件摘要
	
	// 静音时段（24小时制，如 22:00-08:00 表示晚10点到早8点静音）
	QuietHoursStart *string `json:"quietHoursStart" gorm:"type:varchar(5)"` // HH:MM 格式
	QuietHoursEnd   *string `json:"quietHoursEnd" gorm:"type:varchar(5)"`   // HH:MM 格式
	QuietHoursEnabled bool `json:"quietHoursEnabled" gorm:"default:false"`
	
	// Webhook配置
	WebhookURL     string            `json:"webhookUrl" gorm:"type:varchar(500)"`
	WebhookHeaders map[string]string `json:"webhookHeaders" gorm:"type:jsonb"`
	
	// 通知过滤规则（高级配置）
	FilterRules NotificationFilters `json:"filterRules" gorm:"type:jsonb"`
	
	// 时区设置
	Timezone string `json:"timezone" gorm:"type:varchar(50);default:'UTC'"`
	
	// 语言设置
	Language string `json:"language" gorm:"type:varchar(10);default:'zh-CN'"`
	
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	CreatedBy string     `json:"createdBy" gorm:"type:varchar(255)"`
	UpdatedBy string     `json:"updatedBy" gorm:"type:varchar(255)"`
}

// TableName 指定表名
func (NotificationPreferences) TableName() string {
	return "notification_preferences"
}

// ChannelList 渠道列表（支持JSONB存储）
type ChannelList []NotificationChannel

// Scan 实现 sql.Scanner 接口
func (cl *ChannelList) Scan(value interface{}) error {
	if value == nil {
		*cl = []NotificationChannel{}
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), cl)
	}
	return json.Unmarshal(bytes, cl)
}

// Value 实现 driver.Valuer 接口
func (cl ChannelList) Value() (driver.Value, error) {
	if len(cl) == 0 {
		return "[]", nil
	}
	return json.Marshal(cl)
}

// CategoryPreferences 分类偏好设置（哪些类别的通知要接收）
type CategoryPreferences map[NotificationCategory]bool

// Scan 实现 sql.Scanner 接口
func (cp *CategoryPreferences) Scan(value interface{}) error {
	if value == nil {
		*cp = make(CategoryPreferences)
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), cp)
	}
	return json.Unmarshal(bytes, cp)
}

// Value 实现 driver.Valuer 接口
func (cp CategoryPreferences) Value() (driver.Value, error) {
	if len(cp) == 0 {
		return "{}", nil
	}
	return json.Marshal(cp)
}

// NotificationFilters 通知过滤规则（高级功能）
type NotificationFilters struct {
	// 关键词过滤（包含这些关键词才通知）
	IncludeKeywords []string `json:"includeKeywords,omitempty"`
	// 排除关键词（包含这些关键词不通知）
	ExcludeKeywords []string `json:"excludeKeywords,omitempty"`
	// 最小优先级（低于此优先级不通知）
	MinPriority string `json:"minPriority,omitempty"` // low, medium, high, urgent
	// 仅通知我相关的（我创建的、我参与的、@我的）
	OnlyRelevant bool `json:"onlyRelevant"`
}

// Scan 实现 sql.Scanner 接口
func (nf *NotificationFilters) Scan(value interface{}) error {
	if value == nil {
		*nf = NotificationFilters{}
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(value.(string)), nf)
	}
	return json.Unmarshal(bytes, nf)
}

// Value 实现 driver.Valuer 接口
func (nf NotificationFilters) Value() (driver.Value, error) {
	return json.Marshal(nf)
}

// TenantNotificationConfig 租户级别的通知配置
type TenantNotificationConfig struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	TenantID uint   `json:"tenantId" gorm:"uniqueIndex;not null"`
	
	// 可用的通知渠道（租户可以限制哪些渠道可用）
	AvailableChannels ChannelList `json:"availableChannels" gorm:"type:jsonb;default:'[\"websocket\",\"email\",\"webhook\"]'"`
	
	// 默认邮件配置
	DefaultFromEmail string `json:"defaultFromEmail" gorm:"type:varchar(255)"`
	DefaultFromName  string `json:"defaultFromName" gorm:"type:varchar(255)"`
	
	// SMTP配置（租户自定义邮件服务器）
	SMTPHost     string `json:"smtpHost" gorm:"type:varchar(255)"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPUsername string `json:"smtpUsername" gorm:"type:varchar(255)"`
	SMTPPassword string `json:"smtpPassword" gorm:"type:varchar(500)"` // 应加密存储
	UseTLS       bool   `json:"useTls" gorm:"default:true"`
	
	// 默认Webhook配置
	DefaultWebhookURL string `json:"defaultWebhookUrl" gorm:"type:varchar(500)"`
	
	// 通知限流配置（防止通知轰炸）
	RateLimitEnabled     bool `json:"rateLimitEnabled" gorm:"default:true"`
	MaxNotificationsPerHour int `json:"maxNotificationsPerHour" gorm:"default:100"`
	
	// 通知保留策略
	RetentionDays int `json:"retentionDays" gorm:"default:30"` // 通知历史保留天数
	
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (TenantNotificationConfig) TableName() string {
	return "tenant_notification_configs"
}

// NotificationHistory 通知历史记录（用于审计和重发）
type NotificationHistory struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TenantID  uint      `json:"tenantId" gorm:"not null;index:idx_tenant_time,priority:1"`
	UserID    string    `json:"userId" gorm:"type:varchar(255);not null;index"`
	
	Channel   NotificationChannel  `json:"channel" gorm:"type:varchar(20);not null"`
	Category  NotificationCategory `json:"category" gorm:"type:varchar(50);index"`
	
	Subject   string         `json:"subject" gorm:"type:varchar(500)"`
	Body      string         `json:"body" gorm:"type:text"`
	Data      map[string]any `json:"data" gorm:"type:jsonb"`
	
	// 发送状态
	Status       string     `json:"status" gorm:"type:varchar(20);default:'pending'"` // pending, sent, failed, skipped
	SentAt       *time.Time `json:"sentAt"`
	FailedReason string     `json:"failedReason" gorm:"type:text"`
	Attempts     int        `json:"attempts" gorm:"default:0"`
	
	// 关联信息
	RelatedType string `json:"relatedType" gorm:"type:varchar(50)"` // approval, workflow, agent等
	RelatedID   string `json:"relatedId" gorm:"type:varchar(255);index"`
	
	CreatedAt time.Time `json:"createdAt" gorm:"index:idx_tenant_time,priority:2"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (NotificationHistory) TableName() string {
	return "notification_history"
}
