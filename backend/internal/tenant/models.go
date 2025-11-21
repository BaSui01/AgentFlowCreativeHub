package tenant

import "time"

// Tenant represents a logical tenant in the system. All tenant-scoped data
// should reference TenantID to ensure proper isolation.
type Tenant struct {
	ID     string `json:"id" gorm:"primaryKey;type:uuid"`
	Name   string `json:"name" gorm:"size:255;not null"`
	Slug   string `json:"slug" gorm:"size:100;uniqueIndex;not null"`
	Status string `json:"status" gorm:"size:50;not null;default:active"`

	// 套餐信息
	Tier               string     `json:"tier" gorm:"size:50;not null;default:free"`
	TrialEndsAt        *time.Time `json:"trialEndsAt"`
	SubscriptionEndsAt *time.Time `json:"subscriptionEndsAt"`

	// 联系信息
	ContactEmail  string `json:"contactEmail" gorm:"size:255"`
	ContactPhone  string `json:"contactPhone" gorm:"size:50"`
	ContactPerson string `json:"contactPerson" gorm:"size:100"`

	// 公司信息
	Industry    string `json:"industry" gorm:"size:100"`
	CompanySize string `json:"companySize" gorm:"size:50"`
	Country     string `json:"country" gorm:"size:100"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
	DeletedBy string     `json:"deletedBy,omitempty" gorm:"size:100"`
}

// User represents a user that belongs to a specific tenant.
type User struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;not null;index"`

	// 认证信息
	Email        string `json:"email" gorm:"size:255;not null"`
	Username     string `json:"username" gorm:"size:100;not null"`
	PasswordHash string `json:"-" gorm:"size:255;not null"`

	// 个人信息
	FullName  string `json:"fullName" gorm:"size:255"`
	AvatarURL string `json:"avatarUrl" gorm:"type:text"`
	Phone     string `json:"phone" gorm:"size:50"`

	// 偏好设置
	Locale   string `json:"locale" gorm:"size:10;default:zh-CN"`
	Timezone string `json:"timezone" gorm:"size:50;default:Asia/Shanghai"`

	// 状态管理
	Status        string `json:"status" gorm:"size:50;not null;default:active"`
	EmailVerified bool   `json:"emailVerified" gorm:"default:false"`

	// 安全相关
	LastLoginAt   *time.Time `json:"lastLoginAt"`
	LastLoginIP   string     `json:"lastLoginIp" gorm:"size:50"`
	LoginAttempts int        `json:"-" gorm:"default:0"`
	LockedUntil   *time.Time `json:"lockedUntil"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
	DeletedBy string     `json:"deletedBy,omitempty" gorm:"size:100"`
}

// Role represents a named role inside a tenant.
type Role struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;not null;index"`

	Name        string `json:"name" gorm:"size:100;not null"`
	Code        string `json:"code" gorm:"size:100;not null"`
	Description string `json:"description" gorm:"type:text"`

	// 角色属性
	IsSystem  bool `json:"isSystem" gorm:"default:false"`
	IsDefault bool `json:"isDefault" gorm:"default:false"`
	Priority  int  `json:"priority" gorm:"default:0"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}

// Permission describes an action that can be performed on a resource. It can
// be tenant-scoped or platform-wide when TenantID is empty.
type Permission struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;index"` // NULL 表示全局权限

	Resource    string `json:"resource" gorm:"size:100;not null"`
	Action      string `json:"action" gorm:"size:100;not null"`
	Description string `json:"description" gorm:"type:text"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// UserRole links a user to a role within a tenant.
type UserRole struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID   string `json:"userId" gorm:"type:uuid;not null;index"`
	RoleID   string `json:"roleId" gorm:"type:uuid;not null;index"`

	// 权限生效时间（可选）
	EffectiveFrom  *time.Time `json:"effectiveFrom"`
	EffectiveUntil *time.Time `json:"effectiveUntil"`

	// 授予信息
	GrantedBy string `json:"grantedBy" gorm:"type:uuid"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
}

