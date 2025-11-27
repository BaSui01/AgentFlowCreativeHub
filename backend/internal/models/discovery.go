package models

import (
	"context"
	"fmt"
	"time"

	"backend/internal/ai/deepseek"
	"backend/internal/ai/google"
	"backend/internal/ai/ollama"
	"backend/internal/ai/qwen"
	"backend/pkg/aiinterface"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ModelDiscoveryService 模型自动发现服务
type ModelDiscoveryService struct {
	db            *gorm.DB
	clientFactory aiinterface.ClientFactory
}

// NewModelDiscoveryService 创建模型发现服务
func NewModelDiscoveryService(db *gorm.DB, clientFactory aiinterface.ClientFactory) *ModelDiscoveryService {
	return &ModelDiscoveryService{
		db:            db,
		clientFactory: clientFactory,
	}
}

// SyncModelsFromProvider 从提供商同步模型列表
func (s *ModelDiscoveryService) SyncModelsFromProvider(ctx context.Context, tenantID, provider string) (int, error) {
	var models []ModelInfo
	var err error

	// 根据不同提供商获取模型列表
	switch provider {
	case "google":
		models, err = s.getGeminiModels()
	case "gemini":
		models, err = s.getGeminiModels()
	case "deepseek":
		models, err = s.getDeepSeekModels()
	case "qwen":
		models, err = s.getQwenModels()
	case "ollama":
		models, err = s.getOllamaModels()
	case "azure":
		// Azure 模型需要用户手动配置 deployment
		return 0, fmt.Errorf("Azure OpenAI 模型需要手动配置 deployment")
	case "openai":
		models, err = s.getOpenAIModels()
	case "anthropic":
		models, err = s.getAnthropicModels()
	default:
		return 0, fmt.Errorf("不支持的提供商: %s", provider)
	}

	if err != nil {
		return 0, fmt.Errorf("获取模型列表失败: %w", err)
	}

	// 批量插入/更新模型
	count := 0
	now := time.Now().UTC()

	for _, modelInfo := range models {
		// 检查模型是否已存在
		var existingModel Model
		result := s.db.WithContext(ctx).
			Where("tenant_id = ? AND provider = ? AND model_identifier = ?", tenantID, provider, modelInfo.ID).
			First(&existingModel)

		if result.Error == gorm.ErrRecordNotFound {
			// 创建新模型
			model := &Model{
				ID:                 uuid.New().String(),
				TenantID:           tenantID,
				Name:               modelInfo.Name,
				Provider:           provider,
				ModelIdentifier:    modelInfo.ID,
				Type:               modelInfo.Type,
				Category:           modelInfo.Category,
				Description:        modelInfo.Description,
				MaxTokens:          modelInfo.MaxTokens,
				ContextWindow:      modelInfo.ContextWindow,
				InputCostPer1K:     modelInfo.InputCostPer1K,
				OutputCostPer1K:    modelInfo.OutputCostPer1K,
				SupportedLanguages: []string{"zh", "en"},
				Features:           modelInfo.Features,
				APIFormat:          getAPIFormat(provider),
				IsBuiltin:          true,
				IsActive:           true,
				Status:             "active",
				LastSyncedAt:       &now,
				CreatedAt:          now,
				UpdatedAt:          now,
			}

			if err := s.db.WithContext(ctx).Create(model).Error; err != nil {
				return count, fmt.Errorf("创建模型失败: %w", err)
			}
			count++
		} else if result.Error == nil {
			// 更新现有模型
			updates := map[string]any{
				"name":               modelInfo.Name,
				"description":        modelInfo.Description,
				"max_tokens":         modelInfo.MaxTokens,
				"context_window":     modelInfo.ContextWindow,
				"input_cost_per_1k":  modelInfo.InputCostPer1K,
				"output_cost_per_1k": modelInfo.OutputCostPer1K,
				"features":           modelInfo.Features,
				"last_synced_at":     now,
				"updated_at":         now,
			}

			if err := s.db.WithContext(ctx).Model(&existingModel).Updates(updates).Error; err != nil {
				return count, fmt.Errorf("更新模型失败: %w", err)
			}
			count++
		} else {
			return count, fmt.Errorf("查询模型失败: %w", result.Error)
		}
	}

	return count, nil
}

// AutoDiscoverModels 自动发现所有提供商的模型
func (s *ModelDiscoveryService) AutoDiscoverModels(ctx context.Context, tenantID string) (map[string]int, error) {
	providers := []string{"gemini", "deepseek", "qwen", "ollama", "openai", "anthropic"}
	results := make(map[string]int)

	for _, provider := range providers {
		count, err := s.SyncModelsFromProvider(ctx, tenantID, provider)
		if err != nil {
			// 记录错误但继续其他提供商
			results[provider] = -1
			continue
		}
		results[provider] = count
	}

	return results, nil
}

// StartSyncScheduler 启动定时同步任务
func (s *ModelDiscoveryService) StartSyncScheduler(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour) // 每天同步一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 获取所有租户
			var tenants []struct {
				ID string
			}
			if err := s.db.WithContext(ctx).
				Table("tenants").
				Select("id").
				Find(&tenants).Error; err != nil {
				continue
			}

			// 为每个租户同步模型
			for _, tenant := range tenants {
				_, _ = s.AutoDiscoverModels(ctx, tenant.ID)
			}

		case <-ctx.Done():
			return
		}
	}
}

