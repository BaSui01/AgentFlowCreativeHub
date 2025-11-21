package types

import "time"

// AuditLog 审计日志纯数据模型
// 不依赖任何internal包
type AuditLog struct {
	ID            string                 `json:"id"`
	TenantID      string                 `json:"tenant_id"`
	UserID        string                 `json:"user_id"`
	EventType     string                 `json:"event_type"`
	EventCategory string                 `json:"event_category"`
	ResourceType  string                 `json:"resource_type"`
	ResourceID    string                 `json:"resource_id"`
	Action        string                 `json:"action"`
	Result        string                 `json:"result"` // success, failure
	IPAddress     string                 `json:"ip_address,omitempty"`
	UserAgent     string                 `json:"user_agent,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	CreatedAt     time.Time              `json:"created_at"`
}
