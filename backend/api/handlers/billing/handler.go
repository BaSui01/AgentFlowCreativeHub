package billing

import (
	"net/http"
	"strconv"
	"time"

	response "backend/api/handlers/common"
	"backend/internal/billing"

	"github.com/gin-gonic/gin"
)

// Handler 计费管理 API 处理器
type Handler struct {
	service *billing.Service
}

// NewHandler 创建处理器
func NewHandler(service *billing.Service) *Handler {
	return &Handler{service: service}
}

// ============================================================================
// 模型定价管理
// ============================================================================

// ListPricings 获取定价列表
// @Summary 获取模型定价列表
// @Tags Billing
// @Security BearerAuth
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} response.ListResponse{items=[]billing.ModelPricing}
// @Router /api/billing/pricings [get]
func (h *Handler) ListPricings(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	pricings, total, err := h.service.ListPricings(c.Request.Context(), tenantID, page, pageSize)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPage := (int(total) + pageSize - 1) / pageSize
	c.JSON(http.StatusOK, response.ListResponse{
		Items: pricings,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// GetPricing 获取单个定价
// @Summary 获取模型定价详情
// @Tags Billing
// @Security BearerAuth
// @Produce json
// @Param provider query string true "提供商"
// @Param model query string true "模型名称"
// @Success 200 {object} response.APIResponse{data=billing.ModelPricing}
// @Router /api/billing/pricings/query [get]
func (h *Handler) GetPricing(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	provider := c.Query("provider")
	model := c.Query("model")

	if provider == "" || model == "" {
		response.Error(c, http.StatusBadRequest, "provider和model参数必填")
		return
	}

	pricing, err := h.service.GetPricing(c.Request.Context(), tenantID, provider, model)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, pricing)
}

// CreatePricing 创建定价
// @Summary 创建模型定价
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body billing.CreatePricingRequest true "定价信息"
// @Success 200 {object} response.APIResponse{data=billing.ModelPricing}
// @Router /api/billing/pricings [post]
func (h *Handler) CreatePricing(c *gin.Context) {
	var req billing.CreatePricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	req.TenantID = c.GetString("tenant_id")

	pricing, err := h.service.CreatePricing(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, pricing)
}

// UpdatePricing 更新定价
// @Summary 更新模型定价
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "定价ID"
// @Param request body billing.UpdatePricingRequest true "更新信息"
// @Success 200 {object} response.APIResponse
// @Router /api/billing/pricings/{id} [put]
func (h *Handler) UpdatePricing(c *gin.Context) {
	id := c.Param("id")

	var req billing.UpdatePricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdatePricing(c.Request.Context(), id, &req); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// DeletePricing 删除定价
// @Summary 删除模型定价
// @Tags Billing
// @Security BearerAuth
// @Param id path string true "定价ID"
// @Success 200 {object} response.APIResponse
// @Router /api/billing/pricings/{id} [delete]
func (h *Handler) DeletePricing(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeletePricing(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============================================================================
// 成本预估
// ============================================================================

// EstimateCost 预估成本
// @Summary 预估调用成本
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body billing.CostEstimateRequest true "预估请求"
// @Success 200 {object} response.APIResponse{data=billing.CostEstimate}
// @Router /api/billing/estimate [post]
func (h *Handler) EstimateCost(c *gin.Context) {
	var req billing.CostEstimateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	req.TenantID = c.GetString("tenant_id")

	estimate, err := h.service.EstimateCost(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, estimate)
}

// ============================================================================
// 成本报表
// ============================================================================

// GenerateCostReport 生成成本报表
// @Summary 生成成本报表
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body billing.CostReportRequest true "报表请求"
// @Success 200 {object} response.APIResponse{data=billing.CostReport}
// @Router /api/billing/reports [post]
func (h *Handler) GenerateCostReport(c *gin.Context) {
	var req billing.CostReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	req.TenantID = c.GetString("tenant_id")

	report, err := h.service.GenerateCostReport(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, report)
}

// GetCostTrend 获取成本趋势
// @Summary 获取成本趋势
// @Tags Billing
// @Security BearerAuth
// @Produce json
// @Param days query int false "天数" default(30)
// @Success 200 {object} response.APIResponse{data=[]billing.DailyCostItem}
// @Router /api/billing/trend [get]
func (h *Handler) GetCostTrend(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	now := time.Now()
	startDate := now.AddDate(0, 0, -days)

	req := &billing.CostReportRequest{
		TenantID:  tenantID,
		StartDate: &startDate,
		EndDate:   &now,
	}

	report, err := h.service.GenerateCostReport(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, report.DailyTrend)
}

// ============================================================================
// 成本告警
// ============================================================================

// ListAlerts 获取告警列表
// @Summary 获取成本告警列表
// @Tags Billing
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse{data=[]billing.CostAlert}
// @Router /api/billing/alerts [get]
func (h *Handler) ListAlerts(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	alerts, err := h.service.ListAlerts(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, alerts)
}

// GetAlert 获取告警详情
// @Summary 获取成本告警详情
// @Tags Billing
// @Security BearerAuth
// @Param id path string true "告警ID"
// @Success 200 {object} response.APIResponse{data=billing.CostAlert}
// @Router /api/billing/alerts/{id} [get]
func (h *Handler) GetAlert(c *gin.Context) {
	id := c.Param("id")

	alert, err := h.service.GetAlert(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, alert)
}

// CreateAlert 创建告警
// @Summary 创建成本告警
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body billing.CreateAlertRequest true "告警配置"
// @Success 200 {object} response.APIResponse{data=billing.CostAlert}
// @Router /api/billing/alerts [post]
func (h *Handler) CreateAlert(c *gin.Context) {
	var req billing.CreateAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	req.TenantID = c.GetString("tenant_id")

	alert, err := h.service.CreateAlert(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, alert)
}

// UpdateAlert 更新告警
// @Summary 更新成本告警
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "告警ID"
// @Param request body billing.UpdateAlertRequest true "更新信息"
// @Success 200 {object} response.APIResponse
// @Router /api/billing/alerts/{id} [put]
func (h *Handler) UpdateAlert(c *gin.Context) {
	id := c.Param("id")

	var req billing.UpdateAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateAlert(c.Request.Context(), id, &req); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// DeleteAlert 删除告警
// @Summary 删除成本告警
// @Tags Billing
// @Security BearerAuth
// @Param id path string true "告警ID"
// @Success 200 {object} response.APIResponse
// @Router /api/billing/alerts/{id} [delete]
func (h *Handler) DeleteAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteAlert(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// CheckAlerts 检查告警
// @Summary 检查成本告警（手动触发）
// @Tags Billing
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse{data=[]billing.AlertTriggerEvent}
// @Router /api/billing/alerts/check [post]
func (h *Handler) CheckAlerts(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	events, err := h.service.CheckAlerts(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, events)
}

// ============================================================================
// 计费审计
// ============================================================================

// QueryBillingAudit 查询计费审计
// @Summary 查询计费审计记录
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body billing.BillingAuditQuery true "查询条件"
// @Success 200 {object} response.APIResponse
// @Router /api/billing/audit [post]
func (h *Handler) QueryBillingAudit(c *gin.Context) {
	var query billing.BillingAuditQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	query.TenantID = c.GetString("tenant_id")

	records, summary, total, err := h.service.QueryBillingAudit(c.Request.Context(), &query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPage := (int(total) + query.PageSize - 1) / query.PageSize
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"records": records,
			"summary": summary,
			"pagination": response.PaginationMeta{
				Page:      query.Page,
				PageSize:  query.PageSize,
				Total:     total,
				TotalPage: totalPage,
			},
		},
	})
}

// ============================================================================
// Token 计价器
// ============================================================================

// CalculateTokenCost 计算 Token 成本
// @Summary Token 计价器
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body billing.TokenCalculatorRequest true "计算请求"
// @Success 200 {object} response.APIResponse{data=billing.TokenCalculatorResult}
// @Router /api/billing/calculator [post]
func (h *Handler) CalculateTokenCost(c *gin.Context) {
	var req billing.TokenCalculatorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	req.TenantID = c.GetString("tenant_id")

	result, err := h.service.CalculateTokenCost(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, result)
}

// ============================================================================
// 定价策略
// ============================================================================

// ListStrategies 获取策略列表
// @Summary 获取定价策略列表
// @Tags Billing
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse{data=[]billing.PricingStrategy}
// @Router /api/billing/strategies [get]
func (h *Handler) ListStrategies(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	strategies, err := h.service.ListStrategies(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, strategies)
}

// CreateStrategy 创建策略
// @Summary 创建定价策略
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body billing.PricingStrategy true "策略配置"
// @Success 200 {object} response.APIResponse
// @Router /api/billing/strategies [post]
func (h *Handler) CreateStrategy(c *gin.Context) {
	var strategy billing.PricingStrategy
	if err := c.ShouldBindJSON(&strategy); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	strategy.TenantID = c.GetString("tenant_id")

	if err := h.service.CreateStrategy(c.Request.Context(), &strategy); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, strategy)
}

// UpdateStrategy 更新策略
// @Summary 更新定价策略
// @Tags Billing
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "策略ID"
// @Param request body billing.PricingStrategy true "策略配置"
// @Success 200 {object} response.APIResponse
// @Router /api/billing/strategies/{id} [put]
func (h *Handler) UpdateStrategy(c *gin.Context) {
	id := c.Param("id")

	var strategy billing.PricingStrategy
	if err := c.ShouldBindJSON(&strategy); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateStrategy(c.Request.Context(), id, &strategy); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// DeleteStrategy 删除策略
// @Summary 删除定价策略
// @Tags Billing
// @Security BearerAuth
// @Param id path string true "策略ID"
// @Success 200 {object} response.APIResponse
// @Router /api/billing/strategies/{id} [delete]
func (h *Handler) DeleteStrategy(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteStrategy(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}
