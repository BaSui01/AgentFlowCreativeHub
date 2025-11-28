package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"backend/internal/cache"
	modelspkg "backend/internal/models"
)

// LoggingClient 带日志记录的客户端包装器
type LoggingClient struct {
	client    ModelClient
	logger    ModelCallLogger
	tenantID  string
	modelID   string
	model     *modelspkg.Model
	diskCache *cache.DiskCache // L3硬盘缓存
	
	// 缓存统计
	cacheHits   int64
	cacheMisses int64
	statsMu     sync.RWMutex
}

// NewLoggingClient 创建带日志记录的客户端
func NewLoggingClient(
	client ModelClient,
	logger ModelCallLogger,
	tenantID, modelID string,
	model *modelspkg.Model,
	diskCache *cache.DiskCache,
) *LoggingClient {
	return &LoggingClient{
		client:    client,
		logger:    logger,
		tenantID:  tenantID,
		modelID:   modelID,
		model:     model,
		diskCache: diskCache,
	}
}

// ChatCompletion 对话补全（带日志记录+硬盘缓存）
func (c *LoggingClient) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	start := time.Now()

	// L3: 硬盘缓存查询（仅当温度≤0.3时才使用缓存）
	if c.diskCache != nil && req.Temperature <= 0.3 {
		cacheKey := c.generateCacheKey(req)
		
		// 尝试从硬盘缓存获取
		if entry, err := c.diskCache.Get(ctx, cacheKey); err == nil && entry != nil {
			// 缓存命中！从缓存条目反序列化响应
			var cachedResp ChatCompletionResponse
			if err := json.Unmarshal([]byte(entry.Response), &cachedResp); err == nil {
				// 更新缓存命中统计
				c.statsMu.Lock()
				c.cacheHits++
				c.statsMu.Unlock()
				
				// 记录缓存命中日志
				latency := time.Since(start).Milliseconds()
				c.logCall(ctx, &cachedResp, latency, nil)
				return &cachedResp, nil
			}
		}
		
		// 缓存未命中，更新统计
		c.statsMu.Lock()
		c.cacheMisses++
		c.statsMu.Unlock()
	}

	// 缓存未命中或不可用，调用底层客户端
	resp, err := c.client.ChatCompletion(ctx, req)

	// 写入硬盘缓存（仅当成功且温度≤0.3）
	if err == nil && resp != nil && c.diskCache != nil && req.Temperature <= 0.3 {
		cacheKey := c.generateCacheKey(req)
		
		// 序列化响应到JSON
		if respJSON, err := json.Marshal(resp); err == nil {
			// 构建缓存条目
			entry := &cache.CacheEntry{
				CacheKey:   cacheKey,
				Model:      c.modelID,
				PromptHash: cacheKey, // 使用相同的键作为hash
				Response:   string(respJSON),
				TokensUsed: resp.Usage.TotalTokens,
				CostUSD:    c.calculateCost(resp.Usage.PromptTokens, resp.Usage.CompletionTokens),
			}
			
			// 异步写入缓存，不阻塞主流程
			go func() {
				_ = c.diskCache.Set(context.Background(), entry)
			}()
		}
	}

	// 记录日志
	latency := time.Since(start).Milliseconds()
	c.logCall(ctx, resp, latency, err)

	return resp, err
}

// ChatCompletionStream 对话补全（流式，带日志记录）
func (c *LoggingClient) ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamChunk, <-chan error) {
	start := time.Now()

	// 调用底层客户端
	chunkChan, errChan := c.client.ChatCompletionStream(ctx, req)

	// 包装 channel 以记录日志
	wrappedChunkChan := make(chan StreamChunk, 10)
	wrappedErrChan := make(chan error, 1)

	go func() {
		defer close(wrappedChunkChan)
		defer close(wrappedErrChan)

		var totalTokens int
		var promptTokens int
		var completionTokens int
		var err error

		// 转发所有响应块
		for chunk := range chunkChan {
			wrappedChunkChan <- chunk
			// 累计 Token（简化实现，实际需要更精确的计算）
			if !chunk.Done {
				completionTokens += len(chunk.Content) / 4 // 粗略估算
			}
		}

		// 检查错误
		select {
		case err = <-errChan:
			wrappedErrChan <- err
		default:
		}

		// 记录日志
		latency := time.Since(start).Milliseconds()
		totalTokens = promptTokens + completionTokens

		c.logStreamCall(ctx, promptTokens, completionTokens, totalTokens, latency, err)
	}()

	return wrappedChunkChan, wrappedErrChan
}

// Embedding 文本向量化（带日志记录）
func (c *LoggingClient) Embedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	start := time.Now()

	// 调用底层客户端
	resp, err := c.client.Embedding(ctx, req)

	// 记录日志
	latency := time.Since(start).Milliseconds()
	if resp != nil {
		c.logEmbeddingCall(ctx, &resp.Usage, latency, err)
	}

	return resp, err
}

// Name 返回客户端名称
func (c *LoggingClient) Name() string {
	return c.client.Name()
}

// Close 关闭客户端
func (c *LoggingClient) Close() error {
	return c.client.Close()
}

