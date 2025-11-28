package billing

import (
	"time"
)

// ============================================================================
// 模型定价配置
// ============================================================================

// ModelPricing 模型定价配置
// 复用 credits.CreditPricing，此处定义扩展视图
type ModelPricing struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenantId"`
	Provider    string    `json:"provider"`     // openai, anthropic, deepseek, etc.
	Model       string    `json:"model"`        // gpt-4, claude-3-opus, etc.
	InputPrice  float64   `json:"inputPrice"`   // 每1K token输入价格（积分）
	OutputPrice float64   `json:"outputPrice"`  // 每1K token输出价格（积分）
	Currency    string    `json:"currency"`     // credits, usd
	IsActive    bool      `json:"isActive"`
	Priority    int       `json:"priority"`     // 优先级，用于多租户定价覆盖
	Remark      string    `json:"remark"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CreatePricingRequest 创建定价请求
type CreatePricingRequest struct {
	TenantID    string  `json:"tenantId" binding:"required"`
	Provider    string  `json:"provider" binding:"required"`
	Model       string  `json:"model" binding:"required"`
	InputPrice  float64 `json:"inputPrice" binding:"required,gte=0"`
	OutputPrice float64 `json:"outputPrice" binding:"required,gte=0"`
	Currency    string  `json:"currency"`
	Remark      string  `json:"remark"`
}

// UpdatePricingRequest 更新定价请求
type UpdatePricingRequest struct {
	InputPrice  *float64 `json:"inputPrice"`
	OutputPrice *float64 `json:"outputPrice"`
	IsActive    *bool    `json:"isActive"`
	Remark      *string  `json:"remark"`
}

// ============================================================================
// 成本预估
// ============================================================================

// CostEstimateRequest 成本预估请求
type CostEstimateRequest struct {
	TenantID     string `json:"tenantId" binding:"required"`
	Provider     string `json:"provider" binding:"required"`
	Model        string `json:"model" binding:"required"`
	InputTokens  int    `json:"inputTokens" binding:"required,gte=0"`
	OutputTokens int    `json:"outputTokens" binding:"gte=0"`
}

// CostEstimate 成本预估结果
type CostEstimate struct {
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	InputTokens     int     `json:"inputTokens"`
	OutputTokens    int     `json:"outputTokens"`
	InputCost       float64 `json:"inputCost"`       // 输入成本
	OutputCost      float64 `json:"outputCost"`      // 输出成本
	TotalCost       int64   `json:"totalCost"`       // 总成本（积分）
	PricePerKInput  float64 `json:"pricePerKInput"`  // 每1K输入价格
	PricePerKOutput float64 `json:"pricePerKOutput"` // 每1K输出价格
	Currency        string  `json:"currency"`        // 货币单位
}

// ============================================================================
// 成本报表
// ============================================================================

// CostReportRequest 成本报表请求
type CostReportRequest struct {
	TenantID  string     `json:"tenantId" binding:"required"`
	StartDate *time.Time `json:"startDate"`
	EndDate   *time.Time `json:"endDate"`
	GroupBy   string     `json:"groupBy"` // day, week, month, model, user
	UserID    string     `json:"userId"`
	ModelName string     `json:"modelName"`
}

// CostReport 成本报表
type CostReport struct {
	TenantID         string               `json:"tenantId"`
	StartDate        time.Time            `json:"startDate"`
	EndDate          time.Time            `json:"endDate"`
	TotalCost        float64              `json:"totalCost"`
	TotalCalls       int64                `json:"totalCalls"`
	TotalTokens      int64                `json:"totalTokens"`
	AverageCostPerCall float64            `json:"averageCostPerCall"`
	DailyCost        float64              `json:"dailyCost"`
	ProjectedMonthly float64              `json:"projectedMonthly"`
	ByModel          []ModelCostItem      `json:"byModel"`
	ByProvider       []ProviderCostItem   `json:"byProvider"`
	ByUser           []UserCostItem       `json:"byUser"`
	DailyTrend       []DailyCostItem      `json:"dailyTrend"`
	GeneratedAt      time.Time            `json:"generatedAt"`
}

// ModelCostItem 模型成本项
type ModelCostItem struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	CallCount   int64   `json:"callCount"`
	TotalTokens int64   `json:"totalTokens"`
	TotalCost   float64 `json:"totalCost"`
	Percentage  float64 `json:"percentage"`
}

// ProviderCostItem 提供商成本项
type ProviderCostItem struct {
	Provider    string  `json:"provider"`
	CallCount   int64   `json:"callCount"`
	TotalCost   float64 `json:"totalCost"`
	Percentage  float64 `json:"percentage"`
}

// UserCostItem 用户成本项
type UserCostItem struct {
	UserID      string  `json:"userId"`
	Username    string  `json:"username"`
	CallCount   int64   `json:"callCount"`
	TotalCost   float64 `json:"totalCost"`
	Percentage  float64 `json:"percentage"`
}

// DailyCostItem 每日成本项
type DailyCostItem struct {
	Date      string  `json:"date"`
	Cost      float64 `json:"cost"`
	CallCount int64   `json:"callCount"`
	Tokens    int64   `json:"tokens"`
}

// ============================================================================
// 成本告警
// ============================================================================

// CostAlert 成本告警配置
type CostAlert struct {
	ID            string     `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID      string     `json:"tenantId" gorm:"type:uuid;not null;index"`
	Name          string     `json:"name" gorm:"size:100;not null"`
	AlertType     string     `json:"alertType" gorm:"size:50;not null"`     // daily, weekly, monthly, threshold
	Threshold     float64    `json:"threshold" gorm:"type:decimal(10,2)"`   // 成本阈值
	CurrentValue  float64    `json:"currentValue" gorm:"type:decimal(10,2)"`
	UserID        string     `json:"userId" gorm:"type:uuid"`               // 针对特定用户
	ModelName     string     `json:"modelName" gorm:"size:100"`             // 针对特定模型
	NotifyEmail   string     `json:"notifyEmail" gorm:"size:255"`
	NotifyWebhook string     `json:"notifyWebhook" gorm:"size:500"`
	IsEnabled     bool       `json:"isEnabled" gorm:"default:true"`
	LastTriggered *time.Time `json:"lastTriggered"`
	TriggerCount  int        `json:"triggerCount" gorm:"default:0"`
	CreatedAt     time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt     time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (CostAlert) TableName() string {
	return "cost_alerts"
}

