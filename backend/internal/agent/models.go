package agent

import "time"

// AgentConfig Agent 配置
type AgentConfig struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;not null;index"`

	// Agent 信息
	AgentType   string `json:"agentType" gorm:"size:100;not null"`   // writer, reviewer, planner, formatter, translator, analyzer, researcher
	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`

	// 模型配置
	PrimaryModelID   string `json:"primaryModelId" gorm:"column:primary_model_id;type:uuid;not null"`
	SecondaryModelID string `json:"secondaryModelId" gorm:"column:secondary_model_id;type:uuid"`
	FallbackStrategy string `json:"fallbackStrategy" gorm:"column:fallback_strategy;size:32;default:auto"`
	FallbackTimeoutMs int  `json:"fallbackTimeoutMs" gorm:"column:fallback_timeout_ms;default:0"`
	LastFallbackAt  *time.Time `json:"lastFallbackAt" gorm:"column:last_fallback_at"`
	ModelID string `json:"modelId" gorm:"-"`
	ActiveModelID string `json:"activeModelId" gorm:"-"`
	FallbackActive bool `json:"fallbackActive" gorm:"-"`
	PromptTemplateID string `json:"promptTemplateId" gorm:"type:uuid"`
	SystemPrompt     string `json:"systemPrompt" gorm:"type:text"` // 系统提示词

	// RAG 配置（Sprint 6）
	KnowledgeBaseID string `json:"knowledgeBaseId" gorm:"type:uuid"`       // 关联的知识库 ID
	RAGEnabled      bool   `json:"ragEnabled" gorm:"default:false"`        // 是否启用 RAG
	RAGTopK         int    `json:"ragTopK" gorm:"default:3"`               // RAG 检索数量
	RAGMinScore     float64 `json:"ragMinScore" gorm:"type:decimal(3,2);default:0.7"` // RAG 最小相似度

	// 参数
	Temperature float64 `json:"temperature" gorm:"type:decimal(3,2);default:0.7"`
	MaxTokens   int     `json:"maxTokens" gorm:"default:4096"`

	// 工具配置
	AllowedTools []string `json:"allowedTools" gorm:"type:jsonb;serializer:json"` // 允许使用的工具 ID 列表
	AutoToolUse  bool     `json:"autoToolUse" gorm:"default:false"`               // 是否允许模型自主调用工具

	// 扩展配置
	ExtraConfig map[string]any `json:"extraConfig" gorm:"type:jsonb;serializer:json"`

	// 状态
	Status string `json:"status" gorm:"size:50;not null;default:active"` // active, disabled

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}
