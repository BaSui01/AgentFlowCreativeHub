package executor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ParallelExecutor 并行任务执行器
// 用于同时执行多个独立任务，提升工作流执行效率
type ParallelExecutor struct {
	maxConcurrency int
	taskExecutor   TaskExecutor
}

// NewParallelExecutor 创建并行执行器
func NewParallelExecutor(maxConcurrency int, taskExecutor TaskExecutor) *ParallelExecutor {
	if maxConcurrency <= 0 {
		maxConcurrency = 5 // 默认最大并发数
	}
	return &ParallelExecutor{
		maxConcurrency: maxConcurrency,
		taskExecutor:   taskExecutor,
	}
}

// ParallelResult 并行执行结果
type ParallelResult struct {
	TaskID string
	Result *TaskResult
	Error  error
}

// ExecuteParallel 并行执行多个任务
// 使用 goroutine 和 channel 实现并发控制
// 限制最大并发数，避免资源耗尽
func (e *ParallelExecutor) ExecuteParallel(ctx context.Context, tasks []*Task, execCtx *ExecutionContext) (map[string]*TaskResult, error) {
	if len(tasks) == 0 {
		return make(map[string]*TaskResult), nil
	}

	// 创建取消上下文，用于提前终止
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 结果和错误通道
	resultChan := make(chan *ParallelResult, len(tasks))
	
	// 使用 WaitGroup 等待所有任务完成
	var wg sync.WaitGroup
	
	// 使用 channel 实现并发限制
	semaphore := make(chan struct{}, e.maxConcurrency)

	// 启动所有任务
	for _, task := range tasks {
		wg.Add(1)
		go func(t *Task) {
			defer wg.Done()
			// 确保任务上下文已设置
			t.Context = execCtx

			// 获取信号量（限制并发数）
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }() // 释放信号量
			case <-ctx.Done():
				resultChan <- &ParallelResult{
					TaskID: t.ID,
					Error:  ctx.Err(),
				}
				return
			}

			// 执行任务
			result, err := e.taskExecutor.ExecuteTask(ctx, t)
			
			// 发送结果
			resultChan <- &ParallelResult{
				TaskID: t.ID,
				Result: result,
				Error:  err,
			}
		}(task)
	}

	// 等待所有任务完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	results := make(map[string]*TaskResult)
	var firstError error

	for result := range resultChan {
		if result.Error != nil {
			if firstError == nil {
				firstError = result.Error
				// 第一个错误发生时，取消其他任务
				cancel()
			}
			continue
		}

		if result.Result != nil {
			results[result.TaskID] = result.Result
		}
	}

	// 如果有任何任务失败，返回错误
	if firstError != nil {
		return results, fmt.Errorf("并行任务执行失败: %w", firstError)
	}

	return results, nil
}

// ExecuteParallelWithTimeout 并行执行任务（带超时）
func (e *ParallelExecutor) ExecuteParallelWithTimeout(ctx context.Context, tasks []*Task, execCtx *ExecutionContext, timeout time.Duration) (map[string]*TaskResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return e.ExecuteParallel(ctx, tasks, execCtx)
}

// ExecuteParallelBestEffort 并行执行任务（最大努力模式）
// 即使部分任务失败，也会继续执行其他任务
// 返回成功的结果和失败任务的错误映射
func (e *ParallelExecutor) ExecuteParallelBestEffort(ctx context.Context, tasks []*Task, execCtx *ExecutionContext) (map[string]*TaskResult, map[string]error) {
	if len(tasks) == 0 {
		return make(map[string]*TaskResult), make(map[string]error)
	}

	// 结果和错误通道
	resultChan := make(chan *ParallelResult, len(tasks))
	
	// 使用 WaitGroup 等待所有任务完成
	var wg sync.WaitGroup
	
	// 使用 channel 实现并发限制
	semaphore := make(chan struct{}, e.maxConcurrency)

	// 启动所有任务
	for _, task := range tasks {
		wg.Add(1)
		go func(t *Task) {
			defer wg.Done()
			// 确保任务上下文已设置
			t.Context = execCtx

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 执行任务
			result, err := e.taskExecutor.ExecuteTask(ctx, t)
			
			// 发送结果
			resultChan <- &ParallelResult{
				TaskID: t.ID,
				Result: result,
				Error:  err,
			}
		}(task)
	}

	// 等待所有任务完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	results := make(map[string]*TaskResult)
	errors := make(map[string]error)

	for result := range resultChan {
		if result.Error != nil {
			errors[result.TaskID] = result.Error
		} else if result.Result != nil {
			results[result.TaskID] = result.Result
		}
	}

	return results, errors
}
