package subscription

import (
	"time"
)

// PlanTier 套餐等级
type PlanTier string

const (
	PlanTierFree       PlanTier = "free"
	PlanTierBasic      PlanTier = "basic"
	PlanTierPro        PlanTier = "pro"
	PlanTierEnterprise PlanTier = "enterprise"
)

// BillingCycle 计费周期
type BillingCycle string

const (
	BillingCycleMonthly  BillingCycle = "monthly"
	BillingCycleYearly   BillingCycle = "yearly"
	BillingCycleLifetime BillingCycle = "lifetime"
)

// SubscriptionStatus 订阅状态
type SubscriptionStatus string

const (
	StatusActive    SubscriptionStatus = "active"
	StatusTrialing  SubscriptionStatus = "trialing"
	StatusPastDue   SubscriptionStatus = "past_due"
	StatusCanceled  SubscriptionStatus = "canceled"
	StatusExpired   SubscriptionStatus = "expired"
	StatusSuspended SubscriptionStatus = "suspended"
)

// SubscriptionPlan 订阅计划（套餐定义）
type SubscriptionPlan struct {
	ID          string       `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string       `json:"tenantId" gorm:"type:uuid;index"` // 空表示全局套餐
	Name        string       `json:"name" gorm:"size:100;not null"`
	Code        string       `json:"code" gorm:"size:50;not null;uniqueIndex"`
	Tier        PlanTier     `json:"tier" gorm:"size:20;not null"`
	Description string       `json:"description" gorm:"type:text"`
	
	// 定价
	PriceMonthly float64 `json:"priceMonthly" gorm:"type:decimal(10,2)"`
	PriceYearly  float64 `json:"priceYearly" gorm:"type:decimal(10,2)"`
	Currency     string  `json:"currency" gorm:"size:10;default:CNY"`
	
	// 权益配置（JSON）
	Features     string `json:"features" gorm:"type:jsonb"`       // {"maxUsers": 10, "maxStorage": 1024, ...}
	Permissions  string `json:"permissions" gorm:"type:jsonb"`    // ["workflow:create", "agent:execute", ...]
	
	// 试用配置
	TrialDays    int  `json:"trialDays" gorm:"default:0"`
	TrialCredits int64 `json:"trialCredits" gorm:"default:0"` // 试用赠送积分
	
	// 状态
	IsActive   bool `json:"isActive" gorm:"default:true"`
	IsDefault  bool `json:"isDefault" gorm:"default:false"` // 默认套餐（新用户自动分配）
	SortOrder  int  `json:"sortOrder" gorm:"default:0"`
	
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

// PlanFeatures 套餐权益（解析 Features JSON）
type PlanFeatures struct {
	MaxUsers           int   `json:"maxUsers"`
	MaxStorageMB       int   `json:"maxStorageMB"`
	MaxWorkflows       int   `json:"maxWorkflows"`
	MaxKnowledgeBases  int   `json:"maxKnowledgeBases"`
	MaxTokensPerMonth  int64 `json:"maxTokensPerMonth"`
	MaxAPICallsPerDay  int   `json:"maxAPICallsPerDay"`
	MaxCreditsPerMonth int64 `json:"maxCreditsPerMonth"`
	
	// 功能开关
	EnableRAG           bool `json:"enableRAG"`
	EnableWorkflow      bool `json:"enableWorkflow"`
	EnableMultiAgent    bool `json:"enableMultiAgent"`
	EnableCustomModel   bool `json:"enableCustomModel"`
	EnableAPIAccess     bool `json:"enableAPIAccess"`
	EnablePriorityQueue bool `json:"enablePriorityQueue"` // 优先队列
}

// UserSubscription 用户订阅记录
type UserSubscription struct {
	ID        string             `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string             `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID    string             `json:"userId" gorm:"type:uuid;not null;index:idx_subscription_user"`
	PlanID    string             `json:"planId" gorm:"type:uuid;not null;index"`
	PlanCode  string             `json:"planCode" gorm:"size:50;not null"` // 冗余便于查询
	PlanTier  PlanTier           `json:"planTier" gorm:"size:20;not null"` // 冗余便于查询
	Status    SubscriptionStatus `json:"status" gorm:"size:20;not null;index"`
	
	// 周期
	BillingCycle BillingCycle `json:"billingCycle" gorm:"size:20"`
	StartDate    time.Time    `json:"startDate" gorm:"not null"`
	EndDate      *time.Time   `json:"endDate"`                      // 到期时间
	
	// 试用
	TrialStartDate *time.Time `json:"trialStartDate"`
	TrialEndDate   *time.Time `json:"trialEndDate"`
	
	// 自动续订
	AutoRenew        bool       `json:"autoRenew" gorm:"default:false"`
	NextBillingDate  *time.Time `json:"nextBillingDate"`
	
	// 取消信息
	CanceledAt     *time.Time `json:"canceledAt"`
	CancelReason   string     `json:"cancelReason" gorm:"size:500"`
	CancelFeedback string     `json:"cancelFeedback" gorm:"type:text"`
	
	// 支付信息
	PaymentMethod string  `json:"paymentMethod" gorm:"size:50"` // wechat, alipay, card
	LastPaymentAt *time.Time `json:"lastPaymentAt"`
	TotalPaid     float64 `json:"totalPaid" gorm:"type:decimal(10,2)"`
	
	// 元数据
	Metadata  string `json:"metadata" gorm:"type:jsonb"`
	
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

// SubscriptionHistory 订阅变更历史
type SubscriptionHistory struct {
	ID             string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID       string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID         string    `json:"userId" gorm:"type:uuid;not null;index"`
	SubscriptionID string    `json:"subscriptionId" gorm:"type:uuid;not null;index"`
	Action         string    `json:"action" gorm:"size:50;not null"` // create, upgrade, downgrade, renew, cancel, expire
	FromPlanID     string    `json:"fromPlanId" gorm:"type:uuid"`
	ToPlanID       string    `json:"toPlanId" gorm:"type:uuid"`
	FromStatus     string    `json:"fromStatus" gorm:"size:20"`
	ToStatus       string    `json:"toStatus" gorm:"size:20"`
	Amount         float64   `json:"amount" gorm:"type:decimal(10,2)"`
	Remark         string    `json:"remark" gorm:"size:500"`
	OperatorID     string    `json:"operatorId" gorm:"type:uuid"`
	CreatedAt      time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

// SubscriptionStats 订阅统计
type SubscriptionStats struct {
	TenantID         string  `json:"tenantId"`
	TotalUsers       int64   `json:"totalUsers"`
	ActiveUsers      int64   `json:"activeUsers"`
	TrialingUsers    int64   `json:"trialingUsers"`
	PaidUsers        int64   `json:"paidUsers"`
	ChurnedUsers     int64   `json:"churnedUsers"`
	MRR              float64 `json:"mrr"`              // 月经常性收入
	ARR              float64 `json:"arr"`              // 年经常性收入
	RenewalRate      float64 `json:"renewalRate"`      // 续订率
	ChurnRate        float64 `json:"churnRate"`        // 流失率
	AvgSubscription  float64 `json:"avgSubscription"`  // 平均订阅金额
	PlanDistribution map[string]int64 `json:"planDistribution"` // 各套餐用户数
}

// CreatePlanRequest 创建套餐请求
type CreatePlanRequest struct {
	Name         string       `json:"name" binding:"required"`
	Code         string       `json:"code" binding:"required"`
	Tier         PlanTier     `json:"tier" binding:"required"`
	Description  string       `json:"description"`
	PriceMonthly float64      `json:"priceMonthly"`
	PriceYearly  float64      `json:"priceYearly"`
	Currency     string       `json:"currency"`
	Features     PlanFeatures `json:"features"`
	Permissions  []string     `json:"permissions"`
	TrialDays    int          `json:"trialDays"`
	TrialCredits int64        `json:"trialCredits"`
	IsDefault    bool         `json:"isDefault"`
}

// SubscribeRequest 订阅请求
type SubscribeRequest struct {
	TenantID     string       `json:"tenantId"`
	UserID       string       `json:"userId"`
	PlanID       string       `json:"planId" binding:"required"`
	BillingCycle BillingCycle `json:"billingCycle"`
	AutoRenew    bool         `json:"autoRenew"`
	StartTrial   bool         `json:"startTrial"` // 是否开始试用
}

// CancelRequest 取消订阅请求
type CancelRequest struct {
	TenantID       string `json:"tenantId"`
	UserID         string `json:"userId"`
	SubscriptionID string `json:"subscriptionId"`
	Reason         string `json:"reason"`
	Feedback       string `json:"feedback"`
	Immediate      bool   `json:"immediate"` // 立即取消还是到期取消
}
