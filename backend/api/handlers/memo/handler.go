package memo

import (
	"net/http"

	"backend/internal/auth"
	"backend/internal/memo"

	"github.com/gin-gonic/gin"
)

// Handler 备忘录 Handler
type Handler struct {
	service *memo.MemoService
}

// NewHandler 创建 Handler
func NewHandler(service *memo.MemoService) *Handler {
	return &Handler{service: service}
}

// getUserID 获取当前用户ID
func getUserID(c *gin.Context) (string, bool) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		return "", false
	}
	return userCtx.UserID, true
}

// Create 创建备忘录
// @Summary 创建备忘录
// @Description 创建新的备忘录
// @Tags Memo
// @Accept json
// @Produce json
// @Param request body memo.CreateMemoRequest true "备忘录信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/memos [post]
func (h *Handler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req memo.CreateMemoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m := &memo.Memo{
		UserID:   userID,
		Title:    req.Title,
		Content:  req.Content,
		Category: req.Category,
		Tags:     req.Tags,
		Priority: req.Priority,
		DueDate:  req.DueDate,
		Reminder: req.Reminder,
		Color:    req.Color,
		Metadata: req.Metadata,
	}

	if err := h.service.Create(c.Request.Context(), m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"memo": m})
}

// List 列出备忘录
// @Summary 列出备忘录
// @Description 获取用户的备忘录列表
// @Tags Memo
// @Produce json
// @Param category query string false "分类过滤"
// @Param status query string false "状态过滤"
// @Param priority query string false "优先级过滤"
// @Param pinned query bool false "是否置顶"
// @Param archived query bool false "是否已归档"
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Router /api/memos [get]
func (h *Handler) List(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	filter := memo.MemoFilter{
		UserID:   userID,
		Category: c.Query("category"),
		Status:   memo.MemoStatus(c.Query("status")),
		Priority: memo.MemoPriority(c.Query("priority")),
	}

	if c.Query("pinned") == "true" {
		pinned := true
		filter.IsPinned = &pinned
	}
	if c.Query("archived") == "true" {
		archived := true
		filter.IsArchived = &archived
	}

	memos, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"memos": memos,
		"total": len(memos),
	})
}

// Get 获取备忘录详情
// @Summary 获取备忘录详情
// @Description 根据ID获取备忘录详情
// @Tags Memo
// @Produce json
// @Param id path string true "备忘录ID"
// @Success 200 {object} map[string]any
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/memos/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	id := c.Param("id")

	m, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备忘录不存在"})
		return
	}

	// 验证所有权
	if m.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memo": m})
}

// Update 更新备忘录
// @Summary 更新备忘录
// @Description 更新指定备忘录
// @Tags Memo
// @Accept json
// @Produce json
// @Param id path string true "备忘录ID"
// @Param request body memo.UpdateMemoRequest true "更新信息"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/memos/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	id := c.Param("id")

	// 先获取验证所有权
	m, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备忘录不存在"})
		return
	}
	if m.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
		return
	}

	var req memo.UpdateMemoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 应用更新
	if req.Title != nil {
		m.Title = *req.Title
	}
	if req.Content != nil {
		m.Content = *req.Content
	}
	if req.Category != nil {
		m.Category = *req.Category
	}
	if req.Tags != nil {
		m.Tags = req.Tags
	}
	if req.Priority != nil {
		m.Priority = *req.Priority
	}
	if req.DueDate != nil {
		m.DueDate = req.DueDate
	}
	if req.Reminder != nil {
		m.Reminder = req.Reminder
	}
	if req.Color != nil {
		m.Color = *req.Color
	}

	if err := h.service.Update(c.Request.Context(), m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"memo": m})
}

// Delete 删除备忘录
// @Summary 删除备忘录
// @Description 删除指定备忘录
// @Tags Memo
// @Produce json
// @Param id path string true "备忘录ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/memos/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	id := c.Param("id")

	// 验证所有权
	m, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备忘录不存在"})
		return
	}
	if m.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "备忘录已删除"})
}

// Complete 完成备忘录
// @Summary 完成备忘录
// @Description 标记备忘录为已完成
// @Tags Memo
// @Produce json
// @Param id path string true "备忘录ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/memos/{id}/complete [post]
func (h *Handler) Complete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	id := c.Param("id")

	// 验证所有权
	m, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备忘录不存在"})
		return
	}
	if m.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
		return
	}

	if err := h.service.Complete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "备忘录已完成"})
}

// Archive 归档备忘录
// @Summary 归档备忘录
// @Description 将备忘录归档
// @Tags Memo
// @Produce json
// @Param id path string true "备忘录ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/memos/{id}/archive [post]
func (h *Handler) Archive(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	id := c.Param("id")

	// 验证所有权
	m, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备忘录不存在"})
		return
	}
	if m.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
		return
	}

	if err := h.service.Archive(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "备忘录已归档"})
}

// TogglePin 切换置顶状态
// @Summary 切换置顶状态
// @Description 切换备忘录的置顶状态
// @Tags Memo
// @Produce json
// @Param id path string true "备忘录ID"
// @Success 200 {object} map[string]any
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/memos/{id}/pin [post]
func (h *Handler) TogglePin(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	id := c.Param("id")

	// 验证所有权
	m, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备忘录不存在"})
		return
	}
	if m.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问"})
		return
	}

	// 切换置顶状态
	if m.IsPinned {
		err = h.service.Unpin(c.Request.Context(), id)
	} else {
		err = h.service.Pin(c.Request.Context(), id)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pinned := !m.IsPinned
	c.JSON(http.StatusOK, gin.H{
		"pinned":  pinned,
		"message": map[bool]string{true: "已置顶", false: "已取消置顶"}[pinned],
	})
}

// ListCategories 列出分类
// @Summary 列出分类
// @Description 获取用户的备忘录分类列表
// @Tags Memo
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Router /api/memos/categories [get]
func (h *Handler) ListCategories(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	categories, err := h.service.ListCategories(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// ListTags 列出标签
// @Summary 列出标签
// @Description 获取用户的备忘录标签列表
// @Tags Memo
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Router /api/memos/tags [get]
func (h *Handler) ListTags(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	tags, err := h.service.ListTags(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// Search 搜索备忘录
// @Summary 搜索备忘录
// @Description 搜索用户的备忘录
// @Tags Memo
// @Accept json
// @Produce json
// @Param request body memo.SearchRequest true "搜索条件"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/memos/search [post]
func (h *Handler) Search(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req memo.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 使用 List 并加上搜索过滤
	filter := memo.MemoFilter{
		UserID:   userID,
		Category: req.Category,
		Tags:     req.Tags,
		Status:   req.Status,
		Priority: req.Priority,
		Search:   req.Query,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}

	memos, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"memos": memos,
		"total": len(memos),
	})
}
