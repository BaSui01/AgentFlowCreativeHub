package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/internal/workspace"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service 管理命令执行请求
type Service struct {
	db              *gorm.DB
	workspace       *workspace.Service
	maxInFlight     int
	dedupWindow     time.Duration
	contextMaxNodes int
	contextMaxRunes int
}

// ListCommandsParams 控制命令查询过滤。
type ListCommandsParams struct {
	Status  string
	AgentID string
	Limit   int
	Offset  int
}

// NewService 创建服务
func NewService(db *gorm.DB, workspaceSvc *workspace.Service) *Service {
	return &Service{
		db:              db,
		workspace:       workspaceSvc,
		maxInFlight:     10,
		dedupWindow:     60 * time.Second,
		contextMaxNodes: 5,
		contextMaxRunes: 4000,
	}
}

// AutoMigrate 确保表结构存在
func (s *Service) AutoMigrate() error {
	return s.db.AutoMigrate(&CommandRequest{})
}

// ExecuteCommandInput 输入
type ExecuteCommandInput struct {
	TenantID       string
	UserID         string
	AgentID        string
	CommandType    string
	Content        string
	SessionID      string
	ContextNodeIDs []string
	Notes          string
	DeadlineMs     int64
}

// ExecuteCommandResult 输出
type ExecuteCommandResult struct {
	Request      *CommandRequest `json:"request"`
	NewlyCreated bool            `json:"new"`
}

// ExecuteCommand 入队命令
func (s *Service) ExecuteCommand(ctx context.Context, input *ExecuteCommandInput) (*ExecuteCommandResult, error) {
	if input == nil {
		return nil, errors.New("invalid input")
	}
	if strings.TrimSpace(input.AgentID) == "" {
		return nil, errors.New("agentId 不能为空")
	}
	if strings.TrimSpace(input.Content) == "" {
		return nil, errors.New("content 不能为空")
	}

	if err := s.enforceInFlightLimit(ctx, input.TenantID, input.AgentID); err != nil {
		return nil, err
	}

	dedupKey := s.computeDedupKey(input)
	if existing := s.findRecentDuplicate(ctx, input.TenantID, dedupKey); existing != nil {
		return &ExecuteCommandResult{Request: existing, NewlyCreated: false}, nil
	}

	contextNodes, _ := s.workspace.LoadContextNodes(ctx, input.TenantID, input.ContextNodeIDs)
	snapshot, truncated := s.buildContextSnapshot(input, contextNodes)
	nodeIDsJSON, _ := json.Marshal(input.ContextNodeIDs)
	revisionID := uuid.New().String()
	deadline := s.computeDeadline(input)
	queuePos := s.nextQueuePosition(ctx, input.TenantID, input.AgentID)
	queueKey := buildQueueKey(input.TenantID, input.AgentID)

	req := &CommandRequest{
		TenantID:        input.TenantID,
		CommandType:     input.CommandType,
		AgentID:         input.AgentID,
		SessionID:       input.SessionID,
		Status:          "queued",
		ContextRevision: revisionID,
		ContextSnapshot: snapshot,
		ContextNodeIDs:  datatypes.JSON(nodeIDsJSON),
		ContextDigest:   dedupKey,
		Notes:           s.decorateNotes(input.Notes, truncated),
		DedupKey:        dedupKey,
		DeadlineAt:      deadline,
		TraceID:         uuid.New().String(),
		QueuePosition:   queuePos,
		QueueKey:        queueKey,
		CreatedBy:       input.UserID,
	}

	if err := s.db.WithContext(ctx).Create(req).Error; err != nil {
		return nil, err
	}

	return &ExecuteCommandResult{Request: req, NewlyCreated: true}, nil
}

// GetCommand 查询
func (s *Service) GetCommand(ctx context.Context, tenantID, id string) (*CommandRequest, error) {
	var req CommandRequest
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&req).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &req, nil
}

