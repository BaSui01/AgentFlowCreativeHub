package rag

import (
	"context"
	"testing"
	"time"
)

// MockKBSharingStore 模拟存储
type MockKBSharingStore struct {
	shares      map[string]*KBShare
	acceptances map[string]*ShareAcceptance
	logs        []*ShareAccessLog
}

func NewMockKBSharingStore() *MockKBSharingStore {
	return &MockKBSharingStore{
		shares:      make(map[string]*KBShare),
		acceptances: make(map[string]*ShareAcceptance),
		logs:        make([]*ShareAccessLog, 0),
	}
}

func (m *MockKBSharingStore) CreateShare(ctx context.Context, share *KBShare) error {
	m.shares[share.ID] = share
	return nil
}

func (m *MockKBSharingStore) UpdateShare(ctx context.Context, share *KBShare) error {
	m.shares[share.ID] = share
	return nil
}

func (m *MockKBSharingStore) DeleteShare(ctx context.Context, shareID string) error {
	delete(m.shares, shareID)
	return nil
}

func (m *MockKBSharingStore) GetShare(ctx context.Context, shareID string) (*KBShare, error) {
	return m.shares[shareID], nil
}

func (m *MockKBSharingStore) GetShareByToken(ctx context.Context, token string) (*KBShare, error) {
	for _, share := range m.shares {
		if share.ShareToken == token {
			return share, nil
		}
	}
	return nil, nil
}

func (m *MockKBSharingStore) ListSharesByKB(ctx context.Context, kbID string) ([]*KBShare, error) {
	var result []*KBShare
	for _, share := range m.shares {
		if share.KnowledgeBaseID == kbID {
			result = append(result, share)
		}
	}
	return result, nil
}

func (m *MockKBSharingStore) ListSharesByTenant(ctx context.Context, tenantID string) ([]*KBShare, error) {
	var result []*KBShare
	for _, share := range m.shares {
		if share.OwnerTenantID == tenantID {
			result = append(result, share)
		}
	}
	return result, nil
}

func (m *MockKBSharingStore) ListReceivedShares(ctx context.Context, tenantID string) ([]*KBShare, error) {
	var result []*KBShare
	for _, acceptance := range m.acceptances {
		if acceptance.TenantID == tenantID {
			if share, ok := m.shares[acceptance.ShareID]; ok {
				result = append(result, share)
			}
		}
	}
	return result, nil
}

func (m *MockKBSharingStore) CreateShareAcceptance(ctx context.Context, acceptance *ShareAcceptance) error {
	key := acceptance.ShareID + ":" + acceptance.TenantID
	m.acceptances[key] = acceptance
	return nil
}

func (m *MockKBSharingStore) GetShareAcceptance(ctx context.Context, shareID, tenantID string) (*ShareAcceptance, error) {
	key := shareID + ":" + tenantID
	return m.acceptances[key], nil
}

func (m *MockKBSharingStore) ListAcceptancesByShare(ctx context.Context, shareID string) ([]*ShareAcceptance, error) {
	var result []*ShareAcceptance
	for _, acceptance := range m.acceptances {
		if acceptance.ShareID == shareID {
			result = append(result, acceptance)
		}
	}
	return result, nil
}

func (m *MockKBSharingStore) DeleteShareAcceptance(ctx context.Context, shareID, tenantID string) error {
	key := shareID + ":" + tenantID
	delete(m.acceptances, key)
	return nil
}

func (m *MockKBSharingStore) LogShareAccess(ctx context.Context, log *ShareAccessLog) error {
	m.logs = append(m.logs, log)
	return nil
}

