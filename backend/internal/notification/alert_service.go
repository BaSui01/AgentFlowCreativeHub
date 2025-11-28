package notification

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AlertService 告警服务
type AlertService struct {
	rules       map[string]*AlertRule
	mu          sync.RWMutex
	evaluator   *AlertEvaluator
	notifiers   []AlertNotifier
	silences    map[string]*AlertSilence
	history     []*AlertEvent
	historyMu   sync.RWMutex
	maxHistory  int
	stopCh      chan struct{}
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Metric      string            `json:"metric"`       // 指标名称
	Condition   AlertCondition    `json:"condition"`    // 触发条件
	Duration    time.Duration     `json:"duration"`     // 持续时间
	Severity    AlertSeverity     `json:"severity"`     // 严重级别
	Labels      map[string]string `json:"labels"`       // 标签
	Annotations map[string]string `json:"annotations"`  // 注解
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	// 运行时状态
	State         AlertState    `json:"state"`
	LastEval      time.Time     `json:"last_eval"`
	LastFired     time.Time     `json:"last_fired,omitempty"`
	FiringStarted time.Time     `json:"-"`
}

// AlertCondition 告警条件
type AlertCondition struct {
	Operator  string  `json:"operator"`  // gt, lt, eq, ne, gte, lte
	Threshold float64 `json:"threshold"` // 阈值
}

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"
)

// AlertState 告警状态
type AlertState string

const (
	StateInactive AlertState = "inactive"
	StatePending  AlertState = "pending"
	StateFiring   AlertState = "firing"
)

// AlertEvent 告警事件
type AlertEvent struct {
	ID         string            `json:"id"`
	RuleID     string            `json:"rule_id"`
	RuleName   string            `json:"rule_name"`
	State      AlertState        `json:"state"`
	Severity   AlertSeverity     `json:"severity"`
	Value      float64           `json:"value"`
	Threshold  float64           `json:"threshold"`
	Labels     map[string]string `json:"labels"`
	Message    string            `json:"message"`
	FiredAt    time.Time         `json:"fired_at"`
	ResolvedAt *time.Time        `json:"resolved_at,omitempty"`
}

// AlertSilence 告警静默
type AlertSilence struct {
	ID        string            `json:"id"`
	Matchers  map[string]string `json:"matchers"` // 标签匹配
	StartsAt  time.Time         `json:"starts_at"`
	EndsAt    time.Time         `json:"ends_at"`
	Comment   string            `json:"comment"`
	CreatedBy string            `json:"created_by"`
	CreatedAt time.Time         `json:"created_at"`
}

// AlertNotifier 告警通知器接口
type AlertNotifier interface {
	Name() string
	Notify(ctx context.Context, event *AlertEvent) error
}

// MetricsProvider 指标提供者接口
type MetricsProvider interface {
	GetMetricValue(name string) (float64, error)
}

// AlertEvaluator 告警评估器
type AlertEvaluator struct {
	provider MetricsProvider
}

// NewAlertService 创建告警服务
func NewAlertService(provider MetricsProvider) *AlertService {
	as := &AlertService{
		rules:      make(map[string]*AlertRule),
		evaluator:  &AlertEvaluator{provider: provider},
		notifiers:  make([]AlertNotifier, 0),
		silences:   make(map[string]*AlertSilence),
		history:    make([]*AlertEvent, 0),
		maxHistory: 1000,
		stopCh:     make(chan struct{}),
	}

	return as
}

// Start 启动告警评估
func (s *AlertService) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.evaluate()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop 停止告警服务
func (s *AlertService) Stop() {
	close(s.stopCh)
}

// AddRule 添加告警规则
func (s *AlertService) AddRule(rule *AlertRule) error {
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
	}
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if rule.Metric == "" {
		return fmt.Errorf("metric is required")
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.State = StateInactive
	rule.Enabled = true

	s.mu.Lock()
	s.rules[rule.ID] = rule
	s.mu.Unlock()

	return nil
}

// UpdateRule 更新规则
func (s *AlertService) UpdateRule(id string, update *AlertRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.rules[id]
	if !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Description != "" {
		existing.Description = update.Description
	}
	if update.Metric != "" {
		existing.Metric = update.Metric
	}
	if update.Duration > 0 {
		existing.Duration = update.Duration
	}
	if update.Severity != "" {
		existing.Severity = update.Severity
	}
	existing.Condition = update.Condition
	existing.UpdatedAt = time.Now()

	return nil
}

// DeleteRule 删除规则
func (s *AlertService) DeleteRule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	delete(s.rules, id)
	return nil
}

// EnableRule 启用/禁用规则
func (s *AlertService) EnableRule(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, ok := s.rules[id]
	if !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	rule.Enabled = enabled
	rule.UpdatedAt = time.Now()
	return nil
}

// ListRules 列出所有规则
func (s *AlertService) ListRules() []*AlertRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AlertRule, 0, len(s.rules))
	for _, rule := range s.rules {
		result = append(result, rule)
	}
	return result
}

// AddNotifier 添加通知器
func (s *AlertService) AddNotifier(notifier AlertNotifier) {
	s.notifiers = append(s.notifiers, notifier)
}

// AddSilence 添加静默
func (s *AlertService) AddSilence(silence *AlertSilence) error {
	if silence.ID == "" {
		silence.ID = fmt.Sprintf("silence_%d", time.Now().UnixNano())
	}
	if silence.EndsAt.Before(time.Now()) {
		return fmt.Errorf("silence end time must be in the future")
	}

	silence.CreatedAt = time.Now()

	s.mu.Lock()
	s.silences[silence.ID] = silence
	s.mu.Unlock()

	return nil
}

