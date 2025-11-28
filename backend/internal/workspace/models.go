package workspace

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
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
	Metadata  string         `gorm:"type:jsonb" json:"metadata"`
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
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	FileID    string    `gorm:"type:uuid;not null;index:idx_workspace_version_file" json:"file_id"`
	TenantID  string    `gorm:"type:uuid;not null;index:idx_workspace_version_tenant" json:"tenant_id"`
	Content   string    `gorm:"type:text" json:"content"`
	Summary   string    `gorm:"type:text" json:"summary"`
	AgentID   string    `gorm:"type:uuid" json:"agent_id"`
	ToolName  string    `gorm:"size:100" json:"tool_name"`
	Metadata  string    `gorm:"type:jsonb" json:"metadata"`
	CreatedBy string    `gorm:"type:uuid" json:"created_by"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
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
	ID                   string         `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID             string         `gorm:"type:uuid;not null;index:idx_workspace_staging_tenant" json:"tenant_id"`
	FileType             string         `gorm:"size:50;not null" json:"file_type"`
	SuggestedName        string         `gorm:"size:255;not null" json:"suggested_name"`
	SuggestedFolder      string         `gorm:"size:255;not null" json:"suggested_folder"`
	SuggestedPath        string         `gorm:"size:1024;not null" json:"suggested_path"`
	Content              string         `gorm:"type:text" json:"content"`
	Summary              string         `gorm:"type:text" json:"summary"`
	SourceAgentID        string         `gorm:"type:uuid" json:"source_agent_id"`
	SourceAgentName      string         `gorm:"size:255" json:"source_agent_name"`
	SourceCommand        string         `gorm:"size:255" json:"source_command"`
	Status               string         `gorm:"size:50;default:'drafted'" json:"status"`
	ReviewerID           *string        `gorm:"type:uuid" json:"reviewer_id"`
	ReviewerName         *string        `gorm:"size:255" json:"reviewer_name"`
	SecondaryReviewerID  *string        `gorm:"type:uuid" json:"secondaryReviewerId"`
	ReviewedAt           *time.Time     `json:"reviewed_at"`
	ReviewToken          string         `gorm:"size:64" json:"reviewToken"`
	SecondaryReviewToken string         `gorm:"size:64" json:"secondaryReviewToken"`
	RequiresSecondary    bool           `gorm:"default:false" json:"requiresSecondary"`
	SLAExpiresAt         *time.Time     `json:"slaExpiresAt"`
	LastStatusTransition time.Time      `json:"lastStatusTransition"`
	AuditTrail           datatypes.JSON `gorm:"type:jsonb" json:"auditTrail"`
	ResubmitCount        int            `gorm:"default:0" json:"resubmitCount"`
	Metadata             string         `gorm:"type:jsonb" json:"metadata"`
	CreatedBy            string         `gorm:"type:uuid" json:"created_by"`
	UpdatedBy            string         `gorm:"type:uuid" json:"updated_by"`
	CreatedAt            time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt            time.Time      `gorm:"not null" json:"updated_at"`
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

// AgentArtifact 智能体产出物记录
type AgentArtifact struct {
	ID           string         `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID     string         `gorm:"type:uuid;not null;index:idx_artifact_tenant" json:"tenant_id"`
	AgentID      string         `gorm:"type:uuid;not null;index:idx_artifact_agent" json:"agent_id"`
	AgentName    string         `gorm:"size:255;not null" json:"agent_name"`
	SessionID    string         `gorm:"size:255;index:idx_artifact_session" json:"session_id"`
	NodeID       string         `gorm:"type:uuid;index:idx_artifact_node" json:"node_id"`
	ArtifactType string         `gorm:"size:50;not null" json:"artifact_type"`
	FileName     string         `gorm:"size:255;not null" json:"file_name"`
	FilePath     string         `gorm:"size:1024;not null" json:"file_path"`
	FileSize     int64          `gorm:"default:0" json:"file_size"`
	ContentHash  string         `gorm:"size:64" json:"content_hash"`
	Summary      string         `gorm:"type:text" json:"summary"`
	TaskType     string         `gorm:"size:50" json:"task_type"`
	ToolName     string         `gorm:"size:100" json:"tool_name"`
	Sequence     int            `gorm:"default:1" json:"sequence"`
	Status       string         `gorm:"size:50;default:'created'" json:"status"`
	Metadata     datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	CreatedBy    string         `gorm:"type:uuid" json:"created_by"`
	CreatedAt    time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"not null" json:"updated_at"`
}

func (a *AgentArtifact) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = a.CreatedAt
	}
	return nil
}

