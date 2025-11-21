package tools

import "time"

// ToolDefinition 工具定义
type ToolDefinition struct {
	ID          string         `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string         `json:"tenantId" gorm:"type:uuid;not null;index"`
	
	// 基本信息
	Name        string         `json:"name" gorm:"size:100;not null;uniqueIndex:idx_tenant_tool_name"`
	DisplayName string         `json:"displayName" gorm:"size:255;not null"`
	Description string         `json:"description" gorm:"type:text;not null"`
	Category    string         `json:"category" gorm:"size:50"` // search, data_analysis, document, calculation
	
	// 工具类型
	Type        string         `json:"type" gorm:"size:50;not null"` // builtin, http_api, code_interpreter
	
	// 参数定义（JSON Schema）
	Parameters  map[string]any `json:"parameters" gorm:"type:jsonb;serializer:json"`
	
	// HTTP API 配置（仅 type=http_api 时使用）
	HTTPConfig  *HTTPToolConfig `json:"httpConfig,omitempty" gorm:"type:jsonb;serializer:json"`
	
	// 代码解释器配置（仅 type=code_interpreter 时使用）
	CodeConfig  *CodeToolConfig `json:"codeConfig,omitempty" gorm:"type:jsonb;serializer:json"`
	
	// 权限控制
	RequireAuth bool           `json:"requireAuth" gorm:"default:true"`  // 是否需要授权
	Scopes      []string       `json:"scopes" gorm:"type:jsonb;serializer:json"` // 权限范围
	
	// 执行配置
	Timeout     int            `json:"timeout" gorm:"default:30"` // 超时时间（秒）
	MaxRetries  int            `json:"maxRetries" gorm:"default:3"` // 最大重试次数
	
	// 状态
	Status      string         `json:"status" gorm:"size:50;default:active"` // active, disabled
	
	// 时间戳
	CreatedAt   time.Time      `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time      `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt   *time.Time     `json:"deletedAt,omitempty" gorm:"index"`
}

// HTTPToolConfig HTTP API 工具配置
type HTTPToolConfig struct {
	Method  string            `json:"method"`  // GET, POST, PUT, DELETE
	URL     string            `json:"url"`     // API 端点 URL
	Headers map[string]string `json:"headers"` // HTTP 头部
	Auth    *AuthConfig       `json:"auth"`    // 认证配置
}

// AuthConfig 认证配置
type AuthConfig struct {
	Type   string `json:"type"`   // bearer, api_key, basic
	Token  string `json:"token"`  // Bearer Token
	APIKey string `json:"apiKey"` // API Key
	Header string `json:"header"` // API Key 头部名称
}

// CodeToolConfig 代码解释器配置
type CodeToolConfig struct {
	Language    string   `json:"language"`    // python, javascript
	AllowImport []string `json:"allowImport"` // 允许导入的库
	Sandbox     bool     `json:"sandbox"`     // 是否沙箱执行
}

// ToolExecution 工具执行记录
type ToolExecution struct {
	ID           string         `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID     string         `json:"tenantId" gorm:"type:uuid;not null;index"`
	
	// 工具信息
	ToolID       string         `json:"toolId" gorm:"type:uuid;not null"`
	ToolName     string         `json:"toolName" gorm:"size:100;not null"`
	
	// 执行上下文
	AgentID      string         `json:"agentId" gorm:"type:uuid"`
	WorkflowID   *string        `json:"workflowId,omitempty" gorm:"type:uuid"`
	ExecutionID  *string        `json:"executionId,omitempty" gorm:"type:uuid"`
	
	// 输入输出
	Input        map[string]any `json:"input" gorm:"type:jsonb;serializer:json"`
	Output       map[string]any `json:"output" gorm:"type:jsonb;serializer:json"`
	ErrorMessage *string        `json:"errorMessage,omitempty" gorm:"type:text"`
	
	// 执行状态
	Status       string         `json:"status" gorm:"size:50;not null"` // running, success, failed
	StartedAt    time.Time      `json:"startedAt" gorm:"not null"`
	CompletedAt  *time.Time     `json:"completedAt,omitempty"`
	Duration     int64          `json:"duration"` // 执行时长（毫秒）
	
	// 时间戳
	CreatedAt    time.Time      `json:"createdAt" gorm:"not null;autoCreateTime"`
}
