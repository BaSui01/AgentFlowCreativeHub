package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPlanNotFound         = errors.New("订阅计划不存在")
	ErrSubscriptionNotFound = errors.New("订阅记录不存在")
	ErrAlreadySubscribed    = errors.New("已存在有效订阅")
	ErrTrialUsed            = errors.New("试用期已使用")
	ErrPlanCodeExists       = errors.New("套餐代码已存在")
	ErrCannotDowngrade      = errors.New("无法降级到更低套餐")
)

// Service 订阅服务
type Service struct {
	db *gorm.DB
}

// NewService 创建订阅服务
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// ========== 套餐管理 ==========

// CreatePlan 创建订阅计划
func (s *Service) CreatePlan(ctx context.Context, tenantID string, req *CreatePlanRequest) (*SubscriptionPlan, error) {
	// 检查代码唯一性
	var count int64
	s.db.WithContext(ctx).Model(&SubscriptionPlan{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return nil, ErrPlanCodeExists
	}

	featuresJSON, _ := json.Marshal(req.Features)
	permissionsJSON, _ := json.Marshal(req.Permissions)

	plan := &SubscriptionPlan{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		Name:         req.Name,
		Code:         req.Code,
		Tier:         req.Tier,
		Description:  req.Description,
		PriceMonthly: req.PriceMonthly,
		PriceYearly:  req.PriceYearly,
		Currency:     req.Currency,
		Features:     string(featuresJSON),
		Permissions:  string(permissionsJSON),
		TrialDays:    req.TrialDays,
		TrialCredits: req.TrialCredits,
		IsActive:     true,
		IsDefault:    req.IsDefault,
	}

	if plan.Currency == "" {
		plan.Currency = "CNY"
	}

	// 如果设为默认，取消其他默认
	if req.IsDefault {
		s.db.WithContext(ctx).Model(&SubscriptionPlan{}).
			Where("tenant_id = ? OR tenant_id = ''", tenantID).
			Update("is_default", false)
	}

	if err := s.db.WithContext(ctx).Create(plan).Error; err != nil {
		return nil, err
	}

	return plan, nil
}

// GetPlan 获取套餐详情
func (s *Service) GetPlan(ctx context.Context, planID string) (*SubscriptionPlan, error) {
	var plan SubscriptionPlan
	if err := s.db.WithContext(ctx).Where("id = ?", planID).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}
	return &plan, nil
}

// GetPlanByCode 根据代码获取套餐
func (s *Service) GetPlanByCode(ctx context.Context, code string) (*SubscriptionPlan, error) {
	var plan SubscriptionPlan
	if err := s.db.WithContext(ctx).Where("code = ? AND is_active = ?", code, true).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}
	return &plan, nil
}

// ListPlans 列出可用套餐
func (s *Service) ListPlans(ctx context.Context, tenantID string, includeGlobal bool) ([]SubscriptionPlan, error) {
	var plans []SubscriptionPlan
	query := s.db.WithContext(ctx).Where("is_active = ?", true)
	
	if includeGlobal {
		query = query.Where("tenant_id = ? OR tenant_id = ''", tenantID)
	} else {
		query = query.Where("tenant_id = ?", tenantID)
	}
	
	if err := query.Order("sort_order, created_at").Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

// UpdatePlan 更新套餐
func (s *Service) UpdatePlan(ctx context.Context, planID string, updates map[string]interface{}) error {
	result := s.db.WithContext(ctx).Model(&SubscriptionPlan{}).Where("id = ?", planID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPlanNotFound
	}
	return nil
}

// DeletePlan 删除套餐（软删除，设为不活跃）
func (s *Service) DeletePlan(ctx context.Context, planID string) error {
	return s.UpdatePlan(ctx, planID, map[string]interface{}{"is_active": false})
}

// GetDefaultPlan 获取默认套餐
func (s *Service) GetDefaultPlan(ctx context.Context, tenantID string) (*SubscriptionPlan, error) {
	var plan SubscriptionPlan
	err := s.db.WithContext(ctx).
		Where("(tenant_id = ? OR tenant_id = '') AND is_active = ? AND is_default = ?", tenantID, true, true).
		First(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 返回 free 套餐
			return s.GetPlanByCode(ctx, "free")
		}
		return nil, err
	}
	return &plan, nil
}

// ========== 订阅管理 ==========

