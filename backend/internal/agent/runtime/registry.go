package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	agentpkg "backend/internal/agent"
	"backend/internal/agent/prompt"
	"backend/internal/ai"
	"backend/internal/metrics"

	"gorm.io/gorm"
)

// Registry Agent 注册表
type Registry struct {
	db                  *gorm.DB
	clientProvider      ai.ModelProvider
	contextManager      *ContextManager
	ragHelper           *RAGHelper       // RAG 辅助工具
	toolHelper          *ToolHelper      // 工具调用辅助
	memoryService       MemoryService    // Memory 服务
	promptEngine        *prompt.Engine   // Prompt 引擎
	agents              map[string]Agent // 缓存：agentConfigID -> Agent
	defaultHistoryLimit int              // 会话历史窗口大小（条数，<=0 表示全量）
	mu                  sync.RWMutex
}

// 内部常量: 记忆模式与摘要存储 key
const (
	memoryModeFull    = "full"
	memoryModeWindow  = "window"
	memoryModeSummary = "summary"

	memorySummaryKey             = "memory_summary"               // 会话摘要内容
	memorySummaryMessageCountKey = "memory_summary_message_count" // 生成摘要时的历史消息条数
)

// NewRegistry 创建 Agent 注册表
func NewRegistry(db *gorm.DB, clientProvider ai.ModelProvider) *Registry {
	// 初始化 Prompt 引擎 (使用 DB 加载器)
	promptLoader := prompt.NewDBLoader(db)
	promptEngine := prompt.NewEngine(promptLoader)

	return &Registry{
		db:                  db,
		clientProvider:      clientProvider,
		contextManager:      NewContextManager(NewInMemorySessionStore()),
		ragHelper:           nil, // 将在 SetRAGHelper 中设置
		memoryService:       nil, // 将在 SetMemoryService 中设置
		promptEngine:        promptEngine,
		defaultHistoryLimit: 10,
		agents:              make(map[string]Agent),
	}
}

// SetRAGHelper 设置 RAG 辅助工具
func (r *Registry) SetRAGHelper(ragHelper *RAGHelper) {
	r.ragHelper = ragHelper
}

// SetToolHelper 设置工具辅助类
func (r *Registry) SetToolHelper(toolHelper *ToolHelper) {
	r.toolHelper = toolHelper
}

// SetMemoryService 设置记忆服务
func (r *Registry) SetMemoryService(memoryService MemoryService) {
	r.memoryService = memoryService
}

// GetAgent 获取 Agent 实例
// 从数据库加载配置并创建对应的 Agent
func (r *Registry) GetAgent(ctx context.Context, tenantID, agentID string) (Agent, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%s", tenantID, agentID)
	r.mu.RLock()
	if agent, ok := r.agents[cacheKey]; ok {
		r.mu.RUnlock()
		return agent, nil
	}
	r.mu.RUnlock()

	// 从数据库加载配置
	var config agentpkg.AgentConfig
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", agentID, tenantID).
		First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Agent 配置不存在: %s", agentID)
		}
		return nil, fmt.Errorf("查询 Agent 配置失败: %w", err)
	}

	// 创建 Agent
	agent, err := r.createAgent(ctx, &config)
	if err != nil {
		return nil, err
	}

	// 缓存 Agent
	r.mu.Lock()
	r.agents[cacheKey] = agent
	r.mu.Unlock()

	return agent, nil
}

// GetAgentByType 根据类型获取默认 Agent
func (r *Registry) GetAgentByType(ctx context.Context, tenantID, agentType string) (Agent, error) {
	// 从数据库查询该类型的第一个 Agent
	var config agentpkg.AgentConfig
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND agent_type = ? AND deleted_at IS NULL", tenantID, agentType).
		Order("created_at DESC").
		First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("未找到 %s 类型的 Agent 配置", agentType)
		}
		return nil, fmt.Errorf("查询 Agent 配置失败: %w", err)
	}

	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%s", tenantID, config.ID)
	r.mu.RLock()
	if agent, ok := r.agents[cacheKey]; ok {
		r.mu.RUnlock()
		return agent, nil
	}
	r.mu.RUnlock()

	// 创建 Agent
	agent, err := r.createAgent(ctx, &config)
	if err != nil {
		return nil, err
	}

	// 缓存 Agent
	r.mu.Lock()
	r.agents[cacheKey] = agent
	r.mu.Unlock()

	return agent, nil
}

