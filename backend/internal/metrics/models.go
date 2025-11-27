package metrics

import "time"

// ============================================================================
// 模型调用记录
// ============================================================================

// ModelCallLog AI模型调用日志
type ModelCallLog struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenant_id" gorm:"type:uuid;not null;index"`
	UserID    string    `json:"user_id" gorm:"type:uuid;index"`
	SessionID string    `json:"session_id" gorm:"type:uuid;index"`
	
	// 模型信息
	ModelID       string `json:"model_id" gorm:"type:uuid;not null;index"`
	ModelName     string `json:"model_name" gorm:"size:255;not null"`
	Provider      string `json:"provider" gorm:"size:100;not null;index"`
	ModelVersion  string `json:"model_version" gorm:"size:100"`
	
	// 调用信息
	RequestType   string `json:"request_type" gorm:"size:50;not null"` // chat, embedding, completion
	PromptTokens  int    `json:"prompt_tokens" gorm:"not null"`
	CompletionTokens int `json:"completion_tokens" gorm:"not null"`
	TotalTokens   int    `json:"total_tokens" gorm:"not null;index"`
	
	// 成本信息（单位：美元）
	PromptCost      float64 `json:"prompt_cost" gorm:"type:decimal(10,6);not null"`
	CompletionCost  float64 `json:"completion_cost" gorm:"type:decimal(10,6);not null"`
	TotalCost       float64 `json:"total_cost" gorm:"type:decimal(10,6);not null;index"`
	
	// 性能信息（毫秒）
	ResponseTimeMs int `json:"response_time_ms" gorm:"not null"`
	FirstTokenMs   int `json:"first_token_ms"`   // 首token延迟
	
	// 状态
	Status        string `json:"status" gorm:"size:50;not null;default:success"` // success, error, timeout
	ErrorMessage  string `json:"error_message" gorm:"type:text"`
	ErrorCode     string `json:"error_code" gorm:"size:100"`
	
	// 请求上下文
	AgentID       string         `json:"agent_id" gorm:"type:uuid;index"`
	WorkflowID    string         `json:"workflow_id" gorm:"type:uuid;index"`
	Temperature   float64        `json:"temperature" gorm:"type:decimal(3,2)"`
	MaxTokens     int            `json:"max_tokens"`
	ExtraMetadata map[string]any `json:"extra_metadata" gorm:"type:jsonb;serializer:json"`
	
	// 时间戳
	CreatedAt time.Time `json:"created_at" gorm:"not null;autoCreateTime;index"`
}

// TableName 指定表名
func (ModelCallLog) TableName() string {
	return "model_call_logs"
}

// ============================================================================
// 工作流执行记录
// ============================================================================

// WorkflowExecutionLog 工作流执行日志
type WorkflowExecutionLog struct {
	ID         string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID   string    `json:"tenant_id" gorm:"type:uuid;not null;index"`
	UserID     string    `json:"user_id" gorm:"type:uuid;index"`
	WorkflowID string    `json:"workflow_id" gorm:"type:uuid;not null;index"`
	
	// 执行信息
	ExecutionID   string `json:"execution_id" gorm:"type:uuid;not null;uniqueIndex"`
	WorkflowName  string `json:"workflow_name" gorm:"size:255;not null"`
	Status        string `json:"status" gorm:"size:50;not null;index"` // running, success, failed, timeout
	
	// 步骤统计
	TotalSteps      int `json:"total_steps" gorm:"not null"`
	CompletedSteps  int `json:"completed_steps" gorm:"not null"`
	FailedSteps     int `json:"failed_steps" gorm:"not null"`
	
	// 资源消耗
	TotalTokens     int     `json:"total_tokens" gorm:"not null"`
	TotalCost       float64 `json:"total_cost" gorm:"type:decimal(10,6);not null"`
	ModelCallCount  int     `json:"model_call_count" gorm:"not null"` // AI模型调用次数
	
	// 性能指标（毫秒）
	ExecutionTimeMs int `json:"execution_time_ms" gorm:"not null"`
	
	// 错误信息
	ErrorMessage string `json:"error_message" gorm:"type:text"`
	ErrorStep    string `json:"error_step" gorm:"size:255"`
	
	// 时间戳
	StartedAt   time.Time  `json:"started_at" gorm:"not null;index"`
	CompletedAt *time.Time `json:"completed_at" gorm:"index"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null;autoCreateTime"`
}

