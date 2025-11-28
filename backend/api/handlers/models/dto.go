package models

// CreateCredentialRequest 创建模型凭证请求
type CreateCredentialRequest struct {
	Name         string         `json:"name" binding:"required"`
	Provider     string         `json:"provider"`
	APIKey       string         `json:"apiKey" binding:"required"`
	BaseURL      string         `json:"baseUrl"`
	ExtraHeaders map[string]any `json:"extraHeaders"`
	SetAsDefault bool           `json:"setAsDefault"`
}

// UpdateCredentialRequest 更新模型凭证请求
type UpdateCredentialRequest struct {
	Name         *string        `json:"name"`
	APIKey       *string        `json:"apiKey"`
	BaseURL      *string        `json:"baseUrl"`
	ExtraHeaders map[string]any `json:"extraHeaders"`
	Status       *string        `json:"status"`
}

// CreateModelQuotaRequest 创建模型配额请求
type CreateModelQuotaRequest struct {
	ModelID              string  `json:"modelId"`
	MaxTokensPerDay      int64   `json:"maxTokensPerDay"`
	MaxTokensPerMonth    int64   `json:"maxTokensPerMonth"`
	MaxRequestsPerMinute int     `json:"maxRequestsPerMinute"`
	MaxCostPerDay        float64 `json:"maxCostPerDay"`
	MaxCostPerMonth      float64 `json:"maxCostPerMonth"`
}

// UpdateModelQuotaRequest 更新模型配额请求
type UpdateModelQuotaRequest struct {
	MaxTokensPerDay      *int64   `json:"maxTokensPerDay"`
	MaxTokensPerMonth    *int64   `json:"maxTokensPerMonth"`
	MaxRequestsPerMinute *int     `json:"maxRequestsPerMinute"`
	MaxCostPerDay        *float64 `json:"maxCostPerDay"`
	MaxCostPerMonth      *float64 `json:"maxCostPerMonth"`
}

// ModelQuotaResponse 模型配额响应
type ModelQuotaResponse struct {
	ID                   string  `json:"id"`
	TenantID             string  `json:"tenantId"`
	ModelID              string  `json:"modelId"`
	MaxTokensPerDay      int64   `json:"maxTokensPerDay"`
	MaxTokensPerMonth    int64   `json:"maxTokensPerMonth"`
	MaxRequestsPerMinute int     `json:"maxRequestsPerMinute"`
	MaxCostPerDay        float64 `json:"maxCostPerDay"`
	MaxCostPerMonth      float64 `json:"maxCostPerMonth"`
	UsedTokensToday      int64   `json:"usedTokensToday"`
	UsedTokensMonth      int64   `json:"usedTokensMonth"`
	UsedCostToday        float64 `json:"usedCostToday"`
	UsedCostMonth        float64 `json:"usedCostMonth"`
}
