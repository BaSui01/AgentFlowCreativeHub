package workspace

import (
	"errors"
	"net/http"
	"strings"

	response "backend/api/handlers/common"
	auditpkg "backend/internal/audit"
	"backend/internal/agent/runtime"
	"backend/internal/tools"
	workspaceSvc "backend/internal/workspace"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler 提供工作区相关 API
type Handler struct {
	svc           *workspaceSvc.Service
	toolExecutor  *tools.ToolExecutor
	agentRegistry *runtime.Registry
}

// NewHandler 构造函数
func NewHandler(svc *workspaceSvc.Service, executor *tools.ToolExecutor, registry *runtime.Registry) *Handler {
	return &Handler{svc: svc, toolExecutor: executor, agentRegistry: registry}
}

// GetTree 工作区树
// @Summary 获取工作区树
// @Tags Workspace
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/workspace/tree [get]
func (h *Handler) GetTree(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if err := h.svc.EnsureDefaults(c.Request.Context(), tenantID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	nodes, err := h.svc.ListTree(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"nodes": nodes}})
}

type createFolderDTO struct {
	Name     string  `json:"name" binding:"required,min=1,max=100"`
	ParentID *string `json:"parentId"`
	Category string  `json:"category"`
}

// CreateFolder 新建目录
func (h *Handler) CreateFolder(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	_ = h.svc.EnsureDefaults(c.Request.Context(), tenantID, userID)
	var dto createFolderDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	node, err := h.svc.CreateFolder(c.Request.Context(), &workspaceSvc.CreateFolderRequest{
		TenantID: tenantID,
		ParentID: dto.ParentID,
		Name:     dto.Name,
		Category: dto.Category,
		UserID:   userID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response.APIResponse{Success: true, Data: node})
}

type renameNodeDTO struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

// RenameNode 重命名节点
func (h *Handler) RenameNode(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	nodeID := c.Param("id")
	var dto renameNodeDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	node, err := h.svc.UpdateNodeName(c.Request.Context(), tenantID, nodeID, dto.Name, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: node})
}

// DeleteNode 删除节点
func (h *Handler) DeleteNode(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	nodeID := c.Param("id")
	if err := h.svc.DeleteNode(c.Request.Context(), tenantID, nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "删除成功"})
}

// GetFile 获取文件详情
func (h *Handler) GetFile(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	nodeID := c.Param("id")
	file, err := h.svc.GetFileDetail(c.Request.Context(), tenantID, nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: file})
}

type updateFileDTO struct {
	Content  string `json:"content" binding:"required"`
	Summary  string `json:"summary"`
	AgentID  string `json:"agentId"`
	ToolName string `json:"toolName"`
	Metadata string `json:"metadata"`
}

// UpdateFile 更新文件
func (h *Handler) UpdateFile(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	nodeID := c.Param("id")
	var dto updateFileDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	res, err := h.svc.UpdateFileContent(c.Request.Context(), &workspaceSvc.UpdateFileRequest{
		TenantID: tenantID,
		NodeID:   nodeID,
		Content:  dto.Content,
		Summary:  dto.Summary,
		AgentID:  dto.AgentID,
		ToolName: dto.ToolName,
		Metadata: dto.Metadata,
		UserID:   userID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: res})
}

// ListStaging 暂存列表
func (h *Handler) ListStaging(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	status := c.Query("status")
	items, err := h.svc.ListStagingFiles(c.Request.Context(), &workspaceSvc.ListStagingRequest{TenantID: tenantID, Status: status})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"items": items}})
}

type createStagingDTO struct {
	FileType     string         `json:"fileType" binding:"required"`
	Content      string         `json:"content" binding:"required"`
	TitleHint    string         `json:"titleHint"`
	Summary      string         `json:"summary"`
	AgentID      string         `json:"agentId"`
	AgentName    string         `json:"agentName"`
	Command      string         `json:"command"`
	Metadata     string         `json:"metadata"`
	ManualFolder string         `json:"manualFolder"`
	RequiresSecondary bool      `json:"requiresSecondary"`
}

