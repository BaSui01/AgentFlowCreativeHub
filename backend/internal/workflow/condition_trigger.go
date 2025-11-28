package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConditionTrigger 条件触发器模型
type ConditionTrigger struct {
	ID          string          `json:"id" gorm:"primaryKey;size:36"`
	TenantID    string          `json:"tenantId" gorm:"size:36;index"`
	WorkflowID  string          `json:"workflowId" gorm:"size:36;index"`
	Name        string          `json:"name" gorm:"size:100"`
	Description string          `json:"description" gorm:"size:500"`
	Enabled     bool            `json:"enabled" gorm:"default:true"`
	Conditions  json.RawMessage `json:"conditions" gorm:"type:jsonb"` // 触发条件
	Actions     json.RawMessage `json:"actions" gorm:"type:jsonb"`    // 触发动作
	Cooldown    int             `json:"cooldown"`                     // 冷却时间（秒）
	LastFiredAt *time.Time      `json:"lastFiredAt"`
	FireCount   int64           `json:"fireCount"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// TriggerCondition 触发条件
type TriggerCondition struct {
	Type     string      `json:"type"`     // field_change, threshold, schedule, composite
	Field    string      `json:"field"`    // 监控的字段路径
	Operator string      `json:"operator"` // eq, ne, gt, lt, gte, lte, contains, regex, changed
	Value    interface{} `json:"value"`    // 比较值
	Children []TriggerCondition `json:"children,omitempty"` // 复合条件
	Logic    string      `json:"logic,omitempty"`    // and, or (复合条件时)
}

// TriggerAction 触发动作
type TriggerAction struct {
	Type       string                 `json:"type"`       // execute_workflow, send_notification, call_webhook
	Target     string                 `json:"target"`     // 目标ID
	Parameters map[string]interface{} `json:"parameters"` // 动作参数
}

// ConditionTriggerService 条件触发器服务
type ConditionTriggerService struct {
	db          *gorm.DB
	mu          sync.RWMutex
	subscribers map[string][]chan DataChangeEvent // 订阅者
}

// DataChangeEvent 数据变更事件
type DataChangeEvent struct {
	TenantID   string                 `json:"tenantId"`
	EntityType string                 `json:"entityType"` // workspace_file, agent_config, workflow, etc.
	EntityID   string                 `json:"entityId"`
	Operation  string                 `json:"operation"` // create, update, delete
	OldData    map[string]interface{} `json:"oldData,omitempty"`
	NewData    map[string]interface{} `json:"newData,omitempty"`
	UserID     string                 `json:"userId"`
	Timestamp  time.Time              `json:"timestamp"`
}

// NewConditionTriggerService 创建条件触发器服务
func NewConditionTriggerService(db *gorm.DB) *ConditionTriggerService {
	return &ConditionTriggerService{
		db:          db,
		subscribers: make(map[string][]chan DataChangeEvent),
	}
}

// CreateTrigger 创建触发器
func (s *ConditionTriggerService) CreateTrigger(ctx context.Context, trigger *ConditionTrigger) error {
	if trigger.ID == "" {
		trigger.ID = uuid.New().String()
	}
	trigger.CreatedAt = time.Now()
	trigger.UpdatedAt = time.Now()

	return s.db.WithContext(ctx).Create(trigger).Error
}

// UpdateTrigger 更新触发器
func (s *ConditionTriggerService) UpdateTrigger(ctx context.Context, trigger *ConditionTrigger) error {
	trigger.UpdatedAt = time.Now()
	return s.db.WithContext(ctx).Save(trigger).Error
}

// DeleteTrigger 删除触发器
func (s *ConditionTriggerService) DeleteTrigger(ctx context.Context, id, tenantID string) error {
	return s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&ConditionTrigger{}).Error
}

// GetTrigger 获取触发器
func (s *ConditionTriggerService) GetTrigger(ctx context.Context, id, tenantID string) (*ConditionTrigger, error) {
	var trigger ConditionTrigger
	err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&trigger).Error
	if err != nil {
		return nil, err
	}
	return &trigger, nil
}

// ListTriggers 列出触发器
func (s *ConditionTriggerService) ListTriggers(ctx context.Context, tenantID string, workflowID string) ([]*ConditionTrigger, error) {
	var triggers []*ConditionTrigger
	query := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if workflowID != "" {
		query = query.Where("workflow_id = ?", workflowID)
	}
	err := query.Order("created_at DESC").Find(&triggers).Error
	return triggers, err
}

// ListEnabledTriggers 列出启用的触发器
func (s *ConditionTriggerService) ListEnabledTriggers(ctx context.Context, tenantID string) ([]*ConditionTrigger, error) {
	var triggers []*ConditionTrigger
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND enabled = ?", tenantID, true).
		Find(&triggers).Error
	return triggers, err
}

// PublishEvent 发布数据变更事件
func (s *ConditionTriggerService) PublishEvent(ctx context.Context, event DataChangeEvent) error {
	// 获取该租户的所有启用触发器
	triggers, err := s.ListEnabledTriggers(ctx, event.TenantID)
	if err != nil {
		return err
	}

	// 检查每个触发器
	for _, trigger := range triggers {
		if s.shouldFire(trigger, event) {
			go s.fireTrigger(ctx, trigger, event)
		}
	}

	// 通知订阅者
	s.mu.RLock()
	key := event.TenantID + ":" + event.EntityType
	if subs, ok := s.subscribers[key]; ok {
		for _, ch := range subs {
			select {
			case ch <- event:
			default:
				// 通道满了，跳过
			}
		}
	}
	s.mu.RUnlock()

	return nil
}

// Subscribe 订阅数据变更
func (s *ConditionTriggerService) Subscribe(tenantID, entityType string) <-chan DataChangeEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan DataChangeEvent, 100)
	key := tenantID + ":" + entityType
	s.subscribers[key] = append(s.subscribers[key], ch)
	return ch
}

// Unsubscribe 取消订阅
func (s *ConditionTriggerService) Unsubscribe(tenantID, entityType string, ch <-chan DataChangeEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tenantID + ":" + entityType
	subs := s.subscribers[key]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[key] = append(subs[:i], subs[i+1:]...)
			close(sub)
			break
		}
	}
}

// shouldFire 检查是否应该触发
func (s *ConditionTriggerService) shouldFire(trigger *ConditionTrigger, event DataChangeEvent) bool {
	// 检查冷却时间
	if trigger.LastFiredAt != nil && trigger.Cooldown > 0 {
		cooldownEnd := trigger.LastFiredAt.Add(time.Duration(trigger.Cooldown) * time.Second)
		if time.Now().Before(cooldownEnd) {
			return false
		}
	}

	// 解析条件
	var conditions []TriggerCondition
	if err := json.Unmarshal(trigger.Conditions, &conditions); err != nil {
		return false
	}

	// 检查所有条件 (默认 AND 逻辑)
	for _, cond := range conditions {
		if !s.evaluateCondition(cond, event) {
			return false
		}
	}

	return true
}

// evaluateCondition 评估单个条件
func (s *ConditionTriggerService) evaluateCondition(cond TriggerCondition, event DataChangeEvent) bool {
	switch cond.Type {
	case "field_change":
		return s.evaluateFieldChange(cond, event)
	case "threshold":
		return s.evaluateThreshold(cond, event)
	case "composite":
		return s.evaluateComposite(cond, event)
	case "entity_type":
		return event.EntityType == cond.Value
	case "operation":
		return event.Operation == cond.Value
	default:
		return false
	}
}

// evaluateFieldChange 评估字段变更条件
func (s *ConditionTriggerService) evaluateFieldChange(cond TriggerCondition, event DataChangeEvent) bool {
	oldValue := s.getFieldValue(event.OldData, cond.Field)
	newValue := s.getFieldValue(event.NewData, cond.Field)

	switch cond.Operator {
	case "changed":
		return oldValue != newValue
	case "eq":
		return newValue == cond.Value
	case "ne":
		return newValue != cond.Value
	case "contains":
		if str, ok := newValue.(string); ok {
			if pattern, ok := cond.Value.(string); ok {
				return strings.Contains(str, pattern)
			}
		}
		return false
	case "regex":
		if str, ok := newValue.(string); ok {
			if pattern, ok := cond.Value.(string); ok {
				matched, _ := regexp.MatchString(pattern, str)
				return matched
			}
		}
		return false
	default:
		return false
	}
}

// evaluateThreshold 评估阈值条件
func (s *ConditionTriggerService) evaluateThreshold(cond TriggerCondition, event DataChangeEvent) bool {
	value := s.getFieldValue(event.NewData, cond.Field)
	
	numValue, ok := toFloat64(value)
	if !ok {
		return false
	}
	
	threshold, ok := toFloat64(cond.Value)
	if !ok {
		return false
	}

	switch cond.Operator {
	case "gt":
		return numValue > threshold
	case "gte":
		return numValue >= threshold
	case "lt":
		return numValue < threshold
	case "lte":
		return numValue <= threshold
	case "eq":
		return numValue == threshold
	default:
		return false
	}
}

// evaluateComposite 评估复合条件
func (s *ConditionTriggerService) evaluateComposite(cond TriggerCondition, event DataChangeEvent) bool {
	if len(cond.Children) == 0 {
		return true
	}

	switch cond.Logic {
	case "or":
		for _, child := range cond.Children {
			if s.evaluateCondition(child, event) {
				return true
			}
		}
		return false
	default: // "and"
		for _, child := range cond.Children {
			if !s.evaluateCondition(child, event) {
				return false
			}
		}
		return true
	}
}

// getFieldValue 获取嵌套字段值
func (s *ConditionTriggerService) getFieldValue(data map[string]interface{}, path string) interface{} {
	if data == nil {
		return nil
	}

	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

// fireTrigger 触发工作流
func (s *ConditionTriggerService) fireTrigger(ctx context.Context, trigger *ConditionTrigger, event DataChangeEvent) {
	// 解析动作
	var actions []TriggerAction
	if err := json.Unmarshal(trigger.Actions, &actions); err != nil {
		return
	}

	// 执行每个动作
	for _, action := range actions {
		s.executeAction(ctx, action, trigger, event)
	}

	// 更新触发器状态
	now := time.Now()
	s.db.WithContext(ctx).
		Model(&ConditionTrigger{}).
		Where("id = ?", trigger.ID).
		Updates(map[string]interface{}{
			"last_fired_at": now,
			"fire_count":    gorm.Expr("fire_count + 1"),
		})
}

// executeAction 执行触发动作
func (s *ConditionTriggerService) executeAction(ctx context.Context, action TriggerAction, trigger *ConditionTrigger, event DataChangeEvent) {
	switch action.Type {
	case "execute_workflow":
		// 触发工作流执行
		// 需要注入 WorkflowEngine 或通过消息队列
		fmt.Printf("Trigger workflow %s for trigger %s\n", action.Target, trigger.ID)
	case "send_notification":
		// 发送通知
		fmt.Printf("Send notification for trigger %s\n", trigger.ID)
	case "call_webhook":
		// 调用 Webhook
		fmt.Printf("Call webhook %s for trigger %s\n", action.Target, trigger.ID)
	}
}

// toFloat64 转换为 float64
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// TriggerLog 触发日志
type TriggerLog struct {
	ID         string          `json:"id" gorm:"primaryKey;size:36"`
	TriggerID  string          `json:"triggerId" gorm:"size:36;index"`
	TenantID   string          `json:"tenantId" gorm:"size:36;index"`
	Event      json.RawMessage `json:"event" gorm:"type:jsonb"`
	Result     string          `json:"result"` // success, failed
	Error      string          `json:"error,omitempty"`
	ExecutedAt time.Time       `json:"executedAt"`
}

// LogTriggerExecution 记录触发执行
func (s *ConditionTriggerService) LogTriggerExecution(ctx context.Context, trigger *ConditionTrigger, event DataChangeEvent, result string, err error) {
	eventData, _ := json.Marshal(event)
	
	log := &TriggerLog{
		ID:         uuid.New().String(),
		TriggerID:  trigger.ID,
		TenantID:   trigger.TenantID,
		Event:      eventData,
		Result:     result,
		ExecutedAt: time.Now(),
	}
	
	if err != nil {
		log.Error = err.Error()
	}
	
	s.db.WithContext(ctx).Create(log)
}

// GetTriggerLogs 获取触发日志
func (s *ConditionTriggerService) GetTriggerLogs(ctx context.Context, triggerID, tenantID string, limit int) ([]*TriggerLog, error) {
	var logs []*TriggerLog
	err := s.db.WithContext(ctx).
		Where("trigger_id = ? AND tenant_id = ?", triggerID, tenantID).
		Order("executed_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
