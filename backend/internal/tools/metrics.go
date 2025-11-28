package tools

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ToolMetrics 工具性能监控
type ToolMetrics struct {
	tools    map[string]*ToolStats
	mu       sync.RWMutex
	recorder MetricsRecorder
}

// ToolStats 单个工具的统计数据
type ToolStats struct {
	Name           string `json:"name"`
	TotalCalls     atomic.Int64
	SuccessCalls   atomic.Int64
	FailedCalls    atomic.Int64
	TotalDuration  atomic.Int64 // 纳秒
	MinDuration    atomic.Int64
	MaxDuration    atomic.Int64
	LastCalled     atomic.Int64 // Unix 时间戳
	LastError      atomic.Value // string
	latencyBuckets []atomic.Int64
}

// MetricsRecorder 指标记录接口（可对接 Prometheus）
type MetricsRecorder interface {
	RecordToolCall(tool string, success bool, duration time.Duration)
}

// NewToolMetrics 创建工具指标收集器
func NewToolMetrics(recorder MetricsRecorder) *ToolMetrics {
	return &ToolMetrics{
		tools:    make(map[string]*ToolStats),
		recorder: recorder,
	}
}

// getOrCreateStats 获取或创建工具统计
func (m *ToolMetrics) getOrCreateStats(name string) *ToolStats {
	m.mu.RLock()
	stats, ok := m.tools[name]
	m.mu.RUnlock()

	if ok {
		return stats
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if stats, ok = m.tools[name]; ok {
		return stats
	}

	stats = &ToolStats{
		Name: name,
		// 延迟分桶: 10ms, 50ms, 100ms, 500ms, 1s, 5s, 10s, 30s
		latencyBuckets: make([]atomic.Int64, 8),
	}
	stats.MinDuration.Store(int64(^uint64(0) >> 1)) // Max int64
	m.tools[name] = stats
	return stats
}

// RecordCall 记录工具调用
func (m *ToolMetrics) RecordCall(name string, success bool, duration time.Duration, err error) {
	stats := m.getOrCreateStats(name)

	stats.TotalCalls.Add(1)
	if success {
		stats.SuccessCalls.Add(1)
	} else {
		stats.FailedCalls.Add(1)
		if err != nil {
			stats.LastError.Store(err.Error())
		}
	}

	durationNs := duration.Nanoseconds()
	stats.TotalDuration.Add(durationNs)
	stats.LastCalled.Store(time.Now().Unix())

	// 更新最小/最大延迟
	for {
		old := stats.MinDuration.Load()
		if durationNs >= old || stats.MinDuration.CompareAndSwap(old, durationNs) {
			break
		}
	}
	for {
		old := stats.MaxDuration.Load()
		if durationNs <= old || stats.MaxDuration.CompareAndSwap(old, durationNs) {
			break
		}
	}

	// 更新延迟分桶
	bucketIdx := m.getBucketIndex(duration)
	stats.latencyBuckets[bucketIdx].Add(1)

	// 外部记录器
	if m.recorder != nil {
		m.recorder.RecordToolCall(name, success, duration)
	}
}

func (m *ToolMetrics) getBucketIndex(d time.Duration) int {
	ms := d.Milliseconds()
	switch {
	case ms < 10:
		return 0
	case ms < 50:
		return 1
	case ms < 100:
		return 2
	case ms < 500:
		return 3
	case ms < 1000:
		return 4
	case ms < 5000:
		return 5
	case ms < 10000:
		return 6
	default:
		return 7
	}
}

// GetStats 获取工具统计
func (m *ToolMetrics) GetStats(name string) *ToolStatsSnapshot {
	m.mu.RLock()
	stats, ok := m.tools[name]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	return m.snapshotStats(stats)
}

// GetAllStats 获取所有工具统计
func (m *ToolMetrics) GetAllStats() map[string]*ToolStatsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ToolStatsSnapshot)
	for name, stats := range m.tools {
		result[name] = m.snapshotStats(stats)
	}
	return result
}

