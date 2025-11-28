// Package cache 提供缓存相关功能
package cache

import (
	"context"
	"fmt"
	"time"
)

// ExampleLFUCache 演示 LFU 缓存的基本使用
func ExampleLFUCache() {
	// 1. 创建底层磁盘缓存
	diskCache, err := NewDiskCache("example_lfu.db", 24*time.Hour, 1)
	if err != nil {
		panic(err)
	}
	defer diskCache.Close()

	// 2. 创建 LFU 缓存（容量为 100 个条目）
	lfuCache := NewLFUCache(100, diskCache)
	defer lfuCache.Close()

	ctx := context.Background()

	// 3. 写入缓存
	entry := &CacheEntry{
		CacheKey:   GenerateCacheKey("gpt-4", "Hello world"),
		Model:      "gpt-4",
		PromptHash: "hash123",
		Response:   "Hello! How can I help you today?",
		TokensUsed: 50,
		CostUSD:    0.001,
	}
	
	if err := lfuCache.Set(ctx, entry); err != nil {
		fmt.Printf("写入缓存失败: %v\n", err)
		return
	}

	// 4. 读取缓存
	retrieved, err := lfuCache.Get(ctx, entry.CacheKey)
	if err != nil {
		fmt.Printf("读取缓存失败: %v\n", err)
		return
	}
	
	if retrieved != nil {
		fmt.Printf("缓存命中: %s\n", retrieved.Response)
		// 输出: 缓存命中: Hello! How can I help you today?
	}

	// 5. 查看统计信息
	stats, _ := lfuCache.GetStats(ctx)
	fmt.Printf("LFU 跟踪条目数: %v\n", stats["lfu_tracked_entries"])
	fmt.Printf("最小访问频率: %v\n", stats["lfu_min_frequency"])
	fmt.Printf("频率分布: %v\n", stats["frequency_distribution"])
}

// ExampleLFUCacheWithFrequency 演示频率淘汰机制
func ExampleLFUCacheWithFrequency() {
	diskCache, _ := NewDiskCache("example_lfu_freq.db", 24*time.Hour, 1)
	defer diskCache.Close()

	// 创建容量为 3 的小缓存来演示淘汰
	lfuCache := NewLFUCache(3, diskCache)
	defer lfuCache.Close()

	ctx := context.Background()

	// 添加 3 个条目
	for i := 1; i <= 3; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash%d", i),
			Response:   fmt.Sprintf("Response %d", i),
			TokensUsed: 100,
			CostUSD:    0.01,
		}
		lfuCache.Set(ctx, entry)
	}

	// key1 访问 5 次
	for i := 0; i < 5; i++ {
		lfuCache.Get(ctx, "key1")
	}

	// key2 访问 3 次
	for i := 0; i < 3; i++ {
		lfuCache.Get(ctx, "key2")
	}

	// key3 只访问 1 次
	lfuCache.Get(ctx, "key3")

	// 添加第 4 个条目，会淘汰 key3（频率最低）
	entry4 := &CacheEntry{
		CacheKey:   "key4",
		Model:      "gpt-4",
		PromptHash: "hash4",
		Response:   "Response 4",
		TokensUsed: 100,
		CostUSD:    0.01,
	}
	lfuCache.Set(ctx, entry4)

	// 查看频率分布
	stats, _ := lfuCache.GetStats(ctx)
	fmt.Printf("频率分布: %v\n", stats["frequency_distribution"])
	// 输出类似: 频率分布: map[1:1 4:1 6:1]
	// 说明: key4=1次, key2=4次(3+1), key1=6次(5+1)
}
