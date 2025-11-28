// Package cache 提供 Prompt 缓存功能
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// PromptCacheConfig 缓存配置
type PromptCacheConfig struct {
	EmbeddingTTL     time.Duration // Embedding 缓存 TTL（永久建议设为 0 或很长时间）
	SystemPromptTTL  time.Duration // 系统 Prompt 缓存 TTL
	TemplatePromptTTL time.Duration // 模板 Prompt 缓存 TTL
	WorkflowStepTTL  time.Duration // 工作流步骤缓存 TTL
	RAGResultTTL     time.Duration // RAG 结果缓存 TTL
	MaxEntries       int           // 最大条目数
}

// DefaultPromptCacheConfig 默认配置
func DefaultPromptCacheConfig() *PromptCacheConfig {
	return &PromptCacheConfig{
		EmbeddingTTL:      0,                     // 永久缓存
		SystemPromptTTL:   7 * 24 * time.Hour,    // 7 天
		TemplatePromptTTL: 3 * 24 * time.Hour,    // 3 天
		WorkflowStepTTL:   1 * time.Hour,         // 1 小时
		RAGResultTTL:      30 * time.Minute,      // 30 分钟
		MaxEntries:        100000,
	}
}

// CacheType 缓存类型
type CacheType string

const (
	CacheTypeEmbedding     CacheType = "embedding"
	CacheTypeSystemPrompt  CacheType = "system_prompt"
	CacheTypeTemplatePrompt CacheType = "template_prompt"
	CacheTypeWorkflowStep  CacheType = "workflow_step"
	CacheTypeRAGResult     CacheType = "rag_result"
)

