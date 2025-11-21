package qwen

import (
	"backend/pkg/aiinterface"
	"context"
	"fmt"

	

	"github.com/sashabaranov/go-openai"
)

// QwenClient 通义千问客户端
// Qwen API 兼容 OpenAI 格式
type QwenClient struct {
	client *openai.Client
	model  string
}

// NewClient 创建 Qwen 客户端
func NewClient(config *aiinterface.ClientConfig) (aiinterface.ModelClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Qwen API Key 不能为空")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = baseURL

	return &QwenClient{
		client: openai.NewClientWithConfig(clientConfig),
		model:  config.Model,
	}, nil
}

// ChatCompletion 对话补全（非流式）
func (c *QwenClient) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
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
		return nil, fmt.Errorf("Qwen API 调用失败: %w", err)
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
func (c *QwenClient) ChatCompletionStream(ctx context.Context, req *aiinterface.ChatCompletionRequest) (<-chan aiinterface.StreamChunk, <-chan error) {
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
			errChan <- fmt.Errorf("Qwen API 调用失败: %w", err)
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

// Embedding 文本向量化
func (c *QwenClient) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(c.model),
		Input: req.Texts,
	})

	if err != nil {
		return nil, fmt.Errorf("Qwen Embedding 调用失败: %w", err)
	}

	embeddings := make([][]float64, 0, len(resp.Data))
	for _, data := range resp.Data {
		embedding := make([]float64, len(data.Embedding))
		for i, v := range data.Embedding {
			embedding[i] = float64(v)
		}
		embeddings = append(embeddings, embedding)
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
func (c *QwenClient) Name() string {
	return "qwen"
}

// Close 关闭客户端
func (c *QwenClient) Close() error {
	return nil
}

// 预置 Qwen 模型列表
var DefaultQwenModels = []QwenModelInfo{
	{
		ID:            "qwen-max",
		Name:          "Qwen Max",
		MaxTokens:     8000,
		ContextWindow: 32000,
	},
	{
		ID:            "qwen-plus",
		Name:          "Qwen Plus",
		MaxTokens:     8000,
		ContextWindow: 32000,
	},
	{
		ID:            "qwen-turbo",
		Name:          "Qwen Turbo",
		MaxTokens:     8000,
		ContextWindow: 8000,
	},
	{
		ID:            "text-embedding-v2",
		Name:          "Qwen Text Embedding V2",
		MaxTokens:     0,
		ContextWindow: 2048,
	},
}

// QwenModelInfo Qwen 模型信息
type QwenModelInfo struct {
	ID            string
	Name          string
	MaxTokens     int
	ContextWindow int
}


