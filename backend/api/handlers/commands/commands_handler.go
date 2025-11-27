package commands

import (
	"context"
	"net/http"
	"strconv"

	response "backend/api/handlers/common"
	"backend/internal/agent/runtime"
	auditpkg "backend/internal/audit"
	"backend/internal/command"

	"github.com/gin-gonic/gin"
)

// Handler 命令执行 API
type Handler struct {
	service *command.Service
	queue   agentRunClient
}

type agentRunClient interface {
	EnqueueAgentRun(ctx context.Context, payload *runtime.AgentRunPayload) (string, error)
}

// NewHandler 构造函数
func NewHandler(service *command.Service, queue agentRunClient) *Handler {
	return &Handler{service: service, queue: queue}
}

type executeCommandDTO struct {
	AgentID        string   `json:"agentId" binding:"required"`
	CommandType    string   `json:"commandType"`
	Content        string   `json:"content" binding:"required"`
	ContextNodeIDs []string `json:"contextNodeIds"`
	SessionID      string   `json:"sessionId"`
	Notes          string   `json:"notes"`
	DeadlineMs     int64    `json:"deadlineMs"`
}

// Execute 提交命令
func (h *Handler) Execute(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, response.ErrorResponse{Success: false, Message: "命令服务未启用"})
		return
	}
	var dto executeCommandDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	res, err := h.service.ExecuteCommand(c.Request.Context(), &command.ExecuteCommandInput{
		TenantID:       c.GetString("tenant_id"),
		UserID:         c.GetString("user_id"),
		AgentID:        dto.AgentID,
		CommandType:    dto.CommandType,
		Content:        dto.Content,
		SessionID:      dto.SessionID,
		ContextNodeIDs: dto.ContextNodeIDs,
		Notes:          dto.Notes,
		DeadlineMs:     dto.DeadlineMs,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	auditpkg.SetAuditResourceInfo(c, "command_request", res.Request.ID)
	auditpkg.SetAuditChanges(c, map[string]any{"agentId": res.Request.AgentID, "status": res.Request.Status})
	auditpkg.SetAuditMetadata(c, "action", "commands.execute")
	auditpkg.SetAuditMetadata(c, "result_status", res.Request.Status)
	auditpkg.SetAuditMetadata(c, "queue_position", res.Request.QueuePosition)
	if res.NewlyCreated && h.queue != nil {
		payload := &runtime.AgentRunPayload{
			AgentID:     res.Request.AgentID,
			Input:       dto.Content,
			Variables:   map[string]any{"command_type": res.Request.CommandType},
			ContextData: map[string]any{"context_snapshot": res.Request.ContextSnapshot, "notes": res.Request.Notes, "context_revision_id": res.Request.ContextRevision},
			TraceID:     res.Request.TraceID,
			UserID:      c.GetString("user_id"),
			TenantID:    c.GetString("tenant_id"),
			CommandID:   res.Request.ID,
			DeadlineMs:  dto.DeadlineMs,
		}
		if dto.SessionID != "" {
			payload.ContextData["session_id"] = dto.SessionID
		}
		if _, err := h.queue.EnqueueAgentRun(c.Request.Context(), payload); err != nil {
			h.service.MarkFailed(c.Request.Context(), res.Request.ID, "入队失败:"+err.Error())
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "命令入队失败: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusAccepted, response.APIResponse{Success: true, Data: res})
}

// Get 查询命令
func (h *Handler) Get(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, response.ErrorResponse{Success: false, Message: "命令服务未启用"})
		return
	}
	id := c.Param("id")
	req, err := h.service.GetCommand(c.Request.Context(), c.GetString("tenant_id"), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	if req == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "命令不存在"})
		return
	}
	auditpkg.SetAuditResourceInfo(c, "command_request", req.ID)
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: req})
}

// List 返回命令列表（带筛选与分页）。
func (h *Handler) List(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, response.ErrorResponse{Success: false, Message: "命令服务未启用"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	params := command.ListCommandsParams{
		Status:  c.Query("status"),
		AgentID: c.Query("agentId"),
		Limit:   limit,
		Offset:  (page - 1) * limit,
	}
	items, total, err := h.service.ListCommands(c.Request.Context(), c.GetString("tenant_id"), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	totalPages := 0
	if limit > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	c.JSON(http.StatusOK, response.ListResponse{
		Items: items,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  limit,
			Total:     total,
			TotalPage: totalPages,
		},
	})
}