// ModelInfo 模型信息结构
type ModelInfo struct {
	ID              string
	Name            string
	Type            string
	Category        string
	Description     string
	MaxTokens       int
	ContextWindow   int
	InputCostPer1K  float64
	OutputCostPer1K float64
	Features        ModelFeatures
}

// 获取各提供商的模型列表

func (s *ModelDiscoveryService) getGeminiModels() ([]ModelInfo, error) {
	models := make([]ModelInfo, 0)

	for _, m := range google.DefaultGeminiModels {
		features := ModelFeatures{
			Streaming:       m.SupportStreaming,
			Vision:          m.SupportVision,
			FunctionCalling: true,
			Cache:           false,
			JsonMode:        true,
		}

		modelType := "chat"
		category := "chat"
		if m.ID == "gemini-embedding-001" {
			modelType = "embedding"
			category = "embedding"
		}

		models = append(models, ModelInfo{
			ID:              m.ID,
			Name:            m.Name,
			Type:            modelType,
			Category:        category,
			Description:     fmt.Sprintf("Google %s", m.Name),
			MaxTokens:       m.MaxTokens,
			ContextWindow:   m.ContextWindow,
			InputCostPer1K:  0.0, // Gemini 定价需要查询官网
			OutputCostPer1K: 0.0,
			Features:        features,
		})
	}

	return models, nil
}

func (s *ModelDiscoveryService) getDeepSeekModels() ([]ModelInfo, error) {
	models := make([]ModelInfo, 0)

	for _, m := range deepseek.DefaultDeepSeekModels {
		features := ModelFeatures{
			Streaming:       true,
			Vision:          false,
			FunctionCalling: true,
			Cache:           false,
			JsonMode:        true,
		}

		models = append(models, ModelInfo{
			ID:              m.ID,
			Name:            m.Name,
			Type:            "chat",
			Category:        "chat",
			Description:     fmt.Sprintf("DeepSeek %s", m.Name),
			MaxTokens:       m.MaxTokens,
			ContextWindow:   m.ContextWindow,
			InputCostPer1K:  0.001, // DeepSeek 价格示例
			OutputCostPer1K: 0.002,
			Features:        features,
		})
	}

	return models, nil
}

func (s *ModelDiscoveryService) getOpenAIModels() ([]ModelInfo, error) {
	// OpenAI 主流模型列表
	models := []ModelInfo{
		{
			ID:              "gpt-4-turbo",
			Name:            "GPT-4 Turbo",
			Type:            "chat",
			Category:        "chat",
			Description:     "Most capable GPT-4 model",
			MaxTokens:       4096,
			ContextWindow:   128000,
			InputCostPer1K:  0.01,
			OutputCostPer1K: 0.03,
			Features: ModelFeatures{
				Streaming:       true,
				Vision:          true,
				FunctionCalling: true,
				Cache:           false,
				JsonMode:        true,
			},
		},
		{
			ID:              "gpt-4",
			Name:            "GPT-4",
			Type:            "chat",
			Category:        "chat",
			Description:     "Standard GPT-4 model",
			MaxTokens:       8192,
			ContextWindow:   8192,
			InputCostPer1K:  0.03,
			OutputCostPer1K: 0.06,
			Features: ModelFeatures{
				Streaming:       true,
				Vision:          false,
				FunctionCalling: true,
				Cache:           false,
				JsonMode:        true,
			},
		},
		{
			ID:              "gpt-3.5-turbo",
			Name:            "GPT-3.5 Turbo",
			Type:            "chat",
			Category:        "chat",
			Description:     "Fast and cost-effective",
			MaxTokens:       4096,
			ContextWindow:   16385,
			InputCostPer1K:  0.0005,
			OutputCostPer1K: 0.0015,
			Features: ModelFeatures{
				Streaming:       true,
				Vision:          false,
				FunctionCalling: true,
				Cache:           false,
				JsonMode:        true,
			},
		},
		{
			ID:              "text-embedding-3-large",
			Name:            "Text Embedding 3 Large",
			Type:            "embedding",
			Category:        "embedding",
			Description:     "High-quality embeddings",
			MaxTokens:       8191,
			ContextWindow:   8191,
			InputCostPer1K:  0.00013,
			OutputCostPer1K: 0.0,
			Features: ModelFeatures{
				Streaming: false,
				Vision:    false,
			},
		},
	}

	return models, nil
}

