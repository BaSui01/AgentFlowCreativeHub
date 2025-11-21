package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service 管理工作区数据
type Service struct {
	db          *gorm.DB
	naming      *AutoNamingPolicy
	initialized sync.Map
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
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	NodePath    string        `json:"nodePath"`
	Category    string        `json:"category"`
	Metadata    string        `json:"metadata"`
	HasChildren bool          `json:"hasChildren"`
	Children    []*TreeNode   `json:"children"`
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
	TenantID string
	NodeID   string
	Content  string
	Summary  string
	AgentID  string
	ToolName string
	Metadata string
	UserID   string
}

// CreateStagingRequest 创建暂存文件
type CreateStagingRequest struct {
	TenantID     string
	FileType     string
	Content      string
	TitleHint    string
	Summary      string
	AgentID      string
	AgentName    string
	Command      string
	Metadata     string
	CreatedBy    string
	ManualFolder string
}

// ListStagingRequest 查询暂存
type ListStagingRequest struct {
	TenantID string
	Status   string
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

// ListTree 返回整棵树
func (s *Service) ListTree(ctx context.Context, tenantID string) ([]*TreeNode, error) {
	var nodes []WorkspaceNode
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("sort_order ASC, node_path ASC").
		Find(&nodes).Error; err != nil {
		return nil, err
	}
	byParent := make(map[string][]*TreeNode)
	root := make([]*TreeNode, 0)
	for _, n := range nodes {
		tn := &TreeNode{
			ID:       n.ID,
			Name:     n.Name,
			Type:     n.Type,
			NodePath: n.NodePath,
			Category: n.Category,
			Metadata: n.Metadata,
		}
		parent := "root"
		if n.ParentID != nil {
			parent = *n.ParentID
		}
		byParent[parent] = append(byParent[parent], tn)
	}
	var attachChildren func(parentID string, nodes []*TreeNode)
	attachChildren = func(parentID string, nodes []*TreeNode) {
		for _, node := range nodes {
			children := byParent[node.ID]
			if len(children) > 0 {
				sort.SliceStable(children, func(i, j int) bool {
					return strings.Compare(children[i].Name, children[j].Name) < 0
				})
				node.Children = children
				node.HasChildren = true
				attachChildren(node.ID, children)
			}
		}
	}
	root = byParent["root"]
	attachChildren("root", root)
	return root, nil
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

// CreateStagingFile 写入暂存
func (s *Service) CreateStagingFile(ctx context.Context, req *CreateStagingRequest) (*WorkspaceStagingFile, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("内容不能为空")
	}
	tpl := s.naming.ResolveFolder(req.FileType)
	folder := tpl.Slug
	if req.ManualFolder != "" {
		folder = slugify(req.ManualFolder)
	}
	name := s.naming.SuggestName(req.FileType, req.TitleHint, req.Content)
	path := fmt.Sprintf("%s/%s", folder, slugify(name))
	staging := &WorkspaceStagingFile{
		TenantID:        req.TenantID,
		FileType:        strings.ToLower(req.FileType),
		SuggestedName:   name,
		SuggestedFolder: folder,
		SuggestedPath:   path,
		Content:         req.Content,
		Summary:         req.Summary,
		SourceAgentID:   req.AgentID,
		SourceAgentName: req.AgentName,
		SourceCommand:   req.Command,
		Metadata:        req.Metadata,
		CreatedBy:       req.CreatedBy,
		UpdatedBy:       req.CreatedBy,
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
		Order("created_at DESC")
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// PublishStagingFile 将暂存转为正式文件
func (s *Service) PublishStagingFile(ctx context.Context, tenantID, stagingID, reviewerID string) (*WorkspaceFile, *WorkspaceFileVersion, error) {
	var file WorkspaceFile
	var version WorkspaceFileVersion
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var staging WorkspaceStagingFile
		if err := tx.Where("id = ? AND tenant_id = ?", stagingID, tenantID).First(&staging).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("暂存记录不存在")
			}
			return err
		}
		if staging.Status != "pending" {
			return errors.New("该记录已处理")
		}
		folderPath := staging.SuggestedFolder
		parent, err := s.ensureFolderPath(ctx, tx, tenantID, folderPath, reviewerID)
		if err != nil {
			return err
		}
		fileNode, err := s.ensureFileNode(ctx, tx, tenantID, parent, staging.SuggestedName, reviewerID)
		if err != nil {
			return err
		}
		if err := tx.Where("node_id = ?", fileNode.ID).First(&file).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				file = WorkspaceFile{
					TenantID:  tenantID,
					NodeID:    fileNode.ID,
					Category:  fileNode.Category,
					CreatedBy: reviewerID,
					UpdatedBy: reviewerID,
				}
				if err := tx.Create(&file).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
		version = WorkspaceFileVersion{
			FileID:    file.ID,
			TenantID:  tenantID,
			Content:   staging.Content,
			Summary:   staging.Summary,
			AgentID:   staging.SourceAgentID,
			ToolName:  staging.SourceCommand,
			Metadata:  staging.Metadata,
			CreatedBy: reviewerID,
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"latest_version_id": version.ID,
			"review_status":     "published",
			"approver_id":       reviewerID,
			"updated_by":        reviewerID,
		}
		approvedAt := time.Now().UTC()
		updates["approved_at"] = &approvedAt
		if err := tx.Model(&file).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&staging).Updates(map[string]any{
			"status":      "approved",
			"reviewer_id": reviewerID,
			"reviewed_at": approvedAt,
		}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return &file, &version, nil
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
	slug := slugify(name)
	path := slug
	var parentID *string
	if parent != nil {
		path = fmt.Sprintf("%s/%s", parent.NodePath, slug)
		parentID = &parent.ID
	}
	var node WorkspaceNode
	if err := tx.Where("tenant_id = ? AND node_path = ?", tenantID, path).
		First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			node = WorkspaceNode{
				TenantID:  tenantID,
				ParentID:  parentID,
				Name:      name,
				Slug:      slug,
				Type:      "file",
				NodePath:  path,
				Category:  parent.Category,
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
	return &node, nil
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
