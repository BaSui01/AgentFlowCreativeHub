package types

import "time"

// AICallLog AI调用日志数据模型
// 纯数据结构,不依赖任何internal包
type AICallLog struct {
	ID            string                 `json:"id"`
	TenantID      string                 `json:"tenant_id"`
	UserID        string                 `json:"user_id"`
	ModelProvider string                 `json:"model_provider"` // openai, anthropic, azure等
	ModelName     string                 `json:"model_name"`     // gpt-4, claude-3-opus等
	RequestTokens int                    `json:"request_tokens"`
	ResponseTokens int                   `json:"response_tokens"`
	TotalTokens   int                    `json:"total_tokens"`
	LatencyMS     int64                  `json:"latency_ms"`
	Cost          float64                `json:"cost"`
	Status        string                 `json:"status"` // success, error
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}
