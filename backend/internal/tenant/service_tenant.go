package tenant

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrForbidden is returned when the current caller is not allowed to
	// perform the requested tenant-level operation.
	ErrForbidden        = errors.New("tenant: forbidden")
	ErrTenantNameExists = errors.New("tenant: name already exists")
)

// IDGenerator abstracts ID generation so that the service does not depend on
// a specific UUID implementation.
type IDGenerator interface {
	NewID() (string, error)
}

// EmailSender sends verification or notification emails related to tenant
// lifecycle events.
type EmailSender interface {
	SendTenantVerification(ctx context.Context, toEmail, verificationToken string) error
}

// TenantQuotaRepository 已移至 repository.go,提供完整的配额仓储接口


// AuditLogger records high-level audit events for tenant operations.
type AuditLogger interface {
	LogAction(ctx context.Context, tc TenantContext, action, resource string, details any)
}

// TenantService defines the behavior for managing tenants, including admin
// created tenants and self-registration flows.
type TenantService interface {
	CreateTenant(ctx context.Context, params CreateTenantParams) (*Tenant, error)
	ListTenants(ctx context.Context, limit, offset int) ([]*Tenant, int64, error)
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	UpdateTenant(ctx context.Context, id string, params UpdateTenantParams) (*Tenant, error)
	DeleteTenant(ctx context.Context, id string) error
	SelfRegisterTenant(ctx context.Context, params SelfRegisterTenantParams) (*Tenant, *User, error)
}

// CreateTenantParams describes inputs required for an administrator-created tenant.
type CreateTenantParams struct {
	Name              string
	Slug              string
	AdminEmail        string
	AdminUsername     string
	AdminPasswordHash string
}

// UpdateTenantParams describes inputs for updating a tenant.
type UpdateTenantParams struct {
	Name   *string
	Slug   *string
	Status *string
}

// SelfRegisterTenantParams describes inputs required for a self-registered tenant.
type SelfRegisterTenantParams struct {
	Name              string
	Slug              string
	AdminEmail        string
	AdminUsername     string
	AdminPasswordHash string
}

type tenantService struct {
	tenants TenantRepository
	users   UserRepository
	quotas  TenantQuotaRepository
	ids     IDGenerator
	mailer  EmailSender
	audit   AuditLogger
}

// NewTenantService constructs a TenantService with its dependencies.
func NewTenantService(
	tenants TenantRepository,
	users UserRepository,
	quotas TenantQuotaRepository,
	ids IDGenerator,
	mailer EmailSender,
	audit AuditLogger,
) TenantService {
	return &tenantService{
		tenants: tenants,
		users:   users,
		quotas:  quotas,
		ids:     ids,
		mailer:  mailer,
		audit:   audit,
	}
}

// CreateTenant is used by system administrators to create a new tenant and its
// default administrator user.
func (s *tenantService) CreateTenant(ctx context.Context, params CreateTenantParams) (*Tenant, error) {
	tc, ok := FromContext(ctx)
	if !ok || !tc.IsSystemAdmin {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(params.Name) == "" || strings.TrimSpace(params.AdminEmail) == "" {
		return nil, errors.New("tenant: missing required fields")
	}

	// Note: name uniqueness is ultimately enforced by the database unique
	// constraint; additional pre-checks can be added here when a GetByName
	// repository method is available.

	tenantID, err := s.ids.NewID()
	if err != nil {
		return nil, err
	}

	slug := params.Slug
	if slug == "" {
		slug = slugify(params.Name)
	}
	now := time.Now().UTC()

	t := &Tenant{
		ID:        tenantID,
		Name:      params.Name,
		Slug:      slug,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.tenants.Insert(ctx, t); err != nil {
		return nil, err
	}

	// Initialize default quota for the new tenant.
	if s.quotas != nil {
		quotaCtx := WithTenantContext(ctx, TenantContext{TenantID: tenantID, UserID: tc.UserID, IsSystemAdmin: tc.IsSystemAdmin})
		q := &TenantQuota{
			TenantID:      tenantID,
			MaxUsers:      100,
			MaxStorageMB:  10240,
			MaxWorkflows:  100,
			UsedUsers:     0,
			UsedStorageMB: 0,
			UsedWorkflows: 0,
		}
		_ = s.quotas.Insert(quotaCtx, q)
	}

	// Create default administrator user under the new tenant.
	adminID, err := s.ids.NewID()
	if err != nil {
		return nil, err
	}
	userCtx := WithTenantContext(ctx, TenantContext{TenantID: tenantID, UserID: adminID, IsSystemAdmin: true})
	admin := &User{
		ID:           adminID,
		TenantID:     tenantID,
		Email:        params.AdminEmail,
		Username:     params.AdminUsername,
		PasswordHash: params.AdminPasswordHash,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Insert(userCtx, admin); err != nil {
		return nil, err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "tenant.create", "tenant", map[string]any{
			"tenantId":     tenantID,
			"tenantName":   params.Name,
			"adminUserId":  adminID,
			"adminEmail":   params.AdminEmail,
			"createdByUser": tc.UserID,
		})
	}

	return t, nil
}

func (s *tenantService) ListTenants(ctx context.Context, limit, offset int) ([]*Tenant, int64, error) {
	tc, ok := FromContext(ctx)
	if !ok || !tc.IsSystemAdmin {
		return nil, 0, ErrForbidden
	}
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	return s.tenants.List(ctx, limit, offset)
}

func (s *tenantService) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	tc, ok := FromContext(ctx)
	// Only SystemAdmin or the tenant admin themselves can view details (if needed).
	// For now, we restrict generic "GetTenant" to SystemAdmin or if the ID matches the context tenant ID.
	if !ok {
		return nil, ErrForbidden
	}
	if !tc.IsSystemAdmin && tc.TenantID != id {
		return nil, ErrForbidden
	}

	return s.tenants.GetByID(ctx, id)
}

func (s *tenantService) UpdateTenant(ctx context.Context, id string, params UpdateTenantParams) (*Tenant, error) {
	tc, ok := FromContext(ctx)
	if !ok || !tc.IsSystemAdmin {
		return nil, ErrForbidden
	}

	t, err := s.tenants.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		t.Name = strings.TrimSpace(*params.Name)
	}
	if params.Slug != nil {
		slug := strings.TrimSpace(*params.Slug)
		if slug == "" && params.Name != nil {
			slug = slugify(*params.Name)
		}
		if slug != "" {
			t.Slug = slug
		}
	}
	if params.Status != nil {
		t.Status = *params.Status
	}
	t.UpdatedAt = time.Now().UTC()

	if err := s.tenants.Update(ctx, t); err != nil {
		return nil, err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "tenant.update", "tenant", map[string]any{
			"tenantId":   t.ID,
			"tenantName": t.Name,
			"changes":    params,
		})
	}

	return t, nil
}

