package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// AgentConfigVersion Agent 配置版本
type AgentConfigVersion struct {
	ID            string    `json:"id" gorm:"primaryKey;type:uuid"`
	AgentConfigID string    `json:"agentConfigId" gorm:"type:uuid;not null;index"`
	TenantID      string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	Version       int       `json:"version" gorm:"not null"`
	Snapshot      string    `json:"snapshot" gorm:"type:jsonb;not null"` // JSON 快照
	ChangeLog     string    `json:"changeLog" gorm:"type:text"`
	CreatedBy     string    `json:"createdBy" gorm:"type:uuid"`
	CreatedAt     time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
}

func (AgentConfigVersion) TableName() string {
	return "agent_config_versions"
}

// AgentVersionService Agent 配置版本管理服务
type AgentVersionService struct {
	db *gorm.DB
}

// NewAgentVersionService 创建版本服务
func NewAgentVersionService(db *gorm.DB) *AgentVersionService {
	return &AgentVersionService{db: db}
}

// AutoMigrate 自动迁移
func (s *AgentVersionService) AutoMigrate() error {
	return s.db.AutoMigrate(&AgentConfigVersion{})
}

// CreateVersion 创建新版本
func (s *AgentVersionService) CreateVersion(ctx context.Context, agentID, tenantID, userID, changeLog string) (*AgentConfigVersion, error) {
	// 获取当前配置
	var agent AgentConfig
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
		return nil, fmt.Errorf("获取 Agent 配置失败: %w", err)
	}

	// 获取最新版本号
	var latestVersion int
	s.db.WithContext(ctx).Model(&AgentConfigVersion{}).
		Where("agent_config_id = ?", agentID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&latestVersion)

	// 序列化快照
	snapshot, err := json.Marshal(agent)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	version := &AgentConfigVersion{
		ID:            uuid.New().String(),
		AgentConfigID: agentID,
		TenantID:      tenantID,
		Version:       latestVersion + 1,
		Snapshot:      string(snapshot),
		ChangeLog:     changeLog,
		CreatedBy:     userID,
	}

	if err := s.db.WithContext(ctx).Create(version).Error; err != nil {
		return nil, fmt.Errorf("创建版本失败: %w", err)
	}

	return version, nil
}

// ListVersions 列出版本历史
func (s *AgentVersionService) ListVersions(ctx context.Context, agentID, tenantID string) ([]AgentConfigVersion, error) {
	var versions []AgentConfigVersion
	err := s.db.WithContext(ctx).
		Where("agent_config_id = ? AND tenant_id = ?", agentID, tenantID).
		Order("version DESC").
		Find(&versions).Error
	return versions, err
}

// GetVersion 获取指定版本
func (s *AgentVersionService) GetVersion(ctx context.Context, agentID string, version int) (*AgentConfigVersion, error) {
	var v AgentConfigVersion
	err := s.db.WithContext(ctx).
		Where("agent_config_id = ? AND version = ?", agentID, version).
		First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// RevertToVersion 回滚到指定版本
func (s *AgentVersionService) RevertToVersion(ctx context.Context, agentID, tenantID, userID string, version int) (*AgentConfig, error) {
	// 获取目标版本
	targetVersion, err := s.GetVersion(ctx, agentID, version)
	if err != nil {
		return nil, fmt.Errorf("获取目标版本失败: %w", err)
	}

	// 解析快照
	var agent AgentConfig
	if err := json.Unmarshal([]byte(targetVersion.Snapshot), &agent); err != nil {
		return nil, fmt.Errorf("解析快照失败: %w", err)
	}

	// 先保存当前版本
	_, _ = s.CreateVersion(ctx, agentID, tenantID, userID, fmt.Sprintf("回滚前备份 (回滚到版本 %d)", version))

	// 更新配置
	agent.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&agent).Error; err != nil {
		return nil, fmt.Errorf("更新配置失败: %w", err)
	}

	// 创建回滚版本记录
	_, _ = s.CreateVersion(ctx, agentID, tenantID, userID, fmt.Sprintf("回滚到版本 %d", version))

	return &agent, nil
}

