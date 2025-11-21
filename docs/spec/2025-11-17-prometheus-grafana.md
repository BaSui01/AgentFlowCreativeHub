# 📋 下一步功能实施规划

## 🎯 项目当前状态分析

### 已完成功能 ✅

| 模块 | 完成度 | 文件数 | 代码行数 | 状态 |
|------|--------|--------|---------|------|
| **基础设施** | 100% | 15+ | ~3,000 | ✅ 完成 |
| **Agent 运行时** | 100% | 11 | ~3,500 | ✅ **7种全部完成** |
| **AI 模型管理** | 100% | 13 | ~2,600 | ✅ 8个提供商 |
| **RAG 知识库** | 100% | 9 | ~2,160 | ✅ 完整功能 |
| **认证授权** | 100% | 10 | ~2,290 | ✅ JWT+OAuth2 |
| **审计日志** | 100% | 4 | ~800 | ✅ 40+事件类型 |
| **Prompt 模板** | 100% | 4 | ~1,000 | ✅ 版本管理 |
| **工作流编排** | 80% | 6 | ~2,300 | ⏳ 核心已完成 |
| **监控告警** | 0% | 0 | 0 | ❌ 待实施 |
| **前端控制台** | 0% | 0 | 0 | ❌ 待实施 |

**总体完成度**: 约 **70%**

---

## 🎯 推荐的下一步实施

基于项目当前状态和优先级，我推荐按以下顺序实施：

### 🔴 优先级 1 (高优先级，立即实施)

#### **选项 A: 监控告警系统（Prometheus + Grafana）**
**推荐指数**: ⭐⭐⭐⭐⭐

**理由**:
1. **生产就绪**: 系统已有 7 种 Agent、RAG 等核心功能，急需监控保障
2. **问题定位**: 帮助发现性能瓶颈、错误率等关键指标
3. **用户体验**: 实时监控系统健康状况
4. **运维支持**: 必备的生产环境基础设施
5. **技术成熟**: Prometheus 生态成熟，实施风险低

**预计耗时**: 4-6 小时

**技术价值**:
- 实时监控 API 请求（QPS、延迟、错误率）
- Agent 执行统计（成功率、平均耗时）
- RAG 检索性能（检索时间、命中率）
- 系统资源监控（CPU、内存、磁盘）
- 自定义告警规则

---

#### **选项 B: 完善工作流编排系统**
**推荐指数**: ⭐⭐⭐⭐

**理由**:
1. **功能完整性**: 工作流已完成 80%，补齐剩余功能即可上线
2. **业务价值**: 多 Agent 协作的核心能力
3. **用户需求**: 典型场景如 Writer→Reviewer→Formatter 流程
4. **代码基础**: 已有 Parser、Scheduler、Engine 实现

**预计耗时**: 3-4 小时

**待完善功能**:
- 条件分支（根据结果选择不同路径）
- 并行执行（多个 Agent 同时执行）
- 人工审核节点（暂停等待人工确认）
- 失败重试机制（自动重试失败步骤）
- 执行历史查询（完善查询接口）

---

#### **选项 C: Function Calling / 工具调用**
**推荐指数**: ⭐⭐⭐⭐

**理由**:
1. **Agent 能力扩展**: 让 Agent 可以调用外部工具
2. **应用场景丰富**: 搜索、计算、数据库查询等
3. **AI 趋势**: 主流 AI 模型都支持 Function Calling
4. **技术前沿**: 展示平台的先进性

**预计耗时**: 5-7 小时

**核心功能**:
- 工具注册机制（定义工具接口）
- 工具执行引擎（安全沙箱执行）
- 内置工具库（搜索、计算器、时间、天气）
- Function Calling 适配（转换为模型可理解的格式）
- 工具调用日志（审计和调试）

---

### 🟡 优先级 2 (中优先级，后续实施)

#### **选项 D: 前端控制台**
**推荐指数**: ⭐⭐⭐

**理由**:
1. **用户体验**: 提供可视化操作界面
2. **降低门槛**: 无需 API 调用即可使用
3. **功能展示**: 直观展示平台能力

**预计耗时**: 20-30 小时（React + TypeScript）