// AlertType 告警类型
const (
	AlertTypeDaily     = "daily"     // 日消费超阈值
	AlertTypeWeekly    = "weekly"    // 周消费超阈值
	AlertTypeMonthly   = "monthly"   // 月消费超阈值
	AlertTypeThreshold = "threshold" // 累计消费超阈值
	AlertTypeSpike     = "spike"     // 异常波动
)

// CreateAlertRequest 创建告警请求
type CreateAlertRequest struct {
	TenantID      string  `json:"tenantId" binding:"required"`
	Name          string  `json:"name" binding:"required"`
	AlertType     string  `json:"alertType" binding:"required"`
	Threshold     float64 `json:"threshold" binding:"required,gt=0"`
	UserID        string  `json:"userId"`
	ModelName     string  `json:"modelName"`
	NotifyEmail   string  `json:"notifyEmail"`
	NotifyWebhook string  `json:"notifyWebhook"`
}

// UpdateAlertRequest 更新告警请求
type UpdateAlertRequest struct {
	Name          *string  `json:"name"`
	Threshold     *float64 `json:"threshold"`
	NotifyEmail   *string  `json:"notifyEmail"`
	NotifyWebhook *string  `json:"notifyWebhook"`
	IsEnabled     *bool    `json:"isEnabled"`
}

// AlertTriggerEvent 告警触发事件
type AlertTriggerEvent struct {
	AlertID      string    `json:"alertId"`
	AlertName    string    `json:"alertName"`
	AlertType    string    `json:"alertType"`
	TenantID     string    `json:"tenantId"`
	Threshold    float64   `json:"threshold"`
	CurrentValue float64   `json:"currentValue"`
	TriggeredAt  time.Time `json:"triggeredAt"`
	Message      string    `json:"message"`
}

// ============================================================================
// 计费审计
// ============================================================================

