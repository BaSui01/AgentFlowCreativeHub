package approval

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"backend/internal/logger"
	"backend/internal/metrics"
	"backend/internal/notification"
	"backend/internal/tenant"
	workflowpkg "backend/internal/workflow"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Manager 审批管理器
type Manager struct {
	db       *gorm.DB
	notifier *notification.MultiNotifier
	resolver TargetResolver
	logger   *zap.Logger
	eventBus *ApprovalEventBus
}

// ManagerOption 自定义配置
type ManagerOption func(*Manager)

// WithNotifier 注入通知器
func WithNotifier(notifier *notification.MultiNotifier) ManagerOption {
	return func(m *Manager) { m.notifier = notifier }
}

// WithTargetResolver 注入目标解析器
func WithTargetResolver(resolver TargetResolver) ManagerOption {
	return func(m *Manager) { m.resolver = resolver }
}

// WithEventBus 注入事件总线
func WithEventBus(bus *ApprovalEventBus) ManagerOption {
	return func(m *Manager) { m.eventBus = bus }
}

// WithManagerLogger 注入自定义日志器
func WithManagerLogger(l *zap.Logger) ManagerOption {
	return func(m *Manager) { m.logger = l }
}

// NewManager 创建审批管理器
func NewManager(db *gorm.DB, opts ...ManagerOption) *Manager {
	mgr := &Manager{
		db:     db,
		logger: logger.Get(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(mgr)
		}
	}
	return mgr
}

// SetNotifier 设置通知器
func (m *Manager) SetNotifier(notifier *notification.MultiNotifier) {
	m.notifier = notifier
}

// SetTargetResolver 设置解析器
func (m *Manager) SetTargetResolver(resolver TargetResolver) {
	m.resolver = resolver
}

// CreateApprovalRequest 创建审批请求
func (m *Manager) CreateApprovalRequest(ctx context.Context, req *ApprovalRequestInput) (*workflowpkg.ApprovalRequest, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(req.TimeoutSeconds) * time.Second)

	approval := &workflowpkg.ApprovalRequest{
		ID:             uuid.New().String(),
		TenantID:       req.TenantID,
		ExecutionID:    req.ExecutionID,
		WorkflowID:     req.WorkflowID,
		StepID:         req.StepID,
		Status:         "pending",
		Type:           req.Type,
		RequestedBy:    req.RequestedBy,
		StepOutput:     req.StepOutput,
		NotifyChannels: req.NotifyChannels,
		NotifyTargets:  req.NotifyTargets,
		NotifiedAt:     &now,
		TimeoutSeconds: req.TimeoutSeconds,
		ExpiresAt:      &expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := m.db.WithContext(ctx).Create(approval).Error; err != nil {
		return nil, fmt.Errorf("创建审批请求失败: %w", err)
	}

	metrics.ApprovalPendingGauge.WithLabelValues(req.TenantID).Inc()
	m.publishEvent(ApprovalEvent{
		ApprovalID:  approval.ID,
		TenantID:    approval.TenantID,
		ExecutionID: approval.ExecutionID,
		Status:      "pending",
		OccurredAt:  now,
	})
	go m.dispatchNotification(approval.ID, "initial")

	return approval, nil
}

// ApproveRequest 批准请求
func (m *Manager) ApproveRequest(ctx context.Context, approvalID, approvedBy, comment string) error {
	approval, err := m.loadPendingApproval(ctx, approvalID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":      "approved",
		"approved_by": approvedBy,
		"comment":     comment,
		"resolved_at": now,
		"updated_at":  now,
	}
	if err := m.db.WithContext(ctx).Model(&approval).Updates(updates).Error; err != nil {
		return fmt.Errorf("批准请求失败: %w", err)
	}
	m.decrementPendingGaugeWithTenant(approval.TenantID)
	m.recordDecisionMetric(approval.TenantID, "approved", "manual")
	m.publishEvent(ApprovalEvent{
		ApprovalID:   approval.ID,
		TenantID:     approval.TenantID,
		ExecutionID:  approval.ExecutionID,
		Status:       "approved",
		ApprovedBy:   approvedBy,
		AutoApproved: false,
		Comment:      comment,
		OccurredAt:   now,
	})
	return nil
}

