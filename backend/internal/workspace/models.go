package workspace

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkspaceNode 代表工作区中的文件或文件夹
type WorkspaceNode struct {
	ID        string         `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID  string         `gorm:"type:uuid;not null;index:idx_workspace_node_tenant" json:"tenant_id"`
	ParentID  *string        `gorm:"type:uuid;index:idx_workspace_node_parent" json:"parent_id"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Slug      string         `gorm:"size:255;not null" json:"slug"`
	Type      string         `gorm:"size:20;not null" json:"type"`
	NodePath  string         `gorm:"size:1024;not null;index:idx_workspace_node_path" json:"node_path"`
	Category  string         `gorm:"size:50" json:"category"`
	SortOrder int            `gorm:"default:0" json:"sort_order"`
	Metadata  string        `gorm:"type:jsonb" json:"metadata"`
	CreatedBy string         `gorm:"type:uuid" json:"created_by"`
	UpdatedBy string         `gorm:"type:uuid" json:"updated_by"`
	CreatedAt time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 设置默认 ID 与时间戳
func (n *WorkspaceNode) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	if n.UpdatedAt.IsZero() {
		n.UpdatedAt = n.CreatedAt
	}
	return nil
}

// BeforeUpdate 更新时间戳
func (n *WorkspaceNode) BeforeUpdate(tx *gorm.DB) error {
	n.UpdatedAt = time.Now().UTC()
	return nil
}

// WorkspaceFile 工作区文件的元数据
type WorkspaceFile struct {
	ID              string     `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID        string     `gorm:"type:uuid;not null;index:idx_workspace_file_tenant" json:"tenant_id"`
	NodeID          string     `gorm:"type:uuid;not null;uniqueIndex" json:"node_id"`
	LatestVersionID string     `gorm:"type:uuid" json:"latest_version_id"`
	Category        string     `gorm:"size:50" json:"category"`
	AutoTags        []string   `gorm:"type:jsonb" json:"auto_tags"`
	ReviewStatus    string     `gorm:"size:50;default:'published'" json:"review_status"`
	ApproverID      *string    `gorm:"type:uuid" json:"approver_id"`
	ApprovedAt      *time.Time `json:"approved_at"`
	CreatedBy       string     `gorm:"type:uuid" json:"created_by"`
	UpdatedBy       string     `gorm:"type:uuid" json:"updated_by"`
	CreatedAt       time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"not null" json:"updated_at"`
}

func (f *WorkspaceFile) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	if f.UpdatedAt.IsZero() {
		f.UpdatedAt = f.CreatedAt
	}
	return nil
}

func (f *WorkspaceFile) BeforeUpdate(tx *gorm.DB) error {
	f.UpdatedAt = time.Now().UTC()
	return nil
}

// WorkspaceFileVersion 文件的版本内容
type WorkspaceFileVersion struct {
	ID        string        `gorm:"type:uuid;primaryKey" json:"id"`
	FileID    string        `gorm:"type:uuid;not null;index:idx_workspace_version_file" json:"file_id"`
	TenantID  string        `gorm:"type:uuid;not null;index:idx_workspace_version_tenant" json:"tenant_id"`
	Content   string        `gorm:"type:text" json:"content"`
	Summary   string        `gorm:"type:text" json:"summary"`
	AgentID   string        `gorm:"type:uuid" json:"agent_id"`
	ToolName  string        `gorm:"size:100" json:"tool_name"`
	Metadata  string        `gorm:"type:jsonb" json:"metadata"`
	CreatedBy string        `gorm:"type:uuid" json:"created_by"`
	CreatedAt time.Time     `gorm:"not null" json:"created_at"`
}

func (v *WorkspaceFileVersion) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	return nil
}

// WorkspaceStagingFile 暂存区的生成文件
type WorkspaceStagingFile struct {
	ID              string        `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID        string        `gorm:"type:uuid;not null;index:idx_workspace_staging_tenant" json:"tenant_id"`
	FileType        string        `gorm:"size:50;not null" json:"file_type"`
	SuggestedName   string        `gorm:"size:255;not null" json:"suggested_name"`
	SuggestedFolder string        `gorm:"size:255;not null" json:"suggested_folder"`
	SuggestedPath   string        `gorm:"size:1024;not null" json:"suggested_path"`
	Content         string        `gorm:"type:text" json:"content"`
	Summary         string        `gorm:"type:text" json:"summary"`
	SourceAgentID   string        `gorm:"type:uuid" json:"source_agent_id"`
	SourceAgentName string        `gorm:"size:255" json:"source_agent_name"`
	SourceCommand   string        `gorm:"size:255" json:"source_command"`
	Status          string        `gorm:"size:50;default:'pending'" json:"status"`
	ReviewerID      *string       `gorm:"type:uuid" json:"reviewer_id"`
	ReviewerName    *string       `gorm:"size:255" json:"reviewer_name"`
	ReviewedAt      *time.Time    `json:"reviewed_at"`
	Metadata        string        `gorm:"type:jsonb" json:"metadata"`
	CreatedBy       string        `gorm:"type:uuid" json:"created_by"`
	UpdatedBy       string        `gorm:"type:uuid" json:"updated_by"`
	CreatedAt       time.Time     `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time     `gorm:"not null" json:"updated_at"`
}

func (s *WorkspaceStagingFile) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = s.CreatedAt
	}
	return nil
}

func (s *WorkspaceStagingFile) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// WorkspaceContextLink 记录命令与上下文的绑定
type WorkspaceContextLink struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID  string    `gorm:"type:uuid;not null;index:idx_workspace_context_tenant" json:"tenant_id"`
	AgentID   string    `gorm:"type:uuid" json:"agent_id"`
	SessionID string    `gorm:"size:255" json:"session_id"`
	Mentions  []string  `gorm:"type:jsonb" json:"mentions"`
	Commands  []string  `gorm:"type:jsonb" json:"commands"`
	NodeIDs   []string  `gorm:"type:jsonb" json:"node_ids"`
	Notes     string    `gorm:"type:text" json:"notes"`
	Snapshot  string    `gorm:"type:text" json:"snapshot"`
	CreatedBy string    `gorm:"type:uuid" json:"created_by"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

func (l *WorkspaceContextLink) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	return nil
}
