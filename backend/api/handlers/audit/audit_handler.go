package audit

import (
	"net/http"
	"time"

	response "backend/api/handlers/common"
	"backend/internal/auth"
	"backend/internal/models"
	"backend/pkg/types"

	"github.com/gin-gonic/gin"
)

// AuditHandler 审计日志处理器
type AuditHandler struct {
	auditService *models.AuditLogService
}

// NewAuditHandler 创建审计日志处理器
func NewAuditHandler(auditService *models.AuditLogService) *AuditHandler {
	return &AuditHandler{
		auditService: auditService,
	}
}

// QueryLogsRequest 查询审计日志请求
type QueryLogsRequest struct {
	UserID        string   `json:"user_id"`
	EventTypes    []string `json:"event_types"`
	EventCategory string   `json:"event_category"`
	EventLevel    string   `json:"event_level"`
	StartTime     *string  `json:"start_time"` // ISO 8601 格式
	EndTime       *string  `json:"end_time"`   // ISO 8601 格式
	IPAddress     string   `json:"ip_address"`
	RequestPath   string   `json:"request_path"`
	Page          int      `json:"page"`
	PageSize      int      `json:"page_size"`
}

// QueryLogs 查询审计日志
// @Summary 查询审计日志
// @Tags Audit
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body QueryLogsRequest true "查询条件"
// @Success 200 {object} response.ListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/audit/logs/query [post]
func (h *AuditHandler) QueryLogs(c *gin.Context) {
	var req QueryLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	// 获取用户上下文
	userCtx, _ := auth.GetUserContext(c)

	// 构造查询条件
	query := &models.AuditLogQuery{
		TenantID:      userCtx.TenantID,
		EventTypes:    req.EventTypes,
		EventCategory: req.EventCategory,
		EventLevel:    req.EventLevel,
		IPAddress:     req.IPAddress,
		RequestPath:   req.RequestPath,
	}

	// 非管理员只能查看自己的日志
	if !hasAdminRole(userCtx.Roles) {
		query.UserID = userCtx.UserID
	} else if req.UserID != "" {
		query.UserID = req.UserID
	}

	// 解析时间范围
	if req.StartTime != nil {
		if t, err := time.Parse(time.RFC3339, *req.StartTime); err == nil {
			query.StartTime = &t
		}
	}
	if req.EndTime != nil {
		if t, err := time.Parse(time.RFC3339, *req.EndTime); err == nil {
			query.EndTime = &t
		}
	}

	// 分页参数
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	query.Pagination = &types.PaginationRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	// 查询日志
	logs, pagination, err := h.auditService.QueryLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.ListResponse{Items: logs, Pagination: toPaginationMeta(pagination)})
}

// GetLog 获取单条审计日志
// @Summary 获取审计日志详情
// @Tags Audit
// @Security BearerAuth
// @Produce json
// @Param id path string true "日志 ID"
// @Success 200 {object} models.AuditLog
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/audit/logs/{id} [get]
func (h *AuditHandler) GetLog(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少日志 ID"})
		return
	}

	log, err := h.auditService.GetLogByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	if log == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "日志不存在"})
		return
	}

	// 获取用户上下文
	userCtx, _ := auth.GetUserContext(c)

	// 非管理员只能查看自己的日志
	if !hasAdminRole(userCtx.Roles) && log.UserID != userCtx.UserID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// GetUserActivity 获取用户活动摘要
// @Summary 获取指定用户活动摘要
// @Tags Audit
// @Security BearerAuth
// @Produce json
// @Param userID path string true "用户 ID"
// @Param days query int false "统计天数"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/audit/users/{userID}/activity [get]
func (h *AuditHandler) GetUserActivity(c *gin.Context) {
	userID := c.Param("userID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少用户 ID"})
		return
	}

	// 获取用户上下文
	userCtx, _ := auth.GetUserContext(c)

	// 非管理员只能查看自己的活动
	if !hasAdminRole(userCtx.Roles) && userID != userCtx.UserID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	// 默认统计最近 30 天
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if d := parseInt(daysParam); d > 0 && d <= 365 {
			days = d
		}
	}

	summary, err := h.auditService.GetUserActivitySummary(c.Request.Context(), userID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"user_id": userID, "days": days, "summary": summary}})
}

// GetMyActivity 获取当前用户的活动摘要
// @Summary 获取当前用户活动摘要
// @Tags Audit
// @Security BearerAuth
// @Produce json
// @Param days query int false "统计天数"
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/audit/my-activity [get]
func (h *AuditHandler) GetMyActivity(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	// 默认统计最近 30 天
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if d := parseInt(daysParam); d > 0 && d <= 365 {
			days = d
		}
	}

	summary, err := h.auditService.GetUserActivitySummary(c.Request.Context(), userCtx.UserID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"user_id": userCtx.UserID, "days": days, "summary": summary}})
}

// hasAdminRole 检查是否有管理员角色
func hasAdminRole(roles []string) bool {
	for _, role := range roles {
		if role == "admin" || role == "super_admin" {
			return true
		}
	}
	return false
}

// parseInt 解析整数（简化版）
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

func toPaginationMeta(p *types.PaginationResponse) response.PaginationMeta {
	if p == nil {
		return response.PaginationMeta{}
	}
	return response.PaginationMeta{
		Page:      p.Page,
		PageSize:  p.PageSize,
		Total:     p.Total,
		TotalPage: p.TotalPages,
	}
}
