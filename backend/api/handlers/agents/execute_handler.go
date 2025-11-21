package agents

import (
	"io"
	"net/http"

	response "backend/api/handlers/common"
	"backend/internal/agent/runtime"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentExecuteHandler Agent 执行 Handler
type AgentExecuteHandler struct {
	registry    *runtime.Registry
	asyncClient *runtime.AsyncClient
}

// NewAgentExecuteHandler 创建 AgentExecuteHandler 实例
func NewAgentExecuteHandler(registry *runtime.Registry, asyncClient *runtime.AsyncClient) *AgentExecuteHandler {
	return &AgentExecuteHandler{
		registry:    registry,
		asyncClient: asyncClient,
	}
}

// ExecuteAgentRequest 执行 Agent 请求
type ExecuteAgentRequest struct {
	Content     string         `json:"content" binding:"required"`
	Variables   map[string]any `json:"variables,omitempty"`
	SessionID   *string        `json:"session_id,omitempty"`
	ExtraParams map[string]any `json:"extra_params,omitempty"`
}

// AgentExecuteResponse 同步执行响应。
type AgentExecuteResponse struct {
	TraceID string               `json:"trace_id"`
	Result  *runtime.AgentResult `json:"result"`
}

// AgentAsyncResponse 异步执行响应。
type AgentAsyncResponse struct {
	RunID   string `json:"run_id"`
	TraceID string `json:"trace_id"`
	Status  string `json:"status"`
}

// Execute 执行 Agent（非流式）
// @Summary 执行 Agent（同步）
// @Tags Agents
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body ExecuteAgentRequest true "执行请求"
// @Success 200 {object} AgentExecuteResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/agents/{id}/execute [post]
func (h *AgentExecuteHandler) Execute(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	agentID := c.Param("id")

	var req ExecuteAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	// 生成 TraceID
	traceID := uuid.New().String()

	// 构建输入
	input := &runtime.AgentInput{
		Content:     req.Content,
		Variables:   req.Variables,
		ExtraParams: req.ExtraParams,
		Context: &runtime.AgentContext{
			TenantID:  tenantID,
			UserID:    userID,
			SessionID: req.SessionID,
			TraceID:   &traceID,
		},
	}

	// 执行 Agent
	result, err := h.registry.Execute(c.Request.Context(), tenantID, agentID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, AgentExecuteResponse{TraceID: traceID, Result: result})
}

// ExecuteAsync 异步执行 Agent
// @Summary 执行 Agent（异步）
// @Tags Agents
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body ExecuteAgentRequest true "执行请求"
// @Success 202 {object} AgentAsyncResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/agents/{id}/run [post]
func (h *AgentExecuteHandler) ExecuteAsync(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	agentID := c.Param("id")

	var req ExecuteAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	if h.asyncClient == nil {
		c.JSON(http.StatusNotImplemented, response.ErrorResponse{Success: false, Message: "异步执行服务未启用"})
		return
	}

	// 生成 TraceID
	traceID := uuid.New().String()

	// 构建 Payload
	payload := &runtime.AgentRunPayload{
		AgentID:     agentID,
		Input:       req.Content,
		Variables:   req.Variables,
		TraceID:     traceID,
		UserID:      userID,
		TenantID:    tenantID,
		ContextData: req.ExtraParams,
	}

	// 入队
	runID, err := h.asyncClient.EnqueueAgentRun(c.Request.Context(), payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "提交任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, AgentAsyncResponse{RunID: runID, TraceID: traceID, Status: "queued"})
}

// ExecuteStream 执行 Agent（流式）
// @Summary 执行 Agent（流式）
// @Tags Agents
// @Security BearerAuth
// @Accept json
// @Produce text/event-stream
// @Param id path string true "Agent ID"
// @Param request body ExecuteAgentRequest true "执行请求"
// @Success 200 {string} string "SSE Stream"
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/agents/{id}/execute-stream [post]
func (h *AgentExecuteHandler) ExecuteStream(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	agentID := c.Param("id")

	var req ExecuteAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	// 生成 TraceID
	traceID := uuid.New().String()

	// 构建输入
	input := &runtime.AgentInput{
		Content:     req.Content,
		Variables:   req.Variables,
		ExtraParams: req.ExtraParams,
		Context: &runtime.AgentContext{
			TenantID:  tenantID,
			UserID:    userID,
			SessionID: req.SessionID,
			TraceID:   &traceID,
		},
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 执行 Agent（流式）
	chunkChan, errChan := h.registry.ExecuteStream(c.Request.Context(), tenantID, agentID, input)

	// 发送流式响应
	c.Stream(func(w io.Writer) bool {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				return false // Channel 已关闭
			}

			// 发送 SSE 数据
			if chunk.Done {
				c.SSEvent("done", gin.H{"done": true})
				return false
			} else {
				c.SSEvent("message", gin.H{"content": chunk.Content})
				return true
			}

		case err, ok := <-errChan:
			if ok && err != nil {
				c.SSEvent("error", gin.H{"error": err.Error()})
			}
			return false
		}
	})
}

// ExecuteByType 根据类型执行 Agent（非流式）
// @Summary 根据类型执行 Agent（同步）
// @Tags Agents
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param type path string true "Agent 类型"
// @Param request body ExecuteAgentRequest true "执行请求"
// @Success 200 {object} AgentExecuteResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/agents/types/{type}/execute [post]
func (h *AgentExecuteHandler) ExecuteByType(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	agentType := c.Param("type")

	var req ExecuteAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	// 获取 Agent
	agent, err := h.registry.GetAgentByType(c.Request.Context(), tenantID, agentType)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	// 生成 TraceID
	traceID := uuid.New().String()

	// 构建输入
	input := &runtime.AgentInput{
		Content:     req.Content,
		Variables:   req.Variables,
		ExtraParams: req.ExtraParams,
		Context: &runtime.AgentContext{
			TenantID:  tenantID,
			UserID:    userID,
			SessionID: req.SessionID,
			TraceID:   &traceID,
		},
	}

	// 执行 Agent
	result, err := agent.Execute(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, AgentExecuteResponse{TraceID: traceID, Result: result})
}

// ExecuteByTypeStream 根据类型执行 Agent（流式）
// @Summary 根据类型执行 Agent（流式）
// @Tags Agents
// @Security BearerAuth
// @Accept json
// @Produce text/event-stream
// @Param type path string true "Agent 类型"
// @Param request body ExecuteAgentRequest true "执行请求"
// @Success 200 {string} string "SSE Stream"
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/agents/types/{type}/execute-stream [post]
func (h *AgentExecuteHandler) ExecuteByTypeStream(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	agentType := c.Param("type")

	var req ExecuteAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	// 获取 Agent
	agent, err := h.registry.GetAgentByType(c.Request.Context(), tenantID, agentType)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	// 生成 TraceID
	traceID := uuid.New().String()

	// 构建输入
	input := &runtime.AgentInput{
		Content:     req.Content,
		Variables:   req.Variables,
		ExtraParams: req.ExtraParams,
		Context: &runtime.AgentContext{
			TenantID:  tenantID,
			UserID:    userID,
			SessionID: req.SessionID,
			TraceID:   &traceID,
		},
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 执行 Agent（流式）
	chunkChan, errChan := agent.ExecuteStream(c.Request.Context(), input)

	// 发送流式响应
	c.Stream(func(w io.Writer) bool {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				return false
			}

			if chunk.Done {
				c.SSEvent("done", gin.H{"done": true})
				return false
			} else {
				c.SSEvent("message", gin.H{"content": chunk.Content})
				return true
			}

		case err, ok := <-errChan:
			if ok && err != nil {
				c.SSEvent("error", gin.H{"error": err.Error()})
			}
			return false
		}
	})
}