**核心功能**:
- Agent 管理界面
- 工作流可视化编辑器
- 知识库管理界面
- 执行监控看板

---

## 📋 监控告警系统详细设计 (推荐首选)

### 目标

构建完整的监控告警系统，涵盖：
1. **指标采集** - Prometheus 指标暴露
2. **可视化** - Grafana 监控面板
3. **告警规则** - 关键指标告警
4. **日志追踪** - 链路追踪（可选）

---

### 技术方案

#### 1. Prometheus 集成

**组件选择**:
- **prometheus/client_golang** - Go 官方客户端
- **gin-contrib/prom** - Gin 中间件（可选）

**指标类型**:

```go
// Counter - 累计计数
api_requests_total{method="POST",path="/api/agents/:id/execute",status="200"}

// Histogram - 分布统计
api_request_duration_seconds{method="POST",path="/api/agents/:id/execute"}

// Gauge - 即时状态
agent_executions_running{agent_type="writer"}

// Summary - 分位数统计
rag_search_duration_seconds{quantile="0.95"}
```

**核心指标设计**:

| 指标名称 | 类型 | 标签 | 描述 |
|---------|------|------|------|
| `api_requests_total` | Counter | method, path, status | API 请求总数 |
| `api_request_duration_seconds` | Histogram | method, path | API 请求延迟 |
| `agent_executions_total` | Counter | agent_type, status | Agent 执行总数 |
| `agent_execution_duration_seconds` | Histogram | agent_type | Agent 执行耗时 |
| `agent_executions_running` | Gauge | agent_type | 正在执行的 Agent 数量 |
| `rag_searches_total` | Counter | kb_id, status | RAG 检索总数 |
| `rag_search_duration_seconds` | Histogram | kb_id | RAG 检索耗时 |
| `model_calls_total` | Counter | provider, model, status | 模型调用总数 |
| `model_call_tokens` | Counter | provider, model, type | 模型 Token 消耗 |
| `db_connections` | Gauge | - | 数据库连接数 |
| `cache_hits_total` | Counter | cache_type | 缓存命中数 |
| `cache_misses_total` | Counter | cache_type | 缓存未命中数 |

---

#### 2. 实施步骤

##### 阶段 1: 基础 Prometheus 集成 (2 小时)

**文件创建**:
```
backend/internal/metrics/
├── metrics.go           # 指标定义和注册
├── collector.go         # 自定义指标收集器
└── middleware.go        # Gin 中间件

backend/api/
└── metrics.go           # /metrics 端点
```

**metrics.go** (核心指标定义):
```go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// API 指标
	APIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "API 请求总数",
		},
		[]string{"method", "path", "status"},
	)

	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "API 请求延迟",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Agent 指标
	AgentExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_executions_total",
			Help: "Agent 执行总数",
		},
		[]string{"agent_type", "status"},
	)

	AgentExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_execution_duration_seconds",
			Help:    "Agent 执行耗时",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"agent_type"},
	)

	AgentExecutionsRunning = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agent_executions_running",
			Help: "正在执行的 Agent 数量",
		},
		[]string{"agent_type"},
	)

	// RAG 指标
	RAGSearchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rag_searches_total",
			Help: "RAG 检索总数",
		},
		[]string{"kb_id", "status"},
	)

	RAGSearchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rag_search_duration_seconds",
			Help:    "RAG 检索耗时",
			Buckets: []float64{0.01, 0.05, 0.1, 0.2, 0.5, 1},
		},
		[]string{"kb_id"},
	)

	// 数据库指标
	DBConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections",
			Help: "数据库连接数",
		},
	)
)
```

**middleware.go** (Gin 中间件):
```go
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// PrometheusMiddleware Prometheus 指标收集中间件
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 执行请求
		c.Next()

		// 记录指标
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		APIRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		APIRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
```

**集成到 setup.go**:
```go
import (
	"backend/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	
	// Prometheus 中间件
	r.Use(metrics.PrometheusMiddleware())
	
	// Metrics 端点
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	// ... 其他路由
}
```

---

##### 阶段 2: Agent 和 RAG 指标埋点 (1.5 小时)

**修改文件**:
- `backend/internal/agent/runtime/registry.go` - Agent 执行指标
- `backend/internal/rag/rag_service.go` - RAG 检索指标

