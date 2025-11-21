package tools

import (
	"context"
	"fmt"
	"sync"

	"backend/pkg/aiinterface"
)

// ToolRegistry 工具注册表
type ToolRegistry struct {
	mu      sync.RWMutex
	tools   map[string]ToolHandler     // name -> handler
	schemas map[string]*ToolDefinition // name -> definition
}

// ToolHandler 工具执行器接口
type ToolHandler interface {
	// Execute 执行工具
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)
	
	// Validate 验证输入参数
	Validate(input map[string]any) error
}

// NewToolRegistry 创建工具注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:   make(map[string]ToolHandler),
		schemas: make(map[string]*ToolDefinition),
	}
}

// Register 注册工具
func (r *ToolRegistry) Register(name string, handler ToolHandler, definition *ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("工具 %s 已注册", name)
	}
	
	r.tools[name] = handler
	r.schemas[name] = definition
	return nil
}

// Unregister 取消注册工具
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.tools, name)
	delete(r.schemas, name)
}

// Get 获取工具处理器
func (r *ToolRegistry) Get(name string) (ToolHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, exists := r.tools[name]
	return handler, exists
}

// GetDefinition 获取工具定义
func (r *ToolRegistry) GetDefinition(name string) (*ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, exists := r.schemas[name]
	return def, exists
}

// List 列出所有工具
func (r *ToolRegistry) List() []*ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tools := make([]*ToolDefinition, 0, len(r.schemas))
	for _, def := range r.schemas {
		tools = append(tools, def)
	}
	return tools
}

// ListByCategory 按类别列出工具
func (r *ToolRegistry) ListByCategory(category string) []*ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tools := make([]*ToolDefinition, 0)
	for _, def := range r.schemas {
		if def.Category == category {
			tools = append(tools, def)
		}
	}
	return tools
}

// ToOpenAITools 转换为 OpenAI Tools 格式
func (r *ToolRegistry) ToOpenAITools() []aiinterface.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tools := make([]aiinterface.Tool, 0, len(r.schemas))
	for _, def := range r.schemas {
		if def.Status != "active" {
			continue
		}
		
		tools = append(tools, aiinterface.Tool{
			Type: "function",
			Function: aiinterface.FunctionDef{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			},
		})
	}
	return tools
}

// Count 统计工具数量
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
