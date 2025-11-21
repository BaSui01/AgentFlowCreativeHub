package runtime

import (
	"context"

	agentpkg "backend/internal/agent"
)

// Agent AI Agent 接口
type Agent interface {
	// Execute 执行 Agent 任务（非流式）
	Execute(ctx context.Context, input *AgentInput) (*AgentResult, error)

	// ExecuteStream 执行 Agent 任务（流式）
	ExecuteStream(ctx context.Context, input *AgentInput) (<-chan AgentChunk, <-chan error)

	// Name 返回 Agent 名称
	Name() string

	// Type 返回 Agent 类型
	Type() string
}

// AgentInput Agent 输入
type AgentInput struct {
	// Content 输入内容
	Content string `json:"content"`

	// Variables 变量（用于模板渲染）
	Variables map[string]any `json:"variables,omitempty"`

	// History 历史对话（可选）
	History []Message `json:"history,omitempty"`

	// ExtraParams 额外参数
	ExtraParams map[string]any `json:"extra_params,omitempty"`

	// Context 上下文信息
	Context *AgentContext `json:"context,omitempty"`
}

// AgentResult Agent 执行结果
type AgentResult struct {
	// Output 输出内容
	Output string `json:"output"`

	// StructuredOutput 结构化输出 (新增)
	StructuredOutput any `json:"structured_output,omitempty"`

	// Metadata 元数据
	Metadata map[string]any `json:"metadata,omitempty"`

	// Usage Token 使用情况
	Usage *Usage `json:"usage,omitempty"`

	// Cost 成本
	Cost float64 `json:"cost"`

	// LatencyMs 延迟（毫秒）
	LatencyMs int64 `json:"latency_ms"`

	// Status 状态（success, failed, partial）
	Status string `json:"status"`

	// Error 错误信息（如果失败）
	Error string `json:"error,omitempty"`
}

// AgentChunk Agent 流式响应块
type AgentChunk struct {
	// Content 增量内容
	Content string `json:"content"`

	// Done 是否结束
	Done bool `json:"done"`

	// Metadata 元数据（仅在最后一个 chunk）
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Message 消息
type Message struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"` // 消息内容
}

// Usage Token 使用情况
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// AgentContext Agent 上下文
type AgentContext struct {
	// TenantID 租户 ID
	TenantID string `json:"tenant_id"`

	// UserID 用户 ID
	UserID string `json:"user_id"`

	// WorkflowID 工作流 ID（如果在工作流中执行）
	WorkflowID *string `json:"workflow_id,omitempty"`

	// TraceID 追踪 ID
	TraceID *string `json:"trace_id,omitempty"`

	// StepID 步骤 ID（如果在工作流中执行）
	StepID *string `json:"step_id,omitempty"`

	// SessionID 会话 ID（用于多轮对话）
	SessionID *string `json:"session_id,omitempty"`

	// Data 共享数据（工作流中步骤间传递）
	Data map[string]any `json:"data,omitempty"`
}

// AgentConfig Agent 配置
type AgentConfig struct {
	// Name Agent 名称
	Name string `json:"name"`

	// Type Agent 类型
	Type string `json:"type"`

	// ModelID 使用的模型 ID
	ModelID string `json:"model_id"`

	// PromptTemplateID Prompt 模板 ID（可选）
	PromptTemplateID *string `json:"prompt_template_id,omitempty"`

	// SystemPrompt 系统提示词（可选）
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Temperature 温度参数
	Temperature float64 `json:"temperature"`

	// MaxTokens 最大 Token 数
	MaxTokens int `json:"max_tokens"`

	// ExtraConfig 额外配置
	ExtraConfig map[string]any `json:"extra_config,omitempty"`

	// AllowedTools 允许调用的工具列表
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// AgentConfig 完整配置（用于访问 RAG 等扩展配置）
	AgentConfig *agentpkg.AgentConfig `json:"-"`
}

// AgentBuilder Agent 构建器接口
type AgentBuilder interface {
	// Build 构建 Agent
	Build(config *AgentConfig) (Agent, error)

	// SupportedTypes 返回支持的 Agent 类型列表
	SupportedTypes() []string
}

// ExecutionOptions 执行选项
type ExecutionOptions struct {
	// Stream 是否流式响应
	Stream bool

	// Timeout 超时时间（秒）
	Timeout int

	// RetryOnError 出错时是否重试
	RetryOnError bool

	// MaxRetries 最大重试次数
	MaxRetries int

	// SaveHistory 是否保存历史
	SaveHistory bool
}