**Agent 执行指标埋点**:
```go
// registry.go - Execute 方法
func (r *Registry) Execute(ctx context.Context, tenantID, agentID string, input *AgentInput) (*AgentResult, error) {
	agent, err := r.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}

	// 记录开始
	agentType := agent.Type()
	metrics.AgentExecutionsRunning.WithLabelValues(agentType).Inc()
	defer metrics.AgentExecutionsRunning.WithLabelValues(agentType).Dec()

	start := time.Now()
	result, err := agent.Execute(ctx, input)
	duration := time.Since(start).Seconds()

	// 记录指标
	status := "success"
	if err != nil {
		status = "failed"
	}
	metrics.AgentExecutionsTotal.WithLabelValues(agentType, status).Inc()
	metrics.AgentExecutionDuration.WithLabelValues(agentType).Observe(duration)

	return result, err
}
```

**RAG 检索指标埋点**:
```go
// rag_service.go - Search 方法
func (s *RAGService) Search(ctx context.Context, kbID, query string, topK int) ([]*SearchResult, error) {
	start := time.Now()

	// 执行检索
	results, err := s.searchImpl(ctx, kbID, query, topK)

	// 记录指标
	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "failed"
	}
	metrics.RAGSearchesTotal.WithLabelValues(kbID, status).Inc()
	metrics.RAGSearchDuration.WithLabelValues(kbID).Observe(duration)

	return results, err
}
```

---

##### 阶段 3: Grafana 监控面板配置 (1.5 小时)

**创建 Grafana Dashboard JSON**:

**文件**: `config/grafana/agentflow_dashboard.json`

**面板布局**:

1. **系统概览行**:
   - QPS（实时）
   - 平均延迟（P50/P95/P99）
   - 错误率（%）
   - 活跃 Agent 数量

2. **API 监控行**:
   - API 请求趋势（时间序列）
   - API 延迟分布（热力图）
   - 状态码分布（饼图）

3. **Agent 执行行**:
   - Agent 执行次数（按类型）
   - Agent 执行耗时（箱线图）
   - Agent 成功率（%）

4. **RAG 检索行**:
   - RAG 检索 QPS
   - RAG 检索延迟（P95）
   - 知识库使用统计

5. **资源监控行**:
   - CPU 使用率
   - 内存使用率
   - 数据库连接数
   - 缓存命中率

**关键 Prometheus 查询**:

```promql
# API QPS
rate(api_requests_total[5m])

# API P95 延迟
histogram_quantile(0.95, rate(api_request_duration_seconds_bucket[5m]))

# API 错误率
rate(api_requests_total{status=~"5.."}[5m]) / rate(api_requests_total[5m]) * 100

# Agent 执行成功率
rate(agent_executions_total{status="success"}[5m]) / rate(agent_executions_total[5m]) * 100

# RAG 平均检索时间
rate(rag_search_duration_seconds_sum[5m]) / rate(rag_search_duration_seconds_count[5m])
```

---

##### 阶段 4: 告警规则配置 (1 小时)

**文件**: `config/prometheus/alerts.yml`

**关键告警规则**:

```yaml
groups:
  - name: agentflow_alerts
    interval: 30s
    rules:
      # API 错误率告警
      - alert: HighAPIErrorRate
        expr: |
          rate(api_requests_total{status=~"5.."}[5m]) / rate(api_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "API 错误率过高"
          description: "API 错误率 {{ $value | humanizePercentage }}，超过 5%"

      # API 延迟告警
      - alert: HighAPILatency
        expr: |
          histogram_quantile(0.95, rate(api_request_duration_seconds_bucket[5m])) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "API 延迟过高"
          description: "API P95 延迟 {{ $value }}s，超过 2s"

      # Agent 执行失败率告警
      - alert: HighAgentFailureRate
        expr: |
          rate(agent_executions_total{status="failed"}[5m]) / rate(agent_executions_total[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Agent 执行失败率过高"
          description: "Agent {{ $labels.agent_type }} 失败率 {{ $value | humanizePercentage }}"

      # RAG 检索慢查询告警
      - alert: SlowRAGSearch
        expr: |
          histogram_quantile(0.95, rate(rag_search_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "RAG 检索性能下降"
          description: "知识库 {{ $labels.kb_id }} P95 检索时间 {{ $value }}s"

      # 数据库连接数告警
      - alert: HighDBConnections
        expr: db_connections > 20
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "数据库连接数过高"
          description: "当前连接数 {{ $value }}，接近上限"
```

