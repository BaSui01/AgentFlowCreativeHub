package embedding

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AuthType 认证类型
type AuthType string

const (
	AuthTypeNone        AuthType = "none"         // 无认证
	AuthTypeAPIKey      AuthType = "api_key"      // API Key (Authorization: Bearer xxx)
	AuthTypeBasic       AuthType = "basic"        // Basic Auth
	AuthTypeCustom      AuthType = "custom"       // 自定义头
)

// CustomEmbeddingProvider 自定义 Embedding 提供者
// 支持任意兼容 OpenAI 格式或自定义格式的 Embedding API
type CustomEmbeddingProvider struct {
	name           string
	endpoint       string
	model          string
	dimension      int
	maxBatchSize   int
	authType       AuthType
	apiKey         string
	basicUser      string
	basicPassword  string
	customHeaders  map[string]string
	httpClient     *http.Client
	requestFormat  RequestFormat
	responseFormat ResponseFormat
}

// RequestFormat 请求格式配置
type RequestFormat struct {
	InputField string // 输入文本字段名，默认 "input"
	ModelField string // 模型字段名，默认 "model"
	WrapArray  bool   // 是否将单个文本包装成数组
}

// ResponseFormat 响应格式配置
type ResponseFormat struct {
	DataField      string // 数据数组字段名，默认 "data"
	EmbeddingField string // 向量字段名，默认 "embedding"
	IndexField     string // 索引字段名，默认 "index"
}

// CustomEmbeddingConfig 自定义 Embedding 配置
type CustomEmbeddingConfig struct {
	Name          string            // 提供者名称
	Endpoint      string            // API 端点
	Model         string            // 模型名称
	Dimension     int               // 向量维度
	MaxBatchSize  int               // 最大批量大小
	AuthType      AuthType          // 认证类型
	APIKey        string            // API Key
	BasicUser     string            // Basic Auth 用户名
	BasicPassword string            // Basic Auth 密码
	CustomHeaders map[string]string // 自定义请求头
	Timeout       time.Duration     // 超时时间
	RequestFormat  *RequestFormat   // 请求格式
	ResponseFormat *ResponseFormat  // 响应格式
}

// NewCustomEmbeddingProvider 创建自定义 Embedding 提供者
func NewCustomEmbeddingProvider(config *CustomEmbeddingConfig) (*CustomEmbeddingProvider, error) {
	if config.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if config.Name == "" {
		config.Name = "custom"
	}
	if config.Model == "" {
		config.Model = "default"
	}
	if config.Dimension <= 0 {
		config.Dimension = 1536 // OpenAI 默认维度
	}
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 100 // 默认批量大小
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.AuthType == "" {
		config.AuthType = AuthTypeNone
	}

	// 默认请求格式 (OpenAI 兼容)
	reqFmt := RequestFormat{
		InputField: "input",
		ModelField: "model",
		WrapArray:  true,
	}
	if config.RequestFormat != nil {
		if config.RequestFormat.InputField != "" {
			reqFmt.InputField = config.RequestFormat.InputField
		}
		if config.RequestFormat.ModelField != "" {
			reqFmt.ModelField = config.RequestFormat.ModelField
		}
		reqFmt.WrapArray = config.RequestFormat.WrapArray
	}

	// 默认响应格式 (OpenAI 兼容)
	respFmt := ResponseFormat{
		DataField:      "data",
		EmbeddingField: "embedding",
		IndexField:     "index",
	}
	if config.ResponseFormat != nil {
		if config.ResponseFormat.DataField != "" {
			respFmt.DataField = config.ResponseFormat.DataField
		}
		if config.ResponseFormat.EmbeddingField != "" {
			respFmt.EmbeddingField = config.ResponseFormat.EmbeddingField
		}
		if config.ResponseFormat.IndexField != "" {
			respFmt.IndexField = config.ResponseFormat.IndexField
		}
	}

	return &CustomEmbeddingProvider{
		name:          config.Name,
		endpoint:      config.Endpoint,
		model:         config.Model,
		dimension:     config.Dimension,
		maxBatchSize:  config.MaxBatchSize,
		authType:      config.AuthType,
		apiKey:        config.APIKey,
		basicUser:     config.BasicUser,
		basicPassword: config.BasicPassword,
		customHeaders: config.CustomHeaders,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		requestFormat:  reqFmt,
		responseFormat: respFmt,
	}, nil
}

