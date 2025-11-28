package runtime

import (
	"context"
	"fmt"
	"time"

	"backend/internal/agent/prompt"
	"backend/internal/ai"
)

// PlotAgent 剧情推演 Agent
type PlotAgent struct {
	config       *AgentConfig
	modelClient  ai.ModelClient
	ragHelper    *RAGHelper
	toolHelper   *ToolHelper
	promptEngine *prompt.Engine
	name         string
}

// NewPlotAgent 创建 PlotAgent
func NewPlotAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper, promptEngine *prompt.Engine, toolHelper *ToolHelper) *PlotAgent {
	return &PlotAgent{
		config:       config,
		modelClient:  modelClient,
		ragHelper:    ragHelper,
		toolHelper:   toolHelper,
		promptEngine: promptEngine,
		name:         config.Name,
	}
}

// Execute 执行剧情推演任务（非流式）
func (a *PlotAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
	start := time.Now()

	// RAG 增强：从知识库检索相关剧情案例和写作技巧
	if a.ragHelper != nil {
		enrichedInput, err := a.ragHelper.EnrichWithKnowledge(ctx, a.config.AgentConfig, input, a.modelClient)
		if err == nil {
			input = enrichedInput
		}
	}

	// 构建消息列表
	messages, err := a.buildMessages(ctx, input)
	if err != nil {
		return &AgentResult{
			Output:    "",
			Status:    "failed",
			Error:     err.Error(),
			LatencyMs: 0,
		}, err
	}

	var output string
	var usage *Usage
	var modelID string

	// 检查是否允许使用工具
	if a.toolHelper != nil && len(a.config.AllowedTools) > 0 {
		resp, err := a.toolHelper.ExecuteWithTools(
			ctx,
			a.modelClient,
			messages,
			a.config.Temperature,
			a.config.MaxTokens,
			input.Context.TenantID,
			input.Context.UserID,
			a.config.AllowedTools,
		)

		if err != nil {
			return &AgentResult{
				Output:    "",
				Status:    "failed",
				Error:     err.Error(),
				LatencyMs: time.Since(start).Milliseconds(),
			}, err
		}

		output = resp.Content
		usage = &Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		modelID = resp.Model
	} else {
		// 调用 AI 模型
		resp, err := a.modelClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
			Messages:    messages,
			Temperature: a.config.Temperature,
			MaxTokens:   a.config.MaxTokens,
		})

		if err != nil {
			return &AgentResult{
				Output:    "",
				Status:    "failed",
				Error:     err.Error(),
				LatencyMs: time.Since(start).Milliseconds(),
			}, err
		}

		output = resp.Content
		usage = &Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		modelID = resp.Model
	}

	latency := time.Since(start).Milliseconds()

	// 构建结果
	result := &AgentResult{
		Output:    output,
		Usage:     usage,
		Cost:      calculateCost(usage.PromptTokens, usage.CompletionTokens),
		LatencyMs: latency,
		Status:    "success",
		Metadata: map[string]any{
			"model_id": modelID,
		},
	}

	return result, nil
}

// ExecuteStream 执行剧情推演任务（流式）
func (a *PlotAgent) ExecuteStream(ctx context.Context, input *AgentInput) (<-chan AgentChunk, <-chan error) {
	outChan := make(chan AgentChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		// RAG 增强
		if a.ragHelper != nil {
			enrichedInput, err := a.ragHelper.EnrichWithKnowledge(ctx, a.config.AgentConfig, input, a.modelClient)
			if err == nil {
				input = enrichedInput
			}
		}

		// 构建消息列表
		messages, err := a.buildMessages(ctx, input)
		if err != nil {
			errChan <- err
			return
		}

		// 调用 AI 模型流式接口
		chunkChan, modelErrChan := a.modelClient.ChatCompletionStream(ctx, &ai.ChatCompletionRequest{
			Messages:    messages,
			Temperature: a.config.Temperature,
			MaxTokens:   a.config.MaxTokens,
		})

		// 转发响应块
		for chunk := range chunkChan {
			outChan <- AgentChunk{
				Content: chunk.Content,
				Done:    chunk.Done,
			}
		}

		// 检查错误
		select {
		case err := <-modelErrChan:
			if err != nil {
				errChan <- err
			}
		default:
		}
	}()

	return outChan, errChan
}

// Name 返回 Agent 名称
func (a *PlotAgent) Name() string {
	return a.name
}

// Type 返回 Agent 类型
func (a *PlotAgent) Type() string {
	return "plot"
}

// buildMessages 构建消息列表
func (a *PlotAgent) buildMessages(ctx context.Context, input *AgentInput) ([]ai.Message, error) {
	messages := make([]ai.Message, 0)

	// 系统提示词逻辑
	var systemPrompt string

	// 1. 尝试从 Prompt Engine 加载
	if a.config.PromptTemplateID != nil && *a.config.PromptTemplateID != "" && a.promptEngine != nil {
		vars := make(map[string]any)
		if input.Variables != nil {
			for k, v := range input.Variables {
				vars[k] = v
			}
		}
		vars["Content"] = input.Content
		rendered, err := a.promptEngine.Render(ctx, *a.config.PromptTemplateID, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to render system prompt template: %w", err)
		}
		systemPrompt = rendered
	}

	// 2. 如果没有模板或模板为空，使用配置中的 SystemPrompt
	if systemPrompt == "" {
		systemPrompt = a.config.SystemPrompt
	}

	// 3. 如果仍然为空，使用默认硬编码 Prompt
	if systemPrompt == "" {
		systemPrompt = `你是一位专业的剧情推演师，擅长根据当前故事情节生成多个可能的后续发展方向。

推演要求：
1. 基于已有剧情和角色性格合理延伸
2. 每个分支应有独特的发展方向和冲突点
3. 考虑世界观设定的约束
4. 标注每个分支的情感基调（爽、虐、治愈等）
5. 提供分支的悬念设计和读者期待点

输出格式（JSON）：
{
  "branches": [
    {
      "id": 1,
      "title": "分支标题",
      "summary": "简要概述",
      "key_events": ["事件1", "事件2"],
      "emotional_tone": "情感基调",
      "hook": "悬念/爽点",
      "difficulty": "难度（1-5）"
    }
  ],
  "recommendation": "推荐分支及理由"
}`
	}

	// RAG 增强：将知识库上下文注入到系统提示词
	systemPrompt = InjectKnowledgeIntoPrompt(input, systemPrompt)

	messages = append(messages, ai.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// 添加历史对话
	for _, msg := range input.History {
		messages = append(messages, ai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 添加当前输入
	userContent := input.Content

	// 解析额外参数
	if currentPlot, ok := input.ExtraParams["current_plot"].(string); ok && currentPlot != "" {
		userContent = fmt.Sprintf("当前剧情：\n%s\n\n%s", currentPlot, userContent)
	}

	if characters, ok := input.ExtraParams["characters"].(string); ok && characters != "" {
		userContent += fmt.Sprintf("\n\n相关角色：%s", characters)
	}

	if worldSetting, ok := input.ExtraParams["world_setting"].(string); ok && worldSetting != "" {
		userContent += fmt.Sprintf("\n\n世界观设定：%s", worldSetting)
	}

	if numBranches, ok := input.ExtraParams["num_branches"]; ok {
		userContent += fmt.Sprintf("\n\n请生成 %v 个剧情分支", numBranches)
	} else {
		userContent += "\n\n请生成 3 个剧情分支"
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: userContent,
	})

	return messages, nil
}