// DiffVersions 比较两个版本
func (s *AgentVersionService) DiffVersions(ctx context.Context, agentID string, v1, v2 int) (map[string]interface{}, error) {
	version1, err := s.GetVersion(ctx, agentID, v1)
	if err != nil {
		return nil, fmt.Errorf("获取版本 %d 失败: %w", v1, err)
	}

	version2, err := s.GetVersion(ctx, agentID, v2)
	if err != nil {
		return nil, fmt.Errorf("获取版本 %d 失败: %w", v2, err)
	}

	var config1, config2 map[string]interface{}
	json.Unmarshal([]byte(version1.Snapshot), &config1)
	json.Unmarshal([]byte(version2.Snapshot), &config2)

	diff := make(map[string]interface{})
	diff["version1"] = v1
	diff["version2"] = v2
	diff["changes"] = findDifferences(config1, config2)

	return diff, nil
}

// findDifferences 找出两个 map 的差异
func findDifferences(m1, m2 map[string]interface{}) map[string]interface{} {
	changes := make(map[string]interface{})

	for k, v1 := range m1 {
		if v2, ok := m2[k]; ok {
			if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
				changes[k] = map[string]interface{}{
					"old": v1,
					"new": v2,
				}
			}
		} else {
			changes[k] = map[string]interface{}{
				"old":     v1,
				"new":     nil,
				"removed": true,
			}
		}
	}

	for k, v2 := range m2 {
		if _, ok := m1[k]; !ok {
			changes[k] = map[string]interface{}{
				"old":   nil,
				"new":   v2,
				"added": true,
			}
		}
	}

	return changes
}

// ============================================================================
// Agent 配置导入导出
// ============================================================================

// AgentExportFormat 导出格式
type AgentExportFormat string

const (
	AgentFormatJSON AgentExportFormat = "json"
	AgentFormatYAML AgentExportFormat = "yaml"
)

// AgentExportData 导出数据
type AgentExportData struct {
	Version    string            `json:"version" yaml:"version"`
	ExportedAt string            `json:"exportedAt" yaml:"exportedAt"`
	Agents     []AgentExportItem `json:"agents" yaml:"agents"`
}

// AgentExportItem 导出项
type AgentExportItem struct {
	AgentType        string         `json:"agentType" yaml:"agentType"`
	Name             string         `json:"name" yaml:"name"`
	Description      string         `json:"description" yaml:"description"`
	SystemPrompt     string         `json:"systemPrompt" yaml:"systemPrompt"`
	Temperature      float64        `json:"temperature" yaml:"temperature"`
	MaxTokens        int            `json:"maxTokens" yaml:"maxTokens"`
	RAGEnabled       bool           `json:"ragEnabled" yaml:"ragEnabled"`
	RAGTopK          int            `json:"ragTopK" yaml:"ragTopK"`
	RAGMinScore      float64        `json:"ragMinScore" yaml:"ragMinScore"`
	AllowedTools     []string       `json:"allowedTools" yaml:"allowedTools"`
	AutoToolUse      bool           `json:"autoToolUse" yaml:"autoToolUse"`
	ExtraConfig      map[string]any `json:"extraConfig" yaml:"extraConfig"`
	FallbackStrategy string         `json:"fallbackStrategy" yaml:"fallbackStrategy"`
}

// AgentIO Agent 配置导入导出服务
type AgentIO struct {
	db *gorm.DB
}

// NewAgentIO 创建 IO 服务
func NewAgentIO(db *gorm.DB) *AgentIO {
	return &AgentIO{db: db}
}

// ExportAgentRequest 导出请求
type ExportAgentRequest struct {
	AgentIDs []string
	TenantID string
	Format   AgentExportFormat
}

// ExportAgentResult 导出结果
type ExportAgentResult struct {
	Data        []byte
	Filename    string
	ContentType string
}

