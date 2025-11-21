package executor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"backend/internal/agent/runtime"
	"backend/internal/audit"
)

// AgentTaskExecutor Agent任务执行器
// 负责将工作流任务委托给Agent执行,处理上下文映射和结果转换
type AgentTaskExecutor struct {
	agentRegistry *runtime.Registry
	auditService  audit.AuditService
}

// NewAgentTaskExecutor 创建Agent任务执行器
func NewAgentTaskExecutor(registry *runtime.Registry, auditService audit.AuditService) *AgentTaskExecutor {
	return &AgentTaskExecutor{
		agentRegistry: registry,
		auditService:  auditService,
	}
}

// ExecuteTask 执行单个任务
// 实现TaskExecutor接口
func (e *AgentTaskExecutor) ExecuteTask(ctx context.Context, task *Task) (*TaskResult, error) {
	// 检查 MapReduce
	if task.Step.MapReduce != nil && task.Step.MapReduce.Enabled {
		return e.executeMapReduce(ctx, task)
	}
	return e.executeStandardAgent(ctx, task)
}

// executeStandardAgent 执行标准 Agent 任务
func (e *AgentTaskExecutor) executeStandardAgent(ctx context.Context, task *Task) (*TaskResult, error) {
	start := time.Now()

	// 1. 获取Agent实例
	agent, err := e.getAgent(ctx, task)
	if err != nil {
		return &TaskResult{
			ID:     task.ID,
			Status: "failed",
			Error:  fmt.Errorf("获取Agent失败: %w", err),
			Metadata: map[string]any{
				"error_stage": "agent_retrieval",
			},
		}, err
	}

	// 2. 构建AgentInput
	agentInput := e.buildAgentInput(task)

	// 3. 执行Agent
	result, err := agent.Execute(ctx, agentInput)

	latency := time.Since(start)

	// 4. 处理执行错误 (增强 Error Handling)
	if err != nil {
		// 检查是否是上下文超限错误 (Context Window Overflow)
		if isContextLimitError(err) {
			// 尝试降级策略：清理历史或仅重试核心输入
			// 注意：这里仅做简单的标记和返回，更复杂的降级逻辑（如 summarization）应在 Agent 内部或 Workflow 层处理
			return &TaskResult{
				ID:     task.ID,
				Status: "failed",
				Error:  fmt.Errorf("上下文窗口超限: %w", err),
				Metadata: map[string]any{
					"latency_ms":      latency.Milliseconds(),
					"agent_type":      agent.Type(),
					"agent_name":      agent.Name(),
					"error_stage":     "agent_execution",
					"error_type":      "context_limit_exceeded",
					"suggested_retry": true, // 提示工作流引擎可能需要截断输入后重试
				},
			}, err
		}

		return &TaskResult{
			ID:     task.ID,
			Status: "failed",
			Error:  err,
			Metadata: map[string]any{
				"latency_ms":  latency.Milliseconds(),
				"agent_type":  agent.Type(),
				"agent_name":  agent.Name(),
				"error_stage": "agent_execution",
			},
		}, err
	}

	// 5. 构建TaskResult (并记录 Token 使用)
	
	// 记录 Token 消耗
	if result.Usage != nil && e.auditService != nil {
		tokenUsage := &audit.TokenUsage{
			TenantID:         task.Context.TenantID,
			UserID:           task.Context.UserID,
			WorkflowID:       task.Context.WorkflowID,
			ExecutionID:      task.Context.ExecutionID,
			StepID:           task.Step.ID,
			AgentID:          agent.Name(), // 这里其实应该是 Agent ID，但 Agent interface 可能只暴露了 Name/Type
			Model:            getString(result.Metadata, "model_id"),
			Provider:         "unknown", // 需要从 Metadata 中获取更多信息
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
			EstimatedCost:    result.Cost,
		}
		// 异步记录，避免阻塞
		go func() {
			_ = e.auditService.RecordTokenUsage(tokenUsage)
		}()
	}

	return &TaskResult{
		ID:     task.ID,
		Output: result.Output,
		Status: result.Status,
		Metadata: map[string]any{
			"latency_ms": latency.Milliseconds(),
			"agent_type": agent.Type(),
			"agent_name": agent.Name(),
			"usage":      result.Usage,
			"cost":       result.Cost,
		},
	}, nil
}

// isContextLimitError 简单判断是否为上下文相关错误
// 这是一个启发式判断，因为不同模型厂商的错误码不同
func isContextLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// 包含常见关键词
	return strings.Contains(msg, "context length") ||
		strings.Contains(msg, "context window") ||
		strings.Contains(msg, "maximum length") ||
		strings.Contains(msg, "token limit") ||
		strings.Contains(msg, "too many tokens")
}

