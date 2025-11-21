package template

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"backend/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TemplateService Prompt 模板管理服务
type TemplateService struct {
	db *gorm.DB
}

// NewTemplateService 创建 TemplateService 实例
func NewTemplateService(db *gorm.DB) *TemplateService {
	return &TemplateService{db: db}
}

// ListTemplatesRequest 查询模板列表请求
type ListTemplatesRequest struct {
	TenantID   string
	Category   string // writing, reviewing, formatting, translation, analysis
	Visibility string // personal, tenant, public
	CreatedBy  string
	Page       int
	PageSize   int
}

// ListTemplatesResponse 查询模板列表响应
type ListTemplatesResponse struct {
	Templates  []*PromptTemplate `json:"templates"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// ListTemplates 查询模板列表（支持过滤）
func (s *TemplateService) ListTemplates(ctx context.Context, req *ListTemplatesRequest) (*ListTemplatesResponse, error) {
	// 构建查询
	query := s.db.WithContext(ctx).
		Model(&PromptTemplate{}).
		Scopes(common.NotDeleted())

	// 租户过滤
	if req.TenantID != "" {
		query = query.Where("tenant_id = ?", req.TenantID)
	}

	// 分类过滤
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}

	// 可见性过滤
	if req.Visibility != "" {
		query = query.Where("visibility = ?", req.Visibility)
	}

	// 创建者过滤
	if req.CreatedBy != "" {
		query = query.Where("created_by = ?", req.CreatedBy)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("统计模板数量失败: %w", err)
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
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 查询列表
	var templates []*PromptTemplate
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("查询模板列表失败: %w", err)
	}

	// 计算总页数
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &ListTemplatesResponse{
		Templates:  templates,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetTemplate 查询单个模板
func (s *TemplateService) GetTemplate(ctx context.Context, tenantID, templateID string) (*PromptTemplate, error) {
	var tmpl PromptTemplate
	if err := s.db.WithContext(ctx).
		Scopes(common.NotDeleted()).
		Where("id = ? AND (tenant_id = ? OR visibility = ?)", templateID, tenantID, "public").
		First(&tmpl).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("模板不存在")
		}
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}
	return &tmpl, nil
}

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	TenantID    string
	Name        string
	Category    string
	Description string
	Visibility  string
	CreatedBy   string
}

// CreateTemplate 创建模板
func (s *TemplateService) CreateTemplate(ctx context.Context, req *CreateTemplateRequest) (*PromptTemplate, error) {
	// 验证必填字段
	if req.Name == "" {
		return nil, fmt.Errorf("模板名称不能为空")
	}
	if req.Category == "" {
		return nil, fmt.Errorf("模板分类不能为空")
	}
	if req.Visibility == "" {
		req.Visibility = "personal"
	}

	// 创建模板
	tmpl := &PromptTemplate{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		Name:        req.Name,
		Category:    req.Category,
		Description: req.Description,
		Visibility:  req.Visibility,
		UsageCount:  0,
		CreatedBy:   req.CreatedBy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(tmpl).Error; err != nil {
		return nil, fmt.Errorf("创建模板失败: %w", err)
	}

	return tmpl, nil
}

// UpdateTemplateRequest 更新模板请求
type UpdateTemplateRequest struct {
	Name        *string
	Description *string
	Visibility  *string
}

// UpdateTemplate 更新模板
func (s *TemplateService) UpdateTemplate(ctx context.Context, tenantID, templateID string, req *UpdateTemplateRequest) (*PromptTemplate, error) {
	// 查询模板
	tmpl, err := s.GetTemplate(ctx, tenantID, templateID)
	if err != nil {
		return nil, err
	}

	// 权限检查（只能更新自己租户的模板）
	if tmpl.TenantID != tenantID {
		return nil, fmt.Errorf("无权限修改此模板")
	}

	// 更新字段
	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	updates["updated_at"] = time.Now().UTC()

	// 执行更新
	if err := s.db.WithContext(ctx).
		Model(tmpl).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新模板失败: %w", err)
	}

	// 重新查询返回最新数据
	return s.GetTemplate(ctx, tenantID, templateID)
}

// DeleteTemplate 软删除模板
func (s *TemplateService) DeleteTemplate(ctx context.Context, tenantID, templateID, operatorID string) error {
	// 查询模板
	tmpl, err := s.GetTemplate(ctx, tenantID, templateID)
	if err != nil {
		return err
	}

	// 权限检查
	if tmpl.TenantID != tenantID {
		return fmt.Errorf("无权限删除此模板")
	}

	// 软删除
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).
		Model(tmpl).
		Updates(map[string]any{
			"deleted_at": now,
			"deleted_by": operatorID,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("删除模板失败: %w", err)
	}

	return nil
}

// CreateVersionRequest 创建模板版本请求
type CreateVersionRequest struct {
	TemplateID  string
	Version     string
	Content     string
	Variables   map[string]any
	ChangeLog   string
	CreatedBy   string
}

// CreateVersion 创建模板版本
func (s *TemplateService) CreateVersion(ctx context.Context, tenantID string, req *CreateVersionRequest) (*PromptTemplateVersion, error) {
	// 验证必填字段
	if req.Content == "" {
		return nil, fmt.Errorf("模板内容不能为空")
	}
	if req.Version == "" {
		return nil, fmt.Errorf("版本号不能为空")
	}

	// 验证模板是否存在
	tmpl, err := s.GetTemplate(ctx, tenantID, req.TemplateID)
	if err != nil {
		return nil, err
	}

	// 权限检查
	if tmpl.TenantID != tenantID {
		return nil, fmt.Errorf("无权限修改此模板")
	}

	// 检查版本号是否已存在
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&PromptTemplateVersion{}).
		Where("template_id = ? AND version = ?", req.TemplateID, req.Version).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("检查版本号失败: %w", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("版本号已存在")
	}

	// 创建版本
	version := &PromptTemplateVersion{
		ID:         uuid.New().String(),
		TemplateID: req.TemplateID,
		Version:    req.Version,
		Content:    req.Content,
		Variables:  req.Variables,
		Changelog:  req.ChangeLog,
		CreatedBy:  req.CreatedBy,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(version).Error; err != nil {
		return nil, fmt.Errorf("创建模板版本失败: %w", err)
	}

	// 更新模板的当前版本 ID
	if err := s.db.WithContext(ctx).
		Model(tmpl).
		Update("current_version_id", version.ID).Error; err != nil {
		return nil, fmt.Errorf("更新模板当前版本失败: %w", err)
	}

	return version, nil
}

// GetLatestVersion 获取最新版本
func (s *TemplateService) GetLatestVersion(ctx context.Context, tenantID, templateID string) (*PromptTemplateVersion, error) {
	// 验证模板是否存在
	tmpl, err := s.GetTemplate(ctx, tenantID, templateID)
	if err != nil {
		return nil, err
	}

	// 如果有当前版本 ID，直接查询
	if tmpl.CurrentVersionID != nil && *tmpl.CurrentVersionID != "" {
		var version PromptTemplateVersion
		if err := s.db.WithContext(ctx).
			Where("id = ?", *tmpl.CurrentVersionID).
			First(&version).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("查询当前版本失败: %w", err)
			}
		} else {
			return &version, nil
		}
	}

	// 否则查询最新版本
	var version PromptTemplateVersion
	if err := s.db.WithContext(ctx).
		Where("template_id = ?", templateID).
		Order("created_at DESC").
		First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("模板版本不存在")
		}
		return nil, fmt.Errorf("查询最新版本失败: %w", err)
	}

	return &version, nil
}

// RenderTemplateRequest 渲染模板请求
type RenderTemplateRequest struct {
	TemplateID string
	Variables  map[string]any
}

// RenderTemplate 渲染模板（变量注入）
func (s *TemplateService) RenderTemplate(ctx context.Context, tenantID string, req *RenderTemplateRequest) (string, error) {
	// 获取最新版本
	version, err := s.GetLatestVersion(ctx, tenantID, req.TemplateID)
	if err != nil {
		return "", err
	}

	// 解析模板
	tmpl, err := template.New("prompt").Parse(version.Content)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	// 渲染模板
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, req.Variables); err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}

	// 增加使用计数
	s.incrementUsageCount(ctx, req.TemplateID)

	return buf.String(), nil
}

// incrementUsageCount 增加模板使用计数（异步，不阻塞）
func (s *TemplateService) incrementUsageCount(ctx context.Context, templateID string) {
	go func() {
		_ = s.db.Model(&PromptTemplate{}).
			Where("id = ?", templateID).
			UpdateColumn("usage_count", gorm.Expr("usage_count + 1"))
	}()
}
