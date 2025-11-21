package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"backend/internal/infra"
)

var (
	// ErrNotFound is returned when a requested record does not exist in the
	// underlying storage.
	ErrNotFound = errors.New("tenant: not found")
)

// tenantAwareRepo is a small helper embedded by multi-tenant repositories.
// It centralizes access to the DB handle and provides helpers for working
// with TenantContext.
type tenantAwareRepo struct {
	db infra.DB
}

// tenantIDFromContext extracts the tenant identifier from the context. It
// expects TenantContext to have been attached by TenantContextMiddleware.
func tenantIDFromContext(ctx context.Context) (string, error) {
	tc, ok := FromContext(ctx)
	if !ok || tc.TenantID == "" {
		return "", errors.New("tenant: missing tenant context")
	}
	return tc.TenantID, nil
}

// TenantRepository defines operations for managing Tenant records.
type TenantRepository interface {
	Insert(ctx context.Context, t *Tenant) error
	GetByID(ctx context.Context, id string) (*Tenant, error)
	List(ctx context.Context, limit, offset int) ([]*Tenant, int64, error)
	Update(ctx context.Context, t *Tenant) error
	Delete(ctx context.Context, id string) error
}

type tenantRepository struct {
	tenantAwareRepo
}

// NewTenantRepository constructs a TenantRepository backed by the given DB.
func NewTenantRepository(db infra.DB) TenantRepository {
	return &tenantRepository{tenantAwareRepo{db: db}}
}

func (r *tenantRepository) Insert(ctx context.Context, t *Tenant) error {
	const q = `
		INSERT INTO tenants (id, name, slug, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, q, t.ID, t.Name, t.Slug, t.Status, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *tenantRepository) GetByID(ctx context.Context, id string) (*Tenant, error) {
	const q = `
		SELECT id, name, slug, status, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, q, id)
	var t Tenant
	if err := row.Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *tenantRepository) List(ctx context.Context, limit, offset int) ([]*Tenant, int64, error) {
	const countQ = `SELECT COUNT(*) FROM tenants WHERE deleted_at IS NULL`
	var total int64
	if err := r.db.QueryRowContext(ctx, countQ).Scan(&total); err != nil {
		return nil, 0, err
	}

	const listQ = `
		SELECT id, name, slug, status, created_at, updated_at
		FROM tenants
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.QueryContext(ctx, listQ, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tenants []*Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		tenants = append(tenants, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return tenants, total, nil
}

func (r *tenantRepository) Update(ctx context.Context, t *Tenant) error {
	const q = `
		UPDATE tenants
		SET name = $1, slug = $2, status = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, q, t.Name, t.Slug, t.Status, t.UpdatedAt, t.ID)
	return err
}

func (r *tenantRepository) Delete(ctx context.Context, id string) error {
	const q = `
		UPDATE tenants
		SET deleted_at = $1
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, q, time.Now().UTC(), id)
	return err
}

// UserRepository defines operations for managing users within a tenant. All
// methods rely on the tenant_id derived from context for isolation.
type UserRepository interface {
	Insert(ctx context.Context, u *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	ListByTenant(ctx context.Context) ([]*User, error)
	Update(ctx context.Context, u *User) error
}

type userRepository struct {
	tenantAwareRepo
}

// NewUserRepository constructs a UserRepository backed by the given DB.
func NewUserRepository(db infra.DB) UserRepository {
	return &userRepository{tenantAwareRepo{db: db}}
}

func (r *userRepository) Insert(ctx context.Context, u *User) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO users (id, tenant_id, email, username, password_hash, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = r.db.ExecContext(ctx, q, u.ID, tenantID, u.Email, u.Username, u.PasswordHash, u.Status, u.CreatedAt, u.UpdatedAt)
	return err
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, email, username, password_hash, status, created_at, updated_at
		FROM users
		WHERE tenant_id = $1 AND email = $2
	`
	row := r.db.QueryRowContext(ctx, q, tenantID, email)
	var u User
	if err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Username, &u.PasswordHash, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, email, username, password_hash, status, created_at, updated_at
		FROM users
		WHERE tenant_id = $1 AND username = $2
	`
	row := r.db.QueryRowContext(ctx, q, tenantID, username)
	var u User
	if err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Username, &u.PasswordHash, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*User, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, email, username, password_hash, status, created_at, updated_at
		FROM users
		WHERE tenant_id = $1 AND id = $2
	`
	row := r.db.QueryRowContext(ctx, q, tenantID, id)
	var u User
	if err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Username, &u.PasswordHash, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) ListByTenant(ctx context.Context) ([]*User, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, email, username, password_hash, status, created_at, updated_at
		FROM users
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Email, &u.Username, &u.PasswordHash, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *userRepository) Update(ctx context.Context, u *User) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	const q = `
		UPDATE users
		SET email = $1,
			username = $2,
			password_hash = $3,
			status = $4,
			updated_at = $5
		WHERE tenant_id = $6 AND id = $7
	`
	_, err = r.db.ExecContext(ctx, q, u.Email, u.Username, u.PasswordHash, u.Status, u.UpdatedAt, tenantID, u.ID)
	return err
}