// BillingAuditQuery 计费审计查询
type BillingAuditQuery struct {
	TenantID  string     `json:"tenantId" binding:"required"`
	UserID    string     `json:"userId"`
	Provider  string     `json:"provider"`
	Model     string     `json:"model"`
	StartTime *time.Time `json:"startTime"`
	EndTime   *time.Time `json:"endTime"`
	MinCost   *float64   `json:"minCost"`
	MaxCost   *float64   `json:"maxCost"`
	Status    string     `json:"status"`
	Page      int        `json:"page"`
	PageSize  int        `json:"pageSize"`
}

// BillingAuditRecord 计费审计记录
type BillingAuditRecord struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenantId"`
	UserID           string    `json:"userId"`
	Username         string    `json:"username"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	PromptCost       float64   `json:"promptCost"`
	CompletionCost   float64   `json:"completionCost"`
	TotalCost        float64   `json:"totalCost"`
	CreditDeducted   int64     `json:"creditDeducted"`
	AgentID          string    `json:"agentId"`
	WorkflowID       string    `json:"workflowId"`
	Status           string    `json:"status"`
	ResponseTimeMs   int       `json:"responseTimeMs"`
	CreatedAt        time.Time `json:"createdAt"`
}

// BillingAuditSummary 计费审计摘要
type BillingAuditSummary struct {
	TotalRecords     int64   `json:"totalRecords"`
	TotalCost        float64 `json:"totalCost"`
	TotalTokens      int64   `json:"totalTokens"`
	TotalCredits     int64   `json:"totalCredits"`
	SuccessCount     int64   `json:"successCount"`
	FailedCount      int64   `json:"failedCount"`
	AverageCost      float64 `json:"averageCost"`
	AverageTokens    float64 `json:"averageTokens"`
}

// ============================================================================
// Token 计价器
// ============================================================================

// TokenCalculatorRequest Token计价请求
type TokenCalculatorRequest struct {
	TenantID string `json:"tenantId"`
	Provider string `json:"provider" binding:"required"`
	Model    string `json:"model" binding:"required"`
	Text     string `json:"text"`           // 文本内容（可选，自动计算token）
	Tokens   int    `json:"tokens"`         // 直接指定token数
	Type     string `json:"type"`           // input, output, both
}

// TokenCalculatorResult Token计价结果
type TokenCalculatorResult struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Tokens       int     `json:"tokens"`
	EstimatedCost float64 `json:"estimatedCost"` // 美元
	CreditCost   int64   `json:"creditCost"`     // 积分
	PricePerKToken float64 `json:"pricePerKToken"`
	Currency     string  `json:"currency"`
}

// ============================================================================
// 定价策略
// ============================================================================

// PricingStrategy 定价策略
type PricingStrategy struct {
	ID          string            `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string            `json:"tenantId" gorm:"type:uuid;not null;index"`
	Name        string            `json:"name" gorm:"size:100;not null"`
	Description string            `json:"description" gorm:"size:500"`
	StrategyType string           `json:"strategyType" gorm:"size:50;not null"` // flat, tiered, volume
	Rules       []PricingRule     `json:"rules" gorm:"-"`                       // 计算规则（JSON存储）
	RulesJSON   string            `json:"-" gorm:"column:rules;type:jsonb"`
	IsDefault   bool              `json:"isDefault" gorm:"default:false"`
	IsActive    bool              `json:"isActive" gorm:"default:true"`
	CreatedAt   time.Time         `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time         `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (PricingStrategy) TableName() string {
	return "pricing_strategies"
}

// PricingRule 定价规则
type PricingRule struct {
	MinTokens  int     `json:"minTokens"`  // 最小token数
	MaxTokens  int     `json:"maxTokens"`  // 最大token数
	Multiplier float64 `json:"multiplier"` // 价格倍率
	FixedCost  float64 `json:"fixedCost"`  // 固定费用
}

// ============================================================================
// 账单管理
// ============================================================================

// BillStatus 账单状态
type BillStatus string

const (
	BillStatusPending   BillStatus = "pending"   // 待支付
	BillStatusPaid      BillStatus = "paid"      // 已支付
	BillStatusOverdue   BillStatus = "overdue"   // 逾期
	BillStatusCanceled  BillStatus = "canceled"  // 已取消
	BillStatusRefunded  BillStatus = "refunded"  // 已退款
)

