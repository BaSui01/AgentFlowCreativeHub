package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkspaceTemplateType 模板类型
type WorkspaceTemplateType string

const (
	TemplateTypeNovel      WorkspaceTemplateType = "novel"       // 小说
	TemplateTypeScript     WorkspaceTemplateType = "script"      // 剧本
	TemplateTypeArticle    WorkspaceTemplateType = "article"     // 文章
	TemplateTypeProject    WorkspaceTemplateType = "project"     // 项目
	TemplateTypeCustom     WorkspaceTemplateType = "custom"      // 自定义
)

// WorkspaceTemplate 工作空间模板
type WorkspaceTemplate struct {
	ID          string                `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID    string                `gorm:"type:varchar(36);index" json:"tenant_id"` // 空表示系统模板
	
	Name        string                `gorm:"type:varchar(100);not null" json:"name"`
	Description string                `gorm:"type:text" json:"description"`
	Type        WorkspaceTemplateType `gorm:"type:varchar(50);not null;index" json:"type"`
	
	// 模板结构（JSON）
	Structure   string                `gorm:"type:jsonb;not null" json:"structure"`
	
	// 模板预览图（可选）
	PreviewURL  string                `gorm:"type:varchar(500)" json:"preview_url,omitempty"`
	
	// 元数据
	Tags        string                `gorm:"type:text" json:"tags,omitempty"` // 逗号分隔
	IsBuiltin   bool                  `gorm:"default:false" json:"is_builtin"` // 是否内置模板
	IsPublic    bool                  `gorm:"default:false" json:"is_public"`  // 是否公开
	UseCount    int                   `gorm:"default:0" json:"use_count"`      // 使用次数
	
	CreatedBy   string                `gorm:"type:varchar(36)" json:"created_by,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   gorm.DeletedAt        `gorm:"index" json:"-"`
}

// TableName 指定表名
func (WorkspaceTemplate) TableName() string {
	return "workspace_templates"
}

// BeforeCreate GORM Hook
func (t *WorkspaceTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// TemplateNode 模板节点结构
type TemplateNode struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"` // folder, file
	Content     string         `json:"content,omitempty"`     // 文件初始内容
	Placeholder string         `json:"placeholder,omitempty"` // 占位符变量 如 {{title}}
	Children    []TemplateNode `json:"children,omitempty"`
}

// TemplateStructure 模板结构
type TemplateStructure struct {
	RootName    string         `json:"root_name"`    // 根节点名称
	Variables   []TemplateVar  `json:"variables"`    // 模板变量
	Nodes       []TemplateNode `json:"nodes"`        // 子节点
}

// TemplateVar 模板变量
type TemplateVar struct {
	Name         string `json:"name"`          // 变量名
	Label        string `json:"label"`         // 显示标签
	Type         string `json:"type"`          // text, number, date, select
	DefaultValue string `json:"default_value"` // 默认值
	Required     bool   `json:"required"`      // 是否必填
	Options      string `json:"options,omitempty"` // select类型的选项（逗号分隔）
}

// TemplateService 模板服务
type TemplateService struct {
	db               *gorm.DB
	workspaceService *Service
}

// NewTemplateService 创建模板服务
func NewTemplateService(db *gorm.DB, workspaceService *Service) *TemplateService {
	return &TemplateService{
		db:               db,
		workspaceService: workspaceService,
	}
}

