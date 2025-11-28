package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// BenchmarkClientGetJSON 基准测试：基础客户端GET JSON
func BenchmarkClientGetJSON(b *testing.B) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
			"status":  "ok",
		})
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var result map[string]string
		client.GetJSON(ctx, server.URL, &result)
	}
}

// BenchmarkCachedClientGetJSON_Hit 基准测试：缓存命中
func BenchmarkCachedClientGetJSON_Hit(b *testing.B) {
	requestCount := 0

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
			"count":   "1",
		})
	}))
	defer server.Close()

	baseClient := NewClient()
	cachedClient := NewCachedClient(baseClient, WithCacheTTL(1*time.Hour))
	ctx := context.Background()

	// 预热缓存
	var warmup map[string]string
	cachedClient.GetJSON(ctx, server.URL, &warmup)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var result map[string]string
		cachedClient.GetJSON(ctx, server.URL, &result)
	}

	b.ReportMetric(float64(requestCount), "http_requests")
}

// BenchmarkCachedClientGetJSON_Miss 基准测试：缓存未命中
func BenchmarkCachedClientGetJSON_Miss(b *testing.B) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
		})
	}))
	defer server.Close()

	baseClient := NewClient()
	cachedClient := NewCachedClient(baseClient, WithCacheTTL(1*time.Hour))
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 每次请求不同的URL，确保缓存未命中
		var result map[string]string
		url := server.URL + "?i=" + string(rune(i))
		cachedClient.GetJSON(ctx, url, &result)
	}
}

// BenchmarkCachedClient_Concurrent 基准测试：并发访问
func BenchmarkCachedClient_Concurrent(b *testing.B) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
		})
	}))
	defer server.Close()

	baseClient := NewClient()
	cachedClient := NewCachedClient(baseClient, WithCacheTTL(1*time.Hour))
	ctx := context.Background()

	// 预热缓存
	var warmup map[string]string
	cachedClient.GetJSON(ctx, server.URL, &warmup)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var result map[string]string
			cachedClient.GetJSON(ctx, server.URL, &result)
		}
	})
}

// BenchmarkCachedClient_GetStats 基准测试：获取统计信息
func BenchmarkCachedClient_GetStats(b *testing.B) {
	baseClient := NewClient()
	cachedClient := NewCachedClient(baseClient, WithCacheTTL(1*time.Hour))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cachedClient.GetStats()
	}
}
