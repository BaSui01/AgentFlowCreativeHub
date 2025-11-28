package cache

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CacheMonitor 缓存监控
type CacheMonitor struct {
	caches   map[string]*CacheMetrics
	mu       sync.RWMutex
	recorder MetricsRecorder
}

// CacheMetrics 缓存指标
type CacheMetrics struct {
	Name         string
	Hits         atomic.Int64
	Misses       atomic.Int64
	Sets         atomic.Int64
	Deletes      atomic.Int64
	Evictions    atomic.Int64
	Errors       atomic.Int64
	BytesRead    atomic.Int64
	BytesWritten atomic.Int64
	ItemCount    atomic.Int64
	SizeBytes    atomic.Int64
	AvgLatencyNs atomic.Int64
	latencyCount atomic.Int64
}

// MetricsRecorder 指标记录器接口
type MetricsRecorder interface {
	RecordCacheHit(name string)
	RecordCacheMiss(name string)
	RecordCacheSet(name string, size int64)
	RecordCacheEviction(name string)
}

// NewCacheMonitor 创建缓存监控
func NewCacheMonitor(recorder MetricsRecorder) *CacheMonitor {
	return &CacheMonitor{
		caches:   make(map[string]*CacheMetrics),
		recorder: recorder,
	}
}

// GetOrCreateMetrics 获取或创建缓存指标
func (m *CacheMonitor) GetOrCreateMetrics(name string) *CacheMetrics {
	m.mu.RLock()
	metrics, ok := m.caches[name]
	m.mu.RUnlock()

	if ok {
		return metrics
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if metrics, ok = m.caches[name]; ok {
		return metrics
	}

	metrics = &CacheMetrics{Name: name}
	m.caches[name] = metrics
	return metrics
}

// RecordHit 记录命中
func (m *CacheMonitor) RecordHit(name string, latency time.Duration, bytes int64) {
	metrics := m.GetOrCreateMetrics(name)
	metrics.Hits.Add(1)
	metrics.BytesRead.Add(bytes)
	m.updateLatency(metrics, latency)

	if m.recorder != nil {
		m.recorder.RecordCacheHit(name)
	}
}

// RecordMiss 记录未命中
func (m *CacheMonitor) RecordMiss(name string, latency time.Duration) {
	metrics := m.GetOrCreateMetrics(name)
	metrics.Misses.Add(1)
	m.updateLatency(metrics, latency)

	if m.recorder != nil {
		m.recorder.RecordCacheMiss(name)
	}
}

// RecordSet 记录写入
func (m *CacheMonitor) RecordSet(name string, bytes int64) {
	metrics := m.GetOrCreateMetrics(name)
	metrics.Sets.Add(1)
	metrics.BytesWritten.Add(bytes)

	if m.recorder != nil {
		m.recorder.RecordCacheSet(name, bytes)
	}
}

// RecordDelete 记录删除
func (m *CacheMonitor) RecordDelete(name string) {
	metrics := m.GetOrCreateMetrics(name)
	metrics.Deletes.Add(1)
}

// RecordEviction 记录淘汰
func (m *CacheMonitor) RecordEviction(name string) {
	metrics := m.GetOrCreateMetrics(name)
	metrics.Evictions.Add(1)

	if m.recorder != nil {
		m.recorder.RecordCacheEviction(name)
	}
}

// RecordError 记录错误
func (m *CacheMonitor) RecordError(name string) {
	metrics := m.GetOrCreateMetrics(name)
	metrics.Errors.Add(1)
}

// UpdateItemCount 更新条目数
func (m *CacheMonitor) UpdateItemCount(name string, count int64) {
	metrics := m.GetOrCreateMetrics(name)
	metrics.ItemCount.Store(count)
}

// UpdateSizeBytes 更新大小
func (m *CacheMonitor) UpdateSizeBytes(name string, size int64) {
	metrics := m.GetOrCreateMetrics(name)
	metrics.SizeBytes.Store(size)
}

func (m *CacheMonitor) updateLatency(metrics *CacheMetrics, latency time.Duration) {
	count := metrics.latencyCount.Add(1)
	oldAvg := metrics.AvgLatencyNs.Load()
	newAvg := oldAvg + (int64(latency)-oldAvg)/count
	metrics.AvgLatencyNs.Store(newAvg)
}

// GetStats 获取缓存统计
func (m *CacheMonitor) GetStats(name string) *CacheStats {
	m.mu.RLock()
	metrics, ok := m.caches[name]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	return m.metricsToStats(metrics)
}

// GetAllStats 获取所有缓存统计
func (m *CacheMonitor) GetAllStats() map[string]*CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*CacheStats)
	for name, metrics := range m.caches {
		result[name] = m.metricsToStats(metrics)
	}
	return result
}

// CacheStats 缓存统计快照
type CacheStats struct {
	Name         string        `json:"name"`
	Hits         int64         `json:"hits"`
	Misses       int64         `json:"misses"`
	HitRate      float64       `json:"hit_rate"`
	Sets         int64         `json:"sets"`
	Deletes      int64         `json:"deletes"`
	Evictions    int64         `json:"evictions"`
	Errors       int64         `json:"errors"`
	BytesRead    int64         `json:"bytes_read"`
	BytesWritten int64         `json:"bytes_written"`
	ItemCount    int64         `json:"item_count"`
	SizeBytes    int64         `json:"size_bytes"`
	AvgLatency   time.Duration `json:"avg_latency"`
}

