package workspace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pmezard/go-difflib/difflib"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service 管理工作区数据
type Service struct {
	db          *gorm.DB
	naming      *AutoNamingPolicy
	initialized sync.Map
}

// 业务内通用错误
var (
	ErrFileVersionConflict = errors.New("文件版本已变化，请刷新后重试")
	ErrDiffVersionNotFound = errors.New("指定版本不存在")
)

const (
	StagingStatusDrafted                = "drafted"
	StagingStatusAwaitingReview         = "awaiting_review"
	StagingStatusAwaitingSecondary      = "awaiting_secondary_review"
	StagingStatusApprovedPendingArchive = "approved_pending_archive"
	StagingStatusArchived               = "archived"
	StagingStatusChangesRequested       = "changes_requested"
	StagingStatusRejected               = "rejected"
	StagingStatusFailed                 = "failed"
)

const (
	defaultStagingTTLHours = 72
)

const (
	StagingErrorTokenInvalid  = "STG_REVIEW_TOKEN_INVALID"
	StagingErrorConflict      = "STG_CONFLICT"
	StagingErrorPathCollision = "STG_PATH_COLLISION"
)

// StagingError 业务异常
type StagingError struct {
	Code    string
	Message string
}

func (e *StagingError) Error() string {
	return e.Message
}

func newStagingError(code, message string) *StagingError {
	return &StagingError{Code: code, Message: message}
}

type ReviewAction string

const (
	ReviewActionApprove       ReviewAction = "approve"
	ReviewActionReject        ReviewAction = "reject"
	ReviewActionRequestChange ReviewAction = "request_changes"
)

// TreeListOptions 控制树查询行为
type TreeListOptions struct {
	ParentID *string
	Depth    int // -1 表示不限制层级，0 表示仅返回根节点
}

// DiffVersionMeta 差异版本元信息
type DiffVersionMeta struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

// DiffResult 差异结果
type DiffResult struct {
	BaseVersion   DiffVersionMeta `json:"baseVersion"`
	TargetVersion DiffVersionMeta `json:"targetVersion"`
	Hunks         []DiffLine      `json:"hunks"`
}

// NewService 创建服务
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:     db,
		naming: NewAutoNamingPolicy(),
	}
}

// TreeNode 用于前端展示
type TreeNode struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	NodePath    string      `json:"nodePath"`
	Category    string      `json:"category"`
	Metadata    string      `json:"metadata"`
	HasChildren bool        `json:"hasChildren"`
	Children    []*TreeNode `json:"children"`
}

// FileDetail 文件详情
type FileDetail struct {
	Node    *WorkspaceNode        `json:"node"`
	File    *WorkspaceFile        `json:"file"`
	Version *WorkspaceFileVersion `json:"version"`
}

// CreateFolderRequest 新建目录请求
type CreateFolderRequest struct {
	TenantID string
	ParentID *string
	Name     string
	Category string
	UserID   string
}

// UpdateFileRequest 更新文件
type UpdateFileRequest struct {
	TenantID          string
	NodeID            string
	Content           string
	Summary           string
	AgentID           string
	ToolName          string
	Metadata          string
	UserID            string
	ExpectedVersionID string
}

// CreateFileRequest 创建文件
type CreateFileRequest struct {
	TenantID string
	ParentID *string
	Name     string
	Category string
	Content  string
	Summary  string
	AgentID  string
	ToolName string
	Metadata string
	UserID   string
}

// PatchFileRequest 调整文件节点
type PatchFileRequest struct {
	TenantID          string
	NodeID            string
	NewName           *string
	NewParentID       *string
	UserID            string
	ExpectedUpdatedAt *time.Time
}

// SearchFilesRequest 文件搜索条件
type SearchFilesRequest struct {
	TenantID string
	Query    string
	Limit    int
	Offset   int
}

// FileSearchResult 搜索结果
type FileSearchResult struct {
	NodeID   string    `json:"nodeId"`
	Name     string    `json:"name"`
	NodePath string    `json:"nodePath"`
	Snippet  string    `json:"snippet"`
	Updated  time.Time `json:"updatedAt"`
}

// FileHistoryItem 版本记录
type FileHistoryItem struct {
	VersionID string    `json:"versionId"`
	CreatedAt time.Time `json:"createdAt"`
	Summary   string    `json:"summary"`
	CreatedBy string    `json:"createdBy"`
	AgentID   string    `json:"agentId"`
	ToolName  string    `json:"toolName"`
}

// DiffLine 差异行
type DiffLine struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CreateStagingRequest 创建暂存文件
type CreateStagingRequest struct {
	TenantID          string
	FileType          string
	Content           string
	TitleHint         string
	Summary           string
	AgentID           string
	AgentName         string
	Command           string
	Metadata          string
	CreatedBy         string
	ManualFolder      string
	RequiresSecondary bool
}

// ListStagingRequest 查询暂存
type ListStagingRequest struct {
	TenantID string
	Status   string
}

// ReviewStagingRequest 审核请求
type ReviewStagingRequest struct {
	TenantID    string
	StagingID   string
	ReviewerID  string
	Action      ReviewAction
	Reason      string
	ReviewToken string
}

// ContextLinkRequest 命令上下文请求
type ContextLinkRequest struct {
	TenantID  string
	AgentID   string
	SessionID string
	NodeIDs   []string
	Mentions  []string
	Commands  []string
	Notes     string
	UserID    string
}

// ContextNode 提供上下文信息
type ContextNode struct {
	Node    *WorkspaceNode
	File    *WorkspaceFile
	Version *WorkspaceFileVersion
}