// createAgent 创建 Agent 实例
func (r *Registry) createAgent(ctx context.Context, config *agentpkg.AgentConfig) (Agent, error) {
	// 获取模型客户端
	modelClient, err := r.clientProvider.GetClient(ctx, config.TenantID, config.ModelID)
	if err != nil {
		return nil, fmt.Errorf("获取模型客户端失败: %w", err)
	}

	// 转换配置
	agentConfig := &AgentConfig{
		Name:         config.Name,
		Type:         config.AgentType,
		ModelID:      config.ModelID,
		SystemPrompt: config.SystemPrompt,
		Temperature:  config.Temperature,
		MaxTokens:    config.MaxTokens,
		ExtraConfig:  config.ExtraConfig,
		AgentConfig:  config, // 保存完整配置（用于 RAG）
	}

	if config.PromptTemplateID != "" {
		agentConfig.PromptTemplateID = &config.PromptTemplateID
	}

	// 根据类型创建对应的 Agent（所有 Agent 都支持工具调用）
	switch config.AgentType {
	case "writer":
		return NewWriterAgent(agentConfig, modelClient, r.ragHelper, r.promptEngine, r.toolHelper), nil
	case "reviewer":
		return NewReviewerAgent(agentConfig, modelClient, r.ragHelper, r.promptEngine, r.toolHelper), nil
	case "formatter":
		return NewFormatterAgent(agentConfig, modelClient, r.ragHelper, r.promptEngine, r.toolHelper), nil
	case "planner":
		return NewPlannerAgent(agentConfig, modelClient, r.ragHelper, r.promptEngine, r.memoryService, r.toolHelper), nil
	case "translator":
		return NewTranslatorAgent(agentConfig, modelClient, r.ragHelper, r.promptEngine, r.toolHelper), nil
	case "analyzer":
		return NewAnalyzerAgent(agentConfig, modelClient, r.ragHelper, r.promptEngine, r.toolHelper), nil
	case "researcher":
		return NewResearcherAgent(agentConfig, modelClient, r.ragHelper, r.promptEngine, r.toolHelper), nil
	default:
		return nil, fmt.Errorf("不支持的 Agent 类型: %s", config.AgentType)
	}
}

