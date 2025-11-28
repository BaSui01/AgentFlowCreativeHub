package content

import (
	"net/http"
	"strconv"

	"backend/internal/auth"
	"backend/internal/content"

	"github.com/gin-gonic/gin"
)

// Handler 内容管理 API 处理器
type Handler struct {
	service *content.Service
}

// NewHandler 创建处理器
func NewHandler(service *content.Service) *Handler {
	return &Handler{service: service}
}

// getUserContext 获取用户上下文
func getUserContext(c *gin.Context) (tenantID, userID string, ok bool) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		return "", "", false
	}
	return userCtx.TenantID, userCtx.UserID, true
}

// ========== 公开作品 ==========

// PublishWork 发布作品
// @Summary 发布作品
// @Description 发布新的公开作品
// @Tags Content
// @Accept json
// @Produce json
// @Param request body content.PublishWorkRequest true "作品信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/works [post]
func (h *Handler) PublishWork(c *gin.Context) {
	var req content.PublishWorkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	req.TenantID = tenantID
	req.UserID = userID

	work, err := h.service.PublishWork(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, work)
}

// GetWork 获取作品详情
// @Summary 获取作品详情
// @Description 根据ID获取作品详细信息
// @Tags Content
// @Produce json
// @Param id path string true "作品ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/works/{id} [get]
func (h *Handler) GetWork(c *gin.Context) {
	workID := c.Param("id")
	
	work, err := h.service.GetWork(c.Request.Context(), workID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == content.ErrWorkNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 增加浏览量
	h.service.IncrementView(c.Request.Context(), workID)

	c.JSON(http.StatusOK, work)
}

// ListWorks 获取作品列表
// @Summary 获取作品列表
// @Description 获取作品列表，支持分页和筛选
// @Tags Content
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param userId query string false "用户ID"
// @Param categoryId query string false "分类ID"
// @Param tag query string false "标签"
// @Param status query string false "状态"
// @Param keyword query string false "关键词"
// @Param sortBy query string false "排序方式"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/content/works [get]
func (h *Handler) ListWorks(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	query := &content.ListWorksQuery{
		TenantID:   tenantID,
		UserID:     c.Query("userId"),
		CategoryID: c.Query("categoryId"),
		Tag:        c.Query("tag"),
		Status:     content.PublishStatus(c.Query("status")),
		Keyword:    c.Query("keyword"),
		SortBy:     c.DefaultQuery("sortBy", "latest"),
		Page:       page,
		PageSize:   pageSize,
	}

	works, total, err := h.service.ListWorks(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"works": works,
		"total": total,
		"page":  page,
		"pageSize": pageSize,
	})
}

// UpdateWork 更新作品
// @Summary 更新作品
// @Description 更新指定作品信息
// @Tags Content
// @Accept json
// @Produce json
// @Param id path string true "作品ID"
// @Param request body content.UpdateWorkRequest true "更新信息"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/works/{id} [put]
func (h *Handler) UpdateWork(c *gin.Context) {
	workID := c.Param("id")
	var req content.UpdateWorkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateWork(c.Request.Context(), workID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteWork 删除作品
// @Summary 删除作品
// @Description 删除指定作品
// @Tags Content
// @Produce json
// @Param id path string true "作品ID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/works/{id} [delete]
func (h *Handler) DeleteWork(c *gin.Context) {
	workID := c.Param("id")
	
	if err := h.service.DeleteWork(c.Request.Context(), workID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}


// ReviewWork 审核作品
// @Summary 审核作品
// @Description 审核指定作品（管理员）
// @Tags Content
// @Accept json
// @Produce json
// @Param id path string true "作品ID"
// @Param request body content.ReviewWorkRequest true "审核信息"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/works/{id}/review [post]
func (h *Handler) ReviewWork(c *gin.Context) {
	var req content.ReviewWorkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	req.WorkID = c.Param("id")
	req.ReviewerID = userID

	if err := h.service.ReviewWork(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "审核完成"})
}

// SetRecommend 设置推荐
// @Summary 设置推荐
// @Description 设置作品推荐状态（管理员）
// @Tags Content
// @Accept json
// @Produce json
// @Param id path string true "作品ID"
// @Param request body content.RecommendWorkRequest true "推荐信息"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/works/{id}/recommend [post]
func (h *Handler) SetRecommend(c *gin.Context) {
	var req content.RecommendWorkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.WorkID = c.Param("id")

	if err := h.service.SetRecommend(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置成功"})
}

// OfflineWork 下架作品
// @Summary 下架作品
// @Description 下架指定作品（管理员）
// @Tags Content
// @Accept json
// @Produce json
// @Param id path string true "作品ID"
// @Param request body map[string]string false "下架原因"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/works/{id}/offline [post]
func (h *Handler) OfflineWork(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	if err := h.service.OfflineWork(c.Request.Context(), c.Param("id"), req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "下架成功"})
}

// GetRecommendWorks 获取推荐作品
// @Summary 获取推荐作品
// @Description 获取推荐作品列表
// @Tags Content
// @Produce json
// @Param limit query int false "数量限制"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/content/works/recommend [get]
func (h *Handler) GetRecommendWorks(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	works, err := h.service.GetRecommendWorks(c.Request.Context(), tenantID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"works": works})
}

// SearchWorks 搜索作品（支持全文搜索和高级筛选）
// @Summary 搜索作品
// @Description 搜索作品，支持全文搜索和高级筛选
// @Tags Content
// @Accept json
// @Produce json
// @Param request body content.SearchWorksRequest true "搜索条件"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/works/search [post]
func (h *Handler) SearchWorks(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req content.SearchWorksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置租户ID
	req.TenantID = tenantID

	// 设置默认分页参数
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	works, total, err := h.service.SearchWorks(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"works":    works,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	})
}

