package metrics

import (
	"net/http"
	"strconv"
	"time"

	"backend/internal/auth"
	"backend/internal/metrics"

	"github.com/gin-gonic/gin"
)

// Handler 指标统计 Handler
type Handler struct {
	service metrics.MetricsServiceInterface
}

// NewHandler 创建 Handler
func NewHandler(service metrics.MetricsServiceInterface) *Handler {
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

// parseTimeRange 解析时间范围参数
func parseTimeRange(c *gin.Context) (metrics.TimeRange, *time.Time, *time.Time) {
	rangeStr := c.DefaultQuery("range", "week")
	timeRange := metrics.TimeRange(rangeStr)

	var startTime, endTime *time.Time
	if start := c.Query("start"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			startTime = &t
		}
	}
	if end := c.Query("end"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			endTime = &t
		}
	}

	return timeRange, startTime, endTime
}

// GetMyUsage 获取当前用户的使用统计
// @Summary 获取使用统计
// @Description 获取当前用户的使用统计数据
// @Tags Metrics
// @Produce json
// @Param range query string false "时间范围(day/week/month)"
// @Param start query string false "开始时间(RFC3339)"
// @Param end query string false "结束时间(RFC3339)"
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/metrics/usage [get]
func (h *Handler) GetMyUsage(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	timeRange, startTime, endTime := parseTimeRange(c)

	stats, err := h.service.GetTenantUsageStats(c.Request.Context(), tenantID, timeRange, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"usage": stats})
}

// GetMyCost 获取当前用户的成本统计
// @Summary 获取成本统计
// @Description 获取当前用户的成本分析数据
// @Tags Metrics
// @Produce json
// @Param range query string false "时间范围(day/week/month)"
// @Param start query string false "开始时间(RFC3339)"
// @Param end query string false "结束时间(RFC3339)"
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/metrics/cost [get]
func (h *Handler) GetMyCost(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	timeRange, startTime, endTime := parseTimeRange(c)

	analysis, err := h.service.GetCostAnalysis(c.Request.Context(), tenantID, timeRange, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cost": analysis})
}

// GetTenantUsage 获取租户使用统计（管理员）
// @Summary 获取租户使用统计
// @Description 获取租户的使用统计数据（管理员）
// @Tags Metrics
// @Produce json
// @Param range query string false "时间范围(day/week/month)"
// @Param start query string false "开始时间(RFC3339)"
// @Param end query string false "结束时间(RFC3339)"
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/metrics/tenant/usage [get]
func (h *Handler) GetTenantUsage(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	timeRange, startTime, endTime := parseTimeRange(c)

	stats, err := h.service.GetTenantUsageStats(c.Request.Context(), tenantID, timeRange, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"usage": stats})
}

// GetTenantCost 获取租户成本分析（管理员）
// @Summary 获取租户成本分析
// @Description 获取租户的成本分析数据（管理员）
// @Tags Metrics
// @Produce json
// @Param range query string false "时间范围(day/week/month)"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/metrics/tenant/cost [get]
func (h *Handler) GetTenantCost(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	timeRange, startTime, endTime := parseTimeRange(c)

	analysis, err := h.service.GetCostAnalysis(c.Request.Context(), tenantID, timeRange, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cost": analysis})
}

// GetModelStats 获取模型统计（管理员）
// @Summary 获取模型统计
// @Description 获取模型使用统计数据（管理员）
// @Tags Metrics
// @Produce json
// @Param range query string false "时间范围(day/week/month)"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/metrics/models/stats [get]
func (h *Handler) GetModelStats(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	timeRange, startTime, endTime := parseTimeRange(c)

	stats, err := h.service.GetModelStats(c.Request.Context(), tenantID, timeRange, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// GetTopCostModels 获取成本最高的模型（管理员）
// @Summary 获取成本最高的模型
// @Description 获取成本排名前N的模型（管理员）
// @Tags Metrics
// @Produce json
// @Param range query string false "时间范围(day/week/month)"
// @Param limit query int false "返回数量"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/metrics/models/top-cost [get]
func (h *Handler) GetTopCostModels(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	timeRange, _, _ := parseTimeRange(c)
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	models, err := h.service.GetTopCostModels(c.Request.Context(), tenantID, timeRange, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// GetCostTrend 获取成本趋势（管理员）
// @Summary 获取成本趋势
// @Description 获取每日成本趋势数据（管理员）
// @Tags Metrics
// @Produce json
// @Param days query int false "天数"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/metrics/cost/trend [get]
func (h *Handler) GetCostTrend(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	trend, err := h.service.GetDailyCostTrend(c.Request.Context(), tenantID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"trend": trend})
}

// QueryModelCalls 查询模型调用日志（管理员）
// @Summary 查询模型调用日志
// @Description 查询模型调用日志（管理员）
// @Tags Metrics
// @Accept json
// @Produce json
// @Param request body metrics.ModelCallLogsRequest true "查询条件"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/metrics/model-calls/query [post]
func (h *Handler) QueryModelCalls(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req metrics.ModelCallLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.TenantID = tenantID

	logs, total, err := h.service.GetModelCallLogs(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
	})
}

// QueryWorkflowExecutions 查询工作流执行日志（管理员）
// @Summary 查询工作流执行日志
// @Description 查询工作流执行日志（管理员）
// @Tags Metrics
// @Accept json
// @Produce json
// @Param request body metrics.WorkflowExecutionLogsRequest true "查询条件"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/metrics/workflow-executions/query [post]
func (h *Handler) QueryWorkflowExecutions(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req metrics.WorkflowExecutionLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.TenantID = tenantID

	logs, total, err := h.service.GetWorkflowExecutionLogs(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
	})
}