// RejectRequest 拒绝请求
func (m *Manager) RejectRequest(ctx context.Context, approvalID, approvedBy, comment string) error {
	approval, err := m.loadPendingApproval(ctx, approvalID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":      "rejected",
		"approved_by": approvedBy,
		"comment":     comment,
		"resolved_at": now,
		"updated_at":  now,
	}
	if err := m.db.WithContext(ctx).Model(&approval).Updates(updates).Error; err != nil {
		return fmt.Errorf("拒绝请求失败: %w", err)
	}
	m.decrementPendingGaugeWithTenant(approval.TenantID)
	m.recordDecisionMetric(approval.TenantID, "rejected", "manual")
	m.publishEvent(ApprovalEvent{
		ApprovalID:   approval.ID,
		TenantID:     approval.TenantID,
		ExecutionID:  approval.ExecutionID,
		Status:       "rejected",
		ApprovedBy:   approvedBy,
		AutoApproved: false,
		Comment:      comment,
		OccurredAt:   now,
	})
	return nil
}

// GetPendingApprovals 获取待审批请求
func (m *Manager) GetPendingApprovals(ctx context.Context, executionID string) ([]*workflowpkg.ApprovalRequest, error) {
	var approvals []*workflowpkg.ApprovalRequest

	if err := m.db.WithContext(ctx).
		Where("execution_id = ? AND status = ?", executionID, "pending").
		Order("created_at ASC").
		Find(&approvals).Error; err != nil {
		return nil, fmt.Errorf("查询待审批请求失败: %w", err)
	}

	return approvals, nil
}

// CheckAutoApproval 检查是否可以自动批准
func (m *Manager) CheckAutoApproval(ctx context.Context,
	stepOutput map[string]any,
	autoApproveCondition string) (bool, error) {
	expr := strings.TrimSpace(autoApproveCondition)
	if expr == "" {
		return false, nil
	}
	left, operator, right, err := splitCondition(expr)
	if err != nil {
		return false, err
	}
	leftVal, err := resolveOperand(left, stepOutput)
	if err != nil {
		return false, err
	}
	rightVal, err := resolveOperand(right, stepOutput)
	if err != nil {
		return false, err
	}
	return compareValues(leftVal, rightVal, operator)
}

// TimeoutRequest 标记审批请求为超时
func (m *Manager) TimeoutRequest(ctx context.Context, approvalID string) error {
	approval, err := m.loadPendingApproval(ctx, approvalID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":      "timeout",
		"resolved_at": now,
		"updated_at":  now,
	}
	if err := m.db.WithContext(ctx).Model(&approval).Updates(updates).Error; err != nil {
		return fmt.Errorf("标记超时失败: %w", err)
	}
	m.decrementPendingGaugeWithTenant(approval.TenantID)
	m.recordDecisionMetric(approval.TenantID, "timeout", "system")
	m.publishEvent(ApprovalEvent{
		ApprovalID:  approval.ID,
		TenantID:    approval.TenantID,
		ExecutionID: approval.ExecutionID,
		Status:      "timeout",
		OccurredAt:  now,
	})
	return nil
}

// CheckExpiredApprovals 检查并处理过期的审批请求
func (m *Manager) CheckExpiredApprovals(ctx context.Context) error {
	now := time.Now().UTC()

	updates := map[string]any{
		"status":      "timeout",
		"resolved_at": now,
		"updated_at":  now,
	}

	if err := m.db.WithContext(ctx).
		Model(&workflowpkg.ApprovalRequest{}).
		Where("status = ? AND expires_at < ?", "pending", now).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("更新过期审批请求失败: %w", err)
	}

	return nil
}

// ResendNotification 允许管理员手动重发通知
func (m *Manager) ResendNotification(ctx context.Context, tenantID, approvalID string) error {
	var approval workflowpkg.ApprovalRequest
	if err := m.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", approvalID, tenantID).
		First(&approval).Error; err != nil {
		return fmt.Errorf("审批请求不存在")
	}
	settings := m.approvalSettings(ctx, tenantID)
	if settings != nil && settings.ResendLimit > 0 && approval.NotificationAttempts >= settings.ResendLimit {
		return fmt.Errorf("已达到最大重发次数")
	}
	return m.sendNotification(ctx, &approval, "manual_resend")
}

