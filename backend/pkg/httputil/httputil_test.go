package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewClient 测试创建基础客户端
func TestNewClient(t *testing.T) {
	// 测试默认配置
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient() 返回 nil")
	}

	if client.timeout != 30*time.Second {
		t.Errorf("默认超时时间应为30秒，实际为 %v", client.timeout)
	}

	if client.headers["User-Agent"] != "AgentFlowCreativeHub/1.0" {
		t.Errorf("默认User-Agent不正确: %s", client.headers["User-Agent"])
	}

	// 测试自定义配置
	customClient := NewClient(
		WithTimeout(10*time.Second),
		WithHeaders(map[string]string{"X-Custom": "value"}),
		WithRetries(3),
	)

	if customClient.timeout != 10*time.Second {
		t.Errorf("自定义超时时间应为10秒，实际为 %v", customClient.timeout)
	}

	if customClient.headers["X-Custom"] != "value" {
		t.Errorf("自定义头未设置")
	}

	if customClient.retries != 3 {
		t.Errorf("重试次数应为3，实际为 %d", customClient.retries)
	}
}

// TestClientGetJSON 测试GetJSON方法
func TestClientGetJSON(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("期望GET请求，实际为 %s", r.Method)
		}

		// 返回JSON响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
			"status":  "ok",
		})
	}))
	defer server.Close()

	// 创建客户端
	client := NewClient()

	// 执行请求
	var result map[string]string
	err := client.GetJSON(context.Background(), server.URL, &result)
	if err != nil {
		t.Fatalf("GetJSON() 错误: %v", err)
	}

	// 验证结果
	if result["message"] != "success" {
		t.Errorf("期望 message='success'，实际为 '%s'", result["message"])
	}

	if result["status"] != "ok" {
		t.Errorf("期望 status='ok'，实际为 '%s'", result["status"])
	}
}

// TestClientPostJSON 测试PostJSON方法
func TestClientPostJSON(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("期望POST请求，实际为 %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("期望Content-Type为application/json")
		}

		// 解析请求体
		var reqBody map[string]string
		json.NewDecoder(r.Body).Decode(&reqBody)

		// 返回响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"echo": reqBody["message"],
		})
	}))
	defer server.Close()

	// 创建客户端
	client := NewClient()

	// 执行请求
	reqBody := map[string]string{"message": "hello"}
	var result map[string]string
	err := client.PostJSON(context.Background(), server.URL, reqBody, &result)
	if err != nil {
		t.Fatalf("PostJSON() 错误: %v", err)
	}

	// 验证结果
	if result["echo"] != "hello" {
		t.Errorf("期望 echo='hello'，实际为 '%s'", result["echo"])
	}
}

// TestCachedClientGetJSON 测试带缓存的GetJSON
func TestCachedClientGetJSON(t *testing.T) {
	requestCount := 0

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "success",
			"count":   requestCount,
		})
	}))
	defer server.Close()

	// 创建带缓存的客户端
	client := NewClient()
	cachedClient := NewCachedClient(client, WithCacheTTL(1*time.Hour))

	// 第一次请求（缓存未命中）
	var result1 map[string]interface{}
	err := cachedClient.GetJSON(context.Background(), server.URL, &result1)
	if err != nil {
		t.Fatalf("第一次GetJSON()错误: %v", err)
	}

	if result1["message"] != "success" {
		t.Errorf("期望 message='success'")
	}

	firstCount := int(result1["count"].(float64))

	// 第二次请求（应该从内存缓存命中）
	var result2 map[string]interface{}
	err = cachedClient.GetJSON(context.Background(), server.URL, &result2)
	if err != nil {
		t.Fatalf("第二次GetJSON()错误: %v", err)
	}

	secondCount := int(result2["count"].(float64))

	// 验证缓存命中（count应该相同，说明没有发起第二次请求）
	if firstCount != secondCount {
		t.Errorf("缓存未命中，第一次count=%d，第二次count=%d", firstCount, secondCount)
	}

	// 验证实际只发起了一次HTTP请求
	if requestCount != 1 {
		t.Errorf("期望只发起1次请求，实际发起了%d次", requestCount)
	}
}

// TestCacheKeyGeneration 测试缓存键生成
func TestCacheKeyGeneration(t *testing.T) {
	client := NewClient()
	cachedClient := NewCachedClient(client)

	// 相同的请求应该生成相同的缓存键
	key1 := cachedClient.generateCacheKey("GET", "http://example.com", nil)
	key2 := cachedClient.generateCacheKey("GET", "http://example.com", nil)

	if key1 != key2 {
		t.Errorf("相同请求应生成相同缓存键，key1=%s, key2=%s", key1, key2)
	}

	// 不同的URL应该生成不同的缓存键
	key3 := cachedClient.generateCacheKey("GET", "http://example.com/path", nil)
	if key1 == key3 {
		t.Errorf("不同URL应生成不同缓存键")
	}

	// 不同的方法应该生成不同的缓存键
	key4 := cachedClient.generateCacheKey("POST", "http://example.com", nil)
	if key1 == key4 {
		t.Errorf("不同HTTP方法应生成不同缓存键")
	}
}

// TestMemCacheExpiration 测试内存缓存过期
func TestMemCacheExpiration(t *testing.T) {
	client := NewClient()
	// 设置非常短的缓存时间
	cachedClient := NewCachedClient(client, WithCacheTTL(100*time.Millisecond))

	// 写入缓存
	key := "test-key"
	data := []byte(`{"message":"test"}`)
	cachedClient.setToMemCache(key, data)

	// 立即读取（应该命中）
	if _, ok := cachedClient.getFromMemCache(key); !ok {
		t.Error("缓存应该命中")
	}

	// 等待缓存过期
	time.Sleep(150 * time.Millisecond)

	// 再次读取（应该未命中）
	if _, ok := cachedClient.getFromMemCache(key); ok {
		t.Error("缓存应该已过期")
	}
}

// TestClearMemCache 测试清空内存缓存
func TestClearMemCache(t *testing.T) {
	client := NewClient()
	cachedClient := NewCachedClient(client)

	// 写入缓存
	key := "test-key"
	data := []byte(`{"message":"test"}`)
	cachedClient.setToMemCache(key, data)

	// 验证缓存存在
	if _, ok := cachedClient.getFromMemCache(key); !ok {
		t.Error("缓存应该存在")
	}

	// 清空缓存
	cachedClient.ClearMemCache()

	// 验证缓存已清空
	if _, ok := cachedClient.getFromMemCache(key); ok {
		t.Error("缓存应该已清空")
	}
}
