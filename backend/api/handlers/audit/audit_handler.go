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

	// 设置分页
	query.Pagination = &types.PaginationRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	// 查询日志
	logs, paginationResp, err := h.auditService.QueryLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.ListResponse{
		Items: logs,
		Pagination: response.PaginationMeta{
			Page:      paginationResp.Page,
			PageSize:  paginationResp.PageSize,
			Total:     paginationResp.Total,
			TotalPage: int(paginationResp.TotalPages),
		},
	})
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

	// 获取用户上下文
	userCtx, _ := auth.GetUserContext(c)

	log, err := h.auditService.GetLogByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	if log == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "日志不存在"})
		return
	}

	// 检查租户权限
	if log.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	// 非管理员只能查看自己的日志
	if !hasAdminRole(userCtx.Roles) && log.UserID != userCtx.UserID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// ListTenantAuditLogs 返回当前租户最近的审计日志（限管理员权限）。
func (h *AuditHandler) GetTenantAuditLogs(c *gin.Context) {
	userCtx, _ := auth.GetUserContext(c)
	if userCtx.TenantID == "" {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}
	limit := 10
	if raw := c.DefaultQuery("limit", "10"); raw != "" {
		if n := parseInt(raw); n > 0 && n <= 200 {
			limit = n
		}
	}
	query := &models.AuditLogQuery{
		TenantID: userCtx.TenantID,
		Pagination: &types.PaginationRequest{
			Page:     1,
			PageSize: limit,
		},
	}
	logs, paginationResp, err := h.auditService.QueryLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.ListResponse{
		Items: logs,
		Pagination: response.PaginationMeta{
			Page:      paginationResp.Page,
			PageSize:  paginationResp.PageSize,
			Total:     paginationResp.Total,
			TotalPage: int(paginationResp.TotalPages),
		},
	})
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

	// 默认统计最近 30 条
	limit := 30
	if limitParam := c.Query("limit"); limitParam != "" {
		if l := parseInt(limitParam); l > 0 && l <= 100 {
			limit = l
		}
	}

	query := &models.AuditLogQuery{
		TenantID: userCtx.TenantID,
		UserID:   userID,
		Pagination: &types.PaginationRequest{
			Page:     1,
			PageSize: limit,
		},
	}
	logs, _, err := h.auditService.QueryLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"user_id": userID, "limit": limit, "actions": logs}})
}

// GetMyActivity 获取当前用户的活动摘要
// @Summary 获取当前用户活动摘要
// @Tags Audit
// @Security BearerAuth
// @Produce json
// @Param limit query int false "返回条数"
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

	// 默认返回最近 30 条
	limit := 30
	if limitParam := c.Query("limit"); limitParam != "" {
		if l := parseInt(limitParam); l > 0 && l <= 100 {
			limit = l
		}
	}

	query := &models.AuditLogQuery{
		TenantID: userCtx.TenantID,
		UserID:   userCtx.UserID,
		Pagination: &types.PaginationRequest{
			Page:     1,
			PageSize: limit,
		},
	}
	logs, _, err := h.auditService.QueryLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"user_id": userCtx.UserID, "limit": limit, "actions": logs}})
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

// ExportLogsRequest 导出审计日志请求
type ExportLogsRequest struct {
	Format    string  `json:"format" binding:"required"` // csv, json
	StartTime *string `json:"startTime"`
	EndTime   *string `json:"endTime"`
	UserID    string  `json:"userId"`
	Action    string  `json:"action"`
	Limit     int     `json:"limit"`
}

// ExportLogs 导出审计日志
// POST /api/audit/logs/export
func (h *AuditHandler) ExportLogs(c *gin.Context) {
	var req ExportLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	userCtx, _ := auth.GetUserContext(c)
	if userCtx.TenantID == "" {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	// 解析时间
	var startTime, endTime *time.Time
	if req.StartTime != nil {
		if t, err := time.Parse(time.RFC3339, *req.StartTime); err == nil {
			startTime = &t
		}
	}
	if req.EndTime != nil {
		if t, err := time.Parse(time.RFC3339, *req.EndTime); err == nil {
			endTime = &t
		}
	}

	// 构造查询
	query := &models.AuditLogQuery{
		TenantID:  userCtx.TenantID,
		StartTime: startTime,
		EndTime:   endTime,
	}

	// 非管理员只能导出自己的日志
	if !hasAdminRole(userCtx.Roles) {
		query.UserID = userCtx.UserID
	} else if req.UserID != "" {
		query.UserID = req.UserID
	}

	if req.Action != "" {
		query.EventTypes = []string{req.Action}
	}

	// 设置分页
	limit := req.Limit
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}

	query.Pagination = &types.PaginationRequest{Page: 1, PageSize: limit}

	// 查询日志
	logs, _, err := h.auditService.QueryLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	// 根据格式导出
	switch req.Format {
	case "csv":
		h.exportCSV(c, logs)
	case "json":
		h.exportJSON(c, logs)
	default:
		h.exportJSON(c, logs)
	}
}

// exportCSV 导出为 CSV
func (h *AuditHandler) exportCSV(c *gin.Context, logs []*models.AuditLog) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=audit_logs.csv")

	// 写入 BOM 以支持 Excel 中文
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	
	// 写入表头
	c.Writer.WriteString("ID,租户ID,用户ID,事件类型,事件分类,事件级别,请求路径,请求方法,IP地址,用户代理,状态码,创建时间\n")

	// 写入数据
	for _, log := range logs {
		line := escapeCSV(log.ID) + "," +
			escapeCSV(log.TenantID) + "," +
			escapeCSV(log.UserID) + "," +
			escapeCSV(log.EventType) + "," +
			escapeCSV(log.EventCategory) + "," +
			escapeCSV(log.EventLevel) + "," +
			escapeCSV(log.RequestPath) + "," +
			escapeCSV(log.RequestMethod) + "," +
			escapeCSV(log.IPAddress) + "," +
			escapeCSV(log.UserAgent) + "," +
			itoa(log.StatusCode) + "," +
			log.CreatedAt.Format(time.RFC3339) + "\n"
		c.Writer.WriteString(line)
	}
}

// exportJSON 导出为 JSON
func (h *AuditHandler) exportJSON(c *gin.Context, logs []*models.AuditLog) {
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=audit_logs.json")

	result := map[string]interface{}{
		"exportedAt": time.Now().Format(time.RFC3339),
		"totalCount": len(logs),
		"logs":       logs,
	}

	c.JSON(http.StatusOK, result)
}

// escapeCSV CSV 字段转义
func escapeCSV(s string) string {
	if s == "" {
		return ""
	}
	needQuote := false
	for _, c := range s {
		if c == ',' || c == '"' || c == '\n' || c == '\r' {
			needQuote = true
			break
		}
	}
	if !needQuote {
		return s
	}
	// 双引号转义
	result := "\""
	for _, c := range s {
		if c == '"' {
			result += "\"\""
		} else {
			result += string(c)
		}
	}
	result += "\""
	return result
}

// itoa 整数转字符串
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string('0'+byte(n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// itoa64 int64转字符串
func itoa64(n int64) string {
	return itoa(int(n))
}
