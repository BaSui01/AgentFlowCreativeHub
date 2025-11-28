package templates

import (
	"net/http"
	"strconv"

	"backend/internal/template"

	"github.com/gin-gonic/gin"
)

// TemplateHandler Prompt 模板管理 Handler
type TemplateHandler struct {
	service *template.TemplateService
}

// NewTemplateHandler 创建 TemplateHandler 实例
func NewTemplateHandler(service *template.TemplateService) *TemplateHandler {
	return &TemplateHandler{service: service}
}

// ListTemplates 查询模板列表
// @Summary 查询模板列表
// @Description 获取Prompt模板列表
// @Tags Templates
// @Produce json
// @Param category query string false "分类"
// @Param visibility query string false "可见性"
// @Param created_by query string false "创建者"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/templates [get]
func (h *TemplateHandler) ListTemplates(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	req := &template.ListTemplatesRequest{
		TenantID:   tenantID,
		Category:   c.Query("category"),
		Visibility: c.Query("visibility"),
		CreatedBy:  c.Query("created_by"),
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

	resp, err := h.service.ListTemplates(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetTemplate 查询单个模板
// @Summary 获取模板详情
// @Description 获取单个Prompt模板
// @Tags Templates
// @Produce json
// @Param id path string true "模板ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]string
// @Router /api/templates/{id} [get]
func (h *TemplateHandler) GetTemplate(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	templateID := c.Param("id")

	tmpl, err := h.service.GetTemplate(c.Request.Context(), tenantID, templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, tmpl)
}

// CreateTemplate 创建模板
// @Summary 创建模板
// @Description 创建新的Prompt模板
// @Tags Templates
// @Accept json
// @Produce json
// @Param request body template.CreateTemplateRequest true "模板信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Router /api/templates [post]
func (h *TemplateHandler) CreateTemplate(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var req template.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	req.TenantID = tenantID
	req.CreatedBy = userID

	tmpl, err := h.service.CreateTemplate(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, tmpl)
}

// UpdateTemplate 更新模板
// @Summary 更新模板
// @Description 更新Prompt模板
// @Tags Templates
// @Accept json
// @Produce json
// @Param id path string true "模板ID"
// @Param request body template.UpdateTemplateRequest true "更新信息"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Router /api/templates/{id} [put]
func (h *TemplateHandler) UpdateTemplate(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	templateID := c.Param("id")

	var req template.UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	tmpl, err := h.service.UpdateTemplate(c.Request.Context(), tenantID, templateID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, tmpl)
}

// DeleteTemplate 删除模板
// @Summary 删除模板
// @Description 删除Prompt模板
// @Tags Templates
// @Produce json
// @Param id path string true "模板ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/templates/{id} [delete]
func (h *TemplateHandler) DeleteTemplate(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	templateID := c.Param("id")
	operatorID := c.GetString("user_id")

	if err := h.service.DeleteTemplate(c.Request.Context(), tenantID, templateID, operatorID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "模板删除成功",
	})
}

// CreateVersion 创建模板版本
// @Summary 创建模板版本
// @Description 创建模板新版本
// @Tags Templates
// @Accept json
// @Produce json
// @Param id path string true "模板ID"
// @Param request body template.CreateVersionRequest true "版本信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Router /api/templates/{id}/versions [post]
func (h *TemplateHandler) CreateVersion(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	templateID := c.Param("id")
	userID := c.GetString("user_id")

	var req template.CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	req.TemplateID = templateID
	req.CreatedBy = userID

	version, err := h.service.CreateVersion(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, version)
}

// GetLatestVersion 获取最新版本
// @Summary 获取最新版本
// @Description 获取模板的最新版本
// @Tags Templates
// @Produce json
// @Param id path string true "模板ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]string
// @Router /api/templates/{id}/versions/latest [get]
func (h *TemplateHandler) GetLatestVersion(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	templateID := c.Param("id")

	version, err := h.service.GetLatestVersion(c.Request.Context(), tenantID, templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, version)
}

// RenderTemplate 渲染模板
// @Summary 渲染模板
// @Description 使用变量渲染模板
// @Tags Templates
// @Accept json
// @Produce json
// @Param id path string true "模板ID"
// @Param request body template.RenderTemplateRequest true "变量信息"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/templates/{id}/render [post]
func (h *TemplateHandler) RenderTemplate(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	templateID := c.Param("id")

	var req template.RenderTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	req.TemplateID = templateID

	rendered, err := h.service.RenderTemplate(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rendered": rendered,
	})
}
