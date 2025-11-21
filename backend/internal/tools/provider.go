package tools

import "context"

// DefinitionProvider 抽象工具定义提供方,便于上层通过接口依赖而非具体实现
type DefinitionProvider interface {
	GetDefinition(name string) (*ToolDefinition, bool)
	List() []*ToolDefinition
	ListByCategory(category string) []*ToolDefinition
}

// ExecutionProvider 抽象工具执行方,用于统一调用入口
type ExecutionProvider interface {
	Execute(ctx context.Context, req *ToolExecutionRequest) (*ToolExecutionResult, error)
}
