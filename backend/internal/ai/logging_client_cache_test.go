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
