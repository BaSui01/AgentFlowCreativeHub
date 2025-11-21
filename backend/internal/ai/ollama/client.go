package ollama

import (
	"backend/pkg/aiinterface"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	
)

// OllamaClient Ollama 本地模型客户端
type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewClient 创建 Ollama 客户端
func NewClient(config *aiinterface.ClientConfig) (aiinterface.ModelClient, error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second // Ollama 本地推理可能较慢
	}

	return &OllamaClient{
		baseURL: baseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// ChatCompletion 对话补全（非流式）
func (c *OllamaClient) ChatCompletion(ctx context.Context, req *aiinterface.ChatCompletionRequest) (*aiinterface.ChatCompletionResponse, error) {
	// Ollama API 请求格式
	ollamaReq := map[string]any{
		"model": c.model,
		"messages": req.Messages,
		"stream": false,
		"options": map[string]any{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
		},
	}

	if req.MaxTokens > 0 {
		ollamaReq["options"].(map[string]any)["num_predict"] = req.MaxTokens
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Ollama API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &aiinterface.ChatCompletionResponse{
		ID:      "ollama-" + time.Now().Format("20060102150405"),
		Model:   c.model,
		Content: ollamaResp.Message.Content,
		Usage: aiinterface.Usage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

// ChatCompletionStream 对话补全（流式）
func (c *OllamaClient) ChatCompletionStream(ctx context.Context, req *aiinterface.ChatCompletionRequest) (<-chan aiinterface.StreamChunk, <-chan error) {
	chunkChan := make(chan aiinterface.StreamChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		ollamaReq := map[string]any{
			"model":    c.model,
			"messages": req.Messages,
			"stream":   true,
			"options": map[string]any{
				"temperature": req.Temperature,
				"top_p":       req.TopP,
			},
		}

		if req.MaxTokens > 0 {
			ollamaReq["options"].(map[string]any)["num_predict"] = req.MaxTokens
		}

		body, err := json.Marshal(ollamaReq)
		if err != nil {
			errChan <- fmt.Errorf("序列化请求失败: %w", err)
			return
		}

		url := fmt.Sprintf("%s/api/chat", c.baseURL)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			errChan <- fmt.Errorf("创建 HTTP 请求失败: %w", err)
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errChan <- fmt.Errorf("Ollama API 调用失败: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
			return
		}

		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk OllamaStreamChunk
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					break
				}
				errChan <- fmt.Errorf("解析流式响应失败: %w", err)
				return
			}

			chunkChan <- aiinterface.StreamChunk{
				ID:      "ollama-" + time.Now().Format("20060102150405"),
				Model:   c.model,
				Content: chunk.Message.Content,
				Done:    chunk.Done,
			}

			if chunk.Done {
				break
			}
		}
	}()

	return chunkChan, errChan
}

// Embedding 文本向量化
func (c *OllamaClient) Embedding(ctx context.Context, req *aiinterface.EmbeddingRequest) (*aiinterface.EmbeddingResponse, error) {
	embeddings := make([][]float64, 0, len(req.Texts))

	for _, text := range req.Texts {
		ollamaReq := map[string]any{
			"model":  c.model,
			"prompt": text,
		}

		body, err := json.Marshal(ollamaReq)
		if err != nil {
			return nil, fmt.Errorf("序列化请求失败: %w", err)
		}

		url := fmt.Sprintf("%s/api/embeddings", c.baseURL)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("Ollama API 调用失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var embedResp struct {
			Embedding []float64 `json:"embedding"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		embeddings = append(embeddings, embedResp.Embedding)
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
func (c *OllamaClient) Name() string {
	return "ollama"
}

// Close 关闭客户端
func (c *OllamaClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// OllamaResponse Ollama API 响应
type OllamaResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done            bool  `json:"done"`
	PromptEvalCount int   `json:"prompt_eval_count"`
	EvalCount       int   `json:"eval_count"`
}

// OllamaStreamChunk Ollama 流式响应块
type OllamaStreamChunk struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// 常用 Ollama 模型
var DefaultOllamaModels = []OllamaModelInfo{
	{
		ID:            "llama3.1:70b",
		Name:          "Llama 3.1 70B",
		MaxTokens:     4096,
		ContextWindow: 131072,
	},
	{
		ID:            "llama3.1:8b",
		Name:          "Llama 3.1 8B",
		MaxTokens:     4096,
		ContextWindow: 131072,
	},
	{
		ID:            "qwen2.5:7b",
		Name:          "Qwen 2.5 7B",
		MaxTokens:     4096,
		ContextWindow: 32768,
	},
	{
		ID:            "mistral:7b",
		Name:          "Mistral 7B",
		MaxTokens:     4096,
		ContextWindow: 32768,
	},
}

// OllamaModelInfo Ollama 模型信息
type OllamaModelInfo struct {
	ID            string
	Name          string
	MaxTokens     int
	ContextWindow int
}