// Bill 账单
type Bill struct {
	ID              string     `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID        string     `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID          string     `json:"userId" gorm:"type:uuid;not null;index"`
	BillNo          string     `json:"billNo" gorm:"size:50;not null;uniqueIndex"` // 账单编号
	BillType        string     `json:"billType" gorm:"size:50;not null"`           // subscription, credits, usage
	Title           string     `json:"title" gorm:"size:200;not null"`
	Description     string     `json:"description" gorm:"type:text"`
	
	// 金额
	Amount          float64    `json:"amount" gorm:"type:decimal(10,2);not null"`
	Currency        string     `json:"currency" gorm:"size:10;default:CNY"`
	DiscountAmount  float64    `json:"discountAmount" gorm:"type:decimal(10,2);default:0"`
	TaxAmount       float64    `json:"taxAmount" gorm:"type:decimal(10,2);default:0"`
	TotalAmount     float64    `json:"totalAmount" gorm:"type:decimal(10,2);not null"`
	
	// 账期
	BillingPeriodStart *time.Time `json:"billingPeriodStart"`
	BillingPeriodEnd   *time.Time `json:"billingPeriodEnd"`
	DueDate            *time.Time `json:"dueDate"`
	
	// 状态
	Status          BillStatus `json:"status" gorm:"size:20;not null;default:pending;index"`
	PaidAt          *time.Time `json:"paidAt"`
	PaymentID       string     `json:"paymentId" gorm:"type:uuid"` // 关联支付记录
	
	// 关联
	SubscriptionID  string     `json:"subscriptionId" gorm:"type:uuid"`
	InvoiceID       string     `json:"invoiceId" gorm:"type:uuid"`
	
	// 明细
	Items           string     `json:"items" gorm:"type:jsonb"` // BillItem 数组 JSON
	
	CreatedAt       time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Bill) TableName() string {
	return "bills"
}

// BillItem 账单明细项
type BillItem struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	Amount      float64 `json:"amount"`
}

// ============================================================================
// 支付管理
// ============================================================================

// PaymentStatus 支付状态
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"   // 待支付
	PaymentStatusPaying    PaymentStatus = "paying"    // 支付中
	PaymentStatusSuccess   PaymentStatus = "success"   // 支付成功
	PaymentStatusFailed    PaymentStatus = "failed"    // 支付失败
	PaymentStatusClosed    PaymentStatus = "closed"    // 已关闭
	PaymentStatusRefunding PaymentStatus = "refunding" // 退款中
	PaymentStatusRefunded  PaymentStatus = "refunded"  // 已退款
)

// PaymentMethod 支付方式
const (
	PaymentMethodWechat  = "wechat"  // 微信支付
	PaymentMethodAlipay  = "alipay"  // 支付宝
	PaymentMethodCard    = "card"    // 银行卡
	PaymentMethodCredits = "credits" // 积分抵扣
	PaymentMethodBalance = "balance" // 余额支付
)

// Payment 支付记录
type Payment struct {
	ID              string        `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID        string        `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID          string        `json:"userId" gorm:"type:uuid;not null;index"`
	PaymentNo       string        `json:"paymentNo" gorm:"size:50;not null;uniqueIndex"` // 支付单号
	BillID          string        `json:"billId" gorm:"type:uuid;index"`
	
	// 金额
	Amount          float64       `json:"amount" gorm:"type:decimal(10,2);not null"`
	Currency        string        `json:"currency" gorm:"size:10;default:CNY"`
	
	// 支付信息
	PaymentMethod   string        `json:"paymentMethod" gorm:"size:50;not null"`
	Status          PaymentStatus `json:"status" gorm:"size:20;not null;default:pending;index"`
	
	// 第三方支付信息
	TradeNo         string        `json:"tradeNo" gorm:"size:100;index"`      // 第三方交易号
	PayerAccount    string        `json:"payerAccount" gorm:"size:100"`       // 付款账号（脱敏）
	PaymentChannel  string        `json:"paymentChannel" gorm:"size:50"`      // 支付渠道
	
	// 时间
	ExpireAt        *time.Time    `json:"expireAt"`                           // 过期时间
	PaidAt          *time.Time    `json:"paidAt"`                             // 支付时间
	
	// 回调
	NotifyURL       string        `json:"notifyUrl" gorm:"size:500"`
	ReturnURL       string        `json:"returnUrl" gorm:"size:500"`
	NotifyResult    string        `json:"notifyResult" gorm:"type:text"`      // 回调结果
	
	// 退款信息
	RefundedAmount  float64       `json:"refundedAmount" gorm:"type:decimal(10,2);default:0"`
	
	Remark          string        `json:"remark" gorm:"size:500"`
	CreatedAt       time.Time     `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time     `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Payment) TableName() string {
	return "payments"
}

