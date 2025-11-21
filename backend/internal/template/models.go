package template

import "time"

// PromptTemplate Prompt 模板
type PromptTemplate struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string `json:"tenantId" gorm:"type:uuid;not null;index"`
	OwnerUserID string `json:"ownerUserId" gorm:"type:uuid;index"`

	// 模板信息
	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`
	Category    string `json:"category" gorm:"size:100"` // writing, reviewing, formatting, translation, analysis

	// 可见性
	Visibility string `json:"visibility" gorm:"size:50;not null;default:personal"` // personal, tenant, public

	// 最新版本
	CurrentVersionID *string `json:"currentVersionId" gorm:"type:uuid"`

	// 统计
	UsageCount int `json:"usageCount" gorm:"default:0"`

	// 创建人
	CreatedBy string `json:"createdBy" gorm:"size:100"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}

// PromptTemplateVersion 模板版本
type PromptTemplateVersion struct {
	ID         string `json:"id" gorm:"primaryKey;type:uuid"`
	TemplateID string `json:"templateId" gorm:"type:uuid;not null;index"`

	// 版本信息
	Version   string         `json:"version" gorm:"size:50;not null"`
	Content   string         `json:"content" gorm:"type:text;not null"`
	Variables map[string]any `json:"variables" gorm:"type:jsonb;serializer:json"`

	// 变更说明
	Changelog string `json:"changelog" gorm:"type:text"`

	// 创建人
	CreatedBy string `json:"createdBy" gorm:"size:100"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
}
