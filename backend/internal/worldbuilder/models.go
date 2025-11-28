package worldbuilder

import (
	"time"
)

// ============================================================================
// 世界观设定
// ============================================================================

// WorldSetting 世界观设定
type WorldSetting struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	WorkID    string    `json:"workId" gorm:"type:uuid;index"`           // 关联的作品ID
	
	// 基本信息
	Name        string `json:"name" gorm:"size:200;not null"`
	Description string `json:"description" gorm:"type:text"`
	Genre       string `json:"genre" gorm:"size:50;index"`              // 类型：fantasy, scifi, wuxia, modern, etc.
	
	// 设定内容（JSON格式）
	Content     string `json:"content" gorm:"type:text"`                // 完整设定内容
	
	// 版本管理
	LatestVersionID string `json:"latestVersionId" gorm:"type:uuid"`
	VersionCount    int    `json:"versionCount" gorm:"default:1"`
	
	// 状态
	Status    string `json:"status" gorm:"size:20;default:draft"`      // draft, published, archived
	IsLocked  bool   `json:"isLocked" gorm:"default:false"`
	
	// 统计
	EntityCount   int `json:"entityCount" gorm:"default:0"`            // 实体数量
	RelationCount int `json:"relationCount" gorm:"default:0"`          // 关系数量
	
	// 创建信息
	CreatedBy string    `json:"createdBy" gorm:"type:uuid"`
	UpdatedBy string    `json:"updatedBy" gorm:"type:uuid"`
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (WorldSetting) TableName() string {
	return "world_settings"
}

// Genre 类型常量
const (
	GenreFantasy  = "fantasy"  // 玄幻
	GenreWuxia    = "wuxia"    // 武侠
	GenreXianxia  = "xianxia"  // 仙侠
	GenreSciFi    = "scifi"    // 科幻
	GenreModern   = "modern"   // 都市
	GenreHistory  = "history"  // 历史
	GenreMystery  = "mystery"  // 悬疑
	GenreRomance  = "romance"  // 言情
	GenreCustom   = "custom"   // 自定义
)

// ============================================================================
// 世界观版本（快照）
// ============================================================================

// SettingVersion 设定版本（历史快照）
type SettingVersion struct {
	ID         string    `json:"id" gorm:"primaryKey;type:uuid"`
	SettingID  string    `json:"settingId" gorm:"type:uuid;not null;index"`
	TenantID   string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	
	// 快照内容
	Content    string    `json:"content" gorm:"type:text;not null"`     // JSON 格式的完整快照
	Summary    string    `json:"summary" gorm:"size:500"`               // 版本说明
	
	// 版本号
	VersionNum int       `json:"versionNum" gorm:"not null"`
	
	// 变更信息
	ChangeType string    `json:"changeType" gorm:"size:20"`             // create, update, generate, revert
	ChangeDesc string    `json:"changeDesc" gorm:"size:500"`            // 变更描述
	
	CreatedBy  string    `json:"createdBy" gorm:"type:uuid"`
	CreatedAt  time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
}

// TableName 指定表名
func (SettingVersion) TableName() string {
	return "setting_versions"
}

// ============================================================================
// 设定实体
// ============================================================================

