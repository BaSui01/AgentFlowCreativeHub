package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"backend/internal/agent/parser"
	"backend/internal/ai"
	"backend/internal/logger"
	"backend/internal/tools"

	"go.uber.org/zap"
)

// SubAgentType 子代理类型
type SubAgentType string

const (
	SubAgentExplore SubAgentType = "agent_explore"  // 探索代理 - 只读操作，快速理解代码库
	SubAgentPlan    SubAgentType = "agent_plan"     // 规划代理 - 分析需求，输出实现方案
	SubAgentGeneral SubAgentType = "agent_general"  // 通用代理 - 完整工具访问，实际操作
)

// SubAgentConfig 子代理配置
type SubAgentConfig struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	SystemPrompt   string            `json:"system_prompt"`
	AllowedTools   []string          `json:"allowed_tools"`
	ReadOnly       bool              `json:"read_only"`
	MaxIterations  int               `json:"max_iterations"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	ExtraConfig    map[string]any    `json:"extra_config,omitempty"`
}

// SubAgentMessage 子代理消息
type SubAgentMessage struct {
	Type      string         `json:"type"`      // tool_call, tool_result, thinking, output
	Content   string         `json:"content"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolArgs  map[string]any `json:"tool_args,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// SubAgentResult 子代理执行结果
type SubAgentResult struct {
	Success   bool           `json:"success"`
	Result    string         `json:"result"`
	Error     string         `json:"error,omitempty"`
	Usage     *Usage         `json:"usage,omitempty"`
	Messages  []SubAgentMessage `json:"messages,omitempty"`
	LatencyMs int64          `json:"latency_ms"`
}

// SubAgentService 子代理服务
type SubAgentService struct {
	mu            sync.RWMutex
	configs       map[string]*SubAgentConfig
	logger        *zap.Logger
	modelProvider ai.ModelProvider
	toolRegistry  *tools.ToolRegistry
	toolExecutor  *tools.ToolExecutor
	defaultModel  string // 默认模型 ID
}

// SubAgentServiceOption 子代理服务配置选项
type SubAgentServiceOption func(*SubAgentService)

// WithModelProvider 设置模型提供方
func WithModelProvider(provider ai.ModelProvider) SubAgentServiceOption {
	return func(s *SubAgentService) {
		s.modelProvider = provider
	}
}

// WithToolRegistry 设置工具注册表
func WithToolRegistry(registry *tools.ToolRegistry) SubAgentServiceOption {
	return func(s *SubAgentService) {
		s.toolRegistry = registry
	}
}

// WithToolExecutor 设置工具执行器
func WithToolExecutor(executor *tools.ToolExecutor) SubAgentServiceOption {
	return func(s *SubAgentService) {
		s.toolExecutor = executor
	}
}

// WithDefaultModel 设置默认模型
func WithDefaultModel(modelID string) SubAgentServiceOption {
	return func(s *SubAgentService) {
		s.defaultModel = modelID
	}
}

// NewSubAgentService 创建子代理服务
func NewSubAgentService(opts ...SubAgentServiceOption) *SubAgentService {
	svc := &SubAgentService{
		configs: make(map[string]*SubAgentConfig),
		logger:  logger.Get(),
	}
	for _, opt := range opts {
		opt(svc)
	}
	svc.initBuiltinConfigs()
	return svc
}

// initBuiltinConfigs 初始化内置子代理配置
func (s *SubAgentService) initBuiltinConfigs() {
	// 探索代理 - 专门用于快速探索和理解代码库
	s.configs[string(SubAgentExplore)] = &SubAgentConfig{
		ID:          string(SubAgentExplore),
		Name:        "探索代理",
		Description: "专门用于快速探索和理解代码库。擅长搜索代码、查找定义、分析代码结构和依赖关系。只读操作，不会修改文件或执行命令。",
		SystemPrompt: `你是一个代码探索专家。你的任务是帮助用户理解代码库结构、查找代码定义和依赖关系。

核心能力：
- 搜索代码库中的符号和引用
- 分析代码结构和依赖关系
- 查找函数、类、变量的定义
- 理解代码的设计模式和架构

重要限制：
- 你只能执行只读操作
- 不能修改任何文件
- 不能执行终端命令
- 专注于代码理解和分析