// logCall 记录普通调用日志
func (c *LoggingClient) logCall(ctx context.Context, resp *ChatCompletionResponse, latencyMs int64, err error) {
	if c.logger == nil {
		return
	}

	var usage Usage
	if resp != nil {
		usage = resp.Usage
	}

	// 计算成本
	cost := c.calculateCost(usage.PromptTokens, usage.CompletionTokens)

	// 从上下文提取信息
	userID := getContextString(ctx, "user_id")
	workflowID := getContextStringPtr(ctx, "workflow_id")
	traceID := getContextStringPtr(ctx, "trace_id")

	log := &ModelCallLog{
		TenantID:         c.tenantID,
		UserID:           userID,
		ModelID:          c.modelID,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		TotalCost:        cost,
		LatencyMs:        latencyMs,
		WorkflowID:       workflowID,
		TraceID:          traceID,
	}
	c.enrichModelMetadata(log)

	// 异步记录日志（不阻塞主流程）
	go func() {
		_ = c.logger.Log(context.Background(), log)
	}()
}

// logStreamCall 记录流式调用日志
func (c *LoggingClient) logStreamCall(ctx context.Context, promptTokens, completionTokens, totalTokens int, latencyMs int64, err error) {
	if c.logger == nil {
		return
	}

	// 计算成本
	cost := c.calculateCost(promptTokens, completionTokens)

	// 从上下文提取信息
	userID := getContextString(ctx, "user_id")
	workflowID := getContextStringPtr(ctx, "workflow_id")
	traceID := getContextStringPtr(ctx, "trace_id")

	log := &ModelCallLog{
		TenantID:         c.tenantID,
		UserID:           userID,
		ModelID:          c.modelID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		TotalCost:        cost,
		LatencyMs:        latencyMs,
		WorkflowID:       workflowID,
		TraceID:          traceID,
	}
	c.enrichModelMetadata(log)

	// 异步记录日志
	go func() {
		_ = c.logger.Log(context.Background(), log)
	}()
}

// logEmbeddingCall 记录向量化调用日志
func (c *LoggingClient) logEmbeddingCall(ctx context.Context, usage *Usage, latencyMs int64, err error) {
	if c.logger == nil || usage == nil {
		return
	}

	// 计算成本
	cost := c.calculateCost(usage.PromptTokens, 0)

	// 从上下文提取信息
	userID := getContextString(ctx, "user_id")
	workflowID := getContextStringPtr(ctx, "workflow_id")
	traceID := getContextStringPtr(ctx, "trace_id")

	log := &ModelCallLog{
		TenantID:         c.tenantID,
		UserID:           userID,
		ModelID:          c.modelID,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: 0,
		TotalTokens:      usage.TotalTokens,
		TotalCost:        cost,
		LatencyMs:        latencyMs,
		WorkflowID:       workflowID,
		TraceID:          traceID,
	}
	c.enrichModelMetadata(log)

	// 异步记录日志
	go func() {
		_ = c.logger.Log(context.Background(), log)
	}()
}

// enrichModelMetadata 补全模型提供方与名称，便于日志聚合
func (c *LoggingClient) enrichModelMetadata(log *ModelCallLog) {
	if log == nil {
		return
	}
	if c.model != nil {
		if log.ModelProvider == "" {
			log.ModelProvider = c.model.Provider
		}
		if log.ModelName == "" {
			if c.model.ModelIdentifier != "" {
				log.ModelName = c.model.ModelIdentifier
			} else {
				log.ModelName = c.model.Name
			}
		}
	}
	if log.ModelProvider == "" {
		log.ModelProvider = log.ModelID
	}
	if log.ModelName == "" {
		log.ModelName = log.ModelID
	}
}

// calculateCost 计算成本
func (c *LoggingClient) calculateCost(promptTokens, completionTokens int) float64 {
	if c.model == nil {
		return 0
	}

	// 成本 = (输入 Token / 1000) * 输入单价 + (输出 Token / 1000) * 输出单价
	inputCost := float64(promptTokens) / 1000.0 * c.model.InputCostPer1K
	outputCost := float64(completionTokens) / 1000.0 * c.model.OutputCostPer1K

	return inputCost + outputCost
}

// getContextString 从上下文获取字符串值
func getContextString(ctx context.Context, key string) string {
	if val := ctx.Value(key); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getContextStringPtr 从上下文获取字符串指针
func getContextStringPtr(ctx context.Context, key string) *string {
	if val := ctx.Value(key); val != nil {
		if str, ok := val.(string); ok {
			return &str
		}
	}
	return nil
}

// generateCacheKey 生成缓存键
// 根据模型ID、消息内容、温度等参数生成唯一键
func (c *LoggingClient) generateCacheKey(req *ChatCompletionRequest) string {
	// 将所有消息内容序列化为JSON字符串
	messagesJSON, _ := json.Marshal(req.Messages)
	
	// 将所有影响输出的参数编码到prompt中
	promptWithParams := fmt.Sprintf("messages=%s;temp=%.2f;max_tokens=%d;top_p=%.2f",
		string(messagesJSON),
		req.Temperature,
		req.MaxTokens,
		req.TopP,
	)
	
	// 使用DiskCache的GenerateCacheKey生成键
	return cache.GenerateCacheKey(c.modelID, promptWithParams)
}

// GetCacheStats 获取缓存统计信息
func (c *LoggingClient) GetCacheStats() map[string]interface{} {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()
	
	totalRequests := c.cacheHits + c.cacheMisses
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(c.cacheHits) / float64(totalRequests) * 100
	}
	
	return map[string]interface{}{
		"cache_hits":        c.cacheHits,
		"cache_misses":      c.cacheMisses,
		"total_requests":    totalRequests,
		"hit_rate_percent":  hitRate,
		"cache_enabled":     c.diskCache != nil,
	}
}
