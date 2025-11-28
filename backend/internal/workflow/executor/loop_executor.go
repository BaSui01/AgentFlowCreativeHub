package executor

import (
	"context"
	"fmt"
	"time"
)

// LoopNodeType 循环节点类型
const LoopNodeType = "loop"

// LoopConfig 循环节点配置
type LoopConfig struct {
	// 循环类型
	LoopType string `json:"loopType"` // count, while, foreach

	// count 类型：固定次数循环
	MaxIterations int `json:"maxIterations"` // 最大循环次数

	// while 类型：条件循环
	Condition string `json:"condition"` // 循环条件表达式

	// foreach 类型：遍历循环
	Collection    string `json:"collection"`    // 要遍历的数组变量名
	ItemVariable  string `json:"itemVariable"`  // 当前项变量名
	IndexVariable string `json:"indexVariable"` // 索引变量名

	// 通用配置
	BreakCondition         string `json:"breakCondition"`         // 提前退出条件
	ContinueOnError        bool   `json:"continueOnError"`        // 错误时是否继续
	DelayBetweenIterations int    `json:"delayBetweenIterations"` // 迭代间隔(毫秒)

	// 循环体内的步骤（嵌套步骤）
	BodySteps []map[string]any `json:"bodySteps"`
}

// LoopExecutor 循环节点执行器
type LoopExecutor struct {
	stepExecutor StepExecutorFunc
}

// StepExecutorFunc 步骤执行函数类型
type StepExecutorFunc func(ctx context.Context, stepDef *StepDefinition, execCtx *ExecutionContext) (*TaskResult, error)

// NewLoopExecutor 创建循环执行器
func NewLoopExecutor(stepExecutor StepExecutorFunc) *LoopExecutor {
	return &LoopExecutor{stepExecutor: stepExecutor}
}

// LoopState 循环状态
type LoopState struct {
	CurrentIteration int            `json:"currentIteration"`
	TotalIterations  int            `json:"totalIterations"`
	Results          []any          `json:"results"`
	Variables        map[string]any `json:"variables"`
	Completed        bool           `json:"completed"`
	Error            string         `json:"error,omitempty"`
}

// Execute 执行循环节点
func (e *LoopExecutor) Execute(ctx context.Context, execCtx *ExecutionContext, step *StepDefinition) (*TaskResult, error) {
	config, err := e.parseConfig(step)
	if err != nil {
		return nil, fmt.Errorf("解析循环配置失败: %w", err)
	}

	loopState := &LoopState{
		Results:   make([]any, 0),
		Variables: make(map[string]any),
	}

	switch config.LoopType {
	case "count":
		return e.executeCountLoop(ctx, execCtx, config, loopState)
	case "while":
		return e.executeWhileLoop(ctx, execCtx, config, loopState)
	case "foreach":
		return e.executeForEachLoop(ctx, execCtx, config, loopState)
	default:
		return nil, fmt.Errorf("不支持的循环类型: %s", config.LoopType)
	}
}

// parseConfig 解析循环配置
func (e *LoopExecutor) parseConfig(step *StepDefinition) (*LoopConfig, error) {
	config := &LoopConfig{
		MaxIterations: 100, // 默认最大 100 次
		ItemVariable:  "item",
		IndexVariable: "index",
	}

	// 从 step.Input 解析循环配置
	input := step.Input
	if input == nil {
		return nil, fmt.Errorf("缺少循环配置")
	}

	// 解析配置
	if v, ok := input["loopType"].(string); ok {
		config.LoopType = v
	}
	if v, ok := input["maxIterations"].(float64); ok {
		config.MaxIterations = int(v)
	}
	if v, ok := input["condition"].(string); ok {
		config.Condition = v
	}
	if v, ok := input["collection"].(string); ok {
		config.Collection = v
	}
	if v, ok := input["itemVariable"].(string); ok {
		config.ItemVariable = v
	}
	if v, ok := input["indexVariable"].(string); ok {
		config.IndexVariable = v
	}
	if v, ok := input["breakCondition"].(string); ok {
		config.BreakCondition = v
	}
	if v, ok := input["continueOnError"].(bool); ok {
		config.ContinueOnError = v
	}
	if v, ok := input["delayBetweenIterations"].(float64); ok {
		config.DelayBetweenIterations = int(v)
	}
	if v, ok := input["bodySteps"].([]interface{}); ok {
		config.BodySteps = make([]map[string]any, len(v))
		for i, s := range v {
			if m, ok := s.(map[string]interface{}); ok {
				config.BodySteps[i] = m
			}
		}
	}

	return config, nil
}

