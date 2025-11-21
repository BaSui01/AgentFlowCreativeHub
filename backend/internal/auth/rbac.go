package auth

import (
	"net/http"

	tenantctx "backend/internal/tenant"
)

// PermissionChecker defines the contract for checking whether a user represented
// by the given TenantContext is allowed to perform a specific action on a
// resource. Implementations are expected to consult tenant-scoped role and
// permission assignments.
type PermissionChecker interface {
	HasPermission(tc tenantctx.TenantContext, resource, action string) (bool, error)
}

// Logger is a minimal logging interface used by the RequirePermission middleware.
// It is intentionally small so that different logging implementations can satisfy
// it without adapters.
type Logger interface {
	Warn(msg string, fields ...any)
}

// RequirePermissionHTTP returns an http.Handler middleware that enforces the given
// resource/action permission. It relies on a TenantContext being present in the
// request context (typically injected by TenantContextMiddleware).
func RequirePermissionHTTP(checker PermissionChecker, logger Logger, resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if checker == nil {
				logger.Warn("permission checker is not configured")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			tc, ok := tenantctx.FromContext(r.Context())
			if !ok {
				logger.Warn("missing TenantContext in request context")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			allowed, err := checker.HasPermission(tc, resource, action)
			if err != nil {
				logger.Warn("permission check failed", "error", err.Error(), "tenantId", tc.TenantID, "userId", tc.UserID, "resource", resource, "action", action)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if !allowed {
				logger.Warn("permission denied", "tenantId", tc.TenantID, "userId", tc.UserID, "resource", resource, "action", action)
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
