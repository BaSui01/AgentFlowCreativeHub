package agent

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// agentRunModel 数据库模型（避免循环引用）
type agentRunModel struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)"`
	AgentID     string    `gorm:"index;type:varchar(64)"`
	TraceID     string    `gorm:"index;type:varchar(64)"`
	Status      string    `gorm:"type:varchar(20)"`
	Input       string    `gorm:"type:text"`
	Output      string    `gorm:"type:text"`
	Error       string    `gorm:"type:text"`
	TotalTokens int       `gorm:"type:int"`
	LatencyMs   int64     `gorm:"type:bigint"`
	CreatedAt   time.Time `gorm:"index"`
	FinishedAt  *time.Time
}

func (agentRunModel) TableName() string {
	return "agent_run_models"
}

// PerformanceService Agent性能分析服务
type PerformanceService struct {
	db *gorm.DB
}

// NewPerformanceService 创建性能分析服务
func NewPerformanceService(db *gorm.DB) *PerformanceService {
	return &PerformanceService{db: db}
}

// AgentPerformanceStats Agent性能统计
type AgentPerformanceStats struct {
	AgentID         string  `json:"agent_id"`
	AgentType       string  `json:"agent_type"`
	TotalRuns       int64   `json:"total_runs"`
	SuccessRuns     int64   `json:"success_runs"`
	FailedRuns      int64   `json:"failed_runs"`
	SuccessRate     float64 `json:"success_rate"`
	TotalTokens     int64   `json:"total_tokens"`
	AvgTokensPerRun float64 `json:"avg_tokens_per_run"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	P50LatencyMs    float64 `json:"p50_latency_ms"`
	P95LatencyMs    float64 `json:"p95_latency_ms"`
	P99LatencyMs    float64 `json:"p99_latency_ms"`
	MinLatencyMs    int64   `json:"min_latency_ms"`
	MaxLatencyMs    int64   `json:"max_latency_ms"`
}

// AgentDailyStats Agent每日统计
type AgentDailyStats struct {
	Date         string  `json:"date"`
	TotalRuns    int64   `json:"total_runs"`
	SuccessRuns  int64   `json:"success_runs"`
	FailedRuns   int64   `json:"failed_runs"`
	TotalTokens  int64   `json:"total_tokens"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// AgentComparisonStats Agent对比统计
type AgentComparisonStats struct {
	AgentID      string  `json:"agent_id"`
	AgentType    string  `json:"agent_type"`
	TotalRuns    int64   `json:"total_runs"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	TotalTokens  int64   `json:"total_tokens"`
	AvgCost      float64 `json:"avg_cost"`
}

// PerformanceQuery 性能查询参数
type PerformanceQuery struct {
	TenantID  string
	AgentID   string
	AgentType string
	StartTime time.Time
	EndTime   time.Time
}

// GetAgentStats 获取单个Agent的性能统计
func (s *PerformanceService) GetAgentStats(ctx context.Context, query *PerformanceQuery) (*AgentPerformanceStats, error) {
	var stats AgentPerformanceStats

	baseQuery := s.db.WithContext(ctx).Model(&agentRunModel{}).
		Where("created_at BETWEEN ? AND ?", query.StartTime, query.EndTime)

	if query.AgentID != "" {
		baseQuery = baseQuery.Where("agent_id = ?", query.AgentID)
	}

	// 基础统计
	var result struct {
		TotalRuns    int64
		SuccessRuns  int64
		FailedRuns   int64
		TotalTokens  int64
		AvgLatencyMs float64
		MinLatencyMs int64
		MaxLatencyMs int64
	}

	err := baseQuery.Select(`
		COUNT(*) as total_runs,
		SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as success_runs,
		SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_runs,
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
		COALESCE(MIN(latency_ms), 0) as min_latency_ms,
		COALESCE(MAX(latency_ms), 0) as max_latency_ms
	`).Scan(&result).Error
	if err != nil {
		return nil, err
	}

	stats.AgentID = query.AgentID
	stats.TotalRuns = result.TotalRuns
	stats.SuccessRuns = result.SuccessRuns
	stats.FailedRuns = result.FailedRuns
	stats.TotalTokens = result.TotalTokens
	stats.AvgLatencyMs = result.AvgLatencyMs
	stats.MinLatencyMs = result.MinLatencyMs
	stats.MaxLatencyMs = result.MaxLatencyMs

	if stats.TotalRuns > 0 {
		stats.SuccessRate = float64(stats.SuccessRuns) / float64(stats.TotalRuns) * 100
		stats.AvgTokensPerRun = float64(stats.TotalTokens) / float64(stats.TotalRuns)
	}

	// 计算百分位延迟
	stats.P50LatencyMs = s.calculatePercentile(ctx, query, 50)
	stats.P95LatencyMs = s.calculatePercentile(ctx, query, 95)
	stats.P99LatencyMs = s.calculatePercentile(ctx, query, 99)

	return &stats, nil
}

// calculatePercentile 计算延迟百分位数
func (s *PerformanceService) calculatePercentile(ctx context.Context, query *PerformanceQuery, percentile int) float64 {
	var result float64

	subQuery := s.db.WithContext(ctx).Model(&agentRunModel{}).
		Select("latency_ms").
		Where("created_at BETWEEN ? AND ?", query.StartTime, query.EndTime).
		Where("status = 'completed'")

	if query.AgentID != "" {
		subQuery = subQuery.Where("agent_id = ?", query.AgentID)
	}

	// 使用 PostgreSQL 的 percentile_cont 函数
	s.db.WithContext(ctx).Raw(`
		SELECT COALESCE(percentile_cont(?) WITHIN GROUP (ORDER BY latency_ms), 0)
		FROM (?) as subq
	`, float64(percentile)/100, subQuery).Scan(&result)

	return result
}

// GetDailyTrend 获取每日趋势
func (s *PerformanceService) GetDailyTrend(ctx context.Context, query *PerformanceQuery) ([]AgentDailyStats, error) {
	var results []AgentDailyStats

	baseQuery := s.db.WithContext(ctx).Model(&agentRunModel{}).
		Where("created_at BETWEEN ? AND ?", query.StartTime, query.EndTime)

	if query.AgentID != "" {
		baseQuery = baseQuery.Where("agent_id = ?", query.AgentID)
	}

	err := baseQuery.Select(`
		TO_CHAR(created_at, 'YYYY-MM-DD') as date,
		COUNT(*) as total_runs,
		SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as success_runs,
		SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_runs,
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COALESCE(AVG(latency_ms), 0) as avg_latency_ms
	`).Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetAgentComparison 获取多Agent对比
func (s *PerformanceService) GetAgentComparison(ctx context.Context, query *PerformanceQuery) ([]AgentComparisonStats, error) {
	var results []AgentComparisonStats

	baseQuery := s.db.WithContext(ctx).Model(&agentRunModel{}).
		Where("created_at BETWEEN ? AND ?", query.StartTime, query.EndTime)

	err := baseQuery.Select(`
		agent_id,
		COUNT(*) as total_runs,
		CASE WHEN COUNT(*) > 0 THEN 
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)::float / COUNT(*) * 100 
		ELSE 0 END as success_rate,
		COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
		COALESCE(SUM(total_tokens), 0) as total_tokens
	`).Group("agent_id").
		Order("total_runs DESC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetTopAgents 获取调用量最多的Agent
func (s *PerformanceService) GetTopAgents(ctx context.Context, query *PerformanceQuery, limit int) ([]AgentComparisonStats, error) {
	var results []AgentComparisonStats

	baseQuery := s.db.WithContext(ctx).Model(&agentRunModel{}).
		Where("created_at BETWEEN ? AND ?", query.StartTime, query.EndTime)

	err := baseQuery.Select(`
		agent_id,
		COUNT(*) as total_runs,
		CASE WHEN COUNT(*) > 0 THEN 
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)::float / COUNT(*) * 100 
		ELSE 0 END as success_rate,
		COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
		COALESCE(SUM(total_tokens), 0) as total_tokens
	`).Group("agent_id").
		Order("total_runs DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetSlowestRuns 获取最慢的执行记录
func (s *PerformanceService) GetSlowestRuns(ctx context.Context, query *PerformanceQuery, limit int) ([]agentRunModel, error) {
	var results []agentRunModel

	baseQuery := s.db.WithContext(ctx).Model(&agentRunModel{}).
		Where("created_at BETWEEN ? AND ?", query.StartTime, query.EndTime).
		Where("status = 'completed'")

	if query.AgentID != "" {
		baseQuery = baseQuery.Where("agent_id = ?", query.AgentID)
	}

	err := baseQuery.Order("latency_ms DESC").
		Limit(limit).
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetFailedRuns 获取失败的执行记录
func (s *PerformanceService) GetFailedRuns(ctx context.Context, query *PerformanceQuery, limit int) ([]agentRunModel, error) {
	var results []agentRunModel

	baseQuery := s.db.WithContext(ctx).Model(&agentRunModel{}).
		Where("created_at BETWEEN ? AND ?", query.StartTime, query.EndTime).
		Where("status = 'failed'")

	if query.AgentID != "" {
		baseQuery = baseQuery.Where("agent_id = ?", query.AgentID)
	}

	err := baseQuery.Order("created_at DESC").
		Limit(limit).
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetTokenUsageTrend 获取Token使用趋势
func (s *PerformanceService) GetTokenUsageTrend(ctx context.Context, query *PerformanceQuery) ([]struct {
	Date        string `json:"date"`
	TotalTokens int64  `json:"total_tokens"`
	AvgTokens   int64  `json:"avg_tokens"`
}, error) {
	var results []struct {
		Date        string `json:"date"`
		TotalTokens int64  `json:"total_tokens"`
		AvgTokens   int64  `json:"avg_tokens"`
	}

	baseQuery := s.db.WithContext(ctx).Model(&agentRunModel{}).
		Where("created_at BETWEEN ? AND ?", query.StartTime, query.EndTime)

	if query.AgentID != "" {
		baseQuery = baseQuery.Where("agent_id = ?", query.AgentID)
	}

	err := baseQuery.Select(`
		TO_CHAR(created_at, 'YYYY-MM-DD') as date,
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COALESCE(AVG(total_tokens), 0)::bigint as avg_tokens
	`).Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

// PerformanceSummary 性能概览
type PerformanceSummary struct {
	TotalRuns       int64                  `json:"total_runs"`
	SuccessRate     float64                `json:"success_rate"`
	AvgLatencyMs    float64                `json:"avg_latency_ms"`
	TotalTokens     int64                  `json:"total_tokens"`
	TopAgents       []AgentComparisonStats `json:"top_agents"`
	DailyTrend      []AgentDailyStats      `json:"daily_trend"`
	RecentFailures  int64                  `json:"recent_failures"`
}

// GetPerformanceSummary 获取性能概览
func (s *PerformanceService) GetPerformanceSummary(ctx context.Context, query *PerformanceQuery) (*PerformanceSummary, error) {
	summary := &PerformanceSummary{}

	// 获取总体统计
	stats, err := s.GetAgentStats(ctx, query)
	if err != nil {
		return nil, err
	}
	summary.TotalRuns = stats.TotalRuns
	summary.SuccessRate = stats.SuccessRate
	summary.AvgLatencyMs = stats.AvgLatencyMs
	summary.TotalTokens = stats.TotalTokens

	// 获取Top Agent
	topAgents, err := s.GetTopAgents(ctx, query, 5)
	if err != nil {
		return nil, err
	}
	summary.TopAgents = topAgents

	// 获取每日趋势
	dailyTrend, err := s.GetDailyTrend(ctx, query)
	if err != nil {
		return nil, err
	}
	summary.DailyTrend = dailyTrend

	// 获取近期失败数
	summary.RecentFailures = stats.FailedRuns

	return summary, nil
}
