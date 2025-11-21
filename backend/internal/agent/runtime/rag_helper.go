package runtime

import (
	"context"
	"fmt"
	"strings"

	"backend/internal/agent"
	"backend/internal/ai"
	"backend/internal/rag"
)

// RAGHelper RAG 辅助工具
type RAGHelper struct {
	retriever rag.Retriever
}

// NewRAGHelper 创建 RAG 辅助工具
func NewRAGHelper(retriever rag.Retriever) *RAGHelper {
	return &RAGHelper{
		retriever: retriever,
	}
}

// RAGMode 描述 RAG 注入模式
type RAGMode string

const (
	RAGModeNone      RAGMode = "none"
	RAGModeStuff     RAGMode = "stuff"
	RAGModeMapReduce RAGMode = "map_reduce"
)

// RAGOptions RAG 配置选项
// 从 AgentConfig.ExtraConfig 与 AgentInput.ExtraParams 解析,统一控制 RAG 行为
type RAGOptions struct {
	Mode         RAGMode
	TopK         int
	MinScore     float64
	MapMaxChunks int
}

// resolveRAGOptions 解析 RAG 配置
// 优先级: AgentConfig.ExtraConfig < AgentInput.ExtraParams
func resolveRAGOptions(config *agent.AgentConfig, input *AgentInput) RAGOptions {
	// 基础默认值
	opts := RAGOptions{
		Mode:         RAGModeStuff,
		TopK:         3,
		MinScore:     0.7,
		MapMaxChunks: 5,
	}

	if config != nil {
		if config.RAGTopK > 0 {
			opts.TopK = config.RAGTopK
		}
		if config.RAGMinScore > 0 {
			opts.MinScore = config.RAGMinScore
		}
		if cfg := config.ExtraConfig; cfg != nil {
			if v, ok := cfg["rag_mode"].(string); ok && v != "" {
				opts.Mode = RAGMode(v)
			}
			if v, ok := cfg["rag_top_k"]; ok {
				if n, ok2 := toInt(v); ok2 && n > 0 {
					opts.TopK = n
				}
			}
			if v, ok := cfg["rag_min_score"]; ok {
				if f, ok2 := toFloat64(v); ok2 && f > 0 {
					opts.MinScore = f
				}
			}
			if v, ok := cfg["rag_map_max_chunks"]; ok {
				if n, ok2 := toInt(v); ok2 && n > 0 {
					opts.MapMaxChunks = n
				}
			}
		}
	}

	// 覆盖: 请求级 ExtraParams
	if input != nil && input.ExtraParams != nil {
		params := input.ExtraParams
		if v, ok := params["rag_mode"].(string); ok && v != "" {
			opts.Mode = RAGMode(v)
		}
		if v, ok := params["rag_top_k"]; ok {
			if n, ok2 := toInt(v); ok2 && n > 0 {
				opts.TopK = n
			}
		}
		if v, ok := params["rag_min_score"]; ok {
			if f, ok2 := toFloat64(v); ok2 && f > 0 {
				opts.MinScore = f
			}
		}
		if v, ok := params["rag_map_max_chunks"]; ok {
			if n, ok2 := toInt(v); ok2 && n > 0 {
				opts.MapMaxChunks = n
			}
		}
	}

	if opts.TopK <= 0 {
		opts.TopK = 3
	}
	if opts.MinScore <= 0 {
		opts.MinScore = 0.7
	}
	if opts.MapMaxChunks <= 0 {
		opts.MapMaxChunks = 5
	}

	return opts
}

// EnrichWithKnowledge 使用知识库丰富输入内容
// 根据 Agent 配置和输入参数选择不同的 RAG 模式（stuff / map_reduce）
// modelClient 用于在 map-reduce 模式下调用模型做摘要
func (h *RAGHelper) EnrichWithKnowledge(ctx context.Context, config *agent.AgentConfig, input *AgentInput, modelClient ai.ModelClient) (*AgentInput, error) {
	if config == nil || input == nil {
		return input, nil
	}

	// 检查是否启用 RAG
	if !config.RAGEnabled || config.KnowledgeBaseID == "" {
		return input, nil
	}

	// 获取查询文本（使用用户输入作为查询）
	query := input.Content
	if query == "" {
		return input, nil
	}

	// 解析 RAG 配置
	opts := resolveRAGOptions(config, input)
	if opts.Mode == RAGModeNone {
		return input, nil
	}

	// 从知识库检索相关上下文
	searchResp, err := h.retriever.Search(ctx, &rag.SearchRequest{
		KnowledgeBaseID: config.KnowledgeBaseID,
		TenantID:        config.TenantID,
		Query:           query,
		TopK:            opts.TopK,
	})
	if err != nil {
		// 检索失败时记录错误但不中断执行
		fmt.Printf("RAG 检索失败: %v\n", err)
		return input, nil
	}

	results := searchResp.Results
	if len(results) == 0 {
		return input, nil
	}

	var validResults []*rag.SearchResult
	for _, result := range results {
		// Score 和 Similarity 兼容处理
		score := result.Score
		if score == 0 && result.Similarity > 0 {
			score = result.Similarity
		}
		if score >= opts.MinScore {
			validResults = append(validResults, result)
		}
	}

	if len(validResults) == 0 {
		return input, nil
	}

	// 根据模式构建知识上下文
	var contextText string
	switch opts.Mode {
	case RAGModeMapReduce:
		if modelClient != nil {
			contextText, err = h.buildMapReduceContext(ctx, modelClient, validResults, opts.MapMaxChunks)
			if err != nil {
				fmt.Printf("RAG map-reduce 失败, 回退到 stuff 模式: %v\n", err)
				contextText = h.buildContextText(validResults)
			}
		} else {
			// 没有模型客户端时退回简单模式
			contextText = h.buildContextText(validResults)
		}
	default: // 包含 RAGModeStuff 及未知值
		contextText = h.buildContextText(validResults)
	}

	if contextText == "" {
		return input, nil
	}

	// 在原始输入的上下文中添加知识库信息
	if input.Context == nil {
		input.Context = &AgentContext{}
	}
	if input.Context.Data == nil {
		input.Context.Data = make(map[string]any)
	}
	input.Context.Data["knowledge_context"] = contextText
	input.Context.Data["knowledge_source_count"] = len(validResults)

	return input, nil
}

