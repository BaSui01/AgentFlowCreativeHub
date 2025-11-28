package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestDefaultWarmupConfig 测试默认配置
func TestDefaultWarmupConfig(t *testing.T) {
	config := DefaultWarmupConfig()
	
	if !config.Enabled {
		t.Error("预期预热默认启用")
	}
	
	if config.Concurrency != 5 {
		t.Errorf("预期并发数为 5，实际为 %d", config.Concurrency)
	}
	
	if config.Timeout != 30*time.Second {
		t.Errorf("预期超时为 30秒，实际为 %v", config.Timeout)
	}
	
	if config.RetryAttempts != 3 {
		t.Errorf("预期重试次数为 3，实际为 %d", config.RetryAttempts)
	}
}

// TestNewCacheWarmer 测试创建预热器
func TestNewCacheWarmer(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	
	// 测试使用默认配置
	warmer := NewCacheWarmer(nil, storage, logger)
	if warmer == nil {
		t.Fatal("创建预热器失败")
	}
	
	if warmer.config == nil {
		t.Error("配置不应为空")
	}
	
	// 测试使用自定义配置
	customConfig := &WarmupConfig{
		Enabled:       true,
		Concurrency:   10,
		Timeout:       5 * time.Second,
		RetryAttempts: 2,
		RetryDelay:    500 * time.Millisecond,
	}
	
	warmer2 := NewCacheWarmer(customConfig, storage, logger)
	if warmer2.config.Concurrency != 10 {
		t.Errorf("预期并发数为 10，实际为 %d", warmer2.config.Concurrency)
	}
}

// TestRegister 测试注册预热项
func TestRegister(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	warmer := NewCacheWarmer(nil, storage, logger)
	
	item := WarmupItem{
		Key: "test-key",
		Loader: func(ctx context.Context) (any, error) {
			return "test-value", nil
		},
		TTL:      time.Hour,
		Priority: 10,
	}
	
	warmer.Register(item)
	items := warmer.GetItems()
	
	if len(items) != 1 {
		t.Errorf("预期注册 1 个预热项，实际为 %d", len(items))
	}
	
	if items[0].Key != "test-key" {
		t.Errorf("预期键为 test-key，实际为 %s", items[0].Key)
	}
}

// TestRegisterBatch 测试批量注册
func TestRegisterBatch(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	warmer := NewCacheWarmer(nil, storage, logger)
	
	items := []WarmupItem{
		{
			Key: "key1",
			Loader: func(ctx context.Context) (any, error) {
				return "value1", nil
			},
			TTL:      time.Hour,
			Priority: 5,
		},
		{
			Key: "key2",
			Loader: func(ctx context.Context) (any, error) {
				return "value2", nil
			},
			TTL:      time.Hour,
			Priority: 3,
		},
	}
	
	warmer.RegisterBatch(items)
	registeredItems := warmer.GetItems()
	
	if len(registeredItems) != 2 {
		t.Errorf("预期注册 2 个预热项，实际为 %d", len(registeredItems))
	}
}

// TestWarmupDisabled 测试禁用预热
func TestWarmupDisabled(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	
	config := &WarmupConfig{
		Enabled: false,
	}
	
	warmer := NewCacheWarmer(config, storage, logger)
	
	warmer.Register(WarmupItem{
		Key: "test",
		Loader: func(ctx context.Context) (any, error) {
			return "value", nil
		},
		TTL: time.Hour,
	})
	
	ctx := context.Background()
	results := warmer.Warmup(ctx)
	
	if results != nil {
		t.Error("禁用预热应该返回 nil")
	}
}

// TestWarmupSuccess 测试成功预热
func TestWarmupSuccess(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	warmer := NewCacheWarmer(nil, storage, logger)
	
	item := WarmupItem{
		Key: "test-key",
		Loader: func(ctx context.Context) (any, error) {
			return "test-value", nil
		},
		TTL:      time.Hour,
		Priority: 10,
	}
	
	warmer.Register(item)
	
	ctx := context.Background()
	results := warmer.Warmup(ctx)
	
	if len(results) != 1 {
		t.Fatalf("预期 1 个结果，实际为 %d", len(results))
	}
	
	if !results[0].Success {
		t.Errorf("预期预热成功，实际失败: %v", results[0].Error)
	}
	
	if results[0].Key != "test-key" {
		t.Errorf("预期键为 test-key，实际为 %s", results[0].Key)
	}
	
	// 验证数据已写入缓存
	value, ok, err := storage.Get(ctx, "test-key")
	if err != nil {
		t.Errorf("获取缓存失败: %v", err)
	}
	if !ok {
		t.Error("缓存中应该存在该键")
	}
	if value != "test-value" {
		t.Errorf("预期值为 test-value，实际为 %v", value)
	}
}

