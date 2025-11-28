package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// BusinessMetrics 业务指标收集器
type BusinessMetrics struct {
	// Agent 相关指标
	agentExecutions       atomic.Int64
	agentExecutionsFailed atomic.Int64
	agentExecutionTime    *HistogramMetric

	// 工作流相关指标
	workflowExecutions       atomic.Int64
	workflowExecutionsFailed atomic.Int64
	workflowExecutionTime    *HistogramMetric

	// RAG 相关指标
	ragSearches       atomic.Int64
	ragSearchesFailed atomic.Int64
	ragSearchTime     *HistogramMetric
	ragDocumentsIndexed atomic.Int64

	// 用户相关指标
	userLogins        atomic.Int64
	userRegistrations atomic.Int64
	activeUsers       *GaugeMetric

	// Token 使用量
	tokensInput  atomic.Int64
	tokensOutput atomic.Int64

	// 自定义指标
	customCounters map[string]*atomic.Int64
	customGauges   map[string]*GaugeMetric
	customMu       sync.RWMutex
}

// HistogramMetric 直方图指标
type HistogramMetric struct {
	buckets    []float64
	counts     []atomic.Int64
	sum        atomic.Int64
	count      atomic.Int64
	mu         sync.RWMutex
}

// GaugeMetric 仪表盘指标（可增可减）
type GaugeMetric struct {
	value atomic.Int64
}

// NewBusinessMetrics 创建业务指标收集器
func NewBusinessMetrics() *BusinessMetrics {
	return &BusinessMetrics{
		agentExecutionTime:    NewHistogramMetric([]float64{0.1, 0.5, 1, 2, 5, 10, 30, 60}),
		workflowExecutionTime: NewHistogramMetric([]float64{1, 5, 10, 30, 60, 120, 300}),
		ragSearchTime:         NewHistogramMetric([]float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5}),
		activeUsers:           NewGaugeMetric(),
		customCounters:        make(map[string]*atomic.Int64),
		customGauges:          make(map[string]*GaugeMetric),
	}
}

// NewHistogramMetric 创建直方图指标
func NewHistogramMetric(buckets []float64) *HistogramMetric {
	h := &HistogramMetric{
		buckets: buckets,
		counts:  make([]atomic.Int64, len(buckets)+1), // +1 for +Inf
	}
	return h
}

// Observe 记录观测值
func (h *HistogramMetric) Observe(value float64) {
	h.sum.Add(int64(value * 1000)) // 转换为毫秒整数存储
	h.count.Add(1)

	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i].Add(1)
			return
		}
	}
	h.counts[len(h.buckets)].Add(1) // +Inf bucket
}

// GetPercentile 获取百分位数（近似值）
func (h *HistogramMetric) GetPercentile(p float64) float64 {
	total := h.count.Load()
	if total == 0 {
		return 0
	}

	target := int64(float64(total) * p)
	var cumulative int64

	for i, bucket := range h.buckets {
		cumulative += h.counts[i].Load()
		if cumulative >= target {
			return bucket
		}
	}

	return h.buckets[len(h.buckets)-1]
}

// GetStats 获取统计信息
func (h *HistogramMetric) GetStats() map[string]float64 {
	count := h.count.Load()
	if count == 0 {
		return map[string]float64{
			"count": 0,
			"sum":   0,
			"avg":   0,
			"p50":   0,
			"p95":   0,
			"p99":   0,
		}
	}

	sum := float64(h.sum.Load()) / 1000 // 转回秒
	return map[string]float64{
		"count": float64(count),
		"sum":   sum,
		"avg":   sum / float64(count),
		"p50":   h.GetPercentile(0.50),
		"p95":   h.GetPercentile(0.95),
		"p99":   h.GetPercentile(0.99),
	}
}

// NewGaugeMetric 创建仪表盘指标
func NewGaugeMetric() *GaugeMetric {
	return &GaugeMetric{}
}

func (g *GaugeMetric) Set(value int64) {
	g.value.Store(value)
}

func (g *GaugeMetric) Inc() {
	g.value.Add(1)
}

func (g *GaugeMetric) Dec() {
	g.value.Add(-1)
}

func (g *GaugeMetric) Add(delta int64) {
	g.value.Add(delta)
}

func (g *GaugeMetric) Get() int64 {
	return g.value.Load()
}

// ========== Agent 指标 ==========

