package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backend/internal/agent/runtime"
	"backend/internal/audit"
	"backend/internal/infra/queue"
	"backend/internal/metrics"
	"backend/internal/notification"
	workflowpkg "backend/internal/workflow"
	"backend/internal/workflow/approval"
	"backend/internal/workflow/state"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ErrTaskPaused 任务暂停错误
var ErrTaskPaused = fmt.Errorf("task paused")

// AutomationEngine 自动化执行引擎
// 扩展标准 Engine，增加自动化控制功能
type AutomationEngine struct {
	*Engine
	stateManager     *state.StateManager
	approvalManager  *approval.Manager
	agentSwitcher    *AgentSwitcher
	qualityEvaluator *QualityEvaluator
}

// SetApprovalNotifier 为审批流配置通知器
func (e *AutomationEngine) SetApprovalNotifier(notifier *notification.MultiNotifier) {
	if e != nil && e.approvalManager != nil {
		e.approvalManager.SetNotifier(notifier)
	}
}

// NewAutomationEngine 创建自动化执行引擎
func NewAutomationEngine(
	db *gorm.DB,
	redisClient redis.UniversalClient,
	agentRegistry *runtime.Registry,
	queueClient queue.Client,
	auditService audit.AuditService,
	approvalMgr *approval.Manager,
	opts ...EngineOption,
) *AutomationEngine {
	baseEngine := NewEngine(db, agentRegistry, queueClient, auditService, opts...)
	if approvalMgr == nil {
		approvalMgr = approval.NewManager(db)
	}

	return &AutomationEngine{
		Engine:           baseEngine,
		stateManager:     state.NewStateManager(redisClient),
		approvalManager:  approvalMgr,
		qualityEvaluator: NewQualityEvaluator(agentRegistry),
	}
}

// ExecuteWithAutomation 使用自动化模式执行工作流
func (e *AutomationEngine) ExecuteWithAutomation(
	ctx context.Context,
	workflowID, tenantID, userID string,
	input map[string]any,
) (*ExecutionResult, error) {

	// 解析工作流定义
	workflowDef, err := e.loadWorkflowDefinition(ctx, workflowID, tenantID)
	if err != nil {
		return nil, err
	}

	// 初始化 Agent 切换器
	if workflowDef.AutomationConfig != nil && workflowDef.AutomationConfig.AgentSwitching != nil {
		e.agentSwitcher = NewAgentSwitcher(workflowDef.AutomationConfig.AgentSwitching)
	}

	// 检查是否启用自动化
	if workflowDef.AutomationConfig == nil || workflowDef.AutomationConfig.Mode == "manual" {
		// 使用标准执行模式
		return e.Engine.Execute(ctx, workflowID, tenantID, userID, input)
	}

	// 初始化执行状态
	executionID := uuid.New().String()
	err = e.initializeState(ctx, executionID, workflowDef)
	if err != nil {
		return nil, err
	}

	// 执行工作流（带自动化控制）
	return e.executeAutomated(ctx, workflowID, executionID, tenantID, userID, input, workflowDef)
}

// ResumeExecution 恢复执行
func (e *AutomationEngine) ResumeExecution(
	ctx context.Context,
	executionID string,
) (*ExecutionResult, error) {
	// 1. 加载执行状态
	state, err := e.stateManager.GetState(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("加载执行状态失败: %w", err)
	}

	// 2. 从 Metadata 中恢复工作流信息
	workflowID, _ := state.Metadata["workflow_id"].(string)
	tenantID, _ := state.Metadata["tenant_id"].(string)
	userID, _ := state.Metadata["user_id"].(string)
	input, _ := state.Metadata["input"].(map[string]any)

	if workflowID == "" || tenantID == "" {
		return nil, fmt.Errorf("无法从状态中恢复工作流信息")
	}

	workflowDef, err := e.loadWorkflowDefinition(ctx, workflowID, tenantID)
	if err != nil {
		return nil, err
	}

	// 3. 检查审批状态
	if approvalID, ok := state.Metadata["add_approval"].(string); ok && approvalID != "" {
		approvalReq, err := e.getApprovalStatus(ctx, approvalID)
		if err != nil {
			return nil, err
		}
		if approvalReq.Status == "approved" {
			// 更新对应步骤的结果为成功
			// 我们需要知道是哪个步骤暂停了。
			// 我们可以遍历 state.StepResults 找到 paused 的步骤
			// 或者在 Metadata 中保存 paused_step_id

			// 简单遍历
			for stepID, res := range state.StepResults {
				// state.StepResults 是 map[string]any，我们需要转换
				if resMap, ok := res.(map[string]any); ok {
					if status, ok := resMap["Status"].(string); ok && status == "paused" {
						// 更新为 success
						resMap["Status"] = "success"
						resMap["Output"] = approvalReq.StepOutput // 或者保留原输出
						// 注意：这里修改的是 map 副本，我们需要更新 state.StepResults
						state.StepResults[stepID] = resMap

						// 同时，我们需要构建 TaskResult 对象传给 Scheduler
					}
				}
			}
		} else if approvalReq.Status == "rejected" {
			return nil, fmt.Errorf("审批被拒绝")
		} else if approvalReq.Status == "pending" {
			return nil, fmt.Errorf("审批仍在进行中")
		}
	}

	// 4. 恢复执行
	return e.resumeAutomated(ctx, executionID, workflowID, tenantID, userID, input, workflowDef, state.StepResults)
}

