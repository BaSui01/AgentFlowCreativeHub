package credits

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInsufficientCredits = errors.New("积分不足")
	ErrAccountNotFound     = errors.New("积分账户不存在")
	ErrInvalidAmount       = errors.New("无效的积分金额")
)

// Service 积分服务
type Service struct {
	db *gorm.DB
}

// NewService 创建积分服务
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// ============ 账户管理 ============

// GetOrCreateAccount 获取或创建积分账户
func (s *Service) GetOrCreateAccount(ctx context.Context, tenantID, userID string) (*CreditAccount, error) {
	var account CreditAccount
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&account).Error

	if err == nil {
		return &account, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 创建新账户
	account = CreditAccount{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		UserID:        userID,
		Balance:       0,
		WarnThreshold: 100,
	}
	if err := s.db.WithContext(ctx).Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// GetAccount 获取积分账户
func (s *Service) GetAccount(ctx context.Context, tenantID, userID string) (*CreditAccount, error) {
	var account CreditAccount
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAccountNotFound
	}
	return &account, err
}

// GetBalance 获取余额
func (s *Service) GetBalance(ctx context.Context, tenantID, userID string) (int64, error) {
	account, err := s.GetAccount(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return account.Balance, nil
}

// ============ 充值 ============

// Recharge 管理员充值
func (s *Service) Recharge(ctx context.Context, req *RechargeRequest) (*CreditTransaction, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var tx *CreditTransaction
	err := s.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		// 获取或创建账户
		account, err := s.getOrCreateAccountTx(db, req.TenantID, req.UserID)
		if err != nil {
			return err
		}

		// 创建流水
		tx = &CreditTransaction{
			ID:            uuid.New().String(),
			TenantID:      req.TenantID,
			UserID:        req.UserID,
			AccountID:     account.ID,
			Type:          TransactionTypeRecharge,
			Amount:        req.Amount,
			BalanceBefore: account.Balance,
			BalanceAfter:  account.Balance + req.Amount,
			Description:   fmt.Sprintf("管理员充值 %d 积分", req.Amount),
			Remark:        req.Remark,
			OperatorID:    req.OperatorID,
			OperatorName:  req.OperatorName,
		}
		if err := db.Create(tx).Error; err != nil {
			return err
		}

		// 更新账户余额
		return db.Model(account).Updates(map[string]interface{}{
			"balance":     gorm.Expr("balance + ?", req.Amount),
			"total_added": gorm.Expr("total_added + ?", req.Amount),
		}).Error
	})

	return tx, err
}

// ============ 消费 ============

// Consume 消费积分（AI调用时扣除）
func (s *Service) Consume(ctx context.Context, req *ConsumeRequest) (*CreditTransaction, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var tx *CreditTransaction
	err := s.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		// 获取账户并锁定
		var account CreditAccount
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", req.TenantID, req.UserID).
			First(&account).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAccountNotFound
			}
			return err
		}

		// 检查余额
		if account.Balance < req.Amount {
			return ErrInsufficientCredits
		}

		// 创建流水
		tx = &CreditTransaction{
			ID:            uuid.New().String(),
			TenantID:      req.TenantID,
			UserID:        req.UserID,
			AccountID:     account.ID,
			Type:          TransactionTypeConsume,
			Amount:        -req.Amount,
			BalanceBefore: account.Balance,
			BalanceAfter:  account.Balance - req.Amount,
			TokenUsageID:  req.TokenUsageID,
			WorkflowID:    req.WorkflowID,
			AgentID:       req.AgentID,
			Model:         req.Model,
			Description:   req.Description,
		}
		if tx.Description == "" {
			tx.Description = fmt.Sprintf("AI调用消耗 %d 积分 (%s)", req.Amount, req.Model)
		}
		if err := db.Create(tx).Error; err != nil {
			return err
		}

		// 更新账户余额
		return db.Model(&account).Updates(map[string]interface{}{
			"balance":    gorm.Expr("balance - ?", req.Amount),
			"total_used": gorm.Expr("total_used + ?", req.Amount),
		}).Error
	})

	return tx, err
}

// CheckBalance 检查余额是否足够
func (s *Service) CheckBalance(ctx context.Context, tenantID, userID string, required int64) (bool, error) {
	balance, err := s.GetBalance(ctx, tenantID, userID)
	if err != nil {
		return false, err
	}
	return balance >= required, nil
}

// ============ 赠送 ============

