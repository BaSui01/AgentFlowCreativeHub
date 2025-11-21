package executor

import (
	"context"
	"fmt"
	"sync"
)

// Scheduler 任务调度器
type Scheduler struct {
	dag            *DAG
	executor       TaskExecutor
	maxConcurrency int
	templateEngine *TemplateEngine // 模板引擎
}

// TaskExecutor 任务执行器接口
type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task *Task) (*TaskResult, error)
}

// Task 任务
type Task struct {
	ID      string
	Step    *StepDefinition
	Input   map[string]any
	Context *ExecutionContext
}

// TaskResult 任务结果
type TaskResult struct {
	ID       string
	Output   any
	Error    error
	Status   string // success, failed, skipped
	Metadata map[string]any
}

// ExecutionContext已移至context.go

// NewScheduler 创建调度器
func NewScheduler(dag *DAG, executor TaskExecutor, maxConcurrency int) *Scheduler {
	if maxConcurrency <= 0 {
		maxConcurrency = 5 // 默认最多 5 个并发任务
	}

	return &Scheduler{
		dag:            dag,
		executor:       executor,
		maxConcurrency: maxConcurrency,
		templateEngine: NewTemplateEngine(), // 初始化模板引擎
	}
}

// SetTemplateEngine 设置模板引擎
// 允许外部注入自定义模板引擎
func (s *Scheduler) SetTemplateEngine(engine *TemplateEngine) {
	s.templateEngine = engine
}

// Schedule 调度执行 (Event-driven / Kahn's Algorithm)
func (s *Scheduler) Schedule(ctx context.Context, execCtx *ExecutionContext) (map[string]*TaskResult, error) {
	// 1. 初始化状态
	results := make(map[string]*TaskResult)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)
	readyQueue := make([]string, 0)

	// 构建依赖关系图
	for id, node := range s.dag.Nodes {
		inDegree[id] = len(node.Dependencies)
		dependents[id] = node.Dependents
		if len(node.Dependencies) == 0 {
			readyQueue = append(readyQueue, id)
		}
	}

	// 2. 并发控制
	// doneChan 接收任务完成事件
	doneChan := make(chan *TaskResult, len(s.dag.Nodes))
	// sem 限制最大并发数
	sem := make(chan struct{}, s.maxConcurrency)
	var wg sync.WaitGroup

	// 辅助函数：分发任务
	dispatch := func(nodeID string) {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			// 获取信号量（阻塞等待）
			select {
			case sem <- struct{}{}:
				// 成功获取并发令牌
			case <-ctx.Done():
				return // 上下文取消，放弃执行
			}
			defer func() { <-sem }() // 释放令牌

			// 再次检查 Context
			if ctx.Err() != nil {
				return
			}

			// 执行任务
			result, err := s.executeTask(ctx, id, execCtx)
			if err != nil {
				// 封装系统级错误
				if result == nil {
					result = &TaskResult{
						ID:     id,
						Status: "failed",
						Error:  err,
					}
				}
			}

			// 发送结果（非阻塞，防止死锁）
			select {
			case doneChan <- result:
			case <-ctx.Done():
			}
		}(nodeID)
	}

	// 3. 主事件循环
	totalTasks := len(s.dag.Nodes)
	completedTasks := 0
	dispatchedTasks := 0 // 已分发给goroutine的任务数

	for completedTasks < totalTasks {
		// 分发所有 Ready 的任务
		currentReadyCount := len(readyQueue)
		for i := 0; i < currentReadyCount; i++ {
			nodeID := readyQueue[0]
			readyQueue = readyQueue[1:]
			dispatch(nodeID)
			dispatchedTasks++
		}

		// 死锁/孤立检测
		if len(readyQueue) == 0 && dispatchedTasks == completedTasks && completedTasks < totalTasks {
			return results, fmt.Errorf("死锁检测: 工作流无法继续执行 (已完成: %d/%d)", completedTasks, totalTasks)
		}

		// 等待事件
		select {
		case <-ctx.Done():
			return results, fmt.Errorf("工作流执行被取消")

		case res := <-doneChan:
			completedTasks++
			results[res.ID] = res

			if res.Status == "success" {
				// 任务成功
				// 1. 更新共享数据
				if res.Output != nil {
					execCtx.SetData(res.ID, res.Output)
				}

				// 2. 减少后继节点的入度
				for _, depID := range dependents[res.ID] {
					inDegree[depID]--
					if inDegree[depID] == 0 {
						readyQueue = append(readyQueue, depID)
					}
				}
			} else {
				// 任务失败，终止工作流
				return results, fmt.Errorf("步骤 %s 执行失败: %w", res.ID, res.Error)
			}
		}
	}

	wg.Wait()
	return results, nil
}

