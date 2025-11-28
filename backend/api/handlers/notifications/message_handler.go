package notifications

import (
	"net/http"

	"backend/api/handlers/common"
	"backend/internal/notification"

	"github.com/gin-gonic/gin"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	service *notification.MessageService
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(service *notification.MessageService) *MessageHandler {
	return &MessageHandler{service: service}
}

// ListMessages 查询消息列表
// @Summary 查询消息列表
// @Description 查询用户的消息列表（支持过滤和分页）
// @Tags Messages
// @Produce json
// @Param status query string false "消息状态" Enums(unread, read)
// @Param category query string false "消息分类"
// @Param priority query string false "优先级" Enums(low, normal, high, urgent)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} common.PagedResponse{data=[]notification.UserMessage}
// @Router /api/messages [get]
func (h *MessageHandler) ListMessages(c *gin.Context) {
	var req notification.ListMessagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	messages, total, err := h.service.ListMessages(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "查询消息失败: " + err.Error()})
		return
	}

	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	totalPage := (int(total) + pageSize - 1) / pageSize

	c.JSON(http.StatusOK, common.ListResponse{
		Items: messages,
		Pagination: common.PaginationMeta{
			Page:      req.Page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// GetMessage 获取消息详情
// @Summary 获取消息详情
// @Description 根据ID获取消息详情
// @Tags Messages
// @Produce json
// @Param id path string true "消息ID"
// @Success 200 {object} common.Response{data=notification.UserMessage}
// @Router /api/messages/{id} [get]
func (h *MessageHandler) GetMessage(c *gin.Context) {
	messageID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	msg, err := h.service.GetMessage(c.Request.Context(), tenantID, messageID)
	if err != nil {
		c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "消息不存在: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: msg})
}

// MarkAsRead 标记消息为已读
// @Summary 标记消息为已读
// @Description 将指定消息标记为已读状态
// @Tags Messages
// @Produce json
// @Param id path string true "消息ID"
// @Success 200 {object} common.Response
// @Router /api/messages/{id}/read [post]
func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	messageID := c.Param("id")
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if err := h.service.MarkAsRead(c.Request.Context(), tenantID, userID, messageID); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "标记已读失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// MarkAsUnread 标记消息为未读
// @Summary 标记消息为未读
// @Description 将指定消息标记为未读状态
// @Tags Messages
// @Produce json
// @Param id path string true "消息ID"
// @Success 200 {object} common.Response
// @Router /api/messages/{id}/unread [post]
func (h *MessageHandler) MarkAsUnread(c *gin.Context) {
	messageID := c.Param("id")
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if err := h.service.MarkAsUnread(c.Request.Context(), tenantID, userID, messageID); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "标记未读失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// MarkAllAsReadRequest 全部标记已读请求
type MarkAllAsReadRequest struct {
	Category *notification.NotificationCategory `json:"category,omitempty"`
}

// MarkAllAsRead 标记所有消息为已读
// @Summary 标记所有消息为已读
// @Description 将用户所有消息（或指定分类）标记为已读
// @Tags Messages
// @Accept json
// @Produce json
// @Param request body MarkAllAsReadRequest false "请求参数"
// @Success 200 {object} common.Response
// @Router /api/messages/read-all [post]
func (h *MessageHandler) MarkAllAsRead(c *gin.Context) {
	var req MarkAllAsReadRequest
	_ = c.ShouldBindJSON(&req) // 可选参数，忽略错误

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if err := h.service.MarkAllAsRead(c.Request.Context(), tenantID, userID, req.Category); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "批量标记已读失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// BatchMarkAsReadRequest 批量标记已读请求
type BatchMarkAsReadRequest struct {
	MessageIDs []string `json:"message_ids" binding:"required,min=1"`
}

// BatchMarkAsRead 批量标记消息为已读
// @Summary 批量标记消息为已读
// @Description 批量将指定消息标记为已读
// @Tags Messages
// @Accept json
// @Produce json
// @Param request body BatchMarkAsReadRequest true "批量标记请求"
// @Success 200 {object} common.Response
// @Router /api/messages/batch-read [post]
func (h *MessageHandler) BatchMarkAsRead(c *gin.Context) {
	var req BatchMarkAsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if err := h.service.BatchMarkAsRead(c.Request.Context(), tenantID, userID, req.MessageIDs); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "批量标记已读失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// GetUnreadCount 获取未读消息数量
// @Summary 获取未读消息数量
// @Description 获取用户的未读消息总数
// @Tags Messages
// @Produce json
// @Success 200 {object} common.Response{data=int64}
// @Router /api/messages/unread-count [get]
func (h *MessageHandler) GetUnreadCount(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	count, err := h.service.GetUnreadCount(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "获取未读数量失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: count})
}

// GetUnreadCountByCategory 按分类获取未读消息数量
// @Summary 按分类获取未读消息数量
// @Description 获取用户各分类的未读消息数量
// @Tags Messages
// @Produce json
// @Success 200 {object} common.Response{data=map[string]int64}
// @Router /api/messages/unread-count/by-category [get]
func (h *MessageHandler) GetUnreadCountByCategory(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	counts, err := h.service.GetUnreadCountByCategory(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "获取分类未读数量失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: counts})
}

// DeleteMessage 删除消息
// @Summary 删除消息
// @Description 删除指定消息（软删除）
// @Tags Messages
// @Produce json
// @Param id path string true "消息ID"
// @Success 200 {object} common.Response
// @Router /api/messages/{id} [delete]
func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	messageID := c.Param("id")
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if err := h.service.DeleteMessage(c.Request.Context(), tenantID, userID, messageID); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "删除消息失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// BatchDeleteMessagesRequest 批量删除请求
type BatchDeleteMessagesRequest struct {
	MessageIDs []string `json:"message_ids" binding:"required,min=1"`
}

// BatchDeleteMessages 批量删除消息
// @Summary 批量删除消息
// @Description 批量删除指定消息
// @Tags Messages
// @Accept json
// @Produce json
// @Param request body BatchDeleteMessagesRequest true "批量删除请求"
// @Success 200 {object} common.Response
// @Router /api/messages/batch-delete [post]
func (h *MessageHandler) BatchDeleteMessages(c *gin.Context) {
	var req BatchDeleteMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if err := h.service.BatchDeleteMessages(c.Request.Context(), tenantID, userID, req.MessageIDs); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "批量删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// GetStats 获取消息统计
// @Summary 获取消息统计
// @Description 获取用户的消息统计信息
// @Tags Messages
// @Produce json
// @Success 200 {object} common.Response{data=notification.MessageStats}
// @Router /api/messages/stats [get]
func (h *MessageHandler) GetStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	stats, err := h.service.GetStats(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "获取统计失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: stats})
}