// Embed 单条向量化
func (p *CustomEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
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
func (p *CustomEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += p.maxBatchSize {
		end := i + p.maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := p.embedBatchInternal(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", i/p.maxBatchSize, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// embedBatchInternal 内部批量向量化
func (p *CustomEmbeddingProvider) embedBatchInternal(ctx context.Context, texts []string) ([][]float32, error) {
	// 构建请求体
	reqBody := make(map[string]interface{})
	reqBody[p.requestFormat.ModelField] = p.model
	if p.requestFormat.WrapArray {
		reqBody[p.requestFormat.InputField] = texts
	} else if len(texts) == 1 {
		reqBody[p.requestFormat.InputField] = texts[0]
	} else {
		reqBody[p.requestFormat.InputField] = texts
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.setAuthHeaders(req)

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

	return p.parseResponse(respBody, len(texts))
}

// setAuthHeaders 设置认证头
func (p *CustomEmbeddingProvider) setAuthHeaders(req *http.Request) {
	switch p.authType {
	case AuthTypeAPIKey:
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	case AuthTypeBasic:
		auth := base64.StdEncoding.EncodeToString([]byte(p.basicUser + ":" + p.basicPassword))
		req.Header.Set("Authorization", "Basic "+auth)
	case AuthTypeCustom:
		for k, v := range p.customHeaders {
			req.Header.Set(k, v)
		}
	}
}

// parseResponse 解析响应
func (p *CustomEmbeddingProvider) parseResponse(respBody []byte, expectedCount int) ([][]float32, error) {
	var rawResp map[string]interface{}
	if err := json.Unmarshal(respBody, &rawResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 获取数据数组
	dataRaw, ok := rawResp[p.responseFormat.DataField]
	if !ok {
		return nil, fmt.Errorf("missing field '%s' in response", p.responseFormat.DataField)
	}

	dataArray, ok := dataRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("field '%s' is not an array", p.responseFormat.DataField)
	}

	embeddings := make([][]float32, expectedCount)
	for _, item := range dataArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// 获取索引
		index := 0
		if idxRaw, ok := itemMap[p.responseFormat.IndexField]; ok {
			if idxFloat, ok := idxRaw.(float64); ok {
				index = int(idxFloat)
			}
		}

		// 获取向量
		embRaw, ok := itemMap[p.responseFormat.EmbeddingField]
		if !ok {
			continue
		}

		embArray, ok := embRaw.([]interface{})
		if !ok {
			continue
		}

		embedding := make([]float32, len(embArray))
		for i, v := range embArray {
			if f, ok := v.(float64); ok {
				embedding[i] = float32(f)
			}
		}

		if index >= 0 && index < expectedCount {
			embeddings[index] = embedding
		}
	}

	// 检查是否所有结果都有值
	for i, emb := range embeddings {
		if emb == nil {
			return nil, fmt.Errorf("missing embedding for index %d", i)
		}
	}

	return embeddings, nil
}

// GetModel 获取模型名称
func (p *CustomEmbeddingProvider) GetModel() string {
	return p.model
}

// GetProviderName 获取提供者名称
func (p *CustomEmbeddingProvider) GetProviderName() string {
	return p.name
}

// GetDimension 获取向量维度
func (p *CustomEmbeddingProvider) GetDimension() int {
	return p.dimension
}

// SetModel 动态设置模型
func (p *CustomEmbeddingProvider) SetModel(model string) {
	p.model = model
}

// SetEndpoint 动态设置端点
func (p *CustomEmbeddingProvider) SetEndpoint(endpoint string) {
	p.endpoint = endpoint
}

// SetDimension 动态设置维度
func (p *CustomEmbeddingProvider) SetDimension(dimension int) {
	p.dimension = dimension
}