// Gift 赠送积分（注册、活动等）
func (s *Service) Gift(ctx context.Context, req *GiftRequest) (*CreditTransaction, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if req.Type == "" {
		req.Type = TransactionTypeGift
	}

	var tx *CreditTransaction
	err := s.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		account, err := s.getOrCreateAccountTx(db, req.TenantID, req.UserID)
		if err != nil {
			return err
		}

		tx = &CreditTransaction{
			ID:            uuid.New().String(),
			TenantID:      req.TenantID,
			UserID:        req.UserID,
			AccountID:     account.ID,
			Type:          req.Type,
			Amount:        req.Amount,
			BalanceBefore: account.Balance,
			BalanceAfter:  account.Balance + req.Amount,
			Description:   req.Description,
			OperatorID:    req.OperatorID,
			OperatorName:  req.OperatorName,
		}
		if tx.Description == "" {
			tx.Description = fmt.Sprintf("赠送 %d 积分", req.Amount)
		}
		if err := db.Create(tx).Error; err != nil {
			return err
		}

		return db.Model(account).Updates(map[string]interface{}{
			"balance":     gorm.Expr("balance + ?", req.Amount),
			"total_added": gorm.Expr("total_added + ?", req.Amount),
		}).Error
	})

	return tx, err
}

// GiftOnRegister 注册赠送积分
func (s *Service) GiftOnRegister(ctx context.Context, tenantID, userID string, amount int64) (*CreditTransaction, error) {
	return s.Gift(ctx, &GiftRequest{
		TenantID:    tenantID,
		UserID:      userID,
		Amount:      amount,
		Type:        TransactionTypeRegister,
		Description: fmt.Sprintf("新用户注册赠送 %d 积分", amount),
	})
}

// ============ 流水查询 ============

