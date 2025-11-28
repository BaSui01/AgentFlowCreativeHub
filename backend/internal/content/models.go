package content

import (
	"time"
)

// ============================================================================
// 公开作品
// ============================================================================

// PublishStatus 发布状态
type PublishStatus string

const (
	PublishStatusDraft     PublishStatus = "draft"     // 草稿
	PublishStatusPending   PublishStatus = "pending"   // 待审核
	PublishStatusPublished PublishStatus = "published" // 已发布
	PublishStatusRejected  PublishStatus = "rejected"  // 已拒绝
	PublishStatusOffline   PublishStatus = "offline"   // 已下架
)

// PublishedWork 公开作品
type PublishedWork struct {
	ID          string        `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string        `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID      string        `json:"userId" gorm:"type:uuid;not null;index"`
	WorkspaceID string        `json:"workspaceId" gorm:"type:uuid;index"` // 关联工作区节点
	FileID      string        `json:"fileId" gorm:"type:uuid;index"`      // 关联文件

	// 基本信息
	Title       string `json:"title" gorm:"size:200;not null"`
	Summary     string `json:"summary" gorm:"size:500"`
	Content     string `json:"content" gorm:"type:text"`
	CoverImage  string `json:"coverImage" gorm:"size:500"`
	WordCount   int    `json:"wordCount" gorm:"default:0"`

	// 分类和标签
	CategoryID string   `json:"categoryId" gorm:"type:uuid;index"`
	Tags       string   `json:"tags" gorm:"type:jsonb"` // []string JSON
	
	// 发布状态
	Status       PublishStatus `json:"status" gorm:"size:20;not null;default:draft;index"`
	PublishedAt  *time.Time    `json:"publishedAt"`
	ReviewedAt   *time.Time    `json:"reviewedAt"`
	ReviewedBy   string        `json:"reviewedBy" gorm:"type:uuid"`
	RejectReason string        `json:"rejectReason" gorm:"size:500"`

	// 统计
	ViewCount    int64 `json:"viewCount" gorm:"default:0"`
	LikeCount    int64 `json:"likeCount" gorm:"default:0"`
	CommentCount int64 `json:"commentCount" gorm:"default:0"`
	ShareCount   int64 `json:"shareCount" gorm:"default:0"`
	FavoriteCount int64 `json:"favoriteCount" gorm:"default:0"`

	// 设置
	AllowComment bool `json:"allowComment" gorm:"default:true"`
	IsRecommend  bool `json:"isRecommend" gorm:"default:false;index"` // 推荐
	IsFeatured   bool `json:"isFeatured" gorm:"default:false;index"`  // 精选
	SortWeight   int  `json:"sortWeight" gorm:"default:0"`            // 排序权重

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (PublishedWork) TableName() string {
	return "published_works"
}

// ============================================================================
// 内容分类
// ============================================================================

// ContentCategory 内容分类
type ContentCategory struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string `json:"tenantId" gorm:"type:uuid;not null;index"`
	ParentID    string `json:"parentId" gorm:"type:uuid;index"` // 父分类
	Name        string `json:"name" gorm:"size:100;not null"`
	Code        string `json:"code" gorm:"size:50;uniqueIndex"`
	Description string `json:"description" gorm:"size:500"`
	Icon        string `json:"icon" gorm:"size:100"`
	CoverImage  string `json:"coverImage" gorm:"size:500"`
	SortOrder   int    `json:"sortOrder" gorm:"default:0"`
	IsActive    bool   `json:"isActive" gorm:"default:true"`
	WorkCount   int64  `json:"workCount" gorm:"default:0"` // 作品数量

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ContentCategory) TableName() string {
	return "content_categories"
}