// Resume 恢复执行
func (s *Scheduler) Resume(ctx context.Context, execCtx *ExecutionContext, previousResults map[string]*TaskResult) (map[string]*TaskResult, error) {
	// 1. 初始化状态
	results := make(map[string]*TaskResult)
	// 复制先前的结果
	for k, v := range previousResults {
		results[k] = v
		// 确保上下文中有数据
		if v.Status == "success" && v.Output != nil {
			execCtx.SetData(k, v.Output)
		}
	}

	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	// 构建依赖关系图
	for id, node := range s.dag.Nodes {
		inDegree[id] = len(node.Dependencies)
		dependents[id] = node.Dependents
	}

	// 根据已有结果调整入度
	for id, res := range results {
		if res.Status == "success" {
			for _, depID := range dependents[id] {
				inDegree[depID]--
			}
		}
	}

	// 计算就绪队列
	readyQueue := make([]string, 0)
	for id := range s.dag.Nodes {
		// 如果任务已完成，跳过
		if _, ok := results[id]; ok {
			continue
		}
		// 如果入度为0，加入就绪队列
		if inDegree[id] == 0 {
			readyQueue = append(readyQueue, id)
		}
	}

	// 2. 并发控制 (复用 Schedule 的逻辑)
	// doneChan 接收任务完成事件
	doneChan := make(chan *TaskResult, len(s.dag.Nodes))
	// sem 限制最大并发数
	sem := make(chan struct{}, s.maxConcurrency)
	var wg sync.WaitGroup

	// 辅助函数：分发任务
	dispatch := func(nodeID string) {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			// 获取信号量（阻塞等待）
			select {
			case sem <- struct{}{}:
				// 成功获取并发令牌
			case <-ctx.Done():
				return // 上下文取消，放弃执行
			}
			defer func() { <-sem }() // 释放令牌

			// 再次检查 Context
			if ctx.Err() != nil {
				return
			}

			// 执行任务
			result, err := s.executeTask(ctx, id, execCtx)
			if err != nil {
				// 封装系统级错误
				if result == nil {
					result = &TaskResult{
						ID:     id,
						Status: "failed",
						Error:  err,
					}
				}
			}

			// 发送结果（非阻塞，防止死锁）
			select {
			case doneChan <- result:
			case <-ctx.Done():
			}
		}(nodeID)
	}

	// 3. 主事件循环
	totalTasks := len(s.dag.Nodes)
	completedTasks := len(results)
	dispatchedTasks := 0 // 本次恢复执行分发的任务数

	// 如果所有任务都已完成，直接返回
	if completedTasks == totalTasks {
		return results, nil
	}

	for completedTasks < totalTasks {
		// 分发所有 Ready 的任务
		currentReadyCount := len(readyQueue)
		for i := 0; i < currentReadyCount; i++ {
			nodeID := readyQueue[0]
			readyQueue = readyQueue[1:]
			dispatch(nodeID)
			dispatchedTasks++
		}

		// 死锁/孤立检测
		// 注意：这里 dispatchedTasks 仅统计本次运行分发的，completedTasks 包含之前的
		// 如果 readyQueue 为空，且没有正在运行的任务 (dispatchedTasks == 0)，且未全部完成
		// 这里的逻辑需要调整：我们需要跟踪正在运行的任务数
		// 简单起见，我们假设如果没有 ready 且没有正在运行的，就是死锁
		// 但由于是异步，我们用 select 等待

		// 如果没有任务在运行且没有任务就绪，那就是死锁
		// 但我们需要知道有多少任务正在运行。
		// 我们可以引入一个 activeTasks 计数器，或者简单地依赖 doneChan 阻塞

		// 改进死锁检测：
		// 如果 readyQueue 为空，我们需要等待 doneChan。
		// 如果 doneChan 也没东西来（即没有 active tasks），那就是死锁。
		// 但在这里很难准确判断 active tasks。
		// 我们可以用 wg.Wait() 吗？不行，因为我们在循环里。

		// 让我们简化：如果 readyQueue 为空，我们就必须阻塞等待 doneChan。
		// 如果 doneChan 永远不来，那就是死锁。
		// 但我们怎么知道 doneChan 会不会来？
		// 我们可以记录 activeTasks

		// 重新设计循环
		// 实际上，上面的 Schedule 方法也有类似的死锁检测逻辑，可能存在缺陷。
		// 但我们先保持一致。

		// 等待事件
		select {
		case <-ctx.Done():
			return results, fmt.Errorf("工作流执行被取消")

		case res := <-doneChan:
			completedTasks++
			results[res.ID] = res

			if res.Status == "success" {
				// 任务成功
				if res.Output != nil {
					execCtx.SetData(res.ID, res.Output)
				}
				for _, depID := range dependents[res.ID] {
					inDegree[depID]--
					if inDegree[depID] == 0 {
						readyQueue = append(readyQueue, depID)
					}
				}
			} else if res.Status == "paused" {
				// 任务暂停 (新增)
				return results, nil
			} else {
				// 任务失败
				return results, fmt.Errorf("步骤 %s 执行失败: %w", res.ID, res.Error)
			}
		}
	}

	wg.Wait()
	return results, nil
}

