package tenant

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	response "backend/api/handlers/common"
	"backend/internal/auth"
	tenantSvc "backend/internal/tenant"
)

type fakeRoleService struct {
	lastCtx      context.Context
	lastCreate   tenantSvc.CreateRoleParams
	lastUpdate   tenantSvc.UpdateRoleParams
	lastUpdateID string
	permissions  []*tenantSvc.Permission
	userRoles    map[string][]string
}

func (f *fakeRoleService) CreateRole(ctx context.Context, params tenantSvc.CreateRoleParams) (*tenantSvc.Role, error) {
	f.lastCtx = ctx
	f.lastCreate = params
	return &tenantSvc.Role{ID: "role-1", Name: params.Name, Description: params.Description}, nil
}

func (f *fakeRoleService) UpdateRole(ctx context.Context, id string, params tenantSvc.UpdateRoleParams) (*tenantSvc.Role, error) {
	f.lastCtx = ctx
	f.lastUpdate = params
	f.lastUpdateID = id
	return &tenantSvc.Role{ID: id, Name: params.Name, Description: params.Description}, nil
}

func (f *fakeRoleService) DeleteRole(ctx context.Context, id string) error {
	return nil
}

func (f *fakeRoleService) AssignRoleToUser(ctx context.Context, userID, roleID string) error {
	return nil
}

func (f *fakeRoleService) ReplaceUserRoles(ctx context.Context, userID string, roleIDs []string) error {
	return nil
}

func (f *fakeRoleService) UpdateRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	return nil
}

func (f *fakeRoleService) GetUserPermissions(ctx context.Context, userID string) ([]*tenantSvc.Permission, error) {
	return nil, nil
}

func (f *fakeRoleService) ListUserRoles(ctx context.Context, userID string) ([]string, error) {
	f.lastCtx = ctx
	if f.userRoles == nil {
		return nil, nil
	}
	roles := f.userRoles[userID]
	return append([]string(nil), roles...), nil
}

func (f *fakeRoleService) ListPermissions(ctx context.Context) ([]*tenantSvc.Permission, error) {
	f.lastCtx = ctx
	return f.permissions, nil
}

func (f *fakeRoleService) ListRoles(ctx context.Context) ([]*tenantSvc.RoleWithPermissions, error) {
	return nil, nil
}

func TestTenantHandler_CreateRole_WithPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeRoleService{}
	h := NewTenantHandler(nil, nil, service, nil, nil)
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)

	reqBody := map[string]interface{}{
		"name":          "Editor",
		"description":   "Can edit posts",
		"permissionIds": []string{"perm-1", "perm-2"},
	}
	jsonBody, _ := json.Marshal(reqBody)

	c.Request = httptest.NewRequest(http.MethodPost, "/api/tenants/t1/roles", bytes.NewBuffer(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("tenant_id", "t1")
	c.Set(string(auth.UserContextKey), &auth.UserContext{
		UserID:   "u1",
		TenantID: "t1",
		Roles:    []string{"admin"},
	})

	h.CreateRole(c)

	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Equal(t, "Editor", service.lastCreate.Name)
	assert.Equal(t, []string{"perm-1", "perm-2"}, service.lastCreate.PermissionIDs)
}

func TestTenantHandler_ListPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expectedPerms := []*tenantSvc.Permission{
		{ID: "perm-1", Resource: "posts", Action: "create"},
		{ID: "perm-2", Resource: "posts", Action: "delete"},
	}
	service := &fakeRoleService{permissions: expectedPerms}
	h := NewTenantHandler(nil, nil, service, nil, nil)
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)

	c.Request = httptest.NewRequest(http.MethodGet, "/api/tenant/permissions", nil)
	c.Set("tenant_id", "t1")
	c.Set(string(auth.UserContextKey), &auth.UserContext{
		UserID:   "u1",
		TenantID: "t1",
		Roles:    []string{"admin"},
	})

	h.ListPermissions(c)

	assert.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		Items []*tenantSvc.Permission `json:"items"`
	}
	err := json.Unmarshal(resp.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Len(t, body.Items, 2)
	assert.Equal(t, "posts", body.Items[0].Resource)
}

func TestTenantHandler_ListUserRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeRoleService{userRoles: map[string][]string{
		"user-1": {"role-a", "role-b"},
	}}
	h := NewTenantHandler(nil, nil, service, nil, nil)
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)

	c.Params = []gin.Param{{Key: "id", Value: "tenant-1"}, {Key: "userId", Value: "user-1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/tenants/tenant-1/users/user-1/roles", nil)
	c.Set("tenant_id", "tenant-1")
	c.Set(string(auth.UserContextKey), &auth.UserContext{TenantID: "tenant-1", UserID: "admin"})

	h.ListUserRoles(c)

	assert.Equal(t, http.StatusOK, resp.Code)
	var body response.ListResponse
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	items, ok := body.Items.([]interface{})
	if !ok {
		t.Fatalf("unexpected items type: %T", body.Items)
	}
	assert.Len(t, items, 2)
}
