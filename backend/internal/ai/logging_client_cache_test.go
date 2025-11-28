package ai

import (
	"context"
	"os"
	"testing"
	"time"

	"backend/internal/cache"
	modelspkg "backend/internal/models"
)

// mockModelClient 模拟的AI客户端
type mockModelClient struct {
	callCount int // 记录实际API调用次数
}

func (m *mockModelClient) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	m.callCount++
	
	// 模拟API延迟
	time.Sleep(100 * time.Millisecond)
	
	return &ChatCompletionResponse{
		ID:      "mock-response-id",
		Model:   "gpt-3.5-turbo",
		Content: "This is a mock response for: " + req.Messages[0].Content,
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

func (m *mockModelClient) ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamChunk, <-chan error) {
	return nil, nil
}

func (m *mockModelClient) Embedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	return nil, nil
}

func (m *mockModelClient) Name() string {
	return "mock"
}

func (m *mockModelClient) Close() error {
	return nil
}

// TestLoggingClientCache 测试缓存功能
func TestLoggingClientCache(t *testing.T) {
	// 创建临时数据库文件
	tempDB := "./test_cache.db"
	defer os.Remove(tempDB)
	
	// 初始化硬盘缓存
	diskCache, err := cache.NewDiskCache(tempDB, 1*time.Hour, 1)
	if err != nil {
		t.Fatalf("创建DiskCache失败: %v", err)
	}
	defer diskCache.Close()
	
	// 创建mock客户端
	mockClient := &mockModelClient{}
	
	// 创建LoggingClient（带缓存）
	model := &modelspkg.Model{
		ID:               "test-model",
		Name:             "Test Model",
		Provider:         "openai",
		ModelIdentifier:  "gpt-3.5-turbo",
		InputCostPer1K:   0.001,
		OutputCostPer1K:  0.002,
	}
	
	loggingClient := NewLoggingClient(mockClient, nil, "tenant-1", "model-1", model, diskCache)
	
	// 构建测试请求（低温度，可缓存）
	req := &ChatCompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello, world!"},
		},
		Temperature: 0.1, // 低温度，启用缓存
		MaxTokens:   100,
		TopP:        0.9,
	}
	
	ctx := context.Background()
	
	// 第一次调用 - 应该命中API
	t.Log("第一次调用（应该调用实际API）...")
	start1 := time.Now()
	resp1, err := loggingClient.ChatCompletion(ctx, req)
	latency1 := time.Since(start1).Milliseconds()
	
	if err != nil {
		t.Fatalf("第一次调用失败: %v", err)
	}
	if resp1 == nil {
		t.Fatal("响应为空")
	}
	if mockClient.callCount != 1 {
		t.Fatalf("期望API调用次数为1，实际为%d", mockClient.callCount)
	}
	t.Logf("✅ 第一次调用成功 - 延迟: %dms, API调用次数: %d", latency1, mockClient.callCount)
	
	// 等待缓存写入完成（异步写入）
	time.Sleep(200 * time.Millisecond)
	
	// 第二次调用 - 应该命中缓存
	t.Log("第二次调用（应该命中缓存）...")
	start2 := time.Now()
	resp2, err := loggingClient.ChatCompletion(ctx, req)
	latency2 := time.Since(start2).Milliseconds()
	
	if err != nil {
		t.Fatalf("第二次调用失败: %v", err)
	}
	if resp2 == nil {
		t.Fatal("缓存响应为空")
	}
	if mockClient.callCount != 1 {
		t.Fatalf("期望API调用次数仍为1（命中缓存），实际为%d", mockClient.callCount)
	}
	
	// 验证缓存响应与原始响应相同
	if resp2.Content != resp1.Content {
		t.Fatalf("缓存响应内容不一致: 期望 %s, 实际 %s", resp1.Content, resp2.Content)
	}
	
	// 缓存命中应该显著快于第一次调用
	if latency2 >= latency1 {
		t.Logf("⚠️  缓存延迟(%dms)未明显低于API延迟(%dms) - 可能缓存查询较慢", latency2, latency1)
	} else {
		t.Logf("✅ 第二次调用命中缓存 - 延迟: %dms (提速%.1fx), API调用次数: %d",
			latency2, float64(latency1)/float64(latency2), mockClient.callCount)
	}
	
	// 第三次调用不同内容 - 应该未命中缓存
	t.Log("第三次调用（不同内容，应该未命中缓存）...")
	req3 := &ChatCompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Different message"},
		},
		Temperature: 0.1,
		MaxTokens:   100,
		TopP:        0.9,
	}
	
	resp3, err := loggingClient.ChatCompletion(ctx, req3)
	if err != nil {
		t.Fatalf("第三次调用失败: %v", err)
	}
	if resp3 == nil {
		t.Fatal("第三次响应为空")
	}
	if mockClient.callCount != 2 {
		t.Fatalf("期望API调用次数为2，实际为%d", mockClient.callCount)
	}
	t.Logf("✅ 第三次调用成功（不同内容） - API调用次数: %d", mockClient.callCount)
	
	// 测试高温度请求（不应使用缓存）
	t.Log("第四次调用（高温度，不应使用缓存）...")
	req4 := &ChatCompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello, world!"}, // 相同内容
		},
		Temperature: 0.8, // 高温度，不启用缓存
		MaxTokens:   100,
		TopP:        0.9,
	}
	
	resp4, err := loggingClient.ChatCompletion(ctx, req4)
	if err != nil {
		t.Fatalf("第四次调用失败: %v", err)
	}
	if resp4 == nil {
		t.Fatal("第四次响应为空")
	}
	if mockClient.callCount != 3 {
		t.Fatalf("期望API调用次数为3（高温度不缓存），实际为%d", mockClient.callCount)
	}
	t.Logf("✅ 第四次调用成功（高温度） - API调用次数: %d", mockClient.callCount)
	
	t.Log("🎉 所有缓存测试通过！")
}