// RoleRepository defines operations for managing roles within a tenant.
type RoleRepository interface {
	Insert(ctx context.Context, r *Role) error
	GetByID(ctx context.Context, id string) (*Role, error)
	GetByName(ctx context.Context, name string) (*Role, error)
	Update(ctx context.Context, r *Role) error
	Delete(ctx context.Context, id string) error
}

type roleRepository struct {
	tenantAwareRepo
}

func NewRoleRepository(db infra.DB) RoleRepository {
	return &roleRepository{tenantAwareRepo{db: db}}
}

func (r *roleRepository) Insert(ctx context.Context, role *Role) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO roles (id, tenant_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.db.ExecContext(ctx, q, role.ID, tenantID, role.Name, role.Description, role.CreatedAt, role.UpdatedAt)
	return err
}

func (r *roleRepository) GetByID(ctx context.Context, id string) (*Role, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, name, description, created_at, updated_at
		FROM roles
		WHERE tenant_id = $1 AND id = $2
	`
	row := r.db.QueryRowContext(ctx, q, tenantID, id)
	var role Role
	if err := row.Scan(&role.ID, &role.TenantID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *roleRepository) GetByName(ctx context.Context, name string) (*Role, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, name, description, created_at, updated_at
		FROM roles
		WHERE tenant_id = $1 AND name = $2
	`
	row := r.db.QueryRowContext(ctx, q, tenantID, name)
	var role Role
	if err := row.Scan(&role.ID, &role.TenantID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *roleRepository) Update(ctx context.Context, role *Role) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	const q = `
		UPDATE roles
		SET name = $1,
			description = $2,
			updated_at = $3
		WHERE tenant_id = $4 AND id = $5
	`
	_, err = r.db.ExecContext(ctx, q, role.Name, role.Description, role.UpdatedAt, tenantID, role.ID)
	return err
}

func (r *roleRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	const q = `
		DELETE FROM roles
		WHERE tenant_id = $1 AND id = $2
	`
	_, err = r.db.ExecContext(ctx, q, tenantID, id)
	return err
}

// PermissionRepository provides read operations for permissions.
type PermissionRepository interface {
	ListByTenant(ctx context.Context) ([]*Permission, error)
	GetPermissionsForUser(ctx context.Context, userID string) ([]*Permission, error)
}

type permissionRepository struct {
	tenantAwareRepo
}

func NewPermissionRepository(db infra.DB) PermissionRepository {
	return &permissionRepository{tenantAwareRepo{db: db}}
}

