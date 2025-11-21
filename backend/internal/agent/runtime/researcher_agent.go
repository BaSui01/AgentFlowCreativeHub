package runtime

import (
	"context"
	"fmt"
	"time"

	"backend/internal/agent/prompt"
	"backend/internal/ai"
)

// ResearcherAgent 信息检索 Agent
type ResearcherAgent struct {
	config      *AgentConfig
	modelClient ai.ModelClient
	ragHelper   *RAGHelper
	promptEngine *prompt.Engine
	name        string
}

// NewResearcherAgent 创建 ResearcherAgent
func NewResearcherAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper, promptEngine *prompt.Engine) *ResearcherAgent {
	return &ResearcherAgent{
		config:      config,
		modelClient: modelClient,
		ragHelper:   ragHelper,
		promptEngine: promptEngine,
		name:        config.Name,
	}
}

// Execute 执行检索任务（非流式）
func (a *ResearcherAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
	start := time.Now()

	// RAG 增强：从知识库检索相关信息（Researcher Agent 的核心能力）
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

	// 调用 AI 模型
	resp, err := a.modelClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
		Messages:    messages,
		Temperature: a.config.Temperature,
		MaxTokens:   a.config.MaxTokens,
	})

	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &AgentResult{
			Output:    "",
			Status:    "failed",
			Error:     err.Error(),
			LatencyMs: latency,
		}, err
	}

	// 构建结果
	result := &AgentResult{
		Output: resp.Content,
		Usage: &Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Cost:      calculateCost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens),
		LatencyMs: latency,
		Status:    "success",
		Metadata: map[string]any{
			"model_id": resp.Model,
		},
	}

	return result, nil
}

// ExecuteStream 执行检索任务（流式）
func (a *ResearcherAgent) ExecuteStream(ctx context.Context, input *AgentInput) (<-chan AgentChunk, <-chan error) {
	outChan := make(chan AgentChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		// RAG 增强：从知识库检索相关信息
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
func (a *ResearcherAgent) Name() string {
	return a.name
}

// Type 返回 Agent 类型
func (a *ResearcherAgent) Type() string {
	return "researcher"
}

// buildMessages 构建消息列表
func (a *ResearcherAgent) buildMessages(ctx context.Context, input *AgentInput) ([]ai.Message, error) {
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
		systemPrompt = "你是一个专业的研究助手，擅长信息检索和知识汇总。" +
			"你的任务是：\n" +
			"1. 从知识库中检索相关信息\n" +
			"2. 综合多个来源提供全面答案\n" +
			"3. 标注信息来源和可靠性\n" +
			"4. 识别信息冲突并说明\n" +
			"5. 提供延伸阅读建议\n\n" +
			"请基于检索到的信息，提供准确、全面的回答。如果检索到的信息不足，请明确说明。"
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
	userContent := fmt.Sprintf("请检索并回答以下问题：\n\n%s", input.Content)
	
	// 如果有额外参数，添加检索要求
	if sources, ok := input.ExtraParams["sources"].(string); ok {
		userContent += fmt.Sprintf("\n\n信息来源要求：%s", sources)
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: userContent,
	})

	return messages, nil
}
