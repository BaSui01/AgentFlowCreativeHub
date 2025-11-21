package workflow

import "time"

// ApprovalRequest 审批请求
type ApprovalRequest struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string `json:"tenantId" gorm:"type:uuid;not null;index"`
	ExecutionID string `json:"executionId" gorm:"type:uuid;not null;index"`
	WorkflowID  string `json:"workflowId" gorm:"type:uuid;index"`
	StepID      string `json:"stepId" gorm:"size:100;not null"`

	// 审批信息
	Status       string  `json:"status" gorm:"size:50;not null;default:pending"` // pending、approved、rejected、timeout
	Type         string  `json:"type" gorm:"size:50;not null"`                   // required、optional、conditional
	RequestedBy  string  `json:"requestedBy" gorm:"type:uuid;not null"`
	ApprovedBy   *string `json:"approvedBy" gorm:"type:uuid"`
	AutoApproved bool    `json:"autoApproved" gorm:"default:false"`

	// 内容
	StepOutput map[string]any `json:"stepOutput" gorm:"type:jsonb;serializer:json"`
	Comment    string         `json:"comment" gorm:"type:text"`

	// 通知
	NotifyChannels        []string            `json:"notifyChannels" gorm:"type:text[]"`
	NotifyTargets         map[string][]string `json:"notifyTargets" gorm:"type:jsonb;serializer:json"`
	NotifiedAt            *time.Time          `json:"notifiedAt"`
	NotificationAttempts  int                 `json:"notificationAttempts" gorm:"default:0"`
	LastNotifiedAt        *time.Time          `json:"lastNotifiedAt"`
	LastNotificationError string              `json:"lastNotificationError" gorm:"type:text"`

	// 超时
	TimeoutSeconds int        `json:"timeoutSeconds" gorm:"default:3600"`
	ExpiresAt      *time.Time `json:"expiresAt"`

	// 时间戳
	CreatedAt  time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt  time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	ResolvedAt *time.Time `json:"resolvedAt"`
}

// AutomationLog 自动化执行日志
type AutomationLog struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string `json:"tenantId" gorm:"type:uuid;not null;index"`
	ExecutionID string `json:"executionId" gorm:"type:uuid;not null;index"`
	StepID      string `json:"stepId" gorm:"size:100"`

	// 日志信息
	Action  string         `json:"action" gorm:"size:100;not null"` // agent_switch、auto_retry、quality_check、approval_auto
	Details map[string]any `json:"details" gorm:"type:jsonb;serializer:json"`
	Result  string         `json:"result" gorm:"size:50;not null"` // success、failed、skipped

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime;index"`
}