// ContentTag 内容标签
type ContentTag struct {
	ID        string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string `json:"tenantId" gorm:"type:uuid;not null;index"`
	Name      string `json:"name" gorm:"size:50;not null;index"`
	Color     string `json:"color" gorm:"size:20"`
	UseCount  int64  `json:"useCount" gorm:"default:0"` // 使用次数
	IsHot     bool   `json:"isHot" gorm:"default:false"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (ContentTag) TableName() string {
	return "content_tags"
}

// ============================================================================
// 内容举报
// ============================================================================

// ReportStatus 举报状态
type ReportStatus string

const (
	ReportStatusPending   ReportStatus = "pending"   // 待处理
	ReportStatusReviewing ReportStatus = "reviewing" // 处理中
	ReportStatusResolved  ReportStatus = "resolved"  // 已处理
	ReportStatusRejected  ReportStatus = "rejected"  // 已驳回
)

// ReportType 举报类型
const (
	ReportTypeSpam       = "spam"       // 垃圾内容
	ReportTypePorn       = "porn"       // 色情内容
	ReportTypeViolence   = "violence"   // 暴力内容
	ReportTypeFraud      = "fraud"      // 欺诈内容
	ReportTypeCopyright  = "copyright"  // 侵权内容
	ReportTypeOther      = "other"      // 其他
)

// ContentReport 内容举报
type ContentReport struct {
	ID          string       `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string       `json:"tenantId" gorm:"type:uuid;not null;index"`
	WorkID      string       `json:"workId" gorm:"type:uuid;not null;index"` // 被举报作品
	ReporterID  string       `json:"reporterId" gorm:"type:uuid;not null;index"`
	
	// 举报信息
	ReportType  string       `json:"reportType" gorm:"size:50;not null"`
	Reason      string       `json:"reason" gorm:"size:500;not null"`
	Evidence    string       `json:"evidence" gorm:"type:text"` // 证据（图片链接等）
	
	// 处理信息
	Status      ReportStatus `json:"status" gorm:"size:20;not null;default:pending;index"`
	HandlerID   string       `json:"handlerId" gorm:"type:uuid"`
	HandleNote  string       `json:"handleNote" gorm:"size:500"`
	HandleResult string      `json:"handleResult" gorm:"size:50"` // ignore, warn, offline, ban
	HandledAt   *time.Time   `json:"handledAt"`
	
	CreatedAt   time.Time    `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time    `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ContentReport) TableName() string {
	return "content_reports"
}

// ============================================================================
// 内容统计
// ============================================================================

// ContentStats 内容统计（租户级别）
type ContentStats struct {
	TenantID       string `json:"tenantId"`
	TotalWorks     int64  `json:"totalWorks"`
	PublishedWorks int64  `json:"publishedWorks"`
	PendingWorks   int64  `json:"pendingWorks"`
	TotalViews     int64  `json:"totalViews"`
	TotalLikes     int64  `json:"totalLikes"`
	TotalComments  int64  `json:"totalComments"`
	TotalReports   int64  `json:"totalReports"`
	PendingReports int64  `json:"pendingReports"`
}

// WorkStats 单个作品统计
type WorkStats struct {
	WorkID        string    `json:"workId"`
	ViewCount     int64     `json:"viewCount"`
	LikeCount     int64     `json:"likeCount"`
	CommentCount  int64     `json:"commentCount"`
	ShareCount    int64     `json:"shareCount"`
	FavoriteCount int64     `json:"favoriteCount"`
	LastViewAt    time.Time `json:"lastViewAt"`
}

// ============================================================================
// 请求结构
// ============================================================================

// PublishWorkRequest 发布作品请求
type PublishWorkRequest struct {
	TenantID    string   `json:"tenantId"`
	UserID      string   `json:"userId"`
	WorkspaceID string   `json:"workspaceId"`
	FileID      string   `json:"fileId"`
	Title       string   `json:"title" binding:"required"`
	Summary     string   `json:"summary"`
	Content     string   `json:"content"`
	CoverImage  string   `json:"coverImage"`
	CategoryID  string   `json:"categoryId"`
	Tags        []string `json:"tags"`
	AllowComment bool    `json:"allowComment"`
}

// UpdateWorkRequest 更新作品请求
type UpdateWorkRequest struct {
	Title        *string   `json:"title"`
	Summary      *string   `json:"summary"`
	Content      *string   `json:"content"`
	CoverImage   *string   `json:"coverImage"`
	CategoryID   *string   `json:"categoryId"`
	Tags         []string  `json:"tags"`
	AllowComment *bool     `json:"allowComment"`
}

// ReviewWorkRequest 审核作品请求
type ReviewWorkRequest struct {
	WorkID       string `json:"workId" binding:"required"`
	Action       string `json:"action" binding:"required"` // approve, reject
	RejectReason string `json:"rejectReason"`
	ReviewerID   string `json:"reviewerId"`
}

// RecommendWorkRequest 推荐作品请求
type RecommendWorkRequest struct {
	WorkID      string `json:"workId" binding:"required"`
	IsRecommend bool   `json:"isRecommend"`
	IsFeatured  bool   `json:"isFeatured"`
	SortWeight  int    `json:"sortWeight"`
}

// CreateReportRequest 创建举报请求
type CreateReportRequest struct {
	TenantID   string `json:"tenantId"`
	WorkID     string `json:"workId" binding:"required"`
	ReporterID string `json:"reporterId"`
	ReportType string `json:"reportType" binding:"required"`
	Reason     string `json:"reason" binding:"required"`
	Evidence   string `json:"evidence"`
}

