package builtin

import (
	"context"
	"fmt"

	"backend/internal/rag"
	"backend/internal/tools"
)

// KnowledgeTool 知识库检索工具
type KnowledgeTool struct {
	ragService *rag.RAGService
}

// NewKnowledgeTool 创建知识库工具
func NewKnowledgeTool(ragService *rag.RAGService) *KnowledgeTool {
	return &KnowledgeTool{
		ragService: ragService,
	}
}

// Execute 执行检索
func (t *KnowledgeTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 提取参数
	kbID, ok := input["kb_id"].(string)
	if !ok || kbID == "" {
		return nil, fmt.Errorf("kb_id 参数类型错误或为空")
	}
	
	query, ok := input["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query 参数类型错误或为空")
	}
	
	tenantID, _ := input["tenant_id"].(string)
	
	topK := 3
	if k, ok := input["top_k"].(float64); ok {
		topK = int(k)
	}
	
	// 执行 RAG 检索
	results, err := t.ragService.Search(ctx, &rag.SearchRequest{
		KnowledgeBaseID: kbID,
		TenantID:        tenantID,
		Query:           query,
		TopK:            topK,
	})
	
	if err != nil {
		return nil, fmt.Errorf("检索失败: %w", err)
	}
	
	// 格式化结果
	docs := make([]map[string]any, 0, len(results.Results))
	for _, r := range results.Results {
		docs = append(docs, map[string]any{
			"content":     r.Content,
			"score":       r.Score,
			"document_id": r.DocumentID,
			"chunk_id":    r.ChunkID,
		})
	}
	
	return map[string]any{
		"query":     query,
		"documents": docs,
		"count":     len(docs),
	}, nil
}

// Validate 验证输入
func (t *KnowledgeTool) Validate(input map[string]any) error {
	if _, ok := input["kb_id"]; !ok {
		return fmt.Errorf("缺少必需参数: kb_id")
	}
	if _, ok := input["query"]; !ok {
		return fmt.Errorf("缺少必需参数: query")
	}
	return nil
}

// GetDefinition 获取工具定义
func (t *KnowledgeTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "knowledge_search",
		DisplayName: "知识库检索",
		Description: "从指定知识库中检索相关文档内容，支持语义搜索",
		Category:    "search",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kb_id": map[string]any{
					"type":        "string",
					"description": "知识库 ID",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "检索查询文本",
				},
				"tenant_id": map[string]any{
					"type":        "string",
					"description": "租户 ID（可选）",
				},
				"top_k": map[string]any{
					"type":        "integer",
					"description": "返回结果数量（默认3）",
					"default":     3,
					"minimum":     1,
					"maximum":     20,
				},
			},
			"required": []string{"kb_id", "query"},
		},
		Timeout:     10,
		Status:      "active",
		RequireAuth: true,
	}
}