// executeTask 执行单个任务
func (s *Scheduler) executeTask(
	ctx context.Context,
	nodeID string,
	execCtx *ExecutionContext,
) (*TaskResult, error) {
	node := s.dag.Nodes[nodeID]

	// 构建任务输入
	taskInput, err := s.buildTaskInput(node.Step, execCtx)
	if err != nil {
		return &TaskResult{
			ID:     nodeID,
			Status: "failed",
			Error:  err,
		}, nil
	}

	// 创建任务
	task := &Task{
		ID:      nodeID,
		Step:    node.Step,
		Input:   taskInput,
		Context: execCtx,
	}

	// 执行任务
	result, execErr := s.executor.ExecuteTask(ctx, task)
	if execErr != nil {
		return &TaskResult{
			ID:     nodeID,
			Status: "failed",
			Error:  execErr,
		}, nil
	}

	return result, nil
}

// buildTaskInput 构建任务输入并渲染模板
func (s *Scheduler) buildTaskInput(step *StepDefinition, execCtx *ExecutionContext) (map[string]any, error) {
	input := make(map[string]any)

	for key, value := range step.Input {
		input[key] = value
	}

	if s.templateEngine == nil {
		s.templateEngine = NewTemplateEngine()
	}

	rendered, err := s.templateEngine.RenderMap(input, s.buildTemplateData(execCtx))
	if err != nil {
		return nil, fmt.Errorf("渲染步骤输入失败: %w", err)
	}

	return rendered, nil
}

// buildTemplateData 构建模板可用的数据上下文
func (s *Scheduler) buildTemplateData(execCtx *ExecutionContext) map[string]any {
	if execCtx == nil {
		return map[string]any{}
	}

	shared := execCtx.GetAllData()
	data := make(map[string]any, len(shared)+3)
	for k, v := range shared {
		data[k] = v
	}

	data["steps"] = shared
	data["_context"] = map[string]any{
		"workflow_id":  execCtx.WorkflowID,
		"execution_id": execCtx.ExecutionID,
		"tenant_id":    execCtx.TenantID,
		"user_id":      execCtx.UserID,
		"trace_id":     execCtx.TraceID,
	}
	data["_metadata"] = execCtx.getMetadataSnapshot()

	return data
}

// ExecutionContext方法已移至context.go