// ListTransactions 查询流水
func (s *Service) ListTransactions(ctx context.Context, query *TransactionQuery) ([]CreditTransaction, int64, error) {
	db := s.db.WithContext(ctx).Model(&CreditTransaction{}).
		Where("tenant_id = ?", query.TenantID)

	if query.UserID != "" {
		db = db.Where("user_id = ?", query.UserID)
	}
	if query.Type != "" {
		db = db.Where("type = ?", query.Type)
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", query.EndTime)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}

	var transactions []CreditTransaction
	err := db.Order("created_at DESC").
		Limit(query.Limit).
		Offset(query.Offset).
		Find(&transactions).Error

	return transactions, total, err
}

// ============ 统计报表 ============

// GetStats 获取统计数据
func (s *Service) GetStats(ctx context.Context, tenantID, userID string, period string) (*CreditStats, error) {
	stats := &CreditStats{
		TenantID: tenantID,
		UserID:   userID,
		Period:   period,
	}

	// 计算时间范围
	now := time.Now()
	switch period {
	case "daily":
		stats.StartDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		stats.EndDate = stats.StartDate.AddDate(0, 0, 1)
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		stats.StartDate = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		stats.EndDate = stats.StartDate.AddDate(0, 0, 7)
	case "monthly":
		stats.StartDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		stats.EndDate = stats.StartDate.AddDate(0, 1, 0)
	default:
		stats.StartDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		stats.EndDate = stats.StartDate.AddDate(0, 1, 0)
	}

	db := s.db.WithContext(ctx).Model(&CreditTransaction{}).
		Where("tenant_id = ?", tenantID).
		Where("created_at >= ? AND created_at < ?", stats.StartDate, stats.EndDate)

	if userID != "" {
		db = db.Where("user_id = ?", userID)
	}

	// 消费总额
	var consumeSum struct {
		Total int64
	}
	s.db.WithContext(ctx).Model(&CreditTransaction{}).
		Select("COALESCE(SUM(ABS(amount)), 0) as total").
		Where("tenant_id = ? AND type = ?", tenantID, TransactionTypeConsume).
		Where("created_at >= ? AND created_at < ?", stats.StartDate, stats.EndDate).
		Scan(&consumeSum)
	stats.TotalConsumed = consumeSum.Total

	// 充值总额
	var rechargeSum struct {
		Total int64
	}
	s.db.WithContext(ctx).Model(&CreditTransaction{}).
		Select("COALESCE(SUM(amount), 0) as total").
		Where("tenant_id = ? AND type IN ?", tenantID, []TransactionType{TransactionTypeRecharge, TransactionTypeGift, TransactionTypeRegister, TransactionTypeActivity}).
		Where("created_at >= ? AND created_at < ?", stats.StartDate, stats.EndDate).
		Scan(&rechargeSum)
	stats.TotalRecharged = rechargeSum.Total

	// 使用最多的模型
	var topModel struct {
		Model string
		Total int64
	}
	s.db.WithContext(ctx).Model(&CreditTransaction{}).
		Select("model, COALESCE(SUM(ABS(amount)), 0) as total").
		Where("tenant_id = ? AND type = ? AND model != ''", tenantID, TransactionTypeConsume).
		Where("created_at >= ? AND created_at < ?", stats.StartDate, stats.EndDate).
		Group("model").
		Order("total DESC").
		Limit(1).
		Scan(&topModel)
	stats.TopModel = topModel.Model
	stats.TopModelUsage = topModel.Total

	// 日均消耗
	days := stats.EndDate.Sub(stats.StartDate).Hours() / 24
	if days > 0 {
		stats.AvgDaily = float64(stats.TotalConsumed) / days
	}

	return stats, nil
}

// ListUserSummaries 获取用户积分摘要列表
func (s *Service) ListUserSummaries(ctx context.Context, tenantID string, limit, offset int) ([]UserCreditSummary, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var summaries []UserCreditSummary
	var total int64

	// 查询账户总数
	s.db.WithContext(ctx).Model(&CreditAccount{}).
		Where("tenant_id = ?", tenantID).
		Count(&total)

	// 联表查询用户信息
	err := s.db.WithContext(ctx).Raw(`
		SELECT 
			ca.user_id,
			u.username,
			u.email,
			ca.balance,
			ca.total_used,
			ca.total_added,
			(SELECT MAX(created_at) FROM credit_transactions WHERE user_id = ca.user_id AND type = 'consume') as last_used_at,
			(SELECT MAX(created_at) FROM credit_transactions WHERE user_id = ca.user_id AND type = 'recharge') as last_recharge_at
		FROM credit_accounts ca
		LEFT JOIN users u ON u.id = ca.user_id
		WHERE ca.tenant_id = ?
		ORDER BY ca.balance DESC
		LIMIT ? OFFSET ?
	`, tenantID, limit, offset).Scan(&summaries).Error

	return summaries, total, err
}

// ============ 预警 ============

// CheckAndWarn 检查余额并发送预警
func (s *Service) CheckAndWarn(ctx context.Context, tenantID, userID string) (bool, error) {
	account, err := s.GetAccount(ctx, tenantID, userID)
	if err != nil {
		return false, err
	}

	if account.Balance <= account.WarnThreshold {
		// 检查是否在24小时内已预警过
		if account.LastWarnAt != nil && time.Since(*account.LastWarnAt) < 24*time.Hour {
			return false, nil
		}

		// 更新预警时间
		now := time.Now()
		s.db.WithContext(ctx).Model(account).Update("last_warn_at", now)

		return true, nil // 需要发送预警
	}

	return false, nil
}

// UpdateWarnThreshold 更新预警阈值
func (s *Service) UpdateWarnThreshold(ctx context.Context, tenantID, userID string, threshold int64) error {
	return s.db.WithContext(ctx).Model(&CreditAccount{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("warn_threshold", threshold).Error
}

// ============ 导出 ============

// ExportTransactionsCSV 导出流水为CSV
func (s *Service) ExportTransactionsCSV(ctx context.Context, query *TransactionQuery) (string, error) {
	// 设置较大的limit用于导出
	query.Limit = 10000
	query.Offset = 0

	transactions, _, err := s.ListTransactions(ctx, query)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	writer := csv.NewWriter(&builder)

	// 写入表头
	writer.Write([]string{
		"ID", "用户ID", "类型", "金额", "变动前余额", "变动后余额",
		"模型", "描述", "操作人", "时间",
	})

	// 写入数据
	for _, tx := range transactions {
		writer.Write([]string{
			tx.ID,
			tx.UserID,
			string(tx.Type),
			fmt.Sprintf("%d", tx.Amount),
			fmt.Sprintf("%d", tx.BalanceBefore),
			fmt.Sprintf("%d", tx.BalanceAfter),
			tx.Model,
			tx.Description,
			tx.OperatorName,
			tx.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	writer.Flush()
	return builder.String(), writer.Error()
}

// ============ 定价 ============

// GetPricing 获取模型定价
func (s *Service) GetPricing(ctx context.Context, tenantID, provider, model string) (*CreditPricing, error) {
	var pricing CreditPricing
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND provider = ? AND model = ? AND is_active = ?", tenantID, provider, model, true).
		First(&pricing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 返回默认定价
		return &CreditPricing{
			Provider:    provider,
			Model:       model,
			InputPrice:  1.0,  // 默认每1K token 1积分
			OutputPrice: 2.0,  // 默认每1K token 2积分
		}, nil
	}
	return &pricing, err
}

// CalculateCost 计算消耗积分
func (s *Service) CalculateCost(ctx context.Context, tenantID, provider, model string, inputTokens, outputTokens int) (int64, error) {
	pricing, err := s.GetPricing(ctx, tenantID, provider, model)
	if err != nil {
		return 0, err
	}

	inputCost := float64(inputTokens) / 1000.0 * pricing.InputPrice
	outputCost := float64(outputTokens) / 1000.0 * pricing.OutputPrice
	total := int64(inputCost + outputCost + 0.5) // 四舍五入

	if total < 1 {
		total = 1 // 最少消耗1积分
	}

	return total, nil
}

// ============ 内部方法 ============

func (s *Service) getOrCreateAccountTx(db *gorm.DB, tenantID, userID string) (*CreditAccount, error) {
	var account CreditAccount
	err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&account).Error
	if err == nil {
		return &account, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	account = CreditAccount{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		UserID:        userID,
		Balance:       0,
		WarnThreshold: 100,
	}
	if err := db.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
