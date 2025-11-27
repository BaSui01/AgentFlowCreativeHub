package metrics

import (
	"context"
	"fmt"
	"time"

	"backend/internal/common"

	"gorm.io/gorm"
)

// ============================================================================
// MetricsService 接口定义
// ============================================================================

// MetricsServiceInterface 指标统计服务接口
type MetricsServiceInterface interface {
	// RecordModelCall 记录AI模型调用
	RecordModelCall(ctx context.Context, log *ModelCallLog) error

	// RecordWorkflowExecution 记录工作流执行
	RecordWorkflowExecution(ctx context.Context, log *WorkflowExecutionLog) error

	// GetModelStats 获取模型统计信息
	GetModelStats(ctx context.Context, tenantID string, timeRange TimeRange, startTime, endTime *time.Time) ([]*ModelUsageStats, error)

	// GetTenantUsageStats 获取租户使用统计
	GetTenantUsageStats(ctx context.Context, tenantID string, timeRange TimeRange, startTime, endTime *time.Time) (*TenantUsageStats, error)

	// GetCostAnalysis 获取成本分析
	GetCostAnalysis(ctx context.Context, tenantID string, timeRange TimeRange, startTime, endTime *time.Time) (*CostAnalysis, error)

	// GetModelCallLogs 查询模型调用日志
	GetModelCallLogs(ctx context.Context, req *ModelCallLogsRequest) ([]*ModelCallLog, int64, error)

	// GetWorkflowExecutionLogs 查询工作流执行日志
	GetWorkflowExecutionLogs(ctx context.Context, req *WorkflowExecutionLogsRequest) ([]*WorkflowExecutionLog, int64, error)

	// GetDailyCostTrend 获取每日成本趋势
	GetDailyCostTrend(ctx context.Context, tenantID string, days int) ([]DailyCostPoint, error)

	// GetTopCostModels 获取成本最高的模型排行
	GetTopCostModels(ctx context.Context, tenantID string, timeRange TimeRange, limit int) ([]ModelCostBreakdown, error)
}

// ============================================================================
// 请求/响应类型
// ============================================================================

// ModelCallLogsRequest 模型调用日志查询请求
type ModelCallLogsRequest struct {
	common.PaginationRequest
	TenantID   string
	UserID     string
	ModelID    string
	Provider   string
	Status     string
	StartTime  *time.Time
	EndTime    *time.Time
	AgentID    string
	WorkflowID string
}

// WorkflowExecutionLogsRequest 工作流执行日志查询请求
type WorkflowExecutionLogsRequest struct {
	common.PaginationRequest
	TenantID   string
	UserID     string
	WorkflowID string
	Status     string
	StartTime  *time.Time
	EndTime    *time.Time
}

// ============================================================================
// 服务实现
// ============================================================================

type metricsService struct {
	*common.BaseService
}

// NewMetricsService 创建MetricsService实例
func NewMetricsService(db *gorm.DB) MetricsServiceInterface {
	return &metricsService{
		BaseService: common.NewBaseService(db),
	}
}

// RecordModelCall 记录AI模型调用
func (s *metricsService) RecordModelCall(ctx context.Context, log *ModelCallLog) error {
	if log.ID == "" {
		return fmt.Errorf("log ID is required")
	}

	// 计算总Token和总成本
	log.TotalTokens = log.PromptTokens + log.CompletionTokens
	log.TotalCost = log.PromptCost + log.CompletionCost

	if err := s.Create(ctx, log); err != nil {
		return fmt.Errorf("failed to record model call: %w", err)
	}

	return nil
}

// RecordWorkflowExecution 记录工作流执行
func (s *metricsService) RecordWorkflowExecution(ctx context.Context, log *WorkflowExecutionLog) error {
	if log.ID == "" {
		return fmt.Errorf("log ID is required")
	}

	if err := s.Create(ctx, log); err != nil {
		return fmt.Errorf("failed to record workflow execution: %w", err)
	}

	return nil
}

