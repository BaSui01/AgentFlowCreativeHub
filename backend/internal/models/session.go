package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Session 用户会话
type Session struct {
	ID           string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       string    `gorm:"type:uuid;not null;index:idx_session_user" json:"user_id"`
	TenantID     string    `gorm:"type:uuid;not null;index:idx_session_tenant" json:"tenant_id"`
	RefreshToken string    `gorm:"type:text;not null;uniqueIndex" json:"-"`   // 不返回给客户端
	Provider     string    `gorm:"type:varchar(50);not null" json:"provider"` // local, google, github, etc.
	IPAddress    string    `gorm:"type:varchar(100)" json:"ip_address"`
	UserAgent    string    `gorm:"type:text" json:"user_agent"`
	ExpiresAt    time.Time `gorm:"not null;index:idx_session_expires" json:"expires_at"`
	LastUsedAt   time.Time `gorm:"not null" json:"last_used_at"`
	IsRevoked    bool      `gorm:"default:false;index:idx_session_revoked" json:"is_revoked"`
	CreatedAt    time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time `gorm:"not null" json:"updated_at"`
}

// BeforeCreate GORM 钩子：创建前设置 ID
func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now()
	}
	if s.LastUsedAt.IsZero() {
		s.LastUsedAt = time.Now()
	}
	return nil
}

// BeforeUpdate GORM 钩子：更新前设置时间
func (s *Session) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = time.Now()
	return nil
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}

// IsExpired 检查会话是否过期
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid 检查会话是否有效
func (s *Session) IsValid() bool {
	return !s.IsRevoked && !s.IsExpired()
}

// SessionService 会话管理服务
type SessionService struct {
	db *gorm.DB
}

// NewSessionService 创建会话服务
func NewSessionService(db *gorm.DB) *SessionService {
	return &SessionService{db: db}
}

// CreateSession 创建新会话
func (s *SessionService) CreateSession(ctx context.Context, session *Session) error {
	return s.db.WithContext(ctx).Create(session).Error
}

// GetSessionByRefreshToken 通过刷新令牌获取会话
func (s *SessionService) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
	var session Session
	err := s.db.WithContext(ctx).
		Where("refresh_token = ? AND is_revoked = false", refreshToken).
		First(&session).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("会话不存在")
		}
		return nil, err
	}

	// 检查是否过期
	if session.IsExpired() {
		return nil, fmt.Errorf("会话已过期")
	}

	return &session, nil
}

// GetSessionByID 通过 ID 获取会话
func (s *SessionService) GetSessionByID(ctx context.Context, sessionID string) (*Session, error) {
	var session Session
	err := s.db.WithContext(ctx).
		Where("id = ?", sessionID).
		First(&session).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("会话不存在")
		}
		return nil, err
	}

	return &session, nil
}

// UpdateSessionLastUsed 更新会话最后使用时间
func (s *SessionService) UpdateSessionLastUsed(ctx context.Context, sessionID string) error {
	return s.db.WithContext(ctx).
		Model(&Session{}).
		Where("id = ?", sessionID).
		Update("last_used_at", time.Now()).Error
}

// RotateRefreshToken 更新刷新令牌并延长会话
func (s *SessionService) RotateRefreshToken(ctx context.Context, sessionID, refreshToken string, expiresAt time.Time) error {
	return s.db.WithContext(ctx).
		Model(&Session{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{
			"refresh_token": refreshToken,
			"expires_at":    expiresAt,
			"last_used_at":  time.Now(),
			"updated_at":    time.Now(),
		}).Error
}

// RevokeSession 撤销会话
func (s *SessionService) RevokeSession(ctx context.Context, sessionID string) error {
	return s.db.WithContext(ctx).
		Model(&Session{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{
			"is_revoked": true,
			"updated_at": time.Now(),
		}).Error
}

// RevokeSessionByRefreshToken 通过刷新令牌撤销会话
func (s *SessionService) RevokeSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	return s.db.WithContext(ctx).
		Model(&Session{}).
		Where("refresh_token = ?", refreshToken).
		Updates(map[string]any{
			"is_revoked": true,
			"updated_at": time.Now(),
		}).Error
}

// RevokeUserSessions 撤销用户的所有会话
func (s *SessionService) RevokeUserSessions(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).
		Model(&Session{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{
			"is_revoked": true,
			"updated_at": time.Now(),
		}).Error
}

// ListUserSessions 列出用户的所有会话
func (s *SessionService) ListUserSessions(ctx context.Context, userID string, includeRevoked bool) ([]*Session, error) {
	var sessions []*Session
	query := s.db.WithContext(ctx).Where("user_id = ?", userID)

	if !includeRevoked {
		query = query.Where("is_revoked = false")
	}

	err := query.Order("last_used_at DESC").Find(&sessions).Error
	return sessions, err
}

// CleanupExpiredSessions 清理过期会话
func (s *SessionService) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("expires_at < ? AND is_revoked = false", time.Now()).
		Updates(map[string]any{
			"is_revoked": true,
			"updated_at": time.Now(),
		})

	return result.RowsAffected, result.Error
}

// StartCleanupScheduler 启动定时清理任务（每小时清理一次过期会话）
func (s *SessionService) StartCleanupScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				count, err := s.CleanupExpiredSessions(ctx)
				if err != nil {
					// 记录错误但继续运行
					continue
				}
				if count > 0 {
					// 记录清理数量（可以替换为日志）
					_ = count
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
