package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	auditpkg "backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	jwtService     *auth.JWTService
	oauth2Service  *auth.OAuth2Service
	sessionService *models.SessionService
	auditService   *models.AuditLogService
	db             *gorm.DB
	identityStore  *auth.IdentityStore
	stateStore     auth.StateStore
	stateTTL       time.Duration
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(
	jwtService *auth.JWTService,
	oauth2Service *auth.OAuth2Service,
	sessionService *models.SessionService,
	auditService *models.AuditLogService,
	db *gorm.DB,
	identityStore *auth.IdentityStore,
	stateStore auth.StateStore,
) *AuthHandler {
	if identityStore == nil {
		identityStore = auth.NewIdentityStore(db, "")
	}
	if stateStore == nil {
		stateStore = auth.NewMemoryStateStore(10 * time.Minute)
	}
	return &AuthHandler{
		jwtService:     jwtService,
		oauth2Service:  oauth2Service,
		sessionService: sessionService,
		auditService:   auditService,
		db:             db,
		identityStore:  identityStore,
		stateStore:     stateStore,
		stateTTL:       10 * time.Minute,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	*auth.TokenPair
	User *UserInfo `json:"user"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	TenantID string   `json:"tenant_id"`
	Roles    []string `json:"roles"`
}

// Login 用户登录
// @Summary 用户登录
// @Description 使用邮箱和密码登录，获取访问令牌和刷新令牌
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录请求参数"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 401 {object} map[string]string "认证失败"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	identity, err := h.identityStore.FindActiveUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			auditpkg.SetAuditMetadata(c, "email", req.Email)
			auditpkg.SetAuditMetadata(c, "error", "用户不存在")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}

	if identity.PasswordHash == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "该账户未设置密码，请使用 OAuth 登录"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(identity.PasswordHash), []byte(req.Password)); err != nil {
		auditpkg.SetAuditMetadata(c, "email", req.Email)
		auditpkg.SetAuditMetadata(c, "error", "密码错误")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	h.respondWithSession(c, identity, "local")
}

// RefreshRequest 刷新令牌请求
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh 刷新访问令牌
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "刷新令牌请求参数"
// @Success 200 {object} auth.TokenPair
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 401 {object} map[string]string "无效的刷新令牌"
// @Router /api/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 验证会话
	session, err := h.sessionService.GetSessionByRefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的刷新令牌"})
		return
	}

	// 刷新令牌
	tokenPair, err := h.jwtService.RefreshAccessToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "刷新令牌失败"})
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := h.sessionService.RotateRefreshToken(c.Request.Context(), session.ID, tokenPair.RefreshToken, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新会话失败"})
		return
	}

	auditpkg.SetAuditMetadata(c, "session_id", session.ID)

	c.JSON(http.StatusOK, tokenPair)
}

// Logout 用户登出
// @Summary 用户登出
// @Description 撤销当前会话或指定刷新令牌，并将当前访问令牌加入黑名单
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body object{refresh_token=string} false "可选：指定要撤销的刷新令牌"
// @Success 200 {object} map[string]string "登出成功"
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	// 如果请求体中有 refresh_token，撤销该会话
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		_ = h.sessionService.RevokeSessionByRefreshToken(c.Request.Context(), req.RefreshToken)
	}

	// 撤销当前 Access Token (加入黑名单)
	authHeader := c.GetHeader("Authorization")
	tokenString := auth.ExtractTokenFromBearer(authHeader)
	if tokenString != "" {
		if err := h.jwtService.InvalidateToken(c.Request.Context(), tokenString); err != nil {
			// 记录错误但不中断登出流程
			auditpkg.SetAuditMetadata(c, "logout_error", err.Error())
		}
	}

	// 从上下文获取用户信息，撤销所有会话（可选）
	if userCtx, exists := auth.GetUserContext(c); exists {
		auditpkg.SetAuditMetadata(c, "user_id", userCtx.UserID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "登出成功"})
}

// GetOAuth2AuthURL 获取 OAuth2 授权 URL
// @Summary 获取 OAuth2 授权 URL
// @Description 获取指定提供商（如 google, github）的 OAuth2 授权跳转 URL
// @Tags Auth
// @Produce json
// @Param provider path string true "OAuth2 提供商 (google, github)"
// @Success 200 {object} map[string]string "包含 auth_url 和 state"
// @Failure 400 {object} map[string]string "错误信息"
// @Router /api/auth/oauth/{provider} [get]
func (h *AuthHandler) GetOAuth2AuthURL(c *gin.Context) {
	provider := c.Param("provider")

	// 生成随机 state
	state, err := generateRandomState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 state 失败"})
		return
	}

	if err := h.stateStore.Save(c.Request.Context(), state, provider, h.stateTTL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存 state 失败"})
		return
	}

	authURL, err := h.oauth2Service.GetAuthURL(auth.OAuth2Provider(provider), state)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

// OAuth2Callback OAuth2 回调处理
// @Summary OAuth2 回调处理
// @Description 处理 OAuth2 提供商的回调，交换令牌并登录/注册用户
// @Tags Auth
// @Produce json
// @Param provider path string true "OAuth2 提供商 (google, github)"
// @Param code query string true "授权码"
// @Param state query string true "状态码"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]string "参数或验证错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /api/auth/oauth/{provider}/callback [post]
func (h *AuthHandler) OAuth2Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少参数"})
		return
	}

	storedProvider, err := h.stateStore.Consume(c.Request.Context(), state)
	if err != nil || storedProvider != provider {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state 验证失败"})
		return
	}

	// 交换授权码为访问令牌
	token, err := h.oauth2Service.ExchangeCode(c.Request.Context(), auth.OAuth2Provider(provider), code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交换授权码失败: " + err.Error()})
		return
	}

	// 获取用户信息
	userInfo, err := h.oauth2Service.GetUserInfo(c.Request.Context(), auth.OAuth2Provider(provider), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败: " + err.Error()})
		return
	}

	identity, err := h.identityStore.EnsureOAuthUser(c.Request.Context(), userInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "同步用户信息失败"})
		return
	}

	auditpkg.SetAuditMetadata(c, "provider", provider)
	auditpkg.SetAuditMetadata(c, "oauth2_user_id", userInfo.ID)
	auditpkg.SetAuditMetadata(c, "user_id", identity.ID)

	h.respondWithSession(c, identity, provider)
}

func (h *AuthHandler) respondWithSession(c *gin.Context, identity *auth.Identity, provider string) {
	tokenPair, err := h.jwtService.GenerateTokenPair(identity.ID, identity.TenantID, identity.Roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	session := &models.Session{
		UserID:       identity.ID,
		TenantID:     identity.TenantID,
		RefreshToken: tokenPair.RefreshToken,
		Provider:     provider,
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		LastUsedAt:   time.Now(),
	}

	if err := h.sessionService.CreateSession(c.Request.Context(), session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建会话失败"})
		return
	}

	auditpkg.SetAuditMetadata(c, "user_id", identity.ID)
	auditpkg.SetAuditMetadata(c, "provider", provider)

	c.JSON(http.StatusOK, &LoginResponse{
		TokenPair: tokenPair,
		User:      buildUserInfo(identity),
	})
}

func buildUserInfo(identity *auth.Identity) *UserInfo {
	return &UserInfo{
		ID:       identity.ID,
		Email:    identity.Email,
		Name:     identity.Name,
		TenantID: identity.TenantID,
		Roles:    identity.Roles,
	}
}

// generateRandomState 生成随机 state
func generateRandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
