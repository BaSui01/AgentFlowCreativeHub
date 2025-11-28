package content

import (
	"net/http"
	"strconv"

	"backend/internal/content"

	"github.com/gin-gonic/gin"
)

// ListComments 获取作品评论列表
// GET /api/content/works/:id/comments
func (h *Handler) ListComments(c *gin.Context) {
	tenantID, _, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	
	workID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	
	response, err := h.service.ListComments(c.Request.Context(), tenantID, workID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, response)
}

// CreateComment 发表评论
// POST /api/content/works/:id/comments
func (h *Handler) CreateComment(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	
	workID := c.Param("id")
	
	var req content.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	req.WorkID = workID
	
	comment, err := h.service.CreateComment(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"comment": comment})
}

// ToggleLike 点赞/取消点赞作品
// POST /api/content/works/:id/like
func (h *Handler) ToggleLike(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	
	workID := c.Param("id")
	
	response, err := h.service.ToggleLike(c.Request.Context(), tenantID, userID, workID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, response)
}
