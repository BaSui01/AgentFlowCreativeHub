package tenant

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"backend/internal/auth"
	tenantSvc "backend/internal/tenant"
)

type fakeTenantService struct {
	lastCtx    context.Context
	lastCreate tenantSvc.CreateTenantParams
}

func (f *fakeTenantService) CreateTenant(ctx context.Context, params tenantSvc.CreateTenantParams) (*tenantSvc.Tenant, error) {
	f.lastCtx = ctx
	f.lastCreate = params
	return &tenantSvc.Tenant{ID: "tenant-1", Name: params.Name, Slug: params.Slug}, nil
}

func (f *fakeTenantService) ListTenants(ctx context.Context, limit, offset int) ([]*tenantSvc.Tenant, int64, error) {
	// 返回测试数据
	tenants := []*tenantSvc.Tenant{
		{ID: "tenant-1", Name: "Acme Inc", Slug: "acme", Status: "active"},
		{ID: "tenant-2", Name: "Beta Corp", Slug: "beta", Status: "active"},
	}
	return tenants, 2, nil
}

func (f *fakeTenantService) GetTenant(ctx context.Context, id string) (*tenantSvc.Tenant, error) {
	if id == "tenant-1" {
		return &tenantSvc.Tenant{
			ID:     "tenant-1",
			Name:   "Acme Inc",
			Slug:   "acme",
			Status: "active",
		}, nil
	}
	return nil, errors.New("tenant not found")
}

func (f *fakeTenantService) UpdateTenant(ctx context.Context, id string, params tenantSvc.UpdateTenantParams) (*tenantSvc.Tenant, error) {
	name := "Updated Tenant"
	if params.Name != nil {
		name = *params.Name
	}
	return &tenantSvc.Tenant{
		ID:     id,
		Name:   name,
		Slug:   "updated-slug",
		Status: "active",
	}, nil
}

func (f *fakeTenantService) DeleteTenant(ctx context.Context, id string) error {
	return nil
}

func (f *fakeTenantService) SelfRegisterTenant(context.Context, tenantSvc.SelfRegisterTenantParams) (*tenantSvc.Tenant, *tenantSvc.User, error) {
	return nil, nil, nil
}

type noopPasswordHasher struct{}

func (noopPasswordHasher) Hash(password string) (string, error) {
	return "hashed-" + password, nil
}

func TestTenantHandlerCreateTenantHashesPasswordAndInjectsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeTenantService{}
	h := NewTenantHandler(service, nil, nil, nil, noopPasswordHasher{})
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/tenants", bytes.NewBufferString(`{"name":"Acme","slug":"acme","adminEmail":"admin@example.com","adminUsername":"admin","adminPassword":"secret"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("tenant_id", "sys")
	c.Set("user_id", "sys-user")
	c.Set(string(auth.UserContextKey), &auth.UserContext{
		UserID:   "sys-user",
		TenantID: "sys",
		Roles:    []string{"super_admin"},
	})

	h.CreateTenant(c)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.Code)
	}
	if service.lastCreate.AdminPasswordHash != "hashed-secret" {
		t.Fatalf("expected hashed password, got %q", service.lastCreate.AdminPasswordHash)
	}
	if service.lastCreate.AdminPasswordHash == "secret" {
		t.Fatalf("password was not hashed")
	}

	tc, ok := tenantSvc.FromContext(service.lastCtx)
	if !ok {
		t.Fatalf("tenant context missing in service call")
	}
	if !tc.IsSystemAdmin {
		t.Fatalf("expected system admin flag to be true")
	}
	if tc.UserID != "sys-user" {
		t.Fatalf("expected user id sys-user, got %s", tc.UserID)
	}
}


// ============================================================
// HTTP Integration Tests - 测试完整HTTP请求响应流程
// ============================================================

