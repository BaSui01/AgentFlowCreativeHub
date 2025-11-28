package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkDiskCacheSet 基准测试：写入缓存
func BenchmarkDiskCacheSet(b *testing.B) {
	cache, _ := NewDiskCache(":memory:", time.Hour, 1)
	defer cache.Close()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key-%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash-%d", i),
			Response:   "This is a test response for benchmarking",
			TokensUsed: 100,
			CostUSD:    0.002,
		}
		cache.Set(ctx, entry)
	}
}

// BenchmarkDiskCacheGet 基准测试：读取缓存（命中）
func BenchmarkDiskCacheGet(b *testing.B) {
	cache, _ := NewDiskCache(":memory:", time.Hour, 1)
	defer cache.Close()

	ctx := context.Background()

	// 预先写入数据
	for i := 0; i < 100; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key-%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash-%d", i),
			Response:   "This is a test response for benchmarking",
			TokensUsed: 100,
			CostUSD:    0.002,
		}
		cache.Set(ctx, entry)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%100)
		cache.Get(ctx, key)
	}
}

// BenchmarkDiskCacheGetMiss 基准测试：读取缓存（未命中）
func BenchmarkDiskCacheGetMiss(b *testing.B) {
	cache, _ := NewDiskCache(":memory:", time.Hour, 1)
	defer cache.Close()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("nonexistent-key-%d", i)
		cache.Get(ctx, key)
	}
}

// BenchmarkDiskCacheConcurrent 基准测试：并发读写
func BenchmarkDiskCacheConcurrent(b *testing.B) {
	cache, _ := NewDiskCache(":memory:", time.Hour, 1)
	defer cache.Close()

	ctx := context.Background()

	// 预先写入数据
	for i := 0; i < 100; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key-%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash-%d", i),
			Response:   "This is a test response for benchmarking",
			TokensUsed: 100,
			CostUSD:    0.002,
		}
		cache.Set(ctx, entry)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%100)
			cache.Get(ctx, key)
			i++
		}
	})
}

// BenchmarkDiskCacheStats 基准测试：获取统计信息
func BenchmarkDiskCacheStats(b *testing.B) {
	cache, _ := NewDiskCache(":memory:", time.Hour, 1)
	defer cache.Close()

	ctx := context.Background()

	// 预先写入数据
	for i := 0; i < 1000; i++ {
		entry := &CacheEntry{
			CacheKey:   fmt.Sprintf("key-%d", i),
			Model:      "gpt-4",
			PromptHash: fmt.Sprintf("hash-%d", i),
			Response:   "This is a test response for benchmarking",
			TokensUsed: 100,
			CostUSD:    0.002,
		}
		cache.Set(ctx, entry)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.GetStats(ctx)
	}
}