// EnsureDefaults 如果租户尚未初始化则创建基础目录
func (s *Service) EnsureDefaults(ctx context.Context, tenantID, userID string) error {
	if tenantID == "" {
		return errors.New("tenantID 不能为空")
	}
	if _, ok := s.initialized.Load(tenantID); ok {
		return nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&WorkspaceNode{}).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for idx, tpl := range defaultFolders {
				node := &WorkspaceNode{
					ID:        uuid.New().String(),
					TenantID:  tenantID,
					Name:      tpl.Name,
					Slug:      tpl.Slug,
					Type:      "folder",
					NodePath:  tpl.Slug,
					Category:  tpl.Category,
					SortOrder: idx,
					CreatedBy: userID,
					UpdatedBy: userID,
				}
				if err := tx.Create(node).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	s.initialized.Store(tenantID, true)
	return nil
}

// ListTree 返回整棵树（兼容旧接口）
func (s *Service) ListTree(ctx context.Context, tenantID string) ([]*TreeNode, error) {
	return s.ListTreeWithOptions(ctx, tenantID, TreeListOptions{Depth: -1})
}

// ListTreeWithOptions 支持按层级、游标加载的树接口
func (s *Service) ListTreeWithOptions(ctx context.Context, tenantID string, opts TreeListOptions) ([]*TreeNode, error) {
	depth := opts.Depth
	if depth == 0 {
		depth = 2
	}
	var nodes []WorkspaceNode
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("sort_order ASC, node_path ASC").
		Find(&nodes).Error; err != nil {
		return nil, err
	}
	byParent := make(map[string][]*TreeNode)
	index := make(map[string]*TreeNode)
	convert := func(n WorkspaceNode) *TreeNode {
		tn := &TreeNode{
			ID:       n.ID,
			Name:     n.Name,
			Type:     n.Type,
			NodePath: n.NodePath,
			Category: n.Category,
			Metadata: n.Metadata,
		}
		return tn
	}
	for _, n := range nodes {
		tn := convert(n)
		index[n.ID] = tn
		parent := "root"
		if n.ParentID != nil {
			parent = *n.ParentID
		}
		byParent[parent] = append(byParent[parent], tn)
	}
	for _, children := range byParent {
		sort.SliceStable(children, func(i, j int) bool {
			return strings.Compare(children[i].Name, children[j].Name) < 0
		})
	}
	nextDepth := func(current int) int {
		if current < 0 {
			return -1
		}
		if current <= 0 {
			return 0
		}
		return current - 1
	}
	var attachChildren func(node *TreeNode, remaining int)
	attachChildren = func(node *TreeNode, remaining int) {
		children := byParent[node.ID]
		if len(children) == 0 {
			node.HasChildren = false
			return
		}
		node.HasChildren = true
		if remaining == 0 {
			return
		}
		node.Children = children
		for _, child := range children {
			attachChildren(child, nextDepth(remaining))
		}
	}
	if opts.ParentID != nil {
		if parent, ok := index[*opts.ParentID]; ok {
			attachDepth := depth
			if attachDepth < 0 {
				attachChildren(parent, -1)
			} else if attachDepth > 0 {
				attachChildren(parent, attachDepth)
			} else {
				parent.HasChildren = len(byParent[parent.ID]) > 0
			}
			return []*TreeNode{parent}, nil
		}
		return []*TreeNode{}, nil
	}
	roots := byParent["root"]
	unlimited := depth < 0
	childDepth := depth
	if !unlimited {
		if childDepth > 0 {
			childDepth = depth - 1
		}
	}
	for _, node := range roots {
		if unlimited {
			attachChildren(node, -1)
			continue
		}
		if depth <= 0 {
			node.HasChildren = len(byParent[node.ID]) > 0
			continue
		}
		attachChildren(node, childDepth)
	}
	return roots, nil
}

// CreateFolder 新建文件夹
func (s *Service) CreateFolder(ctx context.Context, req *CreateFolderRequest) (*WorkspaceNode, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("文件夹名称不能为空")
	}
	parentPath := ""
	if req.ParentID != nil {
		parent, err := s.GetNode(ctx, req.TenantID, *req.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil || parent.Type != "folder" {
			return nil, errors.New("父级不存在或不是文件夹")
		}
		parentPath = parent.NodePath
	}
	slug := slugify(req.Name)
	path := slugify(req.Name)
	if parentPath != "" {
		path = fmt.Sprintf("%s/%s", parentPath, slug)
	}
	node := &WorkspaceNode{
		TenantID:  req.TenantID,
		ParentID:  req.ParentID,
		Name:      req.Name,
		Slug:      slug,
		Type:      "folder",
		NodePath:  path,
		Category:  req.Category,
		CreatedBy: req.UserID,
		UpdatedBy: req.UserID,
	}
	if err := s.db.WithContext(ctx).Create(node).Error; err != nil {
		return nil, err
	}
	return node, nil
}

// GetNode 查询节点
func (s *Service) GetNode(ctx context.Context, tenantID, nodeID string) (*WorkspaceNode, error) {
	var node WorkspaceNode
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", nodeID, tenantID).
		First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &node, nil
}

// UpdateNodeName 重命名
func (s *Service) UpdateNodeName(ctx context.Context, tenantID, nodeID, name, userID string) (*WorkspaceNode, error) {
	node, err := s.GetNode(ctx, tenantID, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("节点不存在")
	}
	oldPath := node.NodePath
	node.Name = name
	node.Slug = slugify(name)
	if node.ParentID != nil {
		parent, err := s.GetNode(ctx, tenantID, *node.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return nil, errors.New("父级不存在")
		}
		node.NodePath = fmt.Sprintf("%s/%s", parent.NodePath, node.Slug)
	} else {
		node.NodePath = node.Slug
	}
	node.UpdatedBy = userID
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(node).Error; err != nil {
			return err
		}
		if node.Type == "folder" {
			return tx.Model(&WorkspaceNode{}).
				Where("tenant_id = ? AND node_path LIKE ?", tenantID, oldPath+"/%").
				Update("node_path", gorm.Expr("regexp_replace(node_path, ?, ?)", "^"+oldPath, node.NodePath)).Error
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return node, nil
}

// DeleteNode 软删除
func (s *Service) DeleteNode(ctx context.Context, tenantID, nodeID string) error {
	return s.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, nodeID).
		Delete(&WorkspaceNode{}).Error
}

// GetFileDetail 拉取文件内容
func (s *Service) GetFileDetail(ctx context.Context, tenantID, nodeID string) (*FileDetail, error) {
	node, err := s.GetNode(ctx, tenantID, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("节点不存在")
	}
	var file WorkspaceFile
	if err := s.db.WithContext(ctx).
		Where("node_id = ? AND tenant_id = ?", node.ID, tenantID).
		First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &FileDetail{Node: node}, nil
		}
		return nil, err
	}
	var version WorkspaceFileVersion
	if file.LatestVersionID != "" {
		if err := s.db.WithContext(ctx).
			Where("id = ?", file.LatestVersionID).
			First(&version).Error; err != nil {
			return nil, err
		}
	}
	return &FileDetail{Node: node, File: &file, Version: &version}, nil
}

// UpdateFileContent 写入新版本
func (s *Service) UpdateFileContent(ctx context.Context, req *UpdateFileRequest) (*FileDetail, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("文件内容不能为空")
	}
	returnDetail := &FileDetail{}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		node, err := s.GetNode(ctx, req.TenantID, req.NodeID)
		if err != nil {
			return err
		}
		if node == nil {
			return errors.New("节点不存在")
		}
		if node.Type != "file" {
			return errors.New("该节点不是文件")
		}
		var file WorkspaceFile
		createdNewFile := false
		if err := tx.Where("node_id = ?", node.ID).First(&file).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			file = WorkspaceFile{
				TenantID:  req.TenantID,
				NodeID:    node.ID,
				Category:  node.Category,
				CreatedBy: req.UserID,
				UpdatedBy: req.UserID,
			}
			if err := tx.Create(&file).Error; err != nil {
				return err
			}
			createdNewFile = true
		}
		expected := strings.TrimSpace(req.ExpectedVersionID)
		if !createdNewFile && expected != "" && strings.TrimSpace(file.LatestVersionID) != expected {
			return ErrFileVersionConflict
		}
		version := WorkspaceFileVersion{
			FileID:    file.ID,
			TenantID:  req.TenantID,
			Content:   req.Content,
			Summary:   req.Summary,
			AgentID:   req.AgentID,
			ToolName:  req.ToolName,
			Metadata:  req.Metadata,
			CreatedBy: req.UserID,
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		if err := tx.Model(&file).Updates(map[string]any{
			"latest_version_id": version.ID,
			"updated_by":        req.UserID,
		}).Error; err != nil {
			return err
		}
		returnDetail.Node = node
		returnDetail.File = &file
		returnDetail.Version = &version
		return nil
	}); err != nil {
		return nil, err
	}
	return returnDetail, nil
}

// CreateFile 新建文件并可选写入初始版本
func (s *Service) CreateFile(ctx context.Context, req *CreateFileRequest) (*FileDetail, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("文件名不能为空")
	}
	result := &FileDetail{}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent *WorkspaceNode
		if req.ParentID != nil {
			p, err := s.GetNode(ctx, req.TenantID, *req.ParentID)
			if err != nil {
				return err
			}
			if p == nil || p.Type != "folder" {
				return errors.New("父级不存在或不是目录")
			}
			parent = p
		}
		slug := slugify(req.Name)
		path := slug
		if parent != nil {
			path = fmt.Sprintf("%s/%s", parent.NodePath, slug)
		}
		node := &WorkspaceNode{
			TenantID:  req.TenantID,
			ParentID:  nil,
			Name:      req.Name,
			Slug:      slug,
			Type:      "file",
			NodePath:  path,
			Category:  req.Category,
			CreatedBy: req.UserID,
			UpdatedBy: req.UserID,
		}
		if parent != nil {
			node.ParentID = &parent.ID
			node.Category = parent.Category
		}
		if err := tx.Create(node).Error; err != nil {
			return err
		}
		file := &WorkspaceFile{
			TenantID:  req.TenantID,
			NodeID:    node.ID,
			Category:  node.Category,
			CreatedBy: req.UserID,
			UpdatedBy: req.UserID,
		}
		if err := tx.Create(file).Error; err != nil {
			return err
		}
		var version *WorkspaceFileVersion
		if strings.TrimSpace(req.Content) != "" {
			ver := &WorkspaceFileVersion{
				FileID:    file.ID,
				TenantID:  req.TenantID,
				Content:   req.Content,
				Summary:   req.Summary,
				AgentID:   req.AgentID,
				ToolName:  req.ToolName,
				Metadata:  req.Metadata,
				CreatedBy: req.UserID,
			}
			if err := tx.Create(ver).Error; err != nil {
				return err
			}
			version = ver
			if err := tx.Model(file).Update("latest_version_id", ver.ID).Error; err != nil {
				return err
			}
		}
		result.Node = node
		result.File = file
		result.Version = version
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// PatchFile 调整文件节点元数据
func (s *Service) PatchFile(ctx context.Context, req *PatchFileRequest) (*WorkspaceNode, error) {
	var updated *WorkspaceNode
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		node, err := s.GetNode(ctx, req.TenantID, req.NodeID)
		if err != nil {
			return err
		}
		if node == nil || node.Type != "file" {
			return errors.New("文件不存在")
		}
		if req.ExpectedUpdatedAt != nil {
			expected := req.ExpectedUpdatedAt.UTC()
			if !node.UpdatedAt.UTC().Equal(expected) {
				return errors.New("版本已更新，请刷新后重试")
			}
		}
		var parent *WorkspaceNode
		parentID := node.ParentID
		if req.NewParentID != nil {
			if *req.NewParentID == "" {
				parentID = nil
			} else {
				p, err := s.GetNode(ctx, req.TenantID, *req.NewParentID)
				if err != nil {
					return err
				}
				if p == nil || p.Type != "folder" {
					return errors.New("目标父级无效")
				}
				parent = p
				parentID = &p.ID
			}
		} else if node.ParentID != nil {
			p, err := s.GetNode(ctx, req.TenantID, *node.ParentID)
			if err != nil {
				return err
			}
			parent = p
		}
		if req.NewParentID != nil && parent == nil && *req.NewParentID != "" {
			return errors.New("目标父级不存在")
		}
		if req.NewName != nil {
			node.Name = strings.TrimSpace(*req.NewName)
			if node.Name == "" {
				return errors.New("新名称不能为空")
			}
			node.Slug = slugify(node.Name)
		}
		node.ParentID = parentID
		if parent != nil {
			node.Category = parent.Category
		}
		base := ""
		if parent != nil {
			base = parent.NodePath
		}
		if base == "" {
			node.NodePath = node.Slug
		} else {
			node.NodePath = fmt.Sprintf("%s/%s", base, node.Slug)
		}
		node.UpdatedBy = req.UserID
		if err := tx.Save(node).Error; err != nil {
			return err
		}
		updated = node
		return nil
	}); err != nil {
		return nil, err
	}
	return updated, nil
}

