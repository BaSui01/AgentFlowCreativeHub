package tenant

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrEmailExists    = errors.New("tenant: email already exists in tenant")
	ErrUsernameExists = errors.New("tenant: username already exists in tenant")
)

// PasswordHasher abstracts password hashing so the service does not depend on
// a specific implementation or library.
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// UserService defines user lifecycle operations within a tenant.
type UserService interface {
	CreateUser(ctx context.Context, params CreateUserParams) (*User, error)
	UpdateUser(ctx context.Context, id string, params UpdateUserParams) (*User, error)
	DisableUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context) ([]*User, error)
}

// CreateUserParams holds input fields for creating a tenant user.
type CreateUserParams struct {
	Email    string
	Username string
	Password string
}

// UpdateUserParams holds input fields for updating a tenant user.
type UpdateUserParams struct {
	Email       string
	Username    string
	NewPassword *string
}

type userService struct {
	repo   UserRepository
	hasher PasswordHasher
	audit  AuditLogger
}

// NewUserService constructs a UserService.
func NewUserService(repo UserRepository, hasher PasswordHasher, audit AuditLogger) UserService {
	return &userService{repo: repo, hasher: hasher, audit: audit}
}

func (s *userService) CreateUser(ctx context.Context, params CreateUserParams) (*User, error) {
	tc, ok := FromContext(ctx)
	if !ok {
		return nil, ErrForbidden
	}
	// 允许未来通过 role 检查更细粒度权限，这里仅示意需要具备管理权限。
	if !tc.IsSystemAdmin {
		// 在真正实现中可以改为检查具体租户管理员角色。
		return nil, ErrForbidden
	}

	if strings.TrimSpace(params.Email) == "" || strings.TrimSpace(params.Username) == "" || strings.TrimSpace(params.Password) == "" {
		return nil, errors.New("tenant: missing required user fields")
	}

	// 唯一性检查 - 邮箱
	if existing, err := s.repo.GetByEmail(ctx, params.Email); err == nil && existing != nil {
		return nil, ErrEmailExists
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	// 唯一性检查 - 用户名
	if existing, err := s.repo.GetByUsername(ctx, params.Username); err == nil && existing != nil {
		return nil, ErrUsernameExists
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	if s.hasher == nil {
		return nil, errors.New("tenant: password hasher is not configured")
	}
	hash, err := s.hasher.Hash(params.Password)
	if err != nil {
		return nil, err
	}

	userID := tc.UserID
	if userID == "" {
		// 当调用者是租户管理员时，UserID 应由上游认证设置；这里不强制生成新 ID，避免与认证系统冲突。
		userID = ""
	}

	now := time.Now().UTC()
	u := &User{
		ID:           userID,
		Email:        params.Email,
		Username:     params.Username,
		PasswordHash: hash,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Insert(ctx, u); err != nil {
		return nil, err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "user.create", "user", map[string]any{
			"email":    u.Email,
			"username": u.Username,
		})
	}

	return u, nil
}

func (s *userService) UpdateUser(ctx context.Context, id string, params UpdateUserParams) (*User, error) {
	tc, ok := FromContext(ctx)
	if !ok {
		return nil, ErrForbidden
	}

	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Email != "" && params.Email != u.Email {
		// 检查新邮箱唯一性
		if existing, err := s.repo.GetByEmail(ctx, params.Email); err == nil && existing != nil && existing.ID != u.ID {
			return nil, ErrEmailExists
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		u.Email = params.Email
	}

	if params.Username != "" && params.Username != u.Username {
		// 检查新用户名唯一性
		if existing, err := s.repo.GetByUsername(ctx, params.Username); err == nil && existing != nil && existing.ID != u.ID {
			return nil, ErrUsernameExists
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		u.Username = params.Username
	}

	if params.NewPassword != nil && s.hasher != nil {
		hash, err := s.hasher.Hash(*params.NewPassword)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = hash
	}

	u.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "user.update", "user", map[string]any{
			"userId":   u.ID,
			"email":    u.Email,
			"username": u.Username,
		})
	}

	return u, nil
}

func (s *userService) DisableUser(ctx context.Context, id string) error {
	tc, ok := FromContext(ctx)
	if !ok {
		return ErrForbidden
	}

	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	u.Status = "disabled"
	u.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, u); err != nil {
		return err
	}

	if s.audit != nil {
		s.audit.LogAction(ctx, tc, "user.disable", "user", map[string]any{
			"userId": id,
		})
	}

	return nil
}

func (s *userService) ListUsers(ctx context.Context) ([]*User, error) {
	if _, ok := FromContext(ctx); !ok {
		return nil, ErrForbidden
	}
	return s.repo.ListByTenant(ctx)
}