func (e *AutomationEngine) resumeAutomated(
	ctx context.Context,
	executionID, workflowID, tenantID, userID string,
	input map[string]any,
	workflowDef *WorkflowDefinition,
	previousResultsRaw map[string]any,
) (*ExecutionResult, error) {
	start := time.Now()

	// 构建 DAG
	dag, err := e.parser.BuildDAG(workflowDef)
	if err != nil {
		return nil, fmt.Errorf("构建 DAG 失败: %w", err)
	}

	// 转换 previousResults
	previousResults := make(map[string]*TaskResult)
	for k, v := range previousResultsRaw {
		if resMap, ok := v.(map[string]any); ok {
			// 简单的反序列化
			res := &TaskResult{
				ID:       k,
				Status:   resMap["Status"].(string),
				Output:   resMap["Output"],
				Metadata: make(map[string]any),
			}
			if meta, ok := resMap["Metadata"].(map[string]any); ok {
				res.Metadata = meta
			}
			previousResults[k] = res
		}
	}

	// 创建执行上下文
	execCtx := &ExecutionContext{
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		TenantID:    tenantID,
		UserID:      userID,
		Data:        input, // 初始输入
	}
	// 恢复上下文数据
	// Scheduler.Resume 会处理

	// 创建任务执行器
	taskExecutor := &AutomatedTaskExecutor{
		base:             NewAgentTaskExecutor(e.agentRegistry, e.auditService),
		agentSwitcher:    e.agentSwitcher,
		qualityEvaluator: e.qualityEvaluator,
		approvalManager:  e.approvalManager,
		stateManager:     e.stateManager,
		db:               e.db,
		tenantID:         tenantID,
		userID:           userID,
	}

	// 创建调度器
	scheduler := NewScheduler(dag, taskExecutor, e.maxConcurrency)

	// 恢复执行
	results, err := scheduler.Resume(ctx, execCtx, previousResults)
	duration := time.Since(start)

	// 检查是否有暂停的任务
	status := "completed"
	for _, res := range results {
		if res.Status == "paused" {
			status = "paused"
			break
		}
	}

	// 构建结果
	result := &ExecutionResult{
		ExecutionID: executionID,
		WorkflowID:  workflowID,
		Status:      status,
		Input:       input,
		Output:      execCtx.GetAllData(),
		Error:       err,
		StartedAt:   start, // 这里应该是原始开始时间，但为了简单起见使用本次恢复时间
		Duration:    duration,
		Tasks:       results,
	}

	if err != nil {
		result.Status = "failed"
	}

	// 保存状态
	if result.Status == "paused" {
		stateData := map[string]any{
			"status":       "paused",
			"step_results": results,
			"updated_at":   time.Now().UTC(),
		}
		_ = e.stateManager.UpdateState(ctx, executionID, stateData)
	} else {
		_ = e.stateManager.DeleteState(ctx, executionID)
	}

	return result, err
}

