package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"backend/internal/credits"
	"backend/internal/metrics"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service 计费服务 - 整合 credits 和 metrics 功能
type Service struct {
	db             *gorm.DB
	creditsService *credits.Service
	metricsService metrics.MetricsServiceInterface
}

// NewService 创建计费服务
func NewService(db *gorm.DB, creditsService *credits.Service, metricsService metrics.MetricsServiceInterface) *Service {
	return &Service{
		db:             db,
		creditsService: creditsService,
		metricsService: metricsService,
	}
}

// ============================================================================
// 模型定价管理
// ============================================================================

// ListPricings 获取定价列表
func (s *Service) ListPricings(ctx context.Context, tenantID string, page, pageSize int) ([]ModelPricing, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	var pricings []credits.CreditPricing
	var total int64

	query := s.db.WithContext(ctx).Model(&credits.CreditPricing{}).
		Where("tenant_id = ?", tenantID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("provider, model").
		Limit(pageSize).
		Offset(offset).
		Find(&pricings).Error; err != nil {
		return nil, 0, err
	}

	// 转换为 ModelPricing
	result := make([]ModelPricing, len(pricings))
	for i, p := range pricings {
		result[i] = ModelPricing{
			ID:          p.ID,
			TenantID:    p.TenantID,
			Provider:    p.Provider,
			Model:       p.Model,
			InputPrice:  p.InputPrice,
			OutputPrice: p.OutputPrice,
			Currency:    "credits",
			IsActive:    p.IsActive,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		}
	}

	return result, total, nil
}

// GetPricing 获取单个定价
func (s *Service) GetPricing(ctx context.Context, tenantID, provider, model string) (*ModelPricing, error) {
	pricing, err := s.creditsService.GetPricing(ctx, tenantID, provider, model)
	if err != nil {
		return nil, err
	}

	return &ModelPricing{
		ID:          pricing.ID,
		TenantID:    pricing.TenantID,
		Provider:    pricing.Provider,
		Model:       pricing.Model,
		InputPrice:  pricing.InputPrice,
		OutputPrice: pricing.OutputPrice,
		Currency:    "credits",
		IsActive:    pricing.IsActive,
		CreatedAt:   pricing.CreatedAt,
		UpdatedAt:   pricing.UpdatedAt,
	}, nil
}

// CreatePricing 创建定价
func (s *Service) CreatePricing(ctx context.Context, req *CreatePricingRequest) (*ModelPricing, error) {
	pricing := credits.CreditPricing{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		Provider:    req.Provider,
		Model:       req.Model,
		InputPrice:  req.InputPrice,
		OutputPrice: req.OutputPrice,
		IsActive:    true,
	}

	if err := s.db.WithContext(ctx).Create(&pricing).Error; err != nil {
		return nil, fmt.Errorf("创建定价失败: %w", err)
	}

	return &ModelPricing{
		ID:          pricing.ID,
		TenantID:    pricing.TenantID,
		Provider:    pricing.Provider,
		Model:       pricing.Model,
		InputPrice:  pricing.InputPrice,
		OutputPrice: pricing.OutputPrice,
		Currency:    "credits",
		IsActive:    pricing.IsActive,
		Remark:      req.Remark,
		CreatedAt:   pricing.CreatedAt,
		UpdatedAt:   pricing.UpdatedAt,
	}, nil
}

// UpdatePricing 更新定价
func (s *Service) UpdatePricing(ctx context.Context, id string, req *UpdatePricingRequest) error {
	updates := make(map[string]interface{})
	if req.InputPrice != nil {
		updates["input_price"] = *req.InputPrice
	}
	if req.OutputPrice != nil {
		updates["output_price"] = *req.OutputPrice
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Model(&credits.CreditPricing{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeletePricing 删除定价
func (s *Service) DeletePricing(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&credits.CreditPricing{}, "id = ?", id).Error
}

// ============================================================================
// 成本预估
// ============================================================================

// EstimateCost 预估成本
func (s *Service) EstimateCost(ctx context.Context, req *CostEstimateRequest) (*CostEstimate, error) {
	pricing, err := s.creditsService.GetPricing(ctx, req.TenantID, req.Provider, req.Model)
	if err != nil {
		return nil, fmt.Errorf("获取定价失败: %w", err)
	}

	inputCost := float64(req.InputTokens) / 1000.0 * pricing.InputPrice
	outputCost := float64(req.OutputTokens) / 1000.0 * pricing.OutputPrice
	totalCost := int64(inputCost + outputCost + 0.5)
	if totalCost < 1 && (req.InputTokens > 0 || req.OutputTokens > 0) {
		totalCost = 1
	}

	return &CostEstimate{
		Provider:        req.Provider,
		Model:           req.Model,
		InputTokens:     req.InputTokens,
		OutputTokens:    req.OutputTokens,
		InputCost:       inputCost,
		OutputCost:      outputCost,
		TotalCost:       totalCost,
		PricePerKInput:  pricing.InputPrice,
		PricePerKOutput: pricing.OutputPrice,
		Currency:        "credits",
	}, nil
}

// ============================================================================
// 成本报表
// ============================================================================

// GenerateCostReport 生成成本报表
func (s *Service) GenerateCostReport(ctx context.Context, req *CostReportRequest) (*CostReport, error) {
	// 默认时间范围：最近30天
	now := time.Now()
	endDate := now
	startDate := now.AddDate(0, 0, -30)
	if req.StartDate != nil {
		startDate = *req.StartDate
	}
	if req.EndDate != nil {
		endDate = *req.EndDate
	}

	report := &CostReport{
		TenantID:    req.TenantID,
		StartDate:   startDate,
		EndDate:     endDate,
		GeneratedAt: now,
	}

	// 查询总体统计
	var summary struct {
		TotalCost   float64
		TotalCalls  int64
		TotalTokens int64
	}

	query := s.db.WithContext(ctx).Model(&metrics.ModelCallLog{}).
		Select("COALESCE(SUM(total_cost), 0) as total_cost, COUNT(*) as total_calls, COALESCE(SUM(total_tokens), 0) as total_tokens").
		Where("tenant_id = ?", req.TenantID).
		Where("created_at BETWEEN ? AND ?", startDate, endDate)

	if req.UserID != "" {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.ModelName != "" {
		query = query.Where("model_name = ?", req.ModelName)
	}

	if err := query.Scan(&summary).Error; err != nil {
		return nil, fmt.Errorf("查询统计失败: %w", err)
	}

	report.TotalCost = summary.TotalCost
	report.TotalCalls = summary.TotalCalls
	report.TotalTokens = summary.TotalTokens

	if summary.TotalCalls > 0 {
		report.AverageCostPerCall = summary.TotalCost / float64(summary.TotalCalls)
	}

	days := endDate.Sub(startDate).Hours() / 24
	if days > 0 {
		report.DailyCost = summary.TotalCost / days
		report.ProjectedMonthly = report.DailyCost * 30
	}

	// 按模型分组
	report.ByModel = s.getCostByModel(ctx, req.TenantID, startDate, endDate, summary.TotalCost)

	// 按提供商分组
	report.ByProvider = s.getCostByProvider(ctx, req.TenantID, startDate, endDate, summary.TotalCost)

	// 按用户分组
	report.ByUser = s.getCostByUser(ctx, req.TenantID, startDate, endDate, summary.TotalCost)

	// 每日趋势
	report.DailyTrend = s.getDailyTrend(ctx, req.TenantID, startDate, endDate)

	return report, nil
}

func (s *Service) getCostByModel(ctx context.Context, tenantID string, start, end time.Time, total float64) []ModelCostItem {
	var results []struct {
		Provider    string
		ModelName   string
		CallCount   int64
		TotalTokens int64
		TotalCost   float64
	}

	s.db.WithContext(ctx).Model(&metrics.ModelCallLog{}).
		Select("provider, model_name, COUNT(*) as call_count, SUM(total_tokens) as total_tokens, SUM(total_cost) as total_cost").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("provider, model_name").
		Order("total_cost DESC").
		Limit(10).
		Scan(&results)

	items := make([]ModelCostItem, len(results))
	for i, r := range results {
		pct := 0.0
		if total > 0 {
			pct = r.TotalCost / total * 100
		}
		items[i] = ModelCostItem{
			Provider:    r.Provider,
			Model:       r.ModelName,
			CallCount:   r.CallCount,
			TotalTokens: r.TotalTokens,
			TotalCost:   r.TotalCost,
			Percentage:  pct,
		}
	}
	return items
}

func (s *Service) getCostByProvider(ctx context.Context, tenantID string, start, end time.Time, total float64) []ProviderCostItem {
	var results []struct {
		Provider  string
		CallCount int64
		TotalCost float64
	}

	s.db.WithContext(ctx).Model(&metrics.ModelCallLog{}).
		Select("provider, COUNT(*) as call_count, SUM(total_cost) as total_cost").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("provider").
		Order("total_cost DESC").
		Scan(&results)

	items := make([]ProviderCostItem, len(results))
	for i, r := range results {
		pct := 0.0
		if total > 0 {
			pct = r.TotalCost / total * 100
		}
		items[i] = ProviderCostItem{
			Provider:   r.Provider,
			CallCount:  r.CallCount,
			TotalCost:  r.TotalCost,
			Percentage: pct,
		}
	}
	return items
}

func (s *Service) getCostByUser(ctx context.Context, tenantID string, start, end time.Time, total float64) []UserCostItem {
	var results []struct {
		UserID    string
		CallCount int64
		TotalCost float64
	}

	s.db.WithContext(ctx).Model(&metrics.ModelCallLog{}).
		Select("user_id, COUNT(*) as call_count, SUM(total_cost) as total_cost").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Where("user_id IS NOT NULL AND user_id != ''").
		Group("user_id").
		Order("total_cost DESC").
		Limit(10).
		Scan(&results)

	items := make([]UserCostItem, len(results))
	for i, r := range results {
		pct := 0.0
		if total > 0 {
			pct = r.TotalCost / total * 100
		}
		items[i] = UserCostItem{
			UserID:     r.UserID,
			CallCount:  r.CallCount,
			TotalCost:  r.TotalCost,
			Percentage: pct,
		}
	}
	return items
}

func (s *Service) getDailyTrend(ctx context.Context, tenantID string, start, end time.Time) []DailyCostItem {
	var results []struct {
		Date      string
		Cost      float64
		CallCount int64
		Tokens    int64
	}

	s.db.WithContext(ctx).Model(&metrics.ModelCallLog{}).
		Select("DATE(created_at) as date, SUM(total_cost) as cost, COUNT(*) as call_count, SUM(total_tokens) as tokens").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results)

	items := make([]DailyCostItem, len(results))
	for i, r := range results {
		items[i] = DailyCostItem{
			Date:      r.Date,
			Cost:      r.Cost,
			CallCount: r.CallCount,
			Tokens:    r.Tokens,
		}
	}
	return items
}

// ============================================================================
// 成本告警
// ============================================================================

// CreateAlert 创建告警
func (s *Service) CreateAlert(ctx context.Context, req *CreateAlertRequest) (*CostAlert, error) {
	alert := CostAlert{
		ID:            uuid.New().String(),
		TenantID:      req.TenantID,
		Name:          req.Name,
		AlertType:     req.AlertType,
		Threshold:     req.Threshold,
		UserID:        req.UserID,
		ModelName:     req.ModelName,
		NotifyEmail:   req.NotifyEmail,
		NotifyWebhook: req.NotifyWebhook,
		IsEnabled:     true,
	}

	if err := s.db.WithContext(ctx).Create(&alert).Error; err != nil {
		return nil, fmt.Errorf("创建告警失败: %w", err)
	}

	return &alert, nil
}

// ListAlerts 获取告警列表
func (s *Service) ListAlerts(ctx context.Context, tenantID string) ([]CostAlert, error) {
	var alerts []CostAlert
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&alerts).Error
	return alerts, err
}

// GetAlert 获取告警详情
func (s *Service) GetAlert(ctx context.Context, id string) (*CostAlert, error) {
	var alert CostAlert
	err := s.db.WithContext(ctx).First(&alert, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

// UpdateAlert 更新告警
func (s *Service) UpdateAlert(ctx context.Context, id string, req *UpdateAlertRequest) error {
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Threshold != nil {
		updates["threshold"] = *req.Threshold
	}
	if req.NotifyEmail != nil {
		updates["notify_email"] = *req.NotifyEmail
	}
	if req.NotifyWebhook != nil {
		updates["notify_webhook"] = *req.NotifyWebhook
	}
	if req.IsEnabled != nil {
		updates["is_enabled"] = *req.IsEnabled
	}

	if len(updates) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Model(&CostAlert{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeleteAlert 删除告警
func (s *Service) DeleteAlert(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&CostAlert{}, "id = ?", id).Error
}

// CheckAlerts 检查告警（定时任务调用）
func (s *Service) CheckAlerts(ctx context.Context, tenantID string) ([]AlertTriggerEvent, error) {
	var alerts []CostAlert
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND is_enabled = ?", tenantID, true).
		Find(&alerts).Error; err != nil {
		return nil, err
	}

	var events []AlertTriggerEvent
	now := time.Now()

	for _, alert := range alerts {
		var currentValue float64
		var startDate time.Time

		switch alert.AlertType {
		case AlertTypeDaily:
			startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		case AlertTypeWeekly:
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			startDate = now.AddDate(0, 0, -weekday+1)
			startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, now.Location())
		case AlertTypeMonthly:
			startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		case AlertTypeThreshold:
			startDate = time.Time{}
		default:
			continue
		}

		// 查询当前成本
		query := s.db.WithContext(ctx).Model(&metrics.ModelCallLog{}).
			Select("COALESCE(SUM(total_cost), 0)").
			Where("tenant_id = ?", tenantID)

		if !startDate.IsZero() {
			query = query.Where("created_at >= ?", startDate)
		}
		if alert.UserID != "" {
			query = query.Where("user_id = ?", alert.UserID)
		}
		if alert.ModelName != "" {
			query = query.Where("model_name = ?", alert.ModelName)
		}

		if err := query.Scan(&currentValue).Error; err != nil {
			continue
		}

		// 更新当前值
		s.db.WithContext(ctx).Model(&alert).Update("current_value", currentValue)

		// 检查是否触发
		if currentValue >= alert.Threshold {
			// 避免重复触发（24小时内）
			if alert.LastTriggered != nil && now.Sub(*alert.LastTriggered) < 24*time.Hour {
				continue
			}

			event := AlertTriggerEvent{
				AlertID:      alert.ID,
				AlertName:    alert.Name,
				AlertType:    alert.AlertType,
				TenantID:     alert.TenantID,
				Threshold:    alert.Threshold,
				CurrentValue: currentValue,
				TriggeredAt:  now,
				Message:      fmt.Sprintf("成本告警：%s 已达 %.2f（阈值 %.2f）", alert.Name, currentValue, alert.Threshold),
			}
			events = append(events, event)

			// 更新触发时间
			s.db.WithContext(ctx).Model(&alert).Updates(map[string]interface{}{
				"last_triggered": now,
				"trigger_count":  gorm.Expr("trigger_count + 1"),
			})
		}
	}

	return events, nil
}

// ============================================================================
// 计费审计
// ============================================================================

// QueryBillingAudit 查询计费审计记录
func (s *Service) QueryBillingAudit(ctx context.Context, query *BillingAuditQuery) ([]BillingAuditRecord, *BillingAuditSummary, int64, error) {
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 20
	}
	if query.Page <= 0 {
		query.Page = 1
	}

	db := s.db.WithContext(ctx).Model(&metrics.ModelCallLog{}).
		Where("tenant_id = ?", query.TenantID)

	if query.UserID != "" {
		db = db.Where("user_id = ?", query.UserID)
	}
	if query.Provider != "" {
		db = db.Where("provider = ?", query.Provider)
	}
	if query.Model != "" {
		db = db.Where("model_name = ?", query.Model)
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", query.EndTime)
	}
	if query.MinCost != nil {
		db = db.Where("total_cost >= ?", *query.MinCost)
	}
	if query.MaxCost != nil {
		db = db.Where("total_cost <= ?", *query.MaxCost)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	// 统计总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	// 查询摘要
	var summary BillingAuditSummary
	db.Select(`
		COUNT(*) as total_records,
		COALESCE(SUM(total_cost), 0) as total_cost,
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COUNT(CASE WHEN status = 'success' THEN 1 END) as success_count,
		COUNT(CASE WHEN status != 'success' THEN 1 END) as failed_count
	`).Scan(&summary)

	if summary.TotalRecords > 0 {
		summary.AverageCost = summary.TotalCost / float64(summary.TotalRecords)
		summary.AverageTokens = float64(summary.TotalTokens) / float64(summary.TotalRecords)
	}

	// 查询记录
	var logs []metrics.ModelCallLog
	offset := (query.Page - 1) * query.PageSize
	if err := s.db.WithContext(ctx).Model(&metrics.ModelCallLog{}).
		Where("tenant_id = ?", query.TenantID).
		Order("created_at DESC").
		Limit(query.PageSize).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, nil, 0, err
	}

	// 转换为审计记录
	records := make([]BillingAuditRecord, len(logs))
	for i, log := range logs {
		records[i] = BillingAuditRecord{
			ID:               log.ID,
			TenantID:         log.TenantID,
			UserID:           log.UserID,
			Provider:         log.Provider,
			Model:            log.ModelName,
			PromptTokens:     log.PromptTokens,
			CompletionTokens: log.CompletionTokens,
			TotalTokens:      log.TotalTokens,
			PromptCost:       log.PromptCost,
			CompletionCost:   log.CompletionCost,
			TotalCost:        log.TotalCost,
			AgentID:          log.AgentID,
			WorkflowID:       log.WorkflowID,
			Status:           log.Status,
			ResponseTimeMs:   log.ResponseTimeMs,
			CreatedAt:        log.CreatedAt,
		}
	}

	return records, &summary, total, nil
}

// ============================================================================
// Token 计价器
// ============================================================================

// CalculateTokenCost 计算 Token 成本
func (s *Service) CalculateTokenCost(ctx context.Context, req *TokenCalculatorRequest) (*TokenCalculatorResult, error) {
	tokens := req.Tokens
	if tokens <= 0 && req.Text != "" {
		// 简单估算：中文约每字1.5token，英文约每4字符1token
		tokens = estimateTokens(req.Text)
	}

	pricing, err := s.creditsService.GetPricing(ctx, req.TenantID, req.Provider, req.Model)
	if err != nil {
		return nil, err
	}

	var pricePerK float64
	switch req.Type {
	case "input":
		pricePerK = pricing.InputPrice
	case "output":
		pricePerK = pricing.OutputPrice
	default:
		pricePerK = (pricing.InputPrice + pricing.OutputPrice) / 2
	}

	creditCost := int64(float64(tokens)/1000.0*pricePerK + 0.5)
	if creditCost < 1 && tokens > 0 {
		creditCost = 1
	}

	return &TokenCalculatorResult{
		Provider:       req.Provider,
		Model:          req.Model,
		Tokens:         tokens,
		EstimatedCost:  float64(tokens) / 1000.0 * pricePerK / 100, // 假设100积分=1美元
		CreditCost:     creditCost,
		PricePerKToken: pricePerK,
		Currency:       "credits",
	}, nil
}

func estimateTokens(text string) int {
	chineseCount := 0
	otherCount := 0

	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
		} else {
			otherCount++
		}
	}

	return int(float64(chineseCount)*1.5) + otherCount/4
}

// ============================================================================
// 定价策略
// ============================================================================

// CreateStrategy 创建定价策略
func (s *Service) CreateStrategy(ctx context.Context, strategy *PricingStrategy) error {
	strategy.ID = uuid.New().String()

	if len(strategy.Rules) > 0 {
		rulesJSON, _ := json.Marshal(strategy.Rules)
		strategy.RulesJSON = string(rulesJSON)
	}

	return s.db.WithContext(ctx).Create(strategy).Error
}

// ListStrategies 获取策略列表
func (s *Service) ListStrategies(ctx context.Context, tenantID string) ([]PricingStrategy, error) {
	var strategies []PricingStrategy
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("is_default DESC, created_at DESC").
		Find(&strategies).Error

	// 解析 Rules JSON
	for i := range strategies {
		if strategies[i].RulesJSON != "" {
			json.Unmarshal([]byte(strategies[i].RulesJSON), &strategies[i].Rules)
		}
	}

	return strategies, err
}

// UpdateStrategy 更新策略
func (s *Service) UpdateStrategy(ctx context.Context, id string, strategy *PricingStrategy) error {
	if len(strategy.Rules) > 0 {
		rulesJSON, _ := json.Marshal(strategy.Rules)
		strategy.RulesJSON = string(rulesJSON)
	}

	return s.db.WithContext(ctx).Model(&PricingStrategy{}).
		Where("id = ?", id).
		Updates(strategy).Error
}

// DeleteStrategy 删除策略
func (s *Service) DeleteStrategy(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&PricingStrategy{}, "id = ?", id).Error
}

// AutoMigrate 自动迁移表结构
func (s *Service) AutoMigrate() error {
	return s.db.AutoMigrate(
		&CostAlert{},
		&PricingStrategy{},
		&Bill{},
		&Payment{},
		&Invoice{},
		&Refund{},
		&Reconciliation{},
		&ReconciliationDetail{},
	)
}

// ============================================================================
// 账单管理
// ============================================================================

// CreateBill 创建账单
func (s *Service) CreateBill(ctx context.Context, req *CreateBillRequest) (*Bill, error) {
	bill := &Bill{
		ID:                 uuid.New().String(),
		TenantID:           req.TenantID,
		UserID:             req.UserID,
		BillNo:             generateBillNo(),
		BillType:           req.BillType,
		Title:              req.Title,
		Description:        req.Description,
		Amount:             req.Amount,
		Currency:           req.Currency,
		DiscountAmount:     req.DiscountAmount,
		TaxAmount:          req.TaxAmount,
		TotalAmount:        req.Amount - req.DiscountAmount + req.TaxAmount,
		BillingPeriodStart: req.BillingPeriodStart,
		BillingPeriodEnd:   req.BillingPeriodEnd,
		DueDate:            req.DueDate,
		SubscriptionID:     req.SubscriptionID,
		Status:             BillStatusPending,
	}

	if bill.Currency == "" {
		bill.Currency = "CNY"
	}

	if len(req.Items) > 0 {
		itemsJSON, _ := json.Marshal(req.Items)
		bill.Items = string(itemsJSON)
	}

	if err := s.db.WithContext(ctx).Create(bill).Error; err != nil {
		return nil, err
	}

	return bill, nil
}

// GetBill 获取账单详情
func (s *Service) GetBill(ctx context.Context, billID string) (*Bill, error) {
	var bill Bill
	if err := s.db.WithContext(ctx).Where("id = ?", billID).First(&bill).Error; err != nil {
		return nil, err
	}
	return &bill, nil
}

// ListBills 获取账单列表
func (s *Service) ListBills(ctx context.Context, tenantID, userID string, status BillStatus, page, pageSize int) ([]Bill, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var bills []Bill
	var total int64

	query := s.db.WithContext(ctx).Model(&Bill{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&bills)

	return bills, total, nil
}

// CancelBill 取消账单
func (s *Service) CancelBill(ctx context.Context, billID string) error {
	return s.db.WithContext(ctx).Model(&Bill{}).
		Where("id = ? AND status = ?", billID, BillStatusPending).
		Update("status", BillStatusCanceled).Error
}

// ============================================================================
// 支付管理
// ============================================================================

// CreatePayment 创建支付单
func (s *Service) CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*Payment, error) {
	// 获取账单
	bill, err := s.GetBill(ctx, req.BillID)
	if err != nil {
		return nil, fmt.Errorf("账单不存在: %w", err)
	}

	if bill.Status != BillStatusPending {
		return nil, fmt.Errorf("账单状态不允许支付")
	}

	payment := &Payment{
		ID:            uuid.New().String(),
		TenantID:      req.TenantID,
		UserID:        req.UserID,
		PaymentNo:     generatePaymentNo(),
		BillID:        req.BillID,
		Amount:        req.Amount,
		Currency:      bill.Currency,
		PaymentMethod: req.PaymentMethod,
		Status:        PaymentStatusPending,
		NotifyURL:     req.NotifyURL,
		ReturnURL:     req.ReturnURL,
		Remark:        req.Remark,
	}

	// 设置过期时间（30分钟）
	expireAt := time.Now().Add(30 * time.Minute)
	payment.ExpireAt = &expireAt

	if err := s.db.WithContext(ctx).Create(payment).Error; err != nil {
		return nil, err
	}

	return payment, nil
}

// GetPayment 获取支付单详情
func (s *Service) GetPayment(ctx context.Context, paymentID string) (*Payment, error) {
	var payment Payment
	if err := s.db.WithContext(ctx).Where("id = ?", paymentID).First(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

// ListPayments 获取支付列表
func (s *Service) ListPayments(ctx context.Context, tenantID, userID string, status PaymentStatus, page, pageSize int) ([]Payment, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var payments []Payment
	var total int64

	query := s.db.WithContext(ctx).Model(&Payment{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&payments)

	return payments, total, nil
}

// PaymentCallback 支付回调处理
func (s *Service) PaymentCallback(ctx context.Context, paymentNo, tradeNo string, success bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var payment Payment
		if err := tx.Where("payment_no = ?", paymentNo).First(&payment).Error; err != nil {
			return err
		}

		now := time.Now()
		if success {
			payment.Status = PaymentStatusSuccess
			payment.TradeNo = tradeNo
			payment.PaidAt = &now

			// 更新账单状态
			if err := tx.Model(&Bill{}).Where("id = ?", payment.BillID).Updates(map[string]interface{}{
				"status":     BillStatusPaid,
				"paid_at":    now,
				"payment_id": payment.ID,
			}).Error; err != nil {
				return err
			}
		} else {
			payment.Status = PaymentStatusFailed
		}

		return tx.Save(&payment).Error
	})
}

// ============================================================================
// 发票管理
// ============================================================================

// CreateInvoice 创建发票申请
func (s *Service) CreateInvoice(ctx context.Context, req *CreateInvoiceRequest) (*Invoice, error) {
	// 计算发票金额
	var totalAmount float64
	for _, billID := range req.BillIDs {
		var bill Bill
		if err := s.db.WithContext(ctx).Where("id = ? AND status = ?", billID, BillStatusPaid).First(&bill).Error; err != nil {
			return nil, fmt.Errorf("账单 %s 不存在或未支付", billID)
		}
		totalAmount += bill.TotalAmount
	}

	billIDsJSON, _ := json.Marshal(req.BillIDs)

	// 计算税额
	taxRate := 0.06 // 默认6%税率
	taxAmount := totalAmount * taxRate / (1 + taxRate)

	invoice := &Invoice{
		ID:              uuid.New().String(),
		TenantID:        req.TenantID,
		UserID:          req.UserID,
		InvoiceType:     req.InvoiceType,
		TitleType:       req.TitleType,
		Title:           req.Title,
		TaxNo:           req.TaxNo,
		BankName:        req.BankName,
		BankAccount:     req.BankAccount,
		Address:         req.Address,
		Phone:           req.Phone,
		Amount:          totalAmount - taxAmount,
		TaxRate:         taxRate,
		TaxAmount:       taxAmount,
		TotalAmount:     totalAmount,
		BillIDs:         string(billIDsJSON),
		Status:          InvoiceStatusPending,
		ReceiverEmail:   req.ReceiverEmail,
		ReceiverPhone:   req.ReceiverPhone,
		ReceiverAddress: req.ReceiverAddress,
		Remark:          req.Remark,
	}

	if err := s.db.WithContext(ctx).Create(invoice).Error; err != nil {
		return nil, err
	}

	return invoice, nil
}

// GetInvoice 获取发票详情
func (s *Service) GetInvoice(ctx context.Context, invoiceID string) (*Invoice, error) {
	var invoice Invoice
	if err := s.db.WithContext(ctx).Where("id = ?", invoiceID).First(&invoice).Error; err != nil {
		return nil, err
	}
	return &invoice, nil
}

// ListInvoices 获取发票列表
func (s *Service) ListInvoices(ctx context.Context, tenantID, userID string, status InvoiceStatus, page, pageSize int) ([]Invoice, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var invoices []Invoice
	var total int64

	query := s.db.WithContext(ctx).Model(&Invoice{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&invoices)

	return invoices, total, nil
}

// IssueInvoice 开具发票（模拟）
func (s *Service) IssueInvoice(ctx context.Context, invoiceID string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&Invoice{}).
		Where("id = ? AND status = ?", invoiceID, InvoiceStatusPending).
		Updates(map[string]interface{}{
			"status":       InvoiceStatusIssued,
			"invoice_no":   generateInvoiceNo(),
			"invoice_code": fmt.Sprintf("%d", now.Unix()),
			"issued_at":    now,
			"verify_code":  generateVerifyCode(),
		}).Error
}

// VoidInvoice 作废发票
func (s *Service) VoidInvoice(ctx context.Context, invoiceID string) error {
	return s.db.WithContext(ctx).Model(&Invoice{}).
		Where("id = ? AND status = ?", invoiceID, InvoiceStatusIssued).
		Update("status", InvoiceStatusVoided).Error
}

// ============================================================================
// 退款管理
// ============================================================================

// CreateRefund 创建退款申请
func (s *Service) CreateRefund(ctx context.Context, req *CreateRefundRequest) (*Refund, error) {
	// 获取支付单
	payment, err := s.GetPayment(ctx, req.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("支付单不存在: %w", err)
	}

	if payment.Status != PaymentStatusSuccess {
		return nil, fmt.Errorf("支付单状态不允许退款")
	}

	// 检查可退款金额
	remainingAmount := payment.Amount - payment.RefundedAmount
	if req.Amount > remainingAmount {
		return nil, fmt.Errorf("退款金额超过可退金额 %.2f", remainingAmount)
	}

	refund := &Refund{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		UserID:      req.UserID,
		RefundNo:    generateRefundNo(),
		PaymentID:   req.PaymentID,
		BillID:      payment.BillID,
		Amount:      req.Amount,
		Currency:    payment.Currency,
		Reason:      req.Reason,
		Description: req.Description,
		Status:      RefundStatusPending,
		Remark:      req.Remark,
	}

	if err := s.db.WithContext(ctx).Create(refund).Error; err != nil {
		return nil, err
	}

	return refund, nil
}

// GetRefund 获取退款详情
func (s *Service) GetRefund(ctx context.Context, refundID string) (*Refund, error) {
	var refund Refund
	if err := s.db.WithContext(ctx).Where("id = ?", refundID).First(&refund).Error; err != nil {
		return nil, err
	}
	return &refund, nil
}

// ListRefunds 获取退款列表
func (s *Service) ListRefunds(ctx context.Context, tenantID, userID string, status RefundStatus, page, pageSize int) ([]Refund, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var refunds []Refund
	var total int64

	query := s.db.WithContext(ctx).Model(&Refund{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&refunds)

	return refunds, total, nil
}

// ProcessRefund 处理退款（审批）
func (s *Service) ProcessRefund(ctx context.Context, req *ProcessRefundRequest) error {
	now := time.Now()
	updates := make(map[string]interface{})

	if req.Action == "approve" {
		updates["status"] = RefundStatusApproved
		updates["approved_by"] = req.OperatorID
		updates["approved_at"] = now
	} else if req.Action == "reject" {
		updates["status"] = RefundStatusRejected
		updates["rejected_reason"] = req.RejectedReason
	} else {
		return fmt.Errorf("无效的操作: %s", req.Action)
	}

	return s.db.WithContext(ctx).Model(&Refund{}).
		Where("id = ? AND status = ?", req.RefundID, RefundStatusPending).
		Updates(updates).Error
}

// ExecuteRefund 执行退款
func (s *Service) ExecuteRefund(ctx context.Context, refundID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var refund Refund
		if err := tx.Where("id = ? AND status = ?", refundID, RefundStatusApproved).First(&refund).Error; err != nil {
			return err
		}

		now := time.Now()

		// 更新退款状态
		refund.Status = RefundStatusSuccess
		refund.ProcessedAt = &now
		refund.CompletedAt = &now
		if err := tx.Save(&refund).Error; err != nil {
			return err
		}

		// 更新支付单退款金额
		if err := tx.Model(&Payment{}).Where("id = ?", refund.PaymentID).
			Update("refunded_amount", gorm.Expr("refunded_amount + ?", refund.Amount)).Error; err != nil {
			return err
		}

		// 检查是否全额退款
		var payment Payment
		if err := tx.Where("id = ?", refund.PaymentID).First(&payment).Error; err != nil {
			return err
		}

		if payment.RefundedAmount >= payment.Amount {
			// 全额退款，更新支付单和账单状态
			tx.Model(&Payment{}).Where("id = ?", payment.ID).Update("status", PaymentStatusRefunded)
			tx.Model(&Bill{}).Where("id = ?", payment.BillID).Update("status", BillStatusRefunded)
		}

		return nil
	})
}

// ============================================================================
// 财务对账
// ============================================================================

// CreateReconciliation 创建对账任务
func (s *Service) CreateReconciliation(ctx context.Context, tenantID, paymentChannel string, reconcileDate time.Time) (*Reconciliation, error) {
	reconciliation := &Reconciliation{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		ReconcileNo:    generateReconcileNo(),
		ReconcileDate:  reconcileDate,
		PaymentChannel: paymentChannel,
		Status:         ReconcileStatusPending,
	}

	if err := s.db.WithContext(ctx).Create(reconciliation).Error; err != nil {
		return nil, err
	}

	return reconciliation, nil
}

// GetReconciliation 获取对账详情
func (s *Service) GetReconciliation(ctx context.Context, reconcileID string) (*Reconciliation, error) {
	var reconciliation Reconciliation
	if err := s.db.WithContext(ctx).Where("id = ?", reconcileID).First(&reconciliation).Error; err != nil {
		return nil, err
	}
	return &reconciliation, nil
}

// ListReconciliations 获取对账列表
func (s *Service) ListReconciliations(ctx context.Context, tenantID string, page, pageSize int) ([]Reconciliation, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var records []Reconciliation
	var total int64

	query := s.db.WithContext(ctx).Model(&Reconciliation{}).Where("tenant_id = ?", tenantID)
	query.Count(&total)
	query.Order("reconcile_date DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)

	return records, total, nil
}

// GetReconciliationSummary 获取对账汇总
func (s *Service) GetReconciliationSummary(ctx context.Context, tenantID string, startDate, endDate time.Time) (map[string]interface{}, error) {
	var result struct {
		TotalCount      int64   `json:"totalCount"`
		TotalAmount     float64 `json:"totalAmount"`
		MatchedCount    int64   `json:"matchedCount"`
		MatchedAmount   float64 `json:"matchedAmount"`
		MismatchCount   int64   `json:"mismatchCount"`
		MismatchAmount  float64 `json:"mismatchAmount"`
	}

	s.db.WithContext(ctx).Model(&Reconciliation{}).
		Select("COUNT(*) as total_count, COALESCE(SUM(total_amount),0) as total_amount, COALESCE(SUM(matched_count),0) as matched_count, COALESCE(SUM(matched_amount),0) as matched_amount, COALESCE(SUM(mismatch_count),0) as mismatch_count, COALESCE(SUM(mismatch_amount),0) as mismatch_amount").
		Where("tenant_id = ? AND reconcile_date BETWEEN ? AND ?", tenantID, startDate, endDate).
		Scan(&result)

	return map[string]interface{}{
		"totalCount":     result.TotalCount,
		"totalAmount":    result.TotalAmount,
		"matchedCount":   result.MatchedCount,
		"matchedAmount":  result.MatchedAmount,
		"mismatchCount":  result.MismatchCount,
		"mismatchAmount": result.MismatchAmount,
		"startDate":      startDate,
		"endDate":        endDate,
	}, nil
}

// ============================================================================
// 辅助函数
// ============================================================================

func generateBillNo() string {
	return fmt.Sprintf("BILL%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
}

func generatePaymentNo() string {
	return fmt.Sprintf("PAY%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
}

func generateInvoiceNo() string {
	return fmt.Sprintf("INV%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
}

func generateRefundNo() string {
	return fmt.Sprintf("REF%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
}

func generateReconcileNo() string {
	return fmt.Sprintf("REC%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
}

func generateVerifyCode() string {
	return fmt.Sprintf("%08d", time.Now().Nanosecond())
}
