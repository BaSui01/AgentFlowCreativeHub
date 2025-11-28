package template

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// VersionManager 模板版本管理器
type VersionManager struct {
	store    VersionStore
	maxVer   int // 每个模板最大保留版本数
	mu       sync.RWMutex
}

// VersionStore 版本存储接口
type VersionStore interface {
	SaveVersion(ctx context.Context, version *TemplateVersion) error
	GetVersion(ctx context.Context, templateID string, version int) (*TemplateVersion, error)
	ListVersions(ctx context.Context, templateID string) ([]*TemplateVersion, error)
	DeleteVersion(ctx context.Context, templateID string, version int) error
	GetLatestVersion(ctx context.Context, templateID string) (*TemplateVersion, error)
}

// TemplateVersion 模板版本
type TemplateVersion struct {
	ID          string         `json:"id"`
	TemplateID  string         `json:"template_id"`
	Version     int            `json:"version"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Content     map[string]any `json:"content"`
	ContentHash string         `json:"content_hash"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	Comment     string         `json:"comment"`
	Tags        []string       `json:"tags,omitempty"`
	IsPublished bool           `json:"is_published"`
}

// Template 模板定义
type Template struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Content     map[string]any `json:"content"`
	CurrentVer  int            `json:"current_version"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// NewVersionManager 创建版本管理器
func NewVersionManager(store VersionStore, maxVersions int) *VersionManager {
	if maxVersions <= 0 {
		maxVersions = 50 // 默认保留 50 个版本
	}
	return &VersionManager{
		store:  store,
		maxVer: maxVersions,
	}
}

// CreateVersion 创建新版本
func (m *VersionManager) CreateVersion(ctx context.Context, template *Template, userID, comment string) (*TemplateVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取最新版本号
	latest, _ := m.store.GetLatestVersion(ctx, template.ID)
	nextVersion := 1
	if latest != nil {
		nextVersion = latest.Version + 1
	}

	// 计算内容哈希
	contentHash := m.hashContent(template.Content)

	// 检查是否有实际变更
	if latest != nil && latest.ContentHash == contentHash {
		return nil, fmt.Errorf("no changes detected")
	}

	version := &TemplateVersion{
		ID:          fmt.Sprintf("%s_v%d", template.ID, nextVersion),
		TemplateID:  template.ID,
		Version:     nextVersion,
		Name:        template.Name,
		Description: template.Description,
		Content:     template.Content,
		ContentHash: contentHash,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		Comment:     comment,
	}

	if err := m.store.SaveVersion(ctx, version); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	// 清理旧版本
	go m.cleanOldVersions(context.Background(), template.ID)

	return version, nil
}

// GetVersion 获取指定版本
func (m *VersionManager) GetVersion(ctx context.Context, templateID string, version int) (*TemplateVersion, error) {
	return m.store.GetVersion(ctx, templateID, version)
}

// GetLatestVersion 获取最新版本
func (m *VersionManager) GetLatestVersion(ctx context.Context, templateID string) (*TemplateVersion, error) {
	return m.store.GetLatestVersion(ctx, templateID)
}

// ListVersions 列出所有版本
func (m *VersionManager) ListVersions(ctx context.Context, templateID string) ([]*TemplateVersion, error) {
	return m.store.ListVersions(ctx, templateID)
}

// RollbackToVersion 回滚到指定版本
func (m *VersionManager) RollbackToVersion(ctx context.Context, templateID string, version int, userID string) (*TemplateVersion, error) {
	// 获取目标版本
	targetVersion, err := m.store.GetVersion(ctx, templateID, version)
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}

	// 创建新版本（内容与目标版本相同）
	template := &Template{
		ID:          templateID,
		Name:        targetVersion.Name,
		Description: targetVersion.Description,
		Content:     targetVersion.Content,
	}

	comment := fmt.Sprintf("Rollback to version %d", version)
	return m.CreateVersion(ctx, template, userID, comment)
}

// CompareVersions 比较两个版本
func (m *VersionManager) CompareVersions(ctx context.Context, templateID string, v1, v2 int) (*VersionDiff, error) {
	ver1, err := m.store.GetVersion(ctx, templateID, v1)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", v1, err)
	}

	ver2, err := m.store.GetVersion(ctx, templateID, v2)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", v2, err)
	}

	diff := &VersionDiff{
		TemplateID: templateID,
		FromVer:    v1,
		ToVer:      v2,
		Changes:    make([]FieldChange, 0),
	}

	// 比较字段
	m.compareMap("", ver1.Content, ver2.Content, &diff.Changes)

	// 元信息变更
	if ver1.Name != ver2.Name {
		diff.Changes = append(diff.Changes, FieldChange{
			Field:    "_name",
			OldValue: ver1.Name,
			NewValue: ver2.Name,
			Type:     ChangeModified,
		})
	}
	if ver1.Description != ver2.Description {
		diff.Changes = append(diff.Changes, FieldChange{
			Field:    "_description",
			OldValue: ver1.Description,
			NewValue: ver2.Description,
			Type:     ChangeModified,
		})
	}

	return diff, nil
}

// VersionDiff 版本差异
type VersionDiff struct {
	TemplateID string        `json:"template_id"`
	FromVer    int           `json:"from_version"`
	ToVer      int           `json:"to_version"`
	Changes    []FieldChange `json:"changes"`
}

// FieldChange 字段变更
type FieldChange struct {
	Field    string     `json:"field"`
	OldValue any        `json:"old_value,omitempty"`
	NewValue any        `json:"new_value,omitempty"`
	Type     ChangeType `json:"type"`
}

// ChangeType 变更类型
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeRemoved  ChangeType = "removed"
	ChangeModified ChangeType = "modified"
)

func (m *VersionManager) compareMap(prefix string, old, new map[string]any, changes *[]FieldChange) {
	// 检查删除和修改
	for k, oldVal := range old {
		field := k
		if prefix != "" {
			field = prefix + "." + k
		}

		newVal, exists := new[k]
		if !exists {
			*changes = append(*changes, FieldChange{
				Field:    field,
				OldValue: oldVal,
				Type:     ChangeRemoved,
			})
			continue
		}

		// 递归比较嵌套 map
		oldMap, oldIsMap := oldVal.(map[string]any)
		newMap, newIsMap := newVal.(map[string]any)
		if oldIsMap && newIsMap {
			m.compareMap(field, oldMap, newMap, changes)
		} else if !m.isEqual(oldVal, newVal) {
			*changes = append(*changes, FieldChange{
				Field:    field,
				OldValue: oldVal,
				NewValue: newVal,
				Type:     ChangeModified,
			})
		}
	}

	// 检查新增
	for k, newVal := range new {
		field := k
		if prefix != "" {
			field = prefix + "." + k
		}

		if _, exists := old[k]; !exists {
			*changes = append(*changes, FieldChange{
				Field:    field,
				NewValue: newVal,
				Type:     ChangeAdded,
			})
		}
	}
}

func (m *VersionManager) isEqual(a, b any) bool {
	jsonA, _ := json.Marshal(a)
	jsonB, _ := json.Marshal(b)
	return string(jsonA) == string(jsonB)
}

func (m *VersionManager) hashContent(content map[string]any) string {
	data, _ := json.Marshal(content)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (m *VersionManager) cleanOldVersions(ctx context.Context, templateID string) {
	versions, err := m.store.ListVersions(ctx, templateID)
	if err != nil || len(versions) <= m.maxVer {
		return
	}

	// 删除最旧的版本（保留已发布的版本）
	for i := 0; i < len(versions)-m.maxVer; i++ {
		v := versions[i]
		if !v.IsPublished {
			_ = m.store.DeleteVersion(ctx, templateID, v.Version)
		}
	}
}

// PublishVersion 发布版本
func (m *VersionManager) PublishVersion(ctx context.Context, templateID string, version int) error {
	v, err := m.store.GetVersion(ctx, templateID, version)
	if err != nil {
		return err
	}

	v.IsPublished = true
	return m.store.SaveVersion(ctx, v)
}

// TagVersion 给版本添加标签
func (m *VersionManager) TagVersion(ctx context.Context, templateID string, version int, tags []string) error {
	v, err := m.store.GetVersion(ctx, templateID, version)
	if err != nil {
		return err
	}

	v.Tags = tags
	return m.store.SaveVersion(ctx, v)
}

// ExportVersion 导出版本为 JSON
func (m *VersionManager) ExportVersion(ctx context.Context, templateID string, version int) ([]byte, error) {
	v, err := m.store.GetVersion(ctx, templateID, version)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(v, "", "  ")
}

// ImportVersion 从 JSON 导入版本
func (m *VersionManager) ImportVersion(ctx context.Context, data []byte, userID string) (*TemplateVersion, error) {
	var imported TemplateVersion
	if err := json.Unmarshal(data, &imported); err != nil {
		return nil, fmt.Errorf("invalid version data: %w", err)
	}

	// 作为新版本导入
	template := &Template{
		ID:          imported.TemplateID,
		Name:        imported.Name,
		Description: imported.Description,
		Content:     imported.Content,
	}

	return m.CreateVersion(ctx, template, userID, "Imported version")
}
