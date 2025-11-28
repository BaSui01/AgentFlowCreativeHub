package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// EmbeddingCache 向量缓存服务
type EmbeddingCache struct {
	redis       *redis.Client
	localCache  sync.Map // 本地 L1 缓存
	prefix      string
	ttl         time.Duration
	maxLocalSize int
	localCount   int64
	mu           sync.RWMutex
}

// CachedEmbedding 缓存的向量
type CachedEmbedding struct {
	Vector    []float32 `json:"vector"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"createdAt"`
}

// NewEmbeddingCache 创建向量缓存
func NewEmbeddingCache(redisClient *redis.Client, prefix string, ttl time.Duration) *EmbeddingCache {
	if prefix == "" {
		prefix = "emb:"
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour // 默认 7 天
	}
	return &EmbeddingCache{
		redis:        redisClient,
		prefix:       prefix,
		ttl:          ttl,
		maxLocalSize: 10000, // 本地最多缓存 1 万条
	}
}

// Get 获取缓存的向量
func (c *EmbeddingCache) Get(ctx context.Context, text, model string) ([]float32, bool) {
	key := c.makeKey(text, model)

	// 先查本地缓存
	if val, ok := c.localCache.Load(key); ok {
		cached := val.(*CachedEmbedding)
		return cached.Vector, true
	}

	// 再查 Redis
	if c.redis != nil {
		data, err := c.redis.Get(ctx, key).Bytes()
		if err == nil {
			var cached CachedEmbedding
			if json.Unmarshal(data, &cached) == nil {
				// 存入本地缓存
				c.setLocal(key, &cached)
				return cached.Vector, true
			}
		}
	}

	return nil, false
}

// Set 设置缓存
func (c *EmbeddingCache) Set(ctx context.Context, text, model string, vector []float32) error {
	key := c.makeKey(text, model)
	cached := &CachedEmbedding{
		Vector:    vector,
		Model:     model,
		CreatedAt: time.Now(),
	}

	// 存入本地缓存
	c.setLocal(key, cached)

	// 存入 Redis
	if c.redis != nil {
		data, err := json.Marshal(cached)
		if err != nil {
			return err
		}
		return c.redis.Set(ctx, key, data, c.ttl).Err()
	}

	return nil
}

// GetBatch 批量获取缓存
func (c *EmbeddingCache) GetBatch(ctx context.Context, texts []string, model string) (map[string][]float32, []string) {
	results := make(map[string][]float32)
	var missing []string

	for _, text := range texts {
		if vec, ok := c.Get(ctx, text, model); ok {
			results[text] = vec
		} else {
			missing = append(missing, text)
		}
	}

	return results, missing
}

// SetBatch 批量设置缓存
func (c *EmbeddingCache) SetBatch(ctx context.Context, texts []string, model string, vectors [][]float32) error {
	if len(texts) != len(vectors) {
		return fmt.Errorf("texts and vectors length mismatch")
	}

	for i, text := range texts {
		if err := c.Set(ctx, text, model, vectors[i]); err != nil {
			return err
		}
	}

	return nil
}

// Delete 删除缓存
func (c *EmbeddingCache) Delete(ctx context.Context, text, model string) error {
	key := c.makeKey(text, model)
	c.localCache.Delete(key)
	if c.redis != nil {
		return c.redis.Del(ctx, key).Err()
	}
	return nil
}

// Clear 清空缓存
func (c *EmbeddingCache) Clear(ctx context.Context) error {
	// 清空本地缓存
	c.localCache = sync.Map{}
	c.mu.Lock()
	c.localCount = 0
	c.mu.Unlock()

	// 清空 Redis (使用 SCAN 避免阻塞)
	if c.redis != nil {
		iter := c.redis.Scan(ctx, 0, c.prefix+"*", 100).Iterator()
		var keys []string
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
			if len(keys) >= 100 {
				c.redis.Del(ctx, keys...)
				keys = nil
			}
		}
		if len(keys) > 0 {
			c.redis.Del(ctx, keys...)
		}
	}

	return nil
}

