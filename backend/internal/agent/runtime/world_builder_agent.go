package runtime

import (
	"context"
	"fmt"
	"time"

	"backend/internal/agent/prompt"
	"backend/internal/ai"
)

// WorldBuilderAgent 世界观构建 Agent
type WorldBuilderAgent struct {
	config       *AgentConfig
	modelClient  ai.ModelClient
	ragHelper    *RAGHelper
	toolHelper   *ToolHelper
	promptEngine *prompt.Engine
	name         string
}

// NewWorldBuilderAgent 创建 WorldBuilderAgent
func NewWorldBuilderAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper, promptEngine *prompt.Engine, toolHelper *ToolHelper) *WorldBuilderAgent {
	return &WorldBuilderAgent{
		config:       config,
		modelClient:  modelClient,
		ragHelper:    ragHelper,
		toolHelper:   toolHelper,
		promptEngine: promptEngine,
		name:         config.Name,
	}
}

// Execute 执行世界观构建任务（非流式）
func (a *WorldBuilderAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
	start := time.Now()

	// RAG 增强：从知识库检索优秀世界观设定案例
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

// ExecuteStream 执行世界观构建任务（流式）
func (a *WorldBuilderAgent) ExecuteStream(ctx context.Context, input *AgentInput) (<-chan AgentChunk, <-chan error) {
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
func (a *WorldBuilderAgent) Name() string {
	return a.name
}

// Type 返回 Agent 类型
func (a *WorldBuilderAgent) Type() string {
	return "world_builder"
}

// buildMessages 构建消息列表
func (a *WorldBuilderAgent) buildMessages(ctx context.Context, input *AgentInput) ([]ai.Message, error) {
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
		systemPrompt = `你是一位专业的世界观架构师，擅长构建完整且自洽的虚构世界。

构建要求：
1. 从核心创意出发，逐层展开
2. 包含：世界背景、势力体系、修炼/能力体系、社会结构
3. 设定之间要相互关联、逻辑自洽
4. 留有扩展空间，不要封死

输出格式（JSON）：
{
  "world": {
    "name": "世界名称",
    "background": "背景描述",
    "rules": ["核心规则1", "核心规则2"]
  },
  "power_system": {
    "name": "体系名称",
    "levels": ["等级1", "等级2"],
    "unique_features": ["特点1"]
  },
  "factions": [
    {"name": "势力名", "description": "描述", "stance": "立场"}
  ],
  "geography": {
    "regions": ["区域1", "区域2"]
  }
}`
	}

	// 根据角色类型调整提示词
	if role, ok := input.ExtraParams["role"].(string); ok {
		switch role {
		case "character_designer":
			systemPrompt = a.getCharacterDesignerPrompt()
		case "relation_mapper":
			systemPrompt = a.getRelationMapperPrompt()
		}
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
	if coreIdea, ok := input.ExtraParams["core_idea"].(string); ok && coreIdea != "" {
		userContent = fmt.Sprintf("核心创意：%s\n\n%s", coreIdea, userContent)
	}

	if genre, ok := input.ExtraParams["genre"].(string); ok && genre != "" {
		userContent += fmt.Sprintf("\n\n类型：%s", genre)
	}

	if scale, ok := input.ExtraParams["scale"].(string); ok && scale != "" {
		userContent += fmt.Sprintf("\n规模：%s", scale)
	}

	if worldSetting, ok := input.ExtraParams["world_setting"].(string); ok && worldSetting != "" {
		userContent += fmt.Sprintf("\n\n已有世界观设定：\n%s", worldSetting)
	}

	if entities, ok := input.ExtraParams["entities"].(string); ok && entities != "" {
		userContent += fmt.Sprintf("\n\n实体列表：\n%s", entities)
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: userContent,
	})

	return messages, nil
}

// getCharacterDesignerPrompt 返回角色设计师的系统提示词
func (a *WorldBuilderAgent) getCharacterDesignerPrompt() string {
	return `你是一位专业的角色设计师，擅长创造立体、有深度的人物形象。

设计要求：
1. 基本信息：姓名、年龄、外貌、身份
2. 性格特点：主要性格、性格成因、行为模式
3. 背景故事：过去经历、关键转折、心理创伤
4. 目标与动机：表面目标、深层动机、内心冲突
5. 人物弧光：成长方向、可能变化
6. 关系网络：与其他角色的关系和互动模式

输出格式（JSON）：
{
  "basic": {"name": "", "age": "", "appearance": "", "identity": ""},
  "personality": {"traits": [], "cause": "", "behavior_pattern": ""},
  "background": {"history": "", "turning_point": "", "trauma": ""},
  "motivation": {"surface_goal": "", "deep_motivation": "", "inner_conflict": ""},
  "arc": {"growth_direction": "", "potential_change": ""},
  "relationships": [{"target": "", "relation": "", "dynamic": ""}]
}`
}

// getRelationMapperPrompt 返回关系网络构建的系统提示词
func (a *WorldBuilderAgent) getRelationMapperPrompt() string {
	return `你是一位关系分析专家，擅长构建复杂的人物/势力关系网络。

分析维度：
1. 关系类型：血缘、师徒、敌对、同盟、暧昧等
2. 关系强度：亲密程度（1-5）
3. 关系动态：稳定/紧张/变化中
4. 利益关联：共同利益、利益冲突
5. 潜在发展：可能的关系变化

输出格式（JSON）：
{
  "nodes": [{"id": "", "name": "", "type": ""}],
  "edges": [
    {
      "source": "",
      "target": "",
      "relation_type": "",
      "strength": 3,
      "dynamic": "",
      "description": ""
    }
  ],
  "conflicts": [{"parties": [], "nature": "", "potential": ""}],
  "alliances": [{"members": [], "basis": "", "stability": ""}]
}`
}