// ToolStatsSnapshot 工具统计快照
type ToolStatsSnapshot struct {
	Name           string         `json:"name"`
	TotalCalls     int64          `json:"total_calls"`
	SuccessCalls   int64          `json:"success_calls"`
	FailedCalls    int64          `json:"failed_calls"`
	SuccessRate    float64        `json:"success_rate"`
	AvgDuration    time.Duration  `json:"avg_duration"`
	MinDuration    time.Duration  `json:"min_duration"`
	MaxDuration    time.Duration  `json:"max_duration"`
	P50            time.Duration  `json:"p50"`
	P95            time.Duration  `json:"p95"`
	P99            time.Duration  `json:"p99"`
	LastCalled     *time.Time     `json:"last_called,omitempty"`
	LastError      string         `json:"last_error,omitempty"`
	LatencyBuckets map[string]int64 `json:"latency_buckets"`
}

func (m *ToolMetrics) snapshotStats(stats *ToolStats) *ToolStatsSnapshot {
	total := stats.TotalCalls.Load()
	success := stats.SuccessCalls.Load()
	failed := stats.FailedCalls.Load()
	totalDuration := stats.TotalDuration.Load()

	snapshot := &ToolStatsSnapshot{
		Name:         stats.Name,
		TotalCalls:   total,
		SuccessCalls: success,
		FailedCalls:  failed,
	}

	if total > 0 {
		snapshot.SuccessRate = float64(success) / float64(total)
		snapshot.AvgDuration = time.Duration(totalDuration / total)
	}

	minDur := stats.MinDuration.Load()
	if minDur != int64(^uint64(0)>>1) {
		snapshot.MinDuration = time.Duration(minDur)
	}
	snapshot.MaxDuration = time.Duration(stats.MaxDuration.Load())

	// 计算百分位数
	buckets := make([]int64, len(stats.latencyBuckets))
	for i := range buckets {
		buckets[i] = stats.latencyBuckets[i].Load()
	}
	snapshot.P50 = m.calculatePercentile(buckets, 0.50)
	snapshot.P95 = m.calculatePercentile(buckets, 0.95)
	snapshot.P99 = m.calculatePercentile(buckets, 0.99)

	lastCalled := stats.LastCalled.Load()
	if lastCalled > 0 {
		t := time.Unix(lastCalled, 0)
		snapshot.LastCalled = &t
	}

	if lastErr := stats.LastError.Load(); lastErr != nil {
		snapshot.LastError = lastErr.(string)
	}

	// 延迟分桶
	snapshot.LatencyBuckets = map[string]int64{
		"<10ms":   buckets[0],
		"<50ms":   buckets[1],
		"<100ms":  buckets[2],
		"<500ms":  buckets[3],
		"<1s":     buckets[4],
		"<5s":     buckets[5],
		"<10s":    buckets[6],
		">=10s":   buckets[7],
	}

	return snapshot
}

func (m *ToolMetrics) calculatePercentile(buckets []int64, p float64) time.Duration {
	var total int64
	for _, count := range buckets {
		total += count
	}

	if total == 0 {
		return 0
	}

	target := int64(float64(total) * p)
	var cumulative int64

	// 桶对应的上限值（毫秒）
	bucketLimits := []int64{10, 50, 100, 500, 1000, 5000, 10000, 30000}

	for i, count := range buckets {
		cumulative += count
		if cumulative >= target {
			return time.Duration(bucketLimits[i]) * time.Millisecond
		}
	}

	return time.Duration(bucketLimits[len(bucketLimits)-1]) * time.Millisecond
}

// Reset 重置指标
func (m *ToolMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools = make(map[string]*ToolStats)
}

// ResetTool 重置单个工具指标
func (m *ToolMetrics) ResetTool(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tools, name)
}

// ToolExecutionWrapper 工具执行包装器（自动记录指标）
type ToolExecutionWrapper struct {
	metrics *ToolMetrics
}

func NewToolExecutionWrapper(metrics *ToolMetrics) *ToolExecutionWrapper {
	return &ToolExecutionWrapper{metrics: metrics}
}

// Wrap 包装工具执行函数
func (w *ToolExecutionWrapper) Wrap(name string, fn func(ctx context.Context, input map[string]any) (map[string]any, error)) func(ctx context.Context, input map[string]any) (map[string]any, error) {
	return func(ctx context.Context, input map[string]any) (map[string]any, error) {
		start := time.Now()
		result, err := fn(ctx, input)
		duration := time.Since(start)

		w.metrics.RecordCall(name, err == nil, duration, err)

		return result, err
	}
}
