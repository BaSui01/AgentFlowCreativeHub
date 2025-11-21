package executor

import (
	"context"
	"fmt"
	"time"

	"backend/internal/agent/runtime"
	"backend/internal/audit"
	"backend/internal/infra/queue"
	"backend/internal/worker/tasks"
	workflowpkg "backend/internal/workflow"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Engine 工作流执行引擎
type Engine struct {
	db             *gorm.DB
	agentRegistry  *runtime.Registry
	parser         *Parser
	queueClient    queue.Client
	auditService   audit.AuditService
	maxConcurrency int
}

// NewEngine 创建执行引擎
func NewEngine(db *gorm.DB, agentRegistry *runtime.Registry, queueClient queue.Client, auditService audit.AuditService, opts ...EngineOption) *Engine {
	e := &Engine{
		db:             db,
		agentRegistry:  agentRegistry,
		parser:         NewParser(),
		queueClient:    queueClient,
		auditService:   auditService,
		maxConcurrency: 5,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// EngineOption 用于自定义 Engine 配置
type EngineOption func(*Engine)

// WithMaxConcurrency 配置调度器最大并发度
func WithMaxConcurrency(n int) EngineOption {
	return func(e *Engine) {
		if n > 0 {
			e.maxConcurrency = n
		}
	}
}

// MaxConcurrency 返回当前配置
func (e *Engine) MaxConcurrency() int {
	return e.maxConcurrency
}

// Execute 提交工作流执行任务 (异步)
func (e *Engine) Execute(ctx context.Context, workflowID, tenantID, userID string, input map[string]any) (*ExecutionResult, error) {
	// 1. 验证工作流是否存在
	var workflow workflowpkg.Workflow
	if err := e.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", workflowID, tenantID).
		First(&workflow).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("工作流不存在: %s", workflowID)
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	// 2. 创建执行记录 (Status: queued)
	executionID := uuid.New().String()
	now := time.Now().UTC()
	execution := &workflowpkg.WorkflowExecution{
		ID:         executionID,
		WorkflowID: workflowID,
		TenantID:   tenantID,
		UserID:     userID,
		Status:     "queued",
		Input:      input,
		StartedAt:  &now, // 提交时间
		TraceID:    uuid.New().String(),
	}

	if err := e.db.WithContext(ctx).Create(execution).Error; err != nil {
		return nil, fmt.Errorf("创建执行记录失败: %w", err)
	}

	// 3. 入队任务
	payload := tasks.ExecuteWorkflowPayload{
		ExecutionID: executionID,
		WorkflowID:  workflowID,
		TenantID:    tenantID,
		UserID:      userID,
		Input:       input,
	}

	if err := e.queueClient.EnqueueExecuteWorkflow(payload); err != nil {
		// 入队失败，更新状态为 failed
		e.db.WithContext(ctx).Model(execution).Updates(map[string]any{
			"status":        "failed",
			"error_message": fmt.Sprintf("任务入队失败: %v", err),
			"completed_at":  time.Now().UTC(),
		})
		return nil, fmt.Errorf("任务入队失败: %w", err)
	}

	// 4. 返回初始结果
	return &ExecutionResult{
		ExecutionID: executionID,
		WorkflowID:  workflowID,
		Status:      "queued",
		Input:       input,
		StartedAt:   now,
	}, nil
}

// RunExecution 执行工作流 (Worker 调用)
func (e *Engine) RunExecution(ctx context.Context, executionID string) error {
	// 1. 查询执行记录
	var execution workflowpkg.WorkflowExecution
	if err := e.db.WithContext(ctx).
		Where("id = ?", executionID).
		First(&execution).Error; err != nil {
		return fmt.Errorf("查询执行记录失败: %w", err)
	}

	// 2. 查询工作流定义
	var workflow workflowpkg.Workflow
	if err := e.db.WithContext(ctx).
		Where("id = ?", execution.WorkflowID).
		First(&workflow).Error; err != nil {
		return fmt.Errorf("查询工作流失败: %w", err)
	}

	// 3. 更新状态为 running
	if err := e.db.WithContext(ctx).Model(&execution).
		Updates(map[string]any{"status": "running"}).Error; err != nil {
		return fmt.Errorf("更新状态失败: %w", err)
	}

	// 4. 解析工作流定义
	workflowDef, err := e.parser.Parse(workflow.Definition)
	if err != nil {
		return e.failExecution(ctx, &execution, fmt.Errorf("解析工作流定义失败: %w", err))
	}

	// 5. 构建 DAG
	dag, err := e.parser.BuildDAG(workflowDef)
	if err != nil {
		return e.failExecution(ctx, &execution, fmt.Errorf("构建 DAG 失败: %w", err))
	}

	// 6. 创建执行上下文
	execCtx := &ExecutionContext{
		WorkflowID:  execution.WorkflowID,
		ExecutionID: executionID,
		TenantID:    execution.TenantID,
		UserID:      execution.UserID,
		Data:        execution.Input, // 使用记录中的 Input
	}

	// 7. 创建任务执行器与调度器
	taskExecutor := NewAgentTaskExecutor(e.agentRegistry, e.auditService)
	scheduler := NewScheduler(dag, taskExecutor, e.maxConcurrency)

	// 8. 执行调度
	_, err = scheduler.Schedule(ctx, execCtx)

	// 9. 更新最终状态
	updates := map[string]any{
		"completed_at": time.Now().UTC(),
		"output":       execCtx.GetAllData(),
		"updated_at":   time.Now().UTC(),
	}

	status := "completed"
	if err != nil {
		status = "failed"
		updates["error_message"] = err.Error()
	}
	updates["status"] = status

	if err := e.db.WithContext(ctx).Model(&execution).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新执行结果失败: %w", err)
	}

	// 10. 更新统计
	e.updateWorkflowStats(ctx, execution.WorkflowID, status)

	return err
}

func (e *Engine) failExecution(ctx context.Context, execution *workflowpkg.WorkflowExecution, err error) error {
	e.db.WithContext(ctx).Model(execution).Updates(map[string]any{
		"status":        "failed",
		"error_message": err.Error(),
		"completed_at":  time.Now().UTC(),
	})
	e.updateWorkflowStats(ctx, execution.WorkflowID, "failed")
	return err
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	ExecutionID string
	WorkflowID  string
	Status      string
	Input       map[string]any
	Output      map[string]any
	Error       error
	StartedAt   time.Time
	CompletedAt *time.Time
	Duration    time.Duration
	Tasks       map[string]*TaskResult
}

// updateWorkflowStats 更新工作流统计
func (e *Engine) updateWorkflowStats(ctx context.Context, workflowID, status string) {
	updates := map[string]any{
		"total_executions": gorm.Expr("total_executions + 1"),
		"updated_at":       time.Now().UTC(),
	}

	if status == "completed" {
		updates["success_executions"] = gorm.Expr("success_executions + 1")
	} else if status == "failed" {
		updates["failed_executions"] = gorm.Expr("failed_executions + 1")
	}

	_ = e.db.WithContext(ctx).
		Model(&workflowpkg.Workflow{}).
		Where("id = ?", workflowID).
		Updates(updates)
}

// 辅助函数

func timePtr(t time.Time) *time.Time {
	return &t
}
