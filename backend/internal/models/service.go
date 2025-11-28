package models

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backend/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ModelService AI 模型管理服务
type ModelService struct {
	db    *gorm.DB
	cache *modelListCache
}

// NewModelService 创建 ModelService 实例
func NewModelService(db *gorm.DB) *ModelService {
	return &ModelService{
		db:    db,
		cache: newModelListCache(30 * time.Second),
	}
}

type modelListCache struct {
	ttl   time.Duration
	mu    sync.RWMutex
	items map[string]modelCacheEntry
}

type modelCacheEntry struct {
	resp    *ListModelsResponse
	expires time.Time
	tenant  string
}

func newModelListCache(ttl time.Duration) *modelListCache {
	return &modelListCache{
		ttl:   ttl,
		items: make(map[string]modelCacheEntry),
	}
}

func (c *modelListCache) get(key string) (*ListModelsResponse, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		if ok {
			c.mu.Lock()
			delete(c.items, key)
			c.mu.Unlock()
		}
		return nil, false
	}
	return entry.resp, true
}

func (c *modelListCache) set(key, tenant string, resp *ListModelsResponse) {
	if c == nil || resp == nil {
		return
	}
	c.mu.Lock()
	c.items[key] = modelCacheEntry{
		resp:    resp,
		expires: time.Now().Add(c.ttl),
		tenant:  tenant,
	}
	c.mu.Unlock()
}