输出要求：
- 提供清晰的代码结构说明
- 列出相关的文件路径和行号
- 解释代码之间的依赖关系`,
		AllowedTools: []string{
			"ace-code-search",
			"codebase-search",
			"filesystem",
			"web_search",
			"knowledge_search",
		},
		ReadOnly:       true,
		MaxIterations:  10,
		TimeoutSeconds: 120,
	}

	// 规划代理 - 专门用于分析需求和制定实现计划
	s.configs[string(SubAgentPlan)] = &SubAgentConfig{
		ID:          string(SubAgentPlan),
		Name:        "规划代理",
		Description: "专门用于规划复杂任务。分析需求、探索代码、识别相关文件，并创建详细的实现计划。只读操作，输出结构化的实现方案。",
		SystemPrompt: `你是一个软件架构和规划专家。你的任务是分析需求并制定详细的实现计划。

核心能力：
- 分析业务需求和技术要求
- 探索现有代码库结构
- 识别需要修改的文件和模块
- 制定分步实现计划
- 评估风险和依赖关系

输出格式：
1. 需求分析
2. 影响范围评估
3. 实现步骤（按优先级排序）
4. 风险点和注意事项
5. 测试建议

重要限制：
- 你只能执行只读操作
- 不能直接修改代码
- 专注于规划和分析`,
		AllowedTools: []string{
			"ace-code-search",
			"codebase-search",
			"filesystem",
			"web_search",
			"knowledge_search",
			"text_statistics",
		},
		ReadOnly:       true,
		MaxIterations:  15,
		TimeoutSeconds: 180,
	}

	// 通用代理 - 完整工具访问，执行实际操作
	s.configs[string(SubAgentGeneral)] = &SubAgentConfig{
		ID:          string(SubAgentGeneral),
		Name:        "通用代理",
		Description: "通用多步骤任务执行代理。拥有完整的工具访问权限，可以搜索、修改文件和执行命令。适合需要实际操作的复杂任务。",
		SystemPrompt: `你是一个全能的软件开发代理。你可以执行各种开发任务，包括代码编写、文件修改和命令执行。

核心能力：
- 搜索和理解代码
- 创建和修改文件
- 执行终端命令
- 完成复杂的多步骤任务

工作流程：
1. 理解任务要求
2. 分析现有代码
3. 制定执行计划
4. 逐步实施
5. 验证结果

