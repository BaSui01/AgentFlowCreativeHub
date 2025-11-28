package ai

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 模型性能监控指标
var (
	modelRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_model_request_duration_seconds",
			Help:    "AI 模型请求耗时分布",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"provider", "model", "status"},
	)

	modelRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_model_request_total",
			Help: "AI 模型请求总数",
		},
		[]string{"provider", "model", "status"},
	)

	modelTokensUsed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_model_tokens_total",
			Help: "AI 模型 Token 使用总量",
		},
		[]string{"provider", "model", "type"}, // type: input/output
	)

	modelSuccessRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_model_success_rate",
			Help: "AI 模型请求成功率",
		},
		[]string{"provider", "model"},
	)

	modelLatencyP50 = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_model_latency_p50_seconds",
			Help: "AI 模型请求 P50 延迟",
		},
		[]string{"provider", "model"},
	)

	modelLatencyP99 = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_model_latency_p99_seconds",
			Help: "AI 模型请求 P99 延迟",
		},
		[]string{"provider", "model"},
	)
)

// PerformanceMonitor 模型性能监控器
type PerformanceMonitor struct {
	mu          sync.RWMutex
	stats       map[string]*ModelStats // key: provider:model
	window      time.Duration          // 统计窗口
	cleanupTick *time.Ticker
}

// ModelStats 单个模型的统计数据
type ModelStats struct {
	Provider string
	Model    string

	// 请求统计
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64

	// 延迟统计 (滑动窗口)
	Latencies []float64 // 最近 N 次请求延迟

	// Token 统计
	TotalInputTokens  int64
	TotalOutputTokens int64

	// 时间戳
	LastRequestTime time.Time
	FirstRequestTime time.Time
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor(window time.Duration) *PerformanceMonitor {
	pm := &PerformanceMonitor{
		stats:  make(map[string]*ModelStats),
		window: window,
	}

	// 启动定期清理和指标更新
	pm.cleanupTick = time.NewTicker(time.Minute)
	go pm.backgroundUpdate()

	return pm
}

// RecordRequest 记录请求
func (pm *PerformanceMonitor) RecordRequest(ctx context.Context, provider, model string, duration time.Duration, inputTokens, outputTokens int, err error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	key := provider + ":" + model
	stats, ok := pm.stats[key]
	if !ok {
		stats = &ModelStats{
			Provider:         provider,
			Model:            model,
			Latencies:        make([]float64, 0, 1000),
			FirstRequestTime: time.Now(),
		}
		pm.stats[key] = stats
	}

	// 更新统计
	stats.TotalRequests++
	stats.LastRequestTime = time.Now()
	stats.TotalInputTokens += int64(inputTokens)
	stats.TotalOutputTokens += int64(outputTokens)

	durationSec := duration.Seconds()
	status := "success"

	if err != nil {
		stats.FailedRequests++
		status = "error"
	} else {
		stats.SuccessRequests++
	}

	// 保留最近 1000 次延迟
	if len(stats.Latencies) >= 1000 {
		stats.Latencies = stats.Latencies[1:]
	}
	stats.Latencies = append(stats.Latencies, durationSec)

	// 更新 Prometheus 指标
	modelRequestDuration.WithLabelValues(provider, model, status).Observe(durationSec)
	modelRequestTotal.WithLabelValues(provider, model, status).Inc()
	modelTokensUsed.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	modelTokensUsed.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
}

// GetStats 获取模型统计
func (pm *PerformanceMonitor) GetStats(provider, model string) *ModelStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	key := provider + ":" + model
	if stats, ok := pm.stats[key]; ok {
		// 返回副本
		copy := *stats
		copy.Latencies = append([]float64{}, stats.Latencies...)
		return &copy
	}
	return nil
}

// GetAllStats 获取所有模型统计
func (pm *PerformanceMonitor) GetAllStats() map[string]*ModelStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]*ModelStats, len(pm.stats))
	for k, v := range pm.stats {
		copy := *v
		copy.Latencies = append([]float64{}, v.Latencies...)
		result[k] = &copy
	}
	return result
}