// CreateStaging 创建暂存
func (h *Handler) CreateStaging(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	_ = h.svc.EnsureDefaults(c.Request.Context(), tenantID, userID)
	var dto createStagingDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	staging, err := h.svc.CreateStagingFile(c.Request.Context(), &workspaceSvc.CreateStagingRequest{
		TenantID:     tenantID,
		FileType:     dto.FileType,
		Content:      dto.Content,
		TitleHint:    dto.TitleHint,
		Summary:      dto.Summary,
		AgentID:      dto.AgentID,
		AgentName:    dto.AgentName,
		Command:      dto.Command,
		Metadata:     dto.Metadata,
		CreatedBy:    userID,
		ManualFolder: dto.ManualFolder,
		RequiresSecondary: dto.RequiresSecondary,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	auditpkg.SetAuditResourceInfo(c, "workspace_staging", staging.ID)
	auditpkg.SetAuditChanges(c, map[string]any{"fileType": staging.FileType})
	c.JSON(http.StatusCreated, response.APIResponse{Success: true, Data: staging})
}

type reviewStagingDTO struct {
	Action      string `json:"action" binding:"required,oneof=approve reject request_changes"`
	Reason      string `json:"reason"`
	ReviewToken string `json:"reviewToken" binding:"required"`
}

// ReviewStaging 审核处理
func (h *Handler) ReviewStaging(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	stagingID := c.Param("id")
	var dto reviewStagingDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	result, err := h.svc.ReviewStagingFile(c.Request.Context(), &workspaceSvc.ReviewStagingRequest{
		TenantID:    tenantID,
		StagingID:   stagingID,
		ReviewerID:  userID,
		Action:      workspaceSvc.ReviewAction(dto.Action),
		Reason:      dto.Reason,
		ReviewToken: dto.ReviewToken,
	})
	if err != nil {
		if writeStagingError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	auditpkg.SetAuditResourceInfo(c, "workspace_staging", stagingID)
	auditpkg.SetAuditChanges(c, map[string]any{"action": dto.Action, "status": result.Status})
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: result})
}

type attachContextDTO struct {
	AgentID   string   `json:"agentId" binding:"required"`
	SessionID string   `json:"sessionId"`
	NodeIDs   []string `json:"nodeIds"`
	Mentions  []string `json:"mentions"`
	Commands  []string `json:"commands"`
	Notes     string   `json:"notes"`
}

// AttachContext 通过命令注入上下文
func (h *Handler) AttachContext(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	var dto attachContextDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	sessionID := dto.SessionID
	ctxMgr := h.agentRegistry.GetContextManager()
	if sessionID == "" {
		sessionID = uuid.New().String()
		if _, err := ctxMgr.CreateSession(c.Request.Context(), tenantID, userID, sessionID); err != nil {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
			return
		}
	} else {
		if _, err := ctxMgr.GetOrCreateSession(c.Request.Context(), tenantID, userID, sessionID); err != nil {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
			return
		}
	}
	contextNodes, err := h.svc.LoadContextNodes(c.Request.Context(), tenantID, dto.NodeIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	snapshot := buildSnapshot(dto, contextNodes)
	if err := ctxMgr.AddMessage(c.Request.Context(), sessionID, "system", snapshot); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	if _, err := h.svc.CreateContextLink(c.Request.Context(), &workspaceSvc.ContextLinkRequest{
		TenantID:  tenantID,
		AgentID:   dto.AgentID,
		SessionID: sessionID,
		NodeIDs:   dto.NodeIDs,
		Mentions:  dto.Mentions,
		Commands:  dto.Commands,
		Notes:     dto.Notes,
		UserID:    userID,
	}, snapshot); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"sessionId": sessionID}})
}

func buildSnapshot(dto attachContextDTO, nodes []*workspaceSvc.ContextNode) string {
	var parts []string
	parts = append(parts, "【工作区上下文注入】")
	if len(dto.Mentions) > 0 {
		parts = append(parts, "@ 指令:"+strings.Join(dto.Mentions, ","))
	}
	if len(dto.Commands) > 0 {
		parts = append(parts, " / 命令:"+strings.Join(dto.Commands, ","))
	}
	for _, cn := range nodes {
		line := "- " + cn.Node.Name + " (" + cn.Node.NodePath + ")"
		parts = append(parts, line)
		if cn.Version != nil && strings.TrimSpace(cn.Version.Summary) != "" {
			parts = append(parts, "  摘要: "+trimSnippet(cn.Version.Summary))
		}
	}
	if dto.Notes != "" {
		parts = append(parts, "备注: "+dto.Notes)
	}
	return strings.Join(parts, "\n")
}

func writeStagingError(c *gin.Context, err error) bool {
	var stgErr *workspaceSvc.StagingError
	if errors.As(err, &stgErr) {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Code: stgErr.Code, Message: stgErr.Message})
		return true
	}
	return false
}

func trimSnippet(text string) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= 160 {
		return text
	}
	runes := []rune(text)
	return string(runes[:160]) + "..."
}