---

### 部署架构

```
┌─────────────────────────────────────────────────────────────┐
│                   AgentFlowCreativeHub                       │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Backend (Go)                                          │  │
│  │ - Prometheus Middleware                               │  │
│  │ - /metrics endpoint                                   │  │
│  │ - Agent/RAG metrics instrumentation                   │  │
│  └────────────────┬─────────────────────────────────────┘  │
└───────────────────┼─────────────────────────────────────────┘
                    │ HTTP :8080/metrics
                    ▼
┌───────────────────────────────────────────────────────────┐
│ Prometheus                                                 │
│ - Scrape metrics every 15s                                │
│ - Store time-series data                                  │
│ - Evaluate alert rules                                    │
│ - Send alerts to Alertmanager                            │
└────────────────┬──────────────────────────────────────────┘
                 │ PromQL Query
                 ▼
┌───────────────────────────────────────────────────────────┐
│ Grafana                                                    │
│ - AgentFlow Dashboard                                      │
│ - Real-time visualization                                 │
│ - Custom panels and queries                               │
└───────────────────────────────────────────────────────────┘
```

---

### Docker Compose 配置

**文件**: `docker-compose.monitoring.yml`

```yaml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: agentflow-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./config/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
      - ./config/prometheus/alerts.yml:/etc/prometheus/alerts.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/usr/share/prometheus/console_libraries'
      - '--web.console.templates=/usr/share/prometheus/consoles'
    networks:
      - agentflow

  grafana:
    image: grafana/grafana:latest
    container_name: agentflow-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - ./config/grafana/datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
      - ./config/grafana/agentflow_dashboard.json:/etc/grafana/provisioning/dashboards/agentflow_dashboard.json
      - grafana_data:/var/lib/grafana
    networks:
      - agentflow

volumes:
  prometheus_data:
  grafana_data:

networks:
  agentflow:
    external: true
```

---

### 预期成果

**代码变更**:
- 新增文件: 5 个
  - `backend/internal/metrics/metrics.go` (200 行)
  - `backend/internal/metrics/middleware.go` (80 行)
  - `backend/internal/metrics/collector.go` (150 行)
  - `config/prometheus/prometheus.yml` (50 行)
  - `config/prometheus/alerts.yml` (100 行)
  
- 修改文件: 3 个
  - `backend/api/setup.go` (+10 行)
  - `backend/internal/agent/runtime/registry.go` (+15 行)
  - `backend/internal/rag/rag_service.go` (+10 行)

- 配置文件: 2 个
  - `config/grafana/datasources.yml`
  - `config/grafana/agentflow_dashboard.json`

- Docker Compose: 1 个
  - `docker-compose.monitoring.yml`

**总新增代码**: ~630 行

---

### 验收标准

1. ✅ Prometheus 可以成功抓取 `/metrics` 端点
2. ✅ Grafana 可以连接到 Prometheus 数据源
3. ✅ 监控面板显示实时指标
4. ✅ API 请求、Agent 执行、RAG 检索指标正常记录
5. ✅ 告警规则正常评估和触发
6. ✅ 监控系统不影响主应用性能（<1% 开销）

---

### 后续扩展

1. **链路追踪** - Jaeger 集成（Sprint 9）
2. **日志聚合** - ELK/Loki 集成（Sprint 9）
3. **自定义告警通知** - 钉钉、邮件、Slack（Sprint 9）
4. **性能分析** - pprof 集成（Sprint 9）

---

## 时间估算

- **Prometheus 基础集成**: 2 小时
- **指标埋点**: 1.5 小时
- **Grafana 面板配置**: 1.5 小时
- **告警规则配置**: 1 小时

**总计**: 约 **6 小时**

---

准备好开始实施监控告警系统了吗？这将是系统走向生产环境的关键一步！

或者您更倾向于先完善工作流编排或实施 Function Calling？请告诉我您的选择！