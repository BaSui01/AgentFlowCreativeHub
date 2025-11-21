package azure

import (
	"backend/pkg/aiinterface"
	"context"
	"fmt"

	

	"github.com/sashabaranov/go-openai"
)

// AzureClient Azure OpenAI 客户端
// Azure OpenAI 使用与 OpenAI 相同的 SDK，只是端点不同
type AzureClient struct {
	client     *openai.Client
	deployment string // Azure 部署名称
}

// NewClient 创建 Azure OpenAI 客户端
func NewClient(config *aiinterface.ClientConfig) (aiinterface.ModelClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Azure OpenAI API Key 不能为空")
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("Azure OpenAI 端点不能为空")
	}

	// Azure API 版本，默认使用最新版本
	apiVersion := config.Extra["api_version"].(string)
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	// 创建 Azure 配置
	azureConfig := openai.DefaultAzureConfig(config.APIKey, config.BaseURL)
	azureConfig.APIVersion = apiVersion

	client := openai.NewClientWithConfig(azureConfig)

	return &AzureClient{
		client:     client,
		deployment: config.Model, // Azure 使用 deployment 名称
	}, nil
}

// ChatCompletion 对话补全（非流式）
func (c *AzureClient) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
	// 转换为 OpenAI 格式
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.deployment, // Azure 使用 deployment 名称
		Messages:    messages,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
		TopP:        float32(req.TopP),
	})

	if err != nil {
		return nil, fmt.Errorf("Azure OpenAI API 调用失败: %w", err)
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
func (c *AzureClient) ChatCompletionStream(ctx context.Context, req *aiinterface.ChatCompletionRequest) (<-chan aiinterface.StreamChunk, <-chan error) {
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
			Model:       c.deployment,
			Messages:    messages,
			Temperature: float32(req.Temperature),
			MaxTokens:   req.MaxTokens,
			TopP:        float32(req.TopP),
			Stream:      true,
		})

		if err != nil {
			errChan <- fmt.Errorf("Azure OpenAI API 调用失败: %w", err)
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
func (c *AzureClient) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(c.deployment),
		Input: req.Texts,
	})

	if err != nil {
		return nil, fmt.Errorf("Azure OpenAI Embedding 调用失败: %w", err)
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
func (c *AzureClient) Name() string {
	return "azure"
}

// Close 关闭客户端
func (c *AzureClient) Close() error {
	return nil
}