// GetModelStats 获取模型统计信息
func (s *metricsService) GetModelStats(ctx context.Context, tenantID string, timeRange TimeRange, startTime, endTime *time.Time) ([]*ModelUsageStats, error) {
	start, end := s.parseTimeRange(timeRange, startTime, endTime)

	var results []*ModelUsageStats

	query := s.GetDBWithContext(ctx).
		Model(&ModelCallLog{}).
		Select(`
			model_id,
			model_name,
			provider,
			COUNT(*) as call_count,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost,
			AVG(response_time_ms) as avg_response_time,
			(COUNT(CASE WHEN status = 'success' THEN 1 END) * 100.0 / COUNT(*)) as success_rate
		`).
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("model_id, model_name, provider").
		Order("total_cost DESC")

	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get model stats: %w", err)
	}

	// 设置时间范围
	for _, stat := range results {
		stat.PeriodStart = start
		stat.PeriodEnd = end
	}

	return results, nil
}

// GetTenantUsageStats 获取租户使用统计
func (s *metricsService) GetTenantUsageStats(ctx context.Context, tenantID string, timeRange TimeRange, startTime, endTime *time.Time) (*TenantUsageStats, error) {
	start, end := s.parseTimeRange(timeRange, startTime, endTime)

	stats := &TenantUsageStats{
		TenantID:    tenantID,
		PeriodStart: start,
		PeriodEnd:   end,
	}

	// 查询模型调用统计
	var modelStats struct {
		TotalCalls          int64
		TotalTokens         int64
		TotalCost           float64
		AvgResponseTime     float64
	}

	if err := s.GetDBWithContext(ctx).
		Model(&ModelCallLog{}).
		Select(`
			COUNT(*) as total_calls,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost,
			AVG(response_time_ms) as avg_response_time
		`).
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Scan(&modelStats).Error; err != nil {
		return nil, fmt.Errorf("failed to get model stats: %w", err)
	}

	stats.TotalModelCalls = modelStats.TotalCalls
	stats.TotalTokens = modelStats.TotalTokens
	stats.TotalCost = modelStats.TotalCost
	stats.AvgModelResponseTime = modelStats.AvgResponseTime

	// 查询工作流统计
	var workflowStats struct {
		TotalWorkflows      int64
		SuccessfulWorkflows int64
		FailedWorkflows     int64
		AvgExecutionTime    float64
	}

	if err := s.GetDBWithContext(ctx).
		Model(&WorkflowExecutionLog{}).
		Select(`
			COUNT(*) as total_workflows,
			COUNT(CASE WHEN status = 'success' THEN 1 END) as successful_workflows,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_workflows,
			AVG(execution_time_ms) as avg_execution_time
		`).
		Where("tenant_id = ?", tenantID).
		Where("started_at BETWEEN ? AND ?", start, end).
		Scan(&workflowStats).Error; err != nil {
		return nil, fmt.Errorf("failed to get workflow stats: %w", err)
	}

	stats.TotalWorkflows = workflowStats.TotalWorkflows
	stats.SuccessfulWorkflows = workflowStats.SuccessfulWorkflows
	stats.FailedWorkflows = workflowStats.FailedWorkflows
	stats.AvgWorkflowExecutionTime = workflowStats.AvgExecutionTime

	// 获取Top成本模型
	topModels, err := s.GetTopCostModels(ctx, tenantID, timeRange, 5)
	if err == nil {
		stats.TopModels = topModels
	}

	return stats, nil
}

// GetCostAnalysis 获取成本分析
func (s *metricsService) GetCostAnalysis(ctx context.Context, tenantID string, timeRange TimeRange, startTime, endTime *time.Time) (*CostAnalysis, error) {
	start, end := s.parseTimeRange(timeRange, startTime, endTime)

	analysis := &CostAnalysis{
		TenantID:    tenantID,
		PeriodStart: start,
		PeriodEnd:   end,
	}

	// 查询总成本
	var totalCost float64
	if err := s.GetDBWithContext(ctx).
		Model(&ModelCallLog{}).
		Select("COALESCE(SUM(total_cost), 0)").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Scan(&totalCost).Error; err != nil {
		return nil, fmt.Errorf("failed to get total cost: %w", err)
	}

	analysis.TotalCost = totalCost

	// 计算日均成本
	days := int(end.Sub(start).Hours() / 24)
	if days > 0 {
		analysis.DailyCost = totalCost / float64(days)
		analysis.ProjectedMonthlyCost = analysis.DailyCost * 30
	}

	// 获取每日成本趋势
	costTrend, err := s.GetDailyCostTrend(ctx, tenantID, days)
	if err == nil {
		analysis.CostTrend = costTrend
	}

	// 按模型分解成本
	byModel, err := s.GetTopCostModels(ctx, tenantID, timeRange, 10)
	if err == nil {
		analysis.ByModel = byModel
	}

	// 按提供商分解成本
	byProvider, err := s.getCostByProvider(ctx, tenantID, start, end, totalCost)
	if err == nil {
		analysis.ByProvider = byProvider
	}

	// 按Agent分解成本
	byAgent, err := s.getCostByAgent(ctx, tenantID, start, end, totalCost)
	if err == nil {
		analysis.ByAgent = byAgent
	}

	return analysis, nil
}