// CreateStagingFile 写入暂存
func (s *Service) CreateStagingFile(ctx context.Context, req *CreateStagingRequest) (*WorkspaceStagingFile, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("内容不能为空")
	}
	tpl := s.naming.ResolveFolder(req.FileType)
	folder := tpl.Slug
	if req.ManualFolder != "" {
		if strings.Contains(req.ManualFolder, "..") {
			return nil, errors.New("目录不可包含 ..")
		}
		folder = slugify(req.ManualFolder)
	}
	name := s.naming.SuggestName(req.FileType, req.TitleHint, req.Content)
	path := fmt.Sprintf("%s/%s", folder, slugify(name))
	reviewToken := generateReviewToken()
	sla := s.computeStagingSLA(defaultStagingTTLHours)
	now := time.Now().UTC()
	auditTrail := marshalAuditEntries([]map[string]any{
		{"action": StagingStatusAwaitingReview, "actor": req.CreatedBy, "timestamp": now},
	})
	staging := &WorkspaceStagingFile{
		TenantID:             req.TenantID,
		FileType:             strings.ToLower(req.FileType),
		SuggestedName:        name,
		SuggestedFolder:      folder,
		SuggestedPath:        path,
		Content:              req.Content,
		Summary:              req.Summary,
		SourceAgentID:        req.AgentID,
		SourceAgentName:      req.AgentName,
		SourceCommand:        req.Command,
		Metadata:             req.Metadata,
		CreatedBy:            req.CreatedBy,
		UpdatedBy:            req.CreatedBy,
		Status:               StagingStatusAwaitingReview,
		ReviewToken:          reviewToken,
		RequiresSecondary:    req.RequiresSecondary,
		SLAExpiresAt:         sla,
		LastStatusTransition: now,
		AuditTrail:           auditTrail,
	}
	if err := s.db.WithContext(ctx).Create(staging).Error; err != nil {
		return nil, err
	}
	return staging, nil
}

// ListStagingFiles 查询暂存
func (s *Service) ListStagingFiles(ctx context.Context, req *ListStagingRequest) ([]WorkspaceStagingFile, error) {
	var list []WorkspaceStagingFile
	query := s.db.WithContext(ctx).
		Where("tenant_id = ?", req.TenantID).
		Order("COALESCE(sla_expires_at, created_at) ASC, created_at DESC")
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ReviewStagingFile 审核暂存条目
func (s *Service) ReviewStagingFile(ctx context.Context, req *ReviewStagingRequest) (*WorkspaceStagingFile, error) {
	if req == nil {
		return nil, errors.New("无效的请求")
	}
	if strings.TrimSpace(req.ReviewToken) == "" {
		return nil, errors.New("reviewToken 不能为空")
	}
	var result WorkspaceStagingFile
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var staging WorkspaceStagingFile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", req.StagingID, req.TenantID).
			First(&staging).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("暂存记录不存在")
			}
			return err
		}
		expectedToken := staging.ReviewToken
		if staging.Status == StagingStatusAwaitingSecondary {
			expectedToken = staging.SecondaryReviewToken
		}
		if strings.TrimSpace(expectedToken) == "" || expectedToken != strings.TrimSpace(req.ReviewToken) {
			return newStagingError(StagingErrorTokenInvalid, "评审令牌无效或已过期")
		}
		switch req.Action {
		case ReviewActionApprove:
			if err := s.handleStagingApprove(ctx, tx, &staging, req); err != nil {
				return err
			}
		case ReviewActionReject:
			if err := s.handleStagingReject(tx, &staging, req); err != nil {
				return err
			}
		case ReviewActionRequestChange:
			if err := s.handleStagingRequestChanges(tx, &staging, req); err != nil {
				return err
			}
		default:
			return errors.New("不支持的操作: " + string(req.Action))
		}
		result = staging
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) handleStagingApprove(ctx context.Context, tx *gorm.DB, staging *WorkspaceStagingFile, req *ReviewStagingRequest) error {
	now := time.Now().UTC()
	switch staging.Status {
	case StagingStatusAwaitingReview, StagingStatusDrafted:
		if staging.ReviewerID == nil {
			reviewerID := req.ReviewerID
			staging.ReviewerID = &reviewerID
		}
		if staging.RequiresSecondary {
			staging.Status = StagingStatusAwaitingSecondary
			staging.ReviewToken = ""
			staging.SecondaryReviewToken = generateReviewToken()
			staging.UpdatedBy = req.ReviewerID
			staging.LastStatusTransition = now
			appendAuditEntry(staging, map[string]any{
				"action":    StagingStatusAwaitingSecondary,
				"actor":     req.ReviewerID,
				"timestamp": now,
			})
			return tx.Save(staging).Error
		}
		fallthrough
	case StagingStatusAwaitingSecondary:
		if staging.Status == StagingStatusAwaitingSecondary {
			secondaryID := req.ReviewerID
			staging.SecondaryReviewerID = &secondaryID
		}
		_, _, err := s.finalizeStaging(ctx, tx, staging, req.ReviewerID)
		return err
	default:
		return newStagingError(StagingErrorConflict, "当前状态不允许通过审核")
	}
}

func (s *Service) handleStagingReject(tx *gorm.DB, staging *WorkspaceStagingFile, req *ReviewStagingRequest) error {
	now := time.Now().UTC()
	staging.Status = StagingStatusRejected
	staging.ReviewToken = ""
	staging.SecondaryReviewToken = ""
	staging.UpdatedBy = req.ReviewerID
	staging.LastStatusTransition = now
	staging.ReviewedAt = &now
	reviewerID := req.ReviewerID
	if staging.ReviewerID == nil {
		staging.ReviewerID = &reviewerID
	}
	meta := map[string]any{}
	if strings.TrimSpace(staging.Metadata) != "" {
		_ = json.Unmarshal([]byte(staging.Metadata), &meta)
	}
	if req.Reason != "" {
		meta["review_note"] = req.Reason
	}
	if len(meta) > 0 {
		payload, _ := json.Marshal(meta)
		staging.Metadata = string(payload)
	}
	appendAuditEntry(staging, map[string]any{
		"action":    StagingStatusRejected,
		"actor":     req.ReviewerID,
		"reason":    req.Reason,
		"timestamp": now,
	})
	return tx.Save(staging).Error
}

func (s *Service) handleStagingRequestChanges(tx *gorm.DB, staging *WorkspaceStagingFile, req *ReviewStagingRequest) error {
	now := time.Now().UTC()
	staging.Status = StagingStatusChangesRequested
	staging.ReviewToken = generateReviewToken()
	staging.SecondaryReviewToken = ""
	staging.ResubmitCount++
	staging.UpdatedBy = req.ReviewerID
	staging.LastStatusTransition = now
	reviewerID := req.ReviewerID
	if staging.ReviewerID == nil {
		staging.ReviewerID = &reviewerID
	}
	meta := map[string]any{}
	if strings.TrimSpace(staging.Metadata) != "" {
		_ = json.Unmarshal([]byte(staging.Metadata), &meta)
	}
	if req.Reason != "" {
		meta["change_request"] = req.Reason
	}
	if len(meta) > 0 {
		payload, _ := json.Marshal(meta)
		staging.Metadata = string(payload)
	}
	appendAuditEntry(staging, map[string]any{
		"action":    StagingStatusChangesRequested,
		"actor":     req.ReviewerID,
		"reason":    req.Reason,
		"timestamp": now,
	})
	return tx.Save(staging).Error
}

