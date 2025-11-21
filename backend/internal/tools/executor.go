package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ToolExecutor 工具执行引擎
type ToolExecutor struct {
	registry *ToolRegistry
	db       *gorm.DB
}

// NewToolExecutor 创建工具执行引擎
func NewToolExecutor(registry *ToolRegistry, db *gorm.DB) *ToolExecutor {
	return &ToolExecutor{
		registry: registry,
		db:       db,
	}
}

// Execute 执行工具
func (e *ToolExecutor) Execute(ctx context.Context, req *ToolExecutionRequest) (*ToolExecutionResult, error) {
	// 1. 查找工具
	handler, exists := e.registry.Get(req.ToolName)
	if !exists {
		return nil, fmt.Errorf("工具 %s 未注册", req.ToolName)
	}
	
	// 2. 验证参数
	if err := handler.Validate(req.Input); err != nil {
		return nil, fmt.Errorf("参数验证失败: %w", err)
	}
	
	// 3. 创建执行记录
	execution := &ToolExecution{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		ToolID:      req.ToolID,
		ToolName:    req.ToolName,
		AgentID:     req.AgentID,
		WorkflowID:  req.WorkflowID,
		ExecutionID: req.ExecutionID,
		Input:       req.Input,
		Status:      "running",
		StartedAt:   time.Now(),
	}
	
	if e.db != nil {
		if err := e.db.Create(execution).Error; err != nil {
			return nil, fmt.Errorf("创建执行记录失败: %w", err)
		}
	}
	
	// 4. 执行工具（带超时）
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 30 // 默认30秒
	}
	
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	
	startTime := time.Now()
	output, err := handler.Execute(execCtx, req.Input)
	duration := time.Since(startTime).Milliseconds()
	
	// 5. 更新执行记录
	now := time.Now()
	execution.CompletedAt = &now
	execution.Duration = duration
	
	if err != nil {
		errMsg := err.Error()
		execution.ErrorMessage = &errMsg
		execution.Status = "failed"
	} else {
		execution.Output = output
		execution.Status = "success"
	}
	
	if e.db != nil {
		e.db.Save(execution)
	}
	
	// 6. 返回结果
	return &ToolExecutionResult{
		ExecutionID: execution.ID,
		ToolName:    req.ToolName,
		Output:      output,
		Error:       err,
		Duration:    duration,
	}, err
}

// ExecuteBatch 批量执行工具（并行）
func (e *ToolExecutor) ExecuteBatch(ctx context.Context, requests []*ToolExecutionRequest) []*ToolExecutionResult {
	results := make([]*ToolExecutionResult, len(requests))
	
	// 使用 goroutine 并行执行
	var wg sync.WaitGroup
	for i, req := range requests {
		wg.Add(1)
		go func(index int, request *ToolExecutionRequest) {
			defer wg.Done()
			result, _ := e.Execute(ctx, request)
			results[index] = result
		}(i, req)
	}
	
	wg.Wait()
	return results
}

// ToolExecutionRequest 工具执行请求
type ToolExecutionRequest struct {
	TenantID    string         // 租户ID
	ToolID      string         // 工具ID
	ToolName    string         // 工具名称
	Input       map[string]any // 输入参数
	AgentID     string         // Agent ID
	WorkflowID  *string        // 工作流ID（可选）
	ExecutionID *string        // 执行ID（可选）
	Timeout     int            // 超时时间（秒）
}

// ToolExecutionResult 工具执行结果
type ToolExecutionResult struct {
	ExecutionID string         // 执行记录ID
	ToolName    string         // 工具名称
	Output      map[string]any // 输出结果
	Error       error          // 错误信息
	Duration    int64          // 执行时长（毫秒）
}
