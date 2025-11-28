package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatYAML ExportFormat = "yaml"
)

// WorkflowExportData 工作流导出数据
type WorkflowExportData struct {
	Version    string             `json:"version" yaml:"version"`
	ExportedAt string             `json:"exportedAt" yaml:"exportedAt"`
	Workflow   WorkflowExportItem `json:"workflow" yaml:"workflow"`
}

// WorkflowExportItem 导出的工作流项
type WorkflowExportItem struct {
	Name        string             `json:"name" yaml:"name"`
	Description string             `json:"description" yaml:"description"`
	Version     string             `json:"version" yaml:"version"`
	Visibility  string             `json:"visibility" yaml:"visibility"`
	Definition  WorkflowDefinition `json:"definition" yaml:"definition"`
	CreatedBy   string             `json:"createdBy,omitempty" yaml:"createdBy,omitempty"`
}

// BatchExportData 批量导出数据
type BatchExportData struct {
	Version    string               `json:"version" yaml:"version"`
	ExportedAt string               `json:"exportedAt" yaml:"exportedAt"`
	Count      int                  `json:"count" yaml:"count"`
	Workflows  []WorkflowExportItem `json:"workflows" yaml:"workflows"`
}

// WorkflowIO 工作流导入导出服务
type WorkflowIO struct {
	db *gorm.DB
}

// NewWorkflowIO 创建工作流 IO 服务
func NewWorkflowIO(db *gorm.DB) *WorkflowIO {
	return &WorkflowIO{
		db: db,
	}
}

// ExportRequest 导出请求
type ExportRequest struct {
	WorkflowID string
	TenantID   string
	Format     ExportFormat
}

// ExportResult 导出结果
type ExportResult struct {
	Data        []byte
	Filename    string
	ContentType string
}

// Export 导出单个工作流
func (io *WorkflowIO) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	var wf Workflow
	if err := io.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", req.WorkflowID, req.TenantID).
		First(&wf).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("工作流不存在")
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	exportData := WorkflowExportData{
		Version:    "1.0",
		ExportedAt: time.Now().Format(time.RFC3339),
		Workflow: WorkflowExportItem{
			Name:        wf.Name,
			Description: wf.Description,
			Version:     wf.Version,
			Visibility:  wf.Visibility,
			Definition:  wf.Definition,
			CreatedBy:   wf.CreatedBy,
		},
	}

	return io.marshal(exportData, req.Format, wf.Name)
}

// BatchExportRequest 批量导出请求
type BatchExportRequest struct {
	WorkflowIDs []string
	TenantID    string
	Format      ExportFormat
}

// BatchExport 批量导出工作流
func (io *WorkflowIO) BatchExport(ctx context.Context, req *BatchExportRequest) (*ExportResult, error) {
	var workflows []Workflow
	if err := io.db.WithContext(ctx).
		Where("id IN ? AND tenant_id = ?", req.WorkflowIDs, req.TenantID).
		Find(&workflows).Error; err != nil {
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	if len(workflows) == 0 {
		return nil, fmt.Errorf("未找到工作流")
	}

	items := make([]WorkflowExportItem, len(workflows))
	for i, wf := range workflows {
		items[i] = WorkflowExportItem{
			Name:        wf.Name,
			Description: wf.Description,
			Version:     wf.Version,
			Visibility:  wf.Visibility,
			Definition:  wf.Definition,
			CreatedBy:   wf.CreatedBy,
		}
	}

	exportData := BatchExportData{
		Version:    "1.0",
		ExportedAt: time.Now().Format(time.RFC3339),
		Count:      len(items),
		Workflows:  items,
	}

	return io.marshal(exportData, req.Format, "workflows_batch")
}

// marshal 序列化数据
func (io *WorkflowIO) marshal(data interface{}, format ExportFormat, name string) (*ExportResult, error) {
	var (
		bytes       []byte
		err         error
		ext         string
		contentType string
	)

	switch format {
	case FormatYAML:
		bytes, err = yaml.Marshal(data)
		ext = "yaml"
		contentType = "application/x-yaml"
	default:
		bytes, err = json.MarshalIndent(data, "", "  ")
		ext = "json"
		contentType = "application/json"
	}

	if err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	return &ExportResult{
		Data:        bytes,
		Filename:    fmt.Sprintf("%s_%s.%s", name, timestamp, ext),
		ContentType: contentType,
	}, nil
}

// ImportRequest 导入请求
type ImportRequest struct {
	TenantID   string
	UserID     string
	Data       []byte
	Format     ExportFormat
	Overwrite  bool   // 是否覆盖同名工作流
	NamePrefix string // 名称前缀（用于区分导入的工作流）
}

// ImportResult 导入结果
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
	IDs      []string `json:"ids"`
}