// TableName 指定表名
func (WorkflowExecutionLog) TableName() string {
	return "workflow_execution_logs"
}

// ============================================================================
// 聚合统计数据
// ============================================================================

// ModelUsageStats 模型使用统计
type ModelUsageStats struct {
	ModelID        string    `json:"model_id"`
	ModelName      string    `json:"model_name"`
	Provider       string    `json:"provider"`
	CallCount      int64     `json:"call_count"`      // 调用次数
	TotalTokens    int64     `json:"total_tokens"`    // 总Token数
	TotalCost      float64   `json:"total_cost"`      // 总成本
	AvgResponseTime float64  `json:"avg_response_time"` // 平均响应时间(ms)
	SuccessRate    float64   `json:"success_rate"`    // 成功率
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
}

// TenantUsageStats 租户使用统计
type TenantUsageStats struct {
	TenantID        string    `json:"tenant_id"`
	PeriodStart     time.Time `json:"period_start"`
	PeriodEnd       time.Time `json:"period_end"`
	
	// 模型调用统计
	TotalModelCalls int64   `json:"total_model_calls"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalCost       float64 `json:"total_cost"`
	
	// 工作流统计
	TotalWorkflows      int64   `json:"total_workflows"`
	SuccessfulWorkflows int64   `json:"successful_workflows"`
	FailedWorkflows     int64   `json:"failed_workflows"`
	
	// 性能指标
	AvgModelResponseTime    float64 `json:"avg_model_response_time"`    // ms
	AvgWorkflowExecutionTime float64 `json:"avg_workflow_execution_time"` // ms
	
	// 成本分析
	TopModels []ModelCostBreakdown `json:"top_models"` // 成本占比最高的模型
}

// ModelCostBreakdown 模型成本分解
type ModelCostBreakdown struct {
	ModelName   string  `json:"model_name"`
	Provider    string  `json:"provider"`
	CallCount   int64   `json:"call_count"`
	TotalCost   float64 `json:"total_cost"`
	Percentage  float64 `json:"percentage"` // 成本占比
}

// CostAnalysis 成本分析
type CostAnalysis struct {
	TenantID    string    `json:"tenant_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	
	// 总成本
	TotalCost      float64 `json:"total_cost"`
	DailyCost      float64 `json:"daily_cost"`      // 日均成本
	ProjectedMonthlyCost float64 `json:"projected_monthly_cost"` // 预估月成本
	
	// 成本趋势
	CostTrend []DailyCostPoint `json:"cost_trend"`
	
	// 成本分解
	ByModel    []ModelCostBreakdown `json:"by_model"`
	ByProvider []ProviderCostBreakdown `json:"by_provider"`
	ByAgent    []AgentCostBreakdown `json:"by_agent"`
}

// DailyCostPoint 每日成本数据点
type DailyCostPoint struct {
	Date      string  `json:"date"`
	Cost      float64 `json:"cost"`
	CallCount int64   `json:"call_count"`
	Tokens    int64   `json:"tokens"`
}

// ProviderCostBreakdown 提供商成本分解
type ProviderCostBreakdown struct {
	Provider   string  `json:"provider"`
	TotalCost  float64 `json:"total_cost"`
	Percentage float64 `json:"percentage"`
}

// AgentCostBreakdown Agent成本分解
type AgentCostBreakdown struct {
	AgentID    string  `json:"agent_id"`
	AgentName  string  `json:"agent_name"`
	CallCount  int64   `json:"call_count"`
	TotalCost  float64 `json:"total_cost"`
	Percentage float64 `json:"percentage"`
}

// ============================================================================
// 时间范围枚举
// ============================================================================

// TimeRange 时间范围
type TimeRange string

const (
	TimeRangeToday     TimeRange = "today"
	TimeRangeYesterday TimeRange = "yesterday"
	TimeRangeLast7Days TimeRange = "last_7_days"
	TimeRangeLast30Days TimeRange = "last_30_days"
	TimeRangeThisMonth TimeRange = "this_month"
	TimeRangeLastMonth TimeRange = "last_month"
	TimeRangeCustom    TimeRange = "custom"
)
