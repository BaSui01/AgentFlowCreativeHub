package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// OAuthUnbindService OAuth 账号解绑服务
type OAuthUnbindService struct {
	db          *gorm.DB
	auditLogger UnbindAuditLogger
}

// UnbindAuditLogger 解绑审计日志接口
type UnbindAuditLogger interface {
	Log(ctx context.Context, event UnbindAuditEvent) error
}

// LinkedOAuthIdentity 已关联的 OAuth 身份信息
type LinkedOAuthIdentity struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	UserID         string    `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	Avatar         string    `json:"avatar"`
	CreatedAt      time.Time `json:"created_at"`
}

func (LinkedOAuthIdentity) TableName() string {
	return "oauth_identities"
}

// UnbindAuditEvent 解绑审计事件
type UnbindAuditEvent struct {
	UserID    string
	Action    string
	Resource  string
	Details   map[string]any
	IP        string
	UserAgent string
	Timestamp time.Time
}

var (
	ErrIdentityNotFound     = errors.New("oauth identity not found")
	ErrCannotUnbindLast     = errors.New("cannot unbind last login method")
	ErrUnbindNotAllowed     = errors.New("unbind not allowed")
	ErrConfirmationRequired = errors.New("confirmation required")
)

// NewOAuthUnbindService 创建解绑服务
func NewOAuthUnbindService(db *gorm.DB, logger UnbindAuditLogger) *OAuthUnbindService {
	return &OAuthUnbindService{
		db:          db,
		auditLogger: logger,
	}
}

// ListLinkedProviders 列出已关联的第三方账号
func (s *OAuthUnbindService) ListLinkedProviders(ctx context.Context, userID string) ([]LinkedOAuthIdentity, error) {
	var identities []LinkedOAuthIdentity
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&identities).Error
	return identities, err
}

// CanUnbind 检查是否可以解绑
func (s *OAuthUnbindService) CanUnbind(ctx context.Context, userID, provider string) (*UnbindCheck, error) {
	check := &UnbindCheck{
		Provider:    provider,
		CanUnbind:   false,
		HasPassword: false,
	}

	// 检查是否存在该关联
	identities, err := s.ListLinkedProviders(ctx, userID)
	if err != nil {
		return nil, err
	}

	var foundIdentity *LinkedOAuthIdentity
	for i, id := range identities {
		if id.Provider == provider {
			foundIdentity = &identities[i]
			break
		}
	}

	if foundIdentity == nil {
		return nil, ErrIdentityNotFound
	}
	check.Identity = foundIdentity

	// 检查是否有密码
	hasPassword, err := s.hasPassword(ctx, userID)
	if err != nil {
		return nil, err
	}
	check.HasPassword = hasPassword

	// 检查剩余登录方式
	check.OtherProviders = len(identities) - 1

	// 判断是否可以解绑
	// 规则：至少保留一种登录方式（密码或其他 OAuth）
	if hasPassword || len(identities) > 1 {
		check.CanUnbind = true
	} else {
		check.Reason = "解绑后将没有任何登录方式，请先设置密码或绑定其他账号"
	}

	return check, nil
}

func (s *OAuthUnbindService) hasPassword(ctx context.Context, userID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Table("users").
		Where("id = ? AND password_hash IS NOT NULL AND password_hash != ''", userID).
		Count(&count).Error
	return count > 0, err
}

// UnbindCheck 解绑检查结果
type UnbindCheck struct {
	Provider       string               `json:"provider"`
	CanUnbind      bool                 `json:"can_unbind"`
	HasPassword    bool                 `json:"has_password"`
	OtherProviders int                  `json:"other_providers"`
	Reason         string               `json:"reason,omitempty"`
	Identity       *LinkedOAuthIdentity `json:"identity,omitempty"`
}

// Unbind 解绑第三方账号
func (s *OAuthUnbindService) Unbind(ctx context.Context, req *UnbindRequest) error {
	// 先检查是否可以解绑
	check, err := s.CanUnbind(ctx, req.UserID, req.Provider)
	if err != nil {
		return err
	}

	if !check.CanUnbind {
		return fmt.Errorf("%w: %s", ErrCannotUnbindLast, check.Reason)
	}

	// 需要确认
	if req.RequireConfirm && !req.Confirmed {
		return ErrConfirmationRequired
	}

	// 执行解绑
	err = s.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", req.UserID, req.Provider).
		Delete(&LinkedOAuthIdentity{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete identity: %w", err)
	}

	// 记录审计日志
	if s.auditLogger != nil {
		_ = s.auditLogger.Log(ctx, UnbindAuditEvent{
			UserID:   req.UserID,
			Action:   "oauth.unbind",
			Resource: req.Provider,
			Details: map[string]any{
				"provider":          req.Provider,
				"provider_user_id":  check.Identity.ProviderUserID,
				"provider_email":    check.Identity.Email,
				"remaining_methods": check.OtherProviders,
				"has_password":      check.HasPassword,
			},
			IP:        req.IP,
			UserAgent: req.UserAgent,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// UnbindRequest 解绑请求
type UnbindRequest struct {
	UserID         string `json:"user_id"`
	Provider       string `json:"provider"`
	Confirmed      bool   `json:"confirmed"`
	RequireConfirm bool   `json:"-"`
	IP             string `json:"-"`
	UserAgent      string `json:"-"`
}

// UnbindAll 解绑所有第三方账号（用于账号注销前）
func (s *OAuthUnbindService) UnbindAll(ctx context.Context, userID string) (int, error) {
	result := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&LinkedOAuthIdentity{})
	return int(result.RowsAffected), result.Error
}

// GetProviderDisplayName 获取提供商显示名称
func GetProviderDisplayName(provider string) string {
	names := map[string]string{
		"google":   "Google",
		"github":   "GitHub",
		"wechat":   "微信",
		"dingtalk": "钉钉",
		"weibo":    "微博",
		"qq":       "QQ",
	}

	if name, ok := names[provider]; ok {
		return name
	}
	return provider
}
