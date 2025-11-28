package credits

import (
	"time"
)

// CreditAccount 积分账户
type CreditAccount struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID    string    `json:"userId" gorm:"type:uuid;not null;uniqueIndex:idx_credit_account_user"`
	Balance   int64     `json:"balance" gorm:"not null;default:0"`         // 当前余额
	TotalUsed int64     `json:"totalUsed" gorm:"not null;default:0"`       // 累计消耗
	TotalAdded int64    `json:"totalAdded" gorm:"not null;default:0"`      // 累计充值
	FreezeAmount int64  `json:"freezeAmount" gorm:"not null;default:0"`    // 冻结金额
	WarnThreshold int64 `json:"warnThreshold" gorm:"default:100"`          // 预警阈值
	LastWarnAt *time.Time `json:"lastWarnAt"`                              // 上次预警时间
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TransactionType 交易类型
type TransactionType string

const (
	TransactionTypeRecharge   TransactionType = "recharge"    // 充值
	TransactionTypeConsume    TransactionType = "consume"     // 消费
	TransactionTypeGift       TransactionType = "gift"        // 赠送
	TransactionTypeRefund     TransactionType = "refund"      // 退款
	TransactionTypeExpire     TransactionType = "expire"      // 过期
	TransactionTypeAdjust     TransactionType = "adjust"      // 调整
	TransactionTypeRegister   TransactionType = "register"    // 注册赠送
	TransactionTypeActivity   TransactionType = "activity"    // 活动赠送
)

// CreditTransaction 积分流水
type CreditTransaction struct {
	ID            string          `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID      string          `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID        string          `json:"userId" gorm:"type:uuid;not null;index:idx_credit_tx_user"`
	AccountID     string          `json:"accountId" gorm:"type:uuid;not null;index"`
	Type          TransactionType `json:"type" gorm:"size:20;not null;index:idx_credit_tx_type"`
	Amount        int64           `json:"amount" gorm:"not null"`           // 变动金额（正负）
	BalanceBefore int64           `json:"balanceBefore" gorm:"not null"`    // 变动前余额
	BalanceAfter  int64           `json:"balanceAfter" gorm:"not null"`     // 变动后余额
	
	// 关联信息
	TokenUsageID  string `json:"tokenUsageId" gorm:"type:uuid;index"`     // 关联的Token消耗记录
	WorkflowID    string `json:"workflowId" gorm:"type:uuid"`
	AgentID       string `json:"agentId" gorm:"type:uuid"`
	Model         string `json:"model" gorm:"size:100"`                    // AI模型名称
	
	// 描述信息
	Description   string `json:"description" gorm:"size:500"`
	Remark        string `json:"remark" gorm:"size:500"`                   // 管理员备注
	
	// 操作信息
	OperatorID    string `json:"operatorId" gorm:"type:uuid"`             // 操作人（充值/调整时）
	OperatorName  string `json:"operatorName" gorm:"size:100"`
	
	// 时间
	CreatedAt     time.Time `json:"createdAt" gorm:"not null;autoCreateTime;index:idx_credit_tx_time"`
}

// CreditPricing 积分定价配置
type CreditPricing struct {
	ID            string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID      string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	Provider      string    `json:"provider" gorm:"size:50;not null"`      // openai, anthropic, etc.
	Model         string    `json:"model" gorm:"size:100;not null"`        // gpt-4, claude-3, etc.
	InputPrice    float64   `json:"inputPrice" gorm:"type:decimal(10,6)"`  // 每1K token输入价格（积分）
	OutputPrice   float64   `json:"outputPrice" gorm:"type:decimal(10,6)"` // 每1K token输出价格（积分）
	IsActive      bool      `json:"isActive" gorm:"default:true"`
	CreatedAt     time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt     time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// CreditStats 积分统计
type CreditStats struct {
	TenantID       string    `json:"tenantId"`
	UserID         string    `json:"userId"`
	Period         string    `json:"period"`         // daily, weekly, monthly
	TotalConsumed  int64     `json:"totalConsumed"`
	TotalRecharged int64     `json:"totalRecharged"`
	AvgDaily       float64   `json:"avgDaily"`
	TopModel       string    `json:"topModel"`
	TopModelUsage  int64     `json:"topModelUsage"`
	StartDate      time.Time `json:"startDate"`
	EndDate        time.Time `json:"endDate"`
}

// UserCreditSummary 用户积分摘要
type UserCreditSummary struct {
	UserID        string    `json:"userId"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	Balance       int64     `json:"balance"`
	TotalUsed     int64     `json:"totalUsed"`
	TotalAdded    int64     `json:"totalAdded"`
	LastUsedAt    *time.Time `json:"lastUsedAt"`
	LastRechargeAt *time.Time `json:"lastRechargeAt"`
}

// RechargeRequest 充值请求
type RechargeRequest struct {
	TenantID    string `json:"tenantId"`
	UserID      string `json:"userId"`
	Amount      int64  `json:"amount" binding:"required,gt=0"`
	Remark      string `json:"remark"`
	OperatorID  string `json:"operatorId"`
	OperatorName string `json:"operatorName"`
}

// ConsumeRequest 消费请求
type ConsumeRequest struct {
	TenantID      string `json:"tenantId"`
	UserID        string `json:"userId"`
	Amount        int64  `json:"amount"`
	TokenUsageID  string `json:"tokenUsageId"`
	WorkflowID    string `json:"workflowId"`
	AgentID       string `json:"agentId"`
	Model         string `json:"model"`
	Description   string `json:"description"`
}

// GiftRequest 赠送请求
type GiftRequest struct {
	TenantID     string          `json:"tenantId"`
	UserID       string          `json:"userId"`
	Amount       int64           `json:"amount" binding:"required,gt=0"`
	Type         TransactionType `json:"type"` // register, activity, gift
	Description  string          `json:"description"`
	OperatorID   string          `json:"operatorId"`
	OperatorName string          `json:"operatorName"`
}

// TransactionQuery 流水查询条件
type TransactionQuery struct {
	TenantID  string          `json:"tenantId"`
	UserID    string          `json:"userId"`
	Type      TransactionType `json:"type"`
	StartTime *time.Time      `json:"startTime"`
	EndTime   *time.Time      `json:"endTime"`
	Limit     int             `json:"limit"`
	Offset    int             `json:"offset"`
}

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatJSON ExportFormat = "json"
)
