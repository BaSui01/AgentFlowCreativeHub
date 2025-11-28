package subscription

import (
	"net/http"

	"backend/internal/auth"
	"backend/internal/subscription"

	"github.com/gin-gonic/gin"
)

// Handler 订阅 API 处理器
type Handler struct {
	service *subscription.Service
}

// NewHandler 创建处理器
func NewHandler(service *subscription.Service) *Handler {
	return &Handler{service: service}
}

// getUserContext 获取用户上下文辅助方法
func getUserContext(c *gin.Context) (tenantID, userID string, ok bool) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		return "", "", false
	}
	return userCtx.TenantID, userCtx.UserID, true
}

// ========== 套餐管理 ==========

// CreatePlan 创建套餐
// @Summary 创建订阅套餐
// @Description 创建新的订阅套餐（管理员）
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body subscription.CreatePlanRequest true "套餐信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/subscription/plans [post]
func (h *Handler) CreatePlan(c *gin.Context) {
	var req subscription.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	plan, err := h.service.CreatePlan(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, plan)
}

// ListPlans 列出套餐
// @Summary 获取套餐列表
// @Description 获取所有可用的订阅套餐
// @Tags Subscription
// @Produce json
// @Param includeGlobal query bool false "是否包含全局套餐"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/subscription/plans [get]
func (h *Handler) ListPlans(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	includeGlobal := c.Query("includeGlobal") != "false"

	plans, err := h.service.ListPlans(c.Request.Context(), tenantID, includeGlobal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plans": plans,
		"total": len(plans),
	})
}

// GetPlan 获取套餐详情
// @Summary 获取套餐详情
// @Description 根据ID获取套餐详细信息
// @Tags Subscription
// @Produce json
// @Param id path string true "套餐ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/subscription/plans/{id} [get]
func (h *Handler) GetPlan(c *gin.Context) {
	planID := c.Param("id")
	plan, err := h.service.GetPlan(c.Request.Context(), planID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == subscription.ErrPlanNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plan)
}

