package agents

import (
	"io"
	"net/http"
	"strconv"

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

// ListExecutions 获取Agent执行历史
// @Summary 获取Agent执行历史
// @Description 查询指定Agent的所有执行记录（支持分页）
// @Tags Agents
// @Security BearerAuth
// @Produce json
// @Param id path string true "Agent ID"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param status query string false "状态筛选: pending, running, completed, failed"
// @Success 200 {object} map[string]interface{} "执行历史列表"
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/agents/{id}/executions [get]
func (h *AgentExecuteHandler) ListExecutions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	agentID := c.Param("id")
	
	// 解析分页参数
	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if val, err := strconv.Atoi(ps); err == nil && val > 0 && val <= 100 {
			pageSize = val
		}
	}
	
	status := c.Query("status")
	
	// 权限验证：确保Agent属于当前租户
	agent, err := h.registry.GetAgent(c.Request.Context(), tenantID, agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{
			Success: false,
			Message: "Agent不存在或无权访问",
		})
		return
	}
	
	// 从workflow_executions表查询该Agent的执行记录
	// 这里需要通过工作流定义中的节点类型来匹配Agent
	db := h.registry.DB() // 假设Registry有DB访问方法，如果没有需要传入
	if db == nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Success: false,
			Message: "数据库连接不可用",
		})
		return
	}
	
	// 查询执行记录
	var executions []map[string]interface{}
	var total int64
	
	query := db.WithContext(c.Request.Context()).
		Table("workflow_executions").
		Where("tenant_id = ?", tenantID).
		Where("workflow_id IN (SELECT id FROM workflows WHERE tenant_id = ? AND definition::text LIKE ?)", 
			tenantID, "%"+agent.Type()+"%")
	
	if status != "" {
		query = query.Where("status = ?", status)
	}
	
	// 统计总数
	query.Count(&total)
	
	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Select("id, workflow_id, status, input, output, error_message, started_at, completed_at, created_at").
		Scan(&executions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Success: false,
			Message: "查询执行历史失败: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"executions": executions,
		"total":      total,
		"page":       page,
		"pageSize":   pageSize,
		"agent": gin.H{
			"id":   agentID,
			"name": agent.Name(),
			"type": agent.Type(),
		},
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
