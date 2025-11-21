package aiinterface

import "context"

// Message 消息结构
type Message struct {
	Role       string     `json:"role"`                  // system, user, assistant, function, tool
	Content    string     `json:"content"`               // 消息内容
	Name       string     `json:"name,omitempty"`        // 发送消息的 Author 名称 (role=function/tool 时必填)
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`  // 模型请求的工具调用 (role=assistant)
	ToolCallID string     `json:"tool_call_id,omitempty"` // 工具调用的 ID (role=tool)
}

// ChatCompletionRequest 对话补全请求
type ChatCompletionRequest struct {
	Messages    []Message      `json:"messages"`               // 消息列表
	Temperature float64        `json:"temperature"`            // 温度参数（0-2）
	MaxTokens   int            `json:"max_tokens"`             // 最大 Token 数
	TopP        float64        `json:"top_p"`                  // Top P 采样
	Stream      bool           `json:"stream"`                 // 是否流式响应
	Tools       []Tool         `json:"tools,omitempty"`        // 可用工具列表（Function Calling）
	ToolChoice  any            `json:"tool_choice,omitempty"`  // "auto", "none", 或指定工具
	ExtraParams map[string]any `json:"extra_params"`           // 额外参数
}

// ChatCompletionResponse 对话补全响应
type ChatCompletionResponse struct {
	ID        string     `json:"id"`                     // 响应 ID
	Model     string     `json:"model"`                  // 使用的模型
	Content   string     `json:"content"`                // 生成的内容
	Usage     Usage      `json:"usage"`                  // Token 使用情况
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`   // 模型请求的工具调用
}

// Usage Token 使用情况
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`     // 输入 Token 数
	CompletionTokens int `json:"completion_tokens"` // 输出 Token 数
	TotalTokens      int `json:"total_tokens"`      // 总 Token 数
}

// StreamChunk 流式响应块
type StreamChunk struct {
	ID      string `json:"id"`      // 响应 ID
	Model   string `json:"model"`   // 使用的模型
	Content string `json:"content"` // 增量内容
	Done    bool   `json:"done"`    // 是否结束
}

// EmbeddingRequest 向量化请求
type EmbeddingRequest struct {
	Texts []string `json:"texts"` // 待向量化的文本列表
	Model string   `json:"model"` // 模型标识
}

// Tool 工具定义（OpenAI Function Calling 格式）
type Tool struct {
	Type     string      `json:"type"`     // 固定为 "function"
	Function FunctionDef `json:"function"` // 函数定义
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string         `json:"name"`        // 函数名称
	Description string         `json:"description"` // 函数描述
	Parameters  map[string]any `json:"parameters"`  // JSON Schema 参数定义
}

// ToolCall 工具调用请求（模型返回）
type ToolCall struct {
	ID       string `json:"id"`   // 调用 ID
	Type     string `json:"type"` // 固定为 "function"
	Function struct {
		Name      string `json:"name"`      // 函数名称
		Arguments string `json:"arguments"` // JSON 格式的参数
	} `json:"function"`
}

// EmbeddingResponse 向量化响应
type EmbeddingResponse struct {
	Embeddings [][]float64 `json:"embeddings"` // 向量列表
	Usage      Usage       `json:"usage"`      // Token 使用情况
}

// ModelClient AI 模型客户端统一接口
type ModelClient interface {
	// ChatCompletion 对话补全（非流式）
	ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)

	// ChatCompletionStream 对话补全（流式）
	// 返回的 channel 会持续发送响应块，直到完成或出错
	ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamChunk, <-chan error)

	// Embedding 文本向量化
	Embedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)

	// Name 返回客户端名称（如 "openai", "anthropic"）
	Name() string

	// Close 关闭客户端连接
	Close() error
}

// StreamReader 流式响应读取器
type StreamReader interface {
	// Read 读取下一个响应块
	Read() (*StreamChunk, error)

	// Close 关闭流
	Close() error
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Provider   string         // 提供商（openai, anthropic, custom）
	APIKey     string         // API Key
	BaseURL    string         // 基础 URL
	Model      string         // 模型标识
	OrgID      string         // 组织 ID（OpenAI）
	MaxRetries int            // 最大重试次数
	Timeout    int            // 超时时间（秒）
	Extra      map[string]any // 额外配置
}

// ErrorType 错误类型
type ErrorType string

const (
	ErrorTypeAuth          ErrorType = "auth"           // 认证错误
	ErrorTypeRateLimit     ErrorType = "rate_limit"     // 速率限制
	ErrorTypeInvalidParams ErrorType = "invalid_params" // 参数错误
	ErrorTypeServerError   ErrorType = "server_error"   // 服务器错误
	ErrorTypeNetwork       ErrorType = "network"        // 网络错误
	ErrorTypeUnknown       ErrorType = "unknown"        // 未知错误
)

// ClientError 客户端错误
type ClientError struct {
	Type    ErrorType // 错误类型
	Message string    // 错误消息
	Err     error     // 原始错误
}

// Error 实现error接口
func (e *ClientError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap 返回原始错误
func (e *ClientError) Unwrap() error {
	return e.Err
}

// IsRetryable 判断错误是否可重试
func (e *ClientError) IsRetryable() bool {
	return e.Type == ErrorTypeRateLimit || e.Type == ErrorTypeNetwork || e.Type == ErrorTypeServerError
}
