package analytics

import (
	"net/http"
	"strconv"
	"time"

	response "backend/api/handlers/common"
	"backend/internal/analytics"
	"backend/internal/auth"

	"github.com/gin-gonic/gin"
)
// DashboardHandler 仪表盘 API 处理器
type DashboardHandler struct {
	service *analytics.Service
}

// ExportReportRequest 导出报告请求
type ExportReportRequest struct {
	ReportType string    `json:"report_type" binding:"required,oneof=writing_efficiency content_performance user_engagement"`
	DateRange  DateRange `json:"date_range" binding:"required"`
	Format     string    `json:"format" binding:"required,oneof=csv excel pdf"`
}

// DateRange 日期范围
type DateRange struct {
	Start string `json:"start" binding:"required"` // YYYY-MM-DD
	End   string `json:"end" binding:"required"`   // YYYY-MM-DD
}


// NewDashboardHandler 创建处理器
func NewDashboardHandler(service *analytics.Service) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// GetAuthorDashboard 获取作者仪表盘
// @Summary 获取作者仪表盘数据
// @Tags Analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "统计天数（默认30）"
// @Success 200 {object} analytics.AuthorDashboard
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/dashboard [get]
func (h *DashboardHandler) GetAuthorDashboard(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	startTime := time.Now().AddDate(0, 0, -days)
	query := &analytics.AnalyticsQuery{
		TenantID:  userCtx.TenantID,
		UserID:    userCtx.UserID,
		StartTime: &startTime,
	}

	dashboard, err := h.service.GetAuthorDashboard(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取仪表盘失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: dashboard})
}

// GetMonthlyReport 获取月度报告
// @Summary 获取月度写作报告
// @Tags Analytics
// @Security BearerAuth
// @Produce json
// @Param year query int false "年份（默认当前年）"
// @Param month query int false "月份（默认当前月）"
// @Success 200 {object} analytics.MonthlyReport
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/monthly-report [get]
func (h *DashboardHandler) GetMonthlyReport(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if y := c.Query("year"); y != "" {
		if n, err := strconv.Atoi(y); err == nil && n >= 2000 && n <= 2100 {
			year = n
		}
	}
	if m := c.Query("month"); m != "" {
		if n, err := strconv.Atoi(m); err == nil && n >= 1 && n <= 12 {
			month = n
		}
	}

	report, err := h.service.GetMonthlyReport(c.Request.Context(), userCtx.TenantID, userCtx.UserID, year, month)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取月度报告失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: report})
}

// GetTokenTrend 获取 Token 消耗趋势
// @Summary 获取 Token 消耗趋势图数据
// @Tags Analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "统计天数（默认30）"
// @Success 200 {array} analytics.TokenTrend
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/token-trend [get]
func (h *DashboardHandler) GetTokenTrend(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	trend, err := h.service.GetTokenTrend(c.Request.Context(), userCtx.TenantID, userCtx.UserID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取趋势数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: trend})
}

// GetFeatureUsage 获取功能使用分布
// @Summary 获取 AI 功能使用分布
// @Tags Analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "统计天数（默认30）"
// @Success 200 {array} analytics.FeatureUsage
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/feature-usage [get]
func (h *DashboardHandler) GetFeatureUsage(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	usage, err := h.service.GetFeatureUsage(c.Request.Context(), userCtx.TenantID, userCtx.UserID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取功能使用分布失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: usage})
}

// GetModelPreference 获取模型偏好分析
// @Summary 获取 AI 模型使用偏好
// @Tags Analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "统计天数（默认30）"
// @Success 200 {array} analytics.ModelPreference
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/model-preference [get]
func (h *DashboardHandler) GetModelPreference(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	prefs, err := h.service.GetModelPreference(c.Request.Context(), userCtx.TenantID, userCtx.UserID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取模型偏好失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: prefs})
}

// GetWritingEfficiency 获取写作效率分析
// @Summary 获取写作效率统计
// @Tags Analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "统计天数（默认30）"
// @Success 200 {object} analytics.WritingEfficiency
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/writing-efficiency [get]
func (h *DashboardHandler) GetWritingEfficiency(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	efficiency, err := h.service.GetWritingEfficiency(c.Request.Context(), userCtx.TenantID, userCtx.UserID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取写作效率失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: efficiency})
}

// GetWritingHabits 获取写作习惯分析
// @Summary 获取写作习惯统计
// @Tags Analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "统计天数（默认30）"
// @Success 200 {object} analytics.WritingHabits
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/writing-habits [get]
func (h *DashboardHandler) GetWritingHabits(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	habits, err := h.service.GetWritingHabits(c.Request.Context(), userCtx.TenantID, userCtx.UserID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取写作习惯失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: habits})
}

// GetRecentActivities 获取近期活动记录
// @Summary 获取近期 AI 调用和创作记录
// @Tags Analytics
// @Security BearerAuth
// @Produce json
// @Param limit query int false "返回条数（默认20）"
// @Success 200 {array} analytics.RecentActivity
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/recent-activities [get]
func (h *DashboardHandler) GetRecentActivities(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	activities, err := h.service.GetRecentActivities(c.Request.Context(), userCtx.TenantID, userCtx.UserID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取活动记录失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: activities})
}

// ExportReport 导出分析报告
// @Summary 导出分析报告
// @Tags Analytics
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body ExportReportRequest true "导出请求"
// @Success 202 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/analytics/reports/export [post]
func (h *DashboardHandler) ExportReport(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	var req ExportReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	// 验证日期格式
	startTime, err := time.Parse("2006-01-02", req.DateRange.Start)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "开始日期格式错误，应为 YYYY-MM-DD"})
		return
	}

	endTime, err := time.Parse("2006-01-02", req.DateRange.End)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "结束日期格式错误，应为 YYYY-MM-DD"})
		return
	}

	if endTime.Before(startTime) {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "结束日期不能早于开始日期"})
		return
	}

	// 生成任务ID
	taskID := generateTaskID()

	// TODO: 这里应该提交异步任务到Asynq队列
	// task := asynq.NewTask("analytics:export", payload)
	// h.queueClient.Enqueue(ctx, task, asynq.Queue("exports"))

	// 简化实现：返回任务ID和下载链接
	// 实际项目中应该异步生成文件
	downloadURL := "/api/analytics/exports/" + taskID + "/download"

	c.JSON(http.StatusAccepted, response.APIResponse{
		Success: true,
		Message: "导出任务已创建",
		Data: gin.H{
			"task_id":      taskID,
			"download_url": downloadURL,
			"status":       "pending",
			"report_type":  req.ReportType,
			"format":       req.Format,
			"tenant_id":    userCtx.TenantID,
			"user_id":      userCtx.UserID,
			"created_at":   time.Now(),
			"expires_at":   time.Now().Add(7 * 24 * time.Hour), // 7天后过期
		},
	})
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return time.Now().Format("20060102150405") + "-" + strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
}

