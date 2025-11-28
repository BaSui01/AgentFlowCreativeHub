package workspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 内容层级类型常量（四级结构）
const (
	ContentTypeWork    = "work"    // 作品
	ContentTypeVolume  = "volume"  // 卷
	ContentTypeChapter = "chapter" // 章节
	ContentTypeScene   = "scene"   // 场景
)

// OutlineItem 大纲项
type OutlineItem struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`     // work/volume/chapter/scene
	Category    string         `json:"category"` // 内容分类
	NodePath    string         `json:"nodePath"`
	SortOrder   int            `json:"sortOrder"`
	WordCount   int            `json:"wordCount"`
	Summary     string         `json:"summary"`
	HasChildren bool           `json:"hasChildren"`
	Children    []*OutlineItem `json:"children,omitempty"`
}

// OutlineView 大纲总览
type OutlineView struct {
	WorkID       string         `json:"workId"`
	WorkName     string         `json:"workName"`
	TotalWords   int            `json:"totalWords"`
	VolumeCount  int            `json:"volumeCount"`
	ChapterCount int            `json:"chapterCount"`
	SceneCount   int            `json:"sceneCount"`
	Items        []*OutlineItem `json:"items"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

// SortOrderUpdate 排序更新项
type SortOrderUpdate struct {
	NodeID    string `json:"nodeId"`
	SortOrder int    `json:"sortOrder"`
}

// BatchSortRequest 批量排序请求
type BatchSortRequest struct {
	TenantID string
	Updates  []SortOrderUpdate
	UserID   string
}

// BatchMoveRequest 批量移动请求
type BatchMoveRequest struct {
	TenantID    string
	NodeIDs     []string
	NewParentID string
	UserID      string
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	TenantID string
	NodeIDs  []string
	UserID   string
}

// BatchCopyRequest 批量复制请求
type BatchCopyRequest struct {
	TenantID    string
	NodeIDs     []string
	NewParentID string
	UserID      string
}

// AutoSaveRequest 自动保存请求
type AutoSaveRequest struct {
	TenantID  string
	NodeID    string
	Content   string
	UserID    string
	SessionID string
}

// AutoSaveResponse 自动保存响应
type AutoSaveResponse struct {
	VersionID string    `json:"versionId"`
	SavedAt   time.Time `json:"savedAt"`
	WordCount int       `json:"wordCount"`
}

// ImportTextRequest txt导入请求
type ImportTextRequest struct {
	TenantID   string
	ParentID   *string
	Content    string
	FileName   string
	UserID     string
	AutoDetect bool // 是否自动检测章节结构
}

// ImportResult 导入结果
type ImportResult struct {
	WorkID       string `json:"workId"`
	WorkName     string `json:"workName"`
	ChapterCount int    `json:"chapterCount"`
	TotalWords   int    `json:"totalWords"`
}