func (m *CacheMonitor) metricsToStats(metrics *CacheMetrics) *CacheStats {
	hits := metrics.Hits.Load()
	misses := metrics.Misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return &CacheStats{
		Name:         metrics.Name,
		Hits:         hits,
		Misses:       misses,
		HitRate:      hitRate,
		Sets:         metrics.Sets.Load(),
		Deletes:      metrics.Deletes.Load(),
		Evictions:    metrics.Evictions.Load(),
		Errors:       metrics.Errors.Load(),
		BytesRead:    metrics.BytesRead.Load(),
		BytesWritten: metrics.BytesWritten.Load(),
		ItemCount:    metrics.ItemCount.Load(),
		SizeBytes:    metrics.SizeBytes.Load(),
		AvgLatency:   time.Duration(metrics.AvgLatencyNs.Load()),
	}
}

// Reset 重置指标
func (m *CacheMonitor) Reset(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.caches, name)
}

// ResetAll 重置所有指标
func (m *CacheMonitor) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.caches = make(map[string]*CacheMetrics)
}

// PrometheusFormat 输出 Prometheus 格式
func (m *CacheMonitor) PrometheusFormat() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var b []byte

	b = append(b, "# HELP cache_hits_total Total cache hits\n"...)
	b = append(b, "# TYPE cache_hits_total counter\n"...)
	for name, metrics := range m.caches {
		b = append(b, fmt.Sprintf("cache_hits_total{cache=\"%s\"} %d\n", name, metrics.Hits.Load())...)
	}

	b = append(b, "# HELP cache_misses_total Total cache misses\n"...)
	b = append(b, "# TYPE cache_misses_total counter\n"...)
	for name, metrics := range m.caches {
		b = append(b, fmt.Sprintf("cache_misses_total{cache=\"%s\"} %d\n", name, metrics.Misses.Load())...)
	}

	b = append(b, "# HELP cache_hit_rate Cache hit rate\n"...)
	b = append(b, "# TYPE cache_hit_rate gauge\n"...)
	for name, metrics := range m.caches {
		hits := metrics.Hits.Load()
		misses := metrics.Misses.Load()
		total := hits + misses
		var rate float64
		if total > 0 {
			rate = float64(hits) / float64(total)
		}
		b = append(b, fmt.Sprintf("cache_hit_rate{cache=\"%s\"} %.4f\n", name, rate)...)
	}

	b = append(b, "# HELP cache_size_bytes Current cache size in bytes\n"...)
	b = append(b, "# TYPE cache_size_bytes gauge\n"...)
	for name, metrics := range m.caches {
		b = append(b, fmt.Sprintf("cache_size_bytes{cache=\"%s\"} %d\n", name, metrics.SizeBytes.Load())...)
	}

	b = append(b, "# HELP cache_items Current number of cached items\n"...)
	b = append(b, "# TYPE cache_items gauge\n"...)
	for name, metrics := range m.caches {
		b = append(b, fmt.Sprintf("cache_items{cache=\"%s\"} %d\n", name, metrics.ItemCount.Load())...)
	}

	b = append(b, "# HELP cache_evictions_total Total cache evictions\n"...)
	b = append(b, "# TYPE cache_evictions_total counter\n"...)
	for name, metrics := range m.caches {
		b = append(b, fmt.Sprintf("cache_evictions_total{cache=\"%s\"} %d\n", name, metrics.Evictions.Load())...)
	}

	return string(b)
}

// MonitoredCache 带监控的缓存包装器
type MonitoredCache struct {
	cache   Cache
	monitor *CacheMonitor
	name    string
}

// Cache 缓存接口
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Clear() error
}

// NewMonitoredCache 创建带监控的缓存
func NewMonitoredCache(cache Cache, monitor *CacheMonitor, name string) *MonitoredCache {
	return &MonitoredCache{
		cache:   cache,
		monitor: monitor,
		name:    name,
	}
}

func (c *MonitoredCache) Get(key string) ([]byte, bool) {
	start := time.Now()
	value, ok := c.cache.Get(key)
	latency := time.Since(start)

	if ok {
		c.monitor.RecordHit(c.name, latency, int64(len(value)))
	} else {
		c.monitor.RecordMiss(c.name, latency)
	}

	return value, ok
}

func (c *MonitoredCache) Set(key string, value []byte, ttl time.Duration) error {
	err := c.cache.Set(key, value, ttl)
	if err != nil {
		c.monitor.RecordError(c.name)
	} else {
		c.monitor.RecordSet(c.name, int64(len(value)))
	}
	return err
}

func (c *MonitoredCache) Delete(key string) error {
	err := c.cache.Delete(key)
	if err != nil {
		c.monitor.RecordError(c.name)
	} else {
		c.monitor.RecordDelete(c.name)
	}
	return err
}

func (c *MonitoredCache) Clear() error {
	return c.cache.Clear()
}
