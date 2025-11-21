package builtin

import (
	"context"
	"fmt"

	"backend/internal/agent/runtime"
	"backend/internal/tools"
)

// SaveMemoryTool 记忆保存工具
type SaveMemoryTool struct {
	memoryService runtime.MemoryService
}

// NewSaveMemoryTool 创建记忆保存工具
func NewSaveMemoryTool(memoryService runtime.MemoryService) *SaveMemoryTool {
	return &SaveMemoryTool{
		memoryService: memoryService,
	}
}

// Execute 执行记忆保存
func (t *SaveMemoryTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 提取参数
	content, ok := input["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("content 参数类型错误或为空")
	}

	// 获取租户 ID（用于构建知识库 ID）
	tenantID, _ := input["tenant_id"].(string)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id 参数不能为空")
	}

	// 构建知识库 ID（使用租户 ID + "_memory"）
	kbID := tenantID + "_memory"

	// 提取可选参数
	importance, _ := input["importance"].(string)
	if importance == "" {
		importance = "medium"
	}

	tags, _ := input["tags"].([]any)
	if tags == nil {
		tags = []any{}
	}

	// 构建元数据
	metadata := map[string]any{
		"importance": importance,
		"tags":       tags,
	}

	// 如果提供了其他元数据，合并进去
	if extraMeta, ok := input["metadata"].(map[string]any); ok {
		for k, v := range extraMeta {
			metadata[k] = v
		}
	}

	// 调用 MemoryService 保存记忆
	err := t.memoryService.Add(ctx, kbID, content, metadata)
	if err != nil {
		return nil, fmt.Errorf("保存记忆失败: %w", err)
	}

	return map[string]any{
		"success":    true,
		"kb_id":      kbID,
		"content":    content,
		"importance": importance,
		"tags":       tags,
	}, nil
}

// Validate 验证输入
func (t *SaveMemoryTool) Validate(input map[string]any) error {
	if _, ok := input["content"]; !ok {
		return fmt.Errorf("缺少必需参数: content")
	}
	if _, ok := input["tenant_id"]; !ok {
		return fmt.Errorf("缺少必需参数: tenant_id")
	}
	return nil
}

// GetDefinition 获取工具定义
func (t *SaveMemoryTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "save_memory",
		DisplayName: "保存记忆",
		Description: "将重要信息保存到长期记忆中，以便后续检索和使用。支持设置重要程度和标签。",
		Category:    "memory",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "需要保存的记忆内容",
				},
				"tenant_id": map[string]any{
					"type":        "string",
					"description": "租户 ID（用于隔离记忆）",
				},
				"importance": map[string]any{
					"type":        "string",
					"description": "重要程度（high, medium, low）",
					"enum":        []string{"high", "medium", "low"},
					"default":     "medium",
				},
				"tags": map[string]any{
					"type":        "array",
					"description": "标签列表（用于分类和检索）",
					"items": map[string]any{
						"type": "string",
					},
					"default": []string{},
				},
				"metadata": map[string]any{
					"type":        "object",
					"description": "额外的元数据（键值对）",
				},
			},
			"required": []string{"content", "tenant_id"},
		},
		Timeout:     10,
		Status:      "active",
		RequireAuth: true,
	}
}

