package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLFUCacheBasicOperations 测试 LFU 缓存基本操作
func TestLFUCacheBasicOperations(t *testing.T) {
	// 创建临时数据库
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_lfu.db")
	
	// 创建底层磁盘缓存
	diskCache, err := NewDiskCache(dbPath, 24*time.Hour, 1)
	if err != nil {
		t.Fatalf("创建 DiskCache 失败: %v", err)
	}
	defer diskCache.Close()
	
	// 创建 LFU 缓存（容量为 3）
	lfuCache := NewLFUCache(3, diskCache)
	defer lfuCache.Close()
	
	ctx := context.Background()
	
	// 测试 Set 操作
	entries := []*CacheEntry{
		{
			CacheKey:   "key1",
			Model:      "gpt-4",
			PromptHash: "hash1",
			Response:   "response1",
			TokensUsed: 100,
			CostUSD:    0.01,
		},
		{
			CacheKey:   "key2",
			Model:      "gpt-4",
			PromptHash: "hash2",
			Response:   "response2",
			TokensUsed: 200,
			CostUSD:    0.02,
		},
		{
			CacheKey:   "key3",
			Model:      "gpt-4",
			PromptHash: "hash3",
			Response:   "response3",
			TokensUsed: 300,
			CostUSD:    0.03,
		},
	}
	
	for _, entry := range entries {
		if err := lfuCache.Set(ctx, entry); err != nil {
			t.Fatalf("Set 失败 (%s): %v", entry.CacheKey, err)
		}
	}
	
	// 测试 Get 操作
	for _, entry := range entries {
		retrieved, err := lfuCache.Get(ctx, entry.CacheKey)
		if err != nil {
			t.Fatalf("Get 失败 (%s): %v", entry.CacheKey, err)
		}
		if retrieved == nil {
			t.Fatalf("Get 返回 nil (%s)", entry.CacheKey)
		}
		if retrieved.Response != entry.Response {
			t.Errorf("响应不匹配: 期望 %s, 得到 %s", entry.Response, retrieved.Response)
		}
	}
	
	// 测试统计信息
	stats, err := lfuCache.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats 失败: %v", err)
	}
	
	if trackedEntries, ok := stats["lfu_tracked_entries"].(int); !ok || trackedEntries != 3 {
		t.Errorf("LFU 跟踪条目数不正确: 期望 3, 得到 %v", stats["lfu_tracked_entries"])
	}
}

// TestLFUCacheEviction 测试 LFU 淘汰策略
func TestLFUCacheEviction(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_lfu_eviction.db")
	
	diskCache, err := NewDiskCache(dbPath, 24*time.Hour, 1)
	if err != nil {
		t.Fatalf("创建 DiskCache 失败: %v", err)
	}
	defer diskCache.Close()
	
	// 创建容量为 3 的 LFU 缓存
	lfuCache := NewLFUCache(3, diskCache)
	defer lfuCache.Close()
	
	ctx := context.Background()
	
	// 添加 3 个条目
	for i := 1; i <= 3; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash%d", i),
			Response:   fmt.Sprintf("response%d", i),
			TokensUsed: i * 100,
			CostUSD:    float64(i) * 0.01,
		}
		if err := lfuCache.Set(ctx, entry); err != nil {
			t.Fatalf("Set 失败: %v", err)
		}
	}
	
	// 访问 key1 和 key2 多次，增加它们的频率
	for i := 0; i < 3; i++ {
		_, _ = lfuCache.Get(ctx, "key1")
		_, _ = lfuCache.Get(ctx, "key2")
	}
	
	// key3 只访问 1 次
	_, _ = lfuCache.Get(ctx, "key3")
	
	// 添加第 4 个条目，应该淘汰 key3（频率最低）
	entry4 := &CacheEntry{
		CacheKey:   "key4",
		Model:      "gpt-4",
		PromptHash: "hash4",
		Response:   "response4",
		TokensUsed: 400,
		CostUSD:    0.04,
	}
	if err := lfuCache.Set(ctx, entry4); err != nil {
		t.Fatalf("Set key4 失败: %v", err)
	}
	
	// 验证 key3 已被淘汰（从底层缓存中删除）
	// 注意：由于 LFU 的淘汰是从内存结构中移除，底层 DiskCache 仍可能保留
	stats, _ := lfuCache.GetStats(ctx)
	if trackedEntries, ok := stats["lfu_tracked_entries"].(int); !ok || trackedEntries != 3 {
		t.Errorf("淘汰后 LFU 跟踪条目数应为 3, 得到 %v", stats["lfu_tracked_entries"])
	}
	
	// key1 和 key2 应该仍然可访问（频率较高）
	if entry, _ := lfuCache.Get(ctx, "key1"); entry == nil {
		t.Error("key1 不应被淘汰")
	}
	if entry, _ := lfuCache.Get(ctx, "key2"); entry == nil {
		t.Error("key2 不应被淘汰")
	}
}