func (m *BusinessMetrics) RecordAgentExecution(success bool, duration time.Duration) {
	m.agentExecutions.Add(1)
	if !success {
		m.agentExecutionsFailed.Add(1)
	}
	m.agentExecutionTime.Observe(duration.Seconds())
}

func (m *BusinessMetrics) GetAgentStats() map[string]any {
	total := m.agentExecutions.Load()
	failed := m.agentExecutionsFailed.Load()
	successRate := float64(0)
	if total > 0 {
		successRate = float64(total-failed) / float64(total)
	}

	return map[string]any{
		"total":        total,
		"failed":       failed,
		"success_rate": successRate,
		"latency":      m.agentExecutionTime.GetStats(),
	}
}

// ========== 工作流指标 ==========

func (m *BusinessMetrics) RecordWorkflowExecution(success bool, duration time.Duration) {
	m.workflowExecutions.Add(1)
	if !success {
		m.workflowExecutionsFailed.Add(1)
	}
	m.workflowExecutionTime.Observe(duration.Seconds())
}

func (m *BusinessMetrics) GetWorkflowStats() map[string]any {
	total := m.workflowExecutions.Load()
	failed := m.workflowExecutionsFailed.Load()
	successRate := float64(0)
	if total > 0 {
		successRate = float64(total-failed) / float64(total)
	}

	return map[string]any{
		"total":        total,
		"failed":       failed,
		"success_rate": successRate,
		"latency":      m.workflowExecutionTime.GetStats(),
	}
}

// ========== RAG 指标 ==========

func (m *BusinessMetrics) RecordRAGSearch(success bool, duration time.Duration) {
	m.ragSearches.Add(1)
	if !success {
		m.ragSearchesFailed.Add(1)
	}
	m.ragSearchTime.Observe(duration.Seconds())
}

func (m *BusinessMetrics) RecordDocumentIndexed(count int64) {
	m.ragDocumentsIndexed.Add(count)
}

func (m *BusinessMetrics) GetRAGStats() map[string]any {
	total := m.ragSearches.Load()
	failed := m.ragSearchesFailed.Load()
	successRate := float64(0)
	if total > 0 {
		successRate = float64(total-failed) / float64(total)
	}

	return map[string]any{
		"searches": map[string]any{
			"total":        total,
			"failed":       failed,
			"success_rate": successRate,
			"latency":      m.ragSearchTime.GetStats(),
		},
		"documents_indexed": m.ragDocumentsIndexed.Load(),
	}
}

// ========== 用户指标 ==========

func (m *BusinessMetrics) RecordUserLogin() {
	m.userLogins.Add(1)
}

func (m *BusinessMetrics) RecordUserRegistration() {
	m.userRegistrations.Add(1)
}

func (m *BusinessMetrics) SetActiveUsers(count int64) {
	m.activeUsers.Set(count)
}

func (m *BusinessMetrics) UserConnected() {
	m.activeUsers.Inc()
}

func (m *BusinessMetrics) UserDisconnected() {
	m.activeUsers.Dec()
}

func (m *BusinessMetrics) GetUserStats() map[string]any {
	return map[string]any{
		"logins":        m.userLogins.Load(),
		"registrations": m.userRegistrations.Load(),
		"active":        m.activeUsers.Get(),
	}
}

// ========== Token 使用量 ==========

func (m *BusinessMetrics) RecordTokenUsage(input, output int64) {
	m.tokensInput.Add(input)
	m.tokensOutput.Add(output)
}

func (m *BusinessMetrics) GetTokenStats() map[string]int64 {
	return map[string]int64{
		"input":  m.tokensInput.Load(),
		"output": m.tokensOutput.Load(),
		"total":  m.tokensInput.Load() + m.tokensOutput.Load(),
	}
}

// ========== 自定义指标 ==========

func (m *BusinessMetrics) IncrementCounter(name string) {
	m.customMu.Lock()
	if _, ok := m.customCounters[name]; !ok {
		m.customCounters[name] = &atomic.Int64{}
	}
	m.customMu.Unlock()

	m.customMu.RLock()
	m.customCounters[name].Add(1)
	m.customMu.RUnlock()
}

func (m *BusinessMetrics) AddCounter(name string, value int64) {
	m.customMu.Lock()
	if _, ok := m.customCounters[name]; !ok {
		m.customCounters[name] = &atomic.Int64{}
	}
	m.customMu.Unlock()

	m.customMu.RLock()
	m.customCounters[name].Add(value)
	m.customMu.RUnlock()
}

