package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"backend/internal/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PGLogService 基于 Postgres 的日志服务
type PGLogService struct {
	db *gorm.DB
}

// NewPGLogService 创建 PG 日志服务
func NewPGLogService(db *gorm.DB) *PGLogService {
	ensureAgentLogTables(db)
	return &PGLogService{db: db}
}

// ensureAgentLogTables 确保 Agent 日志表存在，仅在缺失时执行迁移，避免重复慢查询
func ensureAgentLogTables(db *gorm.DB) {
	if db == nil {
		return
	}
	migrator := db.Migrator()
	runTableExists := migrator.HasTable(&AgentRunModel{})
	stepTableExists := migrator.HasTable(&AgentStepModel{})
	if runTableExists && stepTableExists {
		return
	}
	if err := migrator.AutoMigrate(&AgentRunModel{}, &AgentStepModel{}); err != nil {
		logger.Warn("Agent 日志表初始化失败", zap.Error(err))
	}
}

// AgentRunModel 数据库模型
type AgentRunModel struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)"`
	AgentID     string    `gorm:"index;type:varchar(64)"`
	TraceID     string    `gorm:"index;type:varchar(64)"`
	Status      string    `gorm:"type:varchar(20)"`
	Input       string    `gorm:"type:text"`
	Output      string    `gorm:"type:text"`
	Error       string    `gorm:"type:text"`
	TotalTokens int       `gorm:"type:int"`
	LatencyMs   int64     `gorm:"type:bigint"`
	CreatedAt   time.Time `gorm:"index"`
	FinishedAt  *time.Time
	Steps       []AgentStepModel `gorm:"foreignKey:RunID;constraint:OnDelete:CASCADE"`
}

// AgentStepModel 数据库模型
type AgentStepModel struct {
	ID         string          `gorm:"primaryKey;type:varchar(64)"`
	RunID      string          `gorm:"index;type:varchar(64)"`
	Type       string          `gorm:"type:varchar(20)"`
	Content    string          `gorm:"type:text"`
	ToolCalls  json.RawMessage `gorm:"type:jsonb"` // 存储为 JSON
	LatencyMs  int64           `gorm:"type:bigint"`
	Timestamp  time.Time       `gorm:"index"`
	TokenUsage json.RawMessage `gorm:"type:jsonb"` // 存储为 JSON
}

func (s *PGLogService) TableName() string {
	return "agent_runs"
}

func (s *PGLogService) CreateRun(ctx context.Context, run *AgentRunLog) error {
	model := &AgentRunModel{
		ID:          run.ID,
		AgentID:     run.AgentID,
		TraceID:     run.TraceID,
		Status:      run.Status,
		Input:       run.Input,
		Output:      run.Output,
		Error:       run.Error,
		TotalTokens: run.TotalTokens,
		LatencyMs:   run.LatencyMs,
		CreatedAt:   run.CreatedAt,
		FinishedAt:  run.FinishedAt,
	}
	return s.db.WithContext(ctx).Create(model).Error
}

func (s *PGLogService) UpdateRun(ctx context.Context, run *AgentRunLog) error {
	updates := map[string]interface{}{
		"status":       run.Status,
		"output":       run.Output,
		"error":        run.Error,
		"total_tokens": run.TotalTokens,
		"latency_ms":   run.LatencyMs,
		"finished_at":  run.FinishedAt,
	}
	return s.db.WithContext(ctx).Model(&AgentRunModel{}).Where("id = ?", run.ID).Updates(updates).Error
}

func (s *PGLogService) AddStep(ctx context.Context, runID string, step *AgentStepLog) error {
	toolCallsJSON, _ := json.Marshal(step.ToolCalls)
	tokenUsageJSON, _ := json.Marshal(step.TokenUsage)

	model := &AgentStepModel{
		ID:         step.ID,
		RunID:      runID,
		Type:       step.Type,
		Content:    step.Content,
		ToolCalls:  toolCallsJSON,
		LatencyMs:  step.LatencyMs,
		Timestamp:  step.Timestamp,
		TokenUsage: tokenUsageJSON,
	}
	return s.db.WithContext(ctx).Create(model).Error
}

func (s *PGLogService) GetRun(ctx context.Context, runID string) (*AgentRunLog, error) {
	var model AgentRunModel
	if err := s.db.WithContext(ctx).Preload("Steps").First(&model, "id = ?", runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	run := &AgentRunLog{
		ID:          model.ID,
		AgentID:     model.AgentID,
		TraceID:     model.TraceID,
		Status:      model.Status,
		Input:       model.Input,
		Output:      model.Output,
		Error:       model.Error,
		TotalTokens: model.TotalTokens,
		LatencyMs:   model.LatencyMs,
		CreatedAt:   model.CreatedAt,
		FinishedAt:  model.FinishedAt,
		Steps:       make([]*AgentStepLog, len(model.Steps)),
	}

	for i, stepModel := range model.Steps {
		var toolCalls []*ToolCallLog
		_ = json.Unmarshal(stepModel.ToolCalls, &toolCalls)
		var tokenUsage *Usage
		_ = json.Unmarshal(stepModel.TokenUsage, &tokenUsage)

		run.Steps[i] = &AgentStepLog{
			ID:         stepModel.ID,
			Type:       stepModel.Type,
			Content:    stepModel.Content,
			ToolCalls:  toolCalls,
			LatencyMs:  stepModel.LatencyMs,
			Timestamp:  stepModel.Timestamp,
			TokenUsage: tokenUsage,
		}
	}

	return run, nil
}

func (s *PGLogService) GetRunTrace(ctx context.Context, runID string) (any, error) {
	// 简单返回 RunLog，前端可根据 Steps 渲染
	return s.GetRun(ctx, runID)
}