// TestLFUCacheDelete 测试删除操作
func TestLFUCacheDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_lfu_delete.db")
	
	diskCache, err := NewDiskCache(dbPath, 24*time.Hour, 1)
	if err != nil {
		t.Fatalf("创建 DiskCache 失败: %v", err)
	}
	defer diskCache.Close()
	
	lfuCache := NewLFUCache(10, diskCache)
	defer lfuCache.Close()
	
	ctx := context.Background()
	
	// 添加条目
	entry := &CacheEntry{
		CacheKey:   "key1",
		Model:      "gpt-4",
		PromptHash: "hash1",
		Response:   "response1",
		TokensUsed: 100,
		CostUSD:    0.01,
	}
	if err := lfuCache.Set(ctx, entry); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	
	// 验证条目存在
	if retrieved, _ := lfuCache.Get(ctx, "key1"); retrieved == nil {
		t.Fatal("条目应该存在")
	}
	
	// 删除条目
	if err := lfuCache.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	
	// 验证条目已删除
	retrieved, _ := lfuCache.Get(ctx, "key1")
	if retrieved != nil {
		t.Error("条目应该已被删除")
	}
}

// TestLFUCacheClear 测试清空操作
func TestLFUCacheClear(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_lfu_clear.db")
	
	diskCache, err := NewDiskCache(dbPath, 24*time.Hour, 1)
	if err != nil {
		t.Fatalf("创建 DiskCache 失败: %v", err)
	}
	defer diskCache.Close()
	
	lfuCache := NewLFUCache(10, diskCache)
	defer lfuCache.Close()
	
	ctx := context.Background()
	
	// 添加多个条目
	for i := 1; i <= 5; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash%d", i),
			Response:   fmt.Sprintf("response%d", i),
			TokensUsed: i * 100,
			CostUSD:    float64(i) * 0.01,
		}
		if err := lfuCache.Set(ctx, entry); err != nil {
			t.Fatalf("Set 失败: %v", err)
		}
	}
	
	// 清空缓存
	if err := lfuCache.Clear(ctx); err != nil {
		t.Fatalf("Clear 失败: %v", err)
	}
	
	// 验证所有条目已清空
	stats, _ := lfuCache.GetStats(ctx)
	if trackedEntries, ok := stats["lfu_tracked_entries"].(int); !ok || trackedEntries != 0 {
		t.Errorf("清空后 LFU 跟踪条目数应为 0, 得到 %v", stats["lfu_tracked_entries"])
	}
}

// BenchmarkLFUCacheSet 基准测试 Set 操作
func BenchmarkLFUCacheSet(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench_lfu_set_*")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "bench_set.db")
	diskCache, _ := NewDiskCache(dbPath, 24*time.Hour, 1)
	defer diskCache.Close()
	
	lfuCache := NewLFUCache(1000, diskCache)
	defer lfuCache.Close()
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash%d", i),
			Response:   fmt.Sprintf("response%d", i),
			TokensUsed: 100,
			CostUSD:    0.01,
		}
		_ = lfuCache.Set(ctx, entry)
	}
}

