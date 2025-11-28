package multimodel

import (
	"time"
)

// DrawRequest 多模型抽卡请求
type DrawRequest struct {
	ModelIDs    []string       `json:"model_ids" binding:"required,min=2,max=5"`    // 要调用的模型ID列表（2-5个）
	AgentType   string         `json:"agent_type" binding:"required"`               // Agent类型（plot, writer等）
	Content     string         `json:"content" binding:"required"`                  // 输入内容
	ExtraParams map[string]any `json:"extra_params,omitempty"`                      // 额外参数
	Temperature *float64       `json:"temperature,omitempty"`                        // 温度（可选）
	MaxTokens   *int           `json:"max_tokens,omitempty"`                         // 最大token（可选）
}

// DrawResult 单个模型的抽卡结果
type DrawResult struct {
	ModelID     string         `json:"model_id"`              // 模型ID
	ModelName   string         `json:"model_name"`            // 模型名称
	Provider    string         `json:"provider"`              // 提供商
	Content     string         `json:"content"`               // 生成内容
	Success     bool           `json:"success"`               // 是否成功
	Error       string         `json:"error,omitempty"`       // 错误信息
	LatencyMs   int64          `json:"latency_ms"`            // 延迟（毫秒）
	TokenUsage  *TokenUsage    `json:"token_usage,omitempty"` // Token使用情况
	Score       float64        `json:"score"`                 // 质量评分（0-100）
	Metadata    map[string]any `json:"metadata,omitempty"`    // 元数据
}

// TokenUsage Token使用情况
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// DrawResponse 多模型抽卡响应
type DrawResponse struct {
	DrawID      string        `json:"draw_id"`       // 抽卡ID
	Results     []DrawResult  `json:"results"`       // 各模型结果
	BestModelID string        `json:"best_model_id"` // 最佳模型ID
	TotalTime   int64         `json:"total_time_ms"` // 总耗时
	SuccessRate float64       `json:"success_rate"`  // 成功率
}

// DrawHistory 抽卡历史记录
type DrawHistory struct {
	ID          string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID    string    `gorm:"type:varchar(36);not null;index" json:"tenant_id"`
	UserID      string    `gorm:"type:varchar(36);not null;index" json:"user_id"`
	AgentType   string    `gorm:"type:varchar(50);not null" json:"agent_type"`
	ModelIDs    string    `gorm:"type:text;not null" json:"model_ids"`            // JSON数组
	InputPrompt string    `gorm:"type:text;not null" json:"input_prompt"`         // 输入提示词
	Results     string    `gorm:"type:jsonb;not null" json:"results"`             // JSON结果数组
	BestModelID string    `gorm:"type:varchar(36)" json:"best_model_id"`
	TotalTimeMs int64     `json:"total_time_ms"`
	SuccessRate float64   `json:"success_rate"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 指定表名
func (DrawHistory) TableName() string {
	return "draw_histories"
}

// ListDrawHistoryRequest 查询抽卡历史请求
type ListDrawHistoryRequest struct {
	AgentType *string `form:"agent_type"`
	Page      int     `form:"page" binding:"min=1"`
	PageSize  int     `form:"page_size" binding:"min=1,max=100"`
}

// RegenerateRequest 重新生成请求
type RegenerateRequest struct {
	DrawID   string  `json:"draw_id" binding:"required"`  // 抽卡ID
	ModelID  string  `json:"model_id" binding:"required"` // 要重新生成的模型ID
}

// CompareRequest 模型对比请求
type CompareRequest struct {
	ModelIDs    []string `json:"model_ids" binding:"required,min=2"`
	Prompt      string   `json:"prompt" binding:"required"`
	Temperature float64  `json:"temperature,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
}