// executeAutomated 自动化执行
func (e *AutomationEngine) executeAutomated(
	ctx context.Context,
	workflowID, executionID, tenantID, userID string,
	input map[string]any,
	workflowDef *WorkflowDefinition,
) (*ExecutionResult, error) {

	start := time.Now()

	// 构建 DAG
	dag, err := e.parser.BuildDAG(workflowDef)
	if err != nil {
		return nil, fmt.Errorf("构建 DAG 失败: %w", err)
	}

	// 创建执行上下文
	execCtx := &ExecutionContext{
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		TenantID:    tenantID,
		UserID:      userID,
		Data:        input,
	}

	// 初始化状态并保存元数据
	if err := e.initializeState(ctx, executionID, workflowDef); err != nil {
		return nil, fmt.Errorf("初始化状态失败: %w", err)
	}
	_ = e.stateManager.UpdateState(ctx, executionID, map[string]any{
		"metadata": map[string]any{
			"workflow_id": workflowID,
			"tenant_id":   tenantID,
			"user_id":     userID,
			"input":       input,
		},
	})

	// 创建任务执行器（增强版）
	taskExecutor := &AutomatedTaskExecutor{
		base:             NewAgentTaskExecutor(e.agentRegistry, e.auditService),
		agentSwitcher:    e.agentSwitcher,
		qualityEvaluator: e.qualityEvaluator,
		approvalManager:  e.approvalManager,
		stateManager:     e.stateManager,
		db:               e.db,
		tenantID:         tenantID,
		userID:           userID,
	}

	// 创建调度器（复用 Engine 并发配置）
	scheduler := NewScheduler(dag, taskExecutor, e.maxConcurrency)

	// 执行工作流
	results, err := scheduler.Schedule(ctx, execCtx)
	duration := time.Since(start)

	// 检查是否有暂停的任务
	status := "completed"
	for _, res := range results {
		if res.Status == "paused" {
			status = "paused"
			break
		}
	}

	// 构建结果
	result := &ExecutionResult{
		ExecutionID: executionID,
		WorkflowID:  workflowID,
		Status:      status,
		Input:       input,
		Output:      execCtx.GetAllData(),
		Error:       err,
		StartedAt:   start,
		Duration:    duration,
		Tasks:       results,
	}

	if err != nil {
		result.Status = "failed"
	}

	// 保存状态
	// 如果暂停，我们保留状态以便恢复
	// 如果完成或失败，我们删除状态（或者归档）
	if result.Status == "paused" {
		// 更新状态为 paused，并保存当前结果
		stateData := map[string]any{
			"status":       "paused",
			"step_results": results,
			"updated_at":   time.Now().UTC(),
		}
		_ = e.stateManager.UpdateState(ctx, executionID, stateData)
	} else {
		// 清理状态
		_ = e.stateManager.DeleteState(ctx, executionID)
	}

	return result, err
}

// AutomatedTaskExecutor 自动化任务执行器
type AutomatedTaskExecutor struct {
	base             *AgentTaskExecutor
	agentSwitcher    *AgentSwitcher
	qualityEvaluator *QualityEvaluator
	approvalManager  *approval.Manager
	stateManager     *state.StateManager
	db               *gorm.DB
	tenantID         string
	userID           string
}

