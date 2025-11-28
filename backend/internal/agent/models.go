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

	// 模型配置（主/备用）
	PrimaryModelID    string     `json:"primaryModelId" gorm:"column:primary_model_id;type:uuid"`
	SecondaryModelID  string     `json:"secondaryModelId" gorm:"column:secondary_model_id;type:uuid"`
	FallbackStrategy  string     `json:"fallbackStrategy" gorm:"column:fallback_strategy;size:32;default:auto"`
	FallbackTimeoutMs int        `json:"fallbackTimeoutMs" gorm:"column:fallback_timeout_ms;default:0"`
	LastFallbackAt    *time.Time `json:"lastFallbackAt" gorm:"column:last_fallback_at"`

	// 任务专用模型（按任务类型路由）
	ToolModelID     string `json:"toolModelId" gorm:"column:tool_model_id;type:uuid"`         // 工具调用：便宜快速
	CreativeModelID string `json:"creativeModelId" gorm:"column:creative_model_id;type:uuid"` // 创作任务：擅长写作
	AnalysisModelID string `json:"analysisModelId" gorm:"column:analysis_model_id;type:uuid"` // 分析任务：强逻辑
	SummaryModelID  string `json:"summaryModelId" gorm:"column:summary_model_id;type:uuid"`   // 摘要任务：性价比高

	// 灵活模型路由配置（支持自定义任务类型）
	ModelRouting map[string]string `json:"modelRouting" gorm:"column:model_routing;type:jsonb;serializer:json"`

	// 计算字段（不存储）
	ModelID        string `json:"modelId" gorm:"-"`
	ActiveModelID  string `json:"activeModelId" gorm:"-"`
	FallbackActive bool   `json:"fallbackActive" gorm:"-"`

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

// TaskType 任务类型常量
const (
	TaskTypeTool     = "tool"     // 工具调用
	TaskTypeCreative = "creative" // 创作任务
	TaskTypeAnalysis = "analysis" // 分析任务
	TaskTypeSummary  = "summary"  // 摘要任务
)

// GetModelForTask 根据任务类型获取对应的模型 ID
// 优先级：专用模型字段 > ModelRouting配置 > 主模型
func (c *AgentConfig) GetModelForTask(taskType string) string {
	// 1. 先检查专用模型字段
	switch taskType {
	case TaskTypeTool:
		if c.ToolModelID != "" {
			return c.ToolModelID
		}
	case TaskTypeCreative:
		if c.CreativeModelID != "" {
			return c.CreativeModelID
		}
	case TaskTypeAnalysis:
		if c.AnalysisModelID != "" {
			return c.AnalysisModelID
		}
	case TaskTypeSummary:
		if c.SummaryModelID != "" {
			return c.SummaryModelID
		}
	}

	// 2. 检查 ModelRouting 配置
	if c.ModelRouting != nil {
		if modelID, ok := c.ModelRouting[taskType]; ok && modelID != "" {
			return modelID
		}
	}

	// 3. 回退到主模型
	if c.ActiveModelID != "" {
		return c.ActiveModelID
	}
	return c.PrimaryModelID
}
