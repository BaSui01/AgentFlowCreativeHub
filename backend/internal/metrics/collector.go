package metrics

import (
	"context"
	"database/sql"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SystemCollector 系统指标收集器
type SystemCollector struct {
	db *sql.DB
}

// NewSystemCollector 创建系统指标收集器
func NewSystemCollector(db *sql.DB) *SystemCollector {
	collector := &SystemCollector{
		db: db,
	}

	// 启动定期收集
	go collector.collectPeriodically()

	return collector
}

// collectPeriodically 定期收集系统指标
func (c *SystemCollector) collectPeriodically() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.collectOnce()
	}
}

// collectOnce 收集一次系统指标
func (c *SystemCollector) collectOnce() {
	// 收集数据库连接数
	if c.db != nil {
		c.collectDBStats()
	}

	// 收集 Go 运行时指标（内存、goroutine）
	c.collectRuntimeStats()
}

// collectDBStats 收集数据库统计信息
func (c *SystemCollector) collectDBStats() {
	stats := c.db.Stats()

	// 更新数据库连接指标
	DBConnections.WithLabelValues("open").Set(float64(stats.OpenConnections))
	DBConnections.WithLabelValues("in_use").Set(float64(stats.InUse))
	DBConnections.WithLabelValues("idle").Set(float64(stats.Idle))
}

// collectRuntimeStats 收集 Go 运行时统计信息
func (c *SystemCollector) collectRuntimeStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 记录内存指标
	goMemoryUsage.Set(float64(m.Alloc))
	goMemoryTotal.Set(float64(m.TotalAlloc))
	goMemorySys.Set(float64(m.Sys))

	// 记录 Goroutine 数量
	goGoroutines.Set(float64(runtime.NumGoroutine()))

	// 记录 GC 信息
	goGCCount.Set(float64(m.NumGC))
}

// Go 运行时指标
var (
	// goMemoryUsage 当前内存使用量（字节）
	goMemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "agentflow_go_memory_usage_bytes",
			Help: "当前 Go 内存使用量",
		},
	)

	// goMemoryTotal 累计内存分配量（字节）
	goMemoryTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "agentflow_go_memory_total_bytes",
			Help: "累计 Go 内存分配量",
		},
	)

	// goMemorySys 系统内存占用（字节）
	goMemorySys = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "agentflow_go_memory_sys_bytes",
			Help: "Go 从系统获取的内存",
		},
	)

	// goGoroutines Goroutine 数量
	goGoroutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "agentflow_go_goroutines",
			Help: "当前 Goroutine 数量",
		},
	)

	// goGCCount GC 执行次数
	goGCCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "agentflow_go_gc_count",
			Help: "GC 执行总次数",
		},
	)
)

// RAGCollector RAG 指标收集器
type RAGCollector struct {
	collectFunc func(ctx context.Context) (map[string]int, error)
}

// NewRAGCollector 创建 RAG 指标收集器
func NewRAGCollector(collectFunc func(ctx context.Context) (map[string]int, error)) *RAGCollector {
	collector := &RAGCollector{
		collectFunc: collectFunc,
	}

	// 启动定期收集
	if collectFunc != nil {
		go collector.collectPeriodically()
	}

	return collector
}

// collectPeriodically 定期收集 RAG 指标
func (c *RAGCollector) collectPeriodically() {
	ticker := time.NewTicker(60 * time.Second) // 每分钟收集一次
	defer ticker.Stop()

	for range ticker.C {
		c.collectOnce()
	}
}

// collectOnce 收集一次 RAG 指标
func (c *RAGCollector) collectOnce() {
	if c.collectFunc == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 调用收集函数获取知识库统计信息
	// 返回格式: {"kb-id-1": 100, "kb-id-2": 200, ...}
	stats, err := c.collectFunc(ctx)
	if err != nil {
		// 记录错误但不中断
		return
	}

	// 更新知识库文档数量
	for kbID, count := range stats {
		RAGDocumentsTotal.WithLabelValues(kbID).Set(float64(count))
	}
}

// RecordAgentExecution 记录 Agent 执行指标
// 使用便捷函数包装，在 Agent 执行前后调用
func RecordAgentExecution(agentType, tenantID string, fn func() error) error {
	// 增加运行中计数
	AgentExecutionsRunning.WithLabelValues(agentType).Inc()
	defer AgentExecutionsRunning.WithLabelValues(agentType).Dec()

	// 记录开始时间
	start := time.Now()

	// 执行函数
	err := fn()

	// 记录耗时
	duration := time.Since(start).Seconds()
	AgentExecutionDuration.WithLabelValues(agentType, tenantID).Observe(duration)

	// 记录执行结果
	status := "success"
	if err != nil {
		status = "failed"
	}
	AgentExecutionsTotal.WithLabelValues(agentType, status, tenantID).Inc()

	return err
}

// RecordRAGSearch 记录 RAG 检索指标
func RecordRAGSearch(kbID string, fn func() (int, error)) (int, error) {
	// 记录开始时间
	start := time.Now()

	// 执行检索
	resultCount, err := fn()

	// 记录耗时
	duration := time.Since(start).Seconds()
	RAGSearchDuration.WithLabelValues(kbID).Observe(duration)

	// 记录结果数量
	if resultCount > 0 {
		RAGSearchResults.WithLabelValues(kbID).Observe(float64(resultCount))
	}

	// 记录检索结果
	status := "success"
	if err != nil {
		status = "failed"
	}
	RAGSearchesTotal.WithLabelValues(kbID, status).Inc()

	return resultCount, err
}

// RecordModelCall 记录模型调用指标
func RecordModelCall(provider, model string, fn func() (int, int, error)) error {
	// 记录开始时间
	start := time.Now()

	// 执行调用
	promptTokens, completionTokens, err := fn()

	// 记录耗时
	duration := time.Since(start).Seconds()
	ModelCallDuration.WithLabelValues(provider, model).Observe(duration)

	// 记录 Token 数量
	if promptTokens > 0 {
		ModelCallTokens.WithLabelValues(provider, model, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		ModelCallTokens.WithLabelValues(provider, model, "completion").Add(float64(completionTokens))
	}

	// 记录调用结果
	status := "success"
	if err != nil {
		status = "failed"
	}
	ModelCallsTotal.WithLabelValues(provider, model, status).Inc()

	return err
}
