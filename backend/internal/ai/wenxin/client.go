// Package wenxin 百度文心一言客户端
// 文心一言兼容 OpenAI API 协议
// API 文档: https://cloud.baidu.com/doc/WENXINWORKSHOP/
package wenxin

import (
	"backend/pkg/aiinterface"
	"context"
	"fmt"
	"io"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	DefaultBaseURL = "https://qianfan.baidubce.com/v2"
)

// 支持的模型
const (
	ModelERNIE4       = "ernie-4.0-8k"
	ModelERNIE4Turbo  = "ernie-4.0-turbo-8k"
	ModelERNIE35      = "ernie-3.5-8k"
	ModelERNIESpeed   = "ernie-speed-8k"
	ModelERNIELite    = "ernie-lite-8k"
	ModelERNIETiny    = "ernie-tiny-8k"
)

// Client 文心一言客户端
type Client struct {
	client     *openai.Client
	modelID    string
	maxRetries int
}

// NewClient 创建文心一言客户端
func NewClient(config *aiinterface.ClientConfig) (*Client, error) {
	if config.APIKey == "" {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeAuth,
			Message: "文心一言 API Key 不能为空",
		}
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = baseURL

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

// Name 返回客户端名称
func (c *Client) Name() string {
	return "wenxin"
}

// ChatCompletion 对话补全
func (c *Client) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	openaiReq := openai.ChatCompletionRequest{
		Model:       c.modelID,
		Messages:    messages,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
		TopP:        float32(req.TopP),
	}

	var resp openai.ChatCompletionResponse
	var err error
	for i := 0; i <= c.maxRetries; i++ {
		resp, err = c.client.CreateChatCompletion(ctx, openaiReq)
		if err == nil {
			break
		}
		if i < c.maxRetries {
			time.Sleep(time.Duration(1<<uint(i)) * time.Second)
		}
	}

	if err != nil {
		return nil, wrapError(err)
	}

	if len(resp.Choices) == 0 {
		return nil, &aiinterface.ClientError{
			Type:    aiinterface.ErrorTypeServerError,
			Message: "文心一言返回空响应",
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

// ChatCompletionStream 流式对话
func (c *Client) ChatCompletionStream(ctx context.Context, req *aiinterface.ChatCompletionRequest) (<-chan aiinterface.StreamChunk, <-chan error) {
	chunkCh := make(chan aiinterface.StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		messages := make([]openai.ChatCompletionMessage, len(req.Messages))
		for i, msg := range req.Messages {
			messages[i] = openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		openaiReq := openai.ChatCompletionRequest{
			Model:       c.modelID,
			Messages:    messages,
			Temperature: float32(req.Temperature),
			MaxTokens:   req.MaxTokens,
			TopP:        float32(req.TopP),
			Stream:      true,
		}

		stream, err := c.client.CreateChatCompletionStream(ctx, openaiReq)
		if err != nil {
			errCh <- wrapError(err)
			return
		}
		defer stream.Close()

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				errCh <- wrapError(err)
				return
			}

			content := ""
			if len(resp.Choices) > 0 {
				content = resp.Choices[0].Delta.Content
			}

			chunkCh <- aiinterface.StreamChunk{
				ID:      resp.ID,
				Content: content,
				Done:    len(resp.Choices) > 0 && resp.Choices[0].FinishReason != "",
			}
		}
	}()

	return chunkCh, errCh
}

// Embedding 向量嵌入
func (c *Client) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	model := req.Model
	if model == "" {
		model = "bge-large-zh"
	}

	openaiReq := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(model),
		Input: req.Texts,
	}

	resp, err := c.client.CreateEmbeddings(ctx, openaiReq)
	if err != nil {
		return nil, wrapError(err)
	}

	embeddings := make([][]float64, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = make([]float64, len(data.Embedding))
		for j, v := range data.Embedding {
			embeddings[i][j] = float64(v)
		}
	}

	return &aiinterface.EmbeddingResponse{
		Embeddings: embeddings,
		Usage: aiinterface.Usage{
			PromptTokens: resp.Usage.PromptTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}

// Close 关闭客户端
func (c *Client) Close() error {
	return nil
}

func wrapError(err error) error {
	return &aiinterface.ClientError{
		Type:    aiinterface.ErrorTypeServerError,
		Message: fmt.Sprintf("文心一言请求失败: %v", err),
		Err:     err,
	}
}
