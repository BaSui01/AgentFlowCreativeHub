package fragment

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FragmentType 片段类型
type FragmentType string

const (
	FragmentTypeInspiration FragmentType = "inspiration" // 灵感片段
	FragmentTypeMaterial    FragmentType = "material"    // 素材片段
	FragmentTypeTodo        FragmentType = "todo"        // 待办事项
	FragmentTypeNote        FragmentType = "note"        // 笔记
	FragmentTypeReference   FragmentType = "reference"   // 参考资料
)

// FragmentStatus 片段状态
type FragmentStatus string

const (
	FragmentStatusPending   FragmentStatus = "pending"   // 待处理
	FragmentStatusCompleted FragmentStatus = "completed" // 已完成
	FragmentStatusArchived  FragmentStatus = "archived"  // 已归档
)

// Fragment 片段模型
type Fragment struct {
	ID          string         `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID    string         `gorm:"type:varchar(36);not null;index" json:"tenant_id"`
	UserID      string         `gorm:"type:varchar(36);not null;index" json:"user_id"`
	WorkspaceID string         `gorm:"type:varchar(36);index" json:"workspace_id,omitempty"`       // 关联工作空间
	WorkID      string         `gorm:"type:varchar(36);index" json:"work_id,omitempty"`           // 关联作品
	ChapterID   string         `gorm:"type:varchar(36);index" json:"chapter_id,omitempty"`        // 关联章节
	
	Type        FragmentType   `gorm:"type:varchar(20);not null;index" json:"type"`               // 片段类型
	Status      FragmentStatus `gorm:"type:varchar(20);default:'pending';index" json:"status"`    // 片段状态
	
	Title       string         `gorm:"type:varchar(200);not null" json:"title"`                   // 标题
	Content     string         `gorm:"type:text;not null" json:"content"`                         // 内容
	Tags        string         `gorm:"type:text" json:"tags"`                                     // 标签（逗号分隔）
	
	Priority    int            `gorm:"default:0;index" json:"priority"`                           // 优先级（0-5）
	DueDate     *time.Time     `gorm:"index" json:"due_date,omitempty"`                           // 截止日期
	CompletedAt *time.Time     `json:"completed_at,omitempty"`                                    // 完成时间
	
	Metadata    string         `gorm:"type:jsonb" json:"metadata,omitempty"`                      // 元数据（JSON）
	
	SortOrder   int            `gorm:"default:0" json:"sort_order"`                               // 排序
	
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate GORM Hook
func (f *Fragment) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return nil
}

// TableName 指定表名
func (Fragment) TableName() string {
	return "fragments"
}

// CreateFragmentRequest 创建片段请求
type CreateFragmentRequest struct {
	Type        FragmentType `json:"type" binding:"required,oneof=inspiration material todo note reference"`
	Title       string       `json:"title" binding:"required,max=200"`
	Content     string       `json:"content" binding:"required"`
	Tags        []string     `json:"tags,omitempty"`
	Priority    *int         `json:"priority,omitempty"`
	DueDate     *time.Time   `json:"due_date,omitempty"`
	WorkspaceID string       `json:"workspace_id,omitempty"`
	WorkID      string       `json:"work_id,omitempty"`
	ChapterID   string       `json:"chapter_id,omitempty"`
	Metadata    string       `json:"metadata,omitempty"`
}

// UpdateFragmentRequest 更新片段请求
type UpdateFragmentRequest struct {
	Title    *string        `json:"title,omitempty"`
	Content  *string        `json:"content,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Status   *FragmentStatus `json:"status,omitempty"`
	Priority *int           `json:"priority,omitempty"`
	DueDate  *time.Time     `json:"due_date,omitempty"`
	Metadata *string        `json:"metadata,omitempty"`
}

// ListFragmentsRequest 列表查询请求
type ListFragmentsRequest struct {
	Type        *FragmentType   `form:"type"`
	Status      *FragmentStatus `form:"status"`
	WorkspaceID *string         `form:"workspace_id"`
	WorkID      *string         `form:"work_id"`
	ChapterID   *string         `form:"chapter_id"`
	Tags        *string         `form:"tags"`         // 逗号分隔的标签
	Keyword     *string         `form:"keyword"`      // 关键词搜索
	Page        int             `form:"page" binding:"min=1"`
	PageSize    int             `form:"page_size" binding:"min=1,max=100"`
	SortBy      string          `form:"sort_by"`      // created_at, priority, due_date
	SortOrder   string          `form:"sort_order"`   // asc, desc
}

// BatchOperationRequest 批量操作请求
type BatchOperationRequest struct {
	IDs       []string        `json:"ids" binding:"required,min=1"`
	Operation string          `json:"operation" binding:"required,oneof=complete archive delete change_status"`
	Status    *FragmentStatus `json:"status,omitempty"` // 用于 change_status
}

// FragmentStatsResponse 统计响应
type FragmentStatsResponse struct {
	TotalCount       int64          `json:"total_count"`
	ByType          map[string]int  `json:"by_type"`
	ByStatus        map[string]int  `json:"by_status"`
	PendingTodos    int64           `json:"pending_todos"`
	OverdueTodos    int64           `json:"overdue_todos"`
	CompletedToday  int64           `json:"completed_today"`
}
