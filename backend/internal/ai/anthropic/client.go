package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"backend/pkg/aiinterface"
)

// Client Anthropic Claude 客户端适配器
type Client struct {
	apiKey     string
	baseURL    string
	modelID    string
	httpClient *http.Client
	maxRetries int
}

// NewClient 创建 Anthropic 客户端
func NewClient(config *aiinterface.ClientConfig) (*Client, error) {
	// 验证配置
	if config.APIKey == "" {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeAuth,
			Message: "Anthropic API Key 不能为空",
		}
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60
	}

	return &Client{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		modelID: config.Model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		maxRetries: maxRetries,
	}, nil
}

// anthropicRequest Anthropic API 请求
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	System      string             `json:"system,omitempty"`
}

// anthropicMessage Anthropic 消息
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse Anthropic API 响应
type anthropicResponse struct {
	ID      string              `json:"id"`
	Model   string              `json:"model"`
	Content []anthropicContent  `json:"content"`
	Usage   anthropicUsage      `json:"usage"`
}

// anthropicContent Anthropic 内容
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicUsage Anthropic Token 使用
type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ChatCompletion 对话补全（非流式）
func (c *Client) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
	// 转换消息格式
	messages := make([]anthropicMessage, 0, len(req.Messages))
	var systemPrompt string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Anthropic 将 system 消息单独处理
			systemPrompt = msg.Content
		} else {
			messages = append(messages, anthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// 构建请求
	anthropicReq := anthropicRequest{
		Model:       c.modelID,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		System:      systemPrompt,
	}

	// 调用 API（带重试）
	var resp *anthropicResponse
	var err error
	for i := 0; i <= c.maxRetries; i++ {
		resp, err = c.doRequest(ctx, anthropicReq)
		if err == nil {
			break
		}

		// 判断是否可重试
		if clientErr, ok := err.(*aiinterface.ClientError); ok && !clientErr.IsRetryable() {
			break
		}

		// 指数退避
		if i < c.maxRetries {
			backoff := time.Duration(1<<uint(i)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil {
		return nil, err
	}

	// 转换响应
	var content string
	if len(resp.Content) > 0 {
		content = resp.Content[0].Text
	}

	return &aiinterface.ChatCompletionResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Content: content,
		Usage: aiinterface.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}

// ChatCompletionStream 对话补全（流式）
func (c *Client) ChatCompletionStream(ctx context.Context, req *aiinterface.ChatCompletionRequest) (<-chan aiinterface.StreamChunk, <-chan error) {
	chunkChan := make(chan aiinterface.StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		// 转换消息格式
		messages := make([]anthropicMessage, 0, len(req.Messages))
		var systemPrompt string

		for _, msg := range req.Messages {
			if msg.Role == "system" {
				systemPrompt = msg.Content
			} else {
				messages = append(messages, anthropicMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}

		// 构建请求
		anthropicReq := anthropicRequest{
			Model:       c.modelID,
			Messages:    messages,
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			Stream:      true,
			System:      systemPrompt,
		}

		// 调用流式 API
		if err := c.doStreamRequest(ctx, anthropicReq, chunkChan); err != nil {
			errChan <- err
			return
		}

		// 发送结束标记
		chunkChan <- aiinterface.StreamChunk{Done: true}
	}()

	return chunkChan, errChan
}

// Embedding 文本向量化（Anthropic 不支持）
func (c *Client) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	return nil, &aiinterface.ClientError{
		Type:    aiinterface.ErrorTypeInvalidParams,
		Message: "Anthropic 不支持 Embedding API",
	}
}

// Name 返回客户端名称
func (c *Client) Name() string {
	return "anthropic"
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(ctx context.Context, req anthropicRequest) (*anthropicResponse, error) {
	// 序列化请求
	body, err := json.Marshal(req)
	if err != nil {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeInvalidParams,
			Message: "序列化请求失败",
			Err:     err,
		}
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeNetwork,
			Message: "创建请求失败",
			Err:     err,
		}
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// 发送请求
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeNetwork,
			Message: "请求失败",
			Err:     err,
		}
	}
	defer httpResp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeNetwork,
			Message: "读取响应失败",
			Err:     err,
		}
	}

	// 检查状态码
	if httpResp.StatusCode != http.StatusOK {
		return nil, c.parseError(httpResp.StatusCode, respBody)
	}

	// 解析响应
	var resp anthropicResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeServerError,
			Message: "解析响应失败",
			Err:     err,
		}
	}

	return &resp, nil
}

// doStreamRequest 执行流式请求
func (c *Client) doStreamRequest(ctx context.Context, req anthropicRequest, chunkChan chan<- aiinterface.StreamChunk) error {
	// 序列化请求
	body, err := json.Marshal(req)
	if err != nil {
		return &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeInvalidParams,
			Message: "序列化请求失败",
			Err:     err,
		}
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeNetwork,
			Message: "创建请求失败",
			Err:     err,
		}
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Accept", "text/event-stream")

	// 发送请求
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeNetwork,
			Message: "请求失败",
			Err:     err,
		}
	}
	defer httpResp.Body.Close()

	// 检查状态码
	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return c.parseError(httpResp.StatusCode, respBody)
	}

	// 读取流式响应（简化实现，实际应解析 SSE 格式）
	// 这里仅作示意，实际需要实现完整的 SSE 解析
	decoder := json.NewDecoder(httpResp.Body)
	for {
		var chunk map[string]any
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return &aiinterface.ClientError{
				Type:    aiinterface.ErrorTypeServerError,
				Message: "读取流失败",
				Err:     err,
			}
		}

		// 简化处理：提取文本内容
		if delta, ok := chunk["delta"].(map[string]any); ok {
			if text, ok := delta["text"].(string); ok {
				chunkChan <- aiinterface.StreamChunk{
					Content: text,
					Done:    false,
				}
			}
		}
	}

	return nil
}

// parseError 解析错误
func (c *Client) parseError(statusCode int, body []byte) *aiinterface.ClientError {
	var errType aiinterface.ErrorType
	message := string(body)

	switch statusCode {
	case 401, 403:
		errType = aiinterface.ErrorTypeAuth
	case 429:
		errType = aiinterface.ErrorTypeRateLimit
	case 400:
		errType = aiinterface.ErrorTypeInvalidParams
	case 500, 502, 503, 504:
		errType = aiinterface.ErrorTypeServerError
	default:
		errType = aiinterface.ErrorTypeUnknown
	}

	return &aiinterface.ClientError{
		Type:    errType,
		Message: fmt.Sprintf("Anthropic API 错误 (HTTP %d): %s", statusCode, message),
	}
}
