package google

import (
	"backend/pkg/aiinterface"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	
	"backend/internal/ai/converters"
)

// GeminiClient Google Gemini 客户端
type GeminiClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	maxRetries int
}

// NewClient 创建 Gemini 客户端
func NewClient(config *aiinterface.ClientConfig) (aiinterface.ModelClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Gemini API Key 不能为空")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &GeminiClient{
		apiKey:     config.APIKey,
		baseURL:    baseURL,
		model:      config.Model,
		maxRetries: maxRetries,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// ChatCompletion 对话补全（非流式）
func (c *GeminiClient) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
	// 1. 转换请求格式
	converter := converters.NewOpenAIToGeminiConverter()
	geminiReq, err := converter.ConvertRequest(req)
	if err != nil {
		return nil, fmt.Errorf("转换请求格式失败: %w", err)
	}

	// 2. 构建 API 请求
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 3. 发送请求（带重试）
	var resp *http.Response
	var lastErr error

	for i := 0; i < c.maxRetries; i++ {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err = c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * time.Second) // 指数退避
			continue
		}

		if resp.StatusCode == http.StatusOK {
			break
		}

		// 读取错误响应
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))

		// 只对 5xx 错误重试
		if resp.StatusCode >= 500 {
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		// 4xx 错误不重试
		return nil, lastErr
	}

	if lastErr != nil {
		return nil, fmt.Errorf("Gemini API 调用失败: %w", lastErr)
	}

	defer resp.Body.Close()

	// 4. 解析响应
	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 5. 转换为统一格式
	return c.convertResponse(&geminiResp), nil
}

// ChatCompletionStream 对话补全（流式）
func (c *GeminiClient) ChatCompletionStream(ctx context.Context, req *aiinterface.ChatCompletionRequest) (<-chan aiinterface.StreamChunk, <-chan error) {
	chunkChan := make(chan aiinterface.StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		// 1. 转换请求格式
		converter := converters.NewOpenAIToGeminiConverter()
		geminiReq, err := converter.ConvertRequest(req)
		if err != nil {
			errChan <- fmt.Errorf("转换请求格式失败: %w", err)
			return
		}

		// 2. 构建流式 API 请求
		url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s", c.baseURL, c.model, c.apiKey)

		body, err := json.Marshal(geminiReq)
		if err != nil {
			errChan <- fmt.Errorf("序列化请求失败: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			errChan <- fmt.Errorf("创建 HTTP 请求失败: %w", err)
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errChan <- fmt.Errorf("HTTP 请求失败: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
			return
		}

		// 3. 读取流式响应
		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk GeminiStreamChunk
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					break
				}
				errChan <- fmt.Errorf("解析流式响应失败: %w", err)
				return
			}

			// 转换为统一格式
			chunkChan <- aiinterface.StreamChunk{
				ID:      chunk.ID,
				Model:   c.model,
				Content: c.extractContent(&chunk),
				Done:    false,
			}
		}

		// 发送结束标记
		chunkChan <- aiinterface.StreamChunk{
			ID:    "done",
			Model: c.model,
			Done:  true,
		}
	}()

	return chunkChan, errChan
}

// Embedding 文本向量化
func (c *GeminiClient) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	// Gemini 的 embedding API
	url := fmt.Sprintf("%s/models/embedding-001:embedContent?key=%s", c.baseURL, c.apiKey)

	type EmbedRequest struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	}

	embeddings := make([][]float64, 0, len(req.Texts))

	for _, text := range req.Texts {
		embedReq := EmbedRequest{}
		embedReq.Content.Parts = []struct {
			Text string `json:"text"`
		}{{Text: text}}

		body, err := json.Marshal(embedReq)
		if err != nil {
			return nil, fmt.Errorf("序列化请求失败: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("HTTP 请求失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var embedResp struct {
			Embedding struct {
				Values []float64 `json:"values"`
			} `json:"embedding"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		embeddings = append(embeddings, embedResp.Embedding.Values)
	}

	return &aiinterface.EmbeddingResponse{
		Embeddings: embeddings,
		Usage: aiinterface.Usage{
			PromptTokens: len(req.Texts) * 10, // 估算
			TotalTokens:  len(req.Texts) * 10,
		},
	}, nil
}

// Name 返回客户端名称
func (c *GeminiClient) Name() string {
	return "gemini"
}

// Close 关闭客户端
func (c *GeminiClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// 辅助方法

func (c *GeminiClient) convertResponse(resp *GeminiResponse) *aiinterface.ChatCompletionResponse {
	content := ""
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		content = resp.Candidates[0].Content.Parts[0].Text
	}

	return &aiinterface.ChatCompletionResponse{
		ID:      resp.ID,
		Model:   c.model,
		Content: content,
		Usage: aiinterface.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		},
	}
}

func (c *GeminiClient) extractContent(chunk *GeminiStreamChunk) string {
	if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
		return chunk.Candidates[0].Content.Parts[0].Text
	}
	return ""
}

