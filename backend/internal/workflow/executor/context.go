package executor

import (
	"sync"

	"backend/internal/agent/runtime"
)

// ExecutionContext 工作流执行上下文(增强版)
// 管理工作流执行过程中的全局信息和步骤间共享数据
type ExecutionContext struct {
	// === 全局信息 (不可变) ===
	WorkflowID  string // 工作流ID
	ExecutionID string // 执行实例ID
	TenantID    string // 租户ID
	UserID      string // 用户ID
	TraceID     string // 追踪ID(用于分布式追踪)

	// === 共享数据 (可读写) ===
	Data map[string]any // 步骤间共享数据

	// === 元数据 ===
	Metadata map[string]any // 额外元数据

	// === 并发控制 ===
	mu sync.RWMutex
}

// NewExecutionContext 创建工作流执行上下文
func NewExecutionContext(workflowID, executionID, tenantID, userID string) *ExecutionContext {
	return &ExecutionContext{
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		TenantID:    tenantID,
		UserID:      userID,
		TraceID:     executionID, // 默认使用ExecutionID作为TraceID
		Data:        make(map[string]any),
		Metadata:    make(map[string]any),
	}
}

// SetData 设置共享数据(兼容旧接口)
func (ec *ExecutionContext) SetData(key string, value any) {
	ec.SetShared(key, value)
}

// GetData 获取共享数据(兼容旧接口)
func (ec *ExecutionContext) GetData(key string) any {
	val, _ := ec.GetShared(key)
	return val
}

// SetShared 设置共享数据(显式共享)
// 线程安全的写入操作
func (ec *ExecutionContext) SetShared(key string, value any) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.Data[key] = value
}

// GetShared 获取共享数据(只读)
// 线程安全的读取操作
func (ec *ExecutionContext) GetShared(key string) (any, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	val, ok := ec.Data[key]
	return val, ok
}

// SetStepOutput 设置步骤输出(便捷方法)
// 自动使用stepID.output作为键名
func (ec *ExecutionContext) SetStepOutput(stepID string, output any) {
	key := stepID + ".output"
	ec.SetShared(key, output)
}

// GetStepOutput 获取步骤输出
// 自动使用stepID.output作为键名
func (ec *ExecutionContext) GetStepOutput(stepID string) (any, bool) {
	key := stepID + ".output"
	return ec.GetShared(key)
}

// GetAllData 获取所有共享数据的只读快照
// 返回数据的浅拷贝,避免外部直接修改
func (ec *ExecutionContext) GetAllData() map[string]any {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	// 浅拷贝
	snapshot := make(map[string]any, len(ec.Data))
	for k, v := range ec.Data {
		snapshot[k] = v
	}
	return snapshot
}

// SetMetadata 设置元数据
func (ec *ExecutionContext) SetMetadata(key string, value any) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.Metadata[key] = value
}

// GetMetadata 获取元数据
func (ec *ExecutionContext) GetMetadata(key string) (any, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	val, ok := ec.Metadata[key]
	return val, ok
}

// ToAgentContext 转换为AgentContext
// 用于将工作流上下文传递给Agent执行
func (ec *ExecutionContext) ToAgentContext(stepID string) *runtime.AgentContext {
	return &runtime.AgentContext{
		TenantID:   ec.TenantID,
		UserID:     ec.UserID,
		WorkflowID: &ec.WorkflowID,
		TraceID:    &ec.TraceID,
		StepID:     &stepID,
		Data:       ec.GetAllData(), // 只读快照
	}
}

// Clone 克隆执行上下文
// 创建一个新的上下文,数据为浅拷贝
func (ec *ExecutionContext) Clone() *ExecutionContext {
	return &ExecutionContext{
		WorkflowID:  ec.WorkflowID,
		ExecutionID: ec.ExecutionID,
		TenantID:    ec.TenantID,
		UserID:      ec.UserID,
		TraceID:     ec.TraceID,
		Data:        ec.GetAllData(),
		Metadata:    ec.getMetadataSnapshot(),
	}
}

// getMetadataSnapshot 获取元数据快照
func (ec *ExecutionContext) getMetadataSnapshot() map[string]any {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	snapshot := make(map[string]any, len(ec.Metadata))
	for k, v := range ec.Metadata {
		snapshot[k] = v
	}
	return snapshot
}