// SearchFiles 简单搜索
func (s *Service) SearchFiles(ctx context.Context, req *SearchFilesRequest) ([]FileSearchResult, int64, error) {
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	rows := make([]struct {
		NodeID         string
		Name           string
		NodePath       string
		Summary        string
		Content        string
		VersionUpdated sql.NullTime
		NodeUpdated    time.Time
	}, 0)
	baseQuery := s.db.WithContext(ctx).
		Table("workspace_nodes AS n").
		Select("n.id AS node_id, n.name, n.node_path, COALESCE(v.summary,'') AS summary, COALESCE(v.content,'') AS content, v.created_at AS version_updated, n.updated_at AS node_updated").
		Joins("LEFT JOIN workspace_files f ON f.node_id = n.id").
		Joins("LEFT JOIN workspace_file_versions v ON v.id = f.latest_version_id").
		Where("n.tenant_id = ? AND n.type = 'file'", req.TenantID)
	if strings.TrimSpace(req.Query) != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(req.Query)) + "%"
		baseQuery = baseQuery.Where("(LOWER(n.name) LIKE ? OR LOWER(n.node_path) LIKE ? OR LOWER(v.summary) LIKE ? OR LOWER(v.content) LIKE ?)", pattern, pattern, pattern, pattern)
	}
	countQuery := baseQuery.Session(&gorm.Session{})
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := baseQuery.Order("COALESCE(v.created_at, n.updated_at) DESC").Limit(limit).Offset(offset).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	results := make([]FileSearchResult, 0, len(rows))
	for _, row := range rows {
		snippet := row.Summary
		if snippet == "" {
			snippet = trimSnippet(row.Content)
		} else {
			snippet = trimSnippet(snippet)
		}
		updated := row.NodeUpdated
		if row.VersionUpdated.Valid {
			updated = row.VersionUpdated.Time
		}
		results = append(results, FileSearchResult{
			NodeID:   row.NodeID,
			Name:     row.Name,
			NodePath: row.NodePath,
			Snippet:  snippet,
			Updated:  updated,
		})
	}
	return results, total, nil
}

// GetFileHistory 返回版本记录
func (s *Service) GetFileHistory(ctx context.Context, tenantID, nodeID string, limit int) ([]FileHistoryItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	file, err := s.loadFileByNode(ctx, tenantID, nodeID)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, errors.New("文件不存在")
	}
	var versions []WorkspaceFileVersion
	if err := s.db.WithContext(ctx).
		Where("file_id = ?", file.ID).
		Order("created_at DESC").
		Limit(limit).
		Find(&versions).Error; err != nil {
		return nil, err
	}
	items := make([]FileHistoryItem, 0, len(versions))
	for _, v := range versions {
		items = append(items, FileHistoryItem{
			VersionID: v.ID,
			CreatedAt: v.CreatedAt,
			Summary:   v.Summary,
			CreatedBy: v.CreatedBy,
			AgentID:   v.AgentID,
			ToolName:  v.ToolName,
		})
	}
	return items, nil
}

// RevertFile 创建回滚版本
func (s *Service) RevertFile(ctx context.Context, tenantID, nodeID, versionID, userID string) (*FileDetail, error) {
	file, err := s.loadFileByNode(ctx, tenantID, nodeID)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, errors.New("文件不存在")
	}
	var target WorkspaceFileVersion
	if err := s.db.WithContext(ctx).
		Where("id = ? AND file_id = ?", versionID, file.ID).
		First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("版本不存在")
		}
		return nil, err
	}
	returnDetail := &FileDetail{}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		newVersion := &WorkspaceFileVersion{
			FileID:    file.ID,
			TenantID:  tenantID,
			Content:   target.Content,
			Summary:   target.Summary,
			AgentID:   target.AgentID,
			ToolName:  target.ToolName,
			Metadata:  target.Metadata,
			CreatedBy: userID,
		}
		if err := tx.Create(newVersion).Error; err != nil {
			return err
		}
		if err := tx.Model(file).Updates(map[string]any{
			"latest_version_id": newVersion.ID,
			"updated_by":        userID,
		}).Error; err != nil {
			return err
		}
		returnDetail.File = file
		returnDetail.Version = newVersion
		node, err := s.GetNode(ctx, tenantID, nodeID)
		if err != nil {
			return err
		}
		returnDetail.Node = node
		return nil
	}); err != nil {
		return nil, err
	}
	return returnDetail, nil
}

// DiffFileVersions 返回差异
func (s *Service) DiffFileVersions(ctx context.Context, tenantID, versionIDA, versionIDB string) (*DiffResult, error) {
	var left, right WorkspaceFileVersion
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", versionIDA, tenantID).
		First(&left).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDiffVersionNotFound
		}
		return nil, err
	}
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", versionIDB, tenantID).
		First(&right).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDiffVersionNotFound
		}
		return nil, err
	}
	aLines := difflib.SplitLines(left.Content)
	bLines := difflib.SplitLines(right.Content)
	matcher := difflib.NewMatcher(aLines, bLines)
	var diff []DiffLine
	for _, op := range matcher.GetOpCodes() {
		switch op.Tag {
		case 'e':
			for _, line := range aLines[op.I1:op.I2] {
				diff = append(diff, DiffLine{Type: "equal", Text: line})
			}
		case 'd':
			for _, line := range aLines[op.I1:op.I2] {
				diff = append(diff, DiffLine{Type: "delete", Text: line})
			}
		case 'i':
			for _, line := range bLines[op.J1:op.J2] {
				diff = append(diff, DiffLine{Type: "insert", Text: line})
			}
		case 'r':
			for _, line := range aLines[op.I1:op.I2] {
				diff = append(diff, DiffLine{Type: "delete", Text: line})
			}
			for _, line := range bLines[op.J1:op.J2] {
				diff = append(diff, DiffLine{Type: "insert", Text: line})
			}
		}
	}
	result := &DiffResult{
		BaseVersion: DiffVersionMeta{
			ID:        left.ID,
			Summary:   left.Summary,
			CreatedBy: left.CreatedBy,
			CreatedAt: left.CreatedAt,
		},
		TargetVersion: DiffVersionMeta{
			ID:        right.ID,
			Summary:   right.Summary,
			CreatedBy: right.CreatedBy,
			CreatedAt: right.CreatedAt,
		},
		Hunks: diff,
	}
	return result, nil
}

func (s *Service) finalizeStaging(ctx context.Context, tx *gorm.DB, staging *WorkspaceStagingFile, reviewerID string) (*WorkspaceFile, *WorkspaceFileVersion, error) {
	now := time.Now().UTC()
	staging.Status = StagingStatusApprovedPendingArchive
	staging.LastStatusTransition = now
	staging.UpdatedBy = reviewerID
	appendAuditEntry(staging, map[string]any{
		"action":    StagingStatusApprovedPendingArchive,
		"actor":     reviewerID,
		"timestamp": now,
	})
	if err := tx.Save(staging).Error; err != nil {
		return nil, nil, err
	}
	parent, err := s.ensureFolderPath(ctx, tx, staging.TenantID, staging.SuggestedFolder, reviewerID)
	if err != nil {
		s.markStagingFailure(tx, staging, reviewerID, err)
		return nil, nil, err
	}
	fileNode, err := s.ensureFileNode(ctx, tx, staging.TenantID, parent, staging.SuggestedName, reviewerID)
	if err != nil {
		s.markStagingFailure(tx, staging, reviewerID, err)
		return nil, nil, err
	}
	var file WorkspaceFile
	if err := tx.Where("node_id = ?", fileNode.ID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			file = WorkspaceFile{
				TenantID:  staging.TenantID,
				NodeID:    fileNode.ID,
				Category:  fileNode.Category,
				CreatedBy: reviewerID,
				UpdatedBy: reviewerID,
			}
			if err := tx.Create(&file).Error; err != nil {
				s.markStagingFailure(tx, staging, reviewerID, err)
				return nil, nil, err
			}
		} else {
			s.markStagingFailure(tx, staging, reviewerID, err)
			return nil, nil, err
		}
	}
	version := WorkspaceFileVersion{
		FileID:    file.ID,
		TenantID:  staging.TenantID,
		Content:   staging.Content,
		Summary:   staging.Summary,
		AgentID:   staging.SourceAgentID,
		ToolName:  staging.SourceCommand,
		Metadata:  staging.Metadata,
		CreatedBy: reviewerID,
	}
	if err := tx.Create(&version).Error; err != nil {
		s.markStagingFailure(tx, staging, reviewerID, err)
		return nil, nil, err
	}
	approvedAt := time.Now().UTC()
	updates := map[string]any{
		"latest_version_id": version.ID,
		"review_status":     "published",
		"approver_id":       reviewerID,
		"approved_at":       &approvedAt,
		"updated_by":        reviewerID,
	}
	if err := tx.Model(&file).Updates(updates).Error; err != nil {
		s.markStagingFailure(tx, staging, reviewerID, err)
		return nil, nil, err
	}
	staging.Status = StagingStatusArchived
	staging.SuggestedPath = fileNode.NodePath
	staging.ReviewToken = ""
	staging.SecondaryReviewToken = ""
	staging.ReviewedAt = &approvedAt
	staging.LastStatusTransition = approvedAt
	staging.UpdatedBy = reviewerID
	appendAuditEntry(staging, map[string]any{
		"action":    StagingStatusArchived,
		"actor":     reviewerID,
		"timestamp": approvedAt,
	})
	if staging.ReviewerID == nil {
		reviewer := reviewerID
		staging.ReviewerID = &reviewer
	}
	if err := tx.Save(staging).Error; err != nil {
		return nil, nil, err
	}
	return &file, &version, nil
}

