package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"backend/internal/worker/tasks"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// WorkflowRunner 工作流执行器抽象，便于注入 mock
type WorkflowRunner interface {
	RunExecution(ctx context.Context, executionID string) error
}

type WorkflowHandler struct {
	runner WorkflowRunner
	logger *zap.Logger
}

func NewWorkflowHandler(runner WorkflowRunner, logger *zap.Logger) *WorkflowHandler {
	return &WorkflowHandler{
		runner: runner,
		logger: logger,
	}
}

func (h *WorkflowHandler) HandleExecuteWorkflow(ctx context.Context, t *asynq.Task) error {
	var p tasks.ExecuteWorkflowPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json unmarshal failed: %w", err)
	}

	h.logger.Info("开始执行工作流任务",
		zap.String("execution_id", p.ExecutionID),
		zap.String("workflow_id", p.WorkflowID),
	)

	// 调用执行器运行任务
	if err := h.runner.RunExecution(ctx, p.ExecutionID); err != nil {
		h.logger.Error("工作流执行失败",
			zap.String("execution_id", p.ExecutionID),
			zap.Error(err),
		)
		return err
	}

	h.logger.Info("工作流执行完成", zap.String("execution_id", p.ExecutionID))
	return nil
}