// HandleReportRequest 处理举报请求
type HandleReportRequest struct {
	ReportID     string `json:"reportId" binding:"required"`
	Action       string `json:"action" binding:"required"` // ignore, warn, offline, ban
	HandleNote   string `json:"handleNote"`
	HandlerID    string `json:"handlerId"`
}

// ListWorksQuery 作品列表查询
type ListWorksQuery struct {
	TenantID   string        `json:"tenantId"`
	UserID     string        `json:"userId"`
	CategoryID string        `json:"categoryId"`
	Tag        string        `json:"tag"`
	Status     PublishStatus `json:"status"`
	Keyword    string        `json:"keyword"`
	SortBy     string        `json:"sortBy"` // latest, popular, recommend
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
}

// CreateCategoryRequest 创建分类请求
type CreateCategoryRequest struct {
	TenantID    string `json:"tenantId"`
	ParentID    string `json:"parentId"`
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	CoverImage  string `json:"coverImage"`
}

// SearchWorksRequest 搜索作品请求（支持全文搜索和高级筛选）
type SearchWorksRequest struct {
	TenantID string `json:"tenantId"`
	
	// 关键词搜索（支持标题、摘要、内容全文搜索）
	Keyword string `json:"keyword"`
	
	// 高级筛选
	UserID       string          `json:"userId"`       // 作者ID
	CategoryIDs  []string        `json:"categoryIds"`  // 分类ID列表（OR关系）
	Tags         []string        `json:"tags"`         // 标签列表（AND关系）
	Status       PublishStatus   `json:"status"`       // 发布状态
	IsRecommend  *bool           `json:"isRecommend"`  // 是否推荐
	IsFeatured   *bool           `json:"isFeatured"`   // 是否精选
	
	// 统计筛选
	MinViewCount  int64 `json:"minViewCount"`  // 最小浏览量
	MinLikeCount  int64 `json:"minLikeCount"`  // 最小点赞数
	MinWordCount  int   `json:"minWordCount"`  // 最小字数
	MaxWordCount  int   `json:"maxWordCount"`  // 最大字数
	
	// 日期范围
	PublishedAfter  *time.Time `json:"publishedAfter"`  // 发布时间起始
	PublishedBefore *time.Time `json:"publishedBefore"` // 发布时间结束
	
	// 排序和分页
	SortBy   string `json:"sortBy"`   // latest, popular, recommend, views, likes
	SortDesc bool   `json:"sortDesc"` // 是否降序，默认true
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// ============================================================================
// 作品评论和点赞
// ============================================================================

// WorkComment 作品评论
type WorkComment struct {
	ID          string     `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string     `json:"tenantId" gorm:"type:uuid;not null;index"`
	WorkID      string     `json:"workId" gorm:"type:uuid;not null;index"`
	UserID      string     `json:"userId" gorm:"type:uuid;not null;index"`
	Content     string     `json:"content" gorm:"type:text;not null"`
	ReplyToID   *string    `json:"replyToId" gorm:"type:uuid;index"` // 回复的评论ID
	LikeCount   int        `json:"likeCount" gorm:"default:0"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
	
	// 关联数据（不存储在数据库）
	Username    string `json:"username" gorm:"-"`
	Avatar      string `json:"avatar" gorm:"-"`
	ReplyToUser string `json:"replyToUser,omitempty" gorm:"-"`
}

func (WorkComment) TableName() string {
	return "work_comments"
}

// WorkLike 作品点赞
type WorkLike struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	WorkID    string    `json:"workId" gorm:"type:uuid;not null;index"`
	UserID    string    `json:"userId" gorm:"type:uuid;not null;index"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (WorkLike) TableName() string {
	return "work_likes"
}

// CreateCommentRequest 创建评论请求
type CreateCommentRequest struct {
	WorkID    string  `json:"workId" binding:"required"`
	Content   string  `json:"content" binding:"required"`
	ReplyToID *string `json:"replyToId"` // 可选，回复的评论ID
}

// CommentListResponse 评论列表响应
type CommentListResponse struct {
	Comments   []WorkComment `json:"comments"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	HasMore    bool          `json:"hasMore"`
}

// LikeResponse 点赞响应
type LikeResponse struct {
	IsLiked   bool  `json:"isLiked"`   // 当前用户是否点赞
	LikeCount int64 `json:"likeCount"` // 总点赞数
}