func (s *Service) markStagingFailure(tx *gorm.DB, staging *WorkspaceStagingFile, reviewerID string, sourceErr error) {
	now := time.Now().UTC()
	staging.Status = StagingStatusFailed
	staging.LastStatusTransition = now
	staging.UpdatedBy = reviewerID
	appendAuditEntry(staging, map[string]any{
		"action":    StagingStatusFailed,
		"actor":     reviewerID,
		"error":     sourceErr.Error(),
		"timestamp": now,
	})
	_ = tx.Save(staging).Error
}

// PublishStagingFile 将暂存转为正式文件
func (s *Service) PublishStagingFile(ctx context.Context, tenantID, stagingID, reviewerID string) (*WorkspaceFile, *WorkspaceFileVersion, error) {
	var file *WorkspaceFile
	var version *WorkspaceFileVersion
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var staging WorkspaceStagingFile
		if err := tx.Where("id = ? AND tenant_id = ?", stagingID, tenantID).First(&staging).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("暂存记录不存在")
			}
			return err
		}
		switch staging.Status {
		case StagingStatusAwaitingReview, StagingStatusDrafted:
			reviewer := reviewerID
			staging.ReviewerID = &reviewer
		case StagingStatusAwaitingSecondary:
			secondary := reviewerID
			staging.SecondaryReviewerID = &secondary
		default:
			return newStagingError(StagingErrorConflict, "当前状态不允许直接发布")
		}
		f, v, err := s.finalizeStaging(ctx, tx, &staging, reviewerID)
		if err != nil {
			return err
		}
		file = f
		version = v
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return file, version, nil
}

// RejectStagingFile 拒绝暂存
func (s *Service) RejectStagingFile(ctx context.Context, tenantID, stagingID, reviewerID, reason string) error {
	metaPatch := map[string]string{"review_note": reason}
	payload, _ := json.Marshal(metaPatch)
	return s.db.WithContext(ctx).
		Model(&WorkspaceStagingFile{}).
		Where("id = ? AND tenant_id = ?", stagingID, tenantID).
		Updates(map[string]any{
			"status":      "rejected",
			"reviewer_id": reviewerID,
			"reviewed_at": time.Now().UTC(),
			"updated_by":  reviewerID,
			"metadata":    string(payload),
		}).Error
}

// CreateContextLink 记录命令上下文
func (s *Service) CreateContextLink(ctx context.Context, req *ContextLinkRequest, snapshot string) (*WorkspaceContextLink, error) {
	link := &WorkspaceContextLink{
		TenantID:  req.TenantID,
		AgentID:   req.AgentID,
		SessionID: req.SessionID,
		Mentions:  req.Mentions,
		Commands:  req.Commands,
		NodeIDs:   req.NodeIDs,
		Notes:     req.Notes,
		Snapshot:  snapshot,
		CreatedBy: req.UserID,
	}
	if err := s.db.WithContext(ctx).Create(link).Error; err != nil {
		return nil, err
	}
	return link, nil
}

// LoadContextNodes 获取节点和最新版本
func (s *Service) LoadContextNodes(ctx context.Context, tenantID string, nodeIDs []string) ([]*ContextNode, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	var nodes []WorkspaceNode
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ?", tenantID, nodeIDs).
		Find(&nodes).Error; err != nil {
		return nil, err
	}
	result := make([]*ContextNode, 0, len(nodes))
	for _, node := range nodes {
		cn := &ContextNode{Node: &node}
		var file WorkspaceFile
		if err := s.db.WithContext(ctx).
			Where("node_id = ?", node.ID).
			First(&file).Error; err == nil {
			cn.File = &file
			if file.LatestVersionID != "" {
				var version WorkspaceFileVersion
				if err := s.db.WithContext(ctx).
					Where("id = ?", file.LatestVersionID).
					First(&version).Error; err == nil {
					cn.Version = &version
				}
			}
		}
		result = append(result, cn)
	}
	return result, nil
}

func (s *Service) loadFileByNode(ctx context.Context, tenantID, nodeID string) (*WorkspaceFile, error) {
	var file WorkspaceFile
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND node_id = ?", tenantID, nodeID).
		First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &file, nil
}

func (s *Service) ensureFolderPath(ctx context.Context, tx *gorm.DB, tenantID, slugPath, userID string) (*WorkspaceNode, error) {
	parts := strings.Split(slugPath, "/")
	var parent *WorkspaceNode
	var parentID *string
	currentPath := ""
	for _, part := range parts {
		part = slugify(part)
		if part == "" {
			continue
		}
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = fmt.Sprintf("%s/%s", currentPath, part)
		}
		var node WorkspaceNode
		if err := tx.Where("tenant_id = ? AND node_path = ?", tenantID, currentPath).
			First(&node).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				node = WorkspaceNode{
					TenantID:  tenantID,
					ParentID:  parentID,
					Name:      part,
					Slug:      part,
					Type:      "folder",
					NodePath:  currentPath,
					CreatedBy: userID,
					UpdatedBy: userID,
				}
				if err := tx.Create(&node).Error; err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		parent = &node
		parentID = &node.ID
	}
	return parent, nil
}

func (s *Service) ensureFileNode(ctx context.Context, tx *gorm.DB, tenantID string, parent *WorkspaceNode, name, userID string) (*WorkspaceNode, error) {
	const maxAttempts = 10
	baseSlug := slugify(name)
	parentPath := ""
	var parentID *string
	var category string
	if parent != nil {
		parentPath = parent.NodePath
		parentID = &parent.ID
		category = parent.Category
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		suffix := ""
		nameSuffix := name
		if attempt > 0 {
			suffix = fmt.Sprintf("-%d", attempt)
			nameSuffix = fmt.Sprintf("%s-%d", name, attempt)
		}
		candidateSlug := baseSlug + suffix
		path := candidateSlug
		if parentPath != "" {
			path = fmt.Sprintf("%s/%s", parentPath, candidateSlug)
		}
		var node WorkspaceNode
		err := tx.Where("tenant_id = ? AND node_path = ?", tenantID, path).First(&node).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			node = WorkspaceNode{
				TenantID:  tenantID,
				ParentID:  parentID,
				Name:      nameSuffix,
				Slug:      candidateSlug,
				Type:      "file",
				NodePath:  path,
				Category:  category,
				CreatedBy: userID,
				UpdatedBy: userID,
			}
			if err := tx.Create(&node).Error; err != nil {
				return nil, err
			}
			return &node, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return nil, newStagingError(StagingErrorPathCollision, "命名冲突无法自动解决，请调整建议名称或目录")
}

func slugify(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = nonWord.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return uuid.New().String()
	}
	return name
}

func trimSnippet(text string) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= 160 {
		return text
	}
	runes := []rune(text)
	return string(runes[:160]) + "..."
}

func generateReviewToken() string {
	return uuid.New().String()
}

func marshalAuditEntries(entries []map[string]any) datatypes.JSON {
	data, err := json.Marshal(entries)
	if err != nil {
		return nil
	}
	return datatypes.JSON(data)
}

func appendAuditEntry(staging *WorkspaceStagingFile, entry map[string]any) {
	var logs []map[string]any
	if len(staging.AuditTrail) > 0 {
		_ = json.Unmarshal(staging.AuditTrail, &logs)
	}
	logs = append(logs, entry)
	staging.AuditTrail = marshalAuditEntries(logs)
}

func (s *Service) computeStagingSLA(hours int) *time.Time {
	if hours <= 0 {
		hours = defaultStagingTTLHours
	}
	deadline := time.Now().Add(time.Duration(hours) * time.Hour)
	return &deadline
}

// ============================================
// 智能体工作空间管理
// ============================================

// CreateArtifactRequest 创建产出物请求
type CreateArtifactRequest struct {
	TenantID    string
	AgentID     string
	AgentName   string
	SessionID   string
	TaskType    ArtifactType
	TitleHint   string
	Content     string
	Summary     string
	ToolName    string
	Metadata    string
	UserID      string
}

// CreateArtifactResult 创建产出物结果
type CreateArtifactResult struct {
	Artifact *AgentArtifact
	Node     *WorkspaceNode
	File     *WorkspaceFile
	Version  *WorkspaceFileVersion
	PathInfo *ArtifactPathResult
}