// ========== 分类管理 ==========

// CreateCategory 创建分类
// @Summary 创建分类
// @Description 创建内容分类（管理员）
// @Tags Content
// @Accept json
// @Produce json
// @Param request body content.CreateCategoryRequest true "分类信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/categories [post]
func (h *Handler) CreateCategory(c *gin.Context) {
	var req content.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	req.TenantID = tenantID
	category, err := h.service.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, category)
}

// ListCategories 获取分类列表
// @Summary 获取分类列表
// @Description 获取内容分类列表
// @Tags Content
// @Produce json
// @Param parentId query string false "父分类ID"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/content/categories [get]
func (h *Handler) ListCategories(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	categories, err := h.service.ListCategories(c.Request.Context(), tenantID, c.Query("parentId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// UpdateCategory 更新分类
// @Summary 更新分类
// @Description 更新内容分类（管理员）
// @Tags Content
// @Accept json
// @Produce json
// @Param id path string true "分类ID"
// @Param request body map[string]any true "更新字段"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/categories/{id} [put]
func (h *Handler) UpdateCategory(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateCategory(c.Request.Context(), c.Param("id"), updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteCategory 删除分类
// @Summary 删除分类
// @Description 删除内容分类（管理员）
// @Tags Content
// @Produce json
// @Param id path string true "分类ID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/categories/{id} [delete]
func (h *Handler) DeleteCategory(c *gin.Context) {
	if err := h.service.DeleteCategory(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ========== 标签管理 ==========

// ListTags 获取标签列表
// @Summary 获取标签列表
// @Description 获取内容标签列表
// @Tags Content
// @Produce json
// @Param hotOnly query bool false "仅热门标签"
// @Param limit query int false "数量限制"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/content/tags [get]
func (h *Handler) ListTags(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	hotOnly := c.Query("hotOnly") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	tags, err := h.service.ListTags(c.Request.Context(), tenantID, hotOnly, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// SetHotTag 设置热门标签
// @Summary 设置热门标签
// @Description 设置标签热门状态（管理员）
// @Tags Content
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param request body map[string]bool true "热门状态"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/tags/{id}/hot [put]
func (h *Handler) SetHotTag(c *gin.Context) {
	var req struct {
		IsHot bool `json:"isHot"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SetHotTag(c.Request.Context(), c.Param("id"), req.IsHot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置成功"})
}

// ========== 举报管理 ==========

// CreateReport 创建举报
// @Summary 创建举报
// @Description 举报内容
// @Tags Content
// @Accept json
// @Produce json
// @Param request body content.CreateReportRequest true "举报信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/reports [post]
func (h *Handler) CreateReport(c *gin.Context) {
	var req content.CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	req.TenantID = tenantID
	req.ReporterID = userID

	report, err := h.service.CreateReport(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == content.ErrAlreadyReported {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

// ListReports 获取举报列表
// @Summary 获取举报列表
// @Description 获取举报列表（管理员）
// @Tags Content
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param status query string false "状态过滤"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/content/reports [get]
func (h *Handler) ListReports(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status := content.ReportStatus(c.Query("status"))

	reports, total, err := h.service.ListReports(c.Request.Context(), tenantID, status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reports":  reports,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// HandleReport 处理举报
// @Summary 处理举报
// @Description 处理举报（管理员）
// @Tags Content
// @Accept json
// @Produce json
// @Param id path string true "举报ID"
// @Param request body content.HandleReportRequest true "处理信息"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/content/reports/{id}/handle [post]
func (h *Handler) HandleReport(c *gin.Context) {
	var req content.HandleReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	req.ReportID = c.Param("id")
	req.HandlerID = userID

	if err := h.service.HandleReport(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "处理完成"})
}

// ========== 统计 ==========

// GetContentStats 获取内容统计
// @Summary 获取内容统计
// @Description 获取内容统计数据（管理员）
// @Tags Content
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/content/stats [get]
func (h *Handler) GetContentStats(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	stats, err := h.service.GetContentStats(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