// ============================================================================
// 发票管理
// ============================================================================

// InvoiceStatus 发票状态
type InvoiceStatus string

const (
	InvoiceStatusPending  InvoiceStatus = "pending"  // 待开票
	InvoiceStatusIssuing  InvoiceStatus = "issuing"  // 开票中
	InvoiceStatusIssued   InvoiceStatus = "issued"   // 已开票
	InvoiceStatusFailed   InvoiceStatus = "failed"   // 开票失败
	InvoiceStatusVoided   InvoiceStatus = "voided"   // 已作废
)

// InvoiceType 发票类型
const (
	InvoiceTypeNormal   = "normal"   // 普通发票
	InvoiceTypeSpecial  = "special"  // 专用发票
	InvoiceTypeElectric = "electric" // 电子发票
)

// Invoice 发票
type Invoice struct {
	ID              string        `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID        string        `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID          string        `json:"userId" gorm:"type:uuid;not null;index"`
	InvoiceNo       string        `json:"invoiceNo" gorm:"size:50;uniqueIndex"` // 发票号码
	InvoiceCode     string        `json:"invoiceCode" gorm:"size:50"`           // 发票代码
	
	// 类型
	InvoiceType     string        `json:"invoiceType" gorm:"size:20;not null"`
	TitleType       string        `json:"titleType" gorm:"size:20"`             // personal, company
	
	// 抬头信息
	Title           string        `json:"title" gorm:"size:200;not null"`       // 发票抬头
	TaxNo           string        `json:"taxNo" gorm:"size:50"`                 // 税号
	BankName        string        `json:"bankName" gorm:"size:100"`             // 开户银行
	BankAccount     string        `json:"bankAccount" gorm:"size:50"`           // 银行账号
	Address         string        `json:"address" gorm:"size:300"`              // 地址
	Phone           string        `json:"phone" gorm:"size:50"`                 // 电话
	
	// 金额
	Amount          float64       `json:"amount" gorm:"type:decimal(10,2);not null"`
	TaxRate         float64       `json:"taxRate" gorm:"type:decimal(5,2);default:0.06"` // 税率
	TaxAmount       float64       `json:"taxAmount" gorm:"type:decimal(10,2)"`
	TotalAmount     float64       `json:"totalAmount" gorm:"type:decimal(10,2);not null"`
	
	// 关联
	BillIDs         string        `json:"billIds" gorm:"type:jsonb"`            // 关联的账单ID列表
	
	// 状态
	Status          InvoiceStatus `json:"status" gorm:"size:20;not null;default:pending;index"`
	IssuedAt        *time.Time    `json:"issuedAt"`
	
	// 电子发票
	FileURL         string        `json:"fileUrl" gorm:"size:500"`              // PDF下载链接
	VerifyCode      string        `json:"verifyCode" gorm:"size:50"`            // 校验码
	
	// 收件信息
	ReceiverEmail   string        `json:"receiverEmail" gorm:"size:100"`
	ReceiverPhone   string        `json:"receiverPhone" gorm:"size:20"`
	ReceiverAddress string        `json:"receiverAddress" gorm:"size:300"`
	
	Remark          string        `json:"remark" gorm:"size:500"`
	CreatedAt       time.Time     `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time     `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Invoice) TableName() string {
	return "invoices"
}

// ============================================================================
// 退款管理
// ============================================================================

// RefundStatus 退款状态
type RefundStatus string

const (
	RefundStatusPending    RefundStatus = "pending"    // 待处理
	RefundStatusApproved   RefundStatus = "approved"   // 已批准
	RefundStatusProcessing RefundStatus = "processing" // 处理中
	RefundStatusSuccess    RefundStatus = "success"    // 退款成功
	RefundStatusRejected   RefundStatus = "rejected"   // 已拒绝
	RefundStatusFailed     RefundStatus = "failed"     // 退款失败
)

