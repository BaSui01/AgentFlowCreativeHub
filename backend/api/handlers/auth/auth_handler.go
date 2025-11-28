package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	auditpkg "backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// RegisterRequest 用户注册请求
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	FullName string `json:"full_name"`
}

// Register 用户注册
// @Summary 用户注册
// @Description 注册新用户账号
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "注册请求参数"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 409 {object} map[string]string "用户已存在"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /api/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 检查邮箱是否已存在
	if existing, err := h.identityStore.FindActiveUserByEmail(c.Request.Context(), req.Email); err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "邮箱已被注册"})
		return
	}

	// 检查用户名是否已存在
	var existingUser struct{ ID string }
	if err := h.db.WithContext(c.Request.Context()).
		Table("users").
		Where("LOWER(username) = ? AND deleted_at IS NULL", strings.ToLower(req.Username)).
		Select("id").
		First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已被使用"})
		return
	}

	// 获取或创建默认租户
	tenantID, err := h.resolveTenantID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法确定租户"})
		return
	}

	// 开始事务
	tx := h.db.WithContext(c.Request.Context()).Begin()
	if err := tx.Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建事务失败"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 生成密码哈希
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 创建用户
	userID := uuid.New().String()
	now := time.Now().UTC()
	fullName := req.FullName
	if fullName == "" {
		fullName = req.Username
	}

	userPayload := map[string]any{
		"id":             userID,
		"tenant_id":      tenantID,
		"email":          strings.ToLower(req.Email),
		"username":       req.Username,
		"full_name":      fullName,
		"password_hash":  string(hashBytes),
		"email_verified": false,
		"status":         "active",
		"created_at":     now,
		"updated_at":     now,
	}

	if err := tx.Table("users").Create(userPayload).Error; err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusConflict, gin.H{"error": "用户已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	// 分配默认角色
	if err := h.assignDefaultRole(c.Request.Context(), tx, userID, tenantID); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分配角色失败"})
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}

	// 查询新创建的用户并登录
	identity, err := h.identityStore.FindActiveUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册成功，但登录失败"})
		return
	}

	auditpkg.SetAuditMetadata(c, "user_id", identity.ID)
	auditpkg.SetAuditMetadata(c, "action", "register")

	// 自动登录
	h.respondWithSession(c, identity, "local")
}

// ForgotPasswordRequest 忘记密码请求
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword 忘记密码
// @Summary 忘记密码
// @Description 发送密码重置邮件（当前版本生成重置令牌并返回）
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ForgotPasswordRequest true "忘记密码请求"
// @Success 200 {object} map[string]string "重置令牌已生成"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 404 {object} map[string]string "用户不存在"
// @Router /api/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 查询用户
	identity, err := h.identityStore.FindActiveUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			// 为了安全，不透露用户是否存在
			c.JSON(http.StatusOK, gin.H{
				"message": "如果该邮箱已注册，重置链接将发送至邮箱",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
		return
	}

	// 生成重置令牌
	resetToken := uuid.New().String()
	expiresAt := time.Now().Add(1 * time.Hour)

	// 保存重置令牌到数据库（使用 password_reset_tokens 表或存储到 Redis）
	tokenData := map[string]any{
		"id":         uuid.New().String(),
		"user_id":    identity.ID,
		"token":      resetToken,
		"expires_at": expiresAt,
		"used":       false,
		"created_at": time.Now().UTC(),
	}

	if err := h.db.WithContext(c.Request.Context()).
		Table("password_reset_tokens").
		Create(tokenData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成重置令牌失败"})
		return
	}

	auditpkg.SetAuditMetadata(c, "email", req.Email)
	auditpkg.SetAuditMetadata(c, "action", "forgot_password")

	// TODO: 实际项目中应发送邮件，这里暂时返回令牌用于测试
	c.JSON(http.StatusOK, gin.H{
		"message":     "如果该邮箱已注册，重置链接将发送至邮箱",
		"reset_token": resetToken, // 仅用于开发测试，生产环境应删除
	})
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ResetPassword 重置密码
// @Summary 重置密码
// @Description 使用重置令牌重置密码
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "重置密码请求"
// @Success 200 {object} map[string]string "密码重置成功"
// @Failure 400 {object} map[string]string "参数错误或令牌无效"
// @Router /api/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 查询重置令牌
	var tokenRecord struct {
		ID        string
		UserID    string
		Token     string
		ExpiresAt time.Time
		Used      bool
	}

	if err := h.db.WithContext(c.Request.Context()).
		Table("password_reset_tokens").
		Where("token = ? AND used = false", req.Token).
		First(&tokenRecord).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的重置令牌"})
		return
	}

	// 检查令牌是否过期
	if time.Now().After(tokenRecord.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "重置令牌已过期"})
		return
	}

	// 生成新密码哈希
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 开始事务
	tx := h.db.WithContext(c.Request.Context()).Begin()
	if err := tx.Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建事务失败"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新密码
	if err := tx.Table("users").
		Where("id = ?", tokenRecord.UserID).
		Updates(map[string]any{
			"password_hash": string(hashBytes),
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}

	// 标记令牌为已使用
	if err := tx.Table("password_reset_tokens").
		Where("id = ?", tokenRecord.ID).
		Update("used", true).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新令牌状态失败"})
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码重置失败"})
		return
	}

	auditpkg.SetAuditMetadata(c, "user_id", tokenRecord.UserID)
	auditpkg.SetAuditMetadata(c, "action", "reset_password")

	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功"})
}

// resolveTenantID 解析租户ID
func (h *AuthHandler) resolveTenantID(ctx context.Context) (string, error) {
	// 尝试从数据库获取第一个租户
	var tenant struct{ ID string }
	if err := h.db.WithContext(ctx).
		Table("tenants").
		Where("deleted_at IS NULL").
		Select("id").
		Order("created_at ASC").
		First(&tenant).Error; err == nil {
		return tenant.ID, nil
	}

	return "", auth.ErrTenantUnavailable
}

// assignDefaultRole 分配默认角色
func (h *AuthHandler) assignDefaultRole(ctx context.Context, tx *gorm.DB, userID, tenantID string) error {
	var role struct {
		ID string
	}

	roleQuery := tx.WithContext(ctx).
		Table("roles").
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("is_default DESC, priority DESC, created_at ASC").
		Select("id").
		Limit(1)

	if err := roleQuery.Scan(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 如果没有默认角色，尝试查找名为 "user" 的角色
			if err := tx.WithContext(ctx).
				Table("roles").
				Where("name = ? AND deleted_at IS NULL", "user").
				Select("id").
				First(&role).Error; err != nil {
				return fmt.Errorf("no default role found")
			}
		} else {
			return err
		}
	}

	userRole := map[string]any{
		"id":         uuid.New().String(),
		"user_id":    userID,
		"role_id":    role.ID,
		"created_at": time.Now().UTC(),
		"updated_at": time.Now().UTC(),
	}

	return tx.Table("user_roles").Create(userRole).Error
}

// generateRandomState 生成随机 state
func generateRandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
