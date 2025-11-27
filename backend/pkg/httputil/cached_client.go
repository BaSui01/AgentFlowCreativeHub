package httputil

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// CachedClient 带缓存的HTTP客户端（使用内存缓存）
type CachedClient struct {
	client      *Client
	memCache    *sync.Map // 内存缓存层
	cacheTTL    time.Duration
	shouldCache func(*http.Request, *http.Response) bool // 缓存条件判断
	mu          sync.RWMutex
}

// CachedClientOption 缓存客户端配置选项
type CachedClientOption func(*CachedClient)

// WithCacheTTL 设置缓存过期时间
func WithCacheTTL(ttl time.Duration) CachedClientOption {
	return func(c *CachedClient) {
		c.cacheTTL = ttl
	}
}

// WithCacheCondition 设置缓存条件判断函数
func WithCacheCondition(fn func(*http.Request, *http.Response) bool) CachedClientOption {
	return func(c *CachedClient) {
		c.shouldCache = fn
	}
}

// NewCachedClient 创建带缓存的HTTP客户端
func NewCachedClient(client *Client, opts ...CachedClientOption) *CachedClient {
	cc := &CachedClient{
		client:   client,
		memCache: &sync.Map{},
		cacheTTL: 1 * time.Hour, // 默认1小时
		shouldCache: func(req *http.Request, resp *http.Response) bool {
			// 默认只缓存GET请求且状态码为200的响应
			return req.Method == "GET" && resp.StatusCode == http.StatusOK
		},
	}

	// 应用选项
	for _, opt := range opts {
		opt(cc)
	}

	return cc
}

// cacheEntry 缓存条目
type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// generateCacheKey 生成缓存键
func (cc *CachedClient) generateCacheKey(method, url string, body []byte) string {
	// 使用MD5哈希生成缓存键
	hash := md5.New()
	hash.Write([]byte(method))
	hash.Write([]byte(url))
	if len(body) > 0 {
		hash.Write(body)
	}
	return "http:" + hex.EncodeToString(hash.Sum(nil))
}

// getFromMemCache 从内存缓存获取
func (cc *CachedClient) getFromMemCache(key string) ([]byte, bool) {
	if value, ok := cc.memCache.Load(key); ok {
		entry := value.(cacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.data, true
		}
		// 过期，删除
		cc.memCache.Delete(key)
	}
	return nil, false
}

// setToMemCache 写入内存缓存
func (cc *CachedClient) setToMemCache(key string, data []byte) {
	cc.memCache.Store(key, cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(cc.cacheTTL),
	})
}



// Get 发送GET请求（带缓存）
func (cc *CachedClient) Get(ctx context.Context, url string) (*http.Response, error) {
	cacheKey := cc.generateCacheKey("GET", url, nil)

	// 检查内存缓存
	if data, ok := cc.getFromMemCache(cacheKey); ok {
		// 模拟http.Response
		return cc.buildResponseFromCache(data), nil
	}

	// 缓存未命中，执行实际请求
	resp, err := cc.client.Get(ctx, url)
	if err != nil {
		return nil, err
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	// 检查是否需要缓存
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if cc.shouldCache(req, resp) {
		// 写入内存缓存
		cc.setToMemCache(cacheKey, body)
	}

	// 重建响应对象
	return cc.buildResponseFromCache(body), nil
}

// GetJSON 发送GET请求并解析JSON响应（带缓存）
func (cc *CachedClient) GetJSON(ctx context.Context, url string, result interface{}) error {
	cacheKey := cc.generateCacheKey("GET", url, nil)

	// 检查内存缓存
	if data, ok := cc.getFromMemCache(cacheKey); ok {
		return json.Unmarshal(data, result)
	}

	// 缓存未命中，执行实际请求
	resp, err := cc.client.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("GET请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP请求返回错误状态: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查是否需要缓存
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if cc.shouldCache(req, resp) {
		// 写入内存缓存
		cc.setToMemCache(cacheKey, body)
	}

	// 解析JSON
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("解析JSON响应失败: %w", err)
	}

	return nil
}

// PostJSON 发送POST请求（JSON格式）并解析JSON响应（带缓存）
func (cc *CachedClient) PostJSON(ctx context.Context, url string, body interface{}, result interface{}) error {
	// 序列化请求体
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	cacheKey := cc.generateCacheKey("POST", url, jsonData)

	// 检查内存缓存（POST请求通常不缓存，但保留支持）
	if data, ok := cc.getFromMemCache(cacheKey); ok {
		if result != nil {
			return json.Unmarshal(data, result)
		}
		return nil
	}

	// 缓存未命中，执行实际请求
	err = cc.client.PostJSON(ctx, url, body, result)
	if err != nil {
		return err
	}

	// POST请求通常不缓存，除非shouldCache明确允许
	// 这里暂时不实现POST缓存逻辑

	return nil
}

// buildResponseFromCache 从缓存数据构建http.Response
func (cc *CachedClient) buildResponseFromCache(data []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(data)),
		Header:     make(http.Header),
	}
}

// ClearMemCache 清空内存缓存
func (cc *CachedClient) ClearMemCache() {
	cc.memCache = &sync.Map{}
}

// Close 关闭客户端
func (cc *CachedClient) Close() error {
	cc.ClearMemCache()
	return cc.client.Close()
}