// DeleteSilence 删除静默
func (s *AlertService) DeleteSilence(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.silences[id]; !ok {
		return fmt.Errorf("silence not found: %s", id)
	}

	delete(s.silences, id)
	return nil
}

// ListSilences 列出静默
func (s *AlertService) ListSilences() []*AlertSilence {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AlertSilence, 0)
	now := time.Now()
	for _, silence := range s.silences {
		if silence.EndsAt.After(now) {
			result = append(result, silence)
		}
	}
	return result
}

// GetHistory 获取告警历史
func (s *AlertService) GetHistory(limit int) []*AlertEvent {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}

	// 返回最近的事件
	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*AlertEvent, limit)
	copy(result, s.history[start:])
	return result
}

// evaluate 评估所有规则
func (s *AlertService) evaluate() {
	s.mu.Lock()
	rules := make([]*AlertRule, 0, len(s.rules))
	for _, rule := range s.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	s.mu.Unlock()

	for _, rule := range rules {
		s.evaluateRule(rule)
	}
}

func (s *AlertService) evaluateRule(rule *AlertRule) {
	value, err := s.evaluator.provider.GetMetricValue(rule.Metric)
	if err != nil {
		return
	}

	rule.LastEval = time.Now()
	firing := s.checkCondition(rule.Condition, value)

	switch rule.State {
	case StateInactive:
		if firing {
			rule.State = StatePending
			rule.FiringStarted = time.Now()
		}

	case StatePending:
		if !firing {
			rule.State = StateInactive
			rule.FiringStarted = time.Time{}
		} else if time.Since(rule.FiringStarted) >= rule.Duration {
			rule.State = StateFiring
			rule.LastFired = time.Now()
			s.fireAlert(rule, value)
		}

	case StateFiring:
		if !firing {
			rule.State = StateInactive
			rule.FiringStarted = time.Time{}
			s.resolveAlert(rule)
		}
	}
}

func (s *AlertService) checkCondition(cond AlertCondition, value float64) bool {
	switch cond.Operator {
	case "gt", ">":
		return value > cond.Threshold
	case "lt", "<":
		return value < cond.Threshold
	case "gte", ">=":
		return value >= cond.Threshold
	case "lte", "<=":
		return value <= cond.Threshold
	case "eq", "==":
		return value == cond.Threshold
	case "ne", "!=":
		return value != cond.Threshold
	default:
		return false
	}
}

func (s *AlertService) fireAlert(rule *AlertRule, value float64) {
	event := &AlertEvent{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		State:     StateFiring,
		Severity:  rule.Severity,
		Value:     value,
		Threshold: rule.Condition.Threshold,
		Labels:    rule.Labels,
		Message:   fmt.Sprintf("%s: %s %.2f %s %.2f", rule.Name, rule.Metric, value, rule.Condition.Operator, rule.Condition.Threshold),
		FiredAt:   time.Now(),
	}

	// 检查是否被静默
	if s.isSilenced(rule) {
		return
	}

	// 记录历史
	s.addHistory(event)

	// 发送通知
	ctx := context.Background()
	for _, notifier := range s.notifiers {
		go notifier.Notify(ctx, event)
	}
}

func (s *AlertService) resolveAlert(rule *AlertRule) {
	now := time.Now()
	event := &AlertEvent{
		ID:         fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		RuleID:     rule.ID,
		RuleName:   rule.Name,
		State:      StateInactive,
		Severity:   rule.Severity,
		Labels:     rule.Labels,
		Message:    fmt.Sprintf("%s: 已恢复", rule.Name),
		FiredAt:    rule.LastFired,
		ResolvedAt: &now,
	}

	s.addHistory(event)
}

func (s *AlertService) isSilenced(rule *AlertRule) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	for _, silence := range s.silences {
		if silence.StartsAt.After(now) || silence.EndsAt.Before(now) {
			continue
		}

		// 检查标签匹配
		match := true
		for k, v := range silence.Matchers {
			if rule.Labels[k] != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (s *AlertService) addHistory(event *AlertEvent) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	s.history = append(s.history, event)

	// 限制历史记录数量
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}
}

// ========== 内置通知器 ==========

// WebhookAlertNotifier Webhook 告警通知器
type WebhookAlertNotifier struct {
	webhook *WebhookService
}

func NewWebhookAlertNotifier(webhook *WebhookService) *WebhookAlertNotifier {
	return &WebhookAlertNotifier{webhook: webhook}
}

func (n *WebhookAlertNotifier) Name() string {
	return "webhook"
}

func (n *WebhookAlertNotifier) Notify(ctx context.Context, event *AlertEvent) error {
	return n.webhook.Emit(ctx, "alert.fired", map[string]any{
		"alert_id":  event.ID,
		"rule_id":   event.RuleID,
		"rule_name": event.RuleName,
		"severity":  event.Severity,
		"value":     event.Value,
		"threshold": event.Threshold,
		"message":   event.Message,
		"labels":    event.Labels,
		"fired_at":  event.FiredAt,
	})
}

// LogAlertNotifier 日志告警通知器（用于测试）
type LogAlertNotifier struct{}

func (n *LogAlertNotifier) Name() string {
	return "log"
}

func (n *LogAlertNotifier) Notify(ctx context.Context, event *AlertEvent) error {
	fmt.Printf("[ALERT] %s: %s (severity=%s, value=%.2f)\n",
		event.RuleName, event.Message, event.Severity, event.Value)
	return nil
}
