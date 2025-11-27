package httputil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client HTTP客户端包装器，提供便利的请求方法
type Client struct {
	httpClient *http.Client
	timeout    time.Duration
	headers    map[string]string
	retries    int
}

// ClientOption 客户端配置选项
type ClientOption func(*Client)

// WithTimeout 设置请求超时时间
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
		c.httpClient.Timeout = timeout
	}
}

// WithHeaders 设置默认请求头
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		c.headers = headers
	}
}

// WithRetries 设置重试次数
func WithRetries(retries int) ClientOption {
	return func(c *Client) {
		c.retries = retries
	}
}

// NewClient 创建HTTP客户端
func NewClient(opts ...ClientOption) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		timeout: 30 * time.Second,
		headers: make(map[string]string),
		retries: 0,
	}

	// 应用选项
	for _, opt := range opts {
		opt(client)
	}

	// 设置默认User-Agent
	if _, ok := client.headers["User-Agent"]; !ok {
		client.headers["User-Agent"] = "AgentFlowCreativeHub/1.0"
	}

	return client
}

// SetHeader 设置单个请求头
func (c *Client) SetHeader(key, value string) {
	c.headers[key] = value
}

// SetHeaders 批量设置请求头
func (c *Client) SetHeaders(headers map[string]string) {
	for k, v := range headers {
		c.headers[k] = v
	}
}

// applyHeaders 将默认headers应用到请求
func (c *Client) applyHeaders(req *http.Request) {
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
}

// Do 执行HTTP请求
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	// 应用默认headers
	c.applyHeaders(req)

	// 执行请求（支持重试）
	var resp *http.Response
	var err error

	for i := 0; i <= c.retries; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			// 请求成功或客户端错误（4xx），不重试
			break
		}

		// 如果不是最后一次重试，稍作延迟
		if i < c.retries {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}

	return resp, err
}

// Get 发送GET请求
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建GET请求失败: %w", err)
	}

	return c.Do(ctx, req)
}

// Post 发送POST请求
func (c *Client) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("创建POST请求失败: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	return c.Do(ctx, req)
}

// GetJSON 发送GET请求并解析JSON响应
func (c *Client) GetJSON(ctx context.Context, url string, result interface{}) error {
	resp, err := c.Get(ctx, url)
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

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("解析JSON响应失败: %w", err)
	}

	return nil
}

// PostJSON 发送POST请求（JSON格式）并解析JSON响应
func (c *Client) PostJSON(ctx context.Context, url string, body interface{}, result interface{}) error {
	// 序列化请求体
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	resp, err := c.Post(ctx, url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("HTTP请求返回错误状态: %d", resp.StatusCode)
	}

	// 如果result为nil，不解析响应体
	if result == nil {
		return nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("解析JSON响应失败: %w", err)
	}

	return nil
}

// Close 关闭客户端（如果需要清理资源）
func (c *Client) Close() error {
	// HTTP客户端不需要显式关闭，保留此方法以备将来扩展
	return nil
}
