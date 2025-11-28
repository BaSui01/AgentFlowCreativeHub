package approval

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApprovalRule 审批规则
type ApprovalRule struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string `json:"tenantId" gorm:"type:uuid;not null;index"`
	WorkflowID  string `json:"workflowId" gorm:"type:uuid;index"` // 空表示全局规则

	// 规则信息
	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`
	Priority    int    `json:"priority" gorm:"default:0"` // 优先级，越高越先匹配

	// 触发条件
	Conditions []RuleCondition `json:"conditions" gorm:"type:jsonb;serializer:json"`
	MatchMode  string          `json:"matchMode" gorm:"size:10;default:'all'"` // all, any

	// 审批动作
	Action     ApprovalAction `json:"action" gorm:"type:jsonb;serializer:json"`
	ActionType string         `json:"actionType" gorm:"size:50;not null"` // auto_approve, auto_reject, assign, escalate, skip

	// 状态
	IsActive bool `json:"isActive" gorm:"default:true"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy string     `json:"createdBy" gorm:"type:uuid"`
}

func (ApprovalRule) TableName() string {
	return "approval_rules"
}

// RuleCondition 规则条件
type RuleCondition struct {
	Field    string `json:"field"`    // 字段名 (如 amount, user_role, department)
	Operator string `json:"operator"` // 操作符 (eq, ne, gt, gte, lt, lte, in, contains, regex)
	Value    any    `json:"value"`    // 比较值
}

// ApprovalAction 审批动作
type ApprovalAction struct {
	// auto_approve / auto_reject
	Reason string `json:"reason,omitempty"` // 自动审批/拒绝原因

	// assign 分配审批人
	AssignTo     []string `json:"assignTo,omitempty"`     // 指定审批人 ID
	AssignToRole string   `json:"assignToRole,omitempty"` // 指定角色

	// escalate 升级
	EscalateTo      string `json:"escalateTo,omitempty"`      // 升级目标
	EscalateAfter   int    `json:"escalateAfter,omitempty"`   // 超时后升级(分钟)
	NotifyOnEscalate bool   `json:"notifyOnEscalate,omitempty"` // 升级时通知

	// 多级审批
	RequiredApprovers int  `json:"requiredApprovers,omitempty"` // 需要的审批人数
	Sequential        bool `json:"sequential,omitempty"`        // 是否顺序审批

	// 超时处理
	TimeoutMinutes int    `json:"timeoutMinutes,omitempty"` // 超时时间
	TimeoutAction  string `json:"timeoutAction,omitempty"`  // 超时动作 (approve, reject, escalate)
}

// RuleEvaluationResult 规则评估结果
type RuleEvaluationResult struct {
	RuleID     string         `json:"ruleId"`
	RuleName   string         `json:"ruleName"`
	Matched    bool           `json:"matched"`
	ActionType string         `json:"actionType"`
	Action     ApprovalAction `json:"action"`
}

// ApprovalRuleEngine 审批规则引擎
type ApprovalRuleEngine struct {
	db *gorm.DB
}

// NewApprovalRuleEngine 创建规则引擎
func NewApprovalRuleEngine(db *gorm.DB) *ApprovalRuleEngine {
	return &ApprovalRuleEngine{db: db}
}

// AutoMigrate 自动迁移
func (e *ApprovalRuleEngine) AutoMigrate() error {
	return e.db.AutoMigrate(&ApprovalRule{})
}

// CreateRule 创建规则
func (e *ApprovalRuleEngine) CreateRule(ctx context.Context, rule *ApprovalRule) error {
	rule.ID = uuid.New().String()
	return e.db.WithContext(ctx).Create(rule).Error
}

// UpdateRule 更新规则
func (e *ApprovalRuleEngine) UpdateRule(ctx context.Context, ruleID, tenantID string, updates map[string]interface{}) error {
	result := e.db.WithContext(ctx).Model(&ApprovalRule{}).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Updates(updates)
	if result.RowsAffected == 0 {
		return fmt.Errorf("规则不存在")
	}
	return result.Error
}