// Execute 执行 Agent（便捷方法）
func (r *Registry) Execute(ctx context.Context, tenantID, agentID string, input *AgentInput) (*AgentResult, error) {
	// 获取 Agent
	agent, err := r.GetAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}

	// 获取 Agent 类型
	agentType := agent.Type()

	// Prometheus 指标：增加正在执行的 Agent 数量
	metrics.AgentExecutionsRunning.WithLabelValues(agentType).Inc()
	defer metrics.AgentExecutionsRunning.WithLabelValues(agentType).Dec()

	// 记录开始时间
	start := time.Now()

	// 如果有会话 ID，添加历史对话（支持按步骤配置窗口大小）
	if input != nil && input.Context != nil && input.Context.SessionID != nil {
		limit := r.defaultHistoryLimit
		if limit <= 0 {
			// <=0 表示全量历史
			limit = 0
		}
		// 允许通过 ExtraParams 覆盖窗口大小（history_limit）
		if input.ExtraParams != nil {
			if v, ok := input.ExtraParams["history_limit"]; ok {
				if n, ok2 := convertToInt(v); ok2 {
					if n < 0 {
						limit = 0
					} else {
						limit = n
					}
				}
			}
		}

		if err := r.contextManager.EnrichInput(ctx, input, *input.Context.SessionID, limit, 0, ""); err != nil {
			// 忽略错误，继续执行
		}
	}

	// 自动上下文压缩策略 (新增)
	// 默认触发阈值: 4000 tokens
	compressThreshold := 4000
	if input.ExtraParams != nil {
		if v, ok := input.ExtraParams["compress_threshold"]; ok {
			if n, ok2 := convertToInt(v); ok2 && n > 0 {
				compressThreshold = n
			}
		}
	}

	// 计算当前 Token 数
	if len(input.History) > 2 {
		currentTokens, _ := CalculateTokenCount(input.History, "gpt-3.5-turbo")
		if currentTokens > compressThreshold {
			// 触发压缩
			newHistory, err := r.CompressHistory(ctx, tenantID, input.History, compressThreshold)
			if err == nil {
				input.History = newHistory
				// 记录压缩操作日志? 暂不记录
			} else {
				// 压缩失败，仅打印日志，继续执行
				fmt.Printf("Context compression failed: %v\n", err)
			}
		}
	}

	// 执行 Agent
	result, err := agent.Execute(ctx, input)

	// Prometheus 指标：记录执行耗时和结果
	duration := time.Since(start).Seconds()
	metrics.AgentExecutionDuration.WithLabelValues(agentType, tenantID).Observe(duration)

	status := "success"
	if err != nil {
		status = "failed"
	}
	metrics.AgentExecutionsTotal.WithLabelValues(agentType, status, tenantID).Inc()

	// 如果执行成功，记录 Token 使用情况
	if err == nil && result != nil && result.Usage != nil {
		if result.Usage.PromptTokens > 0 {
			metrics.AgentInputTokens.WithLabelValues(agentType, tenantID).Add(float64(result.Usage.PromptTokens))
		}
		if result.Usage.CompletionTokens > 0 {
			metrics.AgentOutputTokens.WithLabelValues(agentType, tenantID).Add(float64(result.Usage.CompletionTokens))
		}
	}

	if err != nil {
		return result, err
	}

	// 如果有会话 ID，保存交互
	if input.Context != nil && input.Context.SessionID != nil {
		if err := r.contextManager.SaveInteraction(ctx, *input.Context.SessionID, input.Content, result.Output); err != nil {
			// 忽略错误
		}

		// 在后台尝试更新会话摘要记忆
		r.maybeSummarizeSession(ctx, tenantID, agentID, input)
	}

	return result, nil
}

// ExecuteStream 执行 Agent（流式，便捷方法）
func (r *Registry) ExecuteStream(ctx context.Context, tenantID, agentID string, input *AgentInput) (<-chan AgentChunk, <-chan error) {
	outChan := make(chan AgentChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		// 获取 Agent
		agent, err := r.GetAgent(ctx, tenantID, agentID)
		if err != nil {
			errChan <- err
			return
		}

		// 如果有会话 ID，添加历史对话（支持按步骤配置窗口大小）
		if input != nil && input.Context != nil && input.Context.SessionID != nil {
			limit := r.defaultHistoryLimit
			if limit <= 0 {
				limit = 0
			}
			if input.ExtraParams != nil {
				if v, ok := input.ExtraParams["history_limit"]; ok {
					if n, ok2 := convertToInt(v); ok2 {
						if n < 0 {
							limit = 0
						} else {
							limit = n
						}
					}
				}
			}

			if err := r.contextManager.EnrichInput(ctx, input, *input.Context.SessionID, limit, 0, ""); err != nil {
				// 忽略错误
			}
		}

		// 执行 Agent
		chunkChan, agentErrChan := agent.ExecuteStream(ctx, input)

		var fullOutput string

		// 转发响应块
		for chunk := range chunkChan {
			outChan <- chunk
			if !chunk.Done {
				fullOutput += chunk.Content
			}
		}

		// 检查错误
		select {
		case err := <-agentErrChan:
			if err != nil {
				errChan <- err
				return
			}
		default:
		}

		// 如果有会话 ID，保存交互
		if input.Context != nil && input.Context.SessionID != nil && fullOutput != "" {
			if err := r.contextManager.SaveInteraction(ctx, *input.Context.SessionID, input.Content, fullOutput); err != nil {
				// 忽略错误
			}

			// 在后台尝试更新会话摘要记忆
			r.maybeSummarizeSession(ctx, tenantID, agentID, input)
		}
	}()

	return outChan, errChan
}

// ClearCache 清除缓存
func (r *Registry) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents = make(map[string]Agent)
}

// ClearCacheForAgent 清除指定 Agent 的缓存
func (r *Registry) ClearCacheForAgent(tenantID, agentID string) {
	cacheKey := fmt.Sprintf("%s:%s", tenantID, agentID)
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.agents, cacheKey)
}

