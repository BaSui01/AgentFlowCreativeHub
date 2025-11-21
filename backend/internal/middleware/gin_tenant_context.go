package middleware

import (
	"net/http"
	"strings"

	"backend/internal/auth"
	tenantctx "backend/internal/tenant"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GinTenantContextMiddleware 将 JWT 中解析出的用户信息转换为 tenant.TenantContext，并注入标准 context.Context。
// 仅当上游已经通过 AuthMiddleware 验证身份后使用。
func GinTenantContextMiddleware(logger *zap.Logger) gin.HandlerFunc {
	log := logger
	if log == nil {
		log = zap.NewNop()
	}

	return func(c *gin.Context) {
		userCtx, exists := auth.GetUserContext(c)
		if !exists {
			log.Warn("missing user context before tenant middleware", zap.String("path", c.FullPath()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			return
		}

		tenantID := strings.TrimSpace(userCtx.TenantID)
		if tenantID == "" {
			log.Warn("token missing tenant id", zap.String("user", userCtx.UserID))
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "缺少租户信息"})
			return
		}

		tc := tenantctx.TenantContext{
			TenantID:      tenantID,
			UserID:        strings.TrimSpace(userCtx.UserID),
			Roles:         append([]string{}, userCtx.Roles...),
			IsSystemAdmin: hasSystemAdminRole(userCtx.Roles),
		}

		c.Set("tenant_id", tc.TenantID)
		c.Set("user_id", tc.UserID)

		ctx := tenantctx.WithTenantContext(c.Request.Context(), tc)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func hasSystemAdminRole(roles []string) bool {
	for _, r := range roles {
		clean := strings.ToLower(strings.TrimSpace(r))
		switch clean {
		case "super_admin", "system_admin":
			return true
		}
	}
	return false
}
