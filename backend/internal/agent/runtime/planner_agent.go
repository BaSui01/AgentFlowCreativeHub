package runtime

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"backend/internal/agent/prompt"
	"backend/internal/ai"
)

// PlannerAgent 任务规划 Agent
type PlannerAgent struct {
	config        *AgentConfig
	modelClient   ai.ModelClient
	ragHelper     *RAGHelper
	toolHelper    *ToolHelper
	memoryService MemoryService
	promptEngine  *prompt.Engine
	name          string
	tracer        trace.Tracer
}

// NewPlannerAgent 创建 PlannerAgent
func NewPlannerAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper, promptEngine *prompt.Engine, memoryService MemoryService, toolHelper *ToolHelper) *PlannerAgent {
	return &PlannerAgent{
		config:        config,
		modelClient:   modelClient,
		ragHelper:     ragHelper,
		toolHelper:    toolHelper,
		memoryService: memoryService,
		promptEngine:  promptEngine,
		name:          config.Name,
		tracer:        otel.Tracer("backend/internal/agent/runtime/planner"),
	}
}

// Execute 执行规划任务（非流式）
func (a *PlannerAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
	ctx, span := a.tracer.Start(ctx, "PlannerAgent.Execute")
	defer span.End()

	start := time.Now()

	span.SetAttributes(
		attribute.String("agent_name", a.name),
		attribute.String("model_id", a.config.ModelID),
	)

	// RAG 增强：从知识库检索项目案例和规划模板
	if a.ragHelper != nil {
		_, ragSpan := a.tracer.Start(ctx, "RAG.Enrich")
		enrichedInput, err := a.ragHelper.EnrichWithKnowledge(ctx, a.config.AgentConfig, input, a.modelClient)
		if err == nil {
			input = enrichedInput
		} else {
			ragSpan.RecordError(err)
		}
		ragSpan.End()
	}

	// Memory 增强：从长期记忆检索相关信息
	memoryContext := ""
	if a.memoryService != nil && input.Context != nil && input.Context.TenantID != "" {
		_, memSpan := a.tracer.Start(ctx, "Memory.Search")
		kbID := input.Context.TenantID + "_memory"
		results, err := a.memoryService.Search(ctx, kbID, input.Content, 5)
		if err == nil && len(results) > 0 {
			memoryContext = "相关历史记忆：\n"
			for i, r := range results {
				memoryContext += fmt.Sprintf("%d. %s (相似度: %.2f)\n", i+1, r.Content, r.Score)
			}
			memSpan.SetAttributes(attribute.Int("memory_hits", len(results)))
		} else if err != nil {
			memSpan.RecordError(err)
		}
		memSpan.End()
	}

	// 构建消息列表
	messages, err := a.buildMessages(ctx, input, memoryContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Build messages failed")
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
		_, toolSpan := a.tracer.Start(ctx, "ToolHelper.ExecuteWithTools")
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
		toolSpan.End()

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Tool execution failed")
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
		_, aiSpan := a.tracer.Start(ctx, "AI.ChatCompletion")
		resp, err := a.modelClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
			Messages:    messages,
			Temperature: a.config.Temperature,
			MaxTokens:   a.config.MaxTokens,
		})
		aiSpan.End()

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "AI call failed")
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

	span.SetAttributes(
		attribute.Int("prompt_tokens", usage.PromptTokens),
		attribute.Int("completion_tokens", usage.CompletionTokens),
		attribute.Int("total_tokens", usage.TotalTokens),
		attribute.Int64("latency_ms", latency),
	)

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

// ExecuteStream 执行规划任务（流式）
func (a *PlannerAgent) ExecuteStream(ctx context.Context, input *AgentInput) (<-chan AgentChunk, <-chan error) {
	outChan := make(chan AgentChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		ctx, span := a.tracer.Start(ctx, "PlannerAgent.ExecuteStream")
		defer span.End()

		// RAG 增强：从知识库检索项目案例和规划模板
		if a.ragHelper != nil {
			enrichedInput, err := a.ragHelper.EnrichWithKnowledge(ctx, a.config.AgentConfig, input, a.modelClient)
			if err == nil {
				input = enrichedInput
			}
		}

		// Memory 增强：从长期记忆检索相关信息
		memoryContext := ""
		if a.memoryService != nil && input.Context != nil && input.Context.TenantID != "" {
			kbID := input.Context.TenantID + "_memory"
			results, err := a.memoryService.Search(ctx, kbID, input.Content, 5)
			if err == nil && len(results) > 0 {
				memoryContext = "相关历史记忆：\n"
				for i, r := range results {
					memoryContext += fmt.Sprintf("%d. %s (相似度: %.2f)\n", i+1, r.Content, r.Score)
				}
			}
		}

		// 构建消息列表
		messages, err := a.buildMessages(ctx, input, memoryContext)
		if err != nil {
			span.RecordError(err)
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
				span.RecordError(err)
				errChan <- err
			}
		default:
		}
	}()

	return outChan, errChan
}

// Name 返回 Agent 名称
func (a *PlannerAgent) Name() string {
	return a.name
}

// Type 返回 Agent 类型
func (a *PlannerAgent) Type() string {
	return "planner"
}

// buildMessages 构建消息列表
func (a *PlannerAgent) buildMessages(ctx context.Context, input *AgentInput, memoryContext string) ([]ai.Message, error) {
	messages := make([]ai.Message, 0)

	// 系统提示词逻辑
	var systemPrompt string

	// 1. 尝试从 Prompt Engine 加载
	if a.config.PromptTemplateID != nil && *a.config.PromptTemplateID != "" && a.promptEngine != nil {
		// 准备变量
		vars := make(map[string]any)
		// 默认注入 input.Variables
		if input.Variables != nil {
			for k, v := range input.Variables {
				vars[k] = v
			}
		}
		// 也可以注入其他上下文信息
		vars["Content"] = input.Content

		// 渲染模板
		rendered, err := a.promptEngine.Render(ctx, *a.config.PromptTemplateID, vars)
		if err != nil {
			// 如果模板加载失败，记录错误但尝试降级到硬编码 Prompt (或者直接返回错误)
			// 这里选择返回错误，因为配置了 TemplateID 就应该有效
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
		systemPrompt = "你是一个专业的项目规划专家，擅长将复杂任务分解为可执行的步骤。" +
			"你的任务是：\n" +
			"1. 分析任务目标和约束条件\n" +
			"2. 识别关键里程碑和依赖关系\n" +
			"3. 生成详细的执行计划\n" +
			"4. 评估风险并提供缓解措施\n" +
			"5. 输出结构化的任务清单（JSON/Markdown）\n\n" +
			"请基于历史案例和最佳实践，生成高质量的规划方案。"
	}

	// RAG 增强：将知识库上下文注入到系统提示词
	systemPrompt = InjectKnowledgeIntoPrompt(input, systemPrompt)

	// Memory 增强：注入长期记忆
	if memoryContext != "" {
		systemPrompt += "\n\n" + memoryContext
	}

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
	userContent := fmt.Sprintf("请为以下任务制定详细规划：\n\n%s", input.Content)

	// 如果有额外参数，添加规划要求
	if timeline, ok := input.ExtraParams["timeline"].(string); ok {
		userContent += fmt.Sprintf("\n\n时间要求：%s", timeline)
	}
	if resources, ok := input.ExtraParams["resources"].(string); ok {
		userContent += fmt.Sprintf("\n资源约束：%s", resources)
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: userContent,
	})

	return messages, nil
}