// GetOutlineView 获取大纲总览
func (s *Service) GetOutlineView(ctx context.Context, tenantID, workID string) (*OutlineView, error) {
	// 获取作品根节点
	work, err := s.GetNode(ctx, tenantID, workID)
	if err != nil {
		return nil, err
	}
	if work == nil {
		return nil, errors.New("作品不存在")
	}

	// 查询所有子节点
	var nodes []WorkspaceNode
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Where("node_path LIKE ? OR id = ?", work.NodePath+"/%", workID).
		Order("sort_order ASC, node_path ASC").
		Find(&nodes).Error; err != nil {
		return nil, err
	}

	// 统计数据
	view := &OutlineView{
		WorkID:    work.ID,
		WorkName:  work.Name,
		UpdatedAt: work.UpdatedAt,
	}

	// 获取文件内容统计词数
	wordCounts := make(map[string]int)
	summaries := make(map[string]string)
	for _, n := range nodes {
		if n.Type == "file" {
			detail, err := s.GetFileDetail(ctx, tenantID, n.ID)
			if err == nil && detail.Version != nil {
				wc := countWords(detail.Version.Content)
				wordCounts[n.ID] = wc
				view.TotalWords += wc
				if len(detail.Version.Summary) > 0 {
					summaries[n.ID] = detail.Version.Summary
				}
			}
		}
	}

	// 构建树形结构
	byParent := make(map[string][]*OutlineItem)
	index := make(map[string]*OutlineItem)

	for _, n := range nodes {
		item := &OutlineItem{
			ID:        n.ID,
			Name:      n.Name,
			Type:      n.Type,
			Category:  n.Category,
			NodePath:  n.NodePath,
			SortOrder: n.SortOrder,
			WordCount: wordCounts[n.ID],
			Summary:   summaries[n.ID],
		}
		index[n.ID] = item

		// 统计类型数量
		switch n.Category {
		case ContentTypeVolume:
			view.VolumeCount++
		case ContentTypeChapter:
			view.ChapterCount++
		case ContentTypeScene:
			view.SceneCount++
		}

		parent := "root"
		if n.ParentID != nil {
			parent = *n.ParentID
		}
		byParent[parent] = append(byParent[parent], item)
	}

	// 递归构建子节点
	var buildChildren func(item *OutlineItem)
	buildChildren = func(item *OutlineItem) {
		children := byParent[item.ID]
		if len(children) > 0 {
			item.HasChildren = true
			item.Children = children
			// 累计子节点词数
			for _, child := range children {
				buildChildren(child)
				item.WordCount += child.WordCount
			}
		}
	}

	// 从作品根节点开始构建
	if workItem, ok := index[workID]; ok {
		buildChildren(workItem)
		view.Items = []*OutlineItem{workItem}
	}

	return view, nil
}