// Refund 退款记录
type Refund struct {
	ID              string       `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID        string       `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID          string       `json:"userId" gorm:"type:uuid;not null;index"`
	RefundNo        string       `json:"refundNo" gorm:"size:50;not null;uniqueIndex"` // 退款单号
	
	// 关联
	PaymentID       string       `json:"paymentId" gorm:"type:uuid;not null;index"`
	BillID          string       `json:"billId" gorm:"type:uuid;index"`
	
	// 金额
	Amount          float64      `json:"amount" gorm:"type:decimal(10,2);not null"`
	Currency        string       `json:"currency" gorm:"size:10;default:CNY"`
	
	// 退款原因
	Reason          string       `json:"reason" gorm:"size:50;not null"`       // duplicate, fraud, request, other
	Description     string       `json:"description" gorm:"type:text"`
	
	// 状态
	Status          RefundStatus `json:"status" gorm:"size:20;not null;default:pending;index"`
	
	// 第三方退款信息
	TradeNo         string       `json:"tradeNo" gorm:"size:100"`              // 第三方退款单号
	RefundAccount   string       `json:"refundAccount" gorm:"size:100"`        // 退款账号
	
	// 审批
	ApprovedBy      string       `json:"approvedBy" gorm:"type:uuid"`
	ApprovedAt      *time.Time   `json:"approvedAt"`
	RejectedReason  string       `json:"rejectedReason" gorm:"size:500"`
	
	// 时间
	ProcessedAt     *time.Time   `json:"processedAt"`                          // 退款处理时间
	CompletedAt     *time.Time   `json:"completedAt"`                          // 退款完成时间
	
	Remark          string       `json:"remark" gorm:"size:500"`
	CreatedAt       time.Time    `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time    `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Refund) TableName() string {
	return "refunds"
}

// ============================================================================
// 财务对账
// ============================================================================

// ReconciliationStatus 对账状态
type ReconciliationStatus string

const (
	ReconcileStatusPending   ReconciliationStatus = "pending"   // 待对账
	ReconcileStatusMatched   ReconciliationStatus = "matched"   // 已匹配
	ReconcileStatusMismatch  ReconciliationStatus = "mismatch"  // 不匹配
	ReconcileStatusException ReconciliationStatus = "exception" // 异常
)