// executeCountLoop 执行固定次数循环
func (e *LoopExecutor) executeCountLoop(ctx context.Context, execCtx *ExecutionContext, config *LoopConfig, state *LoopState) (*TaskResult, error) {
	state.TotalIterations = config.MaxIterations

	for i := 0; i < config.MaxIterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		state.CurrentIteration = i
		state.Variables[config.IndexVariable] = i

		// 执行循环体
		result, err := e.executeBody(ctx, execCtx, config, state)
		if err != nil {
			if config.ContinueOnError {
				state.Results = append(state.Results, map[string]any{"error": err.Error()})
				continue
			}
			return nil, err
		}

		state.Results = append(state.Results, result)

		// 检查 break 条件
		if config.BreakCondition != "" {
			shouldBreak, _ := e.evaluateCondition(execCtx, config.BreakCondition)
			if shouldBreak {
				break
			}
		}

		// 迭代间隔
		if config.DelayBetweenIterations > 0 {
			time.Sleep(time.Duration(config.DelayBetweenIterations) * time.Millisecond)
		}
	}

	state.Completed = true
	return &TaskResult{
		ID: "loop",
		Output: map[string]any{
			"iterations": state.CurrentIteration + 1,
			"results":    state.Results,
		},
	}, nil
}

// executeWhileLoop 执行条件循环
func (e *LoopExecutor) executeWhileLoop(ctx context.Context, execCtx *ExecutionContext, config *LoopConfig, state *LoopState) (*TaskResult, error) {
	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 1000 // 安全限制
	}

	for i := 0; i < maxIterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 检查循环条件
		shouldContinue, err := e.evaluateCondition(execCtx, config.Condition)
		if err != nil {
			return nil, fmt.Errorf("评估循环条件失败: %w", err)
		}
		if !shouldContinue {
			break
		}

		state.CurrentIteration = i
		state.Variables[config.IndexVariable] = i

		// 执行循环体
		result, err := e.executeBody(ctx, execCtx, config, state)
		if err != nil {
			if config.ContinueOnError {
				state.Results = append(state.Results, map[string]any{"error": err.Error()})
				continue
			}
			return nil, err
		}

		state.Results = append(state.Results, result)

		// 迭代间隔
		if config.DelayBetweenIterations > 0 {
			time.Sleep(time.Duration(config.DelayBetweenIterations) * time.Millisecond)
		}
	}

	state.Completed = true
	return &TaskResult{
		ID: "loop",
		Output: map[string]any{
			"iterations": state.CurrentIteration + 1,
			"results":    state.Results,
		},
	}, nil
}

