package agents

// createAgentConfigRequest 创建 Agent 配置请求体。
type createAgentConfigRequest struct {
	AgentType   string `json:"agentType" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`

	// 模型配置（主/备用）
	ModelID           string `json:"modelId" binding:"required"` // 主模型
	SecondaryModelID  string `json:"secondaryModelId"`           // 备用模型
	FallbackStrategy  string `json:"fallbackStrategy"`           // 降级策略: auto, immediate, after_retry, manual
	FallbackTimeoutMs int    `json:"fallbackTimeoutMs"`          // 降级超时(毫秒)

	// 任务专用模型
	ToolModelID     string            `json:"toolModelId"`     // 工具调用模型
	CreativeModelID string            `json:"creativeModelId"` // 创作任务模型
	AnalysisModelID string            `json:"analysisModelId"` // 分析任务模型
	SummaryModelID  string            `json:"summaryModelId"`  // 摘要任务模型
	ModelRouting    map[string]string `json:"modelRouting"`    // 自定义任务类型路由

	// Prompt 配置
	PromptTemplateID string `json:"promptTemplateId"`
	SystemPrompt     string `json:"systemPrompt"`

	// 参数配置
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"maxTokens"`

	// 工具配置
	Tools       []string `json:"tools"`       // 允许使用的工具 ID 列表
	AutoToolUse bool     `json:"autoToolUse"` // 是否允许模型自主调用工具

	// RAG 配置
	KnowledgeBaseIDs []string `json:"knowledgeBaseIds"` // 关联的知识库 ID 列表
	RAGEnabled       bool     `json:"ragEnabled"`       // 是否启用 RAG
	RAGTopK          int      `json:"ragTopK"`          // RAG 检索数量
	RAGMinScore      float64  `json:"ragMinScore"`      // RAG 最小相似度

	// 状态
	Status string `json:"status"` // active, disabled

	// 扩展配置
	ExtraConfig map[string]any `json:"extraConfig"`
}

// updateAgentConfigRequest 更新 Agent 配置请求体。
type updateAgentConfigRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`

	// 模型配置
	ModelID           *string `json:"modelId"`
	SecondaryModelID  *string `json:"secondaryModelId"`
	FallbackStrategy  *string `json:"fallbackStrategy"`
	FallbackTimeoutMs *int    `json:"fallbackTimeoutMs"`

	// 任务专用模型
	ToolModelID     *string           `json:"toolModelId"`
	CreativeModelID *string           `json:"creativeModelId"`
	AnalysisModelID *string           `json:"analysisModelId"`
	SummaryModelID  *string           `json:"summaryModelId"`
	ModelRouting    map[string]string `json:"modelRouting"`

	// Prompt 配置
	PromptTemplateID *string `json:"promptTemplateId"`
	SystemPrompt     *string `json:"systemPrompt"`

	// 参数配置
	Temperature *float64 `json:"temperature"`
	MaxTokens   *int     `json:"maxTokens"`

	// 工具配置
	Tools       []string `json:"tools"`
	AutoToolUse *bool    `json:"autoToolUse"`

	// RAG 配置
	KnowledgeBaseIDs []string `json:"knowledgeBaseIds"`
	RAGEnabled       *bool    `json:"ragEnabled"`
	RAGTopK          *int     `json:"ragTopK"`
	RAGMinScore      *float64 `json:"ragMinScore"`

	// 状态
	Status *string `json:"status"`

	// 扩展配置
	ExtraConfig map[string]any `json:"extraConfig"`
}
