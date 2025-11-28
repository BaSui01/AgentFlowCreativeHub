package workspace

import (
	"net/http"

	"backend/api/handlers/common"
	"backend/internal/workspace"

	"github.com/gin-gonic/gin"
)

// TemplateHandler 工作空间模板处理器
type TemplateHandler struct {
	service *workspace.TemplateService
}

// NewTemplateHandler 创建模板处理器
func NewTemplateHandler(service *workspace.TemplateService) *TemplateHandler {
	return &TemplateHandler{service: service}
}

// CreateTemplate 创建模板
// @Summary 创建工作空间模板
// @Description 创建自定义工作空间模板
// @Tags WorkspaceTemplate
// @Accept json
// @Produce json
// @Param request body workspace.CreateTemplateRequest true "创建请求"
// @Success 200 {object} common.Response{data=workspace.WorkspaceTemplate}
// @Router /api/workspace/templates [post]
func (h *TemplateHandler) CreateTemplate(c *gin.Context) {
	var req workspace.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	template, err := h.service.CreateTemplate(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "创建模板失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: template})
}

// GetTemplate 获取模板详情
// @Summary 获取模板详情
// @Description 根据ID获取模板详情
// @Tags WorkspaceTemplate
// @Produce json
// @Param id path string true "模板ID"
// @Success 200 {object} common.Response{data=workspace.WorkspaceTemplate}
// @Router /api/workspace/templates/{id} [get]
func (h *TemplateHandler) GetTemplate(c *gin.Context) {
	templateID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	template, err := h.service.GetTemplate(c.Request.Context(), tenantID, templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "模板不存在: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: template})
}

// ListTemplates 查询模板列表
// @Summary 查询模板列表
// @Description 查询可用的工作空间模板列表
// @Tags WorkspaceTemplate
// @Produce json
// @Param type query string false "模板类型" Enums(novel, script, article, project, custom)
// @Param keyword query string false "关键词搜索"
// @Param builtin query bool false "是否内置模板"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} common.PagedResponse{data=[]workspace.WorkspaceTemplate}
// @Router /api/workspace/templates [get]
func (h *TemplateHandler) ListTemplates(c *gin.Context) {
	var req workspace.ListTemplatesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")

	templates, total, err := h.service.ListTemplates(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "查询模板失败: " + err.Error()})
		return
	}

	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	totalPage := (int(total) + pageSize - 1) / pageSize

	c.JSON(http.StatusOK, common.ListResponse{
		Items: templates,
		Pagination: common.PaginationMeta{
			Page:      req.Page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// UpdateTemplate 更新模板
// @Summary 更新模板
// @Description 更新自定义模板（只能更新自己创建的模板）
// @Tags WorkspaceTemplate
// @Accept json
// @Produce json
// @Param id path string true "模板ID"
// @Param request body workspace.CreateTemplateRequest true "更新请求"
// @Success 200 {object} common.Response{data=workspace.WorkspaceTemplate}
// @Router /api/workspace/templates/{id} [put]
func (h *TemplateHandler) UpdateTemplate(c *gin.Context) {
	templateID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	var req workspace.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	template, err := h.service.UpdateTemplate(c.Request.Context(), tenantID, templateID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "更新模板失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: template})
}

// DeleteTemplate 删除模板
// @Summary 删除模板
// @Description 删除自定义模板（只能删除自己创建的非内置模板）
// @Tags WorkspaceTemplate
// @Produce json
// @Param id path string true "模板ID"
// @Success 200 {object} common.Response
// @Router /api/workspace/templates/{id} [delete]
func (h *TemplateHandler) DeleteTemplate(c *gin.Context) {
	templateID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	if err := h.service.DeleteTemplate(c.Request.Context(), tenantID, templateID); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "删除模板失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// ApplyTemplate 应用模板
// @Summary 应用模板创建工作空间
// @Description 使用模板创建工作空间目录结构
// @Tags WorkspaceTemplate
// @Accept json
// @Produce json
// @Param request body workspace.ApplyTemplateRequest true "应用请求"
// @Success 200 {object} common.Response{data=workspace.WorkspaceNode}
// @Router /api/workspace/templates/apply [post]
func (h *TemplateHandler) ApplyTemplate(c *gin.Context) {
	var req workspace.ApplyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	rootFolder, err := h.service.ApplyTemplate(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "应用模板失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: rootFolder})
}
