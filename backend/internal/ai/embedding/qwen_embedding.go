package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// QwenEmbeddingProvider 通义千问 Embedding 提供者
type QwenEmbeddingProvider struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// QwenEmbeddingConfig 通义千问配置
type QwenEmbeddingConfig struct {
	APIKey  string
	Model   string // text-embedding-v1, text-embedding-v2
	BaseURL string
	Timeout time.Duration
}

// NewQwenEmbeddingProvider 创建通义千问 Embedding 提供者
func NewQwenEmbeddingProvider(config *QwenEmbeddingConfig) *QwenEmbeddingProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding"
	}
	if config.Model == "" {
		config.Model = "text-embedding-v2"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &QwenEmbeddingProvider{
		apiKey:  config.APIKey,
		model:   config.Model,
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// qwenEmbeddingRequest 请求结构
type qwenEmbeddingRequest struct {
	Model string           `json:"model"`
	Input qwenEmbeddingInput `json:"input"`
	Parameters struct {
		TextType string `json:"text_type"` // query, document
	} `json:"parameters"`
}

type qwenEmbeddingInput struct {
	Texts []string `json:"texts"`
}

// qwenEmbeddingResponse 响应结构
type qwenEmbeddingResponse struct {
	Output struct {
		Embeddings []struct {
			TextIndex int       `json:"text_index"`
			Embedding []float32 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

// Embed 单条向量化
func (p *QwenEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return embeddings[0], nil
}

// EmbedBatch 批量向量化
func (p *QwenEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 通义千问单次最多 25 条
	const maxBatchSize = 25
	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := p.embedBatchInternal(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", i/maxBatchSize, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// embedBatchInternal 内部批量向量化
func (p *QwenEmbeddingProvider) embedBatchInternal(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := qwenEmbeddingRequest{
		Model: p.model,
		Input: qwenEmbeddingInput{Texts: texts},
	}
	reqBody.Parameters.TextType = "document"

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result qwenEmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 按索引排序
	embeddings := make([][]float32, len(texts))
	for _, emb := range result.Output.Embeddings {
		if emb.TextIndex < len(embeddings) {
			embeddings[emb.TextIndex] = emb.Embedding
		}
	}

	return embeddings, nil
}

// GetModel 获取模型名称
func (p *QwenEmbeddingProvider) GetModel() string {
	return p.model
}

// GetProviderName 获取提供者名称
func (p *QwenEmbeddingProvider) GetProviderName() string {
	return "qwen"
}

// GetDimension 获取向量维度
func (p *QwenEmbeddingProvider) GetDimension() int {
	switch p.model {
	case "text-embedding-v1":
		return 1536
	case "text-embedding-v2":
		return 1536
	default:
		return 1536
	}
}