func (c *modelListCache) invalidateTenant(tenantID string) {
	if c == nil || tenantID == "" {
		return
	}
	c.mu.Lock()
	for key, entry := range c.items {
		if entry.tenant == tenantID {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()
}

// ListModelsRequest 查询模型列表请求
type ListModelsRequest struct {
	TenantID string
	Provider string // openai, anthropic, custom
	Type     string // chat, embedding
	Status   string // active, deprecated, disabled
	UserTier string // 用户会员等级，用于过滤可用模型（free, basic, pro, enterprise）
	Page     int
	PageSize int
}

// ListModelsResponse 查询模型列表响应
type ListModelsResponse struct {
	Models     []*Model `json:"models"`
	Total      int64    `json:"total"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	TotalPages int      `json:"total_pages"`
}

// ListModels 查询模型列表
func (s *ModelService) ListModels(ctx context.Context, req *ListModelsRequest) (*ListModelsResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	cacheKey := fmt.Sprintf("%s|%s|%s|%s|%s|%d|%d", req.TenantID, req.Provider, req.Type, req.Status, req.UserTier, page, pageSize)
	if cached, ok := s.cache.get(cacheKey); ok {
		return cached, nil
	}

	// 构建查询
	query := s.db.WithContext(ctx).
		Model(&Model{}).
		Scopes(common.NotDeleted())

	// 租户过滤
	if req.TenantID != "" {
		query = query.Where("tenant_id = ?", req.TenantID)
	}

	// 提供商过滤
	if req.Provider != "" {
		query = query.Where("provider = ?", req.Provider)
	}

	// 类型过滤
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}

	// 状态过滤
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	// 会员等级过滤：空数组表示所有等级可用，否则检查是否包含用户等级
	if req.UserTier != "" {
		query = query.Where("(allowed_tiers = '[]'::jsonb OR allowed_tiers IS NULL OR allowed_tiers @> ?::jsonb)", fmt.Sprintf(`["%s"]`, req.UserTier))
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("统计模型数量失败: %w", err)
	}

	// 分页
	offset := (page - 1) * pageSize

	// 查询列表
	var models []*Model
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询模型列表失败: %w", err)
	}

	// 计算总页数
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	resp := &ListModelsResponse{
		Models:     models,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
	s.cache.set(cacheKey, req.TenantID, resp)
	return resp, nil
}

// GetModel 查询单个模型
func (s *ModelService) GetModel(ctx context.Context, tenantID, modelID string) (*Model, error) {
	var model Model
	if err := s.db.WithContext(ctx).
		Scopes(common.NotDeleted()).
		Where("id = ? AND tenant_id = ?", modelID, tenantID).
		First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("模型不存在")
		}
		return nil, fmt.Errorf("查询模型失败: %w", err)
	}
	return &model, nil
}

// CreateModelRequest 创建模型请求
type CreateModelRequest struct {
	TenantID        string
	Name            string
	Provider        string
	ModelIdentifier string
	Type            string
	Category        string // chat, image, audio, video, embedding, rerank
	Description     string
	// 能力配置
	Capabilities            map[string]any
	Features                ModelFeatures
	SupportsStreaming       bool
	SupportsFunctionCalling bool
	// 成本配置
	InputCostPer1K  float64
	OutputCostPer1K float64
	// 限制配置
	MaxTokens       int
	ContextWindow   int
	RateLimitPerMin int
	LatencySloMs    int
	// API 配置
	BaseURL    string
	APIVersion string
	APIFormat  string // openai, claude, gemini, deepseek, custom
	Region     string
	// 语言支持
	SupportedLanguages []string
	// 状态配置
	Status    string
	IsBuiltin bool
	IsActive  bool
	// 会员等级权限
	AllowedTiers []string
	// 凭证配置
	DefaultCredentialID string
	// 元数据
	Metadata map[string]any
}

// CreateModel 创建模型配置
func (s *ModelService) CreateModel(ctx context.Context, req *CreateModelRequest) (*Model, error) {
	// 验证必填字段
	if req.Name == "" {
		return nil, fmt.Errorf("模型名称不能为空")
	}
	if req.Provider == "" {
		return nil, fmt.Errorf("提供商不能为空")
	}
	if req.ModelIdentifier == "" {
		return nil, fmt.Errorf("模型标识不能为空")
	}
	if req.Type == "" {
		return nil, fmt.Errorf("模型类型不能为空")
	}

	// 检查是否已存在
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&Model{}).
		Scopes(common.NotDeleted()).
		Where("tenant_id = ? AND provider = ? AND model_identifier = ?",
			req.TenantID, req.Provider, req.ModelIdentifier).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("检查模型是否存在失败: %w", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("模型已存在")
	}

	// 设置默认值
	status := req.Status
	if status == "" {
		status = "active"
	}
	category := req.Category
	if category == "" {
		category = "chat"
	}
	apiFormat := req.APIFormat
	if apiFormat == "" {
		apiFormat = "openai"
	}
	isActive := req.IsActive
	if !req.IsBuiltin && !req.IsActive {
		isActive = true // 默认启用
	}

	// 创建模型
	model := &Model{
		ID:              uuid.New().String(),
		TenantID:        req.TenantID,
		Name:            req.Name,
		Provider:        req.Provider,
		ModelIdentifier: req.ModelIdentifier,
		Type:            req.Type,
		Category:        category,
		Description:     req.Description,
		// 能力配置
		Capabilities:            req.Capabilities,
		Features:                req.Features,
		SupportsStreaming:       req.SupportsStreaming,
		SupportsFunctionCalling: req.SupportsFunctionCalling,
		// 成本配置
		InputCostPer1K:  req.InputCostPer1K,
		OutputCostPer1K: req.OutputCostPer1K,
		// 限制配置
		MaxTokens:       req.MaxTokens,
		ContextWindow:   req.ContextWindow,
		RateLimitPerMin: req.RateLimitPerMin,
		LatencySloMs:    req.LatencySloMs,
		// API 配置
		BaseURL:    req.BaseURL,
		APIVersion: req.APIVersion,
		APIFormat:  apiFormat,
		Region:     req.Region,
		// 语言支持
		SupportedLanguages: req.SupportedLanguages,
		// 状态配置
		Status:    status,
		IsBuiltin: req.IsBuiltin,
		IsActive:  isActive,
		// 会员等级权限
		AllowedTiers: req.AllowedTiers,
		// 凭证配置
		DefaultCredentialID: req.DefaultCredentialID,
		// 元数据
		Metadata:  req.Metadata,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(model).Error; err != nil {
		return nil, fmt.Errorf("创建模型失败: %w", err)
	}

	s.cache.invalidateTenant(req.TenantID)

	return model, nil
}

// UpdateModelRequest 更新模型请求
type UpdateModelRequest struct {
	Name        *string
	Description *string
	Category    *string
	// 能力配置
	Capabilities            map[string]any
	Features                *ModelFeatures
	SupportsStreaming       *bool
	SupportsFunctionCalling *bool
	// 成本配置
	InputCostPer1K  *float64
	OutputCostPer1K *float64
	// 限制配置
	MaxTokens       *int
	ContextWindow   *int
	RateLimitPerMin *int
	LatencySloMs    *int
	// API 配置
	BaseURL    *string
	APIVersion *string
	APIFormat  *string
	Region     *string
	// 语言支持
	SupportedLanguages []string
	// 状态配置
	Status   *string
	IsActive *bool
	// 会员等级权限
	AllowedTiers []string
	// 凭证配置
	DefaultCredentialID *string
	// 元数据
	Metadata map[string]any
}

// UpdateModel 更新模型配置
func (s *ModelService) UpdateModel(ctx context.Context, tenantID, modelID string, req *UpdateModelRequest) (*Model, error) {
	// 查询模型
	model, err := s.GetModel(ctx, tenantID, modelID)
	if err != nil {
		return nil, err
	}

	// 更新字段
	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	// 能力配置
	if req.Capabilities != nil {
		updates["capabilities"] = req.Capabilities
	}
	if req.Features != nil {
		updates["features"] = *req.Features
	}
	if req.SupportsStreaming != nil {
		updates["supports_streaming"] = *req.SupportsStreaming
	}
	if req.SupportsFunctionCalling != nil {
		updates["supports_function_calling"] = *req.SupportsFunctionCalling
	}
	// 成本配置
	if req.InputCostPer1K != nil {
		updates["input_cost_per_1k"] = *req.InputCostPer1K
	}
	if req.OutputCostPer1K != nil {
		updates["output_cost_per_1k"] = *req.OutputCostPer1K
	}
	// 限制配置
	if req.MaxTokens != nil {
		updates["max_tokens"] = *req.MaxTokens
	}
	if req.ContextWindow != nil {
		updates["context_window"] = *req.ContextWindow
	}
	if req.RateLimitPerMin != nil {
		updates["rate_limit_per_min"] = *req.RateLimitPerMin
	}
	if req.LatencySloMs != nil {
		updates["latency_slo_ms"] = *req.LatencySloMs
	}
	// API 配置
	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}
	if req.APIVersion != nil {
		updates["api_version"] = *req.APIVersion
	}
	if req.APIFormat != nil {
		updates["api_format"] = *req.APIFormat
	}
	if req.Region != nil {
		updates["region"] = *req.Region
	}
	// 语言支持
	if req.SupportedLanguages != nil {
		updates["supported_languages"] = req.SupportedLanguages
	}
	// 状态配置
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	// 会员等级权限
	if req.AllowedTiers != nil {
		updates["allowed_tiers"] = req.AllowedTiers
	}
	// 凭证配置
	if req.DefaultCredentialID != nil {
		updates["default_credential_id"] = *req.DefaultCredentialID
	}
	// 元数据
	if req.Metadata != nil {
		updates["metadata"] = req.Metadata
	}
	updates["updated_at"] = time.Now().UTC()

	// 执行更新
	if err := s.db.WithContext(ctx).
		Model(model).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新模型失败: %w", err)
	}

	s.cache.invalidateTenant(tenantID)

	// 重新查询返回最新数据
	return s.GetModel(ctx, tenantID, modelID)
}

// DeleteModel 软删除模型
func (s *ModelService) DeleteModel(ctx context.Context, tenantID, modelID, operatorID string) error {
	// 查询模型
	model, err := s.GetModel(ctx, tenantID, modelID)
	if err != nil {
		return err
	}

	// 软删除
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).
		Model(model).
		Updates(map[string]any{
			"deleted_at": now,
			"deleted_by": operatorID,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("删除模型失败: %w", err)
	}

	s.cache.invalidateTenant(tenantID)

	return nil
}

// SeedDefaultModels 初始化预置模型
func (s *ModelService) SeedDefaultModels(ctx context.Context, tenantID string) error {
	defaultModels := []*CreateModelRequest{
		// OpenAI GPT-4
		{
			TenantID:        tenantID,
			Name:            "GPT-4 Turbo",
			Provider:        "openai",
			ModelIdentifier: "gpt-4-turbo-preview",
			Type:            "chat",
			Description:     "最新的 GPT-4 Turbo 模型，支持 128K 上下文",
			Capabilities: map[string]any{
				"streaming":        true,
				"function_calling": true,
				"vision":           false,
			},
			InputCostPer1K:     0.01,
			OutputCostPer1K:    0.03,
			MaxTokens:          4096,
			ContextWindow:      128000,
			SupportedLanguages: []string{"zh", "en", "ja", "ko"},
		},
		// OpenAI GPT-3.5
		{
			TenantID:        tenantID,
			Name:            "GPT-3.5 Turbo",
			Provider:        "openai",
			ModelIdentifier: "gpt-3.5-turbo",
			Type:            "chat",
			Description:     "高性价比的对话模型",
			Capabilities: map[string]any{
				"streaming":        true,
				"function_calling": true,
				"vision":           false,
			},
			InputCostPer1K:     0.0005,
			OutputCostPer1K:    0.0015,
			MaxTokens:          4096,
			ContextWindow:      16385,
			SupportedLanguages: []string{"zh", "en", "ja", "ko"},
		},
		// Anthropic Claude
		{
			TenantID:        tenantID,
			Name:            "Claude 3.5 Sonnet",
			Provider:        "anthropic",
			ModelIdentifier: "claude-3-5-sonnet-20241022",
			Type:            "chat",
			Description:     "Anthropic 最新的 Claude 3.5 Sonnet 模型",
			Capabilities: map[string]any{
				"streaming":        true,
				"function_calling": true,
				"vision":           true,
			},
			InputCostPer1K:     0.003,
			OutputCostPer1K:    0.015,
			MaxTokens:          8192,
			ContextWindow:      200000,
			SupportedLanguages: []string{"zh", "en", "ja", "ko"},
		},
		// OpenAI Embedding
		{
			TenantID:        tenantID,
			Name:            "Text Embedding 3 Small",
			Provider:        "openai",
			ModelIdentifier: "text-embedding-3-small",
			Type:            "embedding",
			Description:     "OpenAI 向量化模型（小型）",
			Capabilities: map[string]any{
				"dimensions": 1536,
			},
			InputCostPer1K:     0.00002,
			OutputCostPer1K:    0,
			MaxTokens:          8191,
			ContextWindow:      8191,
			SupportedLanguages: []string{"zh", "en", "ja", "ko"},
		},
		// Google Gemini
		{
			TenantID:        tenantID,
			Name:            "Gemini 1.5 Pro",
			Provider:        "gemini",
			ModelIdentifier: "gemini-1.5-pro",
			Type:            "chat",
			Description:     "Google Gemini 旗舰模型，支持长上下文与多模态",
			Capabilities: map[string]any{
				"streaming":        true,
				"function_calling": true,
				"vision":           true,
			},
			InputCostPer1K:     0.0025,
			OutputCostPer1K:    0.0075,
			MaxTokens:          8192,
			ContextWindow:      2000000,
			SupportedLanguages: []string{"zh", "en", "ja", "ko"},
		},
		{
			TenantID:        tenantID,
			Name:            "Gemini Embedding 001",
			Provider:        "gemini",
			ModelIdentifier: "gemini-embedding-001",
			Type:            "embedding",
			Description:     "Google Gemini 官方向量化模型",
			Capabilities: map[string]any{
				"dimensions": 3072,
			},
			InputCostPer1K:     0.00013,
			OutputCostPer1K:    0,
			MaxTokens:          2048,
			ContextWindow:      2048,
			SupportedLanguages: []string{"zh", "en", "ja", "ko"},
		},
	}

	// 批量创建
	for _, req := range defaultModels {
		if _, err := s.CreateModel(ctx, req); err != nil {
			// 如果已存在，跳过
			if err.Error() == "模型已存在" {
				continue
			}
			return fmt.Errorf("创建预置模型失败 [%s]: %w", req.Name, err)
		}
	}

	return nil
}

// GetModelCallStats 获取模型调用统计
func (s *ModelService) GetModelCallStats(ctx context.Context, tenantID, modelID string, startTime, endTime time.Time) (map[string]any, error) {
	var stats struct {
		TotalCalls            int64   `json:"total_calls"`
		TotalTokens           int64   `json:"total_tokens"`
		TotalCost             float64 `json:"total_cost"`
		AvgLatencyMs          float64 `json:"avg_latency_ms"`
		SuccessRate           float64 `json:"success_rate"`
		TotalPromptTokens     int64   `json:"total_prompt_tokens"`
		TotalCompletionTokens int64   `json:"total_completion_tokens"`
	}

	query := s.db.WithContext(ctx).
		Model(&ModelCallLog{}).
		Where("tenant_id = ? AND model_id = ?", tenantID, modelID)

	if !startTime.IsZero() {
		query = query.Where("created_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		query = query.Where("created_at <= ?", endTime)
	}

	if err := query.
		Select(`
			COUNT(*) as total_calls,
			SUM(total_tokens) as total_tokens,
			SUM(total_cost) as total_cost,
			AVG(latency_ms) as avg_latency_ms,
			SUM(prompt_tokens) as total_prompt_tokens,
			SUM(completion_tokens) as total_completion_tokens
		`).
		Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("查询统计数据失败: %w", err)
	}

	// 成功率（假设没有 status 字段，则认为所有调用都成功）
	stats.SuccessRate = 100.0

	return map[string]any{
		"total_calls":             stats.TotalCalls,
		"total_tokens":            stats.TotalTokens,
		"total_cost":              stats.TotalCost,
		"avg_latency_ms":          stats.AvgLatencyMs,
		"success_rate":            stats.SuccessRate,
		"total_prompt_tokens":     stats.TotalPromptTokens,
		"total_completion_tokens": stats.TotalCompletionTokens,
	}, nil
}
