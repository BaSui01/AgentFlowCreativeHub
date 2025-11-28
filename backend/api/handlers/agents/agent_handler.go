package agents

import (
	"net/http"
	"strconv"

	response "backend/api/handlers/common"
	"backend/internal/agent"

	"github.com/gin-gonic/gin"
)

// AgentHandler Agent 配置管理 Handler
type AgentHandler struct {
	service *agent.AgentService
}

// NewAgentHandler 创建 AgentHandler 实例
func NewAgentHandler(service *agent.AgentService) *AgentHandler {
	return &AgentHandler{service: service}
}

// ListAgentConfigs 查询 Agent 配置列表
// @Summary 查询 Agent 配置列表
// @Tags Agents
// @Security BearerAuth
// @Produce json
// @Param agent_type query string false "Agent 类型"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} agent.ListAgentConfigsResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/agents [get]
func (h *AgentHandler) ListAgentConfigs(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	req := &agent.ListAgentConfigsRequest{
		TenantID:  tenantID,
		AgentType: c.Query("agent_type"),
	}

	if page := c.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			req.Page = p
		}
	}
	if pageSize := c.Query("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil {
			req.PageSize = ps
		}
	}

	resp, err := h.service.ListAgentConfigs(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetAgentConfig 查询单个 Agent 配置
// @Summary 查询 Agent 配置详情
// @Tags Agents
// @Security BearerAuth
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} agent.AgentConfig
// @Failure 404 {object} response.ErrorResponse
// @Router /api/agents/{id} [get]
func (h *AgentHandler) GetAgentConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	agentID := c.Param("id")

	agentConfig, err := h.service.GetAgentConfig(c.Request.Context(), tenantID, agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, agentConfig)
}

// CreateAgentConfig 创建 Agent 配置
// @Summary 创建 Agent 配置
// @Tags Agents
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body createAgentConfigRequest true "Agent 配置"
// @Success 201 {object} agent.AgentConfig
// @Failure 400 {object} response.ErrorResponse
// @Router /api/agents [post]
func (h *AgentHandler) CreateAgentConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var body createAgentConfigRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	req := agent.CreateAgentConfigRequest{
		TenantID:    tenantID,
		AgentType:   body.AgentType,
		Name:        body.Name,
		Description: body.Description,
		// 模型配置
		ModelID:           body.ModelID,
		SecondaryModelID:  body.SecondaryModelID,
		FallbackStrategy:  body.FallbackStrategy,
		FallbackTimeoutMs: body.FallbackTimeoutMs,
		// 任务专用模型
		ToolModelID:     body.ToolModelID,
		CreativeModelID: body.CreativeModelID,
		AnalysisModelID: body.AnalysisModelID,
		SummaryModelID:  body.SummaryModelID,
		ModelRouting:    body.ModelRouting,
		// Prompt
		PromptTemplateID: body.PromptTemplateID,
		SystemPrompt:     body.SystemPrompt,
		// 参数
		Temperature: body.Temperature,
		MaxTokens:   body.MaxTokens,
		// 工具
		Tools:       body.Tools,
		AutoToolUse: body.AutoToolUse,
		// RAG
		KnowledgeBaseIDs: body.KnowledgeBaseIDs,
		RAGEnabled:       body.RAGEnabled,
		RAGTopK:          body.RAGTopK,
		RAGMinScore:      body.RAGMinScore,
		// 状态
		Status: body.Status,
		// 扩展
		ExtraConfig: body.ExtraConfig,
	}

	agentConfig, err := h.service.CreateAgentConfig(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, agentConfig)
}

// UpdateAgentConfig 更新 Agent 配置
// @Summary 更新 Agent 配置
// @Tags Agents
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body updateAgentConfigRequest true "Agent 配置"
// @Success 200 {object} agent.AgentConfig
// @Failure 400 {object} response.ErrorResponse
// @Router /api/agents/{id} [put]
func (h *AgentHandler) UpdateAgentConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	agentID := c.Param("id")

	var body updateAgentConfigRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	req := agent.UpdateAgentConfigRequest{
		Name:        body.Name,
		Description: body.Description,
		// 模型配置
		ModelID:           body.ModelID,
		SecondaryModelID:  body.SecondaryModelID,
		FallbackStrategy:  body.FallbackStrategy,
		FallbackTimeoutMs: body.FallbackTimeoutMs,
		// 任务专用模型
		ToolModelID:     body.ToolModelID,
		CreativeModelID: body.CreativeModelID,
		AnalysisModelID: body.AnalysisModelID,
		SummaryModelID:  body.SummaryModelID,
		ModelRouting:    body.ModelRouting,
		// Prompt
		PromptTemplateID: body.PromptTemplateID,
		SystemPrompt:     body.SystemPrompt,
		// 参数
		Temperature: body.Temperature,
		MaxTokens:   body.MaxTokens,
		// 工具
		Tools:       body.Tools,
		AutoToolUse: body.AutoToolUse,
		// RAG
		KnowledgeBaseIDs: body.KnowledgeBaseIDs,
		RAGEnabled:       body.RAGEnabled,
		RAGTopK:          body.RAGTopK,
		RAGMinScore:      body.RAGMinScore,
		// 状态
		Status: body.Status,
		// 扩展
		ExtraConfig: body.ExtraConfig,
	}

	agentConfig, err := h.service.UpdateAgentConfig(c.Request.Context(), tenantID, agentID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, agentConfig)
}

// DeleteAgentConfig 删除 Agent 配置
// @Summary 删除 Agent 配置
// @Tags Agents
// @Security BearerAuth
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /api/agents/{id} [delete]
func (h *AgentHandler) DeleteAgentConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	agentID := c.Param("id")
	operatorID := c.GetString("user_id")

	if err := h.service.DeleteAgentConfig(c.Request.Context(), tenantID, agentID, operatorID); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "Agent 配置删除成功"})
}

// GetAgentByType 根据类型获取 Agent 配置
// @Summary 根据类型获取 Agent 配置
// @Tags Agents
// @Security BearerAuth
// @Produce json
// @Param type path string true "Agent 类型"
// @Success 200 {object} agent.AgentConfig
// @Failure 404 {object} response.ErrorResponse
// @Router /api/agents/types/{type} [get]
func (h *AgentHandler) GetAgentByType(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	agentType := c.Param("type")

	agentConfig, err := h.service.GetAgentByType(c.Request.Context(), tenantID, agentType)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, agentConfig)
}

// SeedDefaultAgents 初始化预置 Agent
// @Summary 初始化预置 Agent
// @Tags Agents
// @Security BearerAuth
// @Produce json
// @Param default_model_id query string true "默认模型 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /api/agents/seed [post]
func (h *AgentHandler) SeedDefaultAgents(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	defaultModelID := c.Query("default_model_id")

	if defaultModelID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "default_model_id 参数不能为空"})
		return
	}

	if err := h.service.SeedDefaultAgents(c.Request.Context(), tenantID, defaultModelID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "预置 Agent 初始化成功"})
}