// MarkRunning 更新状态
func (s *Service) MarkRunning(ctx context.Context, id string) {
	if id == "" {
		return
	}
	s.db.WithContext(ctx).
		Model(&CommandRequest{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": "running", "queue_position": 0})
}

// MarkCompleted 记录成功
func (s *Service) MarkCompleted(ctx context.Context, id string, preview string, latencyMs int, tokens int) {
	if id == "" {
		return
	}
	updates := map[string]any{
		"status":         "completed",
		"result_preview": trimPreview(preview),
		"latency_ms":     latencyMs,
		"token_cost":     tokens,
	}
	s.db.WithContext(ctx).Model(&CommandRequest{}).Where("id = ?", id).Updates(updates)
}

// MarkFailed 记录失败
func (s *Service) MarkFailed(ctx context.Context, id string, reason string) {
	if id == "" {
		return
	}
	s.db.WithContext(ctx).
		Model(&CommandRequest{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": "failed", "failure_reason": reason})
}

// ListCommands 返回指定租户的命令列表。
func (s *Service) ListCommands(ctx context.Context, tenantID string, params ListCommandsParams) ([]CommandRequest, int64, error) {
	if tenantID == "" {
		return nil, 0, errors.New("tenantID required")
	}
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	query := s.db.WithContext(ctx).Model(&CommandRequest{}).Where("tenant_id = ?", tenantID)
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.AgentID != "" {
		query = query.Where("agent_id = ?", params.AgentID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []CommandRequest
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Service) enforceInFlightLimit(ctx context.Context, tenantID, agentID string) error {
	var count int64
	cutoff := time.Now().Add(-5 * time.Minute)
	_ = s.db.WithContext(ctx).
		Model(&CommandRequest{}).
		Where("tenant_id = ? AND agent_id = ? AND status IN ? AND created_at >= ?",
			tenantID, agentID, []string{"queued", "running"}, cutoff).
		Count(&count)
	if int(count) >= s.maxInFlight {
		return fmt.Errorf("agent %s 当前排队任务过多，请稍后再试", agentID)
	}
	return nil
}

func (s *Service) computeDedupKey(input *ExecuteCommandInput) string {
	base := input.SessionID + "|" + input.CommandType + "|" + input.Content
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:])
}

func buildQueueKey(tenantID, agentID string) string {
	return tenantID + ":" + agentID
}

func (s *Service) nextQueuePosition(ctx context.Context, tenantID, agentID string) int {
	var count int64
	s.db.WithContext(ctx).
		Model(&CommandRequest{}).
		Where("tenant_id = ? AND agent_id = ? AND status = ?", tenantID, agentID, "queued").
		Count(&count)
	pos := int(count) + 1
	if pos < 1 {
		return 1
	}
	return pos
}

func (s *Service) findRecentDuplicate(ctx context.Context, tenantID, dedupKey string) *CommandRequest {
	var req CommandRequest
	cutoff := time.Now().UTC().Add(-s.dedupWindow)
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND dedup_key = ? AND status IN ? AND created_at >= ?",
			tenantID, dedupKey, []string{"queued", "running", "completed"}, cutoff).
		Order("created_at DESC").
		First(&req).Error; err == nil {
		return &req
	}
	return nil
}

func (s *Service) buildContextSnapshot(input *ExecuteCommandInput, nodes []*workspace.ContextNode) (string, bool) {
	runesUsed := 0
	var builder strings.Builder
	builder.WriteString("【命令上下文】\n")
	if input.CommandType != "" {
		builder.WriteString("命令: " + input.CommandType + "\n")
	}
	if input.SessionID != "" {
		builder.WriteString("会话:" + input.SessionID + "\n")
	}
	truncated := false
	for idx, node := range nodes {
		if idx >= s.contextMaxNodes {
			truncated = true
			break
		}
		line := fmt.Sprintf("- %s (%s)\n", node.Node.Name, node.Node.NodePath)
		runesUsed += len([]rune(line))
		builder.WriteString(line)
		if node.Version != nil && strings.TrimSpace(node.Version.Summary) != "" {
			snippet := trimPreview(node.Version.Summary)
			txt := "  摘要: " + snippet + "\n"
			runesUsed += len([]rune(txt))
			builder.WriteString(txt)
		}
		if runesUsed >= s.contextMaxRunes {
			truncated = true
			break
		}
	}
	if truncated {
		builder.WriteString("(部分上下文因超限被自动省略)\n")
	}
	if strings.TrimSpace(input.Notes) != "" {
		builder.WriteString("备注:" + input.Notes)
	}
	return builder.String(), truncated
}

func (s *Service) decorateNotes(notes string, truncated bool) string {
	if !truncated {
		return notes
	}
	if notes == "" {
		return "上下文被截断，需按需补充"
	}
	return notes + " (上下文已部分截断)"
}

func (s *Service) computeDeadline(input *ExecuteCommandInput) *time.Time {
	if input.DeadlineMs <= 0 {
		return nil
	}
	d := time.Now().Add(time.Duration(input.DeadlineMs) * time.Millisecond)
	return &d
}

func trimPreview(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > 480 {
		return string(runes[:480]) + "..."
	}
	return text
}