// Reconciliation 对账记录
type Reconciliation struct {
	ID              string               `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID        string               `json:"tenantId" gorm:"type:uuid;not null;index"`
	ReconcileNo     string               `json:"reconcileNo" gorm:"size:50;not null;uniqueIndex"`
	ReconcileDate   time.Time            `json:"reconcileDate" gorm:"type:date;not null;index"`
	PaymentChannel  string               `json:"paymentChannel" gorm:"size:50;not null"`
	
	// 统计
	TotalCount      int                  `json:"totalCount" gorm:"default:0"`
	TotalAmount     float64              `json:"totalAmount" gorm:"type:decimal(12,2);default:0"`
	MatchedCount    int                  `json:"matchedCount" gorm:"default:0"`
	MatchedAmount   float64              `json:"matchedAmount" gorm:"type:decimal(12,2);default:0"`
	MismatchCount   int                  `json:"mismatchCount" gorm:"default:0"`
	MismatchAmount  float64              `json:"mismatchAmount" gorm:"type:decimal(12,2);default:0"`
	
	// 状态
	Status          ReconciliationStatus `json:"status" gorm:"size:20;not null;default:pending"`
	
	// 对账文件
	FileURL         string               `json:"fileUrl" gorm:"size:500"`
	
	// 处理信息
	ProcessedBy     string               `json:"processedBy" gorm:"type:uuid"`
	ProcessedAt     *time.Time           `json:"processedAt"`
	Remark          string               `json:"remark" gorm:"type:text"`
	
	CreatedAt       time.Time            `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time            `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Reconciliation) TableName() string {
	return "reconciliations"
}

// ReconciliationDetail 对账明细
type ReconciliationDetail struct {
	ID               string               `json:"id" gorm:"primaryKey;type:uuid"`
	ReconciliationID string               `json:"reconciliationId" gorm:"type:uuid;not null;index"`
	PaymentID        string               `json:"paymentId" gorm:"type:uuid;index"`
	
	// 平台数据
	PlatformTradeNo  string               `json:"platformTradeNo" gorm:"size:100"`
	PlatformAmount   float64              `json:"platformAmount" gorm:"type:decimal(10,2)"`
	PlatformTime     *time.Time           `json:"platformTime"`
	
	// 第三方数据
	ChannelTradeNo   string               `json:"channelTradeNo" gorm:"size:100"`
	ChannelAmount    float64              `json:"channelAmount" gorm:"type:decimal(10,2)"`
	ChannelTime      *time.Time           `json:"channelTime"`
	
	// 差异
	AmountDiff       float64              `json:"amountDiff" gorm:"type:decimal(10,2)"`
	Status           ReconciliationStatus `json:"status" gorm:"size:20"`
	Remark           string               `json:"remark" gorm:"size:500"`
	
	CreatedAt        time.Time            `json:"createdAt" gorm:"autoCreateTime"`
}

func (ReconciliationDetail) TableName() string {
	return "reconciliation_details"
}

// ============================================================================
// 请求结构
// ============================================================================

// CreateBillRequest 创建账单请求
type CreateBillRequest struct {
	TenantID           string     `json:"tenantId"`
	UserID             string     `json:"userId" binding:"required"`
	BillType           string     `json:"billType" binding:"required"`
	Title              string     `json:"title" binding:"required"`
	Description        string     `json:"description"`
	Amount             float64    `json:"amount" binding:"required,gt=0"`
	Currency           string     `json:"currency"`
	DiscountAmount     float64    `json:"discountAmount"`
	TaxAmount          float64    `json:"taxAmount"`
	BillingPeriodStart *time.Time `json:"billingPeriodStart"`
	BillingPeriodEnd   *time.Time `json:"billingPeriodEnd"`
	DueDate            *time.Time `json:"dueDate"`
	SubscriptionID     string     `json:"subscriptionId"`
	Items              []BillItem `json:"items"`
}

// CreatePaymentRequest 创建支付请求
type CreatePaymentRequest struct {
	TenantID      string  `json:"tenantId"`
	UserID        string  `json:"userId"`
	BillID        string  `json:"billId" binding:"required"`
	PaymentMethod string  `json:"paymentMethod" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	NotifyURL     string  `json:"notifyUrl"`
	ReturnURL     string  `json:"returnUrl"`
	Remark        string  `json:"remark"`
}

// CreateInvoiceRequest 创建发票请求
type CreateInvoiceRequest struct {
	TenantID        string   `json:"tenantId"`
	UserID          string   `json:"userId"`
	InvoiceType     string   `json:"invoiceType" binding:"required"`
	TitleType       string   `json:"titleType" binding:"required"`
	Title           string   `json:"title" binding:"required"`
	TaxNo           string   `json:"taxNo"`
	BankName        string   `json:"bankName"`
	BankAccount     string   `json:"bankAccount"`
	Address         string   `json:"address"`
	Phone           string   `json:"phone"`
	BillIDs         []string `json:"billIds" binding:"required"`
	ReceiverEmail   string   `json:"receiverEmail"`
	ReceiverPhone   string   `json:"receiverPhone"`
	ReceiverAddress string   `json:"receiverAddress"`
	Remark          string   `json:"remark"`
}

// CreateRefundRequest 创建退款请求
type CreateRefundRequest struct {
	TenantID    string  `json:"tenantId"`
	UserID      string  `json:"userId"`
	PaymentID   string  `json:"paymentId" binding:"required"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Reason      string  `json:"reason" binding:"required"`
	Description string  `json:"description"`
	Remark      string  `json:"remark"`
}

// ProcessRefundRequest 处理退款请求
type ProcessRefundRequest struct {
	RefundID       string `json:"refundId" binding:"required"`
	Action         string `json:"action" binding:"required"` // approve, reject
	RejectedReason string `json:"rejectedReason"`
	OperatorID     string `json:"operatorId"`
}

// StrategyType 策略类型常量
const (
	StrategyTypeFlat   = "flat"   // 固定价格
	StrategyTypeTiered = "tiered" // 阶梯价格
	StrategyTypeVolume = "volume" // 批量折扣
)