// DeleteRule 删除规则
func (e *ApprovalRuleEngine) DeleteRule(ctx context.Context, ruleID, tenantID string) error {
	now := time.Now()
	result := e.db.WithContext(ctx).Model(&ApprovalRule{}).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Update("deleted_at", now)
	if result.RowsAffected == 0 {
		return fmt.Errorf("规则不存在")
	}
	return result.Error
}

// ListRules 列出规则
func (e *ApprovalRuleEngine) ListRules(ctx context.Context, tenantID string, workflowID string) ([]ApprovalRule, error) {
	var rules []ApprovalRule
	query := e.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID)

	if workflowID != "" {
		query = query.Where("workflow_id = ? OR workflow_id IS NULL OR workflow_id = ''", workflowID)
	}

	err := query.Order("priority DESC, created_at ASC").Find(&rules).Error
	return rules, err
}

// Evaluate 评估请求，返回匹配的规则
func (e *ApprovalRuleEngine) Evaluate(ctx context.Context, tenantID, workflowID string, requestData map[string]any) (*RuleEvaluationResult, error) {
	rules, err := e.ListRules(ctx, tenantID, workflowID)
	if err != nil {
		return nil, fmt.Errorf("获取规则失败: %w", err)
	}

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		matched := e.evaluateConditions(rule.Conditions, rule.MatchMode, requestData)
		if matched {
			return &RuleEvaluationResult{
				RuleID:     rule.ID,
				RuleName:   rule.Name,
				Matched:    true,
				ActionType: rule.ActionType,
				Action:     rule.Action,
			}, nil
		}
	}

	return &RuleEvaluationResult{
		Matched: false,
	}, nil
}

// evaluateConditions 评估条件组
func (e *ApprovalRuleEngine) evaluateConditions(conditions []RuleCondition, matchMode string, data map[string]any) bool {
	if len(conditions) == 0 {
		return true // 没有条件，默认匹配
	}

	for _, cond := range conditions {
		matched := e.evaluateCondition(cond, data)

		if matchMode == "any" && matched {
			return true // any 模式：有一个匹配即可
		}
		if matchMode == "all" && !matched {
			return false // all 模式：有一个不匹配则失败
		}
	}

	if matchMode == "any" {
		return false // any 模式：全部不匹配
	}
	return true // all 模式：全部匹配
}

