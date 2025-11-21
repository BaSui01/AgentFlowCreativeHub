package runtime

import (
	"context"
	"fmt"
	"time"

	"backend/internal/agent/prompt"
	"backend/internal/ai"
)

// TranslatorAgent 多语言翻译 Agent
type TranslatorAgent struct {
	config      *AgentConfig
	modelClient ai.ModelClient
	ragHelper   *RAGHelper
	promptEngine *prompt.Engine
	name        string
}

// NewTranslatorAgent 创建 TranslatorAgent
func NewTranslatorAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper, promptEngine *prompt.Engine) *TranslatorAgent {
	return &TranslatorAgent{
		config:      config,
		modelClient: modelClient,
		ragHelper:   ragHelper,
		promptEngine: promptEngine,
		name:        config.Name,
	}
}

// Execute 执行翻译任务（非流式）
func (a *TranslatorAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
	start := time.Now()

	// RAG 增强：从知识库检索术语词典和翻译记忆
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

// ExecuteStream 执行翻译任务（流式）
func (a *TranslatorAgent) ExecuteStream(ctx context.Context, input *AgentInput) (<-chan AgentChunk, <-chan error) {
	outChan := make(chan AgentChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		// RAG 增强：从知识库检索术语词典和翻译记忆
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
func (a *TranslatorAgent) Name() string {
	return a.name
}

// Type 返回 Agent 类型
func (a *TranslatorAgent) Type() string {
	return "translator"
}

// buildMessages 构建消息列表
func (a *TranslatorAgent) buildMessages(ctx context.Context, input *AgentInput) ([]ai.Message, error) {
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
		systemPrompt = "你是一个专业的翻译专家，精通多语言翻译和本地化。" +
			"你的任务是：\n" +
			"1. 准确翻译源语言内容\n" +
			"2. 保持术语一致性\n" +
			"3. 适应目标语言的表达习惯\n" +
			"4. 保留原文的语气和风格\n" +
			"5. 注意文化差异和敏感词\n\n" +
			"请基于术语词典和翻译记忆，提供高质量的翻译。"
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
	userContent := fmt.Sprintf("请翻译以下内容：\n\n%s", input.Content)

	// 如果指定了源语言和目标语言
	if sourceLang, ok := input.ExtraParams["source_lang"].(string); ok {
		userContent += fmt.Sprintf("\n\n源语言：%s", sourceLang)
	}
	if targetLang, ok := input.ExtraParams["target_lang"].(string); ok {
		userContent += fmt.Sprintf("\n目标语言：%s", targetLang)
	}

	// 如果指定了领域
	if domain, ok := input.ExtraParams["domain"].(string); ok {
		userContent += fmt.Sprintf("\n领域：%s", domain)
	}

	// 如果指定了正式程度
	if formality, ok := input.ExtraParams["formality"].(string); ok {
		userContent += fmt.Sprintf("\n正式程度：%s", formality)
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: userContent,
	})

	return messages, nil
}
