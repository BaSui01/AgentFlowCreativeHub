package workflow

import "time"

// WorkflowTemplate 工作流模板
type WorkflowTemplate struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string `json:"tenantId" gorm:"type:uuid;index"` // 租户级模板（为空表示系统模板）
	
	// 模板信息
	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`
	Category    string `json:"category" gorm:"size:100"` // outline-only, quick-write, full-content, multilang, custom
	
	// 模板定义
	Definition  map[string]any `json:"definition" gorm:"type:jsonb;not null;serializer:json"`
	
	// 可见性
	IsSystem    bool   `json:"isSystem" gorm:"default:false"`      // 是否系统预设模板
	IsPublic    bool   `json:"isPublic" gorm:"default:false"`      // 是否公开（租户间共享）
	
	// 使用统计
	UsageCount  int    `json:"usageCount" gorm:"default:0"`
	
	// 创建人
	CreatedBy   string `json:"createdBy" gorm:"size:100"`
	
	// 时间戳
	CreatedAt   time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}

// AgentCapability Agent 能力定义
type AgentCapability struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	
	// Agent 信息
	AgentType   string `json:"agentType" gorm:"size:100;not null;index"`
	Role        string `json:"role" gorm:"size:100;not null"`
	
	// 能力描述
	Name        string   `json:"name" gorm:"size:255;not null"`
	Description string   `json:"description" gorm:"type:text"`
	InputFields []string `json:"inputFields" gorm:"type:text[]"`   // 输入字段列表
	OutputFields []string `json:"outputFields" gorm:"type:text[]"` // 输出字段列表
	
	// System Prompt 模板
	SystemPromptTemplate string `json:"systemPromptTemplate" gorm:"type:text"`
	
	// 推荐配置
	RecommendedConfig map[string]any `json:"recommendedConfig" gorm:"type:jsonb;serializer:json"`
	
	// 是否启用
	Enabled     bool `json:"enabled" gorm:"default:true"`
	
	// 时间戳
	CreatedAt   time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}
