package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/auth"
	tenantctx "backend/internal/tenant"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestGinTenantContextMiddlewareInjectsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(auth.UserContextKey), &auth.UserContext{
			UserID:   "user-1",
			TenantID: "tenant-1",
			Roles:    []string{"super_admin"},
		})
		c.Next()
	})
	r.Use(GinTenantContextMiddleware(zap.NewNop()))
	r.GET("/protected", func(c *gin.Context) {
		if tc, ok := tenantctx.FromContext(c.Request.Context()); !ok || tc.TenantID != "tenant-1" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
}

func TestGinTenantContextMiddlewareRejectsMissingUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(GinTenantContextMiddleware(zap.NewNop()))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.Code)
	}
}