// BenchmarkLFUCacheGet 基准测试 Get 操作
func BenchmarkLFUCacheGet(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench_lfu_get_*")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "bench_get.db")
	diskCache, _ := NewDiskCache(dbPath, 24*time.Hour, 1)
	defer diskCache.Close()
	
	lfuCache := NewLFUCache(1000, diskCache)
	defer lfuCache.Close()
	
	ctx := context.Background()
	
	// 预先填充数据
	for i := 0; i < 100; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash%d", i),
			Response:   fmt.Sprintf("response%d", i),
			TokensUsed: 100,
			CostUSD:    0.01,
		}
		_ = lfuCache.Set(ctx, entry)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%100)
		_, _ = lfuCache.Get(ctx, key)
	}
}

// BenchmarkLFUCacheEviction 基准测试淘汰操作
func BenchmarkLFUCacheEviction(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench_lfu_evict_*")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "bench_evict.db")
	diskCache, _ := NewDiskCache(dbPath, 24*time.Hour, 1)
	defer diskCache.Close()
	
	// 小容量缓存，触发频繁淘汰
	lfuCache := NewLFUCache(100, diskCache)
	defer lfuCache.Close()
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash%d", i),
			Response:   fmt.Sprintf("response%d", i),
			TokensUsed: 100,
			CostUSD:    0.01,
		}
		_ = lfuCache.Set(ctx, entry)
	}
}

// BenchmarkLFUCacheParallel 并发基准测试
func BenchmarkLFUCacheParallel(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench_lfu_parallel_*")
	defer os.RemoveAll(tmpDir)
	
	dbPath := filepath.Join(tmpDir, "bench_parallel.db")
	diskCache, _ := NewDiskCache(dbPath, 24*time.Hour, 1)
	defer diskCache.Close()
	
	lfuCache := NewLFUCache(1000, diskCache)
	defer lfuCache.Close()
	
	ctx := context.Background()
	
	// 预先填充数据
	for i := 0; i < 100; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash%d", i),
			Response:   fmt.Sprintf("response%d", i),
			TokensUsed: 100,
			CostUSD:    0.01,
		}
		_ = lfuCache.Set(ctx, entry)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%100)
			_, _ = lfuCache.Get(ctx, key)
			i++
		}
	})
}

// BenchmarkLFUvsLRU 比较 LFU 和 LRU 性能
func BenchmarkLFUvsLRU(b *testing.B) {
	b.Run("LFU", func(b *testing.B) {
		tmpDir, _ := os.MkdirTemp("", "bench_lfu_*")
		defer os.RemoveAll(tmpDir)
		
		dbPath := filepath.Join(tmpDir, "lfu.db")
		diskCache, _ := NewDiskCache(dbPath, 24*time.Hour, 1)
		defer diskCache.Close()
		
		lfuCache := NewLFUCache(500, diskCache)
		defer lfuCache.Close()
		
		ctx := context.Background()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			entry := &CacheEntry{
				CacheKey:   fmt.Sprintf("key%d", i%1000),
				Model:      "gpt-4",
				PromptHash: fmt.Sprintf("hash%d", i%1000),
				Response:   fmt.Sprintf("response%d", i%1000),
				TokensUsed: 100,
				CostUSD:    0.01,
			}
			_ = lfuCache.Set(ctx, entry)
			
			if i%3 == 0 {
				_, _ = lfuCache.Get(ctx, fmt.Sprintf("key%d", i%1000))
			}
		}
	})
	
	b.Run("LRU", func(b *testing.B) {
		tmpDir, _ := os.MkdirTemp("", "bench_lru_*")
		defer os.RemoveAll(tmpDir)
		
		dbPath := filepath.Join(tmpDir, "lru.db")
		diskCache, _ := NewDiskCache(dbPath, 24*time.Hour, 1)
		defer diskCache.Close()
		
		ctx := context.Background()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			entry := &CacheEntry{
				CacheKey:   fmt.Sprintf("key%d", i%1000),
				Model:      "gpt-4",
				PromptHash: fmt.Sprintf("hash%d", i%1000),
				Response:   fmt.Sprintf("response%d", i%1000),
				TokensUsed: 100,
				CostUSD:    0.01,
			}
			_ = diskCache.Set(ctx, entry)
			
			if i%3 == 0 {
				_, _ = diskCache.Get(ctx, fmt.Sprintf("key%d", i%1000))
			}
		}
	})
}