func (m *Manager) decrementPendingGauge(ctx context.Context, approvalID string) {
	if approvalID == "" {
		return
	}
	var approval workflowpkg.ApprovalRequest
	if err := m.db.WithContext(ctx).
		Select("tenant_id").
		Where("id = ?", approvalID).
		First(&approval).Error; err != nil {
		return
	}
	metrics.ApprovalPendingGauge.WithLabelValues(approval.TenantID).Dec()
}

func (m *Manager) decrementPendingGaugeWithTenant(tenantID string) {
	if tenantID == "" {
		return
	}
	metrics.ApprovalPendingGauge.WithLabelValues(tenantID).Dec()
}

func (m *Manager) approvalSettings(ctx context.Context, tenantID string) *tenant.ApprovalSettings {
	if m.resolver != nil {
		return m.resolver.Settings(ctx, tenantID)
	}
	return defaultApprovalSettings()
}

// dispatchNotification 从数据库加载审批并推送通知
func (m *Manager) dispatchNotification(approvalID, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var approval workflowpkg.ApprovalRequest
	if err := m.db.WithContext(ctx).Where("id = ?", approvalID).First(&approval).Error; err != nil {
		if m.logger != nil {
			m.logger.Warn("加载审批请求失败", zap.String("approvalId", approvalID), zap.String("reason", reason), zap.Error(err))
		}
		return
	}

	if err := m.sendNotification(ctx, &approval, reason); err != nil && m.logger != nil {
		m.logger.Warn("发送审批通知失败", zap.String("approvalId", approvalID), zap.String("reason", reason), zap.Error(err))
	}
}

// sendNotification 构建并投递通知
func (m *Manager) sendNotification(ctx context.Context, approval *workflowpkg.ApprovalRequest, reason string) error {
	if m.notifier == nil {
		return fmt.Errorf("通知器未配置")
	}

	targets, err := m.resolveTargets(ctx, approval)
	if err != nil {
		return err
	}

	notificationData := m.buildNotificationData(ctx, approval)
	now := time.Now().UTC()
	var lastErr error
	delivered := 0
	failures := 0

	orderedChannels := m.sortChannels(approval.NotifyChannels, targets.FallbackOrder)
	for _, channel := range orderedChannels {
		recipientList := targets.Channels[channel]
		if len(recipientList) == 0 {
			continue
		}
		for _, recipient := range recipientList {
			notif := m.buildNotification(channel, recipient, approval, notificationData)
			if notif == nil {
				continue
			}
			if err := m.notifier.Send(ctx, notif); err != nil {
				failures++
				lastErr = err
				metrics.ApprovalNotificationsTotal.WithLabelValues(channel, approval.TenantID, "failed").Inc()
				if m.logger != nil {
					m.logger.Warn(
						"通知发送失败",
						zap.String("channel", channel),
						zap.String("approvalId", approval.ID),
						zap.String("reason", reason),
						zap.Error(err),
					)
				}
				continue
			}
			delivered++
			metrics.ApprovalNotificationsTotal.WithLabelValues(channel, approval.TenantID, "delivered").Inc()
		}
	}

	updates := map[string]any{
		"notification_attempts": gorm.Expr("notification_attempts + 1"),
		"last_notified_at":      now,
		"updated_at":            now,
	}
	if lastErr != nil {
		updates["last_notification_error"] = lastErr.Error()
	} else {
		updates["last_notification_error"] = nil
	}
	if err := m.db.WithContext(ctx).Model(&workflowpkg.ApprovalRequest{}).
		Where("id = ?", approval.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("更新通知状态失败: %w", err)
	}

	if delivered == 0 && failures == 0 && m.logger != nil {
		m.logger.Info("审批通知无可用目标", zap.String("approvalId", approval.ID))
	}

	return lastErr
}