func (s *ModelDiscoveryService) getAnthropicModels() ([]ModelInfo, error) {
	// Anthropic Claude 主流模型
	models := []ModelInfo{
		{
			ID:              "claude-3-5-sonnet-20241022",
			Name:            "Claude 3.5 Sonnet",
			Type:            "chat",
			Category:        "chat",
			Description:     "Most intelligent Claude model",
			MaxTokens:       8192,
			ContextWindow:   200000,
			InputCostPer1K:  0.003,
			OutputCostPer1K: 0.015,
			Features: ModelFeatures{
				Streaming:       true,
				Vision:          true,
				FunctionCalling: true,
				Cache:           true,
				JsonMode:        false,
			},
		},
		{
			ID:              "claude-3-opus-20240229",
			Name:            "Claude 3 Opus",
			Type:            "chat",
			Category:        "chat",
			Description:     "Powerful performance for complex tasks",
			MaxTokens:       4096,
			ContextWindow:   200000,
			InputCostPer1K:  0.015,
			OutputCostPer1K: 0.075,
			Features: ModelFeatures{
				Streaming:       true,
				Vision:          true,
				FunctionCalling: true,
				Cache:           true,
				JsonMode:        false,
			},
		},
		{
			ID:              "claude-3-haiku-20240307",
			Name:            "Claude 3 Haiku",
			Type:            "chat",
			Category:        "chat",
			Description:     "Fastest and most compact",
			MaxTokens:       4096,
			ContextWindow:   200000,
			InputCostPer1K:  0.00025,
			OutputCostPer1K: 0.00125,
			Features: ModelFeatures{
				Streaming:       true,
				Vision:          true,
				FunctionCalling: true,
				Cache:           true,
				JsonMode:        false,
			},
		},
	}

	return models, nil
}

func (s *ModelDiscoveryService) getQwenModels() ([]ModelInfo, error) {
	models := make([]ModelInfo, 0)

	for _, m := range qwen.DefaultQwenModels {
		features := ModelFeatures{
			Streaming:       true,
			Vision:          false,
			FunctionCalling: true,
			Cache:           false,
			JsonMode:        true,
		}

		modelType := "chat"
		category := "chat"
		if m.ID == "text-embedding-v2" {
			modelType = "embedding"
			category = "embedding"
		}

		models = append(models, ModelInfo{
			ID:              m.ID,
			Name:            m.Name,
			Type:            modelType,
			Category:        category,
			Description:     fmt.Sprintf("Qwen %s", m.Name),
			MaxTokens:       m.MaxTokens,
			ContextWindow:   m.ContextWindow,
			InputCostPer1K:  0.0008, // Qwen 价格示例
			OutputCostPer1K: 0.002,
			Features:        features,
		})
	}

	return models, nil
}

func (s *ModelDiscoveryService) getOllamaModels() ([]ModelInfo, error) {
	models := make([]ModelInfo, 0)

	for _, m := range ollama.DefaultOllamaModels {
		features := ModelFeatures{
			Streaming:       true,
			Vision:          false,
			FunctionCalling: false,
			Cache:           false,
			JsonMode:        false,
		}

		models = append(models, ModelInfo{
			ID:              m.ID,
			Name:            m.Name,
			Type:            "chat",
			Category:        "chat",
			Description:     fmt.Sprintf("Ollama %s", m.Name),
			MaxTokens:       m.MaxTokens,
			ContextWindow:   m.ContextWindow,
			InputCostPer1K:  0.0, // 本地模型免费
			OutputCostPer1K: 0.0,
			Features:        features,
		})
	}

	return models, nil
}

// 辅助函数

func getAPIFormat(provider string) string {
	switch provider {
	case "openai", "azure", "deepseek", "qwen", "ollama", "custom":
		return "openai"
	case "anthropic":
		return "claude"
	case "google", "gemini":
		return "gemini"
	default:
		return "openai"
	}
}
