package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"backend/internal/tenant"
)

// MiddlewareFactory provides authentication and authorization middleware.
type MiddlewareFactory struct {
	checker PermissionChecker
}

// NewMiddlewareFactory creates a new MiddlewareFactory.
func NewMiddlewareFactory(checker PermissionChecker) *MiddlewareFactory {
	return &MiddlewareFactory{
		checker: checker,
	}
}

// RequirePermission returns a Gin middleware that checks if the user has the required permission on the resource.
func (f *MiddlewareFactory) RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get TenantContext from Gin context (populated by TenantContextMiddleware)
		// We need to make sure TenantContextMiddleware runs before this.
		// Assuming TenantContextMiddleware sets the context in a way that tenant.FromContext can retrieve it,
		// or we can construct it from Gin context.

		// tenant.FromContext works on context.Context.
		// Middleware usually sets it on request context.

		tc, ok := tenant.FromContext(c.Request.Context())
		if !ok {
			// Try to build it from Gin context if not in request context yet
			// This depends on how TenantContextMiddleware is implemented.
			// Looking at setup.go: middlewarepkg.GinTenantContextMiddleware(logger.Get())
			// Let's assume it sets it on request context.

			// If not, we can try to get it from Gin keys if stored there.
			// But tenant.FromContext is the standard way.

			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		allowed, err := f.checker.HasPermission(tc, resource, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "permission check failed"})
			c.Abort()
			return
		}

		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
			c.Abort()
			return
		}

		c.Next()
	}
}
