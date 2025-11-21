package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"backend/internal/auth"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestIdentityStore_FindActiveUserByEmail(t *testing.T) {
	db := setupIdentityTestDB(t)
	seedIdentityData(t, db)
	store := auth.NewIdentityStore(db, "")

	identity, err := store.FindActiveUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("expected user, got error: %v", err)
	}

	if identity.Email != "admin@example.com" {
		t.Fatalf("unexpected email: %s", identity.Email)
	}
	if len(identity.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(identity.Roles))
	}
}

func TestIdentityStore_EnsureOAuthUser_CreatesUser(t *testing.T) {
	db := setupIdentityTestDB(t)
	seedTenantAndRole(t, db)
	store := auth.NewIdentityStore(db, "")

	info := &auth.OAuth2UserInfo{
		ID:      "google-1",
		Email:   "newuser@example.com",
		Name:    "New User",
		Picture: "https://example.com/avatar.png",
	}

	identity, err := store.EnsureOAuthUser(context.Background(), info)
	if err != nil {
		t.Fatalf("EnsureOAuthUser failed: %v", err)
	}

	if identity.Email != info.Email {
		t.Fatalf("expected email %s, got %s", info.Email, identity.Email)
	}
	if len(identity.Roles) == 0 {
		t.Fatalf("expected roles assigned")
	}
}

func setupIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:identity_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("init sqlite failed: %v", err)
	}

	stmts := []string{
		`CREATE TABLE tenants (id TEXT PRIMARY KEY, name TEXT, created_at TIMESTAMP, deleted_at TIMESTAMP)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			tenant_id TEXT,
			email TEXT UNIQUE,
			username TEXT,
			full_name TEXT,
			avatar_url TEXT,
			password_hash TEXT,
			email_verified BOOLEAN,
			status TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			deleted_at TIMESTAMP
		)`,
		`CREATE TABLE roles (
			id TEXT PRIMARY KEY,
			tenant_id TEXT,
			name TEXT,
			priority INT,
			is_default BOOLEAN,
			deleted_at TIMESTAMP,
			created_at TIMESTAMP
		)`,
		`CREATE TABLE user_roles (
			user_id TEXT,
			role_id TEXT
		)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema failed: %v", err)
		}
	}

	return db
}

func seedIdentityData(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)`, "tenant-1", "Tenant 1", time.Now())
	mustExec(t, db, `INSERT INTO users (id, tenant_id, email, username, full_name, password_hash, email_verified, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"user-1", "tenant-1", "admin@example.com", "admin", "Admin", "hash", true, "active", time.Now(), time.Now())
	mustExec(t, db, `INSERT INTO roles (id, tenant_id, name, priority, is_default, created_at) VALUES (?, ?, ?, ?, ?, ?)`, "role-1", "tenant-1", "admin", 10, false, time.Now())
	mustExec(t, db, `INSERT INTO roles (id, tenant_id, name, priority, is_default, created_at) VALUES (?, ?, ?, ?, ?, ?)`, "role-2", "tenant-1", "editor", 5, false, time.Now())
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)`, "user-1", "role-1")
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)`, "user-1", "role-2")
}

func seedTenantAndRole(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)`, "tenant-2", "Tenant 2", time.Now())
	mustExec(t, db, `INSERT INTO roles (id, tenant_id, name, priority, is_default, created_at) VALUES (?, ?, ?, ?, ?, ?)`, "role-default", "tenant-2", "user", 1, true, time.Now())
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec failed: %v", err)
	}
}
