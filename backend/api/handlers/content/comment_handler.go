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

// DeleteComment 删除评论
// @Summary 删除评论
// @Description 删除指定的评论（仅作者或管理员可删除）
// @Tags Content
// @Security BearerAuth
// @Produce json
// @Param id path string true "作品ID"
// @Param commentId path string true "评论ID"
// @Success 200 {object} map[string]any
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/content/works/{id}/comments/{commentId} [delete]
func (h *Handler) DeleteComment(c *gin.Context) {
	tenantID, userID, ok := getUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	
	commentID := c.Param("commentId")
	
	if err := h.service.DeleteComment(c.Request.Context(), tenantID, userID, commentID); err != nil {
		if err.Error() == "评论不存在" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "无权删除此评论" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "评论删除成功"})
}

// ToggleLike 点赞/取消点赞作品
// @Summary 点赞/取消点赞作品
// @Description 对作品进行点赞或取消点赞操作
// @Tags Content
// @Security BearerAuth
// @Produce json
// @Param id path string true "作品ID"
// @Success 200 {object} map[string]any
// @Router /api/content/works/{id}/like [post]
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
