package cache

import (
	"context"

	"backend/internal/metrics"
)

// RecordCacheHit 记录缓存命中
func (c *DiskCache) RecordCacheHit() {
	c.statsMu.Lock()
	c.cacheHits++
	c.totalRequests++
	c.statsMu.Unlock()

	metrics.CacheHitsTotal.WithLabelValues("disk").Inc()
}

// RecordCacheMiss 记录缓存未命中
func (c *DiskCache) RecordCacheMiss() {
	c.statsMu.Lock()
	c.cacheMisses++
	c.totalRequests++
	c.statsMu.Unlock()

	metrics.CacheMissesTotal.WithLabelValues("disk").Inc()
}

// RecordCacheOperation 记录缓存操作延迟
func RecordCacheOperation(cacheType, operation string, durationSeconds float64) {
	metrics.CacheOperationDuration.WithLabelValues(cacheType, operation).Observe(durationSeconds)
}

// DiskCacheMetrics 硬盘缓存指标
type DiskCacheMetrics struct {
	TotalRequests int64   `json:"total_requests"`
	CacheHits     int64   `json:"cache_hits"`
	CacheMisses   int64   `json:"cache_misses"`
	HitRate       float64 `json:"hit_rate"`
	TotalSize     int64   `json:"total_size_bytes"`
	EntryCount    int64   `json:"entry_count"`
}

// GetMetrics 获取缓存指标
func (c *DiskCache) GetMetrics() *DiskCacheMetrics {
	c.statsMu.RLock()
	totalRequests := c.totalRequests
	cacheHits := c.cacheHits
	cacheMisses := c.cacheMisses
	c.statsMu.RUnlock()

	var totalSize int64
	var entryCount int64

	// 从数据库获取统计
	stats, err := c.GetStats(context.Background())
	if err == nil {
		if v, ok := stats["total_size_kb"].(int64); ok {
			totalSize = v * 1024
		}
		if v, ok := stats["total_entries"].(int); ok {
			entryCount = int64(v)
		}
	}

	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(cacheHits) / float64(totalRequests) * 100
	}

	return &DiskCacheMetrics{
		TotalRequests: totalRequests,
		CacheHits:     cacheHits,
		CacheMisses:   cacheMisses,
		HitRate:       hitRate,
		TotalSize:     totalSize,
		EntryCount:    entryCount,
	}
}
