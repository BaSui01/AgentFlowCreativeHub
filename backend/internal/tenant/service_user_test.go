package tenant

import (
	"context"
	"errors"
	"testing"
)

type fakePasswordHasher struct {
	err error
}

func (h *fakePasswordHasher) Hash(password string) (string, error) {
	if h.err != nil {
		return "", h.err
	}
	return "hash:" + password, nil
}

func TestUserServiceCreateUserRequiresContext(t *testing.T) {
	repo := newFakeUserRepository()
	hasher := &fakePasswordHasher{}
	audit := &fakeAuditLogger{}
	service := NewUserService(repo, hasher, audit)
	_, err := service.CreateUser(context.Background(), CreateUserParams{Email: "a@b.com", Username: "a", Password: "pwd"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("未提供租户上下文时应返回 ErrForbidden, got %v", err)
	}
}

func TestUserServiceCreateUserRejectsDuplicateEmail(t *testing.T) {
	repo := newFakeUserRepository()
	repo.items["u-existing"] = &User{ID: "u-existing", Email: "dup@acme.io", Username: "dup"}
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "tenant-1", UserID: "admin", IsSystemAdmin: true})
	hasher := &fakePasswordHasher{}
	audit := &fakeAuditLogger{}
	service := NewUserService(repo, hasher, audit)
	_, err := service.CreateUser(ctx, CreateUserParams{Email: "dup@acme.io", Username: "user2", Password: "pwd"})
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("重复邮箱应返回 ErrEmailExists, got %v", err)
	}
}

func TestUserServiceCreateUserSuccess(t *testing.T) {
	repo := newFakeUserRepository()
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "tenant-1", UserID: "admin", IsSystemAdmin: true})
	hasher := &fakePasswordHasher{}
	audit := &fakeAuditLogger{}
	service := NewUserService(repo, hasher, audit)
	params := CreateUserParams{Email: "new@acme.io", Username: "new", Password: "secret"}
	user, err := service.CreateUser(ctx, params)
	if err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}
	if user.PasswordHash != "hash:secret" {
		t.Fatalf("密码未被哈希: %s", user.PasswordHash)
	}
	if len(audit.actions) == 0 || audit.actions[0] != "user.create:user" {
		t.Fatalf("审计日志未记录 user.create")
	}
}

func TestUserServiceUpdateUserChangesEmailAndPassword(t *testing.T) {
	repo := newFakeUserRepository()
	repo.items["user-1"] = &User{ID: "user-1", Email: "old@acme.io", Username: "old", PasswordHash: "hash:old"}
	ctx := WithTenantContext(context.Background(), TenantContext{TenantID: "tenant-1", UserID: "admin", IsSystemAdmin: true})
	hasher := &fakePasswordHasher{}
	audit := &fakeAuditLogger{}
	service := NewUserService(repo, hasher, audit)
	newPassword := "new-secret"
	updated, err := service.UpdateUser(ctx, "user-1", UpdateUserParams{Email: "new@acme.io", Username: "new", NewPassword: &newPassword})
	if err != nil {
		t.Fatalf("UpdateUser 失败: %v", err)
	}
	if updated.Email != "new@acme.io" || updated.Username != "new" {
		t.Fatalf("用户字段未更新: %#v", updated)
	}
	if updated.PasswordHash != "hash:"+newPassword {
		t.Fatalf("密码未更新: %s", updated.PasswordHash)
	}
	if len(audit.actions) == 0 || audit.actions[0] != "user.update:user" {
		t.Fatalf("应记录 user.update 审计")
	}
}