// getAgent 获取Agent实例
// 优先使用AgentID,其次使用AgentType
func (e *AgentTaskExecutor) getAgent(ctx context.Context, task *Task) (runtime.Agent, error) {
	// 优先使用AgentID
	if task.Step.AgentID != nil && *task.Step.AgentID != "" {
		return e.agentRegistry.GetAgent(ctx, task.Context.TenantID, *task.Step.AgentID)
	}

	// 否则使用AgentType
	if task.Step.AgentType != "" {
		return e.agentRegistry.GetAgentByType(ctx, task.Context.TenantID, task.Step.AgentType)
	}

	return nil, fmt.Errorf("缺少agent_id或agent_type")
}

// buildAgentInput 构建Agent输入
// 从Task转换为AgentInput,映射上下文信息
func (e *AgentTaskExecutor) buildAgentInput(task *Task) *runtime.AgentInput {
	// 提取content字段(主要输入)
	content := ""
	if contentVal, ok := task.Input["content"]; ok {
		if str, ok := contentVal.(string); ok {
			content = str
		}
	}

	// 构建AgentContext (映射ExecutionContext)
	agentCtx := &runtime.AgentContext{
		TenantID:   task.Context.TenantID,
		UserID:     task.Context.UserID,
		WorkflowID: &task.Context.WorkflowID,
		TraceID:    &task.Context.TraceID,
		StepID:     &task.Step.ID,
		Data:       e.buildSharedData(task.Context), // 只读快照
	}

	// 提取SessionID (如果有)
	if sessionID, ok := task.Input["session_id"].(string); ok && sessionID != "" {
		agentCtx.SessionID = &sessionID
	}

	return &runtime.AgentInput{
		Content:     content,
		Variables:   task.Input,
		Context:     agentCtx,
		ExtraParams: task.Step.ExtraConfig,
	}
}

// buildSharedData 构建共享数据快照(只读)
// 避免Agent直接修改工作流上下文
func (e *AgentTaskExecutor) buildSharedData(execCtx *ExecutionContext) map[string]any {
	// 使用ExecutionContext的GetAllData方法获取快照
	// 该方法已经包含了并发安全的读锁
	return execCtx.GetAllData()
}

// ExecuteTaskWithRetry 支持重试的任务执行
// 根据Step的RetryConfig自动重试失败的任务
func (e *AgentTaskExecutor) ExecuteTaskWithRetry(ctx context.Context, task *Task) (*TaskResult, error) {
	retryConfig := task.Step.Retry
	if retryConfig == nil {
		// 无重试配置,直接执行
		return e.ExecuteTask(ctx, task)
	}

	maxRetries := retryConfig.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // 默认重试3次
	}

	var lastErr error
	var lastResult *TaskResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return &TaskResult{
				ID:     task.ID,
				Status: "failed",
				Error:  fmt.Errorf("任务被取消: %w", ctx.Err()),
			}, ctx.Err()
		default:
		}

		// 第一次之后的尝试需要等待
		if attempt > 0 {
			delay := e.calculateBackoff(retryConfig, attempt)
			time.Sleep(delay)
		}

		// 执行任务
		result, err := e.ExecuteTask(ctx, task)
		if err == nil && result.Status == "success" {
			// 成功,返回结果
			if attempt > 0 {
				// 记录重试次数
				if result.Metadata == nil {
					result.Metadata = make(map[string]any)
				}
				result.Metadata["retry_attempts"] = attempt
			}
			return result, nil
		}

		// 记录失败
		lastErr = err
		lastResult = result

		// 如果是上下文超限错误，并且没有配置自动缩减重试，则立即停止重试（因为再次重试大概率还是失败）
		// 除非后续实现了自动截断输入的逻辑
		if isContextLimitError(err) {
			// 可以选择 break 提前结束
			// break
		}
	}

	// 所有重试都失败
	if lastResult != nil {
		if lastResult.Metadata == nil {
			lastResult.Metadata = make(map[string]any)
		}
		lastResult.Metadata["retry_attempts"] = maxRetries
		lastResult.Metadata["final_failure"] = true
		lastResult.Error = fmt.Errorf("重试%d次后仍失败: %w", maxRetries, lastErr)
		return lastResult, lastErr
	}

	return &TaskResult{
		ID:     task.ID,
		Status: "failed",
		Error:  fmt.Errorf("重试%d次后仍失败: %w", maxRetries, lastErr),
		Metadata: map[string]any{
			"retry_attempts": maxRetries,
			"final_failure":  true,
		},
	}, lastErr
}

// calculateBackoff 计算退避延迟
// 支持固定延迟和指数退避两种策略
func (e *AgentTaskExecutor) calculateBackoff(retry *RetryConfig, attempt int) time.Duration {
	baseDelay := time.Duration(retry.Delay) * time.Second
	if baseDelay == 0 {
		baseDelay = 1 * time.Second // 默认1秒
	}

	switch retry.Backoff {
	case "exponential":
		// 指数退避: delay * 2^attempt
		// 限制最大延迟为60秒
		delay := baseDelay * time.Duration(1<<uint(attempt))
		if delay > 60*time.Second {
			delay = 60 * time.Second
		}
		return delay
	default:
		// 固定延迟
		return baseDelay
	}
}