func (m *Manager) resolveTargets(ctx context.Context, approval *workflowpkg.ApprovalRequest) (*ResolvedTargets, error) {
	if m.resolver != nil {
		return m.resolver.Resolve(ctx, approval)
	}
	settings := defaultApprovalSettings()
	channels := approval.NotifyChannels
	if len(channels) == 0 {
		channels = settings.DefaultChannels
	}
	result := &ResolvedTargets{
		Channels:      map[string][]string{"websocket": {approval.RequestedBy}},
		FallbackOrder: settings.ChannelFallbackOrder,
	}
	for _, channel := range channels {
		if channel == "websocket" {
			result.Channels[channel] = []string{approval.RequestedBy}
		} else {
			result.Channels[channel] = approval.NotifyTargets[channel]
		}
	}
	return result, nil
}

func (m *Manager) buildNotification(channel string, recipient string, approval *workflowpkg.ApprovalRequest, data map[string]any) *notification.Notification {
	switch channel {
	case "webhook":
		if recipient == "" {
			return nil
		}
		return &notification.Notification{
			Type:    "webhook",
			To:      recipient,
			Subject: "新的审批请求",
			Body:    fmt.Sprintf("执行 ID: %s, 步骤: %s", approval.ExecutionID, approval.StepID),
			Data:    data,
		}
	case "email":
		if recipient == "" {
			return nil
		}
		return &notification.Notification{
			Type:    "email",
			To:      recipient,
			Subject: "【审批通知】您有新的审批请求",
			Body:    "",
			Data:    data,
		}
	case "websocket":
		if recipient == "" {
			recipient = approval.RequestedBy
		}
		return &notification.Notification{
			Type:     "websocket",
			TenantID: approval.TenantID,
			To:       recipient,
			Subject:  "审批请求",
			Body:     "您有新的审批请求",
			Data:     data,
		}
	default:
		return nil
	}
}

// SubscribeApproval 订阅审批事件
func (m *Manager) SubscribeApproval(approvalID string) (<-chan ApprovalEvent, func()) {
	if m.eventBus == nil {
		return nil, nil
	}
	return m.eventBus.Subscribe(approvalID)
}

func (m *Manager) publishEvent(evt ApprovalEvent) {
	if m.eventBus == nil {
		return
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}
	m.eventBus.Publish(evt)
}

func (m *Manager) recordDecisionMetric(tenantID, status, decisionType string) {
	if tenantID == "" {
		tenantID = "unknown"
	}
	if decisionType == "" {
		decisionType = "manual"
	}
	metrics.ApprovalDecisionsTotal.WithLabelValues(tenantID, status, decisionType).Inc()
}

func (m *Manager) loadPendingApproval(ctx context.Context, approvalID string) (*workflowpkg.ApprovalRequest, error) {
	var approval workflowpkg.ApprovalRequest
	if err := m.db.WithContext(ctx).
		Where("id = ? AND status = ?", approvalID, "pending").
		First(&approval).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("审批请求不存在或已处理")
		}
		return nil, fmt.Errorf("查询审批请求失败: %w", err)
	}
	return &approval, nil
}

func splitCondition(expr string) (string, string, string, error) {
	operators := []string{"==", "!=", ">=", "<=", ">", "<"}
	for _, op := range operators {
		if idx := strings.Index(expr, op); idx >= 0 {
			left := strings.TrimSpace(expr[:idx])
			right := strings.TrimSpace(expr[idx+len(op):])
			if left == "" || right == "" {
				return "", "", "", fmt.Errorf("无效的自动审批表达式: %s", expr)
			}
			return left, op, right, nil
		}
	}
	return "", "", "", fmt.Errorf("未识别的操作符: %s", expr)
}

func resolveOperand(raw string, scope map[string]any) (any, error) {
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		return strings.Trim(raw, "\""), nil
	}
	if strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") {
		return strings.Trim(raw, "'"), nil
	}
	if raw == "true" || raw == "false" {
		return raw == "true", nil
	}
	if num, err := strconv.ParseFloat(raw, 64); err == nil {
		return num, nil
	}
	if strings.HasPrefix(raw, "{{") && strings.HasSuffix(raw, "}}") {
		path := strings.TrimSpace(raw[2 : len(raw)-2])
		return lookupPath(scope, path)
	}
	return lookupPath(scope, raw)
}