func (m *MockKBSharingStore) GetShareAccessLogs(ctx context.Context, shareID string, limit int) ([]*ShareAccessLog, error) {
	var result []*ShareAccessLog
	for _, log := range m.logs {
		if log.ShareID == shareID {
			result = append(result, log)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func TestCreateShare(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "测试共享",
		Description:     "测试描述",
		CreatedBy:       "user_001",
		ExpiresIn:       24 * time.Hour,
	}

	share, err := service.CreateShare(ctx, req)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}

	if share.ID == "" {
		t.Error("Share ID should not be empty")
	}
	if share.ShareToken == "" {
		t.Error("ShareToken should not be empty for link type")
	}
	if share.Status != ShareStatusActive {
		t.Errorf("Expected status %s, got %s", ShareStatusActive, share.Status)
	}
	if share.ExpiresAt == nil {
		t.Error("ExpiresAt should be set")
	}
}

func TestCreateTenantShare(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeTenant,
		Permission:      SharePermissionWrite,
		TargetTenantID:  "tenant_002",
		Name:            "定向共享",
		CreatedBy:       "user_001",
	}

	share, err := service.CreateShare(ctx, req)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}

	if share.ShareToken != "" {
		t.Error("ShareToken should be empty for tenant type")
	}
	if share.TargetTenantID != "tenant_002" {
		t.Error("TargetTenantID should be set")
	}
}

func TestValidateShareToken(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	// 创建共享
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "测试共享",
		CreatedBy:       "user_001",
	}
	share, _ := service.CreateShare(ctx, req)

	// 验证有效 Token
	validated, err := service.ValidateShareToken(ctx, share.ShareToken)
	if err != nil {
		t.Fatalf("ValidateShareToken failed: %v", err)
	}
	if validated.ID != share.ID {
		t.Error("Validated share ID mismatch")
	}

	// 验证无效 Token
	_, err = service.ValidateShareToken(ctx, "invalid_token")
	if err != ErrShareNotFound {
		t.Errorf("Expected ErrShareNotFound, got %v", err)
	}
}

func TestExpiredShare(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	// 创建共享
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "过期共享",
		CreatedBy:       "user_001",
		ExpiresIn:       1 * time.Second,
	}
	share, _ := service.CreateShare(ctx, req)

	// 等待过期
	time.Sleep(2 * time.Second)

	// 验证应该失败
	_, err := service.ValidateShareToken(ctx, share.ShareToken)
	if err != ErrShareExpired {
		t.Errorf("Expected ErrShareExpired, got %v", err)
	}
}

func TestAcceptShare(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	// 创建共享
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "测试共享",
		CreatedBy:       "user_001",
	}
	share, _ := service.CreateShare(ctx, req)

	// 接受共享
	acceptance, err := service.AcceptShare(ctx, share.ID, "tenant_002", "user_002", "我的知识库")
	if err != nil {
		t.Fatalf("AcceptShare failed: %v", err)
	}

	if acceptance.TenantID != "tenant_002" {
		t.Error("Acceptance TenantID mismatch")
	}
	if acceptance.Alias != "我的知识库" {
		t.Error("Acceptance Alias mismatch")
	}

	// 重复接受应该失败
	_, err = service.AcceptShare(ctx, share.ID, "tenant_002", "user_002", "重复")
	if err != ErrAlreadyAccepted {
		t.Errorf("Expected ErrAlreadyAccepted, got %v", err)
	}
}

func TestTargetTenantRestriction(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	// 创建定向共享
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeTenant,
		Permission:      SharePermissionRead,
		TargetTenantID:  "tenant_002",
		Name:            "定向共享",
		CreatedBy:       "user_001",
	}
	share, _ := service.CreateShare(ctx, req)

	// 目标租户可以接受
	_, err := service.AcceptShare(ctx, share.ID, "tenant_002", "user_002", "别名")
	if err != nil {
		t.Fatalf("Target tenant should accept: %v", err)
	}

	// 其他租户不能接受
	_, err = service.AcceptShare(ctx, share.ID, "tenant_003", "user_003", "别名")
	if err != ErrShareDenied {
		t.Errorf("Expected ErrShareDenied, got %v", err)
	}
}

func TestRevokeShare(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	// 创建共享
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "测试共享",
		CreatedBy:       "user_001",
	}
	share, _ := service.CreateShare(ctx, req)

	// 撤销共享
	err := service.RevokeShare(ctx, share.ID, "user_001")
	if err != nil {
		t.Fatalf("RevokeShare failed: %v", err)
	}

	// 验证应该失败
	_, err = service.ValidateShareToken(ctx, share.ShareToken)
	if err != ErrShareRevoked {
		t.Errorf("Expected ErrShareRevoked, got %v", err)
	}
}

