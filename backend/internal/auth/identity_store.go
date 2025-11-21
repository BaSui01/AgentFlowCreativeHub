package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrUserNotFound 表示用户不存在或处于非激活状态
	ErrUserNotFound = errors.New("auth: user not found")
	// ErrTenantUnavailable 表示无法确定默认租户
	ErrTenantUnavailable = errors.New("auth: tenant unavailable")
)

// Identity 用户身份聚合视图
type Identity struct {
	ID            string
	TenantID      string
	Email         string
	Name          string
	PasswordHash  string
	Roles         []string
	EmailVerified bool
	Status        string
}

// IdentityStore 用户身份存储
type IdentityStore struct {
	db              *gorm.DB
	defaultTenantID string
}

// NewIdentityStore 创建身份存储
func NewIdentityStore(db *gorm.DB, defaultTenantID string) *IdentityStore {
	return &IdentityStore{db: db, defaultTenantID: defaultTenantID}
}

// FindActiveUserByEmail 查询激活用户
func (s *IdentityStore) FindActiveUserByEmail(ctx context.Context, email string) (*Identity, error) {
	cleanEmail := strings.TrimSpace(strings.ToLower(email))
	if cleanEmail == "" {
		return nil, ErrUserNotFound
	}

	var user struct {
		ID            string
		TenantID      string
		Email         string
		Name          string
		PasswordHash  string
		EmailVerified bool
		Status        string
	}

	query := s.db.WithContext(ctx).Table("users").
		Where("LOWER(email) = ? AND status = ? AND deleted_at IS NULL", cleanEmail, "active").
		Select("id, tenant_id, email, COALESCE(full_name, username, email) AS name, password_hash, email_verified, status")

	if err := query.First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	roles, err := s.loadUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &Identity{
		ID:            user.ID,
		TenantID:      user.TenantID,
		Email:         user.Email,
		Name:          user.Name,
		PasswordHash:  user.PasswordHash,
		Roles:         roles,
		EmailVerified: user.EmailVerified,
		Status:        user.Status,
	}, nil
}

// EnsureOAuthUser 确保 OAuth2 用户存在（幂等）
func (s *IdentityStore) EnsureOAuthUser(ctx context.Context, info *OAuth2UserInfo) (*Identity, error) {
	if info == nil || strings.TrimSpace(info.Email) == "" {
		return nil, fmt.Errorf("invalid oauth2 profile")
	}

	if identity, err := s.FindActiveUserByEmail(ctx, info.Email); err == nil {
		return identity, nil
	} else if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	tenantID, err := s.resolveTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Begin()
	if err := tx.Error; err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	userID := uuid.New().String()
	now := time.Now().UTC()
	username := s.deriveUsername(info.Email)
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(uuid.NewString()), bcrypt.DefaultCost)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	userPayload := map[string]any{
		"id":             userID,
		"tenant_id":      tenantID,
		"email":          strings.ToLower(info.Email),
		"username":       username,
		"full_name":      info.Name,
		"avatar_url":     info.Picture,
		"password_hash":  string(hashBytes),
		"email_verified": true,
		"status":         "active",
		"created_at":     now,
		"updated_at":     now,
	}

	if err := tx.Table("users").Create(userPayload).Error; err != nil {
		tx.Rollback()
		// 若并发创建导致冲突，回退到查询
		if strings.Contains(err.Error(), "duplicate") {
			return s.FindActiveUserByEmail(ctx, info.Email)
		}
		return nil, err
	}

	if err := s.assignDefaultRole(ctx, tx, userID, tenantID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return s.FindActiveUserByEmail(ctx, info.Email)
}

func (s *IdentityStore) loadUserRoles(ctx context.Context, userID string) ([]string, error) {
	var roles []string
	err := s.db.WithContext(ctx).
		Table("user_roles").
		Select("roles.name").
		Joins("JOIN roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ? AND roles.deleted_at IS NULL", userID).
		Order("roles.priority DESC").
		Pluck("roles.name", &roles).Error
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return []string{"user"}, nil
	}
	return roles, nil
}

func (s *IdentityStore) assignDefaultRole(ctx context.Context, tx *gorm.DB, userID, tenantID string) error {
	var role struct {
		ID string
	}

	roleQuery := tx.WithContext(ctx).
		Table("roles").
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("is_default DESC, priority DESC, created_at ASC").
		Select("id").
		Limit(1)

	if err := roleQuery.First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	return tx.WithContext(ctx).Table("user_roles").Create(map[string]any{
		"user_id": userID,
		"role_id": role.ID,
	}).Error
}

func (s *IdentityStore) resolveTenantID(ctx context.Context) (string, error) {
	if s.defaultTenantID != "" {
		return s.defaultTenantID, nil
	}

	var tenant struct {
		ID string
	}

	if err := s.db.WithContext(ctx).
		Table("tenants").
		Select("id").
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		Limit(1).
		First(&tenant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrTenantUnavailable
		}
		return "", err
	}

	return tenant.ID, nil
}

func (s *IdentityStore) deriveUsername(email string) string {
	clean := strings.ToLower(strings.TrimSpace(email))
	parts := strings.Split(clean, "@")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return clean
}