// buildContextText 构建简单拼接的上下文文本（stuff 模式）
func (h *RAGHelper) buildContextText(results []*rag.SearchResult) string {
	if len(results) == 0 {
		return ""
	}

	contextParts := []string{
		"以下是从知识库检索到的相关信息，请参考这些信息回答问题：\n",
	}

	for i, result := range results {
		contextParts = append(contextParts, fmt.Sprintf(
			"[参考资料 %d] (相似度: %.2f)\n%s\n",
			i+1,
			result.Score,
			result.Content,
		))
	}

	// 连接所有部分
	fullContext := ""
	for _, part := range contextParts {
		fullContext += part + "\n"
	}

	return fullContext
}

// buildMapReduceContext 使用简单的 Map-Reduce 策略对检索结果进行多级摘要
// 1) 对每个片段做局部总结
// 2) 将局部总结再次汇总为整体上下文
func (h *RAGHelper) buildMapReduceContext(ctx context.Context, client ai.ModelClient, results []*rag.SearchResult, maxChunks int) (string, error) {
	if len(results) == 0 {
		return "", nil
	}

	// 控制参与 map 的片段数量,避免成本过高
	if maxChunks <= 0 {
		maxChunks = 5
	}
	if len(results) < maxChunks {
		maxChunks = len(results)
	}

	summaries := make([]string, 0, maxChunks)
	for i := 0; i < maxChunks; i++ {
		chunk := results[i]
		req := &ai.ChatCompletionRequest{
			Messages: []ai.Message{
				{
					Role:    "system",
					Content: "你是一个检索总结助手,请阅读以下知识片段并用 3-5 行中文提炼核心要点,聚焦关键事实和结论。",
				},
				{
					Role:    "user",
					Content: chunk.Content,
				},
			},
			Temperature: 0.1,
			MaxTokens:   256,
		}

		resp, err := client.ChatCompletion(ctx, req)
		if err != nil {
			// 单个片段失败不终止整体,记录日志后继续
			fmt.Printf("RAG map 阶段总结失败: %v\n", err)
			continue
		}

		summaries = append(summaries, fmt.Sprintf("[片段 %d 总结]\n%s", i+1, resp.Content))
	}

	if len(summaries) == 0 {
		return "", fmt.Errorf("所有 map 阶段总结均失败")
	}

	combined := strings.Join(summaries, "\n\n")
	req := &ai.ChatCompletionRequest{
		Messages: []ai.Message{
			{
				Role:    "system",
				Content: "你是一个知识聚合助手,请基于以下多个资料总结,整合为一段结构化的知识背景,供后续回答问题时作为参考。不要重复原文,而是提炼关键事实、结论和重要约束。",
			},
			{
				Role:    "user",
				Content: combined,
			},
		},
		Temperature: 0.2,
		MaxTokens:   512,
	}

	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("reduce 阶段总结失败: %w", err)
	}

	return resp.Content, nil
}

// InjectKnowledgeIntoPrompt 将知识库上下文注入到 Prompt 中
func InjectKnowledgeIntoPrompt(input *AgentInput, systemPrompt string) string {
	// 检查是否有知识库上下文
	if input.Context == nil || input.Context.Data == nil {
		return systemPrompt
	}

	knowledgeContext, exists := input.Context.Data["knowledge_context"]
	if !exists {
		return systemPrompt
	}

	knowledgeText, ok := knowledgeContext.(string)
	if !ok || knowledgeText == "" {
		return systemPrompt
	}

	// 将知识库上下文添加到系统提示词中
	enhancedPrompt := fmt.Sprintf(
		"%s\n\n%s\n\n请基于上述参考资料和你的知识来回答用户的问题。如果参考资料中没有相关信息，可以基于你的知识回答。",
		systemPrompt,
		knowledgeText,
	)

	return enhancedPrompt
}

// 辅助转换函数: 尽量将任意类型转换为 int/float64
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		var iv int
		_, err := fmt.Sscanf(n, "%d", &iv)
		if err != nil {
			return 0, false
		}
		return iv, true
	default:
		return 0, false
	}
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		var fv float64
		_, err := fmt.Sscanf(n, "%f", &fv)
		if err != nil {
			return 0, false
		}
		return fv, true
	default:
		return 0, false
	}
}