// TestLoggingClient_GetCacheStats 测试缓存统计功能
func TestLoggingClient_GetCacheStats(t *testing.T) {
	// 创建临时数据库文件
	tempDB := ":memory:" // 使用内存数据库，无需清理
	
	// 初始化硬盘缓存
	diskCache, err := cache.NewDiskCache(tempDB, 1*time.Hour, 1)
	if err != nil {
		t.Fatalf("创建DiskCache失败: %v", err)
	}
	defer diskCache.Close()
	
	// 创建mock客户端
	mockClient := &mockModelClient{}
	
	// 创建LoggingClient（带缓存）
	model := &modelspkg.Model{
		ID:              "test-model",
		Provider:        "openai",
		ModelIdentifier: "gpt-3.5-turbo",
		InputCostPer1K:  0.001,
		OutputCostPer1K: 0.002,
	}
	
	loggingClient := NewLoggingClient(mockClient, nil, "tenant-1", "model-1", model, diskCache)
	
	ctx := context.Background()
	
	// 测试初始状态
	t.Log("测试初始状态...")
	stats := loggingClient.GetCacheStats()
	if stats["cache_hits"].(int64) != 0 {
		t.Errorf("初始cache_hits应为0，实际: %v", stats["cache_hits"])
	}
	if stats["cache_misses"].(int64) != 0 {
		t.Errorf("初始cache_misses应为0，实际: %v", stats["cache_misses"])
	}
	if stats["total_requests"].(int64) != 0 {
		t.Errorf("初始total_requests应为0，实际: %v", stats["total_requests"])
	}
	if stats["hit_rate_percent"].(float64) != 0.0 {
		t.Errorf("初始hit_rate_percent应为0.0，实际: %v", stats["hit_rate_percent"])
	}
	if stats["cache_enabled"].(bool) != true {
		t.Errorf("cache_enabled应为true，实际: %v", stats["cache_enabled"])
	}
	t.Logf("✅ 初始状态正确: %+v", stats)
	
	// 构建测试请求（低温度，可缓存）
	req := &ChatCompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello, world!"},
		},
		Temperature: 0.1,
		MaxTokens:   100,
		TopP:        0.9,
	}
	
	// 第一次调用 - 缓存未命中
	t.Log("第一次调用（应该缓存未命中）...")
	_, err = loggingClient.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("第一次调用失败: %v", err)
	}
	
	// 等待缓存写入
	time.Sleep(200 * time.Millisecond)
	
	stats = loggingClient.GetCacheStats()
	if stats["cache_hits"].(int64) != 0 {
		t.Errorf("第一次后cache_hits应为0，实际: %v", stats["cache_hits"])
	}
	if stats["cache_misses"].(int64) != 1 {
		t.Errorf("第一次后cache_misses应为1，实际: %v", stats["cache_misses"])
	}
	if stats["total_requests"].(int64) != 1 {
		t.Errorf("第一次后total_requests应为1，实际: %v", stats["total_requests"])
	}
	if stats["hit_rate_percent"].(float64) != 0.0 {
		t.Errorf("第一次后hit_rate_percent应为0.0，实际: %v", stats["hit_rate_percent"])
	}
	t.Logf("✅ 第一次调用统计正确: %+v", stats)
	
	// 第二次调用 - 缓存命中
	t.Log("第二次调用（应该缓存命中）...")
	_, err = loggingClient.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("第二次调用失败: %v", err)
	}
	
	stats = loggingClient.GetCacheStats()
	if stats["cache_hits"].(int64) != 1 {
		t.Errorf("第二次后cache_hits应为1，实际: %v", stats["cache_hits"])
	}
	if stats["cache_misses"].(int64) != 1 {
		t.Errorf("第二次后cache_misses应为1，实际: %v", stats["cache_misses"])
	}
	if stats["total_requests"].(int64) != 2 {
		t.Errorf("第二次后total_requests应为2，实际: %v", stats["total_requests"])
	}
	if stats["hit_rate_percent"].(float64) != 50.0 {
		t.Errorf("第二次后hit_rate_percent应为50.0，实际: %v", stats["hit_rate_percent"])
	}
	t.Logf("✅ 第二次调用统计正确: %+v", stats)
	
	// 第三次调用 - 再次缓存命中
	t.Log("第三次调用（应该再次缓存命中）...")
	_, err = loggingClient.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("第三次调用失败: %v", err)
	}
	
	stats = loggingClient.GetCacheStats()
	if stats["cache_hits"].(int64) != 2 {
		t.Errorf("第三次后cache_hits应为2，实际: %v", stats["cache_hits"])
	}
	if stats["cache_misses"].(int64) != 1 {
		t.Errorf("第三次后cache_misses应为1，实际: %v", stats["cache_misses"])
	}
	if stats["total_requests"].(int64) != 3 {
		t.Errorf("第三次后total_requests应为3，实际: %v", stats["total_requests"])
	}
	// 允许浮点数误差
	hitRate := stats["hit_rate_percent"].(float64)
	expectedHitRate := 66.66666666666667
	if hitRate < expectedHitRate-0.01 || hitRate > expectedHitRate+0.01 {
		t.Errorf("第三次后hit_rate_percent应约为66.67，实际: %v", hitRate)
	}
	t.Logf("✅ 第三次调用统计正确: %+v", stats)
	
	// 第四次调用 - 不同内容，缓存未命中
	t.Log("第四次调用（不同内容，应该缓存未命中）...")
	req2 := &ChatCompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Different message"},
		},
		Temperature: 0.1,
		MaxTokens:   100,
		TopP:        0.9,
	}
	
	_, err = loggingClient.ChatCompletion(ctx, req2)
	if err != nil {
		t.Fatalf("第四次调用失败: %v", err)
	}
	
	stats = loggingClient.GetCacheStats()
	if stats["cache_hits"].(int64) != 2 {
		t.Errorf("第四次后cache_hits应为2，实际: %v", stats["cache_hits"])
	}
	if stats["cache_misses"].(int64) != 2 {
		t.Errorf("第四次后cache_misses应为2，实际: %v", stats["cache_misses"])
	}
	if stats["total_requests"].(int64) != 4 {
		t.Errorf("第四次后total_requests应为4，实际: %v", stats["total_requests"])
	}
	if stats["hit_rate_percent"].(float64) != 50.0 {
		t.Errorf("第四次后hit_rate_percent应为50.0，实际: %v", stats["hit_rate_percent"])
	}
	t.Logf("✅ 第四次调用统计正确: %+v", stats)
	
	t.Log("🎉 所有缓存统计测试通过！")
}

// TestLoggingClient_GetCacheStats_NoCache 测试无缓存时的统计
func TestLoggingClient_GetCacheStats_NoCache(t *testing.T) {
	mockClient := &mockModelClient{}
	
	// 创建LoggingClient（不启用缓存）
	loggingClient := NewLoggingClient(mockClient, nil, "tenant-1", "model-1", nil, nil)
	
	stats := loggingClient.GetCacheStats()
	if stats["cache_enabled"].(bool) != false {
		t.Errorf("无缓存时cache_enabled应为false，实际: %v", stats["cache_enabled"])
	}
	if stats["total_requests"].(int64) != 0 {
		t.Errorf("无缓存时total_requests应为0，实际: %v", stats["total_requests"])
	}
	
	t.Logf("✅ 无缓存统计正确: %+v", stats)
}
