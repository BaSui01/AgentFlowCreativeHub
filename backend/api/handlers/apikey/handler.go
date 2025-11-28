package apikey

import (
	"net/http"

	"backend/internal/auth"

	"github.com/gin-gonic/gin"
)

// Handler API Key 管理 Handler
type Handler struct {
	service *auth.APIKeyService
}

// NewHandler 创建 Handler
func NewHandler(service *auth.APIKeyService) *Handler {
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

// CreateAPIKey 创建 API Key
// @Summary 创建 API Key
// @Description 创建新的 API Key
// @Tags APIKey
// @Accept json
// @Produce json
// @Param request body auth.CreateAPIKeyRequest true "API Key 信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/apikeys [post]
func (h *Handler) CreateAPIKey(c *gin.Context) {
	var req auth.CreateAPIKeyRequest
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

	resp, err := h.service.CreateAPIKey(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "API Key 创建成功，请妥善保存密钥，此后将无法再次查看完整密钥",
		"apiKey":  resp,
	})
}

// ListAPIKeys 列出 API Keys
// @Summary 列出 API Keys
// @Description 获取用户的 API Key 列表
// @Tags APIKey
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/apikeys [get]
func (h *Handler) ListAPIKeys(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	keys, err := h.service.ListAPIKeys(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"apiKeys": keys,
		"total":   len(keys),
	})
}

// RevokeAPIKey 撤销 API Key
// @Summary 撤销 API Key
// @Description 撤销指定的 API Key（禁用但不删除）
// @Tags APIKey
// @Produce json
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/apikeys/{id}/revoke [post]
func (h *Handler) RevokeAPIKey(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	keyID := c.Param("id")
	if err := h.service.RevokeAPIKey(c.Request.Context(), tenantID, keyID, userID); err != nil {
		if err == auth.ErrAPIKeyNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "API Key 不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API Key 已撤销"})
}

// DeleteAPIKey 删除 API Key
// @Summary 删除 API Key
// @Description 永久删除指定的 API Key
// @Tags APIKey
// @Produce json
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/apikeys/{id} [delete]
func (h *Handler) DeleteAPIKey(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	keyID := c.Param("id")
	if err := h.service.DeleteAPIKey(c.Request.Context(), tenantID, keyID); err != nil {
		if err == auth.ErrAPIKeyNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "API Key 不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API Key 已删除"})
}
