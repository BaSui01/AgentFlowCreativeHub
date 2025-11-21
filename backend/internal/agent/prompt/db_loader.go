package prompt

import (
	"context"
	"errors"
	"fmt"

	"backend/internal/template"

	"gorm.io/gorm"
)

// DBLoader 基于数据库的模板加载器
type DBLoader struct {
	db *gorm.DB
}

// NewDBLoader 创建数据库模板加载器
func NewDBLoader(db *gorm.DB) *DBLoader {
	return &DBLoader{db: db}
}

// Load 加载指定 ID 的模板
func (l *DBLoader) Load(ctx context.Context, id string) (*Template, error) {
	var tmplModel template.PromptTemplate
	// 1. 查询模板基础信息
	if err := l.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&tmplModel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("template not found: %s", id)
		}
		return nil, fmt.Errorf("failed to load template: %w", err)
	}

	// 2. 查询当前版本内容
	if tmplModel.CurrentVersionID == nil {
		return nil, fmt.Errorf("template has no active version: %s", id)
	}

	var verModel template.PromptTemplateVersion
	if err := l.db.WithContext(ctx).
		Where("id = ?", tmplModel.CurrentVersionID).
		First(&verModel).Error; err != nil {
		return nil, fmt.Errorf("failed to load template version: %w", err)
	}

	// 3. 转换变量列表
	var variables []string
	if verModel.Variables != nil {
		for k := range verModel.Variables {
			variables = append(variables, k)
		}
	}

	// 4. 构造返回对象
	return &Template{
		ID:          tmplModel.ID,
		Name:        tmplModel.Name,
		Description: tmplModel.Description,
		Content:     verModel.Content,
		Variables:   variables,
		Version:     verModel.Version,
		Metadata: map[string]string{
			"category":   tmplModel.Category,
			"visibility": tmplModel.Visibility,
			"tenant_id":  tmplModel.TenantID,
		},
	}, nil
}

// List 列出所有可用模板
func (l *DBLoader) List(ctx context.Context) ([]*Template, error) {
	// 仅列出有当前版本的模板
	var tmplModels []template.PromptTemplate
	if err := l.db.WithContext(ctx).
		Where("deleted_at IS NULL AND current_version_id IS NOT NULL").
		Find(&tmplModels).Error; err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	// 批量拉取所有当前版本，避免 N+1 查询
	versionIDs := make([]string, 0, len(tmplModels))
	for _, tmpl := range tmplModels {
		if tmpl.CurrentVersionID != nil {
			versionIDs = append(versionIDs, *tmpl.CurrentVersionID)
		}
	}

	versionMap := make(map[string]template.PromptTemplateVersion, len(versionIDs))
	if len(versionIDs) > 0 {
		var verModels []template.PromptTemplateVersion
		if err := l.db.WithContext(ctx).
			Where("id IN ?", versionIDs).
			Find(&verModels).Error; err != nil {
			return nil, fmt.Errorf("failed to preload template versions: %w", err)
		}
		for _, v := range verModels {
			versionMap[v.ID] = v
		}
	}

	result := make([]*Template, 0, len(tmplModels))
	for _, tmplModel := range tmplModels {
		verModel, ok := versionMap[getStringValue(tmplModel.CurrentVersionID)]
		if !ok {
			// 如果加载版本失败，跳过该模板
			continue
		}

		var variables []string
		if verModel.Variables != nil {
			for k := range verModel.Variables {
				variables = append(variables, k)
			}
		}

		result = append(result, &Template{
			ID:          tmplModel.ID,
			Name:        tmplModel.Name,
			Description: tmplModel.Description,
			Content:     verModel.Content,
			Variables:   variables,
			Version:     verModel.Version,
			Metadata: map[string]string{
				"category":   tmplModel.Category,
				"visibility": tmplModel.Visibility,
				"tenant_id":  tmplModel.TenantID,
			},
		})
	}

	return result, nil
}

// getStringValue 兼容 nil 指针避免重复判断
func getStringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
