package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"backend/internal/ai/anthropic"
	"backend/internal/ai/azure"
	"backend/internal/ai/custom"
	"backend/internal/ai/google"
	"backend/internal/ai/ollama"
	"backend/internal/ai/openai"
	modelspkg "backend/internal/models"
	"backend/internal/security"

	"gorm.io/gorm"
)

// ClientFactory 模型客户端工厂
type ClientFactory struct {
	db      *gorm.DB
	clients map[string]ModelClient // 客户端缓存
	mu      sync.RWMutex
	logger  ModelCallLogger
}

// NewClientFactory 创建客户端工厂
func NewClientFactory(db *gorm.DB, logger ModelCallLogger) *ClientFactory {
	return &ClientFactory{
		db:      db,
		clients: make(map[string]ModelClient),
		logger:  logger,
	}
}

// GetClient 获取模型客户端
// 从数据库加载模型配置并创建对应的客户端
func (f *ClientFactory) GetClient(ctx context.Context, tenantID, modelID string) (ModelClient, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%s", tenantID, modelID)
	f.mu.RLock()
	if client, ok := f.clients[cacheKey]; ok {
		f.mu.RUnlock()
		return client, nil
	}
	f.mu.RUnlock()

	// 从数据库加载模型配置
	var model modelspkg.Model
	if err := f.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", modelID, tenantID).
		First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("模型不存在: %s", modelID)
		}
		return nil, fmt.Errorf("查询模型失败: %w", err)
	}

	// 创建客户端配置
	config := &ClientConfig{
		Provider:   model.Provider,
		Model:      model.ModelIdentifier,
		MaxRetries: 3,
		Timeout:    60,
		BaseURL:    model.BaseURL, // 优先使用数据库配置的 BaseURL
	}

	// 解析凭证（支持数据库存储与环境变量）
	apiKey, overrideBaseURL, extraHeaders := f.resolveCredentials(ctx, &model)
	config.APIKey = apiKey
	if overrideBaseURL != "" {
		config.BaseURL = overrideBaseURL
	}
	if len(extraHeaders) > 0 {
		if config.Extra == nil {
			config.Extra = make(map[string]any)
		}
		config.Extra["headers"] = extraHeaders
	}

	// 如果 DB 中没有配置 BaseURL，则根据 Provider 加载默认值
	if config.BaseURL == "" {
		switch model.Provider {
		case "openai":
			config.BaseURL = "https://api.openai.com/v1"
		case "anthropic":
			config.BaseURL = "https://api.anthropic.com"
		case "google", "gemini":
			config.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
		case "azure":
			config.BaseURL = getAPIKey("AZURE_OPENAI_ENDPOINT")
		case "deepseek":
			config.BaseURL = "https://api.deepseek.com"
		case "qwen":
			config.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		case "ollama":
			config.BaseURL = "http://localhost:11434"
		}
	}

	// 兼容逻辑：如果 config.APIKey 仍为空，尝试从环境变量加载 (为了向后兼容)
	// (已在 resolveCredentials 中处理)

	// 处理 Azure 特殊逻辑
	if model.Provider == "azure" && model.APIVersion != "" {
		if config.Extra == nil {
			config.Extra = make(map[string]any)
		}
		config.Extra["api_version"] = model.APIVersion
	}

	// 创建客户端
	client, err := f.createClient(config, model.APIFormat)
	if err != nil {
		return nil, fmt.Errorf("创建客户端失败: %w", err)
	}

	// 如果启用了日志记录，包装客户端
	if f.logger != nil {
		client = NewLoggingClient(client, f.logger, tenantID, modelID, &model)
	}

	// 缓存客户端
	f.mu.Lock()
	f.clients[cacheKey] = client
	f.mu.Unlock()

	return client, nil
}

// createClient 创建客户端
// apiFormat: 协议格式 (openai, claude, etc.), 默认为 openai
func (f *ClientFactory) createClient(config *ClientConfig, apiFormat string) (ModelClient, error) {
	// 优先使用 APIFormat 决定驱动
	// 如果 apiFormat 为空，回退到 Provider 映射
	format := apiFormat
	if format == "" {
		format = "openai" // 默认
		switch config.Provider {
		case "anthropic":
			format = "claude"
		case "google", "gemini":
			format = "gemini"
		case "ollama":
			format = "ollama"
		}
	}

	switch format {
	case "openai":
		return openai.NewClient(config)
	case "claude", "anthropic":
		return anthropic.NewClient(config)
	case "gemini", "google":
		return google.NewClient(config)
	case "azure":
		return azure.NewClient(config)
	case "ollama":
		return ollama.NewClient(config)
	case "deepseek":
		// Deepseek 兼容 OpenAI 协议
		return openai.NewClient(config)
	case "qwen":
		// Qwen 兼容 OpenAI 协议
		return openai.NewClient(config)
	case "custom":
		// Custom 驱动
		return custom.NewClient(config)
	default:
		// 默认尝试 OpenAI 兼容
		return openai.NewClient(config)
	}
}