func (s *tenantService) DeleteTenant(ctx context.Context, id string) error {
	tc, ok := FromContext(ctx)
	if !ok || !tc.IsSystemAdmin {
		return ErrForbidden
	}

	if err := s.tenants.Delete(ctx, id); err != nil {
		return err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "tenant.delete", "tenant", map[string]any{
			"tenantId": id,
		})
	}
	return nil
}

// SelfRegisterTenant handles the self-service registration flow for a new
// tenant. It creates a tenant with pending verification status, a default
// administrator user with pending activation, initializes quota, and sends
// a verification email.
func (s *tenantService) SelfRegisterTenant(ctx context.Context, params SelfRegisterTenantParams) (*Tenant, *User, error) {
	if strings.TrimSpace(params.Name) == "" || strings.TrimSpace(params.AdminEmail) == "" {
		return nil, nil, errors.New("tenant: missing required fields")
	}

	tenantID, err := s.ids.NewID()
	if err != nil {
		return nil, nil, err
	}
	slug := params.Slug
	if slug == "" {
		slug = slugify(params.Name)
	}
	now := time.Now().UTC()

	t := &Tenant{
		ID:        tenantID,
		Name:      params.Name,
		Slug:      slug,
		Status:    "pending_verification",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.tenants.Insert(ctx, t); err != nil {
		return nil, nil, err
	}

	// Initialize default quota for the new tenant.
	if s.quotas != nil {
		quotaCtx := WithTenantContext(ctx, TenantContext{TenantID: tenantID})
		q := &TenantQuota{
			TenantID:      tenantID,
			MaxUsers:      100,
			MaxStorageMB:  10240,
			MaxWorkflows:  100,
			UsedUsers:     0,
			UsedStorageMB: 0,
			UsedWorkflows: 0,
		}
		_ = s.quotas.Insert(quotaCtx, q)
	}

	adminID, err := s.ids.NewID()
	if err != nil {
		return nil, nil, err
	}
	userCtx := WithTenantContext(ctx, TenantContext{TenantID: tenantID, UserID: adminID, IsSystemAdmin: true})
	admin := &User{
		ID:           adminID,
		TenantID:     tenantID,
		Email:        params.AdminEmail,
		Username:     params.AdminUsername,
		PasswordHash: params.AdminPasswordHash,
		Status:       "pending_activation",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Insert(userCtx, admin); err != nil {
		return nil, nil, err
	}

	if s.mailer != nil {
		verificationToken, err := s.ids.NewID()
		if err == nil {
			_ = s.mailer.SendTenantVerification(ctx, params.AdminEmail, verificationToken)
		}
	}

	if s.audit != nil {
		// For self-registration, we construct a minimal TenantContext for auditing.
		selfCtx := TenantContext{TenantID: tenantID, UserID: adminID}
		s.audit.LogAction(ctx, selfCtx, "tenant.self_register", "tenant", map[string]any{
			"tenantId":   tenantID,
			"tenantName": params.Name,
			"adminUserId": adminID,
			"adminEmail": params.AdminEmail,
		})
	}

	return t, admin, nil
}

// slugify creates a simple URL-friendly slug from a tenant name. It lowercases
// ASCII letters and replaces spaces with hyphens; for non-ASCII characters it
// falls back to returning the original name.
func slugify(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	// Basic implementation: replace spaces with '-' and lowercase.
	replaced := strings.ReplaceAll(trimmed, " ", "-")
	return strings.ToLower(replaced)
}
