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
// POST /api/apikeys
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
// GET /api/apikeys
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
// POST /api/apikeys/:id/revoke
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
// DELETE /api/apikeys/:id
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