// RolePermission links a role to a permission within a tenant.
type RolePermission struct {
	ID           string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID     string `json:"tenantId" gorm:"type:uuid;not null;index"`
	RoleID       string `json:"roleId" gorm:"type:uuid;not null;index"`
	PermissionID string `json:"permissionId" gorm:"type:uuid;not null;index"`

	// 权限约束（可选，JSON 存储条件）
	Conditions map[string]any `json:"conditions" gorm:"type:jsonb;serializer:json"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
}

// TenantConfig stores per-tenant configuration such as branding, language,
// timezone, feature flags, and other customization values.
type TenantConfig struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;not null;uniqueIndex"`

	// 品牌信息
	DisplayName string `json:"displayName" gorm:"size:255"`
	Description string `json:"description" gorm:"type:text"`
	LogoURL     string `json:"logoUrl" gorm:"type:text"`

	// 语言和时区
	Language string `json:"language" gorm:"size:10;default:zh-CN"`
	TimeZone string `json:"timezone" gorm:"size:64;default:Asia/Shanghai"`

	// AI 配置
	DefaultModel        string  `json:"defaultModel" gorm:"size:100"`
	MaxTokensPerRequest int     `json:"maxTokensPerRequest" gorm:"default:4096"`
	Temperature         float64 `json:"temperature" gorm:"type:decimal(3,2);default:0.7"`

	// RAG 配置
	DefaultEmbeddingModel string  `json:"defaultEmbeddingModel" gorm:"size:100"`
	VectorSearchTopK      int     `json:"vectorSearchTopK" gorm:"default:10"`
	VectorSearchThreshold float64 `json:"vectorSearchThreshold" gorm:"type:decimal(3,2);default:0.7"`

	// 工作流配置
	MaxConcurrentWorkflows int `json:"maxConcurrentWorkflows" gorm:"default:5"`
	WorkflowTimeoutSeconds int `json:"workflowTimeoutSeconds" gorm:"default:3600"`

	// 功能开关
	FeatureFlags map[string]bool `json:"featureFlags" gorm:"type:jsonb;serializer:json"`

	// 审批设置
	ApprovalSettings *ApprovalSettings `json:"approvalSettings" gorm:"type:jsonb;serializer:json"`

	// 扩展配置（JSON）
	ExtraConfig map[string]any `json:"extraConfig" gorm:"type:jsonb;serializer:json"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TenantQuota defines quota limits and usage for a tenant.
type TenantQuota struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;not null;uniqueIndex"`

	// 用户配额
	MaxUsers  int `json:"maxUsers" gorm:"default:10"`
	UsedUsers int `json:"usedUsers" gorm:"default:0"`

	// 存储配额（MB）
	MaxStorageMB  int `json:"maxStorageMb" gorm:"default:10240"`
	UsedStorageMB int `json:"usedStorageMb" gorm:"default:0"`

	// 工作流配额
	MaxWorkflows  int `json:"maxWorkflows" gorm:"default:100"`
	UsedWorkflows int `json:"usedWorkflows" gorm:"default:0"`

	// 知识库配额
	MaxKnowledgeBases  int `json:"maxKnowledgeBases" gorm:"default:10"`
	UsedKnowledgeBases int `json:"usedKnowledgeBases" gorm:"default:0"`

	// AI 调用配额
	MaxTokensPerMonth   int64 `json:"maxTokensPerMonth" gorm:"default:1000000"`
	UsedTokensThisMonth int64 `json:"usedTokensThisMonth" gorm:"default:0"`

	// API 调用配额
	MaxAPICallsPerDay int `json:"maxApiCallsPerDay" gorm:"default:10000"`
	UsedAPICallsToday int `json:"usedApiCallsToday" gorm:"default:0"`

	// 配额重置时间
	TokenQuotaResetAt time.Time `json:"tokenQuotaResetAt"`
	APIQuotaResetAt   time.Time `json:"apiQuotaResetAt"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// ApprovalSettings 审批与通知配置
type ApprovalSettings struct {
	DefaultChannels      []string            `json:"defaultChannels"`
	NotificationTargets  map[string][]string `json:"notificationTargets"`
	ChannelFallbackOrder []string            `json:"channelFallbackOrder"`
	OfflineBufferSize    int                 `json:"offlineBufferSize"`
	OfflineTTLSeconds    int                 `json:"offlineTtlSeconds"`
	ResendLimit          int                 `json:"resendLimit"`
}

// AuditLog captures key actions inside a tenant for auditing and compliance.
type AuditLog struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID   string `json:"userId" gorm:"type:uuid"`

	// 操作信息
	Action     string `json:"action" gorm:"size:255;not null"`
	Resource   string `json:"resource" gorm:"size:255;not null"`
	ResourceID string `json:"resourceId" gorm:"size:100"`

	// 变更记录
	OldValue any `json:"oldValue" gorm:"type:jsonb"`
	NewValue any `json:"newValue" gorm:"type:jsonb"`
	Details  any `json:"details" gorm:"type:jsonb"`

	// 请求上下文
	IPAddress string `json:"ipAddress" gorm:"size:50"`
	UserAgent string `json:"userAgent" gorm:"size:500"`
	RequestID string `json:"requestId" gorm:"size:100;index"`
	SessionID string `json:"sessionId" gorm:"size:100"`

	// 结果
	Status       string `json:"status" gorm:"size:50;not null;default:success"`
	ErrorMessage string `json:"errorMessage" gorm:"type:text"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime;index"`
}
