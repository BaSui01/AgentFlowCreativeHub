package custom

import (
	"backend/pkg/aiinterface"
	"context"
	"fmt"

	

	"github.com/sashabaranov/go-openai"
)

// CustomClient 自定义端点客户端
// 支持任何兼容 OpenAI 格式的自定义端点
type CustomClient struct {
	client *openai.Client
	model  string
}

// NewClient 创建自定义端点客户端
func NewClient(config *aiinterface.ClientConfig) (aiinterface.ModelClient, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("自定义端点 BaseURL 不能为空")
	}

	// API Key 可以为空（某些自定义端点不需要）
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = "dummy" // 占位符
	}

	clientConfig := openai.DefaultConfig(apiKey)
	clientConfig.BaseURL = config.BaseURL

	return &CustomClient{
		client: openai.NewClientWithConfig(clientConfig),
		model:  config.Model,
	}, nil
}

// ChatCompletion 对话补全（非流式）
func (c *CustomClient) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
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
		return nil, fmt.Errorf("自定义端点 API 调用失败: %w", err)
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
func (c *CustomClient) ChatCompletionStream(ctx context.Context, req *aiinterface.ChatCompletionRequest) (<-chan aiinterface.StreamChunk, <-chan error) {
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
			errChan <- fmt.Errorf("自定义端点 API 调用失败: %w", err)
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
func (c *CustomClient) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(c.model),
		Input: req.Texts,
	})

	if err != nil {
		return nil, fmt.Errorf("自定义端点 Embedding 调用失败: %w", err)
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
func (c *CustomClient) Name() string {
	return "custom"
}

// Close 关闭客户端
func (c *CustomClient) Close() error {
	return nil
}

// OllamaResponse Ollama API 响应
type OllamaResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done            bool `json:"done"`
	PromptEvalCount int  `json:"prompt_eval_count"`
	EvalCount       int  `json:"eval_count"`
}