// evaluateCondition 评估单个条件
func (e *ApprovalRuleEngine) evaluateCondition(cond RuleCondition, data map[string]any) bool {
	fieldValue, exists := e.getFieldValue(cond.Field, data)
	if !exists {
		return false
	}

	switch cond.Operator {
	case "eq", "==", "=":
		return e.compareEqual(fieldValue, cond.Value)
	case "ne", "!=", "<>":
		return !e.compareEqual(fieldValue, cond.Value)
	case "gt", ">":
		return e.compareNumeric(fieldValue, cond.Value) > 0
	case "gte", ">=":
		return e.compareNumeric(fieldValue, cond.Value) >= 0
	case "lt", "<":
		return e.compareNumeric(fieldValue, cond.Value) < 0
	case "lte", "<=":
		return e.compareNumeric(fieldValue, cond.Value) <= 0
	case "in":
		return e.checkIn(fieldValue, cond.Value)
	case "not_in":
		return !e.checkIn(fieldValue, cond.Value)
	case "contains":
		return e.checkContains(fieldValue, cond.Value)
	case "starts_with":
		return strings.HasPrefix(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", cond.Value))
	case "ends_with":
		return strings.HasSuffix(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", cond.Value))
	case "regex":
		return e.checkRegex(fieldValue, cond.Value)
	case "is_null":
		return fieldValue == nil
	case "is_not_null":
		return fieldValue != nil
	default:
		return false
	}
}

// getFieldValue 从数据中获取字段值（支持嵌套字段）
func (e *ApprovalRuleEngine) getFieldValue(field string, data map[string]any) (any, bool) {
	parts := strings.Split(field, ".")

	current := any(data)
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}

// compareEqual 比较相等
func (e *ApprovalRuleEngine) compareEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// compareNumeric 数值比较
func (e *ApprovalRuleEngine) compareNumeric(a, b any) int {
	aFloat := e.toFloat64(a)
	bFloat := e.toFloat64(b)

	if aFloat < bFloat {
		return -1
	}
	if aFloat > bFloat {
		return 1
	}
	return 0
}

// toFloat64 转换为 float64
func (e *ApprovalRuleEngine) toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

// checkIn 检查是否在列表中
func (e *ApprovalRuleEngine) checkIn(value any, list any) bool {
	strValue := fmt.Sprintf("%v", value)

	switch v := list.(type) {
	case []any:
		for _, item := range v {
			if fmt.Sprintf("%v", item) == strValue {
				return true
			}
		}
	case []string:
		for _, item := range v {
			if item == strValue {
				return true
			}
		}
	case string:
		// 逗号分隔的字符串
		items := strings.Split(v, ",")
		for _, item := range items {
			if strings.TrimSpace(item) == strValue {
				return true
			}
		}
	}
	return false
}

// checkContains 检查是否包含
func (e *ApprovalRuleEngine) checkContains(value, substr any) bool {
	return strings.Contains(fmt.Sprintf("%v", value), fmt.Sprintf("%v", substr))
}

// checkRegex 正则匹配
func (e *ApprovalRuleEngine) checkRegex(value, pattern any) bool {
	re, err := regexp.Compile(fmt.Sprintf("%v", pattern))
	if err != nil {
		return false
	}
	return re.MatchString(fmt.Sprintf("%v", value))
}

// ApplyAction 应用规则动作
func (e *ApprovalRuleEngine) ApplyAction(ctx context.Context, result *RuleEvaluationResult, request *ApprovalRequest) error {
	if !result.Matched {
		return nil
	}

	switch result.ActionType {
	case "auto_approve":
		// 自动批准
		request.Status = "approved"
		request.AutoApproved = true
		request.ApprovalReason = result.Action.Reason
		if request.ApprovalReason == "" {
			request.ApprovalReason = fmt.Sprintf("规则自动批准: %s", result.RuleName)
		}

	case "auto_reject":
		// 自动拒绝
		request.Status = "rejected"
		request.AutoApproved = true
		request.ApprovalReason = result.Action.Reason
		if request.ApprovalReason == "" {
			request.ApprovalReason = fmt.Sprintf("规则自动拒绝: %s", result.RuleName)
		}

	case "assign":
		// 分配审批人
		if len(result.Action.AssignTo) > 0 {
			request.AssignedTo = result.Action.AssignTo
		}

	case "escalate":
		// 升级处理
		request.Escalated = true
		request.EscalateTo = result.Action.EscalateTo

	case "skip":
		// 跳过审批
		request.Status = "skipped"
		request.SkippedReason = fmt.Sprintf("规则跳过: %s", result.RuleName)
	}

	// 设置超时
	if result.Action.TimeoutMinutes > 0 {
		timeout := time.Now().Add(time.Duration(result.Action.TimeoutMinutes) * time.Minute)
		request.TimeoutAt = &timeout
		request.TimeoutAction = result.Action.TimeoutAction
	}

	return nil
}

// ApprovalRequest 审批请求扩展字段
type ApprovalRequest struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	AutoApproved   bool       `json:"autoApproved"`
	ApprovalReason string     `json:"approvalReason"`
	AssignedTo     []string   `json:"assignedTo"`
	Escalated      bool       `json:"escalated"`
	EscalateTo     string     `json:"escalateTo"`
	SkippedReason  string     `json:"skippedReason"`
	TimeoutAt      *time.Time `json:"timeoutAt"`
	TimeoutAction  string     `json:"timeoutAction"`
}

// ToJSON 序列化为 JSON
func (r *ApprovalRule) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON 从 JSON 反序列化
func (r *ApprovalRule) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}