// UpdatePlan 更新套餐
// @Summary 更新套餐
// @Description 更新订阅套餐信息（管理员）
// @Tags Subscription
// @Accept json
// @Produce json
// @Param id path string true "套餐ID"
// @Param request body map[string]any true "更新字段"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/subscription/plans/{id} [put]
func (h *Handler) UpdatePlan(c *gin.Context) {
	planID := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdatePlan(c.Request.Context(), planID, updates); err != nil {
		status := http.StatusInternalServerError
		if err == subscription.ErrPlanNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeletePlan 删除套餐
// @Summary 删除套餐
// @Description 删除订阅套餐（管理员）
// @Tags Subscription
// @Produce json
// @Param id path string true "套餐ID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/subscription/plans/{id} [delete]
func (h *Handler) DeletePlan(c *gin.Context) {
	planID := c.Param("id")
	if err := h.service.DeletePlan(c.Request.Context(), planID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ========== 订阅管理 ==========

// Subscribe 订阅套餐
// @Summary 订阅套餐
// @Description 用户订阅指定套餐
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body subscription.SubscribeRequest true "订阅请求"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/subscription/subscribe [post]
func (h *Handler) Subscribe(c *gin.Context) {
	var req subscription.SubscribeRequest
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
	if req.UserID == "" {
		req.UserID = userID
	}

	sub, err := h.service.Subscribe(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case subscription.ErrPlanNotFound:
			status = http.StatusNotFound
		case subscription.ErrAlreadySubscribed, subscription.ErrTrialUsed:
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, sub)
}

// GetCurrentSubscription 获取当前订阅
// @Summary 获取当前订阅
// @Description 获取当前用户的订阅信息
// @Tags Subscription
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/subscription/current [get]
func (h *Handler) GetCurrentSubscription(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	sub, err := h.service.GetUserSubscription(c.Request.Context(), tenantID, userID)
	if err != nil {
		if err == subscription.ErrSubscriptionNotFound {
			plan, _ := h.service.GetDefaultPlan(c.Request.Context(), tenantID)
			c.JSON(http.StatusOK, gin.H{
				"subscription": nil,
				"defaultPlan":  plan,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	plan, _ := h.service.GetPlan(c.Request.Context(), sub.PlanID)
	c.JSON(http.StatusOK, gin.H{
		"subscription": sub,
		"plan":         plan,
	})
}

// GetUserSubscription 获取指定用户订阅（管理员）
// @Summary 获取用户订阅
// @Description 获取指定用户的订阅信息（管理员）
// @Tags Subscription
// @Produce json
// @Param userId path string true "用户ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/subscription/users/{userId} [get]
func (h *Handler) GetUserSubscription(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	userID := c.Param("userId")

	sub, err := h.service.GetUserSubscription(c.Request.Context(), tenantID, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == subscription.ErrSubscriptionNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	plan, _ := h.service.GetPlan(c.Request.Context(), sub.PlanID)
	c.JSON(http.StatusOK, gin.H{
		"subscription": sub,
		"plan":         plan,
	})
}

// ListUserSubscriptions 获取用户订阅历史
// @Summary 获取订阅历史
// @Description 获取用户的订阅历史记录
// @Tags Subscription
// @Produce json
// @Param userId query string false "用户ID（可选，默认当前用户）"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/subscription/history [get]
func (h *Handler) ListUserSubscriptions(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	queryUserID := c.DefaultQuery("userId", userID)
	subs, err := h.service.ListUserSubscriptions(c.Request.Context(), tenantID, queryUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscriptions": subs,
		"total":         len(subs),
	})
}

// CancelSubscription 取消订阅
// @Summary 取消订阅
// @Description 取消当前订阅
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body subscription.CancelRequest true "取消请求"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/subscription/cancel [post]
func (h *Handler) CancelSubscription(c *gin.Context) {
	var req subscription.CancelRequest
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
	if req.UserID == "" {
		req.UserID = userID
	}

	if err := h.service.CancelSubscription(c.Request.Context(), &req); err != nil {
		status := http.StatusInternalServerError
		if err == subscription.ErrSubscriptionNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "取消成功"})
}

// RenewSubscription 续订
// @Summary 续订订阅
// @Description 续订当前订阅
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body map[string]any true "续订请求"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/subscription/renew [post]
func (h *Handler) RenewSubscription(c *gin.Context) {
	var req struct {
		SubscriptionID string                    `json:"subscriptionId" binding:"required"`
		BillingCycle   subscription.BillingCycle `json:"billingCycle"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.service.RenewSubscription(c.Request.Context(), req.SubscriptionID, req.BillingCycle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sub)
}

// ChangePlan 更换套餐
// @Summary 更换套餐
// @Description 更换订阅套餐
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body map[string]any true "更换请求"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/subscription/change-plan [post]
func (h *Handler) ChangePlan(c *gin.Context) {
	var req struct {
		SubscriptionID string `json:"subscriptionId" binding:"required"`
		NewPlanID      string `json:"newPlanId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.service.ChangePlan(c.Request.Context(), req.SubscriptionID, req.NewPlanID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sub)
}

// ========== 试用管理 ==========

// StartTrial 开始试用
// @Summary 开始试用
// @Description 开始套餐试用
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body map[string]any true "试用请求"
// @Success 201 {object} map[string]any
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/subscription/trial/start [post]
func (h *Handler) StartTrial(c *gin.Context) {
	var req struct {
		PlanID string `json:"planId" binding:"required"`
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

	sub, err := h.service.StartTrial(c.Request.Context(), tenantID, userID, req.PlanID)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case subscription.ErrPlanNotFound:
			status = http.StatusNotFound
		case subscription.ErrTrialUsed:
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, sub)
}

// ConvertTrial 试用转正
// @Summary 试用转正
// @Description 将试用订阅转为正式订阅
// @Tags Subscription
// @Accept json
// @Produce json
// @Param request body map[string]any true "转正请求"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/subscription/trial/convert [post]
func (h *Handler) ConvertTrial(c *gin.Context) {
	var req struct {
		SubscriptionID string                    `json:"subscriptionId" binding:"required"`
		BillingCycle   subscription.BillingCycle `json:"billingCycle"`
		AutoRenew      bool                      `json:"autoRenew"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.service.ConvertTrial(c.Request.Context(), req.SubscriptionID, req.BillingCycle, req.AutoRenew)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sub)
}

// ========== 统计 ==========

// GetStats 获取订阅统计
// @Summary 获取订阅统计
// @Description 获取租户订阅统计数据（管理员）
// @Tags Subscription
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/subscription/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	stats, err := h.service.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// CheckExpiring 检查即将到期订阅
// @Summary 检查即将到期订阅
// @Description 获取即将到期的订阅列表（管理员）
// @Tags Subscription
// @Produce json
// @Param days query int false "提前天数(3/7/14/30)"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/subscription/expiring [get]
func (h *Handler) CheckExpiring(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	days := 7
	if d := c.Query("days"); d != "" {
		switch d {
		case "3":
			days = 3
		case "14":
			days = 14
		case "30":
			days = 30
		}
	}

	subs, err := h.service.CheckExpiring(c.Request.Context(), tenantID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscriptions": subs,
		"total":         len(subs),
		"daysAhead":     days,
	})
}

