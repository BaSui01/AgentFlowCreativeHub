package tools

import (
	"backend/internal/tools"
)

// UpdateToolRequest 更新工具请求
type UpdateToolRequest struct {
	DisplayName *string               `json:"displayName"`
	Description *string               `json:"description"`
	Category    *string               `json:"category"`
	Parameters  map[string]any        `json:"parameters"`
	HTTPConfig  *tools.HTTPToolConfig `json:"httpConfig"`
	RequireAuth *bool                 `json:"requireAuth"`
	Scopes      []string              `json:"scopes"`
	Timeout     *int                  `json:"timeout"`
	MaxRetries  *int                  `json:"maxRetries"`
	Status      *string               `json:"status"`
}

// ToolVersionRequest 工具版本请求
type ToolVersionRequest struct {
	Version  string         `json:"version" binding:"required"`
	ImplType string         `json:"implType" binding:"required"`
	ImplRef  string         `json:"implRef" binding:"required"`
	Config   map[string]any `json:"config"`
}