// SettingEntity 设定实体（角色、地点、物品、势力等）
type SettingEntity struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	SettingID string    `json:"settingId" gorm:"type:uuid;not null;index"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	
	// 实体信息
	Name        string         `json:"name" gorm:"size:200;not null"`
	Type        string         `json:"type" gorm:"size:50;not null;index"` // character, location, item, faction, event, concept
	Category    string         `json:"category" gorm:"size:50"`            // 子类别
	Description string         `json:"description" gorm:"type:text"`
	
	// 详细属性（JSON格式，根据类型不同结构不同）
	Attributes  map[string]any `json:"attributes" gorm:"type:jsonb;serializer:json"`
	
	// 图片
	ImageURL    string         `json:"imageUrl" gorm:"size:500"`
	
	// 标签
	Tags        []string       `json:"tags" gorm:"type:jsonb;serializer:json"`
	
	// 排序
	SortOrder   int            `json:"sortOrder" gorm:"default:0"`
	
	// 父实体（支持层级）
	ParentID    string         `json:"parentId" gorm:"type:uuid;index"`
	
	CreatedBy   string         `json:"createdBy" gorm:"type:uuid"`
	CreatedAt   time.Time      `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time      `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (SettingEntity) TableName() string {
	return "setting_entities"
}

// EntityType 实体类型常量
const (
	EntityTypeCharacter = "character" // 角色
	EntityTypeLocation  = "location"  // 地点
	EntityTypeItem      = "item"      // 物品
	EntityTypeFaction   = "faction"   // 势力
	EntityTypeEvent     = "event"     // 事件
	EntityTypeConcept   = "concept"   // 概念（如修炼体系、世界规则）
	EntityTypeCreature  = "creature"  // 生物
	EntityTypeSkill     = "skill"     // 技能/法术
)

// ============================================================================
// 实体关系
// ============================================================================

// EntityRelation 实体关系
type EntityRelation struct {
	ID         string    `json:"id" gorm:"primaryKey;type:uuid"`
	SettingID  string    `json:"settingId" gorm:"type:uuid;not null;index"`
	TenantID   string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	
	// 关系两端
	SourceID   string    `json:"sourceId" gorm:"type:uuid;not null;index"`
	TargetID   string    `json:"targetId" gorm:"type:uuid;not null;index"`
	
	// 关系信息
	Type       string    `json:"type" gorm:"size:50;not null;index"`    // parent, ally, enemy, belong, love, hate, etc.
	Strength   int       `json:"strength" gorm:"default:3"`             // 关系强度 1-5
	Direction  string    `json:"direction" gorm:"size:20;default:both"` // both, forward, backward
	
	// 关系描述
	Description string   `json:"description" gorm:"type:text"`
	
	// 动态状态
	Dynamic    string    `json:"dynamic" gorm:"size:20;default:stable"` // stable, tense, changing
	
	CreatedAt  time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt  time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (EntityRelation) TableName() string {
	return "entity_relations"
}

// RelationType 关系类型常量
const (
	RelationTypeParent   = "parent"   // 父子
	RelationTypeAlly     = "ally"     // 同盟
	RelationTypeEnemy    = "enemy"    // 敌对
	RelationTypeBelong   = "belong"   // 从属
	RelationTypeLove     = "love"     // 爱慕
	RelationTypeHate     = "hate"     // 仇恨
	RelationTypeMaster   = "master"   // 师徒
	RelationTypeFriend   = "friend"   // 朋友
	RelationTypeRival    = "rival"    // 对手
	RelationTypeFamily   = "family"   // 亲属
)

// ============================================================================
// 设定模板
// ============================================================================

// SettingTemplate 设定生成模板
type SettingTemplate struct {
	ID          string         `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string         `json:"tenantId" gorm:"type:uuid;index"`   // 空表示系统模板
	
	Name        string         `json:"name" gorm:"size:100;not null"`
	Description string         `json:"description" gorm:"size:500"`
	Genre       string         `json:"genre" gorm:"size:50;not null;index"`
	
	// 模板内容（JSON格式的结构定义）
	Structure   map[string]any `json:"structure" gorm:"type:jsonb;serializer:json"`
	
	// 生成提示词
	Prompt      string         `json:"prompt" gorm:"type:text"`
	
	// 示例
	Example     string         `json:"example" gorm:"type:text"`
	
	// 是否系统内置
	IsBuiltin   bool           `json:"isBuiltin" gorm:"default:false"`
	IsActive    bool           `json:"isActive" gorm:"default:true"`
	
	// 使用统计
	UsageCount  int            `json:"usageCount" gorm:"default:0"`
	
	CreatedAt   time.Time      `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time      `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// TableName 指定表名
func (SettingTemplate) TableName() string {
	return "setting_templates"
}

// ============================================================================
// 请求/响应类型
// ============================================================================

// CreateSettingRequest 创建世界观请求
type CreateSettingRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Genre       string `json:"genre"`
	WorkID      string `json:"workId"`
}

