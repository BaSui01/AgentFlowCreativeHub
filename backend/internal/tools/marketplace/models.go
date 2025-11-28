package marketplace

import (
	"time"
)

// ToolPackage 工具包（市场中的工具）
type ToolPackage struct {
	ID          string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string    `json:"tenantId" gorm:"type:uuid;index"`
	AuthorID    string    `json:"authorId" gorm:"type:uuid;not null;index"`
	AuthorName  string    `json:"authorName" gorm:"size:100"`

	// 基本信息
	Name        string    `json:"name" gorm:"size:100;not null;uniqueIndex:idx_package_name"`
	DisplayName string    `json:"displayName" gorm:"size:255;not null"`
	Description string    `json:"description" gorm:"type:text"`
	Category    string    `json:"category" gorm:"size:50;index"` // search, data_analysis, document, ai, utility
	Tags        []string  `json:"tags" gorm:"type:jsonb;serializer:json"`
	Icon        string    `json:"icon" gorm:"size:500"`          // 图标 URL
	Homepage    string    `json:"homepage" gorm:"size:500"`      // 主页 URL
	Repository  string    `json:"repository" gorm:"size:500"`    // 仓库 URL

	// 状态
	Status      string    `json:"status" gorm:"size:50;default:pending;index"` // pending, approved, rejected, deprecated
	Visibility  string    `json:"visibility" gorm:"size:50;default:public"`    // public, private, unlisted

	// 统计
	Downloads   int64     `json:"downloads" gorm:"default:0"`
	Stars       int64     `json:"stars" gorm:"default:0"`
	RatingAvg   float64   `json:"ratingAvg" gorm:"default:0"`
	RatingCount int       `json:"ratingCount" gorm:"default:0"`

	// 时间戳
	CreatedAt   time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}

func (ToolPackage) TableName() string {
	return "tool_packages"
}

// ToolVersion 工具版本
type ToolVersion struct {
	ID          string    `json:"id" gorm:"primaryKey;type:uuid"`
	PackageID   string    `json:"packageId" gorm:"type:uuid;not null;index"`
	TenantID    string    `json:"tenantId" gorm:"type:uuid;index"`

	// 版本信息
	Version     string    `json:"version" gorm:"size:50;not null"`             // 语义化版本号 如 1.0.0
	Changelog   string    `json:"changelog" gorm:"type:text"`                  // 更新日志
	MinVersion  string    `json:"minVersion" gorm:"size:50"`                   // 最低兼容版本

	// 工具定义（JSON）
	Definition  map[string]any `json:"definition" gorm:"type:jsonb;serializer:json"`

	// 状态
	Status      string    `json:"status" gorm:"size:50;default:active"`        // active, deprecated, yanked
	IsLatest    bool      `json:"isLatest" gorm:"default:false;index"`

	// 统计
	Downloads   int64     `json:"downloads" gorm:"default:0"`

	// 时间戳
	CreatedAt   time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	PublishedAt time.Time `json:"publishedAt" gorm:"not null"`
}

func (ToolVersion) TableName() string {
	return "tool_versions"
}

// ToolRating 工具评分
type ToolRating struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	PackageID string    `json:"packageId" gorm:"type:uuid;not null;index"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;index"`
	UserID    string    `json:"userId" gorm:"type:uuid;not null;index"`

	// 评分信息
	Rating    int       `json:"rating" gorm:"not null"`                       // 1-5 星
	Review    string    `json:"review" gorm:"type:text"`                      // 评价内容
	Version   string    `json:"version" gorm:"size:50"`                       // 评价时的版本

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

func (ToolRating) TableName() string {
	return "tool_ratings"
}

// ToolInstall 工具安装记录
type ToolInstall struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	PackageID string    `json:"packageId" gorm:"type:uuid;not null;index"`
	VersionID string    `json:"versionId" gorm:"type:uuid;not null"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID    string    `json:"userId" gorm:"type:uuid;not null;index"`

	// 安装信息
	Version   string    `json:"version" gorm:"size:50;not null"`
	Status    string    `json:"status" gorm:"size:50;default:installed"`      // installed, uninstalled

	// 时间戳
	CreatedAt   time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	UninstalledAt *time.Time `json:"uninstalledAt,omitempty"`
}

func (ToolInstall) TableName() string {
	return "tool_installs"
}

// --- 请求/响应 DTO ---

// PublishRequest 发布工具请求
type PublishRequest struct {
	Name        string         `json:"name" binding:"required"`
	DisplayName string         `json:"displayName" binding:"required"`
	Description string         `json:"description"`
	Category    string         `json:"category" binding:"required"`
	Tags        []string       `json:"tags"`
	Icon        string         `json:"icon"`
	Homepage    string         `json:"homepage"`
	Repository  string         `json:"repository"`
	Visibility  string         `json:"visibility"`
	Version     string         `json:"version" binding:"required"`
	Changelog   string         `json:"changelog"`
	Definition  map[string]any `json:"definition" binding:"required"`
}

// UpdatePackageRequest 更新工具包请求
type UpdatePackageRequest struct {
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Icon        string   `json:"icon"`
	Homepage    string   `json:"homepage"`
	Repository  string   `json:"repository"`
	Visibility  string   `json:"visibility"`
}

// PublishVersionRequest 发布新版本请求
type PublishVersionRequest struct {
	Version    string         `json:"version" binding:"required"`
	Changelog  string         `json:"changelog"`
	MinVersion string         `json:"minVersion"`
	Definition map[string]any `json:"definition" binding:"required"`
}

// RatingRequest 评分请求
type RatingRequest struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Review  string `json:"review"`
	Version string `json:"version"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query      string   `json:"query"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
	SortBy     string   `json:"sortBy"`     // downloads, stars, rating, created, updated
	SortOrder  string   `json:"sortOrder"`  // asc, desc
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
	Status     string   `json:"status"`     // 仅管理员可用
	Visibility string   `json:"visibility"` // 仅管理员可用
}

// PackageListResponse 工具包列表响应
type PackageListResponse struct {
	Packages   []ToolPackage  `json:"packages"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

// PackageDetailResponse 工具包详情响应
type PackageDetailResponse struct {
	Package       *ToolPackage   `json:"package"`
	LatestVersion *ToolVersion   `json:"latestVersion"`
	Versions      []ToolVersion  `json:"versions"`
	Ratings       []ToolRating   `json:"ratings"`
	IsInstalled   bool           `json:"isInstalled"`
	UserRating    *ToolRating    `json:"userRating,omitempty"`
}

// MarketplaceStats 市场统计
type MarketplaceStats struct {
	TotalPackages    int64            `json:"totalPackages"`
	TotalDownloads   int64            `json:"totalDownloads"`
	TotalInstalls    int64            `json:"totalInstalls"`
	CategoryStats    map[string]int64 `json:"categoryStats"`
	TopPackages      []ToolPackage    `json:"topPackages"`
	RecentPackages   []ToolPackage    `json:"recentPackages"`
}
