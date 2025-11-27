package models

import "time"

// ModelFeatures 模型能力特性
type ModelFeatures struct {
	Vision          bool `json:"vision"`
	FunctionCalling bool `json:"functionCalling"`
	Streaming       bool `json:"streaming"`
	Cache           bool `json:"cache"`
	JsonMode        bool `json:"jsonMode"`
}

// Model AI 模型配置
type Model struct {
	ID              string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID        string `json:"tenantId" gorm:"type:uuid;index"`
	ModelIdentifier string `json:"modelIdentifier" gorm:"size:255;not null;index"` // 模型标识符

	// 模型信息
	Name     string `json:"name" gorm:"size:100;not null;uniqueIndex"`
	Provider string `json:"provider" gorm:"size:50;not null"` // openai, anthropic, gemini, google, azure, deepseek, qwen, ollama, custom
	Type     string `json:"type" gorm:"size:50;not null"`     // chat, embedding
	Region   string `json:"region" gorm:"size:64"`            // 默认部署区域，便于多区域调度

	// 🆕 新增字段
	Category     string        `json:"category" gorm:"size:50;default:chat"`                    // chat, image, audio, video, embedding, rerank
	Features     ModelFeatures `json:"features" gorm:"type:jsonb;default:'{}';serializer:json"` // 能力特性
	BaseURL      string        `json:"baseUrl" gorm:"size:500"`                                 // 自定义端点
	APIVersion   string        `json:"apiVersion" gorm:"size:50"`                               // Azure API 版本
	APIFormat    string        `json:"apiFormat" gorm:"size:50;default:openai"`                 // openai, claude, gemini, deepseek, custom
	IsBuiltin    bool          `json:"isBuiltin" gorm:"default:false"`                          // 是否内置模型
	IsActive     bool          `json:"isActive" gorm:"default:true"`                            // 是否启用
	LastSyncedAt *time.Time    `json:"lastSyncedAt"`                                            // 最后同步时间

	// 能力描述
	Description             string         `json:"description" gorm:"type:text"`
	ContextWindow           int            `json:"contextWindow" gorm:"default:0"`
	MaxTokens               int            `json:"maxTokens"`
	SupportedLanguages      []string       `json:"supportedLanguages" gorm:"type:jsonb;serializer:json"`
	Capabilities            map[string]any `json:"capabilities" gorm:"type:jsonb;serializer:json"`
	SupportsStreaming       bool           `json:"supportsStreaming" gorm:"default:false"`
	SupportsFunctionCalling bool           `json:"supportsFunctionCalling" gorm:"default:false"`

	// 成本（按 1000 tokens 计算）
	InputCostPer1K  float64 `json:"inputCostPer1k" gorm:"type:decimal(10,6)"`
	OutputCostPer1K float64 `json:"outputCostPer1k" gorm:"type:decimal(10,6)"`
	RateLimitPerMin int     `json:"rateLimitPerMin" gorm:"default:0"`
	LatencySloMs    int     `json:"latencySloMs" gorm:"default:0"`

	// 状态
	Status              string         `json:"status" gorm:"size:50;not null;default:active"` // active, deprecated, disabled
	HealthStatus        string         `json:"healthStatus" gorm:"size:32;default:unknown"`
	LastHealthCheck     *time.Time     `json:"lastHealthCheck"`
	Metadata            map[string]any `json:"metadata" gorm:"type:jsonb;default:'{}';serializer:json"`
	DefaultCredentialID string         `json:"defaultCredentialId" gorm:"type:uuid"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}

// ModelCredential 模型凭证
type ModelCredential struct {
	ID           string         `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID     string         `json:"tenantId" gorm:"type:uuid;index;not null"`
	ModelID      string         `json:"modelId" gorm:"type:uuid;index;not null"`
	Provider     string         `json:"provider" gorm:"size:64;not null"`
	Name         string         `json:"name" gorm:"size:128;not null"`
	Ciphertext   []byte         `json:"ciphertext" gorm:"column:api_key_ciphertext;not null"`
	BaseURL      string         `json:"baseUrl" gorm:"size:500"`
	ExtraHeaders map[string]any `json:"extraHeaders" gorm:"type:jsonb;default:'{}';serializer:json"`
	Status       string         `json:"status" gorm:"size:32;default:active"`
	CreatedBy    string         `json:"createdBy" gorm:"type:uuid"`
	CreatedAt    time.Time      `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `json:"updatedAt" gorm:"autoUpdateTime"`
}

// ModelCallLog 模型调用日志
type ModelCallLog struct {
	ID              string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID        string `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID          string `json:"userId" gorm:"type:uuid;index"`
	ModelID         string `json:"modelId" gorm:"type:uuid;not null;index"`
	ActiveModelID   string `json:"activeModelId" gorm:"type:uuid;index"`
	FallbackUsed    bool   `json:"fallbackUsed" gorm:"default:false"`
	FallbackModelID string `json:"fallbackModelId" gorm:"type:uuid;index"`
	ErrorType       string `json:"errorType" gorm:"size:64"`

	// Token 统计
	PromptTokens     int `json:"promptTokens" gorm:"not null"`
	CompletionTokens int `json:"completionTokens" gorm:"not null"`
	TotalTokens      int `json:"totalTokens" gorm:"not null"`

	// 成本
	TotalCost float64 `json:"totalCost" gorm:"type:decimal(10,6)"`

	// 性能
	LatencyMs int `json:"latencyMs"`

	// 关联
	WorkflowExecutionID string `json:"workflowExecutionId" gorm:"type:uuid;index"`
	TaskID              string `json:"taskId" gorm:"type:uuid;index"`

	// 追踪
	TraceID string `json:"traceId" gorm:"size:100;index"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime;index"`
}