// ClearCache 清除客户端缓存
func (f *ClientFactory) ClearCache() {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 关闭所有客户端
	for _, client := range f.clients {
		_ = client.Close()
	}

	f.clients = make(map[string]ModelClient)
}

// ClearCacheForModel 清除指定模型的缓存
func (f *ClientFactory) ClearCacheForModel(tenantID, modelID string) {
	cacheKey := fmt.Sprintf("%s:%s", tenantID, modelID)
	f.mu.Lock()
	defer f.mu.Unlock()

	if client, ok := f.clients[cacheKey]; ok {
		_ = client.Close()
		delete(f.clients, cacheKey)
	}
}

// resolveCredentials 解析凭证
// 返回值依次为 API Key、BaseURL 覆盖值、附加请求头
func (f *ClientFactory) resolveCredentials(ctx context.Context, model *modelspkg.Model) (string, string, map[string]string) {
	if key, baseURL, headers := f.loadDefaultCredential(ctx, model); key != "" {
		return key, baseURL, headers
	}

	// 1. 明文/临时 Key
	if apiKey, ok := model.Capabilities["api_key"].(string); ok && strings.TrimSpace(apiKey) != "" {
		return strings.TrimSpace(apiKey), "", nil
	}

	// 2. 通过指定的环境变量/凭证提供者键名
	if envKey, ok := model.Capabilities["api_key_env"].(string); ok && strings.TrimSpace(envKey) != "" {
		if val := strings.TrimSpace(getAPIKey(envKey)); val != "" {
			return val, "", nil
		}
	}

	// 3a. 外部密钥名（例如密钥管理器），与 api_key_env 语义相同但更显式
	if secretKey, ok := model.Capabilities["encrypted_key_env"].(string); ok && strings.TrimSpace(secretKey) != "" {
		if val := strings.TrimSpace(getAPIKey(secretKey)); val != "" {
			return val, "", nil
		}
	}

	// 3b. Base64 编码密钥（轻量“加密”占位，仍需上层确保安全来源）
	if encKey, ok := model.Capabilities["encrypted_api_key"].(string); ok && strings.TrimSpace(encKey) != "" {
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encKey)); err == nil && len(decoded) > 0 {
			return string(decoded), "", nil
		}
	}

	// 4. Provider 默认映射（回退，兼容旧配置）
	envKeyMap := map[string]string{
		"openai":    "OPENAI_API_KEY",
		"anthropic": "ANTHROPIC_API_KEY",
		"google":    "GOOGLE_API_KEY",
		"gemini":    "GEMINI_API_KEY",
		"azure":     "AZURE_OPENAI_API_KEY",
		"deepseek":  "DEEPSEEK_API_KEY",
		"qwen":      "QWEN_API_KEY",
		"custom":    "CUSTOM_API_KEY",
	}
	if envVar, ok := envKeyMap[model.Provider]; ok {
		if val := getAPIKey(envVar); strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val), "", nil
		}
		// Gemini 兼容 GOOGLE_API_KEY，以便复用既有配置
		if model.Provider == "gemini" {
			if val := strings.TrimSpace(getAPIKey("GOOGLE_API_KEY")); val != "" {
				return val, "", nil
			}
		}
	}

	return "", "", nil
}

func (f *ClientFactory) loadDefaultCredential(ctx context.Context, model *modelspkg.Model) (string, string, map[string]string) {
	if model.DefaultCredentialID == "" {
		return "", "", nil
	}
	var cred modelspkg.ModelCredential
	if err := f.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", model.DefaultCredentialID, model.TenantID).
		First(&cred).Error; err != nil {
		return "", "", nil
	}
	secret, err := security.DecryptSecret(cred.Ciphertext)
	if err != nil || strings.TrimSpace(secret) == "" {
		return "", "", nil
	}
	return strings.TrimSpace(secret), strings.TrimSpace(cred.BaseURL), coerceHeaders(cred.ExtraHeaders)
}

func coerceHeaders(raw map[string]any) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]string)
	for k, v := range raw {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		result[key] = fmt.Sprint(v)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