// executeMapReduce 执行 Map-Reduce 任务
func (e *AgentTaskExecutor) executeMapReduce(ctx context.Context, task *Task) (*TaskResult, error) {
	cfg := task.Step.MapReduce
	start := time.Now()

	// 1. 获取输入列表
	listVal, ok := task.Input[cfg.IterateOn]
	if !ok {
		return &TaskResult{
			ID:     task.ID,
			Status: "failed",
			Error:  fmt.Errorf("map_reduce: iterate_on field '%s' not found", cfg.IterateOn),
		}, nil
	}

	var items []any
	switch v := listVal.(type) {
	case []any:
		items = v
	case []string:
		for _, s := range v {
			items = append(items, s)
		}
	default:
		// 尝试作为单一元素列表? 否，报错
		return &TaskResult{
			ID:     task.ID,
			Status: "failed",
			Error:  fmt.Errorf("map_reduce: field '%s' is not a list (got %T)", cfg.IterateOn, listVal),
		}, nil
	}

	// 2. Map 阶段 (并行执行)
	concurrency := cfg.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	sem := make(chan struct{}, concurrency)
	results := make([]any, len(items))
	errs := make([]error, len(items))
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, val any) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// 检查 Context
			if ctx.Err() != nil {
				errs[idx] = ctx.Err()
				return
			}

			// 构建子任务输入
			subInput := make(map[string]any)
			// 复制父任务输入
			for k, v := range task.Input {
				subInput[k] = v
			}
			// 注入 Item
			subInput[cfg.ItemVariable] = val

			// 创建子任务
			// 注意: 必须禁用 map_reduce 以避免递归
			subStep := *task.Step
			subStep.MapReduce = nil

			subTask := &Task{
				ID:      fmt.Sprintf("%s_%d", task.ID, idx),
				Step:    &subStep,
				Input:   subInput,
				Context: task.Context,
			}

			res, err := e.executeStandardAgent(ctx, subTask)
			if err != nil {
				errs[idx] = err
			} else if res.Status == "failed" {
				errs[idx] = res.Error
			} else {
				results[idx] = res.Output
			}
		}(i, item)
	}

	wg.Wait()

	// 检查错误
	for _, err := range errs {
		if err != nil {
			return &TaskResult{
				ID:     task.ID,
				Status: "failed",
				Error:  fmt.Errorf("map task failed: %w", err),
			}, nil
		}
	}

	mapEnd := time.Now()
	mapDuration := mapEnd.Sub(start)
	finalOutput := any(results)
	metadata := map[string]any{
		"map_count":       len(items),
		"map_duration_ms": mapDuration.Milliseconds(),
	}

	// 3. Reduce 阶段：可选 Reducer Agent
	if cfg.ReducerAgent != "" {
		reducerInput := e.buildReducerInput(task.Input, items, results, mapDuration)
		reducerStep := *task.Step
		reducerStep.MapReduce = nil
		reducerID := cfg.ReducerAgent
		reducerStep.AgentID = &reducerID

		reducerTask := &Task{
			ID:      fmt.Sprintf("%s_reduce", task.ID),
			Step:    &reducerStep,
			Input:   reducerInput,
			Context: task.Context,
		}

		reduceResult, err := e.executeStandardAgent(ctx, reducerTask)
		if err != nil {
			return &TaskResult{
				ID:     task.ID,
				Status: "failed",
				Error:  fmt.Errorf("reduce agent 执行失败: %w", err),
			}, nil
		}
		if reduceResult.Status != "success" {
			return &TaskResult{
				ID:     task.ID,
				Status: "failed",
				Error:  fmt.Errorf("reduce agent 返回失败: %v", reduceResult.Error),
				Metadata: map[string]any{
					"reduce_agent": reducerID,
				},
			}, nil
		}

		finalOutput = reduceResult.Output
		metadata["reduce_agent"] = reducerID
		if reduceResult.Metadata != nil {
			metadata["reduce_metadata"] = reduceResult.Metadata
		}
	}

	metadata["latency_ms"] = time.Since(start).Milliseconds()
	return &TaskResult{
		ID:       task.ID,
		Status:   "success",
		Output:   finalOutput,
		Metadata: metadata,
	}, nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// buildReducerInput 构建 Reduce 阶段输入
func (e *AgentTaskExecutor) buildReducerInput(original map[string]any, items []any, results []any, mapDuration time.Duration) map[string]any {
	reducerInput := make(map[string]any, len(original)+3)
	for k, v := range original {
		reducerInput[k] = v
	}

	reducerInput["map_results"] = results
	reducerInput["map_items"] = items
	reducerInput["map_statistics"] = map[string]any{
		"count":       len(results),
		"duration_ms": mapDuration.Milliseconds(),
	}

	return reducerInput
}
