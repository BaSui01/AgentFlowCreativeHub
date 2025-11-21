package openai

import (
	"backend/pkg/aiinterface"
	"context"
	"fmt"
	"time"

	

	openai "github.com/sashabaranov/go-openai"
)

// Client OpenAI 客户端适配器
type Client struct {
	client     *openai.Client
	modelID    string
	maxRetries int
}

// NewClient 创建 OpenAI 客户端
func NewClient(config *aiinterface.ClientConfig) (*Client, error) {
	// 验证配置
	if config.APIKey == "" {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeAuth,
			Message: "OpenAI API Key 不能为空",
		}
	}

	// 创建配置
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}
	if config.OrgID != "" {
		clientConfig.OrgID = config.OrgID
	}

	// 设置默认值
	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	return &Client{
		client:     openai.NewClientWithConfig(clientConfig),
		modelID:    config.Model,
		maxRetries: maxRetries,
	}, nil
}

// ChatCompletion 对话补全（非流式）
func (c *Client) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
	// 转换消息格式
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// 构建请求
	openaiReq := openai.ChatCompletionRequest{
		Model:       c.modelID,
		Messages:    messages,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
		TopP:        float32(req.TopP),
	}

	// 调用 API（带重试）
	var resp openai.ChatCompletionResponse
	var err error
	for i := 0; i <= c.maxRetries; i++ {
		resp, err = c.client.CreateChatCompletion(ctx, openaiReq)
		if err == nil {
			break
		}

		// 判断是否可重试
		if !isRetryableError(err) {
			break
		}

		// 指数退避
		if i < c.maxRetries {
			backoff := time.Duration(1<<uint(i)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil {
		return nil, wrapError(err)
	}

	// 转换响应
	if len(resp.Choices) == 0 {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeServerError,
			Message: "API 返回空响应",
		}
	}

	return &aiinterface.ChatCompletionResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Content: resp.Choices[0].Message.Content,
		Usage: aiinterface.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
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
		messages := make([]openai.ChatCompletionMessage, len(req.Messages))
		for i, msg := range req.Messages {
			messages[i] = openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		// 构建请求
		openaiReq := openai.ChatCompletionRequest{
			Model:       c.modelID,
			Messages:    messages,
			Temperature: float32(req.Temperature),
			MaxTokens:   req.MaxTokens,
			TopP:        float32(req.TopP),
			Stream:      true,
		}

		// 创建流
		stream, err := c.client.CreateChatCompletionStream(ctx, openaiReq)
		if err != nil {
			errChan <- wrapError(err)
			return
		}
		defer stream.Close()

		// 读取流
		for {
			response, err := stream.Recv()
			if err != nil {
				// EOF 表示正常结束
				if err.Error() == "EOF" {
					chunkChan <- aiinterface.StreamChunk{Done: true}
					return
				}
				errChan <- wrapError(err)
				return
			}

			// 发送响应块
			if len(response.Choices) > 0 {
				chunk := aiinterface.StreamChunk{
					ID:      response.ID,
					Model:   response.Model,
					Content: response.Choices[0].Delta.Content,
					Done:    false,
				}
				chunkChan <- chunk
			}
		}
	}()

	return chunkChan, errChan
}

// Embedding 文本向量化
func (c *Client) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	// 构建请求
	openaiReq := openai.EmbeddingRequest{
		Input: req.Texts,
		Model: openai.AdaEmbeddingV2, // 默认使用 text-embedding-ada-002
	}

	// 如果指定了模型，使用指定的模型
	if req.Model != "" {
		openaiReq.Model = openai.EmbeddingModel(req.Model)
	}

	// 调用 API（带重试）
	var resp openai.EmbeddingResponse
	var err error
	for i := 0; i <= c.maxRetries; i++ {
		resp, err = c.client.CreateEmbeddings(ctx, openaiReq)
		if err == nil {
			break
		}

		if !isRetryableError(err) {
			break
		}

		if i < c.maxRetries {
			backoff := time.Duration(1<<uint(i)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil {
		return nil, wrapError(err)
	}

	// 转换响应
	embeddings := make([][]float64, len(resp.Data))
	for i, data := range resp.Data {
		embedding := make([]float64, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float64(v)
		}
		embeddings[i] = embedding
	}

	return &aiinterface.EmbeddingResponse{
		Embeddings: embeddings,
		Usage: aiinterface.Usage{
			PromptTokens: resp.Usage.PromptTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}

// Name 返回客户端名称
func (c *Client) Name() string {
	return "openai"
}

// Close 关闭客户端
func (c *Client) Close() error {
	// OpenAI 客户端无需显式关闭
	return nil
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	// 简化判断：网络错误和服务器错误可重试
	errMsg := err.Error()
	return contains(errMsg, "timeout") ||
		contains(errMsg, "connection") ||
		contains(errMsg, "rate limit") ||
		contains(errMsg, "500") ||
		contains(errMsg, "502") ||
		contains(errMsg, "503") ||
		contains(errMsg, "504")
}

// wrapError 包装错误
func wrapError(err error) *aiinterface.ClientError {
	errMsg := err.Error()

	// 判断错误类型
	var errType aiinterface.ErrorType
	switch {
	case contains(errMsg, "401") || contains(errMsg, "403"):
		errType = aiinterface.ErrorTypeAuth
	case contains(errMsg, "rate limit") || contains(errMsg, "429"):
		errType = aiinterface.ErrorTypeRateLimit
	case contains(errMsg, "400") || contains(errMsg, "invalid"):
		errType = aiinterface.ErrorTypeInvalidParams
	case contains(errMsg, "500") || contains(errMsg, "502") || contains(errMsg, "503"):
		errType = aiinterface.ErrorTypeServerError
	case contains(errMsg, "timeout") || contains(errMsg, "connection"):
		errType = aiinterface.ErrorTypeNetwork
	default:
		errType = aiinterface.ErrorTypeUnknown
	}

	return &aiinterface.ClientError{
		Type:    errType,
		Message: fmt.Sprintf("OpenAI API 错误"),
		Err:     err,
	}
}

// contains 字符串包含判断（不区分大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		len(s) > len(substr)*2 && s[1:len(substr)+1] == substr))
}


