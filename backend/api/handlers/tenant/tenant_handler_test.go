package tenant

import (
	"bytes"
	"context"
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

func (f *fakeTenantService) ListTenants(context.Context, int, int) ([]*tenantSvc.Tenant, int64, error) {
	return nil, 0, nil
}

func (f *fakeTenantService) GetTenant(context.Context, string) (*tenantSvc.Tenant, error) {
	return nil, nil
}

func (f *fakeTenantService) UpdateTenant(context.Context, string, tenantSvc.UpdateTenantParams) (*tenantSvc.Tenant, error) {
	return nil, nil
}

func (f *fakeTenantService) DeleteTenant(context.Context, string) error {
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