// Subscribe 订阅套餐
func (s *Service) Subscribe(ctx context.Context, req *SubscribeRequest) (*UserSubscription, error) {
	plan, err := s.GetPlan(ctx, req.PlanID)
	if err != nil {
		return nil, err
	}

	// 检查是否已有有效订阅
	var existing UserSubscription
	err = s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND status IN ?", req.TenantID, req.UserID, 
			[]SubscriptionStatus{StatusActive, StatusTrialing}).
		First(&existing).Error
	if err == nil {
		return nil, ErrAlreadySubscribed
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := time.Now()
	sub := &UserSubscription{
		ID:           uuid.New().String(),
		TenantID:     req.TenantID,
		UserID:       req.UserID,
		PlanID:       plan.ID,
		PlanCode:     plan.Code,
		PlanTier:     plan.Tier,
		BillingCycle: req.BillingCycle,
		StartDate:    now,
		AutoRenew:    req.AutoRenew,
	}

	// 处理试用期
	if req.StartTrial && plan.TrialDays > 0 {
		// 检查是否已使用过试用
		var trialCount int64
		s.db.WithContext(ctx).Model(&UserSubscription{}).
			Where("tenant_id = ? AND user_id = ? AND trial_start_date IS NOT NULL", req.TenantID, req.UserID).
			Count(&trialCount)
		if trialCount > 0 {
			return nil, ErrTrialUsed
		}

		sub.Status = StatusTrialing
		sub.TrialStartDate = &now
		trialEnd := now.AddDate(0, 0, plan.TrialDays)
		sub.TrialEndDate = &trialEnd
		sub.EndDate = &trialEnd
	} else {
		sub.Status = StatusActive
		// 计算到期时间
		var endDate time.Time
		switch req.BillingCycle {
		case BillingCycleMonthly:
			endDate = now.AddDate(0, 1, 0)
		case BillingCycleYearly:
			endDate = now.AddDate(1, 0, 0)
		case BillingCycleLifetime:
			endDate = now.AddDate(100, 0, 0) // 100年
		default:
			endDate = now.AddDate(0, 1, 0)
		}
		sub.EndDate = &endDate
		if req.AutoRenew {
			sub.NextBillingDate = &endDate
		}
	}

	if err := s.db.WithContext(ctx).Create(sub).Error; err != nil {
		return nil, err
	}

	// 记录历史
	s.recordHistory(ctx, &SubscriptionHistory{
		ID:             uuid.New().String(),
		TenantID:       req.TenantID,
		UserID:         req.UserID,
		SubscriptionID: sub.ID,
		Action:         "create",
		ToPlanID:       plan.ID,
		ToStatus:       string(sub.Status),
	})

	return sub, nil
}

// GetSubscription 获取订阅详情
func (s *Service) GetSubscription(ctx context.Context, subscriptionID string) (*UserSubscription, error) {
	var sub UserSubscription
	if err := s.db.WithContext(ctx).Where("id = ?", subscriptionID).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	return &sub, nil
}

// GetUserSubscription 获取用户当前订阅
func (s *Service) GetUserSubscription(ctx context.Context, tenantID, userID string) (*UserSubscription, error) {
	var sub UserSubscription
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND status IN ?", tenantID, userID,
			[]SubscriptionStatus{StatusActive, StatusTrialing, StatusPastDue}).
		Order("created_at DESC").
		First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	return &sub, nil
}

// ListUserSubscriptions 列出用户订阅历史
func (s *Service) ListUserSubscriptions(ctx context.Context, tenantID, userID string) ([]UserSubscription, error) {
	var subs []UserSubscription
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("created_at DESC").
		Find(&subs).Error
	return subs, err
}

// CancelSubscription 取消订阅
func (s *Service) CancelSubscription(ctx context.Context, req *CancelRequest) error {
	sub, err := s.GetSubscription(ctx, req.SubscriptionID)
	if err != nil {
		return err
	}

	now := time.Now()
	updates := map[string]interface{}{
		"canceled_at":     now,
		"cancel_reason":   req.Reason,
		"cancel_feedback": req.Feedback,
		"auto_renew":      false,
	}

	if req.Immediate {
		updates["status"] = StatusCanceled
		updates["end_date"] = now
	}

	if err := s.db.WithContext(ctx).Model(&UserSubscription{}).Where("id = ?", req.SubscriptionID).Updates(updates).Error; err != nil {
		return err
	}

	// 记录历史
	toStatus := string(sub.Status)
	if req.Immediate {
		toStatus = string(StatusCanceled)
	}
	s.recordHistory(ctx, &SubscriptionHistory{
		ID:             uuid.New().String(),
		TenantID:       req.TenantID,
		UserID:         req.UserID,
		SubscriptionID: req.SubscriptionID,
		Action:         "cancel",
		FromStatus:     string(sub.Status),
		ToStatus:       toStatus,
		Remark:         req.Reason,
	})

	return nil
}