func (r *permissionRepository) ListByTenant(ctx context.Context) ([]*Permission, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, resource, action
		FROM permissions
		WHERE tenant_id IS NULL OR tenant_id = $1
	`
	rows, err := r.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Resource, &p.Action); err != nil {
			return nil, err
		}
		perms = append(perms, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return perms, nil
}

func (r *permissionRepository) GetPermissionsForUser(ctx context.Context, userID string) ([]*Permission, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT DISTINCT p.id, p.tenant_id, p.resource, p.action
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.tenant_id = $1 AND ur.user_id = $2
	`
	rows, err := r.db.QueryContext(ctx, q, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Resource, &p.Action); err != nil {
			return nil, err
		}
		perms = append(perms, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return perms, nil
}

// UserRoleRepository manages user-role assignments.
type UserRoleRepository interface {
	AssignRoleToUser(ctx context.Context, userID, roleID string) error
	RemoveRolesByRole(ctx context.Context, roleID string) error
}

type userRoleRepository struct {
	tenantAwareRepo
}

func NewUserRoleRepository(db infra.DB) UserRoleRepository {
	return &userRoleRepository{tenantAwareRepo{db: db}}
}

func (r *userRoleRepository) AssignRoleToUser(ctx context.Context, userID, roleID string) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO user_roles (id, tenant_id, user_id, role_id)
		VALUES ($1, $2, $3, $4)
	`
	// ID 生成可由上层控制，这里简单使用 userID+roleID 组合场景由上层保证唯一
	_, err = r.db.ExecContext(ctx, q, userID+"-"+roleID, tenantID, userID, roleID)
	return err
}

func (r *userRoleRepository) RemoveRolesByRole(ctx context.Context, roleID string) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	const q = `
		DELETE FROM user_roles
		WHERE tenant_id = $1 AND role_id = $2
	`
	_, err = r.db.ExecContext(ctx, q, tenantID, roleID)
	return err
}

// RolePermissionRepository manages role-permission links.
type RolePermissionRepository interface {
	ReplaceRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error
}

type rolePermissionRepository struct {
	tenantAwareRepo
}

func NewRolePermissionRepository(db infra.DB) RolePermissionRepository {
	return &rolePermissionRepository{tenantAwareRepo{db: db}}
}

func (r *rolePermissionRepository) ReplaceRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	// 删除现有记录
	const delQ = `
		DELETE FROM role_permissions
		WHERE tenant_id = $1 AND role_id = $2
	`
	if _, err := r.db.ExecContext(ctx, delQ, tenantID, roleID); err != nil {
		return err
	}

	// 无新权限则直接返回
	if len(permissionIDs) == 0 {
		return nil
	}

	const insQ = `
		INSERT INTO role_permissions (id, tenant_id, role_id, permission_id)
		VALUES ($1, $2, $3, $4)
	`
	for _, pid := range permissionIDs {
		id := roleID + "-" + pid
		if _, err := r.db.ExecContext(ctx, insQ, id, tenantID, roleID, pid); err != nil {
			return err
		}
	}
	return nil
}

// TenantConfigRepository manages TenantConfig persistence.
type TenantConfigRepository interface {
	GetByTenantID(ctx context.Context) (*TenantConfig, error)
	Upsert(ctx context.Context, cfg *TenantConfig) error
}

type tenantConfigRepository struct {
	tenantAwareRepo
}

func NewTenantConfigRepository(db infra.DB) TenantConfigRepository {
	return &tenantConfigRepository{tenantAwareRepo{db: db}}
}

func (r *tenantConfigRepository) GetByTenantID(ctx context.Context) (*TenantConfig, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT tenant_id, display_name, description, logo_url, language, timezone,
			COALESCE(feature_flags, '{}'::jsonb) AS feature_flags,
			COALESCE(approval_settings, '{}'::jsonb) AS approval_settings
		FROM tenant_configs
		WHERE tenant_id = $1
	`
	row := r.db.QueryRowContext(ctx, q, tenantID)
	var cfg TenantConfig
	var featureFlagsRaw []byte
	var approvalSettingsRaw []byte
	if err := row.Scan(
		&cfg.TenantID,
		&cfg.DisplayName,
		&cfg.Description,
		&cfg.LogoURL,
		&cfg.Language,
		&cfg.TimeZone,
		&featureFlagsRaw,
		&approvalSettingsRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(featureFlagsRaw) > 0 {
		_ = json.Unmarshal(featureFlagsRaw, &cfg.FeatureFlags)
	} else {
		cfg.FeatureFlags = map[string]bool{}
	}
	if len(approvalSettingsRaw) > 0 {
		_ = json.Unmarshal(approvalSettingsRaw, &cfg.ApprovalSettings)
	}
	if cfg.ApprovalSettings == nil {
		cfg.ApprovalSettings = normalizeApprovalSettings(ApprovalSettings{})
	}
	return &cfg, nil
}

func (r *tenantConfigRepository) Upsert(ctx context.Context, cfg *TenantConfig) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO tenant_configs (tenant_id, display_name, description, logo_url, language, timezone, feature_flags, approval_settings)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id) DO UPDATE
		SET display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			logo_url = EXCLUDED.logo_url,
			language = EXCLUDED.language,
			timezone = EXCLUDED.timezone,
			feature_flags = EXCLUDED.feature_flags,
			approval_settings = EXCLUDED.approval_settings
	`
	featureFlagsJSON, _ := json.Marshal(cfg.FeatureFlags)
	approvalSettingsJSON, _ := json.Marshal(cfg.ApprovalSettings)
	_, err = r.db.ExecContext(ctx, q,
		tenantID,
		cfg.DisplayName,
		cfg.Description,
		cfg.LogoURL,
		cfg.Language,
		cfg.TimeZone,
		featureFlagsJSON,
		approvalSettingsJSON,
	)
	return err
}
