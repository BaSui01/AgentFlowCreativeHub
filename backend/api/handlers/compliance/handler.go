package compliance

import (
	"net/http"
	"strconv"
	"time"

	"backend/internal/auth"
	"backend/internal/compliance"

	"github.com/gin-gonic/gin"
)

// Handler 合规管理 API 处理器
type Handler struct {
	service *compliance.Service
}

// NewHandler 创建处理器
func NewHandler(service *compliance.Service) *Handler {
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

// ========== 实名认证 ==========

// SubmitVerification 提交实名认证
// POST /api/compliance/verifications
func (h *Handler) SubmitVerification(c *gin.Context) {
	var req compliance.SubmitVerificationRequest
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

	v, err := h.service.SubmitVerification(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == compliance.ErrAlreadyVerified {
			status = http.StatusConflict
		} else if err == compliance.ErrInvalidIDNumber {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, v)
}

// GetMyVerification 获取我的认证状态
// GET /api/compliance/verifications/me
func (h *Handler) GetMyVerification(c *gin.Context) {
	_, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	v, err := h.service.GetUserVerification(c.Request.Context(), userID)
	if err != nil {
		if err == compliance.ErrVerificationNotFound {
			c.JSON(http.StatusOK, gin.H{"verification": nil, "status": "none"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"verification": v})
}

// ListVerifications 获取认证列表（管理员）
// GET /api/compliance/verifications
func (h *Handler) ListVerifications(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status := compliance.VerificationStatus(c.Query("status"))

	verifications, total, err := h.service.ListVerifications(c.Request.Context(), tenantID, status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"verifications": verifications,
		"total":         total,
		"page":          page,
		"pageSize":      pageSize,
	})
}

// ReviewVerification 审核实名认证
// POST /api/compliance/verifications/:id/review
func (h *Handler) ReviewVerification(c *gin.Context) {
	var req compliance.ReviewVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	req.VerificationID = c.Param("id")
	req.ReviewerID = userID

	if err := h.service.ReviewVerification(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "审核完成"})
}

// ========== 内容分级 ==========

// SetContentRating 设置内容分级
// POST /api/compliance/ratings
func (h *Handler) SetContentRating(c *gin.Context) {
	var req compliance.SetContentRatingRequest
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
	rating, err := h.service.SetContentRating(c.Request.Context(), &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rating)
}

// GetContentRating 获取内容分级
// GET /api/compliance/ratings/:contentId
func (h *Handler) GetContentRating(c *gin.Context) {
	contentID := c.Param("contentId")
	contentType := c.DefaultQuery("contentType", "work")

	rating, err := h.service.GetContentRating(c.Request.Context(), contentID, contentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if rating == nil {
		c.JSON(http.StatusOK, gin.H{"rating": nil, "defaultRating": "all"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rating": rating})
}

// ========== 合规检查 ==========

// RunComplianceCheck 执行合规检查
// POST /api/compliance/checks
func (h *Handler) RunComplianceCheck(c *gin.Context) {
	var req compliance.RunComplianceCheckRequest
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
	check, err := h.service.RunComplianceCheck(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, check)
}

// ListComplianceChecks 获取检查列表
// GET /api/compliance/checks
func (h *Handler) ListComplianceChecks(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status := compliance.CheckStatus(c.Query("status"))

	checks, total, err := h.service.ListComplianceChecks(c.Request.Context(), tenantID, status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"checks":   checks,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// ========== 版权保护 ==========

// RegisterCopyright 登记版权
// POST /api/compliance/copyrights
func (h *Handler) RegisterCopyright(c *gin.Context) {
	var req struct {
		ContentID     string `json:"contentId" binding:"required"`
		CopyrightType string `json:"copyrightType"`
		Author        string `json:"author"`
		Declaration   string `json:"declaration"`
		LicenseType   string `json:"licenseType"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	record, err := h.service.RegisterCopyright(c.Request.Context(), tenantID, req.ContentID, userID,
		req.CopyrightType, req.Author, req.Declaration, req.LicenseType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}

// GetCopyrightRecord 获取版权记录
// GET /api/compliance/copyrights/:contentId
func (h *Handler) GetCopyrightRecord(c *gin.Context) {
	contentID := c.Param("contentId")

	record, err := h.service.GetCopyrightRecord(c.Request.Context(), contentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if record == nil {
		c.JSON(http.StatusOK, gin.H{"copyright": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"copyright": record})
}

// ========== 风险提示 ==========

// ListRiskAlerts 获取风险提示列表
// GET /api/compliance/alerts
func (h *Handler) ListRiskAlerts(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	unresolvedOnly := c.Query("unresolvedOnly") == "true"

	// 普通用户只能看自己的提示
	queryUserID := ""
	if c.Query("all") != "true" {
		queryUserID = userID
	}

	alerts, total, err := h.service.ListRiskAlerts(c.Request.Context(), tenantID, queryUserID, unresolvedOnly, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts":   alerts,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// ResolveRiskAlert 解决风险提示
// POST /api/compliance/alerts/:id/resolve
func (h *Handler) ResolveRiskAlert(c *gin.Context) {
	_, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	if err := h.service.ResolveRiskAlert(c.Request.Context(), c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已解决"})
}

// ========== 合规报告 ==========

// GenerateComplianceReport 生成合规报告
// POST /api/compliance/reports
func (h *Handler) GenerateComplianceReport(c *gin.Context) {
	var req struct {
		ReportType  string `json:"reportType" binding:"required"`
		PeriodStart string `json:"periodStart" binding:"required"`
		PeriodEnd   string `json:"periodEnd" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	periodStart, _ := time.Parse("2006-01-02", req.PeriodStart)
	periodEnd, _ := time.Parse("2006-01-02", req.PeriodEnd)

	genReq := &compliance.GenerateReportRequest{
		TenantID:    tenantID,
		ReportType:  req.ReportType,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}

	report, err := h.service.GenerateComplianceReport(c.Request.Context(), genReq, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

// ListComplianceReports 获取报告列表
// GET /api/compliance/reports
func (h *Handler) ListComplianceReports(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	reports, total, err := h.service.ListComplianceReports(c.Request.Context(), tenantID, page, pageSize)
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

// GetComplianceReport 获取报告详情
// GET /api/compliance/reports/:id
func (h *Handler) GetComplianceReport(c *gin.Context) {
	report, err := h.service.GetComplianceReport(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// ========== 统计 ==========

// GetComplianceStats 获取合规统计
// GET /api/compliance/stats
func (h *Handler) GetComplianceStats(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	stats, err := h.service.GetComplianceStats(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