func TestAccessLimit(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	// 创建有访问限制的共享
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "限制共享",
		CreatedBy:       "user_001",
		MaxAccesses:     2,
	}
	share, _ := service.CreateShare(ctx, req)

	// 第一次接受
	_, err := service.AcceptShare(ctx, share.ID, "tenant_002", "user_002", "别名1")
	if err != nil {
		t.Fatalf("First accept failed: %v", err)
	}

	// 第二次接受
	_, err = service.AcceptShare(ctx, share.ID, "tenant_003", "user_003", "别名2")
	if err != nil {
		t.Fatalf("Second accept failed: %v", err)
	}

	// 第三次接受应该失败
	_, err = service.AcceptShare(ctx, share.ID, "tenant_004", "user_004", "别名3")
	if err != ErrShareLimitReached {
		t.Errorf("Expected ErrShareLimitReached, got %v", err)
	}
}

func TestLeaveShare(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	// 创建共享
	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "测试共享",
		CreatedBy:       "user_001",
	}
	share, _ := service.CreateShare(ctx, req)

	// 接受共享
	_, err := service.AcceptShare(ctx, share.ID, "tenant_002", "user_002", "别名")
	if err != nil {
		t.Fatalf("AcceptShare failed: %v", err)
	}

	// 离开共享
	err = service.LeaveShare(ctx, share.ID, "tenant_002")
	if err != nil {
		t.Fatalf("LeaveShare failed: %v", err)
	}

	// 可以重新接受
	_, err = service.AcceptShare(ctx, share.ID, "tenant_002", "user_002", "新别名")
	if err != nil {
		t.Fatalf("Re-accept should work: %v", err)
	}
}

func TestGenerateShareLink(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "测试共享",
		CreatedBy:       "user_001",
	}
	share, _ := service.CreateShare(ctx, req)

	link := service.GenerateShareLink("https://example.com", share)
	expected := "https://example.com/shared/kb/" + share.ShareToken
	if link != expected {
		t.Errorf("Expected %s, got %s", expected, link)
	}
}

func TestRefreshShareToken(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "测试共享",
		CreatedBy:       "user_001",
	}
	share, _ := service.CreateShare(ctx, req)
	oldToken := share.ShareToken

	// 刷新 Token
	newToken, err := service.RefreshShareToken(ctx, share.ID, "user_001")
	if err != nil {
		t.Fatalf("RefreshShareToken failed: %v", err)
	}

	if newToken == oldToken {
		t.Error("New token should be different from old token")
	}

	// 旧 Token 应该失效
	_, err = service.ValidateShareToken(ctx, oldToken)
	if err != ErrShareNotFound {
		t.Error("Old token should be invalid")
	}

	// 新 Token 应该有效
	_, err = service.ValidateShareToken(ctx, newToken)
	if err != nil {
		t.Errorf("New token should be valid: %v", err)
	}
}

func TestShareStats(t *testing.T) {
	store := NewMockKBSharingStore()
	service := NewKBSharingService(store, nil, 5*time.Minute)

	ctx := context.Background()

	req := &CreateShareRequest{
		KnowledgeBaseID: "kb_001",
		OwnerTenantID:   "tenant_001",
		ShareType:       ShareTypeLink,
		Permission:      SharePermissionRead,
		Name:            "测试共享",
		CreatedBy:       "user_001",
	}
	share, _ := service.CreateShare(ctx, req)

	// 接受共享
	service.AcceptShare(ctx, share.ID, "tenant_002", "user_002", "别名1")
	service.AcceptShare(ctx, share.ID, "tenant_003", "user_003", "别名2")

	// 获取统计
	stats, err := service.GetShareStats(ctx, share.ID)
	if err != nil {
		t.Fatalf("GetShareStats failed: %v", err)
	}

	if stats.AcceptedTenants != 2 {
		t.Errorf("Expected 2 accepted tenants, got %d", stats.AcceptedTenants)
	}
	if stats.TotalAccesses != 2 {
		t.Errorf("Expected 2 total accesses, got %d", stats.TotalAccesses)
	}
}
