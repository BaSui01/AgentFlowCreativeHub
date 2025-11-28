package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisSessionStore Redis 会话存储
type RedisSessionStore struct {
	client     *redis.Client
	prefix     string
	defaultTTL time.Duration
}

// SessionData 会话数据
type SessionData struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"userId"`
	TenantID     string                 `json:"tenantId"`
	DeviceID     string                 `json:"deviceId"`
	DeviceType   string                 `json:"deviceType"`
	IP           string                 `json:"ip"`
	UserAgent    string                 `json:"userAgent"`
	Data         map[string]interface{} `json:"data"`
	CreatedAt    time.Time              `json:"createdAt"`
	LastActiveAt time.Time              `json:"lastActiveAt"`
	ExpiresAt    time.Time              `json:"expiresAt"`
}

// NewRedisSessionStore 创建 Redis 会话存储
func NewRedisSessionStore(client *redis.Client, prefix string, defaultTTL time.Duration) *RedisSessionStore {
	if prefix == "" {
		prefix = "session:"
	}
	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}
	return &RedisSessionStore{
		client:     client,
		prefix:     prefix,
		defaultTTL: defaultTTL,
	}
}

// Create 创建会话
func (s *RedisSessionStore) Create(ctx context.Context, session *SessionData) error {
	if session.ID == "" {
		return fmt.Errorf("session ID is required")
	}

	session.CreatedAt = time.Now()
	session.LastActiveAt = time.Now()
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = time.Now().Add(s.defaultTTL)
	}

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	// 存储会话
	key := s.prefix + session.ID
	ttl := time.Until(session.ExpiresAt)
	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("set session: %w", err)
	}

	// 添加到用户的会话集合 (用于多设备管理)
	userKey := s.prefix + "user:" + session.UserID
	if err := s.client.SAdd(ctx, userKey, session.ID).Err(); err != nil {
		return fmt.Errorf("add to user sessions: %w", err)
	}
	s.client.Expire(ctx, userKey, s.defaultTTL*30) // 用户会话集合保留更长时间

	return nil
}

// Get 获取会话
func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (*SessionData, error) {
	key := s.prefix + sessionID
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // 会话不存在
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		s.Delete(ctx, sessionID)
		return nil, nil
	}

	return &session, nil
}

// Update 更新会话
func (s *RedisSessionStore) Update(ctx context.Context, session *SessionData) error {
	session.LastActiveAt = time.Now()

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	key := s.prefix + session.ID
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return s.Delete(ctx, session.ID)
	}

	return s.client.Set(ctx, key, data, ttl).Err()
}

// Delete 删除会话
func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	// 获取会话以找到 userID
	session, _ := s.Get(ctx, sessionID)

	key := s.prefix + sessionID
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	// 从用户会话集合中移除
	if session != nil {
		userKey := s.prefix + "user:" + session.UserID
		s.client.SRem(ctx, userKey, sessionID)
	}

	return nil
}

// Refresh 刷新会话 (续期)
func (s *RedisSessionStore) Refresh(ctx context.Context, sessionID string, ttl time.Duration) error {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}

	session.ExpiresAt = time.Now().Add(ttl)
	session.LastActiveAt = time.Now()

	return s.Update(ctx, session)
}

// GetUserSessions 获取用户的所有会话
func (s *RedisSessionStore) GetUserSessions(ctx context.Context, userID string) ([]*SessionData, error) {
	userKey := s.prefix + "user:" + userID
	sessionIDs, err := s.client.SMembers(ctx, userKey).Result()
	if err != nil {
		return nil, fmt.Errorf("get user sessions: %w", err)
	}

	var sessions []*SessionData
	for _, id := range sessionIDs {
		session, err := s.Get(ctx, id)
		if err != nil {
			continue
		}
		if session != nil {
			sessions = append(sessions, session)
		} else {
			// 清理无效的会话 ID
			s.client.SRem(ctx, userKey, id)
		}
	}

	return sessions, nil
}

// DeleteUserSessions 删除用户的所有会话 (强制下线)
func (s *RedisSessionStore) DeleteUserSessions(ctx context.Context, userID string) error {
	sessions, err := s.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		s.Delete(ctx, session.ID)
	}

	// 删除用户会话集合
	userKey := s.prefix + "user:" + userID
	return s.client.Del(ctx, userKey).Err()
}

// DeleteOtherSessions 删除用户的其他会话 (保留当前会话)
func (s *RedisSessionStore) DeleteOtherSessions(ctx context.Context, userID, currentSessionID string) error {
	sessions, err := s.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if session.ID != currentSessionID {
			s.Delete(ctx, session.ID)
		}
	}

	return nil
}

// CountUserSessions 统计用户会话数
func (s *RedisSessionStore) CountUserSessions(ctx context.Context, userID string) (int64, error) {
	userKey := s.prefix + "user:" + userID
	return s.client.SCard(ctx, userKey).Result()
}

// EnforceMaxSessions 限制最大会话数 (删除最旧的会话)
func (s *RedisSessionStore) EnforceMaxSessions(ctx context.Context, userID string, maxSessions int) error {
	sessions, err := s.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	if len(sessions) <= maxSessions {
		return nil
	}

	// 按创建时间排序
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].CreatedAt.After(sessions[j].CreatedAt) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	// 删除最旧的会话
	toDelete := len(sessions) - maxSessions
	for i := 0; i < toDelete; i++ {
		s.Delete(ctx, sessions[i].ID)
	}

	return nil
}

// SetData 设置会话数据
func (s *RedisSessionStore) SetData(ctx context.Context, sessionID string, key string, value interface{}) error {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}

	if session.Data == nil {
		session.Data = make(map[string]interface{})
	}
	session.Data[key] = value

	return s.Update(ctx, session)
}

// GetData 获取会话数据
func (s *RedisSessionStore) GetData(ctx context.Context, sessionID string, key string) (interface{}, error) {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}

	if session.Data == nil {
		return nil, nil
	}

	return session.Data[key], nil
}

// Touch 更新会话活跃时间
func (s *RedisSessionStore) Touch(ctx context.Context, sessionID string) error {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return nil
	}

	session.LastActiveAt = time.Now()
	return s.Update(ctx, session)
}

// CleanupExpired 清理过期会话 (定期任务)
func (s *RedisSessionStore) CleanupExpired(ctx context.Context) (int, error) {
	// Redis 会自动清理过期的 key，这里主要清理用户会话集合中的无效引用
	var cleaned int

	// 扫描所有用户会话集合
	iter := s.client.Scan(ctx, 0, s.prefix+"user:*", 100).Iterator()
	for iter.Next(ctx) {
		userKey := iter.Val()
		sessionIDs, _ := s.client.SMembers(ctx, userKey).Result()

		for _, id := range sessionIDs {
			key := s.prefix + id
			exists, _ := s.client.Exists(ctx, key).Result()
			if exists == 0 {
				s.client.SRem(ctx, userKey, id)
				cleaned++
			}
		}
	}

	return cleaned, nil
}