// ExecuteTask 执行任务（带自动化控制）
func (e *AutomatedTaskExecutor) ExecuteTask(ctx context.Context, task *Task) (*TaskResult, error) {

	// 1. 检查是否需要审批
	if task.Step.ApprovalRequired {
		approved, err := e.handleApproval(ctx, task)
		if err != nil {
			if err == ErrTaskPaused {
				return &TaskResult{
					ID:     task.ID,
					Status: "paused",
					Metadata: map[string]any{
						"approval_status": "pending",
					},
				}, nil
			}
			return nil, fmt.Errorf("审批处理失败: %w", err)
		}
		if !approved {
			return &TaskResult{
				ID:     task.ID,
				Status: "rejected",
				Metadata: map[string]any{
					"approval_status": "rejected",
				},
			}, fmt.Errorf("任务被拒绝")
		}
	}

	// 2. 执行任务（带重试）
	var result *TaskResult
	var err error

	retryConfig := task.Step.Retry
	maxRetries := 0
	if retryConfig != nil {
		maxRetries = retryConfig.MaxRetries
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 执行任务
		result, err = e.base.ExecuteTask(ctx, task)

		if err == nil && result.Status == "success" {
			break // 成功，退出重试循环
		}

		// 判断是否应该重试
		if attempt < maxRetries {
			delay := e.getRetryDelay(retryConfig, attempt)
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	if err != nil {
		return result, err
	}

	// 3. 质量检查
	if task.Step.QualityCheck != nil && task.Step.QualityCheck.Enabled {
		// 类型断言：将 Output 转换为 string
		outputStr, ok := result.Output.(string)
		if !ok {
			outputStr = fmt.Sprintf("%v", result.Output)
		}

		score, evalErr := e.qualityEvaluator.EvaluateQuality(ctx, outputStr, task.Step.QualityCheck)
		if evalErr == nil {
			result.Metadata["quality_score"] = score

			// 检查是否需要重写
			if e.qualityEvaluator.NeedsRewrite(score, task.Step.QualityCheck) {
				// 使用重写 Agent
				rewriteResult, rewriteErr := e.rewriteWithAgent(ctx, task, outputStr, task.Step.QualityCheck.RewriteAgent)
				if rewriteErr == nil {
					return rewriteResult, nil
				}
			}
		}
	}

	// 4. Agent 切换判断
	if e.agentSwitcher != nil {
		nextAgent, switchErr := e.agentSwitcher.DetermineNextAgent(ctx, task.Step.ID, convertToAgentResult(result))
		if switchErr == nil && nextAgent != "" {
			result.Metadata["next_agent_suggestion"] = nextAgent
		}
	}

	return result, nil
}

// handleApproval 处理审批
func (e *AutomatedTaskExecutor) handleApproval(ctx context.Context, task *Task) (bool, error) {
	// 检查是否可以自动批准
	if autoApproved, err := e.shouldAutoApprove(ctx, task); err != nil {
		return false, err
	} else if autoApproved {
		return true, nil
	}

	notifyChannels := []string{"websocket"}
	notifyTargets := map[string][]string{}
	timeoutSeconds := 3600
	approvalType := "required"

	if cfg := task.Step.ApprovalConfig; cfg != nil {
		if cfg.Type != "" {
			approvalType = cfg.Type
		}
		if len(cfg.NotifyChannels) > 0 {
			notifyChannels = cfg.NotifyChannels
		}
		if cfg.NotifyTargets != nil {
			notifyTargets = cfg.NotifyTargets
		}
		if cfg.TimeoutSeconds > 0 {
			timeoutSeconds = cfg.TimeoutSeconds
		}
	}

	// 创建审批请求
	approvalInput := &approval.ApprovalRequestInput{
		TenantID:       e.tenantID,
		ExecutionID:    task.Context.ExecutionID,
		WorkflowID:     task.Context.WorkflowID,
		StepID:         task.Step.ID,
		Type:           approvalType,
		RequestedBy:    e.userID,
		StepOutput:     task.Input,
		NotifyChannels: notifyChannels,
		NotifyTargets:  notifyTargets,
		TimeoutSeconds: timeoutSeconds,
	}

	approvalReq, err := e.approvalManager.CreateApprovalRequest(ctx, approvalInput)
	if err != nil {
		return false, err
	}

	// 更新状态：等待审批
	_ = e.stateManager.UpdateState(ctx, task.Context.ExecutionID, map[string]any{
		"status":       "paused",
		"add_approval": approvalReq.ID,
	})

	// 返回 false 表示未批准，但这里我们需要一种机制告诉 Executor 任务是“暂停”而不是“拒绝”
	// 在 ExecuteTask 中，我们通过返回特定的 error 或修改 TaskResult 来实现
	// 但 handleApproval 返回 bool, error
	// 我们修改约定：返回 false, nil 且不报错，表示需要暂停？
	// 不，ExecuteTask 逻辑是：
	/*
		if !approved {
			return &TaskResult{... "rejected" ...}, fmt.Errorf("任务被拒绝")
		}
	*/
	// 所以我们需要在这里返回一个特殊的 error 或者改变 handleApproval 的签名
	// 为了最小化改动，我们可以返回 false, ErrTaskPaused

	return false, ErrTaskPaused
}

func (e *AutomatedTaskExecutor) shouldAutoApprove(ctx context.Context, task *Task) (bool, error) {
	if task.Step.ApprovalConfig == nil || task.Step.ApprovalConfig.AutoApproveIf == nil {
		return false, nil
	}
	expr := strings.TrimSpace(task.Step.ApprovalConfig.AutoApproveIf.Expression)
	if expr == "" {
		return false, nil
	}
	condExecutor := NewConditionExecutor(nil)
	result, err := condExecutor.EvaluateCondition(expr, task.Context)
	if err == nil && result {
		if e.db != nil {
			logDetails := map[string]any{
				"expression": expr,
				"step_id":    task.Step.ID,
			}
			_ = e.db.WithContext(ctx).Create(&workflowpkg.AutomationLog{
				ID:          uuid.New().String(),
				TenantID:    e.tenantID,
				ExecutionID: task.Context.ExecutionID,
				StepID:      task.Step.ID,
				Action:      "approval_auto",
				Details:     logDetails,
				Result:      "success",
			}).Error
		}
		metrics.ApprovalDecisionsTotal.WithLabelValues(e.tenantID, "approved", "auto").Inc()
	}
	return result, err
}

// getApprovalStatus 获取审批状态
func (e *AutomatedTaskExecutor) getApprovalStatus(ctx context.Context, approvalID string) (*workflowpkg.ApprovalRequest, error) {
	var approvalReq workflowpkg.ApprovalRequest

	if err := e.db.WithContext(ctx).
		Where("id = ?", approvalID).
		First(&approvalReq).Error; err != nil {
		return nil, fmt.Errorf("查询审批状态失败: %w", err)
	}

	return &approvalReq, nil
}

// rewriteWithAgent 使用指定 Agent 重写
func (e *AutomatedTaskExecutor) rewriteWithAgent(
	ctx context.Context,
	originalTask *Task,
	originalOutput string,
	rewriteAgentType string,
) (*TaskResult, error) {

	// 创建重写任务
	rewriteTask := &Task{
		ID: originalTask.ID + "_rewrite",
		Step: &StepDefinition{
			ID:        originalTask.Step.ID + "_rewrite",
			AgentType: rewriteAgentType,
			Input: map[string]any{
				"content": originalOutput,
				"task":    "rewrite",
			},
		},
		Context: originalTask.Context,
	}

	return e.base.ExecuteTask(ctx, rewriteTask)
}

// getRetryDelay 获取重试延迟
func (e *AutomatedTaskExecutor) getRetryDelay(config *RetryConfig, attempt int) int {
	if config == nil {
		return 0
	}

	if e.agentSwitcher != nil {
		return e.agentSwitcher.GetRetryDelay(config, attempt)
	}

	return config.Delay
}

// 辅助函数

func (e *AutomationEngine) initializeState(ctx context.Context, executionID string, workflowDef *WorkflowDefinition) error {
	initialState := &state.WorkflowState{
		ExecutionID: executionID,
		Status:      "running",
		StepResults: make(map[string]any),
		Metadata:    make(map[string]any),
	}

	if workflowDef.AutomationConfig != nil {
		initialState.Mode = workflowDef.AutomationConfig.Mode
		initialState.MaxRounds = workflowDef.AutomationConfig.MaxRounds
	}

	return e.stateManager.SaveState(ctx, initialState)
}

// getApprovalStatus 获取审批状态
func (e *AutomationEngine) getApprovalStatus(ctx context.Context, approvalID string) (*workflowpkg.ApprovalRequest, error) {
	var approvalReq workflowpkg.ApprovalRequest

	if err := e.db.WithContext(ctx).
		Where("id = ?", approvalID).
		First(&approvalReq).Error; err != nil {
		return nil, fmt.Errorf("查询审批状态失败: %w", err)
	}

	return &approvalReq, nil
}

func (e *AutomationEngine) loadWorkflowDefinition(ctx context.Context, workflowID, tenantID string) (*WorkflowDefinition, error) {
	// 从数据库加载工作流
	var workflow struct {
		Definition workflowpkg.WorkflowDefinition `gorm:"column:definition;type:jsonb;serializer:json"`
	}

	if err := e.db.WithContext(ctx).
		Table("workflows").
		Select("definition").
		Where("id = ? AND tenant_id = ?", workflowID, tenantID).
		First(&workflow).Error; err != nil {
		return nil, fmt.Errorf("加载工作流定义失败: %w", err)
	}

	// 解析定义
	return e.parser.Parse(workflow.Definition)
}

func convertToAgentResult(taskResult *TaskResult) *runtime.AgentResult {
	// 类型断言：将 Output 转换为 string
	outputStr, ok := taskResult.Output.(string)
	if !ok {
		outputStr = fmt.Sprintf("%v", taskResult.Output)
	}

	return &runtime.AgentResult{
		Output:   outputStr,
		Metadata: taskResult.Metadata,
		Status:   taskResult.Status,
	}
}
