package kbsharing

import (
	"net/http"

	"backend/internal/auth"
	"backend/internal/rag"

	"github.com/gin-gonic/gin"
)

// Handler 知识库共享 Handler
type Handler struct {
	service *rag.KBSharingService
}

// NewHandler 创建 Handler
func NewHandler(service *rag.KBSharingService) *Handler {
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

// CreateShare 创建知识库共享
// @Summary 创建知识库共享
// @Description 创建知识库共享链接
// @Tags KBSharing
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param request body rag.CreateShareRequest true "共享信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/knowledge-bases/{id}/sharing [post]
func (h *Handler) CreateShare(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	kbID := c.Param("id")

	var req rag.CreateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.KnowledgeBaseID = kbID
	req.OwnerTenantID = tenantID
	req.CreatedBy = userID

	share, err := h.service.CreateShare(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"share": share})
}

// ListShares 列出知识库的共享
// @Summary 列出知识库共享
// @Description 获取知识库的所有共享
// @Tags KBSharing
// @Produce json
// @Param id path string true "知识库ID"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/knowledge-bases/{id}/sharing [get]
func (h *Handler) ListShares(c *gin.Context) {
	_, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	kbID := c.Param("id")

	shares, err := h.service.ListKBShares(c.Request.Context(), kbID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"shares": shares})
}

// GetShare 获取共享详情
// @Summary 获取共享详情
// @Description 获取共享详细信息
// @Tags KBSharing
// @Produce json
// @Param id path string true "知识库ID"
// @Param shareId path string true "共享ID"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/knowledge-bases/{id}/sharing/{shareId} [get]
func (h *Handler) GetShare(c *gin.Context) {
	_, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	shareID := c.Param("shareId")

	share, err := h.service.GetShare(c.Request.Context(), shareID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"share": share})
}

// UpdateShare 更新共享
// @Summary 更新共享
// @Description 更新共享配置
// @Tags KBSharing
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param shareId path string true "共享ID"
// @Param request body map[string]any true "更新字段"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/knowledge-bases/{id}/sharing/{shareId} [put]
func (h *Handler) UpdateShare(c *gin.Context) {
	_, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	shareID := c.Param("shareId")

	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateShare(c.Request.Context(), shareID, userID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "共享已更新"})
}

// DeleteShare 删除共享
// @Summary 删除共享
// @Description 删除知识库共享
// @Tags KBSharing
// @Produce json
// @Param id path string true "知识库ID"
// @Param shareId path string true "共享ID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/knowledge-bases/{id}/sharing/{shareId} [delete]
func (h *Handler) DeleteShare(c *gin.Context) {
	_, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	shareID := c.Param("shareId")

	if err := h.service.DeleteShare(c.Request.Context(), shareID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "共享已删除"})
}

// AcceptShare 接受共享
// @Summary 接受共享
// @Description 接受知识库共享邀请
// @Tags KBSharing
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param shareId path string true "共享ID"
// @Param request body map[string]string false "别名设置"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/knowledge-bases/{id}/sharing/{shareId}/accept [post]
func (h *Handler) AcceptShare(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	shareID := c.Param("shareId")

	var req struct {
		Alias string `json:"alias"`
	}
	c.ShouldBindJSON(&req)

	acceptance, err := h.service.AcceptShare(c.Request.Context(), shareID, tenantID, userID, req.Alias)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"acceptance": acceptance})
}

// RevokeAcceptance 撤销共享接受（离开共享）
// @Summary 撤销共享接受
// @Description 取消接受共享/离开共享
// @Tags KBSharing
// @Produce json
// @Param id path string true "知识库ID"
// @Param shareId path string true "共享ID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/knowledge-bases/{id}/sharing/{shareId}/accept [delete]
func (h *Handler) RevokeAcceptance(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	shareID := c.Param("shareId")

	if err := h.service.LeaveShare(c.Request.Context(), shareID, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已取消接受共享"})
}

// GetAccessLogs 获取共享访问日志
// @Summary 获取共享访问日志
// @Description 获取共享的访问记录
// @Tags KBSharing
// @Produce json
// @Param id path string true "知识库ID"
// @Param shareId path string true "共享ID"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/knowledge-bases/{id}/sharing/{shareId}/logs [get]
func (h *Handler) GetAccessLogs(c *gin.Context) {
	_, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	shareID := c.Param("shareId")

	logs, err := h.service.GetAccessLogs(c.Request.Context(), shareID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// GetSharedKB 通过 token 验证共享
// @Summary 通过token获取共享知识库
// @Description 通过共享token验证并获取知识库
// @Tags KBSharing
// @Produce json
// @Param token path string true "共享Token"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/shared-kb/{token} [get]
func (h *Handler) GetSharedKB(c *gin.Context) {
	_, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	token := c.Param("token")

	share, err := h.service.ValidateShareToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"share": share})
}

// SearchSharedKB 在共享的知识库中搜索
// @Summary 搜索共享知识库
// @Description 在共享知识库中进行搜索
// @Tags KBSharing
// @Accept json
// @Produce json
// @Param token path string true "共享Token"
// @Param request body map[string]string true "搜索条件"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/shared-kb/{token}/search [post]
func (h *Handler) SearchSharedKB(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	token := c.Param("token")

	// 先验证 token 获取 shareID
	share, err := h.service.ValidateShareToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := h.service.QuerySharedKB(c.Request.Context(), share.ID, tenantID, userID, req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// ListReceivedShares 列出收到的共享
// @Summary 列出收到的共享
// @Description 获取收到的所有知识库共享
// @Tags KBSharing
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/received-shares [get]
func (h *Handler) ListReceivedShares(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	shares, err := h.service.ListReceivedShares(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"shares": shares})
}
