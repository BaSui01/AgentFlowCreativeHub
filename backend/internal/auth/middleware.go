package auth

import (
	"context"
	"net/http"
	"strings"

	"backend/internal/tenant"

	"github.com/gin-gonic/gin"
)

// ContextKey 上下文键类型
type ContextKey string

const (
	// UserContextKey 用户上下文键
	UserContextKey ContextKey = "user"
	// TenantContextKey 租户上下文键
	TenantContextKey ContextKey = "tenant"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(jwtService *JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取令牌
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "缺少认证令牌",
			})
			c.Abort()
			return
		}

		// 提取纯令牌
		token := ExtractTokenFromBearer(authHeader)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "无效的令牌格式",
			})
			c.Abort()
			return
		}

		// 验证令牌
		claims, err := jwtService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "令牌验证失败: " + err.Error(),
			})
			c.Abort()
			return
		}

		// 确保是访问令牌
		if claims.TokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "令牌类型错误",
			})
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set(string(UserContextKey), &UserContext{
			UserID:   claims.UserID,
			TenantID: claims.TenantID,
			Roles:    claims.Roles,
		})

		c.Next()
	}
}

// OptionalAuthMiddleware 可选认证中间件（用于公开端点，如果有令牌则验证）
func OptionalAuthMiddleware(jwtService *JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		token := ExtractTokenFromBearer(authHeader)
		if token == "" {
			c.Next()
			return
		}

		claims, err := jwtService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			// 令牌无效但不拦截请求
			c.Next()
			return
		}

		if claims.TokenType == "access" {
			c.Set(string(UserContextKey), &UserContext{
				UserID:   claims.UserID,
				TenantID: claims.TenantID,
				Roles:    claims.Roles,
			})
		}

		c.Next()
	}
}

// RequirePermission 权限检查中间件
func RequirePermission(requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userCtx, exists := GetUserContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未认证",
			})
			c.Abort()
			return
		}

		// 检查权限
		if !hasPermission(userCtx.Roles, requiredPermissions) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "权限不足",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole 角色检查中间件
func RequireRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userCtx, exists := GetUserContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未认证",
			})
			c.Abort()
			return
		}

		// 检查角色
		if !hasRole(userCtx.Roles, requiredRoles) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "角色权限不足",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// UserContext 用户上下文
type UserContext struct {
	UserID   string
	TenantID string
	Roles    []string
}

// GetUserContext 从 Gin Context 获取用户上下文
func GetUserContext(c *gin.Context) (*UserContext, bool) {
	userCtx, exists := c.Get(string(UserContextKey))
	if !exists {
		return nil, false
	}

	ctx, ok := userCtx.(*UserContext)
	return ctx, ok
}

// GetTenantContext 从 Gin Context 获取租户上下文（兼容旧代码）
func GetTenantContext(c *gin.Context) *tenant.TenantContext {
	userCtx, exists := GetUserContext(c)
	if !exists {
		return nil
	}

	return &tenant.TenantContext{
		TenantID: userCtx.TenantID,
		UserID:   userCtx.UserID,
	}
}

// SetUserContext 在标准 context.Context 中设置用户上下文
func SetUserContext(ctx context.Context, userCtx *UserContext) context.Context {
	return context.WithValue(ctx, UserContextKey, userCtx)
}

// GetUserContextFromStdContext 从标准 context.Context 获取用户上下文
func GetUserContextFromStdContext(ctx context.Context) (*UserContext, bool) {
	userCtx, ok := ctx.Value(UserContextKey).(*UserContext)
	return userCtx, ok
}

// hasPermission 检查是否有指定权限（基于角色推断权限）
func hasPermission(userRoles []string, requiredPermissions []string) bool {
	// admin 角色拥有所有权限
	for _, role := range userRoles {
		if role == "admin" || role == "super_admin" {
			return true
		}
	}

	// 简化实现：权限等同于角色
	// 生产环境应该维护 role -> permissions 映射表
	for _, required := range requiredPermissions {
		if hasRole(userRoles, []string{required}) {
			return true
		}
	}

	return false
}

// hasRole 检查是否有指定角色
func hasRole(userRoles []string, requiredRoles []string) bool {
	roleMap := make(map[string]bool)
	for _, role := range userRoles {
		roleMap[strings.ToLower(role)] = true
	}

	for _, required := range requiredRoles {
		if roleMap[strings.ToLower(required)] {
			return true
		}
	}

	return false
}