// Export 导出 Agent 配置
func (io *AgentIO) Export(ctx context.Context, req *ExportAgentRequest) (*ExportAgentResult, error) {
	var agents []AgentConfig
	query := io.db.WithContext(ctx).Where("tenant_id = ?", req.TenantID)

	if len(req.AgentIDs) > 0 {
		query = query.Where("id IN ?", req.AgentIDs)
	}

	if err := query.Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("查询 Agent 配置失败: %w", err)
	}

	if len(agents) == 0 {
		return nil, fmt.Errorf("未找到 Agent 配置")
	}

	items := make([]AgentExportItem, len(agents))
	for i, a := range agents {
		items[i] = AgentExportItem{
			AgentType:        a.AgentType,
			Name:             a.Name,
			Description:      a.Description,
			SystemPrompt:     a.SystemPrompt,
			Temperature:      a.Temperature,
			MaxTokens:        a.MaxTokens,
			RAGEnabled:       a.RAGEnabled,
			RAGTopK:          a.RAGTopK,
			RAGMinScore:      a.RAGMinScore,
			AllowedTools:     a.AllowedTools,
			AutoToolUse:      a.AutoToolUse,
			ExtraConfig:      a.ExtraConfig,
			FallbackStrategy: a.FallbackStrategy,
		}
	}

	exportData := AgentExportData{
		Version:    "1.0",
		ExportedAt: time.Now().Format(time.RFC3339),
		Agents:     items,
	}

	var data []byte
	var err error
	var ext, contentType string

	switch req.Format {
	case AgentFormatYAML:
		data, err = yaml.Marshal(exportData)
		ext = "yaml"
		contentType = "application/x-yaml"
	default:
		data, err = json.MarshalIndent(exportData, "", "  ")
		ext = "json"
		contentType = "application/json"
	}

	if err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	return &ExportAgentResult{
		Data:        data,
		Filename:    fmt.Sprintf("agents_%s.%s", timestamp, ext),
		ContentType: contentType,
	}, nil
}

// ImportAgentRequest 导入请求
type ImportAgentRequest struct {
	TenantID       string
	UserID         string
	Data           []byte
	Format         AgentExportFormat
	Overwrite      bool
	DefaultModelID string // 导入时使用的默认模型 ID
}

// ImportAgentResult 导入结果
type ImportAgentResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
	IDs      []string `json:"ids"`
}

// Import 导入 Agent 配置
func (io *AgentIO) Import(ctx context.Context, req *ImportAgentRequest) (*ImportAgentResult, error) {
	var exportData AgentExportData

	var unmarshal func([]byte, interface{}) error
	if req.Format == AgentFormatYAML {
		unmarshal = yaml.Unmarshal
	} else {
		unmarshal = json.Unmarshal
	}

	if err := unmarshal(req.Data, &exportData); err != nil {
		return nil, fmt.Errorf("解析导入数据失败: %w", err)
	}

	result := &ImportAgentResult{
		IDs: make([]string, 0),
	}

	for _, item := range exportData.Agents {
		// 检查同名配置
		var existing AgentConfig
		err := io.db.WithContext(ctx).
			Where("tenant_id = ? AND name = ?", req.TenantID, item.Name).
			First(&existing).Error

		if err == nil && !req.Overwrite {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: 已存在同名配置", item.Name))
			result.Skipped++
			continue
		}

		agent := &AgentConfig{
			ID:               uuid.New().String(),
			TenantID:         req.TenantID,
			AgentType:        item.AgentType,
			Name:             item.Name,
			Description:      item.Description,
			PrimaryModelID:   req.DefaultModelID,
			SystemPrompt:     item.SystemPrompt,
			Temperature:      item.Temperature,
			MaxTokens:        item.MaxTokens,
			RAGEnabled:       item.RAGEnabled,
			RAGTopK:          item.RAGTopK,
			RAGMinScore:      item.RAGMinScore,
			AllowedTools:     item.AllowedTools,
			AutoToolUse:      item.AutoToolUse,
			ExtraConfig:      item.ExtraConfig,
			FallbackStrategy: item.FallbackStrategy,
			Status:           "active",
		}

		if err == nil && req.Overwrite {
			agent.ID = existing.ID
			if err := io.db.WithContext(ctx).Save(agent).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: 更新失败 - %v", item.Name, err))
				result.Skipped++
				continue
			}
		} else {
			if err := io.db.WithContext(ctx).Create(agent).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: 创建失败 - %v", item.Name, err))
				result.Skipped++
				continue
			}
		}

		result.IDs = append(result.IDs, agent.ID)
		result.Imported++
	}

	return result, nil
}
