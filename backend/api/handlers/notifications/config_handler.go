package notifications

import (
	"strconv"

	"backend/internal/common"
	"backend/internal/notification"

	"github.com/gin-gonic/gin"
)

// NotificationConfigHandler 通知配置Handler
type NotificationConfigHandler struct {
	configService notification.NotificationConfigServiceInterface
}

// NewNotificationConfigHandler 创建通知配置Handler
func NewNotificationConfigHandler(svc notification.NotificationConfigServiceInterface) *NotificationConfigHandler {
	return &NotificationConfigHandler{configService: svc}
}

// GetUserPreferences 获取用户通知偏好
// GET /api/notifications/preferences
func (h *NotificationConfigHandler) GetUserPreferences(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")

	prefs, err := h.configService.GetUserPreferences(c.Request.Context(), tenantID, userID)
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccess(c, prefs)
}

// UpdateUserPreferences 更新用户通知偏好
// PUT /api/notifications/preferences
func (h *NotificationConfigHandler) UpdateUserPreferences(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")

	var req notification.NotificationPreferences
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ResponseBadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.configService.UpdateUserPreferences(c.Request.Context(), tenantID, userID, &req); err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "更新成功", nil)
}

// ResetUserPreferences 重置用户偏好为默认值
// POST /api/notifications/preferences/reset
func (h *NotificationConfigHandler) ResetUserPreferences(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")

	if err := h.configService.ResetUserPreferences(c.Request.Context(), tenantID, userID); err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "已重置为默认设置", nil)
}

// EnableChannel 启用通知渠道
// POST /api/notifications/channels/:channel/enable
func (h *NotificationConfigHandler) EnableChannel(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")
	channel := notification.NotificationChannel(c.Param("channel"))

	if err := h.configService.EnableChannel(c.Request.Context(), tenantID, userID, channel); err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "渠道已启用", nil)
}

// DisableChannel 禁用通知渠道
// POST /api/notifications/channels/:channel/disable
func (h *NotificationConfigHandler) DisableChannel(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")
	channel := notification.NotificationChannel(c.Param("channel"))

	if err := h.configService.DisableChannel(c.Request.Context(), tenantID, userID, channel); err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "渠道已禁用", nil)
}

// GetEnabledChannels 获取启用的渠道列表
// GET /api/notifications/channels
func (h *NotificationConfigHandler) GetEnabledChannels(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")

	channels, err := h.configService.GetEnabledChannels(c.Request.Context(), tenantID, userID)
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccess(c, gin.H{"channels": channels})
}

// SubscribeCategory 订阅通知分类
// POST /api/notifications/categories/:category/subscribe
func (h *NotificationConfigHandler) SubscribeCategory(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")
	category := notification.NotificationCategory(c.Param("category"))

	if err := h.configService.SubscribeCategory(c.Request.Context(), tenantID, userID, category); err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "已订阅该类别通知", nil)
}

// UnsubscribeCategory 取消订阅通知分类
// POST /api/notifications/categories/:category/unsubscribe
func (h *NotificationConfigHandler) UnsubscribeCategory(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")
	category := notification.NotificationCategory(c.Param("category"))

	if err := h.configService.UnsubscribeCategory(c.Request.Context(), tenantID, userID, category); err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "已取消订阅该类别通知", nil)
}

// SetQuietHours 设置静音时段
// POST /api/notifications/quiet-hours
func (h *NotificationConfigHandler) SetQuietHours(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")

	var req struct {
		Start   string `json:"start" binding:"required"`   // HH:MM
		End     string `json:"end" binding:"required"`     // HH:MM
		Enabled bool   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ResponseBadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.configService.SetQuietHours(c.Request.Context(), tenantID, userID, req.Start, req.End, req.Enabled); err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "静音时段已设置", nil)
}

// GetNotificationHistory 获取通知历史
// GET /api/notifications/history?limit=50
func (h *NotificationConfigHandler) GetNotificationHistory(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")
	userID := c.GetString("user_id")

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history, err := h.configService.GetNotificationHistory(c.Request.Context(), tenantID, userID, limit)
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccess(c, gin.H{
		"items": history,
		"total": len(history),
	})
}

// GetNotificationStats 获取通知统计
// GET /api/notifications/stats?start_time=2024-01-01&end_time=2024-12-31
func (h *NotificationConfigHandler) GetNotificationStats(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")

	startTime := c.DefaultQuery("start_time", "")
	endTime := c.DefaultQuery("end_time", "")

	stats, err := h.configService.GetNotificationStats(c.Request.Context(), tenantID, startTime, endTime)
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccess(c, stats)
}

// GetTenantConfig 获取租户通知配置
// GET /api/notifications/config/tenant
func (h *NotificationConfigHandler) GetTenantConfig(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")

	config, err := h.configService.GetTenantConfig(c.Request.Context(), tenantID)
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccess(c, config)
}

// UpdateTenantConfig 更新租户通知配置（需要管理员权限）
// PUT /api/notifications/config/tenant
func (h *NotificationConfigHandler) UpdateTenantConfig(c *gin.Context) {
	tenantID := c.GetUint("tenant_id")

	var config notification.TenantNotificationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		common.ResponseBadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.configService.UpdateTenantConfig(c.Request.Context(), tenantID, &config); err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "租户配置已更新", nil)
}
