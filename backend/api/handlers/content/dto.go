package content

import (
	"time"

	"backend/api/handlers/common"
)

// ========== 作品 ==========

// PublishWorkRequest 发布作品请求
type PublishWorkRequest struct {
	TenantID    string                 `json:"-"`
	UserID      string                 `json:"-"`
	WorkspaceID string                 `json:"workspaceId"`
	FileID      string                 `json:"fileId"`
	Title       string                 `json:"title" binding:"required,min=1,max=200"`
	Summary     string                 `json:"summary" binding:"max=500"`
	Content     string                 `json:"content"`
	CoverImage  string                 `json:"coverImage"`
	CategoryID  string                 `json:"categoryId"`
	Tags        []string               `json:"tags"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UpdateWorkRequest 更新作品请求
type UpdateWorkRequest struct {
	Title      *string                `json:"title" binding:"omitempty,min=1,max=200"`
	Summary    *string                `json:"summary" binding:"omitempty,max=500"`
	Content    *string                `json:"content"`
	CoverImage *string                `json:"coverImage"`
	CategoryID *string                `json:"categoryId"`
	Tags       []string               `json:"tags"`
	Status     *string                `json:"status"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// WorkListResponse 作品列表响应
type WorkListResponse struct {
	Works      interface{}           `json:"works"`
	Pagination common.PaginationMeta `json:"pagination"`
}

// WorkDetailResponse 作品详情响应
type WorkDetailResponse struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Summary      string                 `json:"summary"`
	Content      string                 `json:"content"`
	CoverImage   string                 `json:"coverImage"`
	WordCount    int64                  `json:"wordCount"`
	Status       string                 `json:"status"`
	PublishedAt  *time.Time             `json:"publishedAt"`
	ViewCount    int64                  `json:"viewCount"`
	LikeCount    int64                  `json:"likeCount"`
	CommentCount int64                  `json:"commentCount"`
	ShareCount   int64                  `json:"shareCount"`
	IsLiked      bool                   `json:"isLiked"`
	Category     interface{}            `json:"category,omitempty"`
	Tags         []string               `json:"tags"`
	Author       interface{}            `json:"author"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

// ========== 评论 ==========

// CreateCommentRequest 创建评论请求
type CreateCommentRequest struct {
	WorkID    string `json:"-"`
	Content   string `json:"content" binding:"required,min=1,max=2000"`
	ReplyToID string `json:"replyToId"`
}

// DeleteCommentRequest 删除评论请求
type DeleteCommentRequest struct {
	CommentID string `json:"commentId" binding:"required"`
}

// CommentResponse 评论响应
type CommentResponse struct {
	ID        string      `json:"id"`
	Content   string      `json:"content"`
	LikeCount int         `json:"likeCount"`
	ReplyTo   interface{} `json:"replyTo,omitempty"`
	Author    interface{} `json:"author"`
	CreatedAt time.Time   `json:"createdAt"`
	Replies   interface{} `json:"replies,omitempty"`
}

// CommentListResponse 评论列表响应
type CommentListResponse struct {
	Comments   []CommentResponse     `json:"comments"`
	Pagination common.PaginationMeta `json:"pagination"`
}

// ========== 点赞 ==========

// ToggleLikeResponse 点赞响应
type ToggleLikeResponse struct {
	Liked     bool  `json:"liked"`
	LikeCount int64 `json:"likeCount"`
}

// ========== 分类 ==========

// CreateCategoryRequest 创建分类请求
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=50"`
	Description string `json:"description"`
	ParentID    string `json:"parentId"`
	SortOrder   int    `json:"sortOrder"`
	Icon        string `json:"icon"`
}

// UpdateCategoryRequest 更新分类请求
type UpdateCategoryRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=50"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sortOrder"`
	Icon        *string `json:"icon"`
	IsActive    *bool   `json:"isActive"`
}

// ========== 举报 ==========

// CreateReportRequest 创建举报请求
type CreateReportRequest struct {
	TargetType string `json:"targetType" binding:"required,oneof=work comment"`
	TargetID   string `json:"targetId" binding:"required"`
	Reason     string `json:"reason" binding:"required,min=1,max=500"`
	Category   string `json:"category"`
}

// HandleReportRequest 处理举报请求
type HandleReportRequest struct {
	Action  string `json:"action" binding:"required,oneof=approve reject ignore"`
	Comment string `json:"comment"`
}

// ========== 搜索 ==========

// SearchWorksRequest 搜索作品请求
type SearchWorksRequest struct {
	Keyword    string   `json:"keyword"`
	CategoryID string   `json:"categoryId"`
	Tags       []string `json:"tags"`
	Status     string   `json:"status"`
	SortBy     string   `json:"sortBy"`
	SortOrder  string   `json:"sortOrder"`
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
}
