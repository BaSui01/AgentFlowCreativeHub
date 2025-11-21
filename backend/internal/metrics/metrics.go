package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// API 指标
var (
	// APIRequestsTotal API 请求总数
	APIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_api_requests_total",
			Help: "API 请求总数",
		},
		[]string{"method", "path", "status"},
	)

	// APIRequestDuration API 请求延迟（秒）
	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_api_request_duration_seconds",
			Help:    "API 请求延迟分布",
			Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
		},
		[]string{"method", "path"},
	)

	// APIRequestSize API 请求体大小（字节）
	APIRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_api_request_size_bytes",
			Help:    "API 请求体大小分布",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "path"},
	)

	// APIResponseSize API 响应体大小（字节）
	APIResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_api_response_size_bytes",
			Help:    "API 响应体大小分布",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "path"},
	)
)

// Agent 执行指标
var (
	// AgentExecutionsTotal Agent 执行总数
	AgentExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_agent_executions_total",
			Help: "Agent 执行总数",
		},
		[]string{"agent_type", "status", "tenant_id"},
	)

	// AgentExecutionDuration Agent 执行耗时（秒）
	AgentExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_agent_execution_duration_seconds",
			Help:    "Agent 执行耗时分布",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"agent_type", "tenant_id"},
	)

	// AgentExecutionsRunning 正在执行的 Agent 数量
	AgentExecutionsRunning = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentflow_agent_executions_running",
			Help: "正在执行的 Agent 数量",
		},
		[]string{"agent_type"},
	)

	// AgentInputTokens Agent 输入 Token 数量
	AgentInputTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_agent_input_tokens_total",
			Help: "Agent 输入 Token 总数",
		},
		[]string{"agent_type", "tenant_id"},
	)

	// AgentOutputTokens Agent 输出 Token 数量
	AgentOutputTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_agent_output_tokens_total",
			Help: "Agent 输出 Token 总数",
		},
		[]string{"agent_type", "tenant_id"},
	)
)

// RAG 检索指标
var (
	// RAGSearchesTotal RAG 检索总数
	RAGSearchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_rag_searches_total",
			Help: "RAG 检索总数",
		},
		[]string{"kb_id", "status"},
	)

	// RAGSearchDuration RAG 检索耗时（秒）
	RAGSearchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_rag_search_duration_seconds",
			Help:    "RAG 检索耗时分布",
			Buckets: []float64{0.01, 0.05, 0.1, 0.2, 0.5, 1, 2},
		},
		[]string{"kb_id"},
	)

	// RAGSearchResults RAG 检索结果数量
	RAGSearchResults = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_rag_search_results",
			Help:    "RAG 检索返回结果数量分布",
			Buckets: []float64{1, 3, 5, 10, 20, 50},
		},
		[]string{"kb_id"},
	)

	// RAGDocumentsTotal 知识库文档总数
	RAGDocumentsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentflow_rag_documents_total",
			Help: "知识库文档总数",
		},
		[]string{"kb_id"},
	)

	// RAGChunksTotal 知识库文档块总数
	RAGChunksTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentflow_rag_chunks_total",
			Help: "知识库文档块总数",
		},
		[]string{"kb_id"},
	)
)

// AI 模型调用指标
var (
	// ModelCallsTotal 模型调用总数
	ModelCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_model_calls_total",
			Help: "AI 模型调用总数",
		},
		[]string{"provider", "model", "status"},
	)

	// ModelCallDuration 模型调用耗时（秒）
	ModelCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_model_call_duration_seconds",
			Help:    "AI 模型调用耗时分布",
			Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30, 60},
		},
		[]string{"provider", "model"},
	)

	// ModelCallTokens 模型调用 Token 数量
	ModelCallTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_model_call_tokens_total",
			Help: "AI 模型调用 Token 总数",
		},
		[]string{"provider", "model", "type"}, // type: prompt, completion
	)
)

// 工作流执行指标
var (
	// WorkflowExecutionsTotal 工作流执行总数
	WorkflowExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_workflow_executions_total",
			Help: "工作流执行总数",
		},
		[]string{"workflow_id", "status", "tenant_id"},
	)

	// WorkflowExecutionDuration 工作流执行耗时（秒）
	WorkflowExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_workflow_execution_duration_seconds",
			Help:    "工作流执行耗时分布",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"workflow_id"},
	)

	// WorkflowTasksTotal 工作流任务总数
	WorkflowTasksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_workflow_tasks_total",
			Help: "工作流任务总数",
		},
		[]string{"workflow_id", "step_id", "status"},
	)
)

// 审批与通知指标
var (
	ApprovalPendingGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentflow_approval_pending_total",
			Help: "当前待审批数量",
		},
		[]string{"tenant_id"},
	)

	ApprovalNotificationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_approval_notifications_total",
			Help: "审批通知发送次数",
		},
		[]string{"channel", "tenant_id", "status"},
	)

	ApprovalDecisionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_approval_decisions_total",
			Help: "审批决策次数",
		},
		[]string{"tenant_id", "status", "decision_type"},
	)

	WebSocketConnectionsGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentflow_ws_connections",
			Help: "WebSocket 在线连接数",
		},
		[]string{"tenant_id"},
	)
)

// 数据库指标
var (
	// DBConnections 数据库连接数
	DBConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentflow_db_connections",
			Help: "数据库连接数",
		},
		[]string{"state"}, // state: open, in_use, idle
	)

	// DBQueryDuration 数据库查询耗时（秒）
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_db_query_duration_seconds",
			Help:    "数据库查询耗时分布",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		},
		[]string{"operation"}, // operation: select, insert, update, delete
	)

	// DBQueriesTotal 数据库查询总数
	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_db_queries_total",
			Help: "数据库查询总数",
		},
		[]string{"operation", "status"},
	)
)

// 缓存指标
var (
	// CacheHitsTotal 缓存命中数
	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_cache_hits_total",
			Help: "缓存命中总数",
		},
		[]string{"cache_type"},
	)

	// CacheMissesTotal 缓存未命中数
	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentflow_cache_misses_total",
			Help: "缓存未命中总数",
		},
		[]string{"cache_type"},
	)

	// CacheOperationDuration 缓存操作耗时（秒）
	CacheOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentflow_cache_operation_duration_seconds",
			Help:    "缓存操作耗时分布",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05},
		},
		[]string{"cache_type", "operation"}, // operation: get, set, delete
	)
)

// 系统指标
var (
	// BuildInfo 构建信息
	BuildInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentflow_build_info",
			Help: "AgentFlow 构建信息",
		},
		[]string{"version", "go_version", "commit"},
	)
)

// RecordBuildInfo 记录构建信息
func RecordBuildInfo(version, goVersion, commit string) {
	BuildInfo.WithLabelValues(version, goVersion, commit).Set(1)
}