// GetContextManager 获取上下文管理器
func (r *Registry) GetContextManager() *ContextManager {
	return r.contextManager
}

// CompressHistory 压缩历史消息
func (r *Registry) CompressHistory(ctx context.Context, tenantID string, history []Message, targetTokens int) ([]Message, error) {
	// 1. 检查是否需要压缩
	if len(history) <= 2 {
		return history, nil
	}

	// 2. 确定保留多少近期消息
	// 策略: 总是保留 System Prompt (如果是第一条)
	// 保留最近的消息直到达到 targetTokens 的一半
	// 剩余的旧消息进行摘要

	var systemPrompt *Message
	messagesToProcess := history
	if len(history) > 0 && history[0].Role == "system" {
		systemPrompt = &history[0]
		messagesToProcess = history[1:]
	}

	if len(messagesToProcess) == 0 {
		return history, nil
	}

	// 倒序计算近期消息
	reservedTokens := targetTokens / 2
	currentTokens := 0
	splitIndex := 0 // messagesToProcess 中的分割点

	for i := len(messagesToProcess) - 1; i >= 0; i-- {
		msg := messagesToProcess[i]
		tokens, _ := CalculateTokenCount([]Message{msg}, "gpt-3.5-turbo")
		if currentTokens+tokens > reservedTokens {
			splitIndex = i + 1
			break
		}
		currentTokens += tokens
	}

	// 如果分割点很靠前，说明大部分都是近期消息，或者无法压缩
	if splitIndex <= 1 {
		return history, nil
	}

	toSummarize := messagesToProcess[:splitIndex]
	recentMessages := messagesToProcess[splitIndex:]

	// 3. 调用 AI 生成摘要
	summaryModelID := "gpt-3.5-turbo"

	// 尝试获取客户端
	modelClient, err := r.clientProvider.GetClient(ctx, tenantID, summaryModelID)
	if err != nil {
		return history, fmt.Errorf("failed to get summary model client: %w", err)
	}

	var builder strings.Builder
	for _, msg := range toSummarize {
		builder.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, msg.Content))
	}

	req := &ai.ChatCompletionRequest{
		Messages: []ai.Message{
			{Role: "system", Content: "请简要总结以下对话历史，保留关键信息。"},
			{Role: "user", Content: builder.String()},
		},
		MaxTokens:   500, // 摘要不应太长
		Temperature: 0.3,
	}

	resp, err := modelClient.ChatCompletion(ctx, req)
	if err != nil {
		return history, fmt.Errorf("summary generation failed: %w", err)
	}

	// 4. 构建新历史
	newHistory := make([]Message, 0)
	if systemPrompt != nil {
		newHistory = append(newHistory, *systemPrompt)
	}
	newHistory = append(newHistory, Message{
		Role:    "system",
		Content: fmt.Sprintf("【历史对话摘要】: %s", resp.Content),
	})
	newHistory = append(newHistory, recentMessages...)

	return newHistory, nil
}