// GetModelCallLogs 查询模型调用日志
func (s *metricsService) GetModelCallLogs(ctx context.Context, req *ModelCallLogsRequest) ([]*ModelCallLog, int64, error) {
	query := s.GetDBWithContext(ctx).Model(&ModelCallLog{})

	// 租户过滤
	query = s.ApplyTenantFilter(query, req.TenantID)

	// 其他过滤条件
	if req.UserID != "" {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.ModelID != "" {
		query = query.Where("model_id = ?", req.ModelID)
	}
	if req.Provider != "" {
		query = query.Where("provider = ?", req.Provider)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.AgentID != "" {
		query = query.Where("agent_id = ?", req.AgentID)
	}
	if req.WorkflowID != "" {
		query = query.Where("workflow_id = ?", req.WorkflowID)
	}

	// 时间范围
	if req.StartTime != nil {
		query = query.Where("created_at >= ?", req.StartTime)
	}
	if req.EndTime != nil {
		query = query.Where("created_at <= ?", req.EndTime)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	// 分页查询
	var logs []*ModelCallLog
	query = s.ApplyPaginationRequest(query, req.PaginationRequest)
	query = query.Order("created_at DESC")

	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get logs: %w", err)
	}

	return logs, total, nil
}

// GetWorkflowExecutionLogs 查询工作流执行日志
func (s *metricsService) GetWorkflowExecutionLogs(ctx context.Context, req *WorkflowExecutionLogsRequest) ([]*WorkflowExecutionLog, int64, error) {
	query := s.GetDBWithContext(ctx).Model(&WorkflowExecutionLog{})

	// 租户过滤
	query = s.ApplyTenantFilter(query, req.TenantID)

	// 其他过滤条件
	if req.UserID != "" {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.WorkflowID != "" {
		query = query.Where("workflow_id = ?", req.WorkflowID)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	// 时间范围
	if req.StartTime != nil {
		query = query.Where("started_at >= ?", req.StartTime)
	}
	if req.EndTime != nil {
		query = query.Where("started_at <= ?", req.EndTime)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	// 分页查询
	var logs []*WorkflowExecutionLog
	query = s.ApplyPaginationRequest(query, req.PaginationRequest)
	query = query.Order("started_at DESC")

	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get logs: %w", err)
	}

	return logs, total, nil
}

// GetDailyCostTrend 获取每日成本趋势
func (s *metricsService) GetDailyCostTrend(ctx context.Context, tenantID string, days int) ([]DailyCostPoint, error) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	var results []struct {
		Date      string
		Cost      float64
		CallCount int64
		Tokens    int64
	}

	if err := s.GetDBWithContext(ctx).
		Model(&ModelCallLog{}).
		Select(`
			DATE(created_at) as date,
			SUM(total_cost) as cost,
			COUNT(*) as call_count,
			SUM(total_tokens) as tokens
		`).
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get daily cost trend: %w", err)
	}

	// 转换为DailyCostPoint
	points := make([]DailyCostPoint, len(results))
	for i, r := range results {
		points[i] = DailyCostPoint{
			Date:      r.Date,
			Cost:      r.Cost,
			CallCount: r.CallCount,
			Tokens:    r.Tokens,
		}
	}

	return points, nil
}

// GetTopCostModels 获取成本最高的模型排行
func (s *metricsService) GetTopCostModels(ctx context.Context, tenantID string, timeRange TimeRange, limit int) ([]ModelCostBreakdown, error) {
	start, end := s.parseTimeRange(timeRange, nil, nil)

	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	// 先获取总成本
	var totalCost float64
	if err := s.GetDBWithContext(ctx).
		Model(&ModelCallLog{}).
		Select("COALESCE(SUM(total_cost), 0)").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Scan(&totalCost).Error; err != nil {
		return nil, fmt.Errorf("failed to get total cost: %w", err)
	}

	var results []struct {
		ModelName string
		Provider  string
		CallCount int64
		TotalCost float64
	}

	if err := s.GetDBWithContext(ctx).
		Model(&ModelCallLog{}).
		Select(`
			model_name,
			provider,
			COUNT(*) as call_count,
			SUM(total_cost) as total_cost
		`).
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("model_name, provider").
		Order("total_cost DESC").
		Limit(limit).
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get top cost models: %w", err)
	}

	// 转换并计算百分比
	breakdowns := make([]ModelCostBreakdown, len(results))
	for i, r := range results {
		percentage := 0.0
		if totalCost > 0 {
			percentage = (r.TotalCost / totalCost) * 100
		}

		breakdowns[i] = ModelCostBreakdown{
			ModelName:  r.ModelName,
			Provider:   r.Provider,
			CallCount:  r.CallCount,
			TotalCost:  r.TotalCost,
			Percentage: percentage,
		}
	}

	return breakdowns, nil
}

// getCostByProvider 按提供商分解成本
func (s *metricsService) getCostByProvider(ctx context.Context, tenantID string, start, end time.Time, totalCost float64) ([]ProviderCostBreakdown, error) {
	var results []struct {
		Provider  string
		TotalCost float64
	}

	if err := s.GetDBWithContext(ctx).
		Model(&ModelCallLog{}).
		Select("provider, SUM(total_cost) as total_cost").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("provider").
		Order("total_cost DESC").
		Scan(&results).Error; err != nil {
		return nil, err
	}

	breakdowns := make([]ProviderCostBreakdown, len(results))
	for i, r := range results {
		percentage := 0.0
		if totalCost > 0 {
			percentage = (r.TotalCost / totalCost) * 100
		}

		breakdowns[i] = ProviderCostBreakdown{
			Provider:   r.Provider,
			TotalCost:  r.TotalCost,
			Percentage: percentage,
		}
	}

	return breakdowns, nil
}

// getCostByAgent 按Agent分解成本
func (s *metricsService) getCostByAgent(ctx context.Context, tenantID string, start, end time.Time, totalCost float64) ([]AgentCostBreakdown, error) {
	var results []struct {
		AgentID   string
		CallCount int64
		TotalCost float64
	}

	if err := s.GetDBWithContext(ctx).
		Model(&ModelCallLog{}).
		Select("agent_id, COUNT(*) as call_count, SUM(total_cost) as total_cost").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Where("agent_id IS NOT NULL AND agent_id != ''").
		Group("agent_id").
		Order("total_cost DESC").
		Limit(10).
		Scan(&results).Error; err != nil {
		return nil, err
	}

	breakdowns := make([]AgentCostBreakdown, len(results))
	for i, r := range results {
		percentage := 0.0
		if totalCost > 0 {
			percentage = (r.TotalCost / totalCost) * 100
		}

		breakdowns[i] = AgentCostBreakdown{
			AgentID:    r.AgentID,
			AgentName:  r.AgentID, // TODO: 关联Agent表获取名称
			CallCount:  r.CallCount,
			TotalCost:  r.TotalCost,
			Percentage: percentage,
		}
	}

	return breakdowns, nil
}

// parseTimeRange 解析时间范围
func (s *metricsService) parseTimeRange(timeRange TimeRange, startTime, endTime *time.Time) (time.Time, time.Time) {
	now := time.Now()

	// 自定义时间范围
	if timeRange == TimeRangeCustom && startTime != nil && endTime != nil {
		return *startTime, *endTime
	}

	var start, end time.Time

	switch timeRange {
	case TimeRangeToday:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = now

	case TimeRangeYesterday:
		yesterday := now.AddDate(0, 0, -1)
		start = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		end = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999999999, now.Location())

	case TimeRangeLast7Days:
		start = now.AddDate(0, 0, -7)
		end = now

	case TimeRangeLast30Days:
		start = now.AddDate(0, 0, -30)
		end = now

	case TimeRangeThisMonth:
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = now

	case TimeRangeLastMonth:
		lastMonth := now.AddDate(0, -1, 0)
		start = time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)

	default:
		// 默认最近7天
		start = now.AddDate(0, 0, -7)
		end = now
	}

	return start, end
}