// AutoMigrate 自动迁移表
func (s *TemplateService) AutoMigrate() error {
	return s.db.AutoMigrate(&WorkspaceTemplate{})
}

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	Name        string                `json:"name" binding:"required,max=100"`
	Description string                `json:"description,omitempty"`
	Type        WorkspaceTemplateType `json:"type" binding:"required"`
	Structure   TemplateStructure     `json:"structure" binding:"required"`
	PreviewURL  string                `json:"preview_url,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	IsPublic    bool                  `json:"is_public"`
}

// CreateTemplate 创建模板
func (s *TemplateService) CreateTemplate(ctx context.Context, tenantID, userID string, req *CreateTemplateRequest) (*WorkspaceTemplate, error) {
	structureJSON, err := json.Marshal(req.Structure)
	if err != nil {
		return nil, fmt.Errorf("序列化模板结构失败: %w", err)
	}

	template := &WorkspaceTemplate{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Structure:   string(structureJSON),
		PreviewURL:  req.PreviewURL,
		IsPublic:    req.IsPublic,
		CreatedBy:   userID,
	}

	if len(req.Tags) > 0 {
		tagsJSON, _ := json.Marshal(req.Tags)
		template.Tags = string(tagsJSON)
	}

	if err := s.db.WithContext(ctx).Create(template).Error; err != nil {
		return nil, fmt.Errorf("创建模板失败: %w", err)
	}

	return template, nil
}

// GetTemplate 获取模板详情
func (s *TemplateService) GetTemplate(ctx context.Context, tenantID, templateID string) (*WorkspaceTemplate, error) {
	var template WorkspaceTemplate
	query := s.db.WithContext(ctx).Where("id = ?", templateID)
	
	// 系统模板或租户自己的模板
	query = query.Where("(tenant_id = '' OR tenant_id = ? OR is_public = ?)", tenantID, true)
	
	if err := query.First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

// ListTemplatesRequest 查询模板请求
type ListTemplatesRequest struct {
	Type     *WorkspaceTemplateType `form:"type"`
	Keyword  *string                `form:"keyword"`
	Builtin  *bool                  `form:"builtin"`
	Page     int                    `form:"page" binding:"min=1"`
	PageSize int                    `form:"page_size" binding:"min=1,max=100"`
}

// ListTemplates 查询模板列表
func (s *TemplateService) ListTemplates(ctx context.Context, tenantID string, req *ListTemplatesRequest) ([]*WorkspaceTemplate, int64, error) {
	query := s.db.WithContext(ctx).
		Where("(tenant_id = '' OR tenant_id = ? OR is_public = ?)", tenantID, true)

	if req.Type != nil {
		query = query.Where("type = ?", *req.Type)
	}
	if req.Keyword != nil && *req.Keyword != "" {
		keyword := "%" + *req.Keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", keyword, keyword)
	}
	if req.Builtin != nil {
		query = query.Where("is_builtin = ?", *req.Builtin)
	}

	// 计算总数
	var total int64
	if err := query.Model(&WorkspaceTemplate{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize).Order("use_count DESC, created_at DESC")

	var templates []*WorkspaceTemplate
	if err := query.Find(&templates).Error; err != nil {
		return nil, 0, err
	}

	return templates, total, nil
}

// UpdateTemplate 更新模板
func (s *TemplateService) UpdateTemplate(ctx context.Context, tenantID, templateID string, req *CreateTemplateRequest) (*WorkspaceTemplate, error) {
	template, err := s.GetTemplate(ctx, tenantID, templateID)
	if err != nil {
		return nil, err
	}

	// 只能更新自己的模板
	if template.TenantID != tenantID {
		return nil, fmt.Errorf("无权修改此模板")
	}

	updates := make(map[string]any)
	updates["name"] = req.Name
	updates["description"] = req.Description
	updates["type"] = req.Type
	updates["preview_url"] = req.PreviewURL
	updates["is_public"] = req.IsPublic

	structureJSON, err := json.Marshal(req.Structure)
	if err != nil {
		return nil, fmt.Errorf("序列化模板结构失败: %w", err)
	}
	updates["structure"] = string(structureJSON)

	if len(req.Tags) > 0 {
		tagsJSON, _ := json.Marshal(req.Tags)
		updates["tags"] = string(tagsJSON)
	}

	if err := s.db.WithContext(ctx).Model(template).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新模板失败: %w", err)
	}

	return s.GetTemplate(ctx, tenantID, templateID)
}

// DeleteTemplate 删除模板
func (s *TemplateService) DeleteTemplate(ctx context.Context, tenantID, templateID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND is_builtin = ?", templateID, tenantID, false).
		Delete(&WorkspaceTemplate{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("模板不存在或无权删除")
	}

	return nil
}

// ApplyTemplateRequest 应用模板请求
type ApplyTemplateRequest struct {
	TemplateID  string            `json:"template_id" binding:"required"`
	Variables   map[string]string `json:"variables,omitempty"` // 模板变量值
	ParentID    string            `json:"parent_id,omitempty"` // 创建在哪个目录下
}

// ApplyTemplate 应用模板创建工作空间结构
func (s *TemplateService) ApplyTemplate(ctx context.Context, tenantID, userID string, req *ApplyTemplateRequest) (*WorkspaceNode, error) {
	// 获取模板
	template, err := s.GetTemplate(ctx, tenantID, req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("模板不存在: %w", err)
	}

	// 解析模板结构
	var structure TemplateStructure
	if err := json.Unmarshal([]byte(template.Structure), &structure); err != nil {
		return nil, fmt.Errorf("解析模板结构失败: %w", err)
	}

	// 替换变量
	rootName := replaceVariables(structure.RootName, req.Variables)

	// 创建根目录
	var parentIDPtr *string
	if req.ParentID != "" {
		parentIDPtr = &req.ParentID
	}
	rootNode, err := s.workspaceService.CreateFolder(ctx, &CreateFolderRequest{
		TenantID: tenantID,
		ParentID: parentIDPtr,
		Name:     rootName,
	})
	if err != nil {
		return nil, fmt.Errorf("创建根目录失败: %w", err)
	}

	// 递归创建子节点
	if err := s.createNodesRecursive(ctx, tenantID, rootNode.ID, structure.Nodes, req.Variables); err != nil {
		// 回滚：删除已创建的根目录
		_ = s.workspaceService.DeleteNode(ctx, tenantID, rootNode.ID)
		return nil, fmt.Errorf("创建模板结构失败: %w", err)
	}

	// 更新使用次数
	_ = s.db.WithContext(ctx).Model(&WorkspaceTemplate{}).
		Where("id = ?", req.TemplateID).
		Update("use_count", gorm.Expr("use_count + 1")).Error

	return rootNode, nil
}

// createNodesRecursive 递归创建节点
func (s *TemplateService) createNodesRecursive(ctx context.Context, tenantID, parentID string, nodes []TemplateNode, variables map[string]string) error {
	for _, node := range nodes {
		name := replaceVariables(node.Name, variables)
		parentIDPtr := &parentID
		
		if node.Type == "folder" {
			// 创建文件夹
			folder, err := s.workspaceService.CreateFolder(ctx, &CreateFolderRequest{
				TenantID: tenantID,
				ParentID: parentIDPtr,
				Name:     name,
			})
			if err != nil {
				return err
			}

			// 递归创建子节点
			if len(node.Children) > 0 {
				if err := s.createNodesRecursive(ctx, tenantID, folder.ID, node.Children, variables); err != nil {
					return err
				}
			}
		} else {
			// 创建文件
			content := replaceVariables(node.Content, variables)
			_, err := s.workspaceService.CreateFile(ctx, &CreateFileRequest{
				TenantID: tenantID,
				ParentID: parentIDPtr,
				Name:     name,
				Content:  content,
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// replaceVariables 替换变量
func replaceVariables(text string, variables map[string]string) string {
	if variables == nil {
		return text
	}
	for key, value := range variables {
		placeholder := "{{" + key + "}}"
		text = replaceAll(text, placeholder, value)
	}
	return text
}

// replaceAll 替换所有匹配
func replaceAll(s, old, new string) string {
	result := s
	for {
		idx := findIndex(result, old)
		if idx == -1 {
			break
		}
		result = result[:idx] + new + result[idx+len(old):]
	}
	return result
}

// findIndex 查找子串位置
func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// InitBuiltinTemplates 初始化内置模板
func (s *TemplateService) InitBuiltinTemplates(ctx context.Context) error {
	builtinTemplates := []WorkspaceTemplate{
		{
			ID:          "builtin-novel-basic",
			TenantID:    "",
			Name:        "小说基础模板",
			Description: "适用于网络小说创作的基础目录结构",
			Type:        TemplateTypeNovel,
			Structure:   getNovelBasicStructure(),
			IsBuiltin:   true,
			IsPublic:    true,
		},
		{
			ID:          "builtin-novel-advanced",
			TenantID:    "",
			Name:        "小说进阶模板",
			Description: "包含世界观设定、角色档案的完整小说模板",
			Type:        TemplateTypeNovel,
			Structure:   getNovelAdvancedStructure(),
			IsBuiltin:   true,
			IsPublic:    true,
		},
		{
			ID:          "builtin-script",
			TenantID:    "",
			Name:        "剧本模板",
			Description: "适用于影视剧本创作的目录结构",
			Type:        TemplateTypeScript,
			Structure:   getScriptStructure(),
			IsBuiltin:   true,
			IsPublic:    true,
		},
		{
			ID:          "builtin-article-series",
			TenantID:    "",
			Name:        "系列文章模板",
			Description: "适用于系列文章或教程的目录结构",
			Type:        TemplateTypeArticle,
			Structure:   getArticleSeriesStructure(),
			IsBuiltin:   true,
			IsPublic:    true,
		},
	}

	for _, template := range builtinTemplates {
		// 使用 UPSERT 逻辑
		var existing WorkspaceTemplate
		if err := s.db.WithContext(ctx).Where("id = ?", template.ID).First(&existing).Error; err == gorm.ErrRecordNotFound {
			if err := s.db.WithContext(ctx).Create(&template).Error; err != nil {
				return fmt.Errorf("创建内置模板失败: %w", err)
			}
		}
	}

	return nil
}

// 内置模板结构定义
func getNovelBasicStructure() string {
	structure := TemplateStructure{
		RootName: "{{title}}",
		Variables: []TemplateVar{
			{Name: "title", Label: "作品名称", Type: "text", Required: true},
		},
		Nodes: []TemplateNode{
			{Name: "第一卷", Type: "folder", Children: []TemplateNode{
				{Name: "第一章 开篇.md", Type: "file", Content: "# 第一章 开篇\n\n在此开始你的故事..."},
				{Name: "第二章.md", Type: "file", Content: "# 第二章\n\n"},
				{Name: "第三章.md", Type: "file", Content: "# 第三章\n\n"},
			}},
			{Name: "大纲.md", Type: "file", Content: "# {{title}} 大纲\n\n## 故事梗概\n\n## 主要角色\n\n## 章节规划\n"},
		},
	}
	data, _ := json.Marshal(structure)
	return string(data)
}

func getNovelAdvancedStructure() string {
	structure := TemplateStructure{
		RootName: "{{title}}",
		Variables: []TemplateVar{
			{Name: "title", Label: "作品名称", Type: "text", Required: true},
			{Name: "genre", Label: "类型", Type: "select", Options: "玄幻,都市,科幻,历史,武侠"},
		},
		Nodes: []TemplateNode{
			{Name: "设定", Type: "folder", Children: []TemplateNode{
				{Name: "世界观.md", Type: "file", Content: "# 世界观设定\n\n## 世界背景\n\n## 力量体系\n\n## 势力分布\n"},
				{Name: "角色档案", Type: "folder", Children: []TemplateNode{
					{Name: "主角.md", Type: "file", Content: "# 主角档案\n\n## 基本信息\n\n## 性格特点\n\n## 成长轨迹\n"},
					{Name: "配角.md", Type: "file", Content: "# 配角档案\n\n"},
				}},
				{Name: "地图.md", Type: "file", Content: "# 地图设定\n\n"},
			}},
			{Name: "正文", Type: "folder", Children: []TemplateNode{
				{Name: "第一卷", Type: "folder", Children: []TemplateNode{
					{Name: "第001章 开篇.md", Type: "file", Content: "# 第001章 开篇\n\n"},
				}},
			}},
			{Name: "大纲.md", Type: "file", Content: "# {{title}} 大纲\n\n类型：{{genre}}\n\n## 故事梗概\n\n## 核心卖点\n\n## 章节规划\n"},
			{Name: "灵感收集.md", Type: "file", Content: "# 灵感收集\n\n记录创作过程中的灵感和想法...\n"},
		},
	}
	data, _ := json.Marshal(structure)
	return string(data)
}

func getScriptStructure() string {
	structure := TemplateStructure{
		RootName: "《{{title}}》",
		Variables: []TemplateVar{
			{Name: "title", Label: "剧本名称", Type: "text", Required: true},
			{Name: "episodes", Label: "集数", Type: "number", DefaultValue: "24"},
		},
		Nodes: []TemplateNode{
			{Name: "策划", Type: "folder", Children: []TemplateNode{
				{Name: "故事大纲.md", Type: "file", Content: "# 《{{title}}》故事大纲\n\n## 故事简介\n\n## 故事主线\n\n## 人物关系\n"},
				{Name: "角色设定.md", Type: "file", Content: "# 角色设定\n\n## 主要角色\n\n## 次要角色\n"},
				{Name: "分集大纲.md", Type: "file", Content: "# 分集大纲\n\n## 第1集\n\n## 第2集\n"},
			}},
			{Name: "剧本", Type: "folder", Children: []TemplateNode{
				{Name: "第01集.md", Type: "file", Content: "# 《{{title}}》第01集\n\n## 场景1\n\n"},
			}},
			{Name: "素材", Type: "folder", Children: []TemplateNode{
				{Name: "参考资料.md", Type: "file", Content: "# 参考资料\n\n"},
			}},
		},
	}
	data, _ := json.Marshal(structure)
	return string(data)
}

func getArticleSeriesStructure() string {
	structure := TemplateStructure{
		RootName: "{{title}}",
		Variables: []TemplateVar{
			{Name: "title", Label: "系列名称", Type: "text", Required: true},
		},
		Nodes: []TemplateNode{
			{Name: "文章", Type: "folder", Children: []TemplateNode{
				{Name: "01-开篇.md", Type: "file", Content: "# {{title}} - 开篇\n\n## 前言\n\n## 本文概述\n"},
			}},
			{Name: "素材", Type: "folder", Children: []TemplateNode{
				{Name: "图片", Type: "folder"},
				{Name: "参考链接.md", Type: "file", Content: "# 参考链接\n\n"},
			}},
			{Name: "README.md", Type: "file", Content: "# {{title}}\n\n## 系列介绍\n\n## 目录\n\n## 作者\n"},
		},
	}
	data, _ := json.Marshal(structure)
	return string(data)
}
