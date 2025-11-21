package tenant

import (
	"context"
	"errors"
	"testing"
)

type fakeTenantRepository struct {
	items map[string]*Tenant
}

func newFakeTenantRepository() *fakeTenantRepository {
	return &fakeTenantRepository{items: make(map[string]*Tenant)}
}

func (r *fakeTenantRepository) Insert(_ context.Context, t *Tenant) error {
	if _, exists := r.items[t.ID]; exists {
		return errors.New("duplicate tenant")
	}
	r.items[t.ID] = t
	return nil
}

func (r *fakeTenantRepository) GetByID(_ context.Context, id string) (*Tenant, error) {
	if t, ok := r.items[id]; ok {
		return t, nil
	}
	return nil, ErrNotFound
}

func (r *fakeTenantRepository) List(_ context.Context, limit, offset int) ([]*Tenant, int64, error) {
	result := make([]*Tenant, 0, len(r.items))
	for _, t := range r.items {
		result = append(result, t)
	}
	return result, int64(len(result)), nil
}

func (r *fakeTenantRepository) Update(_ context.Context, t *Tenant) error {
	if _, ok := r.items[t.ID]; !ok {
		return ErrNotFound
	}
	r.items[t.ID] = t
	return nil
}

func (r *fakeTenantRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}

type fakeUserRepository struct {
	items map[string]*User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{items: make(map[string]*User)}
}

func (r *fakeUserRepository) Insert(_ context.Context, u *User) error {
	r.items[u.ID] = u
	return nil
}

func (r *fakeUserRepository) GetByEmail(_ context.Context, email string) (*User, error) {
	for _, u := range r.items {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, ErrNotFound
}

func (r *fakeUserRepository) GetByUsername(_ context.Context, username string) (*User, error) {
	for _, u := range r.items {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, ErrNotFound
}

func (r *fakeUserRepository) GetByID(_ context.Context, id string) (*User, error) {
	if u, ok := r.items[id]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

func (r *fakeUserRepository) ListByTenant(_ context.Context) ([]*User, error) {
	result := make([]*User, 0, len(r.items))
	for _, u := range r.items {
		result = append(result, u)
	}
	return result, nil
}

func (r *fakeUserRepository) Update(_ context.Context, u *User) error {
	r.items[u.ID] = u
	return nil
}

type fakeQuotaRepository struct {
	items map[string]*TenantQuota
}

func newFakeQuotaRepository() *fakeQuotaRepository {
	return &fakeQuotaRepository{items: make(map[string]*TenantQuota)}
}

func (r *fakeQuotaRepository) Insert(_ context.Context, q *TenantQuota) error {
	r.items[q.TenantID] = q
	return nil
}

type sequenceIDGenerator struct {
	values []string
	idx    int
}

func (g *sequenceIDGenerator) NewID() (string, error) {
	if g.idx >= len(g.values) {
		return "", errors.New("no ids")
	}
	id := g.values[g.idx]
	g.idx++
	return id, nil
}

type noopEmailSender struct{}

func (noopEmailSender) SendTenantVerification(context.Context, string, string) error {
	return nil
}

type fakeAuditLogger struct {
	actions []string
}

func (l *fakeAuditLogger) LogAction(_ context.Context, _ TenantContext, action, resource string, _ any) {
	l.actions = append(l.actions, action+":"+resource)
}

func newTenantServiceForTest(ids []string) (TenantService, *fakeTenantRepository, *fakeUserRepository, *fakeQuotaRepository, *fakeAuditLogger) {
	tRepo := newFakeTenantRepository()
	uRepo := newFakeUserRepository()
	qRepo := newFakeQuotaRepository()
	audit := &fakeAuditLogger{}
	gen := &sequenceIDGenerator{values: ids}
	service := NewTenantService(tRepo, uRepo, qRepo, gen, noopEmailSender{}, audit)
	return service, tRepo, uRepo, qRepo, audit
}

func TestTenantServiceCreateTenantRequiresAdmin(t *testing.T) {
	service, _, _, _, _ := newTenantServiceForTest([]string{"tenant-1", "user-1"})
	ctx := context.Background()
	_, err := service.CreateTenant(ctx, CreateTenantParams{Name: "Demo", Slug: "demo", AdminEmail: "a@b.com", AdminUsername: "admin", AdminPasswordHash: "hash"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("预期返回 ErrForbidden, 实际: %v", err)
	}
}

func TestTenantServiceCreateTenantPersistsTenantUserAndQuota(t *testing.T) {
	service, tRepo, uRepo, qRepo, audit := newTenantServiceForTest([]string{"tenant-1", "user-1"})
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "sys", UserID: "root", IsSystemAdmin: true})
	params := CreateTenantParams{
		Name:              "Acme",
		Slug:              "",
		AdminEmail:        "admin@acme.io",
		AdminUsername:     "acme-admin",
		AdminPasswordHash: "hashed-secret",
	}

	created, err := service.CreateTenant(ctx, params)
	if err != nil {
		t.Fatalf("CreateTenant 失败: %v", err)
	}
	if created.ID != "tenant-1" {
		t.Fatalf("tenant id 不符: %s", created.ID)
	}
	if created.Slug != "acme" {
		t.Fatalf("slug 未自动生成, got %s", created.Slug)
	}
	if _, ok := tRepo.items[created.ID]; !ok {
		t.Fatalf("tenant 未写入仓储")
	}
	admin, ok := uRepo.items["user-1"]
	if !ok {
		t.Fatalf("管理员用户未创建")
	}
	if admin.Email != "admin@acme.io" || admin.PasswordHash != "hashed-secret" {
		t.Fatalf("管理员字段不匹配: %#v", admin)
	}
	if _, ok := qRepo.items[created.ID]; !ok {
		t.Fatalf("配额未初始化")
	}
	if len(audit.actions) == 0 || audit.actions[0] != "tenant.create:tenant" {
		t.Fatalf("审计日志未记录")
	}
}

func TestTenantServiceCreateTenantRejectsMissingFields(t *testing.T) {
	service, _, _, _, _ := newTenantServiceForTest([]string{"tenant-1", "user-1"})
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "sys", UserID: "root", IsSystemAdmin: true})
	_, err := service.CreateTenant(ctx, CreateTenantParams{Name: "", AdminEmail: ""})
	if err == nil {
		t.Fatalf("缺少字段时应返回错误")
	}
}
