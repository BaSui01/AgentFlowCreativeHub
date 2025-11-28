package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewCustomEmbeddingProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *CustomEmbeddingConfig
		wantErr bool
	}{
		{
			name: "基础配置",
			config: &CustomEmbeddingConfig{
				Endpoint: "http://localhost:8080/v1/embeddings",
			},
			wantErr: false,
		},
		{
			name: "完整配置",
			config: &CustomEmbeddingConfig{
				Name:         "my-embedding",
				Endpoint:     "http://localhost:8080/v1/embeddings",
				Model:        "text-embedding-3-small",
				Dimension:    1536,
				MaxBatchSize: 50,
				AuthType:     AuthTypeAPIKey,
				APIKey:       "test-key",
				Timeout:      30 * time.Second,
			},
			wantErr: false,
		},
		{
			name:    "缺少端点",
			config:  &CustomEmbeddingConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewCustomEmbeddingProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCustomEmbeddingProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("NewCustomEmbeddingProvider() returned nil provider")
			}
		})
	}
}

func TestCustomEmbeddingProvider_Embed(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Authorization header")
		}

		// 返回模拟响应
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"index":     0,
					"embedding": []float64{0.1, 0.2, 0.3, 0.4, 0.5},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewCustomEmbeddingProvider(&CustomEmbeddingConfig{
		Endpoint:  server.URL,
		AuthType:  AuthTypeAPIKey,
		APIKey:    "test-key",
		Dimension: 5,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	embedding, err := provider.Embed(context.Background(), "测试文本")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(embedding) != 5 {
		t.Errorf("Expected 5 dimensions, got %d", len(embedding))
	}

	expected := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	for i, v := range embedding {
		if v != expected[i] {
			t.Errorf("embedding[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestCustomEmbeddingProvider_EmbedBatch(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		inputs := reqBody["input"].([]interface{})
		data := make([]map[string]interface{}, len(inputs))
		for i := range inputs {
			data[i] = map[string]interface{}{
				"index":     i,
				"embedding": []float64{float64(i) * 0.1, float64(i) * 0.2},
			}
		}

		resp := map[string]interface{}{"data": data}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, _ := NewCustomEmbeddingProvider(&CustomEmbeddingConfig{
		Endpoint:     server.URL,
		MaxBatchSize: 2,
		Dimension:    2,
	})

	// 测试需要分批的情况
	texts := []string{"文本1", "文本2", "文本3"}
	embeddings, err := provider.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(embeddings) != 3 {
		t.Errorf("Expected 3 embeddings, got %d", len(embeddings))
	}

	// 验证分批调用 (3 条文本，批量大小 2，应该调用 2 次)
	if callCount != 2 {
		t.Errorf("Expected 2 API calls, got %d", callCount)
	}
}

func TestCustomEmbeddingProvider_AuthTypes(t *testing.T) {
	tests := []struct {
		name           string
		authType       AuthType
		apiKey         string
		basicUser      string
		basicPassword  string
		customHeaders  map[string]string
		expectedHeader string
		expectedValue  string
	}{
		{
			name:           "API Key 认证",
			authType:       AuthTypeAPIKey,
			apiKey:         "my-api-key",
			expectedHeader: "Authorization",
			expectedValue:  "Bearer my-api-key",
		},
		{
			name:           "Basic 认证",
			authType:       AuthTypeBasic,
			basicUser:      "user",
			basicPassword:  "pass",
			expectedHeader: "Authorization",
			expectedValue:  "Basic dXNlcjpwYXNz", // base64(user:pass)
		},
		{
			name:           "自定义头",
			authType:       AuthTypeCustom,
			customHeaders:  map[string]string{"X-API-Key": "custom-key"},
			expectedHeader: "X-API-Key",
			expectedValue:  "custom-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get(tt.expectedHeader) != tt.expectedValue {
					t.Errorf("Header %s = %s, want %s", tt.expectedHeader, r.Header.Get(tt.expectedHeader), tt.expectedValue)
				}

				resp := map[string]interface{}{
					"data": []map[string]interface{}{
						{"index": 0, "embedding": []float64{0.1}},
					},
				}
				json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			provider, _ := NewCustomEmbeddingProvider(&CustomEmbeddingConfig{
				Endpoint:      server.URL,
				AuthType:      tt.authType,
				APIKey:        tt.apiKey,
				BasicUser:     tt.basicUser,
				BasicPassword: tt.basicPassword,
				CustomHeaders: tt.customHeaders,
			})

			_, err := provider.Embed(context.Background(), "test")
			if err != nil {
				t.Errorf("Embed failed: %v", err)
			}
		})
	}
}

func TestCustomEmbeddingProvider_CustomFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// 验证自定义请求格式
		if _, ok := reqBody["texts"]; !ok {
			t.Error("Expected 'texts' field in request")
		}
		if _, ok := reqBody["model_name"]; !ok {
			t.Error("Expected 'model_name' field in request")
		}

		// 返回自定义响应格式
		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"idx":    0,
					"vector": []float64{0.1, 0.2, 0.3},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, _ := NewCustomEmbeddingProvider(&CustomEmbeddingConfig{
		Endpoint:  server.URL,
		Dimension: 3,
		RequestFormat: &RequestFormat{
			InputField: "texts",
			ModelField: "model_name",
			WrapArray:  true,
		},
		ResponseFormat: &ResponseFormat{
			DataField:      "results",
			EmbeddingField: "vector",
			IndexField:     "idx",
		},
	})

	embedding, err := provider.Embed(context.Background(), "测试")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(embedding) != 3 {
		t.Errorf("Expected 3 dimensions, got %d", len(embedding))
	}
}

func TestCustomEmbeddingProvider_GetMethods(t *testing.T) {
	provider, _ := NewCustomEmbeddingProvider(&CustomEmbeddingConfig{
		Name:      "test-provider",
		Endpoint:  "http://localhost/embeddings",
		Model:     "test-model",
		Dimension: 768,
	})

	if provider.GetProviderName() != "test-provider" {
		t.Errorf("GetProviderName() = %s, want test-provider", provider.GetProviderName())
	}
	if provider.GetModel() != "test-model" {
		t.Errorf("GetModel() = %s, want test-model", provider.GetModel())
	}
	if provider.GetDimension() != 768 {
		t.Errorf("GetDimension() = %d, want 768", provider.GetDimension())
	}
}

func TestCustomEmbeddingProvider_SetMethods(t *testing.T) {
	provider, _ := NewCustomEmbeddingProvider(&CustomEmbeddingConfig{
		Endpoint: "http://localhost/embeddings",
	})

	provider.SetModel("new-model")
	if provider.GetModel() != "new-model" {
		t.Errorf("SetModel failed")
	}

	provider.SetEndpoint("http://new-endpoint/embeddings")
	if provider.endpoint != "http://new-endpoint/embeddings" {
		t.Errorf("SetEndpoint failed")
	}

	provider.SetDimension(1024)
	if provider.GetDimension() != 1024 {
		t.Errorf("SetDimension failed")
	}
}

func TestCustomEmbeddingProvider_EmptyInput(t *testing.T) {
	provider, _ := NewCustomEmbeddingProvider(&CustomEmbeddingConfig{
		Endpoint: "http://localhost/embeddings",
	})

	result, err := provider.EmbedBatch(context.Background(), []string{})
	if err != nil {
		t.Errorf("EmbedBatch with empty input should not error: %v", err)
	}
	if result != nil {
		t.Errorf("EmbedBatch with empty input should return nil, got %v", result)
	}
}

func TestCustomEmbeddingProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	provider, _ := NewCustomEmbeddingProvider(&CustomEmbeddingConfig{
		Endpoint: server.URL,
	})

	_, err := provider.Embed(context.Background(), "test")
	if err == nil {
		t.Error("Expected error for API error response")
	}
}
