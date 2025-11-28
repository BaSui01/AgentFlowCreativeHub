package moderation

import (
	"time"
)

// ============================================================================
// 审核任务
// ============================================================================

// ModerationTask 审核任务
type ModerationTask struct {
	ID          string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	
	// 内容信息
	ContentType string    `json:"contentType" gorm:"size:50;not null;index"` // chapter, comment, profile, etc.
	ContentID   string    `json:"contentId" gorm:"type:uuid;not null;index"` // 关联的内容ID
	Title       string    `json:"title" gorm:"size:255"`                      // 内容标题
	Content     string    `json:"content" gorm:"type:text;not null"`          // 待审核内容
	ContentMeta map[string]any `json:"contentMeta" gorm:"type:jsonb;serializer:json"` // 内容元信息
	
	// 提交者信息
	SubmitterID   string  `json:"submitterId" gorm:"type:uuid;not null;index"`
	SubmitterName string  `json:"submitterName" gorm:"size:100"`
	
	// 审核状态
	Status      TaskStatus `json:"status" gorm:"size:20;not null;default:pending;index"`
	Priority    int        `json:"priority" gorm:"default:0;index"`           // 优先级：0-低，1-中，2-高
	
	// AI 预审结果
	AIReviewed   bool       `json:"aiReviewed" gorm:"default:false"`
	AIRiskLevel  string     `json:"aiRiskLevel" gorm:"size:20"`              // safe, low, medium, high
	AIRiskScore  float64    `json:"aiRiskScore" gorm:"type:decimal(5,2)"`    // 0-100
	AIFindings   []AIFinding `json:"aiFindings" gorm:"-"`
	AIFindingsJSON string   `json:"-" gorm:"column:ai_findings;type:jsonb"`
	
	// 当前审核级别
	CurrentLevel int       `json:"currentLevel" gorm:"default:1"`            // 当前审核级别（初审=1，复审=2...）
	MaxLevel     int       `json:"maxLevel" gorm:"default:1"`                // 最大审核级别
	
	// 分配信息
	AssignedTo   string    `json:"assignedTo" gorm:"type:uuid;index"`        // 分配给的审核员
	AssignedAt   *time.Time `json:"assignedAt"`
	
	// 截止时间
	Deadline     *time.Time `json:"deadline"`
	
	// 时间戳
	CreatedAt    time.Time  `json:"createdAt" gorm:"not null;autoCreateTime;index"`
	UpdatedAt    time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (ModerationTask) TableName() string {
	return "moderation_tasks"
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 待审核
	TaskStatusReviewing TaskStatus = "reviewing" // 审核中
	TaskStatusApproved  TaskStatus = "approved"  // 已通过
	TaskStatusRejected  TaskStatus = "rejected"  // 已拒绝
	TaskStatusRevision  TaskStatus = "revision"  // 需修改
	TaskStatusEscalated TaskStatus = "escalated" // 已升级
)

// AIFinding AI 发现的问题
type AIFinding struct {
	Category    string  `json:"category"`    // 问题类别
	Description string  `json:"description"` // 问题描述
	Severity    string  `json:"severity"`    // 严重程度
	Position    string  `json:"position"`    // 问题位置
	Suggestion  string  `json:"suggestion"`  // 修改建议
	Confidence  float64 `json:"confidence"`  // 置信度
}

// ============================================================================
// 审核记录
// ============================================================================

// ModerationRecord 审核记录（每次审核操作）
type ModerationRecord struct {
	ID        string     `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string     `json:"tenantId" gorm:"type:uuid;not null;index"`
	TaskID    string     `json:"taskId" gorm:"type:uuid;not null;index"`
	
	// 审核员信息
	ReviewerID   string  `json:"reviewerId" gorm:"type:uuid;not null;index"`
	ReviewerName string  `json:"reviewerName" gorm:"size:100"`
	
	// 审核级别
	Level     int        `json:"level" gorm:"not null"`                   // 审核级别
	
	// 审核结果
	Action    ReviewAction `json:"action" gorm:"size:20;not null"`        // 审核动作
	Decision  string       `json:"decision" gorm:"size:20;not null"`      // 决定：approve, reject, revision, escalate
	
	// 审核意见
	Comment   string     `json:"comment" gorm:"type:text"`                // 审核意见
	Reason    string     `json:"reason" gorm:"size:255"`                  // 拒绝/修改原因
	Tags      []string   `json:"tags" gorm:"-"`                           // 问题标签
	TagsJSON  string     `json:"-" gorm:"column:tags;type:jsonb"`
	
	// 违规信息
	ViolationType string `json:"violationType" gorm:"size:100"`           // 违规类型
	ViolationLevel string `json:"violationLevel" gorm:"size:20"`          // 违规级别
	
	// 处理措施
	Punishment  string   `json:"punishment" gorm:"size:50"`               // 处理措施：warning, takedown, suspend, ban
	
	// 时间
	ReviewedAt time.Time `json:"reviewedAt" gorm:"not null;autoCreateTime"`
}

// TableName 指定表名
func (ModerationRecord) TableName() string {
	return "moderation_records"
}

// ReviewAction 审核动作
type ReviewAction string

const (
	ActionApprove  ReviewAction = "approve"  // 通过
	ActionReject   ReviewAction = "reject"   // 拒绝
	ActionRevision ReviewAction = "revision" // 要求修改
	ActionEscalate ReviewAction = "escalate" // 升级到上级
	ActionReassign ReviewAction = "reassign" // 重新分配
)

// ============================================================================
// 敏感词库
// ============================================================================

// SensitiveWord 敏感词
type SensitiveWord struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	Word      string    `json:"word" gorm:"size:100;not null;index"`
	Category  string    `json:"category" gorm:"size:50;not null;index"`   // 类别：politics, porn, violence, etc.
	Level     string    `json:"level" gorm:"size:20;not null;default:medium"` // 级别：low, medium, high
	Action    string    `json:"action" gorm:"size:20;default:flag"`       // 动作：flag, replace, block
	Replace   string    `json:"replace" gorm:"size:100"`                  // 替换词
	IsActive  bool      `json:"isActive" gorm:"default:true"`
	CreatedBy string    `json:"createdBy" gorm:"type:uuid"`
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (SensitiveWord) TableName() string {
	return "sensitive_words"
}

// SensitiveWordCategory 敏感词类别
const (
	CategoryPolitics   = "politics"   // 政治敏感
	CategoryPorn       = "porn"       // 色情低俗
	CategoryViolence   = "violence"   // 暴力血腥
	CategoryGambling   = "gambling"   // 赌博
	CategoryDrug       = "drug"       // 毒品
	CategoryFraud      = "fraud"      // 诈骗
	CategoryAd         = "ad"         // 广告
	CategoryAbuse      = "abuse"      // 辱骂
	CategoryCustom     = "custom"     // 自定义
)

// ============================================================================
// 审核标准配置
// ============================================================================

// ModerationRule 审核规则
type ModerationRule struct {
	ID          string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	Description string    `json:"description" gorm:"size:500"`
	ContentType string    `json:"contentType" gorm:"size:50;not null;index"` // 适用内容类型
	
	// 规则配置
	RuleType    string    `json:"ruleType" gorm:"size:50;not null"`        // keyword, regex, ai, length
	Condition   string    `json:"condition" gorm:"type:text"`              // 规则条件
	Action      string    `json:"action" gorm:"size:20;not null"`          // flag, block, auto_reject
	Priority    int       `json:"priority" gorm:"default:0"`
	
	// AI 审核配置
	AIEnabled   bool      `json:"aiEnabled" gorm:"default:false"`
	AIThreshold float64   `json:"aiThreshold" gorm:"type:decimal(5,2);default:0.8"` // AI 风险阈值
	
	// 多级审核配置
	RequireLevels int     `json:"requireLevels" gorm:"default:1"`          // 需要的审核级别数
	
	IsActive    bool      `json:"isActive" gorm:"default:true"`
	CreatedAt   time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (ModerationRule) TableName() string {
	return "moderation_rules"
}

// ============================================================================
// 请求/响应类型
// ============================================================================

// SubmitContentRequest 提交审核请求
type SubmitContentRequest struct {
	ContentType string         `json:"contentType" binding:"required"`
	ContentID   string         `json:"contentId" binding:"required"`
	Title       string         `json:"title"`
	Content     string         `json:"content" binding:"required"`
	ContentMeta map[string]any `json:"contentMeta"`
	Priority    int            `json:"priority"`
}

// ReviewRequest 审核请求
type ReviewRequest struct {
	TaskID    string       `json:"taskId" binding:"required"`
	Action    ReviewAction `json:"action" binding:"required"`
	Comment   string       `json:"comment"`
	Reason    string       `json:"reason"`
	Tags      []string     `json:"tags"`
	Punishment string      `json:"punishment"`
}

// TaskQuery 任务查询
type TaskQuery struct {
	TenantID    string     `json:"tenantId"`
	Status      TaskStatus `json:"status"`
	ContentType string     `json:"contentType"`
	AssignedTo  string     `json:"assignedTo"`
	SubmitterID string     `json:"submitterId"`
	AIRiskLevel string     `json:"aiRiskLevel"`
	StartTime   *time.Time `json:"startTime"`
	EndTime     *time.Time `json:"endTime"`
	Page        int        `json:"page"`
	PageSize    int        `json:"pageSize"`
}

// BatchWordRequest 批量敏感词请求
type BatchWordRequest struct {
	Words    []string `json:"words" binding:"required"`
	Category string   `json:"category" binding:"required"`
	Level    string   `json:"level"`
	Action   string   `json:"action"`
}

// FilterResult 过滤结果
type FilterResult struct {
	Original    string        `json:"original"`    // 原始内容
	Filtered    string        `json:"filtered"`    // 过滤后内容
	HasSensitive bool         `json:"hasSensitive"` // 是否包含敏感词
	Matches     []MatchedWord `json:"matches"`     // 匹配的敏感词
}

// MatchedWord 匹配的敏感词
type MatchedWord struct {
	Word     string `json:"word"`
	Category string `json:"category"`
	Level    string `json:"level"`
	Position int    `json:"position"`
	Action   string `json:"action"`
}

// ModerationStats 审核统计
type ModerationStats struct {
	TenantID       string    `json:"tenantId"`
	Period         string    `json:"period"`          // daily, weekly, monthly
	TotalTasks     int64     `json:"totalTasks"`
	PendingTasks   int64     `json:"pendingTasks"`
	ApprovedTasks  int64     `json:"approvedTasks"`
	RejectedTasks  int64     `json:"rejectedTasks"`
	AvgReviewTime  float64   `json:"avgReviewTime"`   // 平均审核时间（小时）
	AIReviewRate   float64   `json:"aiReviewRate"`    // AI 预审率
	EscalationRate float64   `json:"escalationRate"`  // 升级率
	ByContentType  map[string]int64 `json:"byContentType"`
	ByRiskLevel    map[string]int64 `json:"byRiskLevel"`
	StartDate      time.Time `json:"startDate"`
	EndDate        time.Time `json:"endDate"`
}
