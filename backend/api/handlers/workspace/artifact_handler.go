package workspace

import (
	"net/http"
	"strconv"

	response "backend/api/handlers/common"
	auditpkg "backend/internal/audit"
	workspaceSvc "backend/internal/workspace"

	"github.com/gin-gonic/gin"
)

// ArtifactHandler 智能体产出物管理 API
type ArtifactHandler struct {
	svc *workspaceSvc.Service
}

// NewArtifactHandler 构造函数
func NewArtifactHandler(svc *workspaceSvc.Service) *ArtifactHandler {
	return &ArtifactHandler{svc: svc}
}

// ============================================
// 智能体工作空间 API
// ============================================

// GetAgentWorkspace 获取智能体工作空间
// @Summary 获取智能体工作空间信息
// @Tags AgentWorkspace
// @Security BearerAuth
// @Param agentId path string true "智能体ID"
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/workspace/agents/{agentId} [get]
func (h *ArtifactHandler) GetAgentWorkspace(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	agentID := c.Param("agentId")

	workspace, err := h.svc.GetAgentWorkspace(c.Request.Context(), tenantID, agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	if workspace == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "智能体工作空间不存在"})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: workspace})
}

type ensureAgentWorkspaceDTO struct {
	AgentID   string `json:"agentId" binding:"required"`
	AgentName string `json:"agentName" binding:"required"`
}

// EnsureAgentWorkspace 确保智能体工作空间存在
// @Summary 创建或获取智能体工作空间
// @Tags AgentWorkspace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body ensureAgentWorkspaceDTO true "请求体"
// @Success 200 {object} response.APIResponse
// @Router /api/workspace/agents [post]
func (h *ArtifactHandler) EnsureAgentWorkspace(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var dto ensureAgentWorkspaceDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	workspace, err := h.svc.EnsureAgentWorkspace(c.Request.Context(), tenantID, dto.AgentID, dto.AgentName, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	auditpkg.SetAuditResourceInfo(c, "agent_workspace", workspace.ID)
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: workspace})
}

// ============================================
// 智能体产出物 API
// ============================================

type createArtifactDTO struct {
	AgentID   string `json:"agentId" binding:"required"`
	AgentName string `json:"agentName" binding:"required"`
	SessionID string `json:"sessionId"`
	TaskType  string `json:"taskType" binding:"required"`
	TitleHint string `json:"titleHint"`
	Content   string `json:"content" binding:"required"`
	Summary   string `json:"summary"`
	ToolName  string `json:"toolName"`
	Metadata  string `json:"metadata"`
}

// CreateArtifact 创建智能体产出物
// @Summary 创建智能体产出物
// @Tags AgentArtifact
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body createArtifactDTO true "请求体"
// @Success 201 {object} response.APIResponse
// @Router /api/workspace/artifacts [post]
func (h *ArtifactHandler) CreateArtifact(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var dto createArtifactDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	result, err := h.svc.CreateAgentArtifact(c.Request.Context(), &workspaceSvc.CreateArtifactRequest{
		TenantID:  tenantID,
		AgentID:   dto.AgentID,
		AgentName: dto.AgentName,
		SessionID: dto.SessionID,
		TaskType:  workspaceSvc.ArtifactType(dto.TaskType),
		TitleHint: dto.TitleHint,
		Content:   dto.Content,
		Summary:   dto.Summary,
		ToolName:  dto.ToolName,
		Metadata:  dto.Metadata,
		UserID:    userID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	auditpkg.SetAuditResourceInfo(c, "agent_artifact", result.Artifact.ID)
	auditpkg.SetAuditChanges(c, map[string]any{
		"agentName": dto.AgentName,
		"taskType":  dto.TaskType,
		"filePath":  result.PathInfo.FullPath,
	})

	c.JSON(http.StatusCreated, response.APIResponse{
		Success: true,
		Data: gin.H{
			"artifact": result.Artifact,
			"node":     result.Node,
			"pathInfo": result.PathInfo,
		},
	})
}

// ListAgentArtifacts 列出智能体产出物
// @Summary 列出指定智能体的产出物
// @Tags AgentArtifact
// @Security BearerAuth
// @Param agentId path string true "智能体ID"
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/workspace/agents/{agentId}/artifacts [get]
func (h *ArtifactHandler) ListAgentArtifacts(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	agentID := c.Param("agentId")

	limit := 20
	offset := 0
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}
	if v := c.Query("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			offset = parsed
		}
	}

	artifacts, total, err := h.svc.ListAgentArtifacts(c.Request.Context(), tenantID, agentID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"items":  artifacts,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// ListSessionArtifacts 列出会话产出物
// @Summary 列出指定会话的产出物
// @Tags AgentArtifact
// @Security BearerAuth
// @Param sessionId path string true "会话ID"
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/workspace/sessions/{sessionId}/artifacts [get]
func (h *ArtifactHandler) ListSessionArtifacts(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	sessionID := c.Param("sessionId")

	limit := 20
	offset := 0
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}
	if v := c.Query("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			offset = parsed
		}
	}

	artifacts, total, err := h.svc.ListSessionArtifacts(c.Request.Context(), tenantID, sessionID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"items":  artifacts,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// ============================================
// 会话工作空间 API
// ============================================

type ensureSessionWorkspaceDTO struct {
	SessionID string `json:"sessionId" binding:"required"`
}

// EnsureSessionWorkspace 确保会话工作空间存在
// @Summary 创建或获取会话工作空间
// @Tags SessionWorkspace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body ensureSessionWorkspaceDTO true "请求体"
// @Success 200 {object} response.APIResponse
// @Router /api/workspace/sessions [post]
func (h *ArtifactHandler) EnsureSessionWorkspace(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var dto ensureSessionWorkspaceDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	workspace, err := h.svc.EnsureSessionWorkspace(c.Request.Context(), tenantID, dto.SessionID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	auditpkg.SetAuditResourceInfo(c, "session_workspace", workspace.ID)
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: workspace})
}

// GetArtifactTypes 获取支持的产出物类型
// @Summary 获取支持的产出物类型列表
// @Tags AgentArtifact
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/workspace/artifact-types [get]
func (h *ArtifactHandler) GetArtifactTypes(c *gin.Context) {
	types := []gin.H{
		{"value": "outline", "label": "大纲", "ext": ".md"},
		{"value": "draft", "label": "草稿", "ext": ".md"},
		{"value": "research", "label": "研究", "ext": ".md"},
		{"value": "analysis", "label": "分析", "ext": ".md"},
		{"value": "report", "label": "报告", "ext": ".md"},
		{"value": "code", "label": "代码", "ext": "auto"},
		{"value": "data", "label": "数据", "ext": "auto"},
		{"value": "other", "label": "其他", "ext": ".md"},
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: types})
}