// TestTenantHandler_ListTenants_HTTP 测试租户列表HTTP接口
func TestTenantHandler_ListTenants_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功返回租户列表", func(t *testing.T) {
		service := &fakeTenantService{}
		h := NewTenantHandler(service, nil, nil, nil, noopPasswordHasher{})
		
		router := gin.New()
		router.GET("/api/tenants", func(c *gin.Context) {
			c.Set("tenant_id", "sys")
			c.Set("user_id", "sys-user")
			c.Set(string(auth.UserContextKey), &auth.UserContext{
				UserID:   "sys-user",
				TenantID: "sys",
				Roles:    []string{"super_admin"},
			})
			h.ListTenants(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tenants?page=1&pageSize=10", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if !bytes.Contains([]byte(body), []byte("Acme Inc")) {
			t.Fatalf("expected response to contain 'Acme Inc', got %s", body)
		}
	})

	t.Run("HTTP_分页参数验证", func(t *testing.T) {
		service := &fakeTenantService{}
		h := NewTenantHandler(service, nil, nil, nil, noopPasswordHasher{})
		
		router := gin.New()
		router.GET("/api/tenants", func(c *gin.Context) {
			c.Set("tenant_id", "sys")
			c.Set("user_id", "sys-user")
			c.Set(string(auth.UserContextKey), &auth.UserContext{
				UserID:   "sys-user",
				TenantID: "sys",
				Roles:    []string{"super_admin"},
			})
			h.ListTenants(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tenants?page=2&pageSize=20", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})
}

// TestTenantHandler_GetTenant_HTTP 测试获取单个租户HTTP接口
func TestTenantHandler_GetTenant_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功返回租户详情", func(t *testing.T) {
		service := &fakeTenantService{}
		h := NewTenantHandler(service, nil, nil, nil, noopPasswordHasher{})
		
		router := gin.New()
		router.GET("/api/tenants/:id", func(c *gin.Context) {
			c.Set("tenant_id", "sys")
			c.Set("user_id", "sys-user")
			c.Set(string(auth.UserContextKey), &auth.UserContext{
				UserID:   "sys-user",
				TenantID: "sys",
				Roles:    []string{"super_admin"},
			})
			h.GetTenant(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tenants/tenant-1", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if !bytes.Contains([]byte(body), []byte("Acme Inc")) {
			t.Fatalf("expected response to contain 'Acme Inc', got %s", body)
		}
	})

	t.Run("HTTP_租户不存在返回404", func(t *testing.T) {
		service := &fakeTenantService{}
		h := NewTenantHandler(service, nil, nil, nil, noopPasswordHasher{})
		
		router := gin.New()
		router.GET("/api/tenants/:id", func(c *gin.Context) {
			c.Set("tenant_id", "sys")
			c.Set("user_id", "sys-user")
			c.Set(string(auth.UserContextKey), &auth.UserContext{
				UserID:   "sys-user",
				TenantID: "sys",
				Roles:    []string{"super_admin"},
			})
			h.GetTenant(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tenants/nonexistent", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}
	})
}

// TestTenantHandler_UpdateTenant_HTTP 测试更新租户HTTP接口
func TestTenantHandler_UpdateTenant_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功更新租户", func(t *testing.T) {
		service := &fakeTenantService{}
		h := NewTenantHandler(service, nil, nil, nil, noopPasswordHasher{})
		
		router := gin.New()
		router.PUT("/api/tenants/:id", func(c *gin.Context) {
			c.Set("tenant_id", "sys")
			c.Set("user_id", "sys-user")
			c.Set(string(auth.UserContextKey), &auth.UserContext{
				UserID:   "sys-user",
				TenantID: "sys",
				Roles:    []string{"super_admin"},
			})
			h.UpdateTenant(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/tenants/tenant-1", bytes.NewBufferString(`{"name":"Updated Name"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if !bytes.Contains([]byte(body), []byte("Updated Name")) {
			t.Fatalf("expected response to contain 'Updated Name', got %s", body)
		}
	})

	t.Run("HTTP_参数错误返回400", func(t *testing.T) {
		service := &fakeTenantService{}
		h := NewTenantHandler(service, nil, nil, nil, noopPasswordHasher{})
		
		router := gin.New()
		router.PUT("/api/tenants/:id", func(c *gin.Context) {
			c.Set("tenant_id", "sys")
			c.Set("user_id", "sys-user")
			c.Set(string(auth.UserContextKey), &auth.UserContext{
				UserID:   "sys-user",
				TenantID: "sys",
				Roles:    []string{"super_admin"},
			})
			h.UpdateTenant(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/tenants/tenant-1", bytes.NewBufferString(`invalid json`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})
}

// TestTenantHandler_DeleteTenant_HTTP 测试删除租户HTTP接口
func TestTenantHandler_DeleteTenant_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功删除租户", func(t *testing.T) {
		service := &fakeTenantService{}
		h := NewTenantHandler(service, nil, nil, nil, noopPasswordHasher{})
		
		router := gin.New()
		router.DELETE("/api/tenants/:id", func(c *gin.Context) {
			c.Set("tenant_id", "sys")
			c.Set("user_id", "sys-user")
			c.Set(string(auth.UserContextKey), &auth.UserContext{
				UserID:   "sys-user",
				TenantID: "sys",
				Roles:    []string{"super_admin"},
			})
			h.DeleteTenant(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/tenants/tenant-1", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", w.Code)
		}
	})
}
