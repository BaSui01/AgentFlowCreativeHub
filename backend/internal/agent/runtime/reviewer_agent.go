package runtime

import (
	"context"
	"fmt"
	"time"

	"backend/internal/agent/prompt"
	"backend/internal/ai"
)

// ReviewerAgent 内容审校 Agent
type ReviewerAgent struct {
	config      *AgentConfig
	modelClient ai.ModelClient
	ragHelper   *RAGHelper
	promptEngine *prompt.Engine
	name        string
}

// NewReviewerAgent 创建 ReviewerAgent
func NewReviewerAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper, promptEngine *prompt.Engine) *ReviewerAgent {
	return &ReviewerAgent{
		config:      config,
		modelClient: modelClient,
		ragHelper:   ragHelper,
		promptEngine: promptEngine,
		name:        config.Name,
	}
}

// Execute 执行审校任务（非流式）
func (a *ReviewerAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
	start := time.Now()

	// RAG 增强：从知识库检索审校标准和案例
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

// ExecuteStream 执行审校任务（流式）
func (a *ReviewerAgent) ExecuteStream(ctx context.Context, input *AgentInput) (<-chan AgentChunk, <-chan error) {
	outChan := make(chan AgentChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		// RAG 增强：从知识库检索审校标准和案例
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
func (a *ReviewerAgent) Name() string {
	return a.name
}

// Type 返回 Agent 类型
func (a *ReviewerAgent) Type() string {
	return "reviewer"
}

// buildMessages 构建消息列表
func (a *ReviewerAgent) buildMessages(ctx context.Context, input *AgentInput) ([]ai.Message, error) {
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
		systemPrompt = "你是一个专业的内容审校者，擅长发现文本中的错误和改进空间。" +
			"请仔细审查提供的内容，指出以下问题：\n" +
			"1. 语法错误和拼写错误\n" +
			"2. 逻辑不清或表述不当\n" +
			"3. 结构问题\n" +
			"4. 可读性和流畅度问题\n" +
			"5. 改进建议\n\n" +
			"请提供详细的审校意见和具体的修改建议。"
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
	userContent := fmt.Sprintf("请审校以下内容：\n\n%s", input.Content)
	
	// 如果有额外参数，添加审校要求
	if criteria, ok := input.ExtraParams["criteria"].(string); ok {
		userContent += fmt.Sprintf("\n\n审校标准：%s", criteria)
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: userContent,
	})

	return messages, nil
}
