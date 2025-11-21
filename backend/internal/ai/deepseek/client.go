package deepseek

import (
	"backend/pkg/aiinterface"
	"context"
	"fmt"

	

	"github.com/sashabaranov/go-openai"
)

// DeepSeekClient DeepSeek 客户端
// DeepSeek API 兼容 OpenAI 格式
type DeepSeekClient struct {
	client *openai.Client
	model  string
}

// NewClient 创建 DeepSeek 客户端
func NewClient(config *aiinterface.ClientConfig) (aiinterface.ModelClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("DeepSeek API Key 不能为空")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = baseURL

	return &DeepSeekClient{
		client: openai.NewClientWithConfig(clientConfig),
		model:  config.Model,
	}, nil
}

// ChatCompletion 对话补全（非流式）
func (c *DeepSeekClient) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
		TopP:        float32(req.TopP),
	})

	if err != nil {
		return nil, fmt.Errorf("DeepSeek API 调用失败: %w", err)
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &aiinterface.ChatCompletionResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Content: content,
		Usage: aiinterface.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// ChatCompletionStream 对话补全（流式）
func (c *DeepSeekClient) ChatCompletionStream(ctx context.Context, req *aiinterface.ChatCompletionRequest) (<-chan aiinterface.StreamChunk, <-chan error) {
	chunkChan := make(chan aiinterface.StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
		for _, msg := range req.Messages {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
			Model:       c.model,
			Messages:    messages,
			Temperature: float32(req.Temperature),
			MaxTokens:   req.MaxTokens,
			TopP:        float32(req.TopP),
			Stream:      true,
		})

		if err != nil {
			errChan <- fmt.Errorf("DeepSeek API 调用失败: %w", err)
			return
		}
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				errChan <- fmt.Errorf("读取流失败: %w", err)
				return
			}

			if len(response.Choices) > 0 {
				chunkChan <- aiinterface.StreamChunk{
					ID:      response.ID,
					Model:   response.Model,
					Content: response.Choices[0].Delta.Content,
					Done:    false,
				}
			}
		}

		chunkChan <- aiinterface.StreamChunk{Done: true}
	}()

	return chunkChan, errChan
}

// Embedding DeepSeek 暂不支持 Embedding
func (c *DeepSeekClient) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	return nil, fmt.Errorf("DeepSeek 暂不支持 Embedding 功能")
}

// Name 返回客户端名称
func (c *DeepSeekClient) Name() string {
	return "deepseek"
}

// Close 关闭客户端
func (c *DeepSeekClient) Close() error {
	return nil
}

// 预置 DeepSeek 模型列表
var DefaultDeepSeekModels = []DeepSeekModelInfo{
	{
		ID:            "deepseek-chat",
		Name:          "DeepSeek Chat",
		MaxTokens:     4096,
		ContextWindow: 32768,
	},
	{
		ID:            "deepseek-coder",
		Name:          "DeepSeek Coder",
		MaxTokens:     4096,
		ContextWindow: 16384,
	},
}

// DeepSeekModelInfo DeepSeek 模型信息
type DeepSeekModelInfo struct {
	ID            string
	Name          string
	MaxTokens     int
	ContextWindow int
}