// EnsureAgentWorkspace 确保智能体工作空间存在
func (s *Service) EnsureAgentWorkspace(ctx context.Context, tenantID, agentID, agentName, userID string) (*AgentWorkspace, error) {
	var workspace AgentWorkspace
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND agent_id = ?", tenantID, agentID).
		First(&workspace).Error
	if err == nil {
		return &workspace, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 创建智能体工作空间
	return s.createAgentWorkspace(ctx, tenantID, agentID, agentName, userID)
}

func (s *Service) createAgentWorkspace(ctx context.Context, tenantID, agentID, agentName, userID string) (*AgentWorkspace, error) {
	var result *AgentWorkspace

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		agentSlug := slugifyAgent(agentName)

		// 确保 agents 根目录存在
		agentsRoot, err := s.ensureFolderByPath(ctx, tx, tenantID, "agents", "智能体产出", userID)
		if err != nil {
			return err
		}

		// 创建智能体专属目录
		rootNode := &WorkspaceNode{
			TenantID:  tenantID,
			ParentID:  &agentsRoot.ID,
			Name:      agentName,
			Slug:      agentSlug,
			Type:      "folder",
			NodePath:  fmt.Sprintf("agents/%s", agentSlug),
			Category:  "agent_output",
			CreatedBy: userID,
			UpdatedBy: userID,
		}
		if err := tx.Create(rootNode).Error; err != nil {
			return err
		}

		// 创建子目录
		subFolders := GetDefaultAgentFolders()
		var outputsNodeID, draftsNodeID, logsNodeID string

		for _, tpl := range subFolders {
			subNode := &WorkspaceNode{
				TenantID:  tenantID,
				ParentID:  &rootNode.ID,
				Name:      tpl.Name,
				Slug:      tpl.Slug,
				Type:      "folder",
				NodePath:  fmt.Sprintf("%s/%s", rootNode.NodePath, tpl.Slug),
				Category:  tpl.Category,
				CreatedBy: userID,
				UpdatedBy: userID,
			}
			if err := tx.Create(subNode).Error; err != nil {
				return err
			}

			switch tpl.Slug {
			case "outputs":
				outputsNodeID = subNode.ID
			case "drafts":
				draftsNodeID = subNode.ID
			case "logs":
				logsNodeID = subNode.ID
			}
		}

		// 创建工作空间记录
		workspace := &AgentWorkspace{
			TenantID:      tenantID,
			AgentID:       agentID,
			AgentName:     agentName,
			RootNodeID:    rootNode.ID,
			OutputsNodeID: outputsNodeID,
			DraftsNodeID:  draftsNodeID,
			LogsNodeID:    logsNodeID,
		}
		if err := tx.Create(workspace).Error; err != nil {
			return err
		}

		result = workspace
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

// EnsureSessionWorkspace 确保会话工作空间存在
func (s *Service) EnsureSessionWorkspace(ctx context.Context, tenantID, sessionID, userID string) (*SessionWorkspace, error) {
	var workspace SessionWorkspace
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		First(&workspace).Error
	if err == nil {
		return &workspace, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return s.createSessionWorkspace(ctx, tenantID, sessionID, userID)
}

func (s *Service) createSessionWorkspace(ctx context.Context, tenantID, sessionID, userID string) (*SessionWorkspace, error) {
	var result *SessionWorkspace

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sessionSlug := slugifySession(sessionID)

		// 确保 sessions 根目录存在
		sessionsRoot, err := s.ensureFolderByPath(ctx, tx, tenantID, "sessions", "会话记录", userID)
		if err != nil {
			return err
		}

		// 创建会话专属目录
		rootNode := &WorkspaceNode{
			TenantID:  tenantID,
			ParentID:  &sessionsRoot.ID,
			Name:      sessionSlug,
			Slug:      sessionSlug,
			Type:      "folder",
			NodePath:  fmt.Sprintf("sessions/%s", sessionSlug),
			Category:  "session",
			CreatedBy: userID,
			UpdatedBy: userID,
		}
		if err := tx.Create(rootNode).Error; err != nil {
			return err
		}

		// 创建子目录
		subFolders := GetDefaultSessionFolders()
		var contextNodeID, artifactsNodeID, historyNodeID string

		for _, tpl := range subFolders {
			subNode := &WorkspaceNode{
				TenantID:  tenantID,
				ParentID:  &rootNode.ID,
				Name:      tpl.Name,
				Slug:      tpl.Slug,
				Type:      "folder",
				NodePath:  fmt.Sprintf("%s/%s", rootNode.NodePath, tpl.Slug),
				Category:  tpl.Category,
				CreatedBy: userID,
				UpdatedBy: userID,
			}
			if err := tx.Create(subNode).Error; err != nil {
				return err
			}

			switch tpl.Slug {
			case "context":
				contextNodeID = subNode.ID
			case "artifacts":
				artifactsNodeID = subNode.ID
			case "history":
				historyNodeID = subNode.ID
			}
		}

		// 创建工作空间记录
		workspace := &SessionWorkspace{
			TenantID:        tenantID,
			SessionID:       sessionID,
			RootNodeID:      rootNode.ID,
			ContextNodeID:   contextNodeID,
			ArtifactsNodeID: artifactsNodeID,
			HistoryNodeID:   historyNodeID,
		}
		if err := tx.Create(workspace).Error; err != nil {
			return err
		}

		result = workspace
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

// CreateAgentArtifact 创建智能体产出物
func (s *Service) CreateAgentArtifact(ctx context.Context, req *CreateArtifactRequest) (*CreateArtifactResult, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("内容不能为空")
	}

	// 生成路径信息
	pathInfo := s.naming.GenerateArtifactPath(&ArtifactNamingRequest{
		AgentName: req.AgentName,
		AgentID:   req.AgentID,
		SessionID: req.SessionID,
		TaskType:  req.TaskType,
		TitleHint: req.TitleHint,
		Content:   req.Content,
	})

	result := &CreateArtifactResult{
		PathInfo: pathInfo,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 确保智能体工作空间存在
		agentWs, err := s.EnsureAgentWorkspace(ctx, req.TenantID, req.AgentID, req.AgentName, req.UserID)
		if err != nil {
			return err
		}

		// 获取输出目录
		var outputsNode WorkspaceNode
		if err := tx.Where("id = ?", agentWs.OutputsNodeID).First(&outputsNode).Error; err != nil {
			return err
		}

		// 创建文件节点
		fileNode := &WorkspaceNode{
			TenantID:  req.TenantID,
			ParentID:  &outputsNode.ID,
			Name:      pathInfo.FileName,
			Slug:      slugify(pathInfo.FileName),
			Type:      "file",
			NodePath:  pathInfo.FullPath,
			Category:  string(req.TaskType),
			CreatedBy: req.UserID,
			UpdatedBy: req.UserID,
		}
		if err := tx.Create(fileNode).Error; err != nil {
			return err
		}

		// 创建文件元数据
		file := &WorkspaceFile{
			TenantID:  req.TenantID,
			NodeID:    fileNode.ID,
			Category:  string(req.TaskType),
			CreatedBy: req.UserID,
			UpdatedBy: req.UserID,
		}
		if err := tx.Create(file).Error; err != nil {
			return err
		}

		// 创建版本
		version := &WorkspaceFileVersion{
			FileID:    file.ID,
			TenantID:  req.TenantID,
			Content:   req.Content,
			Summary:   req.Summary,
			AgentID:   req.AgentID,
			ToolName:  req.ToolName,
			Metadata:  req.Metadata,
			CreatedBy: req.UserID,
		}
		if err := tx.Create(version).Error; err != nil {
			return err
		}

		// 更新文件最新版本
		if err := tx.Model(file).Update("latest_version_id", version.ID).Error; err != nil {
			return err
		}

		// 创建产出物记录
		artifact := &AgentArtifact{
			TenantID:     req.TenantID,
			AgentID:      req.AgentID,
			AgentName:    req.AgentName,
			SessionID:    req.SessionID,
			NodeID:       fileNode.ID,
			ArtifactType: string(req.TaskType),
			FileName:     pathInfo.FileName,
			FilePath:     pathInfo.FullPath,
			FileSize:     int64(len(req.Content)),
			Summary:      req.Summary,
			TaskType:     string(req.TaskType),
			ToolName:     req.ToolName,
			Status:       "created",
			CreatedBy:    req.UserID,
		}
		if err := tx.Create(artifact).Error; err != nil {
			return err
		}

		// 更新工作空间统计
		now := time.Now().UTC()
		if err := tx.Model(agentWs).Updates(map[string]any{
			"artifact_count":   gorm.Expr("artifact_count + 1"),
			"total_file_size":  gorm.Expr("total_file_size + ?", artifact.FileSize),
			"last_activity_at": &now,
		}).Error; err != nil {
			return err
		}

		result.Artifact = artifact
		result.Node = fileNode
		result.File = file
		result.Version = version
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListAgentArtifacts 列出智能体产出物
func (s *Service) ListAgentArtifacts(ctx context.Context, tenantID, agentID string, limit, offset int) ([]AgentArtifact, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var artifacts []AgentArtifact
	var total int64

	baseQuery := s.db.WithContext(ctx).Model(&AgentArtifact{}).
		Where("tenant_id = ? AND agent_id = ?", tenantID, agentID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := baseQuery.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&artifacts).Error; err != nil {
		return nil, 0, err
	}

	return artifacts, total, nil
}

// ListSessionArtifacts 列出会话产出物
func (s *Service) ListSessionArtifacts(ctx context.Context, tenantID, sessionID string, limit, offset int) ([]AgentArtifact, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var artifacts []AgentArtifact
	var total int64

	baseQuery := s.db.WithContext(ctx).Model(&AgentArtifact{}).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := baseQuery.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&artifacts).Error; err != nil {
		return nil, 0, err
	}

	return artifacts, total, nil
}

// GetAgentWorkspace 获取智能体工作空间
func (s *Service) GetAgentWorkspace(ctx context.Context, tenantID, agentID string) (*AgentWorkspace, error) {
	var workspace AgentWorkspace
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND agent_id = ?", tenantID, agentID).
		First(&workspace).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &workspace, nil
}

// ensureFolderByPath 确保指定路径的文件夹存在
func (s *Service) ensureFolderByPath(ctx context.Context, tx *gorm.DB, tenantID, slug, name, userID string) (*WorkspaceNode, error) {
	var node WorkspaceNode
	err := tx.Where("tenant_id = ? AND node_path = ? AND type = 'folder'", tenantID, slug).First(&node).Error
	if err == nil {
		return &node, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 创建新文件夹
	node = WorkspaceNode{
		TenantID:  tenantID,
		Name:      name,
		Slug:      slug,
		Type:      "folder",
		NodePath:  slug,
		CreatedBy: userID,
		UpdatedBy: userID,
	}
	if err := tx.Create(&node).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// ============================================
// 文件上传下载管理
// ============================================

const (
	UploadStatusPending   = "pending"
	UploadStatusUploading = "uploading"
	UploadStatusCompleted = "completed"
	UploadStatusFailed    = "failed"
)

const (
	DefaultChunkSize  = 5 * 1024 * 1024 // 5MB
	MaxFileSize       = 500 * 1024 * 1024 // 500MB
	DefaultUploadPath = "uploads"
)

// UploadFileRequest 文件上传请求
type UploadFileRequest struct {
	TenantID     string
	FileName     string
	MimeType     string
	FileSize     int64
	ParentID     *string
	UserID       string
	StoragePath  string
}

// UploadChunkRequest 分片上传请求
type UploadChunkRequest struct {
	TenantID   string
	UploadID   string
	ChunkIndex int
	ChunkData  io.Reader
	ChunkSize  int64
	UserID     string
}

// InitiateUploadRequest 初始化分片上传
type InitiateUploadRequest struct {
	TenantID   string
	FileName   string
	MimeType   string
	FileSize   int64
	ChunkSize  int64
	ParentID   *string
	UserID     string
}

// InitiateUploadResponse 初始化上传响应
type InitiateUploadResponse struct {
	UploadID    string   `json:"uploadId"`
	ChunkSize   int64    `json:"chunkSize"`
	ChunkCount  int      `json:"chunkCount"`
	StoragePath string   `json:"storagePath"`
}

// UploadedFile 上传完成的文件信息
type UploadedFile struct {
	Upload  *FileUpload    `json:"upload"`
	Node    *WorkspaceNode `json:"node"`
	File    *WorkspaceFile `json:"file"`
}

// FilePreview 文件预览信息
type FilePreview struct {
	NodeID      string `json:"nodeId"`
	FileName    string `json:"fileName"`
	MimeType    string `json:"mimeType"`
	FileSize    int64  `json:"fileSize"`
	PreviewType string `json:"previewType"`
	Content     string `json:"content,omitempty"`
	PreviewURL  string `json:"previewUrl,omitempty"`
	CanPreview  bool   `json:"canPreview"`
}

// GetStoragePath 获取存储路径
func (s *Service) GetStoragePath() string {
	return DefaultUploadPath
}

// InitiateUpload 初始化文件上传（支持大文件分片）
func (s *Service) InitiateUpload(ctx context.Context, req *InitiateUploadRequest) (*InitiateUploadResponse, error) {
	if strings.TrimSpace(req.FileName) == "" {
		return nil, errors.New("文件名不能为空")
	}
	if req.FileSize <= 0 {
		return nil, errors.New("文件大小无效")
	}
	if req.FileSize > MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制(%dMB)", MaxFileSize/1024/1024)
	}

	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	chunkCount := int((req.FileSize + chunkSize - 1) / chunkSize)

	// 生成存储路径
	timestamp := time.Now().Format("20060102")
	storagePath := filepath.Join(DefaultUploadPath, req.TenantID, timestamp, uuid.New().String())

	// 确保目录存在
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	// 创建上传记录
	upload := &FileUpload{
		TenantID:     req.TenantID,
		FileName:     filepath.Base(req.FileName),
		OriginalName: req.FileName,
		MimeType:     req.MimeType,
		FileSize:     req.FileSize,
		StoragePath:  storagePath,
		Status:       UploadStatusPending,
		ChunkCount:   chunkCount,
		CreatedBy:    req.UserID,
	}

	if err := s.db.WithContext(ctx).Create(upload).Error; err != nil {
		return nil, err
	}

	return &InitiateUploadResponse{
		UploadID:    upload.ID,
		ChunkSize:   chunkSize,
		ChunkCount:  chunkCount,
		StoragePath: storagePath,
	}, nil
}

// UploadChunk 上传文件分片
func (s *Service) UploadChunk(ctx context.Context, req *UploadChunkRequest) error {
	var upload FileUpload
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", req.UploadID, req.TenantID).
		First(&upload).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("上传任务不存在")
		}
		return err
	}

	if upload.Status == UploadStatusCompleted {
		return errors.New("上传已完成")
	}

	// 更新状态为上传中
	if upload.Status == UploadStatusPending {
		if err := s.db.WithContext(ctx).Model(&upload).Update("status", UploadStatusUploading).Error; err != nil {
			return err
		}
	}

	// 写入分片文件
	chunkPath := filepath.Join(upload.StoragePath, fmt.Sprintf("chunk_%05d", req.ChunkIndex))
	chunkFile, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("创建分片文件失败: %w", err)
	}
	defer chunkFile.Close()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(chunkFile, hasher), req.ChunkData)
	if err != nil {
		return fmt.Errorf("写入分片数据失败: %w", err)
	}

	// 记录分片信息
	chunk := &FileChunk{
		UploadID:    req.UploadID,
		ChunkIndex:  req.ChunkIndex,
		ChunkSize:   written,
		StoragePath: chunkPath,
		Hash:        hex.EncodeToString(hasher.Sum(nil)),
	}
	if err := s.db.WithContext(ctx).Create(chunk).Error; err != nil {
		return err
	}

	return nil
}

// CompleteUpload 完成文件上传（合并分片）
func (s *Service) CompleteUpload(ctx context.Context, tenantID, uploadID, userID string, parentID *string) (*UploadedFile, error) {
	var upload FileUpload
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", uploadID, tenantID).
		First(&upload).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("上传任务不存在")
		}
		return nil, err
	}

	if upload.Status == UploadStatusCompleted {
		return nil, errors.New("上传已完成")
	}

	// 检查分片是否完整
	var chunks []FileChunk
	if err := s.db.WithContext(ctx).
		Where("upload_id = ?", uploadID).
		Order("chunk_index ASC").
		Find(&chunks).Error; err != nil {
		return nil, err
	}

	if len(chunks) != upload.ChunkCount {
		return nil, fmt.Errorf("分片不完整: 期望 %d 个，实际 %d 个", upload.ChunkCount, len(chunks))
	}

	// 合并分片
	finalPath := filepath.Join(upload.StoragePath, upload.FileName)
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return nil, fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer finalFile.Close()

	hasher := sha256.New()
	for _, chunk := range chunks {
		chunkFile, err := os.Open(chunk.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("打开分片文件失败: %w", err)
		}
		if _, err := io.Copy(io.MultiWriter(finalFile, hasher), chunkFile); err != nil {
			chunkFile.Close()
			return nil, fmt.Errorf("合并分片失败: %w", err)
		}
		chunkFile.Close()
		// 删除分片文件
		os.Remove(chunk.StoragePath)
	}

	// 更新上传记录
	now := time.Now().UTC()
	upload.Status = UploadStatusCompleted
	upload.Hash = hex.EncodeToString(hasher.Sum(nil))
	upload.StoragePath = finalPath
	upload.UploadedAt = &now

	if err := s.db.WithContext(ctx).Save(&upload).Error; err != nil {
		return nil, err
	}

	// 创建工作空间节点
	result := &UploadedFile{Upload: &upload}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 确定父目录
		var parent *WorkspaceNode
		parentPath := ""
		if parentID != nil && *parentID != "" {
			p, err := s.GetNode(ctx, tenantID, *parentID)
			if err != nil {
				return err
			}
			if p != nil && p.Type == "folder" {
				parent = p
				parentPath = p.NodePath
			}
		}

		// 创建文件节点
		slug := slugify(upload.FileName)
		nodePath := slug
		if parentPath != "" {
			nodePath = parentPath + "/" + slug
		}

		node := &WorkspaceNode{
			TenantID:  tenantID,
			ParentID:  parentID,
			Name:      upload.OriginalName,
			Slug:      slug,
			Type:      "file",
			NodePath:  nodePath,
			Category:  "upload",
			CreatedBy: userID,
			UpdatedBy: userID,
		}
		if parent != nil {
			node.Category = parent.Category
		}
		if err := tx.Create(node).Error; err != nil {
			return err
		}

		// 创建文件元数据
		file := &WorkspaceFile{
			TenantID:  tenantID,
			NodeID:    node.ID,
			Category:  "upload",
			CreatedBy: userID,
			UpdatedBy: userID,
		}
		if err := tx.Create(file).Error; err != nil {
			return err
		}

		// 关联上传记录
		if err := tx.Model(&upload).Update("node_id", node.ID).Error; err != nil {
			return err
		}

		result.Node = node
		result.File = file
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// UploadSingleFile 单文件上传（小文件直接上传）
func (s *Service) UploadSingleFile(ctx context.Context, req *UploadFileRequest, data io.Reader) (*UploadedFile, error) {
	if strings.TrimSpace(req.FileName) == "" {
		return nil, errors.New("文件名不能为空")
	}
	if req.FileSize > MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制(%dMB)", MaxFileSize/1024/1024)
	}

	// 生成存储路径
	timestamp := time.Now().Format("20060102")
	storagePath := filepath.Join(DefaultUploadPath, req.TenantID, timestamp)
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	finalPath := filepath.Join(storagePath, uuid.New().String()+"_"+filepath.Base(req.FileName))
	file, err := os.Create(finalPath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hasher), data)
	if err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	now := time.Now().UTC()
	upload := &FileUpload{
		TenantID:     req.TenantID,
		FileName:     filepath.Base(finalPath),
		OriginalName: req.FileName,
		MimeType:     req.MimeType,
		FileSize:     written,
		StoragePath:  finalPath,
		Status:       UploadStatusCompleted,
		Hash:         hex.EncodeToString(hasher.Sum(nil)),
		UploadedAt:   &now,
		CreatedBy:    req.UserID,
	}

	if err := s.db.WithContext(ctx).Create(upload).Error; err != nil {
		return nil, err
	}

	// 创建工作空间节点
	result := &UploadedFile{Upload: upload}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent *WorkspaceNode
		parentPath := ""
		if req.ParentID != nil && *req.ParentID != "" {
			p, err := s.GetNode(ctx, req.TenantID, *req.ParentID)
			if err != nil {
				return err
			}
			if p != nil && p.Type == "folder" {
				parent = p
				parentPath = p.NodePath
			}
		}

		slug := slugify(req.FileName)
		nodePath := slug
		if parentPath != "" {
			nodePath = parentPath + "/" + slug
		}

		node := &WorkspaceNode{
			TenantID:  req.TenantID,
			ParentID:  req.ParentID,
			Name:      req.FileName,
			Slug:      slug,
			Type:      "file",
			NodePath:  nodePath,
			Category:  "upload",
			CreatedBy: req.UserID,
			UpdatedBy: req.UserID,
		}
		if parent != nil {
			node.Category = parent.Category
		}
		if err := tx.Create(node).Error; err != nil {
			return err
		}

		wsFile := &WorkspaceFile{
			TenantID:  req.TenantID,
			NodeID:    node.ID,
			Category:  "upload",
			CreatedBy: req.UserID,
			UpdatedBy: req.UserID,
		}
		if err := tx.Create(wsFile).Error; err != nil {
			return err
		}

		if err := tx.Model(upload).Update("node_id", node.ID).Error; err != nil {
			return err
		}

		result.Node = node
		result.File = wsFile
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// GetFileForDownload 获取文件下载信息
func (s *Service) GetFileForDownload(ctx context.Context, tenantID, nodeID string) (*FileUpload, error) {
	var upload FileUpload
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND node_id = ? AND status = ?", tenantID, nodeID, UploadStatusCompleted).
		First(&upload).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("文件不存在或未完成上传")
		}
		return nil, err
	}
	return &upload, nil
}

