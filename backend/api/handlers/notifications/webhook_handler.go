package notifications

import (
	"net/http"

	"backend/api/handlers/common"
	"backend/internal/notification"

	"github.com/gin-gonic/gin"
)

// WebhookHandler Webhook管理处理器
type WebhookHandler struct {
	service *notification.WebhookService
}

// NewWebhookHandler 创建Webhook处理器
func NewWebhookHandler(service *notification.WebhookService) *WebhookHandler {
	return &WebhookHandler{service: service}
}

// RegisterEndpointRequest 注册端点请求
type RegisterEndpointRequest struct {
	Name    string            `json:"name" binding:"required"`
	URL     string            `json:"url" binding:"required,url"`
	Secret  string            `json:"secret"`
	Headers map[string]string `json:"headers"`
	Events  []string          `json:"events"` // 订阅的事件类型，空表示所有
}

// UpdateEndpointRequest 更新端点请求
type UpdateEndpointRequest struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Secret  string            `json:"secret"`
	Headers map[string]string `json:"headers"`
	Events  []string          `json:"events"`
}

// TestWebhookRequest 测试Webhook请求
type TestWebhookRequest struct {
	EventType string         `json:"event_type" binding:"required"`
	Payload   map[string]any `json:"payload"`
}

// RegisterEndpoint 注册Webhook端点
// @Summary 注册Webhook端点
// @Tags Webhook
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body RegisterEndpointRequest true "端点信息"
// @Success 200 {object} common.APIResponse{data=notification.WebhookEndpoint}
// @Router /api/webhooks [post]
func (h *WebhookHandler) RegisterEndpoint(c *gin.Context) {
	var req RegisterEndpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	endpoint := &notification.WebhookEndpoint{
		Name:    req.Name,
		URL:     req.URL,
		Secret:  req.Secret,
		Headers: req.Headers,
		Events:  req.Events,
	}

	if err := h.service.RegisterEndpoint(endpoint); err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, endpoint)
}

// ListEndpoints 列出所有端点
// @Summary 列出所有Webhook端点
// @Tags Webhook
// @Security BearerAuth
// @Produce json
// @Success 200 {object} common.APIResponse{data=[]notification.WebhookEndpoint}
// @Router /api/webhooks [get]
func (h *WebhookHandler) ListEndpoints(c *gin.Context) {
	endpoints := h.service.ListEndpoints()
	common.Success(c, endpoints)
}

// UpdateEndpoint 更新端点
// @Summary 更新Webhook端点
// @Tags Webhook
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "端点ID"
// @Param request body UpdateEndpointRequest true "更新信息"
// @Success 200 {object} common.APIResponse
// @Router /api/webhooks/{id} [put]
func (h *WebhookHandler) UpdateEndpoint(c *gin.Context) {
	id := c.Param("id")

	var req UpdateEndpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	update := &notification.WebhookEndpoint{
		Name:    req.Name,
		URL:     req.URL,
		Secret:  req.Secret,
		Headers: req.Headers,
		Events:  req.Events,
	}

	if err := h.service.UpdateEndpoint(id, update); err != nil {
		common.Error(c, http.StatusNotFound, err.Error())
		return
	}

	common.Success(c, nil)
}

// DeleteEndpoint 删除端点
// @Summary 删除Webhook端点
// @Tags Webhook
// @Security BearerAuth
// @Param id path string true "端点ID"
// @Success 200 {object} common.APIResponse
// @Router /api/webhooks/{id} [delete]
func (h *WebhookHandler) DeleteEndpoint(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteEndpoint(id); err != nil {
		common.Error(c, http.StatusNotFound, err.Error())
		return
	}

	common.Success(c, nil)
}

// SetActive 设置端点活动状态
// @Summary 设置Webhook端点活动状态
// @Tags Webhook
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "端点ID"
// @Param active query bool true "是否激活"
// @Success 200 {object} common.APIResponse
// @Router /api/webhooks/{id}/active [put]
func (h *WebhookHandler) SetActive(c *gin.Context) {
	id := c.Param("id")
	active := c.Query("active") == "true"

	if err := h.service.SetEndpointActive(id, active); err != nil {
		common.Error(c, http.StatusNotFound, err.Error())
		return
	}

	common.Success(c, nil)
}

// TestWebhook 测试Webhook
// @Summary 测试Webhook端点
// @Tags Webhook
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "端点ID"
// @Param request body TestWebhookRequest true "测试数据"
// @Success 200 {object} common.APIResponse{data=notification.WebhookDelivery}
// @Router /api/webhooks/{id}/test [post]
func (h *WebhookHandler) TestWebhook(c *gin.Context) {
	var req TestWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Payload == nil {
		req.Payload = map[string]any{
			"message": "This is a test webhook",
			"test":    true,
		}
	}

	deliveries, err := h.service.EmitSync(c.Request.Context(), req.EventType, req.Payload)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, deliveries)
}

// GetEventTypes 获取支持的事件类型
// @Summary 获取支持的Webhook事件类型
// @Tags Webhook
// @Security BearerAuth
// @Produce json
// @Success 200 {object} common.APIResponse
// @Router /api/webhooks/event-types [get]
func (h *WebhookHandler) GetEventTypes(c *gin.Context) {
	eventTypes := []map[string]string{
		{"type": notification.EventAgentExecuted, "description": "Agent执行完成"},
		{"type": notification.EventAgentFailed, "description": "Agent执行失败"},
		{"type": notification.EventWorkflowStarted, "description": "工作流开始执行"},
		{"type": notification.EventWorkflowCompleted, "description": "工作流执行完成"},
		{"type": notification.EventWorkflowFailed, "description": "工作流执行失败"},
		{"type": notification.EventDocumentIndexed, "description": "文档索引完成"},
		{"type": notification.EventUserCreated, "description": "用户创建"},
		{"type": notification.EventUserLoggedIn, "description": "用户登录"},
	}

	common.Success(c, eventTypes)
}