// TestWarmupWithRetry 测试重试机制
func TestWarmupWithRetry(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	
	config := &WarmupConfig{
		Enabled:       true,
		Concurrency:   5,
		Timeout:       5 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    100 * time.Millisecond,
	}
	
	warmer := NewCacheWarmer(config, storage, logger)
	
	attempts := 0
	item := WarmupItem{
		Key: "retry-key",
		Loader: func(ctx context.Context) (any, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("临时错误")
			}
			return "success", nil
		},
		TTL:      time.Hour,
		Priority: 10,
	}
	
	warmer.Register(item)
	
	ctx := context.Background()
	results := warmer.Warmup(ctx)
	
	if len(results) != 1 {
		t.Fatalf("预期 1 个结果，实际为 %d", len(results))
	}
	
	if !results[0].Success {
		t.Errorf("预期重试后成功，实际失败: %v", results[0].Error)
	}
	
	if results[0].Retries < 2 {
		t.Errorf("预期至少重试 2 次，实际重试 %d 次", results[0].Retries)
	}
}

// TestWarmupTimeout 测试超时
func TestWarmupTimeout(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	
	config := &WarmupConfig{
		Enabled:       true,
		Concurrency:   5,
		Timeout:       100 * time.Millisecond, // 很短的超时
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}
	
	warmer := NewCacheWarmer(config, storage, logger)
	
	item := WarmupItem{
		Key: "timeout-key",
		Loader: func(ctx context.Context) (any, error) {
			select {
			case <-time.After(1 * time.Second): // 模拟慢操作
				return "value", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		TTL:      time.Hour,
		Priority: 10,
	}
	
	warmer.Register(item)
	
	ctx := context.Background()
	results := warmer.Warmup(ctx)
	
	if len(results) != 1 {
		t.Fatalf("预期 1 个结果，实际为 %d", len(results))
	}
	
	if results[0].Success {
		t.Error("预期超时导致失败，但实际成功")
	}
}

// TestWarmupConcurrency 测试并发预热
func TestWarmupConcurrency(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	
	config := &WarmupConfig{
		Enabled:       true,
		Concurrency:   3,
		Timeout:       5 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    100 * time.Millisecond,
	}
	
	warmer := NewCacheWarmer(config, storage, logger)
	
	// 注册多个预热项
	for i := 0; i < 10; i++ {
		idx := i
		item := WarmupItem{
			Key: string(rune('a' + idx)),
			Loader: func(ctx context.Context) (any, error) {
				time.Sleep(100 * time.Millisecond) // 模拟加载时间
				return "value", nil
			},
			TTL:      time.Hour,
			Priority: i,
		}
		warmer.Register(item)
	}
	
	ctx := context.Background()
	startTime := time.Now()
	results := warmer.Warmup(ctx)
	duration := time.Since(startTime)
	
	if len(results) != 10 {
		t.Errorf("预期 10 个结果，实际为 %d", len(results))
	}
	
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	
	if successCount != 10 {
		t.Errorf("预期 10 个成功，实际 %d 个", successCount)
	}
	
	// 由于并发为 3，10个任务应该分批执行
	// 总时间应该大约是 (10/3)*100ms = 334ms
	// 但小于 10*100ms = 1000ms（串行执行）
	if duration > 1*time.Second {
		t.Errorf("并发执行时间过长: %v", duration)
	}
	
	t.Logf("并发预热耗时: %v", duration)
}

// TestWarmupClear 测试清除预热项
func TestWarmupClear(t *testing.T) {
	logger := zap.NewNop()
	storage := NewMemoryWarmupStorage()
	warmer := NewCacheWarmer(nil, storage, logger)
	
	item := WarmupItem{
		Key: "test-key",
		Loader: func(ctx context.Context) (any, error) {
			return "value", nil
		},
		TTL:      time.Hour,
		Priority: 10,
	}
	
	warmer.Register(item)
	
	items := warmer.GetItems()
	if len(items) != 1 {
		t.Errorf("预期 1 个预热项，实际为 %d", len(items))
	}
	
	warmer.Clear()
	
	items = warmer.GetItems()
	if len(items) != 0 {
		t.Errorf("清除后预期 0 个预热项，实际为 %d", len(items))
	}
}

// TestMemoryWarmupStorage 测试内存存储
func TestMemoryWarmupStorage(t *testing.T) {
	storage := NewMemoryWarmupStorage()
	ctx := context.Background()
	
	// 测试 Set
	err := storage.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Errorf("Set 失败: %v", err)
	}
	
	// 测试 Get - 存在的键
	value, ok, err := storage.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get 失败: %v", err)
	}
	if !ok {
		t.Error("预期键存在")
	}
	if value != "value1" {
		t.Errorf("预期值为 value1，实际为 %v", value)
	}
	
	// 测试 Get - 不存在的键
	_, ok, err = storage.Get(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Get 失败: %v", err)
	}
	if ok {
		t.Error("预期键不存在")
	}
}
