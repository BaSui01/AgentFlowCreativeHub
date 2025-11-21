package agents

// createAgentConfigRequest 创建 Agent 配置请求体。
type createAgentConfigRequest struct {
	AgentType        string         `json:"agentType" binding:"required"`
	Name             string         `json:"name" binding:"required"`
	Description      string         `json:"description"`
	ModelID          string         `json:"modelId" binding:"required"`
	PromptTemplateID string         `json:"promptTemplateId"`
	SystemPrompt     string         `json:"systemPrompt"`
	Temperature      float64        `json:"temperature"`
	MaxTokens        int            `json:"maxTokens"`
	ExtraConfig      map[string]any `json:"extraConfig"`
}

// updateAgentConfigRequest 更新 Agent 配置请求体。
type updateAgentConfigRequest struct {
	Name             *string        `json:"name"`
	Description      *string        `json:"description"`
	ModelID          *string        `json:"modelId"`
	PromptTemplateID *string        `json:"promptTemplateId"`
	SystemPrompt     *string        `json:"systemPrompt"`
	Temperature      *float64       `json:"temperature"`
	MaxTokens        *int           `json:"maxTokens"`
	ExtraConfig      map[string]any `json:"extraConfig"`
}
