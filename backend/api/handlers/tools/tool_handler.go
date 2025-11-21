package tools

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	response "backend/api/handlers/common"
	"backend/internal/tools"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RegisterToolRequest 注册工具请求
type RegisterToolRequest struct {
	Name        string                `json:"name" binding:"required"`
	DisplayName string                `json:"displayName" binding:"required"`
	Description string                `json:"description" binding:"required"`
	Category    string                `json:"category" binding:"required"`
	Type        string                `json:"type" binding:"required"`
	Parameters  map[string]any        `json:"parameters"`
	HTTPConfig  *tools.HTTPToolConfig `json:"httpConfig"`
	RequireAuth *bool                 `json:"requireAuth"`
	Scopes      []string              `json:"scopes"`
	Timeout     int                   `json:"timeout"`
	MaxRetries  int                   `json:"maxRetries"`
}

// ToolHandler 工具管理 Handler
type ToolHandler struct {
	registry *tools.ToolRegistry
	executor *tools.ToolExecutor
	db       *gorm.DB
}

// NewToolHandler 创建 ToolHandler
func NewToolHandler(registry *tools.ToolRegistry, executor *tools.ToolExecutor, db *gorm.DB) *ToolHandler {
	return &ToolHandler{
		registry: registry,
		executor: executor,
		db:       db,
	}
}

// ListTools 查询工具列表
// @Summary 查询工具列表
// @Tags Tools
// @Security BearerAuth
// @Produce json
// @Param category query string false "工具分类"
// @Success 200 {object} response.APIResponse{data=[]tools.ToolDefinition}
// @Router /api/tools [get]
func (h *ToolHandler) ListTools(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	category := c.Query("category") // 可选：按类别过滤

	var toolsList []*tools.ToolDefinition
	if category != "" {
		toolsList = h.registry.ListByCategory(category)
	} else {
		toolsList = h.registry.List()
	}

	filtered := filterToolsForTenant(toolsList, tenantID)

	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"tools": filtered,
			"count": len(filtered),
		},
	})
}

// GetTool 查询工具详情
// @Summary 获取工具详情
// @Tags Tools
// @Security BearerAuth
// @Produce json
// @Param name path string true "工具名称"
// @Success 200 {object} tools.ToolDefinition
// @Failure 404 {object} response.ErrorResponse
// @Router /api/tools/{name} [get]
func (h *ToolHandler) GetTool(c *gin.Context) {
	name := c.Param("name")
	tenantID := c.GetString("tenant_id")

	definition, exists := h.registry.GetDefinition(name)
	if !exists {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "工具不存在"})
		return
	}
	if !isToolAccessible(definition, tenantID) {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "工具不存在"})
		return
	}

	c.JSON(http.StatusOK, definition)
}

// ExecuteTool 执行工具
// @Summary 执行工具
// @Tags Tools
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param name path string true "工具名称"
// @Param request body toolExecuteRequest true "执行输入"
// @Success 200 {object} toolExecuteResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/tools/{name}/execute [post]
func (h *ToolHandler) ExecuteTool(c *gin.Context) {
	name := c.Param("name")
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	definition, exists := h.registry.GetDefinition(name)
	if !exists || !isToolAccessible(definition, tenantID) {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "工具不存在"})
		return
	}

	var req toolExecuteRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	// 添加 tenant_id 到输入参数（某些工具可能需要）
	if tenantID != "" {
		req.Input["tenant_id"] = tenantID
	}

	// 执行工具
	execReq := &tools.ToolExecutionRequest{
		TenantID: tenantID,
		ToolID:   chooseToolID(definition, name),
		ToolName: name,
		Input:    req.Input,
		AgentID:  userID, // 使用 userID 作为 AgentID
		Timeout:  30,
	}

	result, err := h.executor.Execute(c.Request.Context(), execReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "工具执行失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, toolExecuteResponse{
		ExecutionID: result.ExecutionID,
		ToolName:    result.ToolName,
		Output:      result.Output,
		DurationMs:  result.Duration,
	})
}