// Import 导入工作流
func (io *WorkflowIO) Import(ctx context.Context, req *ImportRequest) (*ImportResult, error) {
	// 尝试解析为批量导出格式
	var batchData BatchExportData
	var singleData WorkflowExportData
	var items []WorkflowExportItem

	// 根据格式解析
	var unmarshal func([]byte, interface{}) error
	if req.Format == FormatYAML {
		unmarshal = yaml.Unmarshal
	} else {
		unmarshal = json.Unmarshal
	}

	// 尝试批量格式
	if err := unmarshal(req.Data, &batchData); err == nil && len(batchData.Workflows) > 0 {
		items = batchData.Workflows
	} else if err := unmarshal(req.Data, &singleData); err == nil && singleData.Workflow.Name != "" {
		items = []WorkflowExportItem{singleData.Workflow}
	} else {
		return nil, fmt.Errorf("无法解析导入数据")
	}

	result := &ImportResult{
		IDs: make([]string, 0),
	}

	for _, item := range items {
		// 简单验证工作流定义
		if len(item.Definition.Nodes) == 0 && len(item.Definition.Steps) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: 工作流定义为空", item.Name))
			result.Skipped++
			continue
		}

		// 检查同名工作流
		var existing Workflow
		err := io.db.WithContext(ctx).
			Where("tenant_id = ? AND name = ?", req.TenantID, item.Name).
			First(&existing).Error

		if err == nil && !req.Overwrite {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: 已存在同名工作流", item.Name))
			result.Skipped++
			continue
		}

		// 处理名称
		name := item.Name
		if req.NamePrefix != "" {
			name = req.NamePrefix + "_" + name
		}

		// 创建或更新
		wf := &Workflow{
			ID:          uuid.New().String(),
			TenantID:    req.TenantID,
			Name:        name,
			Description: item.Description,
			Version:     item.Version,
			Visibility:  item.Visibility,
			Definition:  item.Definition,
			CreatedBy:   req.UserID,
		}

		if err == nil && req.Overwrite {
			// 更新现有
			wf.ID = existing.ID
			if err := io.db.WithContext(ctx).Save(wf).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: 更新失败 - %v", item.Name, err))
				result.Skipped++
				continue
			}
		} else {
			// 创建新的
			if err := io.db.WithContext(ctx).Create(wf).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: 创建失败 - %v", item.Name, err))
				result.Skipped++
				continue
			}
		}

		result.IDs = append(result.IDs, wf.ID)
		result.Imported++
	}

	return result, nil
}

// CloneRequest 克隆工作流请求
type CloneRequest struct {
	WorkflowID string
	TenantID   string
	UserID     string
	NewName    string
}

// Clone 克隆工作流到新租户或新名称
func (io *WorkflowIO) Clone(ctx context.Context, req *CloneRequest) (*Workflow, error) {
	var source Workflow
	if err := io.db.WithContext(ctx).
		Where("id = ?", req.WorkflowID).
		First(&source).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("源工作流不存在")
		}
		return nil, fmt.Errorf("查询源工作流失败: %w", err)
	}

	name := req.NewName
	if name == "" {
		name = source.Name + "_copy"
	}

	clone := &Workflow{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		Name:        name,
		Description: source.Description,
		Version:     "v1.0",
		Visibility:  "personal",
		Definition:  source.Definition,
		CreatedBy:   req.UserID,
	}

	if err := io.db.WithContext(ctx).Create(clone).Error; err != nil {
		return nil, fmt.Errorf("克隆工作流失败: %w", err)
	}

	return clone, nil
}
