package knowledge

import (
	"net/http"

	response "backend/api/handlers/common"
	auditpkg "backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/models"
	"backend/pkg/types"

	"github.com/gin-gonic/gin"
)

// KBHandler 知识库处理器
type KBHandler struct {
	kbService *models.KnowledgeBaseService
}

// NewKBHandler 创建知识库处理器
func NewKBHandler(kbService *models.KnowledgeBaseService) *KBHandler {
	return &KBHandler{
		kbService: kbService,
	}
}

// CreateRequest 创建知识库请求
type CreateRequest struct {
	Name        string                 `json:"name" binding:"required,min=1,max=200"`
	Description string                 `json:"description"`
	Type        string                 `json:"type" binding:"required,oneof=document url api database"`
	Config      map[string]interface{} `json:"config"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Create 创建知识库
// @Summary 创建知识库
// @Tags KnowledgeBase
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateRequest true "知识库信息"
// @Success 201 {object} models.KnowledgeBase
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/knowledge-bases [post]
func (h *KBHandler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	// 获取用户上下文
	userCtx, _ := auth.GetUserContext(c)

	kb := &models.KnowledgeBase{
		TenantID:    userCtx.TenantID,
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Status:      "active",
		Config:      req.Config,
		Metadata:    req.Metadata,
		CreatedBy:   userCtx.UserID,
		UpdatedBy:   userCtx.UserID,
	}

	if err := h.kbService.CreateKnowledgeBase(c.Request.Context(), kb); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "创建知识库失败: " + err.Error()})
		return
	}

	// 设置审计信息
	auditpkg.SetAuditResourceInfo(c, "knowledge_base", kb.ID)

	c.JSON(http.StatusCreated, kb)
}

// List 列出知识库
// @Summary 知识库列表
// @Tags KnowledgeBase
// @Security BearerAuth
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.ListResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/knowledge-bases [get]
func (h *KBHandler) List(c *gin.Context) {
	// 获取用户上下文
	userCtx, _ := auth.GetUserContext(c)

	// 分页参数
	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if parsed := parseInt(p); parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed := parseInt(ps); parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	pagination := &types.PaginationRequest{
		Page:     page,
		PageSize: pageSize,
	}

	kbs, paginationResp, err := h.kbService.ListKnowledgeBases(c.Request.Context(), userCtx.TenantID, pagination)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询知识库失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.ListResponse{
		Items:      kbs,
		Pagination: toPaginationMeta(paginationResp),
	})
}

// Get 获取知识库详情
// @Summary 知识库详情
// @Tags KnowledgeBase
// @Security BearerAuth
// @Produce json
// @Param id path string true "知识库 ID"
// @Success 200 {object} models.KnowledgeBase
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/knowledge-bases/{id} [get]
func (h *KBHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}

	kb, err := h.kbService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询知识库失败: " + err.Error()})
		return
	}

	if kb == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "知识库不存在"})
		return
	}

	// 检查权限
	userCtx, _ := auth.GetUserContext(c)
	if kb.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	c.JSON(http.StatusOK, kb)
}

// UpdateRequest 更新知识库请求
type UpdateRequest struct {
	Name        string                 `json:"name" binding:"omitempty,min=1,max=200"`
	Description string                 `json:"description"`
	Status      string                 `json:"status" binding:"omitempty,oneof=active inactive"`
	Config      map[string]interface{} `json:"config"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Update 更新知识库
// @Summary 更新知识库
// @Tags KnowledgeBase
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "知识库 ID"
// @Param request body UpdateRequest true "更新内容"
// @Success 200 {object} models.KnowledgeBase
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/knowledge-bases/{id} [put]
func (h *KBHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}

	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	// 获取现有知识库
	kb, err := h.kbService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询知识库失败: " + err.Error()})
		return
	}

	if kb == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "知识库不存在"})
		return
	}

	// 检查权限
	userCtx, _ := auth.GetUserContext(c)
	if kb.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	// 更新字段
	if req.Name != "" {
		kb.Name = req.Name
	}
	if req.Description != "" {
		kb.Description = req.Description
	}
	if req.Status != "" {
		kb.Status = req.Status
	}
	if req.Config != nil {
		kb.Config = req.Config
	}
	if req.Metadata != nil {
		kb.Metadata = req.Metadata
	}
	kb.UpdatedBy = userCtx.UserID

	if err := h.kbService.UpdateKnowledgeBase(c.Request.Context(), kb); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "更新知识库失败: " + err.Error()})
		return
	}

	// 设置审计信息
	auditpkg.SetAuditResourceInfo(c, "knowledge_base", kb.ID)

	c.JSON(http.StatusOK, kb)
}

// Delete 删除知识库
// @Summary 删除知识库
// @Tags KnowledgeBase
// @Security BearerAuth
// @Produce json
// @Param id path string true "知识库 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/knowledge-bases/{id} [delete]
func (h *KBHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}

	// 获取现有知识库
	kb, err := h.kbService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询知识库失败: " + err.Error()})
		return
	}

	if kb == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "知识库不存在"})
		return
	}

	// 检查权限
	userCtx, _ := auth.GetUserContext(c)
	if kb.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	if err := h.kbService.DeleteKnowledgeBase(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "删除知识库失败: " + err.Error()})
		return
	}

	// 设置审计信息
	auditpkg.SetAuditResourceInfo(c, "knowledge_base", id)

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "删除成功"})
}

// parseInt 解析整数
func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return n
}