func (a *AgentArtifact) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// AgentWorkspace 智能体专属工作空间
type AgentWorkspace struct {
	ID             string         `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID       string         `gorm:"type:uuid;not null;index:idx_agent_workspace_tenant" json:"tenant_id"`
	AgentID        string         `gorm:"type:uuid;not null;uniqueIndex:idx_agent_workspace_unique" json:"agent_id"`
	AgentName      string         `gorm:"size:255;not null" json:"agent_name"`
	RootNodeID     string         `gorm:"type:uuid" json:"root_node_id"`
	OutputsNodeID  string         `gorm:"type:uuid" json:"outputs_node_id"`
	DraftsNodeID   string         `gorm:"type:uuid" json:"drafts_node_id"`
	LogsNodeID     string         `gorm:"type:uuid" json:"logs_node_id"`
	ArtifactCount  int            `gorm:"default:0" json:"artifact_count"`
	TotalFileSize  int64          `gorm:"default:0" json:"total_file_size"`
	LastActivityAt *time.Time     `json:"last_activity_at"`
	Settings       datatypes.JSON `gorm:"type:jsonb" json:"settings"`
	CreatedAt      time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"not null" json:"updated_at"`
}

func (aw *AgentWorkspace) BeforeCreate(tx *gorm.DB) error {
	if aw.ID == "" {
		aw.ID = uuid.New().String()
	}
	if aw.CreatedAt.IsZero() {
		aw.CreatedAt = time.Now().UTC()
	}
	if aw.UpdatedAt.IsZero() {
		aw.UpdatedAt = aw.CreatedAt
	}
	return nil
}

func (aw *AgentWorkspace) BeforeUpdate(tx *gorm.DB) error {
	aw.UpdatedAt = time.Now().UTC()
	return nil
}

// SessionWorkspace 会话专属工作空间
type SessionWorkspace struct {
	ID              string         `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID        string         `gorm:"type:uuid;not null;index:idx_session_workspace_tenant" json:"tenant_id"`
	SessionID       string         `gorm:"size:255;not null;uniqueIndex:idx_session_workspace_unique" json:"session_id"`
	RootNodeID      string         `gorm:"type:uuid" json:"root_node_id"`
	ContextNodeID   string         `gorm:"type:uuid" json:"context_node_id"`
	ArtifactsNodeID string         `gorm:"type:uuid" json:"artifacts_node_id"`
	HistoryNodeID   string         `gorm:"type:uuid" json:"history_node_id"`
	ArtifactCount   int            `gorm:"default:0" json:"artifact_count"`
	AgentIDs        []string       `gorm:"type:jsonb" json:"agent_ids"`
	Settings        datatypes.JSON `gorm:"type:jsonb" json:"settings"`
	ExpiresAt       *time.Time     `json:"expires_at"`
	CreatedAt       time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"not null" json:"updated_at"`
}

func (sw *SessionWorkspace) BeforeCreate(tx *gorm.DB) error {
	if sw.ID == "" {
		sw.ID = uuid.New().String()
	}
	if sw.CreatedAt.IsZero() {
		sw.CreatedAt = time.Now().UTC()
	}
	if sw.UpdatedAt.IsZero() {
		sw.UpdatedAt = sw.CreatedAt
	}
	return nil
}

func (sw *SessionWorkspace) BeforeUpdate(tx *gorm.DB) error {
	sw.UpdatedAt = time.Now().UTC()
	return nil
}

// FileUpload 文件上传记录
type FileUpload struct {
	ID           string     `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID     string     `gorm:"type:uuid;not null;index:idx_file_upload_tenant" json:"tenant_id"`
	FileName     string     `gorm:"size:255;not null" json:"file_name"`
	OriginalName string     `gorm:"size:255;not null" json:"original_name"`
	MimeType     string     `gorm:"size:100" json:"mime_type"`
	FileSize     int64      `gorm:"not null" json:"file_size"`
	StoragePath  string     `gorm:"size:1024;not null" json:"storage_path"`
	NodeID       *string    `gorm:"type:uuid;index:idx_file_upload_node" json:"node_id"`
	Hash         string     `gorm:"size:64" json:"hash"`
	Status       string     `gorm:"size:20;default:'completed'" json:"status"`
	ChunkCount   int        `gorm:"default:0" json:"chunk_count"`
	UploadedAt   *time.Time `json:"uploaded_at"`
	CreatedBy    string     `gorm:"type:uuid" json:"created_by"`
	CreatedAt    time.Time  `gorm:"not null" json:"created_at"`
}

func (f *FileUpload) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	return nil
}

// FileChunk 分片上传记录（大文件）
type FileChunk struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	UploadID    string    `gorm:"type:uuid;not null;index:idx_file_chunk_upload" json:"upload_id"`
	ChunkIndex  int       `gorm:"not null" json:"chunk_index"`
	ChunkSize   int64     `gorm:"not null" json:"chunk_size"`
	StoragePath string    `gorm:"size:1024;not null" json:"storage_path"`
	Hash        string    `gorm:"size:64" json:"hash"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
}

func (c *FileChunk) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	return nil
}