// ListExecutions 查询工具执行历史
// @Summary 工具执行列表
// @Tags Tools
// @Security BearerAuth
// @Produce json
// @Param name path string true "工具名称"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param status query string false "执行状态"
// @Success 200 {object} toolsExecutionListResponse
// @Router /api/tools/{name}/executions [get]
func (h *ToolHandler) ListExecutions(c *gin.Context) {
	name := c.Param("name")
	tenantID := c.GetString("tenant_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status") // 可选：按状态过滤

	var executions []tools.ToolExecution
	var total int64

	query := h.db.Where("tool_name = ? AND tenant_id = ?", name, tenantID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Model(&tools.ToolExecution{}).Count(&total)
	query.Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&executions)

	c.JSON(http.StatusOK, toolsExecutionListResponse{
		Executions: executions,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}

// GetExecution 查询单个执行详情
// @Summary 工具执行详情
// @Tags Tools
// @Security BearerAuth
// @Produce json
// @Param id path string true "执行 ID"
// @Success 200 {object} tools.ToolExecution
// @Failure 404 {object} response.ErrorResponse
// @Router /api/tools/executions/{id} [get]
func (h *ToolHandler) GetExecution(c *gin.Context) {
	executionID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	var execution tools.ToolExecution
	if err := h.db.Where("id = ? AND tenant_id = ?", executionID, tenantID).
		First(&execution).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "执行记录不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, execution)
}

// RegisterTool 注册自定义工具
// @Summary 注册自定义工具
// @Tags Tools
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body RegisterToolRequest true "工具定义"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /api/tools/register [post]
func (h *ToolHandler) RegisterTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if strings.TrimSpace(tenantID) == "" {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "缺少租户上下文，无法注册工具"})
		return
	}

	var req RegisterToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	definition, handler, err := h.buildToolDefinition(&req, tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	if err := h.registry.Register(definition.Name, handler, definition); err != nil {
		c.JSON(http.StatusConflict, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response.APIResponse{Success: true, Data: gin.H{"tool": definition}})
}

// UnregisterTool 注销工具
// @Summary 注销工具
// @Tags Tools
// @Security BearerAuth
// @Produce json
// @Param name path string true "工具名称"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/tools/{name} [delete]
func (h *ToolHandler) UnregisterTool(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	name := c.Param("name")
	definition, exists := h.registry.GetDefinition(name)
	if !exists {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "工具不存在"})
		return
	}
	if definition.Type == "builtin" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "内置工具不可注销"})
		return
	}
	if !isToolAccessible(definition, tenantID) {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权操作该工具"})
		return
	}

	h.registry.Unregister(name)
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "工具已注销"})
}

// ListToolsByCategory 按分类列出工具
// @Summary 按分类列出工具
// @Tags Tools
// @Security BearerAuth
// @Produce json
// @Param category path string true "工具分类"
// @Success 200 {object} response.APIResponse{data=[]tools.ToolDefinition}
// @Router /api/tools/categories/{category} [get]
func (h *ToolHandler) ListToolsByCategory(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	category := c.Param("category")
	definitions := h.registry.ListByCategory(category)
	filtered := filterToolsForTenant(definitions, tenantID)
	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"tools": filtered,
			"count": len(filtered),
		},
	})
}

