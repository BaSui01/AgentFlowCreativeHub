package approval

import (
	"context"
	"testing"

	"backend/internal/logger"
	"backend/internal/tenant"
	workflowpkg "backend/internal/workflow"

	sqlite "github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func init() {
	_ = logger.Init("debug", "console", "stdout")
}

type stubConfigService struct {
	config *tenant.TenantConfig
}

func (s *stubConfigService) GetConfig(ctx context.Context) (*tenant.TenantConfig, error) {
	return s.config, nil
}

func (s *stubConfigService) UpdateConfig(ctx context.Context, params tenant.UpdateTenantConfigParams) (*tenant.TenantConfig, error) {
	return s.config, nil
}

func TestConfigTargetResolver(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:resolver_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&tenant.User{}, &tenant.Role{}, &tenant.UserRole{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	roleID := uuid.New().String()
	role := tenant.Role{ID: roleID, TenantID: tenantID, Name: "Approver", Code: "approver"}
	user := tenant.User{ID: userID, TenantID: tenantID, Email: "owner@example.com", Username: "owner", PasswordHash: "x"}
	roleUser := tenant.UserRole{ID: uuid.New().String(), TenantID: tenantID, UserID: userID, RoleID: roleID}
	_ = db.Create(&role).Error
	_ = db.Create(&user).Error
	_ = db.Create(&roleUser).Error

	configSvc := &stubConfigService{config: &tenant.TenantConfig{
		TenantID: tenantID,
		ApprovalSettings: &tenant.ApprovalSettings{
			DefaultChannels: []string{"websocket", "email"},
			NotificationTargets: map[string][]string{
				"email": []string{"role:approver"},
			},
		},
	}}
	resolver := NewConfigTargetResolver(db, configSvc)
	approvalReq := &workflowpkg.ApprovalRequest{
		TenantID:       tenantID,
		RequestedBy:    userID,
		NotifyChannels: []string{"email", "websocket"},
	}
	result, err := resolver.Resolve(context.Background(), approvalReq)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if len(result.Channels["email"]) != 1 || result.Channels["email"][0] != "owner@example.com" {
		t.Fatalf("expected role email fallback, got %v", result.Channels["email"])
	}
	if len(result.Channels["websocket"]) != 1 || result.Channels["websocket"][0] != userID {
		t.Fatalf("expected websocket fallback to requester")
	}
}