// Stats 获取缓存统计
func (c *EmbeddingCache) Stats(ctx context.Context) map[string]interface{} {
	c.mu.RLock()
	localCount := c.localCount
	c.mu.RUnlock()

	stats := map[string]interface{}{
		"localCount": localCount,
		"maxLocal":   c.maxLocalSize,
	}

	if c.redis != nil {
		// 统计 Redis 中的缓存数量
		count, _ := c.redis.DBSize(ctx).Result()
		stats["redisTotal"] = count
	}

	return stats
}

// makeKey 生成缓存键
func (c *EmbeddingCache) makeKey(text, model string) string {
	// 使用 SHA256 哈希文本内容
	hash := sha256.Sum256([]byte(text))
	return c.prefix + model + ":" + hex.EncodeToString(hash[:16]) // 只取前 16 字节
}

// setLocal 设置本地缓存
func (c *EmbeddingCache) setLocal(key string, cached *CachedEmbedding) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果本地缓存已满，清理一半
	if c.localCount >= int64(c.maxLocalSize) {
		c.evictLocal()
	}

	c.localCache.Store(key, cached)
	c.localCount++
}

// evictLocal 清理本地缓存
func (c *EmbeddingCache) evictLocal() {
	// 简单策略：清理一半缓存
	count := 0
	c.localCache.Range(func(key, value interface{}) bool {
		if count < c.maxLocalSize/2 {
			c.localCache.Delete(key)
			count++
			return true
		}
		return false
	})
	c.localCount -= int64(count)
}

// CachedEmbeddingProvider 带缓存的 Embedding 提供者包装器
type CachedEmbeddingProvider struct {
	provider EmbeddingProvider
	cache    *EmbeddingCache
}

// NewCachedEmbeddingProvider 创建带缓存的 Embedding 提供者
func NewCachedEmbeddingProvider(provider EmbeddingProvider, cache *EmbeddingCache) *CachedEmbeddingProvider {
	return &CachedEmbeddingProvider{
		provider: provider,
		cache:    cache,
	}
}

// Embed 单条向量化 (带缓存)
func (p *CachedEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	model := p.provider.GetModel()

	// 查缓存
	if vec, ok := p.cache.Get(ctx, text, model); ok {
		return vec, nil
	}

	// 调用原始提供者
	vec, err := p.provider.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// 存入缓存
	p.cache.Set(ctx, text, model, vec)

	return vec, nil
}

// EmbedBatch 批量向量化 (带缓存)
func (p *CachedEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	model := p.provider.GetModel()

	// 批量查缓存
	cached, missing := p.cache.GetBatch(ctx, texts, model)

	// 如果全部命中，直接返回
	if len(missing) == 0 {
		result := make([][]float32, len(texts))
		for i, text := range texts {
			result[i] = cached[text]
		}
		return result, nil
	}

	// 对缺失的进行向量化
	missingVectors, err := p.provider.EmbedBatch(ctx, missing)
	if err != nil {
		return nil, err
	}

	// 存入缓存
	p.cache.SetBatch(ctx, missing, model, missingVectors)

	// 合并结果
	missingMap := make(map[string][]float32)
	for i, text := range missing {
		missingMap[text] = missingVectors[i]
	}

	result := make([][]float32, len(texts))
	for i, text := range texts {
		if vec, ok := cached[text]; ok {
			result[i] = vec
		} else {
			result[i] = missingMap[text]
		}
	}

	return result, nil
}

// GetModel 获取模型名称
func (p *CachedEmbeddingProvider) GetModel() string {
	return p.provider.GetModel()
}

// GetProviderName 获取提供者名称
func (p *CachedEmbeddingProvider) GetProviderName() string {
	return p.provider.GetProviderName()
}