注意事项：
- 在修改前先理解代码
- 保持代码风格一致
- 处理好错误情况
- 记录重要的更改`,
		AllowedTools: []string{
			"ace-code-search",
			"codebase-search",
			"filesystem",
			"terminal-execute",
			"todo-manager",
			"notebook",
			"web_search",
			"knowledge_search",
			"http_api",
			"calculator",
			"text_statistics",
			"text_converter",
			"keyword_extractor",
			"text_summarizer",
		},
		ReadOnly:       false,
		MaxIterations:  20,
		TimeoutSeconds: 300,
	}
}

// RegisterConfig 注册自定义子代理配置
func (s *SubAgentService) RegisterConfig(config *SubAgentConfig) error {
	if config == nil || config.ID == "" {
		return errors.New("无效的子代理配置")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs[config.ID] = config
	return nil
}

// GetConfig 获取子代理配置
func (s *SubAgentService) GetConfig(agentID string) (*SubAgentConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.configs[agentID]
	return config, exists
}

// ListConfigs 列出所有子代理配置
func (s *SubAgentService) ListConfigs() []*SubAgentConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make([]*SubAgentConfig, 0, len(s.configs))
	for _, config := range s.configs {
		configs = append(configs, config)
	}
	return configs
}

// ExecuteOptions 执行选项
type ExecuteOptions struct {
	AgentID     string                      // 子代理 ID
	Prompt      string                      // 用户提示词
	TenantID    string                      // 租户 ID
	UserID      string                      // 用户 ID
	ModelID     string                      // 模型 ID（可选，默认使用服务配置）
	Temperature float64                     // 温度参数（0-1）
	OnMessage   func(msg SubAgentMessage)   // 消息回调
	YoloMode    bool                        // 自动批准所有工具调用
}

// Execute 执行子代理
func (s *SubAgentService) Execute(ctx context.Context, opts *ExecuteOptions) (*SubAgentResult, error) {
	if opts == nil || opts.AgentID == "" || opts.Prompt == "" {
		return nil, errors.New("无效的执行参数")
	}

	config, exists := s.GetConfig(opts.AgentID)
	if !exists {
		return nil, fmt.Errorf("未找到子代理配置: %s", opts.AgentID)
	}

	startTime := time.Now()
	messages := make([]SubAgentMessage, 0)

	// 创建超时上下文
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 发送开始消息
	s.emitMessage(opts, &messages, SubAgentMessage{
		Type:      "thinking",
		Content:   fmt.Sprintf("子代理 [%s] 开始执行任务...", config.Name),
		Timestamp: time.Now(),
	})

	// 检查依赖是否已注入
	if s.modelProvider == nil {
		return nil, errors.New("模型提供方未配置")
	}

	// 确定模型 ID
	modelID := opts.ModelID
	if modelID == "" {
		modelID = s.defaultModel
	}
	if modelID == "" {
		return nil, errors.New("未指定模型 ID")
	}

	// 获取 AI 客户端
	client, err := s.modelProvider.GetClient(ctx, opts.TenantID, modelID)
	if err != nil {
		return &SubAgentResult{
			Success:   false,
			Error:     fmt.Sprintf("获取模型客户端失败: %v", err),
			Messages:  messages,
			LatencyMs: time.Since(startTime).Milliseconds(),
		}, err
	}

	// 构建初始消息
	aiMessages := []ai.Message{
		{Role: "system", Content: config.SystemPrompt},
		{Role: "user", Content: opts.Prompt},
	}

	// 获取工具定义
	toolDefs := s.buildToolDefinitions(config.AllowedTools)

	// 设置温度
	temperature := opts.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	// 执行循环
	var totalUsage Usage
	maxIterations := config.MaxIterations
	if maxIterations == 0 {
		maxIterations = 10
	}

	for iteration := 0; iteration < maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return &SubAgentResult{
				Success:   false,
				Error:     "执行超时或被取消",
				Messages:  messages,
				LatencyMs: time.Since(startTime).Milliseconds(),
				Usage:     &totalUsage,
			}, ctx.Err()
		default:
		}

		s.emitMessage(opts, &messages, SubAgentMessage{
			Type:      "thinking",
			Content:   fmt.Sprintf("迭代 %d/%d: 调用 AI 模型...", iteration+1, maxIterations),
			Timestamp: time.Now(),
		})

		// 调用 AI 模型
		aiReq := &ai.ChatCompletionRequest{
			Messages:    aiMessages,
			Temperature: temperature,
			MaxTokens:   4096,
			Tools:       toolDefs,
			ToolChoice:  "auto",
		}

		aiResp, err := client.ChatCompletion(ctx, aiReq)
		if err != nil {
			return &SubAgentResult{
				Success:   false,
				Error:     fmt.Sprintf("AI 调用失败: %v", err),
				Messages:  messages,
				LatencyMs: time.Since(startTime).Milliseconds(),
				Usage:     &totalUsage,
			}, err
		}

		// 累计 Token 用量
		totalUsage.PromptTokens += aiResp.Usage.PromptTokens
		totalUsage.CompletionTokens += aiResp.Usage.CompletionTokens
		totalUsage.TotalTokens += aiResp.Usage.TotalTokens

		// 检查是否需要调用工具
		if len(aiResp.ToolCalls) == 0 {
			// 无工具调用，返回最终结果
			s.emitMessage(opts, &messages, SubAgentMessage{
				Type:      "output",
				Content:   aiResp.Content,
				Timestamp: time.Now(),
			})

			s.logger.Info("子代理执行完成",
				zap.String("agent_id", opts.AgentID),
				zap.Int("iterations", iteration+1),
				zap.Int64("latency_ms", time.Since(startTime).Milliseconds()),
			)

			return &SubAgentResult{
				Success:   true,
				Result:    aiResp.Content,
				Messages:  messages,
				LatencyMs: time.Since(startTime).Milliseconds(),
				Usage:     &totalUsage,
			}, nil
		}

		// 执行工具调用
		toolResults, toolMessages := s.executeToolCalls(ctx, aiResp.ToolCalls, config, opts)
		messages = append(messages, toolMessages...)

		// 更新对话历史
		aiMessages = append(aiMessages, ai.Message{
			Role:      "assistant",
			Content:   aiResp.Content,
			ToolCalls: aiResp.ToolCalls,
		})

		for i, toolCall := range aiResp.ToolCalls {
			aiMessages = append(aiMessages, ai.Message{
				Role:       "tool",
				Content:    toolResults[i],
				ToolCallID: toolCall.ID,
				Name:       toolCall.Function.Name,
			})
		}
	}

	// 超过最大迭代次数
	s.logger.Warn("子代理超过最大迭代次数",
		zap.String("agent_id", opts.AgentID),
		zap.Int("max_iterations", maxIterations),
	)

	return &SubAgentResult{
		Success:   false,
		Error:     fmt.Sprintf("超过最大迭代次数 (%d)", maxIterations),
		Messages:  messages,
		LatencyMs: time.Since(startTime).Milliseconds(),
		Usage:     &totalUsage,
	}, nil
}

// emitMessage 发送消息并记录
func (s *SubAgentService) emitMessage(opts *ExecuteOptions, messages *[]SubAgentMessage, msg SubAgentMessage) {
	*messages = append(*messages, msg)
	if opts.OnMessage != nil {
		opts.OnMessage(msg)
	}
}

// buildToolDefinitions 构建工具定义
func (s *SubAgentService) buildToolDefinitions(allowedTools []string) []ai.Tool {
	if s.toolRegistry == nil || len(allowedTools) == 0 {
		return nil
	}

	toolDefs := make([]ai.Tool, 0, len(allowedTools))
	for _, toolName := range allowedTools {
		if def, exists := s.toolRegistry.GetDefinition(toolName); exists && def.Status == "active" {
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
	return toolDefs
}

// executeToolCalls 执行工具调用
func (s *SubAgentService) executeToolCalls(
	ctx context.Context,
	toolCalls []ai.ToolCall,
	config *SubAgentConfig,
	opts *ExecuteOptions,
) ([]string, []SubAgentMessage) {
	results := make([]string, len(toolCalls))
	messages := make([]SubAgentMessage, 0)

	// 并发执行工具调用
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, toolCall := range toolCalls {
		wg.Add(1)
		go func(idx int, tc ai.ToolCall) {
			defer wg.Done()

			// 记录工具调用消息
			mu.Lock()
			messages = append(messages, SubAgentMessage{
				Type:      "tool_call",
				Content:   fmt.Sprintf("调用工具: %s", tc.Function.Name),
				ToolName:  tc.Function.Name,
				Timestamp: time.Now(),
			})
			mu.Unlock()

			// 检查只读模式
			if config.ReadOnly && !s.isReadOnlyTool(tc.Function.Name) {
				mu.Lock()
				results[idx] = fmt.Sprintf("错误: 子代理 [%s] 为只读模式，不允许调用工具 %s", config.Name, tc.Function.Name)
				mu.Unlock()
				return
			}

			// 解析参数
			argsStr := parser.RepairJSON(tc.Function.Arguments)
			var params map[string]any
			if err := json.Unmarshal([]byte(argsStr), &params); err != nil {
				mu.Lock()
				results[idx] = fmt.Sprintf("参数解析失败: %s", err.Error())
				mu.Unlock()
				return
			}

			// 执行工具
			var resultStr string
			if s.toolExecutor != nil {
				execReq := &tools.ToolExecutionRequest{
					TenantID: opts.TenantID,
					ToolID:   tc.Function.Name,
					ToolName: tc.Function.Name,
					Input:    params,
					AgentID:  opts.UserID,
					Timeout:  30,
				}

				execResult, err := s.toolExecutor.Execute(ctx, execReq)
				if err != nil {
					resultStr = fmt.Sprintf("工具执行失败: %s", err.Error())
				} else {
					resultJSON, _ := json.Marshal(execResult.Output)
					resultStr = string(resultJSON)
				}
			} else {
				resultStr = fmt.Sprintf("工具执行器未配置，无法执行工具 %s", tc.Function.Name)
			}

			// 记录工具结果消息
			mu.Lock()
			results[idx] = resultStr
			messages = append(messages, SubAgentMessage{
				Type:      "tool_result",
				Content:   resultStr,
				ToolName:  tc.Function.Name,
				Timestamp: time.Now(),
			})
			mu.Unlock()
		}(i, toolCall)
	}

	wg.Wait()
	return results, messages
}

// isReadOnlyTool 检查是否为只读工具
func (s *SubAgentService) isReadOnlyTool(toolName string) bool {
	readOnlyTools := map[string]bool{
		"ace-code-search":    true,
		"codebase-search":    true,
		"web_search":         true,
		"knowledge_search":   true,
		"text_statistics":    true,
		"keyword_extractor":  true,
		"calculator":         true,
	}
	return readOnlyTools[toolName]
}

// GetToolDefinitions 获取子代理的工具定义（用于 MCP）
func (s *SubAgentService) GetToolDefinitions() []map[string]any {
	configs := s.ListConfigs()
	tools := make([]map[string]any, 0, len(configs))

	for _, config := range configs {
		tool := map[string]any{
			"name":        config.ID,
			"description": fmt.Sprintf("%s: %s", config.Name, config.Description),
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]any{
						"type":        "string",
						"description": "【重要】必须提供完整的上下文信息。子代理无法访问主会话的对话历史。包含：(1) 完整的任务描述和业务需求；(2) 已知的文件位置和代码路径；(3) 已发现的相关代码片段或模式；(4) 任何约束条件或重要上下文。",
					},
				},
				"required": []string{"prompt"},
			},
		}
		tools = append(tools, tool)
	}

	return tools
}
