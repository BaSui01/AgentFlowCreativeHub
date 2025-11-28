package runtime

import (
	"context"
	"fmt"
	"time"

	"backend/internal/agent/prompt"
	"backend/internal/ai"
)

// FormatterAgent 格式化 Agent
type FormatterAgent struct {
	config       *AgentConfig
	modelClient  ai.ModelClient
	ragHelper    *RAGHelper
	toolHelper   *ToolHelper
	promptEngine *prompt.Engine
	name         string
}

// NewFormatterAgent 创建 FormatterAgent
func NewFormatterAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper, promptEngine *prompt.Engine, toolHelper *ToolHelper) *FormatterAgent {
	return &FormatterAgent{
		config:       config,
		modelClient:  modelClient,
		ragHelper:    ragHelper,
		toolHelper:   toolHelper,
		promptEngine: promptEngine,
		name:         config.Name,
	}
}

// Execute 执行格式化任务（非流式）
func (a *FormatterAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
	start := time.Now()

	// RAG 增强：从知识库检索格式化规范和模板
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

// ExecuteStream 执行格式化任务（流式）
func (a *FormatterAgent) ExecuteStream(ctx context.Context, input *AgentInput) (<-chan AgentChunk, <-chan error) {
	outChan := make(chan AgentChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		// RAG 增强：从知识库检索格式化规范和模板
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
func (a *FormatterAgent) Name() string {
	return a.name
}

// Type 返回 Agent 类型
func (a *FormatterAgent) Type() string {
	return "formatter"
}

// buildMessages 构建消息列表
func (a *FormatterAgent) buildMessages(ctx context.Context, input *AgentInput) ([]ai.Message, error) {
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
		systemPrompt = "你是一个专业的内容格式化工具，擅长将内容整理为统一的格式和风格。" +
			"你的任务是：\n" +
			"1. 统一格式（标题层级、段落间距、列表格式等）\n" +
			"2. 优化排版（提升可读性）\n" +
			"3. 保持内容的完整性和准确性\n" +
			"4. 确保专业和一致的风格\n\n" +
			"请直接输出格式化后的内容，不要添加额外的解释。"
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
	userContent := fmt.Sprintf("请格式化以下内容：\n\n%s", input.Content)
	
	// 如果指定了格式要求
	if format, ok := input.ExtraParams["format"].(string); ok {
		userContent += fmt.Sprintf("\n\n格式要求：%s", format)
	}

	// 如果指定了风格要求
	if style, ok := input.ExtraParams["style"].(string); ok {
		userContent += fmt.Sprintf("\n\n风格要求：%s", style)
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: userContent,
	})

	return messages, nil
}