// buildToolDefinition 根据请求构建工具定义和对应 handler
func (h *ToolHandler) buildToolDefinition(req *RegisterToolRequest, tenantID string) (*tools.ToolDefinition, tools.ToolHandler, error) {
	if req == nil {
		return nil, nil, fmt.Errorf("请求不能为空")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, nil, fmt.Errorf("工具名称不能为空")
	}
	typeName := strings.ToLower(req.Type)
	switch typeName {
	case "http_api":
		if req.HTTPConfig == nil {
			return nil, nil, fmt.Errorf("httpConfig 不能为空")
		}
		if strings.TrimSpace(req.HTTPConfig.Method) == "" || strings.TrimSpace(req.HTTPConfig.URL) == "" {
			return nil, nil, fmt.Errorf("HTTP 配置缺少 method 或 url")
		}
		requireAuth := true
		if req.RequireAuth != nil {
			requireAuth = *req.RequireAuth
		}
		definition := &tools.ToolDefinition{
			ID:          uuid.New().String(),
			TenantID:    tenantID,
			Name:        req.Name,
			DisplayName: req.DisplayName,
			Description: req.Description,
			Category:    req.Category,
			Type:        "http_api",
			Parameters:  req.Parameters,
			HTTPConfig:  req.HTTPConfig,
			Scopes:      req.Scopes,
			Timeout:     req.Timeout,
			MaxRetries:  req.MaxRetries,
			RequireAuth: requireAuth,
			Status:      "active",
		}
		if definition.Parameters == nil {
			definition.Parameters = map[string]any{}
		}
		if definition.Timeout == 0 {
			definition.Timeout = 30
		}
		if definition.MaxRetries == 0 {
			definition.MaxRetries = 3
		}
		handler := tools.NewDynamicHTTPTool(definition)
		return definition, handler, nil
	default:
		return nil, nil, fmt.Errorf("暂不支持的工具类型: %s", req.Type)
	}
}

func filterToolsForTenant(defs []*tools.ToolDefinition, tenantID string) []*tools.ToolDefinition {
	if tenantID == "" {
		return defs
	}
	filtered := make([]*tools.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		if isToolAccessible(def, tenantID) {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

func isToolAccessible(def *tools.ToolDefinition, tenantID string) bool {
	if def == nil {
		return false
	}
	return def.TenantID == "" || def.TenantID == tenantID
}

func chooseToolID(def *tools.ToolDefinition, fallback string) string {
	if def != nil && def.ID != "" {
		return def.ID
	}
	return fallback
}

// GetToolsForOpenAI 获取 OpenAI 格式的工具列表
// @Summary OpenAI 工具列表
// @Tags Tools
// @Security BearerAuth
// @Produce json
// @Param category query string false "工具分类"
// @Success 200 {object} response.APIResponse
// @Router /api/tools/openai-format [get]
func (h *ToolHandler) GetToolsForOpenAI(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	category := c.Query("category") // 可选：按类别过滤

	// 如果指定类别，先过滤
	if category != "" {
		filteredTools := h.registry.ListByCategory(category)
		filteredTools = filterToolsForTenant(filteredTools, tenantID)
		// 临时创建一个新注册表来转换
		tempRegistry := tools.NewToolRegistry()
		for _, def := range filteredTools {
			handler, _ := h.registry.Get(def.Name)
			tempRegistry.Register(def.Name, handler, def)
		}
		openaiTools := tempRegistry.ToOpenAITools()
		c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"tools": openaiTools, "count": len(openaiTools)}})
		return
	}

	// 获取所有工具的 OpenAI 格式
	all := filterToolsForTenant(h.registry.List(), tenantID)
	tempRegistry := tools.NewToolRegistry()
	for _, def := range all {
		handler, _ := h.registry.Get(def.Name)
		tempRegistry.Register(def.Name, handler, def)
	}
	openaiTools := tempRegistry.ToOpenAITools()
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"tools": openaiTools, "count": len(openaiTools)}})
}

// toolExecuteRequest 工具执行请求体。
type toolExecuteRequest struct {
	Input map[string]any `json:"input" binding:"required"`
}

// toolExecuteResponse 工具执行响应。
type toolExecuteResponse struct {
	ExecutionID string         `json:"execution_id"`
	ToolName    string         `json:"tool_name"`
	Output      map[string]any `json:"output"`
	DurationMs  int64          `json:"duration_ms"`
}

// toolsExecutionListResponse 工具执行列表响应。
type toolsExecutionListResponse struct {
	Executions []tools.ToolExecution   `json:"executions"`
	Pagination response.PaginationMeta `json:"pagination"`
}