// RenewSubscription 续订
func (s *Service) RenewSubscription(ctx context.Context, subscriptionID string, cycle BillingCycle) (*UserSubscription, error) {
	sub, err := s.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	plan, err := s.GetPlan(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	
	// 计算新的到期时间（从当前到期时间或现在开始）
	startFrom := now
	if sub.EndDate != nil && sub.EndDate.After(now) {
		startFrom = *sub.EndDate
	}

	var newEndDate time.Time
	var amount float64
	switch cycle {
	case BillingCycleMonthly:
		newEndDate = startFrom.AddDate(0, 1, 0)
		amount = plan.PriceMonthly
	case BillingCycleYearly:
		newEndDate = startFrom.AddDate(1, 0, 0)
		amount = plan.PriceYearly
	default:
		newEndDate = startFrom.AddDate(0, 1, 0)
		amount = plan.PriceMonthly
	}

	updates := map[string]interface{}{
		"status":            StatusActive,
		"end_date":          newEndDate,
		"billing_cycle":     cycle,
		"last_payment_at":   now,
		"total_paid":        gorm.Expr("total_paid + ?", amount),
	}

	if sub.AutoRenew {
		updates["next_billing_date"] = newEndDate
	}

	if err := s.db.WithContext(ctx).Model(&UserSubscription{}).Where("id = ?", subscriptionID).Updates(updates).Error; err != nil {
		return nil, err
	}

	// 记录历史
	s.recordHistory(ctx, &SubscriptionHistory{
		ID:             uuid.New().String(),
		TenantID:       sub.TenantID,
		UserID:         sub.UserID,
		SubscriptionID: subscriptionID,
		Action:         "renew",
		FromStatus:     string(sub.Status),
		ToStatus:       string(StatusActive),
		Amount:         amount,
	})

	return s.GetSubscription(ctx, subscriptionID)
}

// ChangePlan 更换套餐（升降级）
func (s *Service) ChangePlan(ctx context.Context, subscriptionID, newPlanID string) (*UserSubscription, error) {
	sub, err := s.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	newPlan, err := s.GetPlan(ctx, newPlanID)
	if err != nil {
		return nil, err
	}

	oldPlanID := sub.PlanID
	action := "upgrade"
	if tierRank(newPlan.Tier) < tierRank(sub.PlanTier) {
		action = "downgrade"
	}

	updates := map[string]interface{}{
		"plan_id":   newPlanID,
		"plan_code": newPlan.Code,
		"plan_tier": newPlan.Tier,
	}

	if err := s.db.WithContext(ctx).Model(&UserSubscription{}).Where("id = ?", subscriptionID).Updates(updates).Error; err != nil {
		return nil, err
	}

	// 记录历史
	s.recordHistory(ctx, &SubscriptionHistory{
		ID:             uuid.New().String(),
		TenantID:       sub.TenantID,
		UserID:         sub.UserID,
		SubscriptionID: subscriptionID,
		Action:         action,
		FromPlanID:     oldPlanID,
		ToPlanID:       newPlanID,
	})

	return s.GetSubscription(ctx, subscriptionID)
}

// ========== 试用管理 ==========

// StartTrial 开始试用
func (s *Service) StartTrial(ctx context.Context, tenantID, userID, planID string) (*UserSubscription, error) {
	return s.Subscribe(ctx, &SubscribeRequest{
		TenantID:   tenantID,
		UserID:     userID,
		PlanID:     planID,
		StartTrial: true,
	})
}

// ConvertTrial 试用转正
func (s *Service) ConvertTrial(ctx context.Context, subscriptionID string, cycle BillingCycle, autoRenew bool) (*UserSubscription, error) {
	sub, err := s.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	if sub.Status != StatusTrialing {
		return nil, errors.New("非试用状态无法转正")
	}

	plan, err := s.GetPlan(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var endDate time.Time
	var amount float64
	switch cycle {
	case BillingCycleMonthly:
		endDate = now.AddDate(0, 1, 0)
		amount = plan.PriceMonthly
	case BillingCycleYearly:
		endDate = now.AddDate(1, 0, 0)
		amount = plan.PriceYearly
	default:
		endDate = now.AddDate(0, 1, 0)
		amount = plan.PriceMonthly
	}

	updates := map[string]interface{}{
		"status":          StatusActive,
		"billing_cycle":   cycle,
		"start_date":      now,
		"end_date":        endDate,
		"auto_renew":      autoRenew,
		"last_payment_at": now,
		"total_paid":      amount,
	}
	if autoRenew {
		updates["next_billing_date"] = endDate
	}

	if err := s.db.WithContext(ctx).Model(&UserSubscription{}).Where("id = ?", subscriptionID).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.recordHistory(ctx, &SubscriptionHistory{
		ID:             uuid.New().String(),
		TenantID:       sub.TenantID,
		UserID:         sub.UserID,
		SubscriptionID: subscriptionID,
		Action:         "convert",
		FromStatus:     string(StatusTrialing),
		ToStatus:       string(StatusActive),
		Amount:         amount,
	})

	return s.GetSubscription(ctx, subscriptionID)
}

// ========== 统计 ==========

// GetStats 获取订阅统计
func (s *Service) GetStats(ctx context.Context, tenantID string) (*SubscriptionStats, error) {
	stats := &SubscriptionStats{
		TenantID:         tenantID,
		PlanDistribution: make(map[string]int64),
	}

	// 总用户数
	s.db.WithContext(ctx).Model(&UserSubscription{}).
		Where("tenant_id = ?", tenantID).
		Distinct("user_id").
		Count(&stats.TotalUsers)

	// 活跃订阅
	s.db.WithContext(ctx).Model(&UserSubscription{}).
		Where("tenant_id = ? AND status = ?", tenantID, StatusActive).
		Count(&stats.ActiveUsers)

	// 试用中
	s.db.WithContext(ctx).Model(&UserSubscription{}).
		Where("tenant_id = ? AND status = ?", tenantID, StatusTrialing).
		Count(&stats.TrialingUsers)

	// 付费用户
	s.db.WithContext(ctx).Model(&UserSubscription{}).
		Where("tenant_id = ? AND total_paid > 0", tenantID).
		Distinct("user_id").
		Count(&stats.PaidUsers)

	// 流失用户
	s.db.WithContext(ctx).Model(&UserSubscription{}).
		Where("tenant_id = ? AND status IN ?", tenantID, []SubscriptionStatus{StatusCanceled, StatusExpired}).
		Distinct("user_id").
		Count(&stats.ChurnedUsers)

	// MRR（月度订阅收入）
	type MRRResult struct {
		Total float64
	}
	var mrrResult MRRResult
	s.db.WithContext(ctx).Model(&UserSubscription{}).
		Select("COALESCE(SUM(CASE WHEN billing_cycle = 'monthly' THEN total_paid ELSE total_paid/12 END), 0) as total").
		Where("tenant_id = ? AND status = ?", tenantID, StatusActive).
		Scan(&mrrResult)
	stats.MRR = mrrResult.Total
	stats.ARR = stats.MRR * 12

	// 套餐分布
	type PlanCount struct {
		PlanCode string
		Count    int64
	}
	var planCounts []PlanCount
	s.db.WithContext(ctx).Model(&UserSubscription{}).
		Select("plan_code, COUNT(DISTINCT user_id) as count").
		Where("tenant_id = ? AND status IN ?", tenantID, []SubscriptionStatus{StatusActive, StatusTrialing}).
		Group("plan_code").
		Scan(&planCounts)
	for _, pc := range planCounts {
		stats.PlanDistribution[pc.PlanCode] = pc.Count
	}

	// 计算续订率和流失率
	if stats.TotalUsers > 0 {
		stats.ChurnRate = float64(stats.ChurnedUsers) / float64(stats.TotalUsers) * 100
		stats.RenewalRate = 100 - stats.ChurnRate
	}

	return stats, nil
}

// CheckExpiring 检查即将到期的订阅（用于发送提醒）
func (s *Service) CheckExpiring(ctx context.Context, tenantID string, daysAhead int) ([]UserSubscription, error) {
	deadline := time.Now().AddDate(0, 0, daysAhead)
	var subs []UserSubscription
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ? AND end_date <= ? AND end_date > ?", 
			tenantID, StatusActive, deadline, time.Now()).
		Find(&subs).Error
	return subs, err
}

// ProcessExpired 处理过期订阅
func (s *Service) ProcessExpired(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).Model(&UserSubscription{}).
		Where("status IN ? AND end_date < ?", 
			[]SubscriptionStatus{StatusActive, StatusTrialing, StatusPastDue}, time.Now()).
		Update("status", StatusExpired)
	return result.RowsAffected, result.Error
}

// ========== 辅助方法 ==========

func (s *Service) recordHistory(ctx context.Context, h *SubscriptionHistory) {
	h.CreatedAt = time.Now()
	s.db.WithContext(ctx).Create(h)
}

func tierRank(tier PlanTier) int {
	switch tier {
	case PlanTierFree:
		return 0
	case PlanTierBasic:
		return 1
	case PlanTierPro:
		return 2
	case PlanTierEnterprise:
		return 3
	default:
		return 0
	}
}

// GetPlanFeatures 解析套餐权益
func (p *SubscriptionPlan) GetPlanFeatures() (*PlanFeatures, error) {
	var features PlanFeatures
	if p.Features == "" {
		return &features, nil
	}
	err := json.Unmarshal([]byte(p.Features), &features)
	return &features, err
}

// GetPermissions 解析套餐权限
func (p *SubscriptionPlan) GetPermissions() ([]string, error) {
	var perms []string
	if p.Permissions == "" {
		return perms, nil
	}
	err := json.Unmarshal([]byte(p.Permissions), &perms)
	return perms, err
}