// executeForEachLoop 执行遍历循环
func (e *LoopExecutor) executeForEachLoop(ctx context.Context, execCtx *ExecutionContext, config *LoopConfig, state *LoopState) (*TaskResult, error) {
	// 获取要遍历的集合
	collection, err := e.getCollection(execCtx, config.Collection)
	if err != nil {
		return nil, fmt.Errorf("获取集合失败: %w", err)
	}

	state.TotalIterations = len(collection)

	// 安全限制
	if state.TotalIterations > config.MaxIterations {
		state.TotalIterations = config.MaxIterations
	}

	for i := 0; i < state.TotalIterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		state.CurrentIteration = i
		state.Variables[config.IndexVariable] = i
		state.Variables[config.ItemVariable] = collection[i]

		// 将当前项注入到执行上下文
		execCtx.SetData(config.ItemVariable, collection[i])
		execCtx.SetData(config.IndexVariable, i)

		// 执行循环体
		result, err := e.executeBody(ctx, execCtx, config, state)
		if err != nil {
			if config.ContinueOnError {
				state.Results = append(state.Results, map[string]any{"error": err.Error()})
				continue
			}
			return nil, err
		}

		state.Results = append(state.Results, result)

		// 检查 break 条件
		if config.BreakCondition != "" {
			shouldBreak, _ := e.evaluateCondition(execCtx, config.BreakCondition)
			if shouldBreak {
				break
			}
		}

		// 迭代间隔
		if config.DelayBetweenIterations > 0 {
			time.Sleep(time.Duration(config.DelayBetweenIterations) * time.Millisecond)
		}
	}

	state.Completed = true
	return &TaskResult{
		ID: "loop",
		Output: map[string]any{
			"iterations": state.CurrentIteration + 1,
			"results":    state.Results,
		},
	}, nil
}

// executeBody 执行循环体
func (e *LoopExecutor) executeBody(ctx context.Context, execCtx *ExecutionContext, config *LoopConfig, state *LoopState) (any, error) {
	if len(config.BodySteps) == 0 {
		return nil, nil
	}

	// 执行循环体内的步骤
	bodyResults := make(map[string]any)

	for _, stepConfig := range config.BodySteps {
		stepID, _ := stepConfig["id"].(string)
		if stepID == "" {
			continue
		}

		// 创建子步骤定义
		stepType, _ := stepConfig["type"].(string)
		subStep := &StepDefinition{
			ID:   stepID,
			Type: stepType,
		}
		if input, ok := stepConfig["input"].(map[string]any); ok {
			subStep.Input = input
		}

		// 如果有步骤执行器，执行子步骤
		if e.stepExecutor != nil {
			result, err := e.stepExecutor(ctx, subStep, execCtx)
			if err != nil {
				bodyResults[stepID] = map[string]any{"error": err.Error()}
			} else if result != nil {
				bodyResults[stepID] = result.Output
			}
		} else {
			// 简化处理
			bodyResults[stepID] = map[string]any{
				"executed":  true,
				"iteration": state.CurrentIteration,
			}
		}
	}

	return bodyResults, nil
}

// evaluateCondition 评估条件表达式
func (e *LoopExecutor) evaluateCondition(execCtx *ExecutionContext, condition string) (bool, error) {
	// 简化的条件评估
	if condition == "" {
		return false, nil
	}

	// 委托给条件执行器
	condExecutor := NewConditionExecutor(nil)
	return condExecutor.EvaluateCondition(condition, execCtx)
}

// getCollection 从上下文获取集合
func (e *LoopExecutor) getCollection(execCtx *ExecutionContext, collectionExpr string) ([]any, error) {
	// 从共享数据中获取
	if v, ok := execCtx.GetShared(collectionExpr); ok {
		if arr, ok := v.([]any); ok {
			return arr, nil
		}
		if arr, ok := v.([]interface{}); ok {
			result := make([]any, len(arr))
			for i, item := range arr {
				result[i] = item
			}
			return result, nil
		}
		// 尝试转换字符串数组
		if arr, ok := v.([]string); ok {
			result := make([]any, len(arr))
			for i, item := range arr {
				result[i] = item
			}
			return result, nil
		}
	}

	// 尝试从所有数据中查找
	allData := execCtx.GetAllData()
	for key, val := range allData {
		if key == collectionExpr {
			if arr, ok := val.([]any); ok {
				return arr, nil
			}
		}
	}

	return nil, fmt.Errorf("找不到集合: %s", collectionExpr)
}