// GetFileByUploadID 通过上传ID获取文件
func (s *Service) GetFileByUploadID(ctx context.Context, tenantID, uploadID string) (*FileUpload, error) {
	var upload FileUpload
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", uploadID, tenantID).
		First(&upload).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("文件不存在")
		}
		return nil, err
	}
	return &upload, nil
}

// ============================================
// 文件预览服务
// ============================================

// 可预览的 MIME 类型
var previewableMimeTypes = map[string]string{
	"text/plain":             "text",
	"text/html":              "text",
	"text/css":               "text",
	"text/javascript":        "text",
	"application/javascript": "text",
	"application/json":       "text",
	"text/markdown":          "markdown",
	"text/x-markdown":        "markdown",
	"image/jpeg":             "image",
	"image/png":              "image",
	"image/gif":              "image",
	"image/webp":             "image",
	"image/svg+xml":          "image",
	"application/pdf":        "pdf",
	"video/mp4":              "video",
	"video/webm":             "video",
	"audio/mpeg":             "audio",
	"audio/wav":              "audio",
}

// GetFilePreview 获取文件预览信息
func (s *Service) GetFilePreview(ctx context.Context, tenantID, nodeID string) (*FilePreview, error) {
	node, err := s.GetNode(ctx, tenantID, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, errors.New("文件不存在")
	}

	preview := &FilePreview{
		NodeID:   nodeID,
		FileName: node.Name,
	}

	// 首先检查是否是上传的文件
	var upload FileUpload
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND node_id = ?", tenantID, nodeID).
		First(&upload).Error; err == nil {
		preview.MimeType = upload.MimeType
		preview.FileSize = upload.FileSize

		if previewType, ok := previewableMimeTypes[upload.MimeType]; ok {
			preview.PreviewType = previewType
			preview.CanPreview = true

			// 文本类型直接读取内容
			if previewType == "text" || previewType == "markdown" {
				content, err := s.readFileContent(upload.StoragePath, 1024*1024) // 最多读取1MB
				if err == nil {
					preview.Content = content
				}
			}
		}
		return preview, nil
	}

	// 检查是否是工作空间文件（数据库存储内容）
	detail, err := s.GetFileDetail(ctx, tenantID, nodeID)
	if err != nil {
		return nil, err
	}
	if detail.Version != nil {
		preview.MimeType = s.guessMimeType(node.Name)
		preview.FileSize = int64(len(detail.Version.Content))
		preview.PreviewType = "text"
		preview.CanPreview = true
		preview.Content = detail.Version.Content
	}

	return preview, nil
}