// BatchUpdateSortOrder 批量更新排序
func (s *Service) BatchUpdateSortOrder(ctx context.Context, req *BatchSortRequest) error {
	if len(req.Updates) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, u := range req.Updates {
			if err := tx.Model(&WorkspaceNode{}).
				Where("id = ? AND tenant_id = ?", u.NodeID, req.TenantID).
				Updates(map[string]interface{}{
					"sort_order": u.SortOrder,
					"updated_by": req.UserID,
					"updated_at": time.Now().UTC(),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// BatchMoveNodes 批量移动节点
func (s *Service) BatchMoveNodes(ctx context.Context, req *BatchMoveRequest) error {
	if len(req.NodeIDs) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 验证目标父节点
		var newParent *WorkspaceNode
		if req.NewParentID != "" {
			var p WorkspaceNode
			if err := tx.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", req.NewParentID, req.TenantID).
				First(&p).Error; err != nil {
				return errors.New("目标文件夹不存在")
			}
			if p.Type != "folder" {
				return errors.New("目标不是文件夹")
			}
			newParent = &p
		}

		for _, nodeID := range req.NodeIDs {
			var node WorkspaceNode
			if err := tx.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", nodeID, req.TenantID).
				First(&node).Error; err != nil {
				continue
			}

			oldPath := node.NodePath
			newPath := node.Slug
			if newParent != nil {
				newPath = fmt.Sprintf("%s/%s", newParent.NodePath, node.Slug)
				node.ParentID = &newParent.ID
			} else {
				node.ParentID = nil
			}
			node.NodePath = newPath
			node.UpdatedBy = req.UserID
			node.UpdatedAt = time.Now().UTC()

			if err := tx.Save(&node).Error; err != nil {
				return err
			}

			// 更新子节点路径
			if node.Type == "folder" {
				if err := tx.Model(&WorkspaceNode{}).
					Where("tenant_id = ? AND node_path LIKE ? AND deleted_at IS NULL", req.TenantID, oldPath+"/%").
					Update("node_path", gorm.Expr("REPLACE(node_path, ?, ?)", oldPath, newPath)).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// BatchDeleteNodes 批量删除节点
func (s *Service) BatchDeleteNodes(ctx context.Context, req *BatchDeleteRequest) (int, error) {
	if len(req.NodeIDs) == 0 {
		return 0, nil
	}

	var deleted int
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, nodeID := range req.NodeIDs {
			var node WorkspaceNode
			if err := tx.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", nodeID, req.TenantID).
				First(&node).Error; err != nil {
				continue
			}

			// 删除节点及其子节点
			result := tx.Where("tenant_id = ? AND (id = ? OR node_path LIKE ?)", req.TenantID, nodeID, node.NodePath+"/%").
				Delete(&WorkspaceNode{})
			if result.Error != nil {
				return result.Error
			}
			deleted += int(result.RowsAffected)
		}
		return nil
	})
	return deleted, err
}

// BatchCopyNodes 批量复制节点
func (s *Service) BatchCopyNodes(ctx context.Context, req *BatchCopyRequest) ([]string, error) {
	if len(req.NodeIDs) == 0 {
		return nil, nil
	}

	var copiedIDs []string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 验证目标父节点
		var newParent *WorkspaceNode
		if req.NewParentID != "" {
			var p WorkspaceNode
			if err := tx.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", req.NewParentID, req.TenantID).
				First(&p).Error; err != nil {
				return errors.New("目标文件夹不存在")
			}
			newParent = &p
		}

		for _, nodeID := range req.NodeIDs {
			var node WorkspaceNode
			if err := tx.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", nodeID, req.TenantID).
				First(&node).Error; err != nil {
				continue
			}

			// 复制节点
			newID := uuid.New().String()
			newPath := node.Slug + "-copy"
			if newParent != nil {
				newPath = fmt.Sprintf("%s/%s-copy", newParent.NodePath, node.Slug)
			}

			newNode := WorkspaceNode{
				ID:        newID,
				TenantID:  req.TenantID,
				ParentID:  nil,
				Name:      node.Name + " (副本)",
				Slug:      node.Slug + "-copy",
				Type:      node.Type,
				NodePath:  newPath,
				Category:  node.Category,
				SortOrder: node.SortOrder + 1,
				Metadata:  node.Metadata,
				CreatedBy: req.UserID,
				UpdatedBy: req.UserID,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			if newParent != nil {
				newNode.ParentID = &newParent.ID
			}

			if err := tx.Create(&newNode).Error; err != nil {
				return err
			}
			copiedIDs = append(copiedIDs, newID)

			// 如果是文件，复制内容
			if node.Type == "file" {
				if err := s.copyFileContent(ctx, tx, req.TenantID, node.ID, newID, req.UserID); err != nil {
					return err
				}
			}

			// 如果是文件夹，递归复制子节点
			if node.Type == "folder" {
				if err := s.copyFolderChildren(ctx, tx, req.TenantID, node.ID, newID, newPath, req.UserID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return copiedIDs, err
}

// copyFileContent 复制文件内容
func (s *Service) copyFileContent(ctx context.Context, tx *gorm.DB, tenantID, srcNodeID, dstNodeID, userID string) error {
	var srcFile WorkspaceFile
	if err := tx.Where("node_id = ? AND tenant_id = ?", srcNodeID, tenantID).First(&srcFile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	// 创建新文件记录
	newFile := WorkspaceFile{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		NodeID:    dstNodeID,
		Category:  srcFile.Category,
		CreatedBy: userID,
		UpdatedBy: userID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// 复制最新版本内容
	if srcFile.LatestVersionID != "" {
		var srcVersion WorkspaceFileVersion
		if err := tx.Where("id = ?", srcFile.LatestVersionID).First(&srcVersion).Error; err == nil {
			newVersion := WorkspaceFileVersion{
				ID:        uuid.New().String(),
				FileID:    newFile.ID,
				TenantID:  tenantID,
				Content:   srcVersion.Content,
				Summary:   srcVersion.Summary + " (副本)",
				CreatedBy: userID,
				CreatedAt: time.Now().UTC(),
			}
			if err := tx.Create(&newVersion).Error; err != nil {
				return err
			}
			newFile.LatestVersionID = newVersion.ID
		}
	}

	return tx.Create(&newFile).Error
}

// copyFolderChildren 递归复制子节点
func (s *Service) copyFolderChildren(ctx context.Context, tx *gorm.DB, tenantID, srcFolderID, dstFolderID, dstPath, userID string) error {
	var children []WorkspaceNode
	if err := tx.Where("parent_id = ? AND tenant_id = ? AND deleted_at IS NULL", srcFolderID, tenantID).
		Find(&children).Error; err != nil {
		return err
	}

	for _, child := range children {
		newID := uuid.New().String()
		newPath := fmt.Sprintf("%s/%s", dstPath, child.Slug)

		newNode := WorkspaceNode{
			ID:        newID,
			TenantID:  tenantID,
			ParentID:  &dstFolderID,
			Name:      child.Name,
			Slug:      child.Slug,
			Type:      child.Type,
			NodePath:  newPath,
			Category:  child.Category,
			SortOrder: child.SortOrder,
			Metadata:  child.Metadata,
			CreatedBy: userID,
			UpdatedBy: userID,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := tx.Create(&newNode).Error; err != nil {
			return err
		}

		if child.Type == "file" {
			if err := s.copyFileContent(ctx, tx, tenantID, child.ID, newID, userID); err != nil {
				return err
			}
		} else if child.Type == "folder" {
			if err := s.copyFolderChildren(ctx, tx, tenantID, child.ID, newID, newPath, userID); err != nil {
				return err
			}
		}
	}
	return nil
}

// AutoSaveContent 自动保存内容（防抖版本）
func (s *Service) AutoSaveContent(ctx context.Context, req *AutoSaveRequest) (*AutoSaveResponse, error) {
	if req.NodeID == "" || req.Content == "" {
		return nil, errors.New("参数不完整")
	}

	node, err := s.GetNode(ctx, req.TenantID, req.NodeID)
	if err != nil {
		return nil, err
	}
	if node == nil || node.Type != "file" {
		return nil, errors.New("文件不存在")
	}

	var file WorkspaceFile
	var isNew bool
	if err := s.db.WithContext(ctx).Where("node_id = ? AND tenant_id = ?", req.NodeID, req.TenantID).
		First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			file = WorkspaceFile{
				ID:        uuid.New().String(),
				TenantID:  req.TenantID,
				NodeID:    req.NodeID,
				Category:  node.Category,
				CreatedBy: req.UserID,
				UpdatedBy: req.UserID,
			}
			isNew = true
		} else {
			return nil, err
		}
	}

	// 创建新版本
	version := WorkspaceFileVersion{
		ID:        uuid.New().String(),
		FileID:    file.ID,
		TenantID:  req.TenantID,
		Content:   req.Content,
		Summary:   fmt.Sprintf("自动保存 - %s", time.Now().Format("15:04:05")),
		CreatedBy: req.UserID,
		CreatedAt: time.Now().UTC(),
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if isNew {
			if err := tx.Create(&file).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		return tx.Model(&file).Updates(map[string]interface{}{
			"latest_version_id": version.ID,
			"updated_by":        req.UserID,
			"updated_at":        time.Now().UTC(),
		}).Error
	})
	if err != nil {
		return nil, err
	}

	return &AutoSaveResponse{
		VersionID: version.ID,
		SavedAt:   version.CreatedAt,
		WordCount: countWords(req.Content),
	}, nil
}

// ImportFromText 从txt文件导入（智能解析章节）
func (s *Service) ImportFromText(ctx context.Context, req *ImportTextRequest) (*ImportResult, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("内容不能为空")
	}

	// 解析章节结构
	chapters := parseChapters(req.Content, req.AutoDetect)
	if len(chapters) == 0 {
		return nil, errors.New("未能解析出章节结构")
	}

	// 创建作品根节点
	workName := strings.TrimSuffix(req.FileName, ".txt")
	if workName == "" {
		workName = "导入作品"
	}

	result := &ImportResult{
		WorkName: workName,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建作品文件夹
		workNode := &WorkspaceNode{
			ID:        uuid.New().String(),
			TenantID:  req.TenantID,
			ParentID:  req.ParentID,
			Name:      workName,
			Slug:      slugify(workName),
			Type:      "folder",
			Category:  ContentTypeWork,
			CreatedBy: req.UserID,
			UpdatedBy: req.UserID,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if req.ParentID != nil {
			var parent WorkspaceNode
			if err := tx.Where("id = ?", *req.ParentID).First(&parent).Error; err == nil {
				workNode.NodePath = fmt.Sprintf("%s/%s", parent.NodePath, workNode.Slug)
			}
		} else {
			workNode.NodePath = workNode.Slug
		}

		if err := tx.Create(workNode).Error; err != nil {
			return err
		}
		result.WorkID = workNode.ID

		// 创建章节
		for i, ch := range chapters {
			chapterNode := &WorkspaceNode{
				ID:        uuid.New().String(),
				TenantID:  req.TenantID,
				ParentID:  &workNode.ID,
				Name:      ch.Title,
				Slug:      fmt.Sprintf("chapter-%d", i+1),
				Type:      "file",
				NodePath:  fmt.Sprintf("%s/chapter-%d", workNode.NodePath, i+1),
				Category:  ContentTypeChapter,
				SortOrder: i,
				CreatedBy: req.UserID,
				UpdatedBy: req.UserID,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			if err := tx.Create(chapterNode).Error; err != nil {
				return err
			}

			// 创建文件和版本
			file := &WorkspaceFile{
				ID:        uuid.New().String(),
				TenantID:  req.TenantID,
				NodeID:    chapterNode.ID,
				Category:  ContentTypeChapter,
				CreatedBy: req.UserID,
				UpdatedBy: req.UserID,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}

			version := &WorkspaceFileVersion{
				ID:        uuid.New().String(),
				FileID:    file.ID,
				TenantID:  req.TenantID,
				Content:   ch.Content,
				Summary:   fmt.Sprintf("导入章节: %s", ch.Title),
				CreatedBy: req.UserID,
				CreatedAt: time.Now().UTC(),
			}
			file.LatestVersionID = version.ID

			if err := tx.Create(file).Error; err != nil {
				return err
			}
			if err := tx.Create(version).Error; err != nil {
				return err
			}

			result.ChapterCount++
			result.TotalWords += countWords(ch.Content)
		}

		return nil
	})

	return result, err
}

// ParsedChapter 解析的章节
type ParsedChapter struct {
	Title   string
	Content string
}

// parseChapters 智能解析章节结构
func parseChapters(content string, autoDetect bool) []ParsedChapter {
	var chapters []ParsedChapter

	// 章节标题正则模式
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^第[一二三四五六七八九十百千零\d]+章[\s\.:：]*.+`),
		regexp.MustCompile(`^第[一二三四五六七八九十百千零\d]+节[\s\.:：]*.+`),
		regexp.MustCompile(`^Chapter\s*\d+[\s\.:：]*.+`),
		regexp.MustCompile(`^[【\[]第?[一二三四五六七八九十百千零\d]+[章节]?[】\]].+`),
		regexp.MustCompile(`^\d+[\.\s、]+.+`),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentChapter *ParsedChapter
	var contentBuilder strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if currentChapter != nil {
				contentBuilder.WriteString("\n")
			}
			continue
		}

		isTitle := false
		if autoDetect {
			for _, p := range patterns {
				if p.MatchString(line) {
					isTitle = true
					break
				}
			}
		}

		if isTitle {
			// 保存上一章
			if currentChapter != nil {
				currentChapter.Content = strings.TrimSpace(contentBuilder.String())
				if currentChapter.Content != "" {
					chapters = append(chapters, *currentChapter)
				}
			}
			// 开始新章节
			currentChapter = &ParsedChapter{Title: line}
			contentBuilder.Reset()
		} else if currentChapter != nil {
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		} else if len(chapters) == 0 {
			// 第一章之前的内容作为序章
			if currentChapter == nil {
				currentChapter = &ParsedChapter{Title: "序章"}
			}
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		}
	}

	// 保存最后一章
	if currentChapter != nil {
		currentChapter.Content = strings.TrimSpace(contentBuilder.String())
		if currentChapter.Content != "" {
			chapters = append(chapters, *currentChapter)
		}
	}

	return chapters
}

// countWords 统计中英文字数
func countWords(content string) int {
	if content == "" {
		return 0
	}
	// 简单统计：中文按字符数，英文按空格分词
	count := 0
	for _, r := range content {
		if r > 127 { // 非ASCII字符（主要是中文）
			count++
		}
	}
	// 英文单词
	words := strings.Fields(content)
	for _, w := range words {
		isEnglish := true
		for _, r := range w {
			if r > 127 {
				isEnglish = false
				break
			}
		}
		if isEnglish && len(w) > 0 {
			count++
		}
	}
	return count
}