// maybeSummarizeSession 在启用摘要记忆时,根据会话历史生成/更新会话摘要
// 触发条件:
// - ExtraParams["memory_mode"] == "summary" 或
// - ExtraParams["memory_summary_enabled"] 显式为 true
// 实现原则:
// - 不阻塞主请求链路,在后台 goroutine 中执行
// - 任何错误都应静默失败,避免影响正常对话
func (r *Registry) maybeSummarizeSession(ctx context.Context, tenantID, agentID string, input *AgentInput) {
	if input == nil || input.Context == nil || input.Context.SessionID == nil {
		return
	}

	sessionID := *input.Context.SessionID

	enabled := false
	triggerMessages := 30   // 默认超过 30 条历史后开始生成摘要
	summaryMaxTokens := 512 // 摘要最大长度
	updateMinDelta := 6     // 距离上次摘要至少新增 6 条消息才更新

	// 从 ExtraParams 读取控制参数
	if input.ExtraParams != nil {
		// memory_mode=summary 时默认开启摘要
		if v, ok := input.ExtraParams["memory_mode"]; ok {
			if s, ok2 := v.(string); ok2 && s == memoryModeSummary {
				enabled = true
			}
		}

		if v, ok := input.ExtraParams["memory_summary_enabled"]; ok {
			if b, ok2 := convertToBool(v); ok2 {
				enabled = b
			}
		}

		if v, ok := input.ExtraParams["summary_trigger_messages"]; ok {
			if n, ok2 := convertToInt(v); ok2 && n > 0 {
				triggerMessages = n
			}
		}

		if v, ok := input.ExtraParams["summary_max_tokens"]; ok {
			if n, ok2 := convertToInt(v); ok2 && n > 0 {
				summaryMaxTokens = n
			}
		}
	}

	if !enabled {
		return
	}

	// 在后台执行摘要,避免阻塞主链路
	go func(parentCtx context.Context) {
		// 使用派生超时上下文,避免长时间占用资源
		ctx2, cancel := context.WithTimeout(parentCtx, 10*time.Second)
		defer cancel()

		// 获取完整历史
		history, err := r.contextManager.GetHistory(ctx, sessionID, 0, 0, "")
		if err != nil {
			return
		}
		if len(history) < triggerMessages {
			return
		}

		// 读取上次摘要覆盖的历史条数,减少无意义重算
		var lastCount int
		if v, err2 := r.contextManager.GetData(ctx, sessionID, memorySummaryMessageCountKey); err2 == nil {
			if n, ok := convertToInt(v); ok {
				lastCount = n
			}
		}
		if lastCount > 0 && len(history)-lastCount < updateMinDelta {
			return
		}

		// 将历史消息拼接为文本
		var builder strings.Builder
		for _, msg := range history {
			builder.WriteString("[")
			builder.WriteString(msg.Role)
			builder.WriteString("] ")
			builder.WriteString(msg.Content)
			builder.WriteString("\n")
		}
		conversationText := builder.String()

		// 加载 Agent 配置,确定摘要使用的模型
		var agentConfig agentpkg.AgentConfig
		if err := r.db.WithContext(ctx2).
			Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", agentID, tenantID).
			First(&agentConfig).Error; err != nil {
			return
		}

		summaryModelID := agentConfig.ModelID
		if agentConfig.ExtraConfig != nil {
			if v, ok := agentConfig.ExtraConfig["summary_model_id"]; ok {
				if s, ok2 := v.(string); ok2 && s != "" {
					summaryModelID = s
				}
			}
		}

		modelClient, err := r.clientProvider.GetClient(ctx2, tenantID, summaryModelID)
		if err != nil {
			return
		}

		// 调用模型生成摘要
		req := &ai.ChatCompletionRequest{
			Messages: []ai.Message{
				{
					Role:    "system",
					Content: "你是一个对话总结助手,请阅读以下多轮人机对话,用简明的中文总结对话中的关键信息、用户目标、已经给出的结论以及未解决的问题。请输出结构化的摘要,便于后续继续对话时快速恢复上下文。",
				},
				{
					Role:    "user",
					Content: conversationText,
				},
			},
			Temperature: 0.1,
			MaxTokens:   summaryMaxTokens,
		}

		resp, err := modelClient.ChatCompletion(ctx2, req)
		if err != nil {
			return
		}

		summary := strings.TrimSpace(resp.Content)
		if summary == "" {
			return
		}

		// 将摘要及其覆盖的历史条数写入会话数据
		_ = r.contextManager.SetData(ctx, sessionID, memorySummaryKey, summary)
		_ = r.contextManager.SetData(ctx, sessionID, memorySummaryMessageCountKey, len(history))
	}(ctx)
}

// convertToInt 将任意类型尽量转换为 int, 主要用于解析 JSON 反序列化后的数值类型
func convertToInt(v any) (int, bool) {
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

// convertToBool 将任意类型尽量转换为 bool, 支持 bool/数字/字符串
func convertToBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		s := strings.TrimSpace(strings.ToLower(b))
		if s == "true" || s == "1" || s == "yes" || s == "y" {
			return true, true
		}
		if s == "false" || s == "0" || s == "no" || s == "n" {
			return false, true
		}
		return false, false
	case int, int32, int64, float32, float64:
		if n, ok := convertToInt(v); ok {
			return n != 0, true
		}
		return false, false
	default:
		return false, false
	}
}
