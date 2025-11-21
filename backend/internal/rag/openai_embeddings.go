package rag

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIEmbeddingProvider OpenAI向量化服务提供者
type OpenAIEmbeddingProvider struct {
	client *openai.Client
	model  string // 默认使用 text-embedding-3-small
}

// NewOpenAIEmbeddingProvider 创建OpenAI向量化提供者
func NewOpenAIEmbeddingProvider(apiKey string, model string) *OpenAIEmbeddingProvider {
	client := openai.NewClient(apiKey)

	// 如果未指定模型,使用默认模型
	if model == "" {
		model = string(openai.SmallEmbedding3) // text-embedding-3-small (显式转换为string)
	}

	return &OpenAIEmbeddingProvider{
		client: client,
		model:  model,
	}
}

// Embed 将文本转换为向量
// ctx: 上下文
// text: 要向量化的文本
// 返回: 向量(float32数组)和错误
func (p *OpenAIEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("文本不能为空")
	}

	// 调用OpenAI API
	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(p.model),
	})

	if err != nil {
		return nil, fmt.Errorf("调用OpenAI Embeddings API失败: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("OpenAI API返回空向量")
	}

	return resp.Data[0].Embedding, nil
}

// EmbedBatch 批量向量化文本
// ctx: 上下文
// texts: 要向量化的文本列表
// 返回: 向量列表和错误
func (p *OpenAIEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// OpenAI API 限制每次请求最多2048个输入
	// 如果超过限制,分批处理
	const batchSize = 2048
	allEmbeddings := make([][]float32, 0, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := p.embedBatchInternal(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("批量向量化失败(batch %d-%d): %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// embedBatchInternal 内部批量向量化方法
func (p *OpenAIEmbeddingProvider) embedBatchInternal(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: openai.EmbeddingModel(p.model),
	})

	if err != nil {
		return nil, fmt.Errorf("调用OpenAI Embeddings API失败: %w", err)
	}

	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("OpenAI API返回向量数量不匹配: 期望%d, 实际%d", len(texts), len(resp.Data))
	}

	// 提取向量
	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// GetDimension 获取向量维度
func (p *OpenAIEmbeddingProvider) GetDimension() int {
	// text-embedding-3-small: 1536维
	// text-embedding-3-large: 3072维
	// text-embedding-ada-002: 1536维
	switch p.model {
	case string(openai.LargeEmbedding3):
		return 3072
	case string(openai.SmallEmbedding3), string(openai.AdaEmbeddingV2):
		return 1536
	default:
		return 1536 // 默认维度
	}
}

// GetModel 获取当前使用的模型
func (p *OpenAIEmbeddingProvider) GetModel() string {
	return p.model
}

// GetProviderName 获取提供商名称
func (p *OpenAIEmbeddingProvider) GetProviderName() string {
	return "openai"
}
