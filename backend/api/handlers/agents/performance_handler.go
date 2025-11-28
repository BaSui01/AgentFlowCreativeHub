package agents

import (
	"net/http"
	"strconv"
	"time"

	"backend/api/handlers/common"
	"backend/internal/agent"

	"github.com/gin-gonic/gin"
)

// PerformanceHandler Agent性能分析处理器
type PerformanceHandler struct {
	service *agent.PerformanceService
}

// NewPerformanceHandler 创建性能分析处理器
func NewPerformanceHandler(service *agent.PerformanceService) *PerformanceHandler {
	return &PerformanceHandler{service: service}
}

// GetAgentStats 获取Agent性能统计
// @Summary 获取Agent性能统计
// @Tags Agent Performance
// @Security BearerAuth
// @Produce json
// @Param agent_id query string false "Agent ID"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param days query int false "最近天数 (默认7)"
// @Success 200 {object} common.APIResponse{data=agent.AgentPerformanceStats}
// @Router /api/agents/performance/stats [get]
func (h *PerformanceHandler) GetAgentStats(c *gin.Context) {
	query := h.parseQuery(c)

	stats, err := h.service.GetAgentStats(c.Request.Context(), query)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, stats)
}

// GetDailyTrend 获取每日趋势
// @Summary 获取Agent每日执行趋势
// @Tags Agent Performance
// @Security BearerAuth
// @Produce json
// @Param agent_id query string false "Agent ID"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param days query int false "最近天数 (默认7)"
// @Success 200 {object} common.APIResponse{data=[]agent.AgentDailyStats}
// @Router /api/agents/performance/trend [get]
func (h *PerformanceHandler) GetDailyTrend(c *gin.Context) {
	query := h.parseQuery(c)

	trend, err := h.service.GetDailyTrend(c.Request.Context(), query)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, trend)
}

// GetAgentComparison 获取Agent对比
// @Summary 获取多Agent性能对比
// @Tags Agent Performance
// @Security BearerAuth
// @Produce json
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param days query int false "最近天数 (默认7)"
// @Success 200 {object} common.APIResponse{data=[]agent.AgentComparisonStats}
// @Router /api/agents/performance/comparison [get]
func (h *PerformanceHandler) GetAgentComparison(c *gin.Context) {
	query := h.parseQuery(c)

	comparison, err := h.service.GetAgentComparison(c.Request.Context(), query)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, comparison)
}

// GetTopAgents 获取Top Agent
// @Summary 获取调用量最多的Agent
// @Tags Agent Performance
// @Security BearerAuth
// @Produce json
// @Param limit query int false "返回数量 (默认10)"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param days query int false "最近天数 (默认7)"
// @Success 200 {object} common.APIResponse{data=[]agent.AgentComparisonStats}
// @Router /api/agents/performance/top [get]
func (h *PerformanceHandler) GetTopAgents(c *gin.Context) {
	query := h.parseQuery(c)
	limit := 10
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "10")); err == nil && l > 0 {
		limit = l
	}

	topAgents, err := h.service.GetTopAgents(c.Request.Context(), query, limit)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, topAgents)
}

// GetSlowestRuns 获取最慢执行
// @Summary 获取最慢的执行记录
// @Tags Agent Performance
// @Security BearerAuth
// @Produce json
// @Param agent_id query string false "Agent ID"
// @Param limit query int false "返回数量 (默认10)"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Success 200 {object} common.APIResponse
// @Router /api/agents/performance/slowest [get]
func (h *PerformanceHandler) GetSlowestRuns(c *gin.Context) {
	query := h.parseQuery(c)
	limit := 10
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "10")); err == nil && l > 0 {
		limit = l
	}

	runs, err := h.service.GetSlowestRuns(c.Request.Context(), query, limit)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, runs)
}

// GetFailedRuns 获取失败执行
// @Summary 获取失败的执行记录
// @Tags Agent Performance
// @Security BearerAuth
// @Produce json
// @Param agent_id query string false "Agent ID"
// @Param limit query int false "返回数量 (默认10)"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Success 200 {object} common.APIResponse
// @Router /api/agents/performance/failed [get]
func (h *PerformanceHandler) GetFailedRuns(c *gin.Context) {
	query := h.parseQuery(c)
	limit := 10
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "10")); err == nil && l > 0 {
		limit = l
	}

	runs, err := h.service.GetFailedRuns(c.Request.Context(), query, limit)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, runs)
}

// GetTokenUsageTrend 获取Token使用趋势
// @Summary 获取Token使用趋势
// @Tags Agent Performance
// @Security BearerAuth
// @Produce json
// @Param agent_id query string false "Agent ID"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param days query int false "最近天数 (默认7)"
// @Success 200 {object} common.APIResponse
// @Router /api/agents/performance/tokens [get]
func (h *PerformanceHandler) GetTokenUsageTrend(c *gin.Context) {
	query := h.parseQuery(c)

	trend, err := h.service.GetTokenUsageTrend(c.Request.Context(), query)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, trend)
}

// GetPerformanceSummary 获取性能概览
// @Summary 获取Agent性能概览
// @Tags Agent Performance
// @Security BearerAuth
// @Produce json
// @Param agent_id query string false "Agent ID"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param days query int false "最近天数 (默认7)"
// @Success 200 {object} common.APIResponse{data=agent.PerformanceSummary}
// @Router /api/agents/performance/summary [get]
func (h *PerformanceHandler) GetPerformanceSummary(c *gin.Context) {
	query := h.parseQuery(c)

	summary, err := h.service.GetPerformanceSummary(c.Request.Context(), query)
	if err != nil {
		common.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	common.Success(c, summary)
}

// parseQuery 解析查询参数
func (h *PerformanceHandler) parseQuery(c *gin.Context) *agent.PerformanceQuery {
	query := &agent.PerformanceQuery{
		TenantID:  c.GetString("tenant_id"),
		AgentID:   c.Query("agent_id"),
		AgentType: c.Query("agent_type"),
	}

	// 解析时间范围
	if startStr := c.Query("start_time"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			query.StartTime = t
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			query.EndTime = t
		}
	}

	// 如果未指定时间范围，默认最近7天
	if query.StartTime.IsZero() || query.EndTime.IsZero() {
		days := 7
		if d, err := strconv.Atoi(c.DefaultQuery("days", "7")); err == nil && d > 0 {
			days = d
		}
		query.EndTime = time.Now()
		query.StartTime = query.EndTime.AddDate(0, 0, -days)
	}

	return query
}
