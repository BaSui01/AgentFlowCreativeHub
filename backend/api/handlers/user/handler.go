package user

import (
	"net/http"
	"strconv"

	"backend/internal/auth"
	"backend/internal/user"

	"github.com/gin-gonic/gin"
)

// Handler 用户资料 Handler
type Handler struct {
	service *user.ProfileService
}

// NewHandler 创建 Handler
func NewHandler(service *user.ProfileService) *Handler {
	return &Handler{service: service}
}

// getUserID 获取当前用户ID
func getUserID(c *gin.Context) (string, bool) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		return "", false
	}
	return userCtx.UserID, true
}

// GetProfile 获取用户资料
// @Summary 获取用户资料
// @Description 获取当前用户的个人资料信息
// @Tags User
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/profile [get]
func (h *Handler) GetProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	profile, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"profile": profile})
}

// UpdateProfile 更新用户资料
// @Summary 更新用户资料
// @Description 更新当前用户的个人资料
// @Tags User
// @Accept json
// @Produce json
// @Param request body user.UserProfile true "用户资料"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/profile [put]
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var profile user.UserProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile.UserID = userID
	if err := h.service.UpdateProfile(c.Request.Context(), &profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"profile": profile})
}

// GetPreferences 获取用户偏好设置
// @Summary 获取用户偏好设置
// @Description 获取当前用户的偏好设置
// @Tags User
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/preferences [get]
func (h *Handler) GetPreferences(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	prefs, err := h.service.GetPreferences(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"preferences": prefs})
}

// UpdatePreferences 更新用户偏好设置
// @Summary 更新用户偏好设置
// @Description 更新当前用户的偏好设置
// @Tags User
// @Accept json
// @Produce json
// @Param request body user.UserPreferences true "偏好设置"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/preferences [put]
func (h *Handler) UpdatePreferences(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var prefs user.UserPreferences
	if err := c.ShouldBindJSON(&prefs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdatePreferences(c.Request.Context(), userID, &prefs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "偏好设置已更新"})
}

// GetActivity 获取用户活动统计
// @Summary 获取用户活动统计
// @Description 获取当前用户的活动统计数据
// @Tags User
// @Produce json
// @Param days query int false "统计天数"
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/user/activity [get]
func (h *Handler) GetActivity(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	activity, err := h.service.GetActivity(c.Request.Context(), userID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"activity": activity})
}
