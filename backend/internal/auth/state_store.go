package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrStateNotFound 表示 state 不存在或已过期
var ErrStateNotFound = errors.New("auth: state not found")

// StateStore OAuth2 state 存储接口
type StateStore interface {
	Save(ctx context.Context, state string, provider string, ttl time.Duration) error
	Consume(ctx context.Context, state string) (string, error)
}

// MemoryStateStore 内存实现
type MemoryStateStore struct {
	mu     sync.Mutex
	data   map[string]stateEntry
	maxTTL time.Duration
}

type stateEntry struct {
	provider string
	expires  time.Time
}

// NewMemoryStateStore 创建内存 state 存储
func NewMemoryStateStore(maxTTL time.Duration) *MemoryStateStore {
	return &MemoryStateStore{
		data:   make(map[string]stateEntry),
		maxTTL: maxTTL,
	}
}

// Save 写入 state
func (s *MemoryStateStore) Save(ctx context.Context, state string, provider string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[state] = stateEntry{
		provider: provider,
		expires:  time.Now().Add(minDuration(ttl, s.maxTTL)),
	}
	return nil
}

// Consume 读取并删除 state
func (s *MemoryStateStore) Consume(ctx context.Context, state string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.data[state]
	if !ok {
		return "", ErrStateNotFound
	}
	delete(s.data, state)
	if time.Now().After(entry.expires) {
		return "", ErrStateNotFound
	}
	return entry.provider, nil
}

// RedisStateStore Redis 实现
type RedisStateStore struct {
	client *redis.Client
	prefix string
}

// NewRedisStateStore 创建 Redis state 存储
func NewRedisStateStore(client *redis.Client) *RedisStateStore {
	return &RedisStateStore{
		client: client,
		prefix: "oauth2:state:",
	}
}

// Save 写入 state
func (s *RedisStateStore) Save(ctx context.Context, state string, provider string, ttl time.Duration) error {
	if s.client == nil {
		return errors.New("redis client is nil")
	}
	return s.client.Set(ctx, s.prefix+state, provider, ttl).Err()
}

// Consume 读取并删除 state
func (s *RedisStateStore) Consume(ctx context.Context, state string) (string, error) {
	if s.client == nil {
		return "", errors.New("redis client is nil")
	}
	cmd := s.client.GetDel(ctx, s.prefix+state)
	if err := cmd.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrStateNotFound
		}
		return "", err
	}
	return cmd.Val(), nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