// readFileContent 读取文件内容
func (s *Service) readFileContent(path string, maxSize int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	limited := io.LimitReader(file, maxSize)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// guessMimeType 根据文件名猜测 MIME 类型
func (s *Service) guessMimeType(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return "application/octet-stream"
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// 自定义扩展名映射
		customTypes := map[string]string{
			".md":    "text/markdown",
			".go":    "text/x-go",
			".py":    "text/x-python",
			".js":    "application/javascript",
			".ts":    "text/typescript",
			".tsx":   "text/typescript-jsx",
			".jsx":   "text/javascript-jsx",
			".vue":   "text/x-vue",
			".yaml":  "text/yaml",
			".yml":   "text/yaml",
			".toml":  "text/toml",
			".sql":   "text/x-sql",
			".sh":    "text/x-shellscript",
			".bash":  "text/x-shellscript",
			".zsh":   "text/x-shellscript",
			".rs":    "text/x-rust",
			".rb":    "text/x-ruby",
			".java":  "text/x-java",
			".kt":    "text/x-kotlin",
			".swift": "text/x-swift",
			".c":     "text/x-c",
			".cpp":   "text/x-c++",
			".h":     "text/x-c",
			".hpp":   "text/x-c++",
		}
		if t, ok := customTypes[strings.ToLower(ext)]; ok {
			return t
		}
		return "application/octet-stream"
	}
	return mimeType
}

// ListUploads 列出上传记录
func (s *Service) ListUploads(ctx context.Context, tenantID string, limit, offset int) ([]FileUpload, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var uploads []FileUpload
	var total int64

	baseQuery := s.db.WithContext(ctx).Model(&FileUpload{}).
		Where("tenant_id = ?", tenantID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := baseQuery.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&uploads).Error; err != nil {
		return nil, 0, err
	}

	return uploads, total, nil
}

// DeleteUpload 删除上传文件
func (s *Service) DeleteUpload(ctx context.Context, tenantID, uploadID string) error {
	var upload FileUpload
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", uploadID, tenantID).
		First(&upload).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("文件不存在")
		}
		return err
	}

	// 删除物理文件
	if upload.StoragePath != "" {
		os.Remove(upload.StoragePath)
		// 尝试删除空目录
		os.Remove(filepath.Dir(upload.StoragePath))
	}

	// 删除分片记录
	if err := s.db.WithContext(ctx).
		Where("upload_id = ?", uploadID).
		Delete(&FileChunk{}).Error; err != nil {
		return err
	}

	// 删除上传记录
	return s.db.WithContext(ctx).Delete(&upload).Error
}