// GetSummary 获取模型性能摘要
func (pm *PerformanceMonitor) GetSummary(provider, model string) *ModelPerformanceSummary {
	stats := pm.GetStats(provider, model)
	if stats == nil {
		return nil
	}

	return &ModelPerformanceSummary{
		Provider:          provider,
		Model:             model,
		TotalRequests:     stats.TotalRequests,
		SuccessRate:       pm.calcSuccessRate(stats),
		AvgLatency:        pm.calcAvgLatency(stats.Latencies),
		P50Latency:        pm.calcPercentile(stats.Latencies, 50),
		P95Latency:        pm.calcPercentile(stats.Latencies, 95),
		P99Latency:        pm.calcPercentile(stats.Latencies, 99),
		TotalInputTokens:  stats.TotalInputTokens,
		TotalOutputTokens: stats.TotalOutputTokens,
		LastRequestTime:   stats.LastRequestTime,
	}
}

// ModelPerformanceSummary 模型性能摘要
type ModelPerformanceSummary struct {
	Provider          string    `json:"provider"`
	Model             string    `json:"model"`
	TotalRequests     int64     `json:"totalRequests"`
	SuccessRate       float64   `json:"successRate"`
	AvgLatency        float64   `json:"avgLatencyMs"`
	P50Latency        float64   `json:"p50LatencyMs"`
	P95Latency        float64   `json:"p95LatencyMs"`
	P99Latency        float64   `json:"p99LatencyMs"`
	TotalInputTokens  int64     `json:"totalInputTokens"`
	TotalOutputTokens int64     `json:"totalOutputTokens"`
	LastRequestTime   time.Time `json:"lastRequestTime"`
}

// backgroundUpdate 后台更新 Prometheus Gauge 指标
func (pm *PerformanceMonitor) backgroundUpdate() {
	for range pm.cleanupTick.C {
		pm.mu.RLock()
		for _, stats := range pm.stats {
			successRate := pm.calcSuccessRate(stats)
			p50 := pm.calcPercentile(stats.Latencies, 50)
			p99 := pm.calcPercentile(stats.Latencies, 99)

			modelSuccessRate.WithLabelValues(stats.Provider, stats.Model).Set(successRate)
			modelLatencyP50.WithLabelValues(stats.Provider, stats.Model).Set(p50)
			modelLatencyP99.WithLabelValues(stats.Provider, stats.Model).Set(p99)
		}
		pm.mu.RUnlock()
	}
}

// calcSuccessRate 计算成功率
func (pm *PerformanceMonitor) calcSuccessRate(stats *ModelStats) float64 {
	if stats.TotalRequests == 0 {
		return 0
	}
	return float64(stats.SuccessRequests) / float64(stats.TotalRequests)
}

// calcAvgLatency 计算平均延迟
func (pm *PerformanceMonitor) calcAvgLatency(latencies []float64) float64 {
	if len(latencies) == 0 {
		return 0
	}
	sum := 0.0
	for _, l := range latencies {
		sum += l
	}
	return (sum / float64(len(latencies))) * 1000 // 转换为毫秒
}

// calcPercentile 计算百分位数
func (pm *PerformanceMonitor) calcPercentile(latencies []float64, percentile float64) float64 {
	if len(latencies) == 0 {
		return 0
	}

	// 复制并排序
	sorted := append([]float64{}, latencies...)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	idx := int(float64(len(sorted)-1) * percentile / 100)
	return sorted[idx] * 1000 // 转换为毫秒
}

// Close 关闭监控器
func (pm *PerformanceMonitor) Close() {
	if pm.cleanupTick != nil {
		pm.cleanupTick.Stop()
	}
}

// 全局监控器实例
var globalMonitor *PerformanceMonitor
var monitorOnce sync.Once

// GetGlobalMonitor 获取全局监控器
func GetGlobalMonitor() *PerformanceMonitor {
	monitorOnce.Do(func() {
		globalMonitor = NewPerformanceMonitor(time.Hour)
	})
	return globalMonitor
}
