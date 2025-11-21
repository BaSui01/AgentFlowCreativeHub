package notification

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// OfflineStore 用于缓存离线 WebSocket 消息
type OfflineStore interface {
	Append(ctx context.Context, tenantID, userID string, payload []byte) error
	Drain(ctx context.Context, tenantID, userID string) ([][]byte, error)
}

// MemoryOfflineStore 简单内存实现
type MemoryOfflineStore struct {
	mu    sync.Mutex
	limit int
	data  map[string]map[string][][]byte
}

// NewMemoryOfflineStore 创建内存存储
func NewMemoryOfflineStore(limit int) *MemoryOfflineStore {
	if limit <= 0 {
		limit = 50
	}
	return &MemoryOfflineStore{
		limit: limit,
		data:  make(map[string]map[string][][]byte),
	}
}

func (s *MemoryOfflineStore) Append(_ context.Context, tenantID, userID string, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[tenantID]; !ok {
		s.data[tenantID] = make(map[string][][]byte)
	}
	queue := append([][]byte{append([]byte(nil), payload...)}, s.data[tenantID][userID]...)
	if len(queue) > s.limit {
		queue = queue[:s.limit]
	}
	s.data[tenantID][userID] = queue
	return nil
}

func (s *MemoryOfflineStore) Drain(_ context.Context, tenantID, userID string) ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	queue := s.data[tenantID][userID]
	delete(s.data[tenantID], userID)
	if len(s.data[tenantID]) == 0 {
		delete(s.data, tenantID)
	}
	return queue, nil
}

// RedisOfflineStore 基于 Redis 的实现
type RedisOfflineStore struct {
	client *redis.Client
	limit  int
	ttl    time.Duration
}

// NewRedisOfflineStore 创建 redis 存储
func NewRedisOfflineStore(client *redis.Client, limit int, ttl time.Duration) *RedisOfflineStore {
	if limit <= 0 {
		limit = 100
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &RedisOfflineStore{client: client, limit: limit, ttl: ttl}
}

func (s *RedisOfflineStore) Append(ctx context.Context, tenantID, userID string, payload []byte) error {
	if s == nil || s.client == nil {
		return nil
	}
	key := s.key(tenantID, userID)
	pipe := s.client.TxPipeline()
	pipe.LPush(ctx, key, payload)
	pipe.LTrim(ctx, key, 0, int64(s.limit-1))
	pipe.Expire(ctx, key, s.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisOfflineStore) Drain(ctx context.Context, tenantID, userID string) ([][]byte, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	key := s.key(tenantID, userID)
	values, err := s.client.LRange(ctx, key, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	if len(values) > 0 {
		_ = s.client.Del(ctx, key).Err()
	}
	result := make([][]byte, 0, len(values))
	for _, v := range values {
		result = append(result, []byte(v))
	}
	return result, nil
}

func (s *RedisOfflineStore) key(tenantID, userID string) string {
	return "ws_offline:" + tenantID + ":" + userID
}