// PromptCacheEntry 缓存条目
type PromptCacheEntry struct {
	Key       string          `json:"key"`
	Type      CacheType       `json:"type"`
	Value     json.RawMessage `json:"value"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	HitCount  int64           `json:"hit_count"`
}

// PromptCache Prompt 缓存管理器
type PromptCache struct {
	diskCache *DiskCache
	memCache  map[string]*PromptCacheEntry
	config    *PromptCacheConfig
	mu        sync.RWMutex
	
	// 统计
	hits   int64
	misses int64
}

// NewPromptCache 创建 Prompt 缓存
func NewPromptCache(diskCache *DiskCache, config *PromptCacheConfig) *PromptCache {
	if config == nil {
		config = DefaultPromptCacheConfig()
	}
	return &PromptCache{
		diskCache: diskCache,
		memCache:  make(map[string]*PromptCacheEntry),
		config:    config,
	}
}

// generateKey 生成缓存键
func (c *PromptCache) generateKey(cacheType CacheType, content string) string {
	hash := sha256.Sum256([]byte(string(cacheType) + ":" + content))
	return hex.EncodeToString(hash[:])
}

// getTTL 获取对应类型的 TTL
func (c *PromptCache) getTTL(cacheType CacheType) time.Duration {
	switch cacheType {
	case CacheTypeEmbedding:
		return c.config.EmbeddingTTL
	case CacheTypeSystemPrompt:
		return c.config.SystemPromptTTL
	case CacheTypeTemplatePrompt:
		return c.config.TemplatePromptTTL
	case CacheTypeWorkflowStep:
		return c.config.WorkflowStepTTL
	case CacheTypeRAGResult:
		return c.config.RAGResultTTL
	default:
		return time.Hour
	}
}

// Get 获取缓存
func (c *PromptCache) Get(ctx context.Context, cacheType CacheType, content string) (json.RawMessage, bool) {
	key := c.generateKey(cacheType, content)
	
	// 先查内存缓存
	c.mu.RLock()
	if entry, ok := c.memCache[key]; ok {
		if entry.ExpiresAt == nil || entry.ExpiresAt.After(time.Now()) {
			c.mu.RUnlock()
			c.mu.Lock()
			entry.HitCount++
			c.hits++
			c.mu.Unlock()
			return entry.Value, true
		}
	}
	c.mu.RUnlock()
	
	// 内存未命中，查磁盘缓存
	if c.diskCache != nil {
		if entry, err := c.diskCache.Get(ctx, key); err == nil && entry != nil {
			// 反序列化
			var cachedValue json.RawMessage
			if err := json.Unmarshal([]byte(entry.Response), &cachedValue); err == nil {
				// 回填内存缓存
				c.mu.Lock()
				c.memCache[key] = &PromptCacheEntry{
					Key:       key,
					Type:      cacheType,
					Value:     cachedValue,
					CreatedAt: entry.CreatedAt,
					ExpiresAt: entry.ExpiresAt,
					HitCount:  int64(entry.HitCount),
				}
				c.hits++
				c.mu.Unlock()
				return cachedValue, true
			}
		}
	}
	
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
	return nil, false
}

// Set 设置缓存
func (c *PromptCache) Set(ctx context.Context, cacheType CacheType, content string, value interface{}) error {
	key := c.generateKey(cacheType, content)
	
	// 序列化值
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	
	ttl := c.getTTL(cacheType)
	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expiresAt = &t
	}
	
	entry := &PromptCacheEntry{
		Key:       key,
		Type:      cacheType,
		Value:     valueBytes,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		HitCount:  0,
	}
	
	// 写入内存缓存
	c.mu.Lock()
	c.memCache[key] = entry
	c.mu.Unlock()
	
	// 写入磁盘缓存
	if c.diskCache != nil {
		diskEntry := &CacheEntry{
			CacheKey:       key,
			Model:          string(cacheType),
			PromptHash:     content[:min(100, len(content))], // 存储部分内容用于调试
			Response:       string(valueBytes),
			CreatedAt:      entry.CreatedAt,
			LastAccessedAt: entry.CreatedAt,
			ExpiresAt:      expiresAt,
		}
		return c.diskCache.Set(ctx, diskEntry)
	}
	
	return nil
}

// ============================================================================
// Embedding 缓存便捷方法
// ============================================================================

// GetEmbedding 获取 Embedding 缓存
func (c *PromptCache) GetEmbedding(ctx context.Context, text string) ([]float32, bool) {
	value, ok := c.Get(ctx, CacheTypeEmbedding, text)
	if !ok {
		return nil, false
	}
	
	var embedding []float32
	if err := json.Unmarshal(value, &embedding); err != nil {
		return nil, false
	}
	return embedding, true
}

// SetEmbedding 设置 Embedding 缓存
func (c *PromptCache) SetEmbedding(ctx context.Context, text string, embedding []float32) error {
	return c.Set(ctx, CacheTypeEmbedding, text, embedding)
}

// ============================================================================
// 系统 Prompt 缓存便捷方法
// ============================================================================

// GetSystemPromptResponse 获取系统 Prompt 响应缓存
func (c *PromptCache) GetSystemPromptResponse(ctx context.Context, systemPrompt, userMessage string) (string, bool) {
	key := systemPrompt + "|" + userMessage
	value, ok := c.Get(ctx, CacheTypeSystemPrompt, key)
	if !ok {
		return "", false
	}
	
	var response string
	if err := json.Unmarshal(value, &response); err != nil {
		return "", false
	}
	return response, true
}

// SetSystemPromptResponse 设置系统 Prompt 响应缓存
func (c *PromptCache) SetSystemPromptResponse(ctx context.Context, systemPrompt, userMessage, response string) error {
	key := systemPrompt + "|" + userMessage
	return c.Set(ctx, CacheTypeSystemPrompt, key, response)
}

// ============================================================================
// RAG 结果缓存
// ============================================================================

// RAGResult RAG 检索结果
type RAGResult struct {
	Query   string   `json:"query"`
	Results []string `json:"results"`
	Scores  []float64 `json:"scores"`
}

// GetRAGResult 获取 RAG 结果缓存
func (c *PromptCache) GetRAGResult(ctx context.Context, kbID, query string) (*RAGResult, bool) {
	key := kbID + "|" + query
	value, ok := c.Get(ctx, CacheTypeRAGResult, key)
	if !ok {
		return nil, false
	}
	
	var result RAGResult
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, false
	}
	return &result, true
}

// SetRAGResult 设置 RAG 结果缓存
func (c *PromptCache) SetRAGResult(ctx context.Context, kbID, query string, result *RAGResult) error {
	key := kbID + "|" + query
	return c.Set(ctx, CacheTypeRAGResult, key, result)
}

// ============================================================================
// 统计
// ============================================================================

// PromptCacheStats 缓存统计
type PromptCacheStats struct {
	MemoryEntries int   `json:"memory_entries"`
	TotalHits     int64 `json:"total_hits"`
	TotalMisses   int64 `json:"total_misses"`
	HitRate       float64 `json:"hit_rate"`
}

// GetStats 获取缓存统计
func (c *PromptCache) GetStats() *PromptCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}
	
	return &PromptCacheStats{
		MemoryEntries: len(c.memCache),
		TotalHits:     c.hits,
		TotalMisses:   c.misses,
		HitRate:       hitRate,
	}
}

// Cleanup 清理过期条目
func (c *PromptCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for key, entry := range c.memCache {
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(now) {
			delete(c.memCache, key)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
