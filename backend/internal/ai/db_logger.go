package ai

import (
	"context"
	"time"

	"backend/pkg/types"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DBLogger 数据库日志记录器
type DBLogger struct {
	db *gorm.DB
}

// NewDBLogger 创建数据库日志记录器
func NewDBLogger(db *gorm.DB) *DBLogger {
	return &DBLogger{db: db}
}

// Log 记录模型调用日志
func (l *DBLogger) Log(ctx context.Context, log *ModelCallLog) error {
	modelProvider := log.ModelProvider
	if modelProvider == "" {
		modelProvider = log.ModelID
	}
	modelName := log.ModelName
	if modelName == "" {
		modelName = log.ModelID
	}

	// 转换为types.AICallLog (纯数据模型)
	dbLog := &types.AICallLog{
		ID:             uuid.New().String(),
		TenantID:       log.TenantID,
		UserID:         log.UserID,
		ModelProvider:  modelProvider,
		ModelName:      modelName,
		RequestTokens:  log.PromptTokens,
		ResponseTokens: log.CompletionTokens,
		TotalTokens:    log.TotalTokens,
		LatencyMS:      log.LatencyMs,
		Cost:           log.TotalCost,
		Status:         "success",
		Metadata: map[string]interface{}{
			"workflow_id": log.WorkflowID,
			"trace_id":    log.TraceID,
		},
		CreatedAt: time.Now().UTC(),
	}

	// 写入数据库 (使用gorm直接存储types.AICallLog)
	// 注意: 需要确保数据库表结构与types.AICallLog匹配
	if err := l.db.WithContext(ctx).Table("ai_call_logs").Create(dbLog).Error; err != nil {
		// 记录日志失败不应影响主流程
		return err
	}

	return nil
}