// GenerateSettingRequest AI 生成设定请求
type GenerateSettingRequest struct {
	SettingID   string   `json:"settingId"`                    // 空则创建新设定
	CoreIdea    string   `json:"coreIdea" binding:"required"`  // 核心创意
	Genre       string   `json:"genre"`                        // 类型
	Scale       string   `json:"scale"`                        // 规模：small, medium, large
	TemplateID  string   `json:"templateId"`                   // 使用的模板
	Focus       []string `json:"focus"`                        // 重点生成：world, characters, factions, etc.
	Constraints string   `json:"constraints"`                  // 约束条件
}

// ModifySettingRequest 增量修改设定请求
type ModifySettingRequest struct {
	SettingID   string `json:"settingId" binding:"required"`
	EntityID    string `json:"entityId"`                       // 空则修改整体设定
	Instruction string `json:"instruction" binding:"required"` // 修改指令
	Context     string `json:"context"`                        // 上下文
}

// CreateEntityRequest 创建实体请求
type CreateEntityRequest struct {
	SettingID   string         `json:"settingId" binding:"required"`
	Name        string         `json:"name" binding:"required"`
	Type        string         `json:"type" binding:"required"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Attributes  map[string]any `json:"attributes"`
	ParentID    string         `json:"parentId"`
	Tags        []string       `json:"tags"`
}

// CreateRelationRequest 创建关系请求
type CreateRelationRequest struct {
	SettingID   string `json:"settingId" binding:"required"`
	SourceID    string `json:"sourceId" binding:"required"`
	TargetID    string `json:"targetId" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Strength    int    `json:"strength"`
	Description string `json:"description"`
}

// EntityQuery 实体查询
type EntityQuery struct {
	SettingID string   `json:"settingId"`
	Type      string   `json:"type"`
	Category  string   `json:"category"`
	Keyword   string   `json:"keyword"`
	Tags      []string `json:"tags"`
	ParentID  string   `json:"parentId"`
	Page      int      `json:"page"`
	PageSize  int      `json:"pageSize"`
}

// VersionDiff 版本差异
type VersionDiff struct {
	BaseVersion   VersionMeta `json:"baseVersion"`
	TargetVersion VersionMeta `json:"targetVersion"`
	Changes       []DiffItem  `json:"changes"`
}

// VersionMeta 版本元信息
type VersionMeta struct {
	ID         string    `json:"id"`
	VersionNum int       `json:"versionNum"`
	Summary    string    `json:"summary"`
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

// DiffItem 差异项
type DiffItem struct {
	Path      string `json:"path"`      // 变更路径
	Type      string `json:"type"`      // add, remove, modify
	OldValue  string `json:"oldValue"`
	NewValue  string `json:"newValue"`
}

// RelationGraph 关系图数据
type RelationGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode 图节点
type GraphNode struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Category string         `json:"category"`
	Data     map[string]any `json:"data"`
}

// GraphEdge 图边
type GraphEdge struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	Type        string `json:"type"`
	Strength    int    `json:"strength"`
	Description string `json:"description"`
}

// SettingStats 设定统计
type SettingStats struct {
	SettingID      string         `json:"settingId"`
	TotalEntities  int64          `json:"totalEntities"`
	TotalRelations int64          `json:"totalRelations"`
	TotalVersions  int64          `json:"totalVersions"`
	ByType         map[string]int `json:"byType"`
	ByCategory     map[string]int `json:"byCategory"`
	LastUpdated    time.Time      `json:"lastUpdated"`
}


// ============================================================================
// 侧边栏查阅
// ============================================================================

// EntitySummary 实体摘要（用于侧边栏显示）
type EntitySummary struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// WorkSettingsSummary 作品设定摘要（用于侧边栏）
type WorkSettingsSummary struct {
	WorkID      string          `json:"work_id"`
	SettingID   string          `json:"setting_id"`
	SettingName string          `json:"setting_name"`
	Entities    []EntitySummary `json:"entities"`
}