func (m *BusinessMetrics) SetGauge(name string, value int64) {
	m.customMu.Lock()
	if _, ok := m.customGauges[name]; !ok {
		m.customGauges[name] = NewGaugeMetric()
	}
	m.customGauges[name].Set(value)
	m.customMu.Unlock()
}

func (m *BusinessMetrics) GetCustomMetrics() map[string]any {
	m.customMu.RLock()
	defer m.customMu.RUnlock()

	counters := make(map[string]int64)
	for name, counter := range m.customCounters {
		counters[name] = counter.Load()
	}

	gauges := make(map[string]int64)
	for name, gauge := range m.customGauges {
		gauges[name] = gauge.Get()
	}

	return map[string]any{
		"counters": counters,
		"gauges":   gauges,
	}
}

// ========== 导出所有指标 ==========

func (m *BusinessMetrics) GetAllStats() map[string]any {
	return map[string]any{
		"agent":    m.GetAgentStats(),
		"workflow": m.GetWorkflowStats(),
		"rag":      m.GetRAGStats(),
		"user":     m.GetUserStats(),
		"tokens":   m.GetTokenStats(),
		"custom":   m.GetCustomMetrics(),
	}
}

// PrometheusFormat 导出 Prometheus 格式
func (m *BusinessMetrics) PrometheusFormat() string {
	var b []byte

	// Agent 指标
	b = append(b, []byte("# HELP agentflow_agent_executions_total Total agent executions\n")...)
	b = append(b, []byte("# TYPE agentflow_agent_executions_total counter\n")...)
	b = append(b, []byte("agentflow_agent_executions_total "+formatInt64(m.agentExecutions.Load())+"\n")...)

	b = append(b, []byte("# HELP agentflow_agent_executions_failed_total Failed agent executions\n")...)
	b = append(b, []byte("# TYPE agentflow_agent_executions_failed_total counter\n")...)
	b = append(b, []byte("agentflow_agent_executions_failed_total "+formatInt64(m.agentExecutionsFailed.Load())+"\n")...)

	// 工作流指标
	b = append(b, []byte("# HELP agentflow_workflow_executions_total Total workflow executions\n")...)
	b = append(b, []byte("# TYPE agentflow_workflow_executions_total counter\n")...)
	b = append(b, []byte("agentflow_workflow_executions_total "+formatInt64(m.workflowExecutions.Load())+"\n")...)

	// RAG 指标
	b = append(b, []byte("# HELP agentflow_rag_searches_total Total RAG searches\n")...)
	b = append(b, []byte("# TYPE agentflow_rag_searches_total counter\n")...)
	b = append(b, []byte("agentflow_rag_searches_total "+formatInt64(m.ragSearches.Load())+"\n")...)

	b = append(b, []byte("# HELP agentflow_documents_indexed_total Total documents indexed\n")...)
	b = append(b, []byte("# TYPE agentflow_documents_indexed_total counter\n")...)
	b = append(b, []byte("agentflow_documents_indexed_total "+formatInt64(m.ragDocumentsIndexed.Load())+"\n")...)

	// 用户指标
	b = append(b, []byte("# HELP agentflow_user_logins_total Total user logins\n")...)
	b = append(b, []byte("# TYPE agentflow_user_logins_total counter\n")...)
	b = append(b, []byte("agentflow_user_logins_total "+formatInt64(m.userLogins.Load())+"\n")...)

	b = append(b, []byte("# HELP agentflow_active_users Current active users\n")...)
	b = append(b, []byte("# TYPE agentflow_active_users gauge\n")...)
	b = append(b, []byte("agentflow_active_users "+formatInt64(m.activeUsers.Get())+"\n")...)

	// Token 指标
	b = append(b, []byte("# HELP agentflow_tokens_total Total tokens used\n")...)
	b = append(b, []byte("# TYPE agentflow_tokens_total counter\n")...)
	b = append(b, []byte("agentflow_tokens_total{type=\"input\"} "+formatInt64(m.tokensInput.Load())+"\n")...)
	b = append(b, []byte("agentflow_tokens_total{type=\"output\"} "+formatInt64(m.tokensOutput.Load())+"\n")...)

	return string(b)
}

func formatInt64(v int64) string {
	return string(formatIntBuf(v))
}

func formatIntBuf(v int64) []byte {
	if v == 0 {
		return []byte("0")
	}

	negative := v < 0
	if negative {
		v = -v
	}

	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}

	if negative {
		i--
		buf[i] = '-'
	}

	return buf[i:]
}
