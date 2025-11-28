package moderation

import (
	"net/http"
	"strconv"

	response "backend/api/handlers/common"
	"backend/internal/moderation"

	"github.com/gin-gonic/gin"
)

// Handler 内容审核 API 处理器
type Handler struct {
	service *moderation.Service
}

// NewHandler 创建处理器
func NewHandler(service *moderation.Service) *Handler {
	return &Handler{service: service}
}

// ============================================================================
// 任务管理
// ============================================================================

// SubmitContent 提交内容审核
// @Summary 提交内容审核
// @Tags Moderation
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body moderation.SubmitContentRequest true "审核内容"
// @Success 200 {object} response.APIResponse{data=moderation.ModerationTask}
// @Router /api/moderation/submit [post]
func (h *Handler) SubmitContent(c *gin.Context) {
	var req moderation.SubmitContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	userName := c.GetString("username")

	task, err := h.service.SubmitContent(c.Request.Context(), tenantID, userID, userName, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, task)
}

// ListTasks 获取审核任务列表
// @Summary 获取审核任务列表
// @Tags Moderation
// @Security BearerAuth
// @Produce json
// @Param status query string false "状态"
// @Param contentType query string false "内容类型"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} response.ListResponse
// @Router /api/moderation/tasks [get]
func (h *Handler) ListTasks(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	query := &moderation.TaskQuery{
		TenantID:    tenantID,
		Status:      moderation.TaskStatus(c.Query("status")),
		ContentType: c.Query("contentType"),
		AssignedTo:  c.Query("assignedTo"),
		AIRiskLevel: c.Query("riskLevel"),
		Page:        page,
		PageSize:    pageSize,
	}

	tasks, total, err := h.service.ListTasks(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPage := (int(total) + pageSize - 1) / pageSize
	c.JSON(http.StatusOK, response.ListResponse{
		Items: tasks,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// GetTask 获取任务详情
// @Summary 获取审核任务详情
// @Tags Moderation
// @Security BearerAuth
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} response.APIResponse{data=moderation.ModerationTask}
// @Router /api/moderation/tasks/{id} [get]
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.service.GetTask(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "任务不存在")
		return
	}

	response.Success(c, task)
}

// GetPendingQueue 获取待审核队列
// @Summary 获取待审核队列
// @Tags Moderation
// @Security BearerAuth
// @Produce json
// @Param limit query int false "数量限制"
// @Success 200 {object} response.APIResponse{data=[]moderation.ModerationTask}
// @Router /api/moderation/queue [get]
func (h *Handler) GetPendingQueue(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	reviewerID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	tasks, err := h.service.GetPendingQueue(c.Request.Context(), tenantID, reviewerID, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, tasks)
}

// AssignTask 分配任务
// @Summary 分配审核任务
// @Tags Moderation
// @Security BearerAuth
// @Param id path string true "任务ID"
// @Success 200 {object} response.APIResponse
// @Router /api/moderation/tasks/{id}/assign [post]
func (h *Handler) AssignTask(c *gin.Context) {
	taskID := c.Param("id")
	reviewerID := c.GetString("user_id")

	if err := h.service.AssignTask(c.Request.Context(), taskID, reviewerID); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============================================================================
// 审核操作
// ============================================================================

// ReviewTask 审核任务
// @Summary 审核任务
// @Tags Moderation
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body moderation.ReviewRequest true "审核信息"
// @Success 200 {object} response.APIResponse{data=moderation.ModerationRecord}
// @Router /api/moderation/review [post]
func (h *Handler) ReviewTask(c *gin.Context) {
	var req moderation.ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	reviewerID := c.GetString("user_id")
	reviewerName := c.GetString("username")

	record, err := h.service.ReviewTask(c.Request.Context(), tenantID, reviewerID, reviewerName, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, record)
}

// GetTaskRecords 获取审核记录
// @Summary 获取任务审核记录
// @Tags Moderation
// @Security BearerAuth
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} response.APIResponse{data=[]moderation.ModerationRecord}
// @Router /api/moderation/tasks/{id}/records [get]
func (h *Handler) GetTaskRecords(c *gin.Context) {
	taskID := c.Param("id")

	records, err := h.service.GetTaskRecords(c.Request.Context(), taskID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, records)
}

// ============================================================================
// 敏感词管理
// ============================================================================

// ListSensitiveWords 获取敏感词列表
// @Summary 获取敏感词列表
// @Tags Moderation
// @Security BearerAuth
// @Produce json
// @Param category query string false "类别"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} response.ListResponse
// @Router /api/moderation/words [get]
func (h *Handler) ListSensitiveWords(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	category := c.Query("category")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

	words, total, err := h.service.ListSensitiveWords(c.Request.Context(), tenantID, category, page, pageSize)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPage := (int(total) + pageSize - 1) / pageSize
	c.JSON(http.StatusOK, response.ListResponse{
		Items: words,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// AddSensitiveWord 添加敏感词
// @Summary 添加敏感词
// @Tags Moderation
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "敏感词信息"
// @Success 200 {object} response.APIResponse{data=moderation.SensitiveWord}
// @Router /api/moderation/words [post]
func (h *Handler) AddSensitiveWord(c *gin.Context) {
	var req struct {
		Word     string `json:"word" binding:"required"`
		Category string `json:"category" binding:"required"`
		Level    string `json:"level"`
		Action   string `json:"action"`
		Replace  string `json:"replace"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	word, err := h.service.AddSensitiveWord(c.Request.Context(), tenantID, userID,
		req.Word, req.Category, req.Level, req.Action, req.Replace)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, word)
}

// BatchAddWords 批量添加敏感词
// @Summary 批量添加敏感词
// @Tags Moderation
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body moderation.BatchWordRequest true "批量敏感词"
// @Success 200 {object} response.APIResponse
// @Router /api/moderation/words/batch [post]
func (h *Handler) BatchAddWords(c *gin.Context) {
	var req moderation.BatchWordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	count, err := h.service.BatchAddWords(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, map[string]int{"added": count})
}

// DeleteSensitiveWord 删除敏感词
// @Summary 删除敏感词
// @Tags Moderation
// @Security BearerAuth
// @Param id path string true "敏感词ID"
// @Success 200 {object} response.APIResponse
// @Router /api/moderation/words/{id} [delete]
func (h *Handler) DeleteSensitiveWord(c *gin.Context) {
	id := c.Param("id")
	tenantID := c.GetString("tenant_id")

	if err := h.service.DeleteSensitiveWord(c.Request.Context(), id, tenantID); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// FilterContent 过滤内容
// @Summary 过滤内容中的敏感词
// @Tags Moderation
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "内容"
// @Success 200 {object} response.APIResponse{data=moderation.FilterResult}
// @Router /api/moderation/filter [post]
func (h *Handler) FilterContent(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")

	result := h.service.FilterContent(c.Request.Context(), tenantID, req.Content)
	response.Success(c, result)
}

// ============================================================================
// 审核规则
// ============================================================================

// ListRules 获取审核规则列表
// @Summary 获取审核规则列表
// @Tags Moderation
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse{data=[]moderation.ModerationRule}
// @Router /api/moderation/rules [get]
func (h *Handler) ListRules(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	rules, err := h.service.ListRules(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, rules)
}

// CreateRule 创建审核规则
// @Summary 创建审核规则
// @Tags Moderation
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body moderation.ModerationRule true "规则配置"
// @Success 200 {object} response.APIResponse
// @Router /api/moderation/rules [post]
func (h *Handler) CreateRule(c *gin.Context) {
	var rule moderation.ModerationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	rule.TenantID = c.GetString("tenant_id")

	if err := h.service.CreateRule(c.Request.Context(), &rule); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, rule)
}

// UpdateRule 更新审核规则
// @Summary 更新审核规则
// @Tags Moderation
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "规则ID"
// @Param request body object true "更新内容"
// @Success 200 {object} response.APIResponse
// @Router /api/moderation/rules/{id} [put]
func (h *Handler) UpdateRule(c *gin.Context) {
	id := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateRule(c.Request.Context(), id, updates); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// DeleteRule 删除审核规则
// @Summary 删除审核规则
// @Tags Moderation
// @Security BearerAuth
// @Param id path string true "规则ID"
// @Success 200 {object} response.APIResponse
// @Router /api/moderation/rules/{id} [delete]
func (h *Handler) DeleteRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteRule(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============================================================================
// 统计
// ============================================================================

// GetStats 获取审核统计
// @Summary 获取审核统计
// @Tags Moderation
// @Security BearerAuth
// @Produce json
// @Param period query string false "统计周期" default(monthly)
// @Success 200 {object} response.APIResponse{data=moderation.ModerationStats}
// @Router /api/moderation/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	period := c.DefaultQuery("period", "monthly")

	stats, err := h.service.GetStats(c.Request.Context(), tenantID, period)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, stats)
}