func lookupPath(scope map[string]any, path string) (any, error) {
	if scope == nil {
		return nil, fmt.Errorf("步输出为空，无法解析 %s", path)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("无效的路径")
	}
	segments := strings.Split(path, ".")
	var current any = scope
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			return nil, fmt.Errorf("无效的路径: %s", path)
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("路径 %s 无法解析", path)
		}
		current, ok = m[seg]
		if !ok {
			return nil, fmt.Errorf("路径 %s 不存在", path)
		}
	}
	return current, nil
}

func compareValues(left, right any, operator string) (bool, error) {
	switch l := left.(type) {
	case float64:
		r, ok := toFloat(right)
		if !ok {
			return false, fmt.Errorf("无法比较数值: %v", right)
		}
		return compareFloat(l, r, operator), nil
	case int:
		r, ok := toFloat(right)
		if !ok {
			return false, fmt.Errorf("无法比较数值: %v", right)
		}
		return compareFloat(float64(l), r, operator), nil
	case string:
		r, ok := right.(string)
		if !ok {
			return false, fmt.Errorf("无法比较字符串: %v", right)
		}
		return compareString(l, r, operator), nil
	case bool:
		r, ok := right.(bool)
		if !ok {
			return false, fmt.Errorf("无法比较布尔值: %v", right)
		}
		return compareBool(l, r, operator)
	default:
		rFloat, okL := toFloat(l)
		if okL {
			r, okR := toFloat(right)
			if !okR {
				return false, fmt.Errorf("无法比较数值: %v", right)
			}
			return compareFloat(rFloat, r, operator), nil
		}
	}
	return false, fmt.Errorf("不支持的比较类型: %T", left)
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case jsonNumber:
		if f, err := val.Float64(); err == nil {
			return f, true
		}
		return 0, false
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func compareFloat(left, right float64, operator string) bool {
	switch operator {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	default:
		return false
	}
}

func compareString(left, right, operator string) bool {
	switch operator {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	default:
		return false
	}
}

func compareBool(left, right bool, operator string) (bool, error) {
	switch operator {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	default:
		return false, fmt.Errorf("布尔值仅支持 == 和 !=")
	}
}

// jsonNumber 用于兼容 json.Number
type jsonNumber interface {
	Float64() (float64, error)
}

func (m *Manager) buildNotificationData(ctx context.Context, approval *workflowpkg.ApprovalRequest) map[string]any {
	workflowName := ""
	if approval.WorkflowID != "" {
		var wf workflowpkg.Workflow
		if err := m.db.WithContext(ctx).
			Select("name").Where("id = ?", approval.WorkflowID).
			First(&wf).Error; err == nil {
			workflowName = wf.Name
		}
	}
	return map[string]any{
		"ApprovalID":   approval.ID,
		"ExecutionID":  approval.ExecutionID,
		"StepID":       approval.StepID,
		"RequestedBy":  approval.RequestedBy,
		"CreatedAt":    approval.CreatedAt.Format("2006-01-02 15:04:05"),
		"ExpiresAt":    formatExpiresAt(approval.ExpiresAt),
		"WorkflowName": workflowName,
		"StepName":     approval.StepID,
		"StepOutput":   approval.StepOutput,
		"ApproveURL":   fmt.Sprintf("https://app.example.com/approvals/%s/approve", approval.ID),
		"RejectURL":    fmt.Sprintf("https://app.example.com/approvals/%s/reject", approval.ID),
		"DashboardURL": "https://app.example.com/approvals",
	}
}

func (m *Manager) sortChannels(requested []string, fallback []string) []string {
	order := make([]string, 0, len(requested))
	seen := make(map[string]struct{})
	for _, channel := range requested {
		if _, ok := seen[channel]; ok {
			continue
		}
		seen[channel] = struct{}{}
		order = append(order, channel)
	}
	for _, channel := range fallback {
		if _, ok := seen[channel]; ok {
			continue
		}
		seen[channel] = struct{}{}
		order = append(order, channel)
	}
	return order
}

// ApprovalRequestInput 审批请求输入
type ApprovalRequestInput struct {
	TenantID       string
	ExecutionID    string
	WorkflowID     string
	StepID         string
	Type           string
	RequestedBy    string
	StepOutput     map[string]any
	NotifyChannels []string
	NotifyTargets  map[string][]string
	TimeoutSeconds int
}

func formatExpiresAt(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return ts.Format("2006-01-02 15:04:05")
}
