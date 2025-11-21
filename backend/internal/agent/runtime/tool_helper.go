package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"backend/internal/agent/parser"
	"backend/internal/ai"
	"backend/internal/tools"
)

// ToolHelper 工具调用辅助类
type ToolHelper struct {
	registry *tools.ToolRegistry
	executor *tools.ToolExecutor
	tracer   trace.Tracer
}

// NewToolHelper 创建工具辅助类
func NewToolHelper(registry *tools.ToolRegistry, executor *tools.ToolExecutor) *ToolHelper {
	return &ToolHelper{
		registry: registry,
		executor: executor,
		tracer:   otel.Tracer("backend/internal/agent/runtime"),
	}
}

// ExecuteWithTools 带工具调用的执行（支持多轮对话和并发工具执行）
func (h *ToolHelper) ExecuteWithTools(
	ctx context.Context,
	client ai.ModelClient,
	messages []ai.Message,
	temperature float64,
	maxTokens int,
	tenantID string,
	userID string,
	availableTools []string, // 可用工具名称列表
) (*ai.ChatCompletionResponse, error) {

	ctx, span := h.tracer.Start(ctx, "ToolHelper.ExecuteWithTools")
	defer span.End()

	span.SetAttributes(
		attribute.Int("available_tools_count", len(availableTools)),
		attribute.String("tenant_id", tenantID),
		attribute.String("user_id", userID),
	)

	if len(availableTools) == 0 {
		return client.ChatCompletion(ctx, &ai.ChatCompletionRequest{
			Messages:    messages,
			Temperature: temperature,
			MaxTokens:   maxTokens,
		})
	}

	// 获取工具定义
	toolDefs := make([]ai.Tool, 0, len(availableTools))
	for _, toolName := range availableTools {
		if def, exists := h.registry.GetDefinition(toolName); exists && def.Status == "active" {
			toolDefs = append(toolDefs, ai.Tool{
				Type: "function",
				Function: ai.FunctionDef{
					Name:        def.Name,
					Description: def.Description,
					Parameters:  def.Parameters,
				},
			})
		}
	}

	if len(toolDefs) == 0 {
		return client.ChatCompletion(ctx, &ai.ChatCompletionRequest{
			Messages:    messages,
			Temperature: temperature,
			MaxTokens:   maxTokens,
		})
	}

	maxRounds := 5
	conversationMessages := make([]ai.Message, len(messages))
	copy(conversationMessages, messages)

	for round := 0; round < maxRounds; round++ {
		roundSpanCtx, roundSpan := h.tracer.Start(ctx, fmt.Sprintf("Round-%d", round))
		
		// 1. 调用 AI 模型
		aiReq := &ai.ChatCompletionRequest{
			Messages:    conversationMessages,
			Temperature: temperature,
			MaxTokens:   maxTokens,
			Tools:       toolDefs,
			ToolChoice:  "auto",
		}

		aiResp, err := client.ChatCompletion(roundSpanCtx, aiReq)
		if err != nil {
			roundSpan.RecordError(err)
			roundSpan.SetStatus(codes.Error, "AI Model call failed")
			roundSpan.End()
			return nil, fmt.Errorf("AI 调用失败: %w", err)
		}

		// 2. 检查是否需要调用工具
		if len(aiResp.ToolCalls) == 0 {
			roundSpan.End()
			return aiResp, nil
		}

		roundSpan.SetAttributes(attribute.Int("tool_calls_count", len(aiResp.ToolCalls)))

		// 3. 并发执行工具调用
		toolResults := make([]string, len(aiResp.ToolCalls))
		var wg sync.WaitGroup
		var mu sync.Mutex

		for i, toolCall := range aiResp.ToolCalls {
			wg.Add(1)
			go func(idx int, tc ai.ToolCall) {
				defer wg.Done()
				
				// Start a span for each tool execution
				_, toolSpan := h.tracer.Start(roundSpanCtx, fmt.Sprintf("ToolExecute:%s", tc.Function.Name))
				defer toolSpan.End()
				
				toolSpan.SetAttributes(attribute.String("tool_name", tc.Function.Name))

				// 健壮的 JSON 解析
				argsStr := parser.RepairJSON(tc.Function.Arguments)
				
				var params map[string]any
				if err := json.Unmarshal([]byte(argsStr), &params); err != nil {
					mu.Lock()
					toolResults[idx] = fmt.Sprintf("参数解析失败: %s", err.Error())
					mu.Unlock()
					toolSpan.RecordError(err)
					toolSpan.SetStatus(codes.Error, "JSON unmarshal failed")
					return
				}

				execReq := &tools.ToolExecutionRequest{
					TenantID: tenantID,
					ToolID:   tc.Function.Name,
					ToolName: tc.Function.Name,
					Input:    params,
					AgentID:  userID,
					Timeout:  30,
				}

				execResult, err := h.executor.Execute(ctx, execReq) // propagate ctx? usually better to use spanCtx but executor might need cancellation
				var resultStr string
				if err != nil {
					resultStr = fmt.Sprintf("工具执行失败: %s", err.Error())
					toolSpan.RecordError(err)
					toolSpan.SetStatus(codes.Error, "Tool execution failed")
				} else {
					resultJSON, _ := json.Marshal(execResult.Output)
					resultStr = string(resultJSON)
				}

				mu.Lock()
				toolResults[idx] = resultStr
				mu.Unlock()
			}(i, toolCall)
		}

		wg.Wait()
		roundSpan.End()

		// 4. 更新对话历史
		// 4.1 添加 Assistant 的 ToolCall 消息
		assistantMsg := ai.Message{
			Role:      "assistant",
			Content:   aiResp.Content, // 可能为空
			ToolCalls: aiResp.ToolCalls,
		}
		conversationMessages = append(conversationMessages, assistantMsg)

		// 4.2 添加 Tool 结果消息
		for i, toolCall := range aiResp.ToolCalls {
			conversationMessages = append(conversationMessages, ai.Message{
				Role:       "tool",
				Content:    toolResults[i],
				ToolCallID: toolCall.ID,
				Name:       toolCall.Function.Name,
			})
		}
	}

	span.RecordError(fmt.Errorf("max rounds exceeded"))
	span.SetStatus(codes.Error, "Max rounds exceeded")
	return nil, fmt.Errorf("超过最大工具调用轮次 (%d)", maxRounds)
}

// GetAvailableTools 获取指定类别的可用工具
func (h *ToolHelper) GetAvailableTools(category string) []string {
	if category == "" {
		allTools := h.registry.List()
		names := make([]string, 0, len(allTools))
		for _, tool := range allTools {
			if tool.Status == "active" {
				names = append(names, tool.Name)
			}
		}
		return names
	}

	categoryTools := h.registry.ListByCategory(category)
	names := make([]string, 0, len(categoryTools))
	for _, tool := range categoryTools {
		if tool.Status == "active" {
			names = append(names, tool.Name)
		}
	}
	return names
}
