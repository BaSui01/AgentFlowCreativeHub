package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// WenxinEmbeddingProvider 文心一言 Embedding 提供者
type WenxinEmbeddingProvider struct {
	apiKey      string
	secretKey   string
	model       string
	baseURL     string
	httpClient  *http.Client
	accessToken string
	tokenExpiry time.Time
	mu          sync.RWMutex
}

// WenxinEmbeddingConfig 文心一言配置
type WenxinEmbeddingConfig struct {
	APIKey    string
	SecretKey string
	Model     string // embedding-v1, bge-large-zh, tao-8k
	Timeout   time.Duration
}

// NewWenxinEmbeddingProvider 创建文心一言 Embedding 提供者
func NewWenxinEmbeddingProvider(config *WenxinEmbeddingConfig) *WenxinEmbeddingProvider {
	if config.Model == "" {
		config.Model = "embedding-v1"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &WenxinEmbeddingProvider{
		apiKey:    config.APIKey,
		secretKey: config.SecretKey,
		model:     config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// getAccessToken 获取访问令牌
func (p *WenxinEmbeddingProvider) getAccessToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		token := p.accessToken
		p.mu.RUnlock()
		return token, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// 双重检查
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	// 请求新令牌
	url := fmt.Sprintf("https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s",
		p.apiKey, p.secretKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("token error: %s", result.Error)
	}

	p.accessToken = result.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second) // 提前 5 分钟过期

	return p.accessToken, nil
}

// wenxinEmbeddingRequest 请求结构
type wenxinEmbeddingRequest struct {
	Input []string `json:"input"`
}

// wenxinEmbeddingResponse 响应结构
type wenxinEmbeddingResponse struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Data   []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

// Embed 单条向量化
func (p *WenxinEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
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
func (p *WenxinEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 文心一言单次最多 16 条
	const maxBatchSize = 16
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
func (p *WenxinEmbeddingProvider) embedBatchInternal(ctx context.Context, texts []string) ([][]float32, error) {
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// 构建 URL
	url := p.getEndpointURL(token)

	reqBody := wenxinEmbeddingRequest{Input: texts}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result wenxinEmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.ErrorCode != 0 {
		return nil, fmt.Errorf("API error %d: %s", result.ErrorCode, result.ErrorMsg)
	}

	// 按索引排序
	embeddings := make([][]float32, len(texts))
	for _, emb := range result.Data {
		if emb.Index < len(embeddings) {
			embeddings[emb.Index] = emb.Embedding
		}
	}

	return embeddings, nil
}

// getEndpointURL 获取端点 URL
func (p *WenxinEmbeddingProvider) getEndpointURL(token string) string {
	base := "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/embeddings"
	switch p.model {
	case "embedding-v1":
		return fmt.Sprintf("%s/embedding-v1?access_token=%s", base, token)
	case "bge-large-zh":
		return fmt.Sprintf("%s/bge_large_zh?access_token=%s", base, token)
	case "bge-large-en":
		return fmt.Sprintf("%s/bge_large_en?access_token=%s", base, token)
	case "tao-8k":
		return fmt.Sprintf("%s/tao_8k?access_token=%s", base, token)
	default:
		return fmt.Sprintf("%s/embedding-v1?access_token=%s", base, token)
	}
}

// GetModel 获取模型名称
func (p *WenxinEmbeddingProvider) GetModel() string {
	return p.model
}

// GetProviderName 获取提供者名称
func (p *WenxinEmbeddingProvider) GetProviderName() string {
	return "wenxin"
}

// GetDimension 获取向量维度
func (p *WenxinEmbeddingProvider) GetDimension() int {
	switch p.model {
	case "embedding-v1":
		return 384
	case "bge-large-zh":
		return 1024
	case "bge-large-en":
		return 1024
	case "tao-8k":
		return 1024
	default:
		return 384
	}
}
