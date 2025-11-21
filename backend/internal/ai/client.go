package ai

import (
	"context"

	"backend/pkg/aiinterface"
)

// 重新导出aiinterface包的类型,保持向后兼容
// 这样ai包的使用者无需修改代码,同时避免了子包对父包的依赖
type (
	Message                = aiinterface.Message
	ChatCompletionRequest  = aiinterface.ChatCompletionRequest
	ChatCompletionResponse = aiinterface.ChatCompletionResponse
	Usage                  = aiinterface.Usage
	StreamChunk            = aiinterface.StreamChunk
	EmbeddingRequest       = aiinterface.EmbeddingRequest
	EmbeddingResponse      = aiinterface.EmbeddingResponse
	Tool                   = aiinterface.Tool
	FunctionDef            = aiinterface.FunctionDef
	ToolCall               = aiinterface.ToolCall
	ModelClient            = aiinterface.ModelClient
	StreamReader           = aiinterface.StreamReader
	ClientConfig           = aiinterface.ClientConfig
	ClientError            = aiinterface.ClientError
	ErrorType              = aiinterface.ErrorType
)

// ModelProvider 模型提供方接口,用于通过租户和模型ID获取模型客户端
// 现有的 ClientFactory 实现了该接口,上层可依赖此接口以便后续替换实现
type ModelProvider interface {
	GetClient(ctx context.Context, tenantID, modelID string) (ModelClient, error)
}

// 重新导出常量
const (
	ErrorTypeAuth          = aiinterface.ErrorTypeAuth
	ErrorTypeRateLimit     = aiinterface.ErrorTypeRateLimit
	ErrorTypeInvalidParams = aiinterface.ErrorTypeInvalidParams
	ErrorTypeServerError   = aiinterface.ErrorTypeServerError
	ErrorTypeNetwork       = aiinterface.ErrorTypeNetwork
	ErrorTypeUnknown       = aiinterface.ErrorTypeUnknown
)

// ModelCallLogger 模型调用日志记录器接口
type ModelCallLogger interface {
	// Log 记录模型调用
	Log(ctx context.Context, log *ModelCallLog) error
}

// ModelCallLog 模型调用日志
type ModelCallLog struct {
	TenantID         string  `json:"tenant_id"`
	UserID           string  `json:"user_id"`
	ModelID          string  `json:"model_id"`
	ModelProvider    string  `json:"model_provider"`
	ModelName        string  `json:"model_name"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TotalCost        float64 `json:"total_cost"`
	LatencyMs        int64   `json:"latency_ms"`
	WorkflowID       *string `json:"workflow_id,omitempty"`
	TraceID          *string `json:"trace_id,omitempty"`
}

// 注意: 所有客户端配置、错误类型等定义已迁移到pkg/aiinterface
// 这里通过type alias提供向后兼容
