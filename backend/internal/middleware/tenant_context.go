package middleware

import (
	"context"
	"net/http"

	tenantctx "backend/internal/tenant"
)

// TenantExtractor defines how tenant and user identity information is extracted from
// an incoming HTTP request. A concrete implementation is expected to parse whatever
// authentication mechanism is in use (e.g. JWT in Authorization header) and return
// a populated TenantContext.
type TenantExtractor interface {
	Extract(r *http.Request) (tenantctx.TenantContext, error)
}

// Logger is a minimal logging interface used by the middleware. It matches common
// logging libraries (zap, logrus, etc.) without forcing a specific dependency here.
type Logger interface {
	Warn(msg string, fields ...any)
}

// TenantContextMiddleware returns an http.Handler that ensures a TenantContext is
// available for downstream handlers. It relies on the provided TenantExtractor to
// parse identity information from the request; on failure it responds with 401 or
// 400 and does not call the next handler.
func TenantContextMiddleware(extractor TenantExtractor, logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if extractor == nil {
				logger.Warn("tenant extractor is not configured")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			tc, err := extractor.Extract(r)
			if err != nil {
				// 这里不直接区分所有错误类型，具体实现可以在 Extract 中区分认证失败与请求不合法等情况。
				logger.Warn("failed to extract tenant context", "error", err.Error())
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			if tc.TenantID == "" {
				logger.Warn("missing tenant id in extracted context")
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			ctx := tenantctx.WithTenantContext(r.Context(), tc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithTenantContext is a helper that can be used in non-HTTP entrypoints (e.g. background
// jobs) to attach a TenantContext to a context.Context when no HTTP middleware is involved.
func WithTenantContext(ctx context.Context, tc tenantctx.TenantContext) context.Context {
	return tenantctx.WithTenantContext(ctx, tc)
}
