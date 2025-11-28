package worldbuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"backend/internal/agent/runtime"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service 世界观构建服务
type Service struct {
	db            *gorm.DB
	agentRegistry *runtime.Registry
}

// NewService 创建服务
func NewService(db *gorm.DB, agentRegistry *runtime.Registry) *Service {
	return &Service{
		db:            db,
		agentRegistry: agentRegistry,
	}
}

// AutoMigrate 自动迁移表结构
func (s *Service) AutoMigrate() error {
	return s.db.AutoMigrate(
		&WorldSetting{},
		&SettingVersion{},
		&SettingEntity{},
		&EntityRelation{},
		&SettingTemplate{},
	)
}

// ============================================================================
// 世界观设定 CRUD
// ============================================================================

// CreateSetting 创建世界观设定
func (s *Service) CreateSetting(ctx context.Context, tenantID, userID string, req *CreateSettingRequest) (*WorldSetting, error) {
	setting := &WorldSetting{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		WorkID:      req.WorkID,
		Name:        req.Name,
		Description: req.Description,
		Genre:       req.Genre,
		Status:      "draft",
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}

	if setting.Genre == "" {
		setting.Genre = GenreCustom
	}

	if err := s.db.WithContext(ctx).Create(setting).Error; err != nil {
		return nil, fmt.Errorf("创建世界观失败: %w", err)
	}

	// 创建初始版本
	s.createVersion(ctx, setting.ID, tenantID, userID, "{}", "创建世界观", "create")

	return setting, nil
}

// GetSetting 获取设定详情
func (s *Service) GetSetting(ctx context.Context, id string) (*WorldSetting, error) {
	var setting WorldSetting
	if err := s.db.WithContext(ctx).First(&setting, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}

// ListSettings 获取设定列表
func (s *Service) ListSettings(ctx context.Context, tenantID, workID string, page, pageSize int) ([]WorldSetting, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	db := s.db.WithContext(ctx).Model(&WorldSetting{}).Where("tenant_id = ?", tenantID)
	if workID != "" {
		db = db.Where("work_id = ?", workID)
	}

	var total int64
	db.Count(&total)

	var settings []WorldSetting
	offset := (page - 1) * pageSize
	err := db.Order("updated_at DESC").Limit(pageSize).Offset(offset).Find(&settings).Error

	return settings, total, err
}

// UpdateSetting 更新设定基本信息
func (s *Service) UpdateSetting(ctx context.Context, id, userID string, updates map[string]interface{}) error {
	updates["updated_by"] = userID
	return s.db.WithContext(ctx).Model(&WorldSetting{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteSetting 删除设定
func (s *Service) DeleteSetting(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除关系
		tx.Delete(&EntityRelation{}, "setting_id = ?", id)
		// 删除实体
		tx.Delete(&SettingEntity{}, "setting_id = ?", id)
		// 删除版本
		tx.Delete(&SettingVersion{}, "setting_id = ?", id)
		// 删除设定
		return tx.Delete(&WorldSetting{}, "id = ?", id).Error
	})
}

// ============================================================================
// AI 生成
// ============================================================================

// GenerateSetting AI 生成设定
func (s *Service) GenerateSetting(ctx context.Context, tenantID, userID string, req *GenerateSettingRequest) (*WorldSetting, error) {
	var setting *WorldSetting
	var err error

	// 获取或创建设定
	if req.SettingID != "" {
		setting, err = s.GetSetting(ctx, req.SettingID)
		if err != nil {
			return nil, fmt.Errorf("获取设定失败: %w", err)
		}
	} else {
		setting, err = s.CreateSetting(ctx, tenantID, userID, &CreateSettingRequest{
			Name:  "AI生成的世界观",
			Genre: req.Genre,
		})
		if err != nil {
			return nil, err
		}
	}

	// 获取 world_builder agent
	agent, err := s.agentRegistry.GetAgentByType(ctx, tenantID, "world_builder")
	if err != nil {
		return nil, fmt.Errorf("获取世界观构建Agent失败: %w", err)
	}

	// 构建生成提示
	var promptBuilder strings.Builder
	promptBuilder.WriteString(req.CoreIdea)

	if len(req.Focus) > 0 {
		promptBuilder.WriteString("\n\n重点生成：")
		promptBuilder.WriteString(strings.Join(req.Focus, "、"))
	}

	if req.Constraints != "" {
		promptBuilder.WriteString("\n\n约束条件：")
		promptBuilder.WriteString(req.Constraints)
	}

	// 获取模板
	if req.TemplateID != "" {
		template, _ := s.GetTemplate(ctx, req.TemplateID)
		if template != nil && template.Prompt != "" {
			promptBuilder.WriteString("\n\n参考模板：")
			promptBuilder.WriteString(template.Prompt)
		}
	}

	// 调用 AI
	input := &runtime.AgentInput{
		Content: promptBuilder.String(),
		Context: &runtime.AgentContext{
			TenantID: tenantID,
			UserID:   userID,
		},
		ExtraParams: map[string]any{
			"genre":         req.Genre,
			"scale":         req.Scale,
			"world_setting": setting.Content,
		},
	}

	result, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("AI生成失败: %w", err)
	}

	// 更新设定内容
	setting.Content = result.Output
	setting.UpdatedBy = userID
	if err := s.db.WithContext(ctx).Save(setting).Error; err != nil {
		return nil, err
	}

	// 创建版本快照
	s.createVersion(ctx, setting.ID, tenantID, userID, result.Output, "AI生成世界观", "generate")

	// 解析生成的内容，创建实体
	s.parseAndCreateEntities(ctx, setting.ID, tenantID, userID, result.Output)

	// 更新统计
	s.updateSettingStats(ctx, setting.ID)

	return setting, nil
}

// ModifySetting 增量修改设定
func (s *Service) ModifySetting(ctx context.Context, tenantID, userID string, req *ModifySettingRequest) (*WorldSetting, error) {
	setting, err := s.GetSetting(ctx, req.SettingID)
	if err != nil {
		return nil, err
	}

	// 获取 world_builder agent
	agent, err := s.agentRegistry.GetAgentByType(ctx, tenantID, "world_builder")
	if err != nil {
		return nil, fmt.Errorf("获取世界观构建Agent失败: %w", err)
	}

	// 构建修改提示
	var promptBuilder strings.Builder
	promptBuilder.WriteString("请根据以下指令修改世界观设定：\n\n")
	promptBuilder.WriteString("修改指令：")
	promptBuilder.WriteString(req.Instruction)

	if req.Context != "" {
		promptBuilder.WriteString("\n\n上下文：")
		promptBuilder.WriteString(req.Context)
	}

	// 调用 AI
	input := &runtime.AgentInput{
		Content: promptBuilder.String(),
		Context: &runtime.AgentContext{
			TenantID: tenantID,
			UserID:   userID,
		},
		ExtraParams: map[string]any{
			"world_setting": setting.Content,
		},
	}

	result, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("AI修改失败: %w", err)
	}

	// 更新设定内容
	setting.Content = result.Output
	setting.UpdatedBy = userID
	if err := s.db.WithContext(ctx).Save(setting).Error; err != nil {
		return nil, err
	}

	// 创建版本快照
	s.createVersion(ctx, setting.ID, tenantID, userID, result.Output, req.Instruction, "update")

	return setting, nil
}

// GenerateCharacter AI 生成角色
func (s *Service) GenerateCharacter(ctx context.Context, tenantID, userID, settingID, instruction string) (*SettingEntity, error) {
	setting, err := s.GetSetting(ctx, settingID)
	if err != nil {
		return nil, err
	}

	// 获取 world_builder agent (character_designer role)
	agent, err := s.agentRegistry.GetAgentByType(ctx, tenantID, "world_builder")
	if err != nil {
		return nil, fmt.Errorf("获取角色设计Agent失败: %w", err)
	}

	input := &runtime.AgentInput{
		Content: instruction,
		Context: &runtime.AgentContext{
			TenantID: tenantID,
			UserID:   userID,
		},
		ExtraParams: map[string]any{
			"role":          "character_designer",
			"world_setting": setting.Content,
		},
	}

	result, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("AI生成角色失败: %w", err)
	}

	// 解析结果创建实体
	entity := &SettingEntity{
		ID:          uuid.New().String(),
		SettingID:   settingID,
		TenantID:    tenantID,
		Name:        "AI生成的角色",
		Type:        EntityTypeCharacter,
		Description: result.Output,
		CreatedBy:   userID,
	}

	// 尝试解析 JSON 格式的角色信息
	var charInfo map[string]any
	if err := json.Unmarshal([]byte(result.Output), &charInfo); err == nil {
		if name, ok := charInfo["basic"].(map[string]any)["name"].(string); ok {
			entity.Name = name
		}
		entity.Attributes = charInfo
	}

	if err := s.db.WithContext(ctx).Create(entity).Error; err != nil {
		return nil, err
	}

	s.updateSettingStats(ctx, settingID)
	return entity, nil
}

// GenerateRelations AI 生成关系网络
func (s *Service) GenerateRelations(ctx context.Context, tenantID, userID, settingID string) (*RelationGraph, error) {
	setting, err := s.GetSetting(ctx, settingID)
	if err != nil {
		return nil, err
	}

	// 获取现有实体
	entities, _, _ := s.ListEntities(ctx, &EntityQuery{SettingID: settingID, PageSize: 100})

	// 构建实体列表
	var entityNames []string
	for _, e := range entities {
		entityNames = append(entityNames, fmt.Sprintf("%s(%s)", e.Name, e.Type))
	}

	// 获取 world_builder agent (relation_mapper role)
	agent, err := s.agentRegistry.GetAgentByType(ctx, tenantID, "world_builder")
	if err != nil {
		return nil, fmt.Errorf("获取关系构建Agent失败: %w", err)
	}

	input := &runtime.AgentInput{
		Content: "请为以下实体构建关系网络",
		Context: &runtime.AgentContext{
			TenantID: tenantID,
			UserID:   userID,
		},
		ExtraParams: map[string]any{
			"role":          "relation_mapper",
			"world_setting": setting.Content,
			"entities":      strings.Join(entityNames, "\n"),
		},
	}

	result, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("AI生成关系失败: %w", err)
	}

	// 解析结果
	graph := &RelationGraph{
		Nodes: make([]GraphNode, 0),
		Edges: make([]GraphEdge, 0),
	}

	// 添加节点
	for _, e := range entities {
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:       e.ID,
			Name:     e.Name,
			Type:     e.Type,
			Category: e.Category,
		})
	}

	// 尝试解析 AI 生成的关系
	var relResult struct {
		Edges []struct {
			Source       string `json:"source"`
			Target       string `json:"target"`
			RelationType string `json:"relation_type"`
			Strength     int    `json:"strength"`
			Description  string `json:"description"`
		} `json:"edges"`
	}

	if err := json.Unmarshal([]byte(result.Output), &relResult); err == nil {
		// 创建实体名称到ID的映射
		nameToID := make(map[string]string)
		for _, e := range entities {
			nameToID[e.Name] = e.ID
		}

		for _, edge := range relResult.Edges {
			sourceID := nameToID[edge.Source]
			targetID := nameToID[edge.Target]
			if sourceID == "" || targetID == "" {
				continue
			}

			relation := &EntityRelation{
				ID:          uuid.New().String(),
				SettingID:   settingID,
				TenantID:    tenantID,
				SourceID:    sourceID,
				TargetID:    targetID,
				Type:        edge.RelationType,
				Strength:    edge.Strength,
				Description: edge.Description,
			}
			s.db.WithContext(ctx).Create(relation)

			graph.Edges = append(graph.Edges, GraphEdge{
				ID:          relation.ID,
				Source:      sourceID,
				Target:      targetID,
				Type:        edge.RelationType,
				Strength:    edge.Strength,
				Description: edge.Description,
			})
		}
	}

	s.updateSettingStats(ctx, settingID)
	return graph, nil
}

// ============================================================================
// 实体管理
// ============================================================================

// CreateEntity 创建实体
func (s *Service) CreateEntity(ctx context.Context, tenantID, userID string, req *CreateEntityRequest) (*SettingEntity, error) {
	entity := &SettingEntity{
		ID:          uuid.New().String(),
		SettingID:   req.SettingID,
		TenantID:    tenantID,
		Name:        req.Name,
		Type:        req.Type,
		Category:    req.Category,
		Description: req.Description,
		Attributes:  req.Attributes,
		ParentID:    req.ParentID,
		Tags:        req.Tags,
		CreatedBy:   userID,
	}

	if err := s.db.WithContext(ctx).Create(entity).Error; err != nil {
		return nil, err
	}

	s.updateSettingStats(ctx, req.SettingID)
	return entity, nil
}

// GetEntity 获取实体
func (s *Service) GetEntity(ctx context.Context, id string) (*SettingEntity, error) {
	var entity SettingEntity
	if err := s.db.WithContext(ctx).First(&entity, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

// ListEntities 获取实体列表
func (s *Service) ListEntities(ctx context.Context, query *EntityQuery) ([]SettingEntity, int64, error) {
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 50
	}
	if query.Page <= 0 {
		query.Page = 1
	}

	db := s.db.WithContext(ctx).Model(&SettingEntity{}).Where("setting_id = ?", query.SettingID)

	if query.Type != "" {
		db = db.Where("type = ?", query.Type)
	}
	if query.Category != "" {
		db = db.Where("category = ?", query.Category)
	}
	if query.ParentID != "" {
		db = db.Where("parent_id = ?", query.ParentID)
	}
	if query.Keyword != "" {
		db = db.Where("name ILIKE ? OR description ILIKE ?", "%"+query.Keyword+"%", "%"+query.Keyword+"%")
	}

	var total int64
	db.Count(&total)

	var entities []SettingEntity
	offset := (query.Page - 1) * query.PageSize
	err := db.Order("sort_order, created_at").Limit(query.PageSize).Offset(offset).Find(&entities).Error

	return entities, total, err
}

// UpdateEntity 更新实体
func (s *Service) UpdateEntity(ctx context.Context, id string, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&SettingEntity{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteEntity 删除实体
func (s *Service) DeleteEntity(ctx context.Context, id string) error {
	var entity SettingEntity
	if err := s.db.WithContext(ctx).First(&entity, "id = ?", id).Error; err != nil {
		return err
	}

	// 删除相关关系
	s.db.WithContext(ctx).Delete(&EntityRelation{}, "source_id = ? OR target_id = ?", id, id)
	// 删除实体
	err := s.db.WithContext(ctx).Delete(&SettingEntity{}, "id = ?", id).Error

	s.updateSettingStats(ctx, entity.SettingID)
	return err
}

// ============================================================================
// 关系管理
// ============================================================================

// CreateRelation 创建关系
func (s *Service) CreateRelation(ctx context.Context, tenantID string, req *CreateRelationRequest) (*EntityRelation, error) {
	relation := &EntityRelation{
		ID:          uuid.New().String(),
		SettingID:   req.SettingID,
		TenantID:    tenantID,
		SourceID:    req.SourceID,
		TargetID:    req.TargetID,
		Type:        req.Type,
		Strength:    req.Strength,
		Description: req.Description,
	}

	if relation.Strength == 0 {
		relation.Strength = 3
	}

	if err := s.db.WithContext(ctx).Create(relation).Error; err != nil {
		return nil, err
	}

	s.updateSettingStats(ctx, req.SettingID)
	return relation, nil
}

// GetRelationGraph 获取关系图
func (s *Service) GetRelationGraph(ctx context.Context, settingID string) (*RelationGraph, error) {
	graph := &RelationGraph{
		Nodes: make([]GraphNode, 0),
		Edges: make([]GraphEdge, 0),
	}

	// 获取实体
	var entities []SettingEntity
	s.db.WithContext(ctx).Where("setting_id = ?", settingID).Find(&entities)

	for _, e := range entities {
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:       e.ID,
			Name:     e.Name,
			Type:     e.Type,
			Category: e.Category,
			Data:     e.Attributes,
		})
	}

	// 获取关系
	var relations []EntityRelation
	s.db.WithContext(ctx).Where("setting_id = ?", settingID).Find(&relations)

	for _, r := range relations {
		graph.Edges = append(graph.Edges, GraphEdge{
			ID:          r.ID,
			Source:      r.SourceID,
			Target:      r.TargetID,
			Type:        r.Type,
			Strength:    r.Strength,
			Description: r.Description,
		})
	}

	return graph, nil
}

// DeleteRelation 删除关系
func (s *Service) DeleteRelation(ctx context.Context, id string) error {
	var relation EntityRelation
	if err := s.db.WithContext(ctx).First(&relation, "id = ?", id).Error; err != nil {
		return err
	}
	err := s.db.WithContext(ctx).Delete(&EntityRelation{}, "id = ?", id).Error
	s.updateSettingStats(ctx, relation.SettingID)
	return err
}

// ============================================================================
// 版本管理
// ============================================================================

// GetVersionHistory 获取版本历史
func (s *Service) GetVersionHistory(ctx context.Context, settingID string) ([]SettingVersion, error) {
	var versions []SettingVersion
	err := s.db.WithContext(ctx).
		Where("setting_id = ?", settingID).
		Order("version_num DESC").
		Find(&versions).Error
	return versions, err
}

// GetVersion 获取指定版本
func (s *Service) GetVersion(ctx context.Context, versionID string) (*SettingVersion, error) {
	var version SettingVersion
	if err := s.db.WithContext(ctx).First(&version, "id = ?", versionID).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

// RevertToVersion 恢复到指定版本
func (s *Service) RevertToVersion(ctx context.Context, settingID, versionID, userID string) (*WorldSetting, error) {
	version, err := s.GetVersion(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("版本不存在: %w", err)
	}

	setting, err := s.GetSetting(ctx, settingID)
	if err != nil {
		return nil, err
	}

	// 更新设定内容
	setting.Content = version.Content
	setting.UpdatedBy = userID
	if err := s.db.WithContext(ctx).Save(setting).Error; err != nil {
		return nil, err
	}

	// 创建恢复版本记录
	s.createVersion(ctx, settingID, setting.TenantID, userID, version.Content,
		fmt.Sprintf("恢复到版本 %d", version.VersionNum), "revert")

	return setting, nil
}

// DiffVersions 对比两个版本
func (s *Service) DiffVersions(ctx context.Context, versionIDA, versionIDB string) (*VersionDiff, error) {
	versionA, err := s.GetVersion(ctx, versionIDA)
	if err != nil {
		return nil, fmt.Errorf("版本A不存在: %w", err)
	}

	versionB, err := s.GetVersion(ctx, versionIDB)
	if err != nil {
		return nil, fmt.Errorf("版本B不存在: %w", err)
	}

	diff := &VersionDiff{
		BaseVersion: VersionMeta{
			ID:         versionA.ID,
			VersionNum: versionA.VersionNum,
			Summary:    versionA.Summary,
			CreatedBy:  versionA.CreatedBy,
			CreatedAt:  versionA.CreatedAt,
		},
		TargetVersion: VersionMeta{
			ID:         versionB.ID,
			VersionNum: versionB.VersionNum,
			Summary:    versionB.Summary,
			CreatedBy:  versionB.CreatedBy,
			CreatedAt:  versionB.CreatedAt,
		},
		Changes: []DiffItem{},
	}

	// 简单的内容差异比较
	if versionA.Content != versionB.Content {
		diff.Changes = append(diff.Changes, DiffItem{
			Path:     "content",
			Type:     "modify",
			OldValue: versionA.Content,
			NewValue: versionB.Content,
		})
	}

	return diff, nil
}

// ============================================================================
// 模板管理
// ============================================================================

// GetTemplate 获取模板
func (s *Service) GetTemplate(ctx context.Context, id string) (*SettingTemplate, error) {
	var template SettingTemplate
	if err := s.db.WithContext(ctx).First(&template, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

// ListTemplates 获取模板列表
func (s *Service) ListTemplates(ctx context.Context, tenantID, genre string) ([]SettingTemplate, error) {
	var templates []SettingTemplate
	db := s.db.WithContext(ctx).Where("is_active = ?", true).
		Where("tenant_id = ? OR is_builtin = ?", tenantID, true)

	if genre != "" {
		db = db.Where("genre = ?", genre)
	}

	err := db.Order("is_builtin DESC, usage_count DESC").Find(&templates).Error
	return templates, err
}

// CreateTemplate 创建模板
func (s *Service) CreateTemplate(ctx context.Context, tenantID string, template *SettingTemplate) error {
	template.ID = uuid.New().String()
	template.TenantID = tenantID
	template.IsBuiltin = false
	return s.db.WithContext(ctx).Create(template).Error
}

// ============================================================================
// 统计
// ============================================================================

// GetStats 获取设定统计
func (s *Service) GetStats(ctx context.Context, settingID string) (*SettingStats, error) {
	stats := &SettingStats{
		SettingID:  settingID,
		ByType:     make(map[string]int),
		ByCategory: make(map[string]int),
	}

	// 实体数量
	s.db.WithContext(ctx).Model(&SettingEntity{}).
		Where("setting_id = ?", settingID).
		Count(&stats.TotalEntities)

	// 关系数量
	s.db.WithContext(ctx).Model(&EntityRelation{}).
		Where("setting_id = ?", settingID).
		Count(&stats.TotalRelations)

	// 版本数量
	s.db.WithContext(ctx).Model(&SettingVersion{}).
		Where("setting_id = ?", settingID).
		Count(&stats.TotalVersions)

	// 按类型统计
	var typeStats []struct {
		Type  string
		Count int
	}
	s.db.WithContext(ctx).Model(&SettingEntity{}).
		Select("type, COUNT(*) as count").
		Where("setting_id = ?", settingID).
		Group("type").
		Scan(&typeStats)
	for _, ts := range typeStats {
		stats.ByType[ts.Type] = ts.Count
	}

	// 按类别统计
	var catStats []struct {
		Category string
		Count    int
	}
	s.db.WithContext(ctx).Model(&SettingEntity{}).
		Select("category, COUNT(*) as count").
		Where("setting_id = ? AND category != ''", settingID).
		Group("category").
		Scan(&catStats)
	for _, cs := range catStats {
		stats.ByCategory[cs.Category] = cs.Count
	}

	return stats, nil
}

// ============================================================================
// 内部方法
// ============================================================================

func (s *Service) createVersion(ctx context.Context, settingID, tenantID, userID, content, summary, changeType string) {
	// 获取当前最大版本号
	var maxVersion int
	s.db.WithContext(ctx).Model(&SettingVersion{}).
		Where("setting_id = ?", settingID).
		Select("COALESCE(MAX(version_num), 0)").
		Scan(&maxVersion)

	version := &SettingVersion{
		ID:         uuid.New().String(),
		SettingID:  settingID,
		TenantID:   tenantID,
		Content:    content,
		Summary:    summary,
		VersionNum: maxVersion + 1,
		ChangeType: changeType,
		ChangeDesc: summary,
		CreatedBy:  userID,
	}

	s.db.WithContext(ctx).Create(version)

	// 更新设定的最新版本ID和版本计数
	s.db.WithContext(ctx).Model(&WorldSetting{}).
		Where("id = ?", settingID).
		Updates(map[string]interface{}{
			"latest_version_id": version.ID,
			"version_count":     maxVersion + 1,
		})
}

func (s *Service) updateSettingStats(ctx context.Context, settingID string) {
	var entityCount, relationCount int64
	s.db.WithContext(ctx).Model(&SettingEntity{}).Where("setting_id = ?", settingID).Count(&entityCount)
	s.db.WithContext(ctx).Model(&EntityRelation{}).Where("setting_id = ?", settingID).Count(&relationCount)

	s.db.WithContext(ctx).Model(&WorldSetting{}).
		Where("id = ?", settingID).
		Updates(map[string]interface{}{
			"entity_count":   entityCount,
			"relation_count": relationCount,
		})
}

func (s *Service) parseAndCreateEntities(ctx context.Context, settingID, tenantID, userID, content string) {
	// 尝试解析 JSON 格式的世界观设定
	var worldData map[string]any
	if err := json.Unmarshal([]byte(content), &worldData); err != nil {
		return
	}

	// 解析势力
	if factions, ok := worldData["factions"].([]interface{}); ok {
		for _, f := range factions {
			if faction, ok := f.(map[string]interface{}); ok {
				entity := &SettingEntity{
					ID:        uuid.New().String(),
					SettingID: settingID,
					TenantID:  tenantID,
					Type:      EntityTypeFaction,
					CreatedBy: userID,
				}
				if name, ok := faction["name"].(string); ok {
					entity.Name = name
				}
				if desc, ok := faction["description"].(string); ok {
					entity.Description = desc
				}
				entity.Attributes = faction
				s.db.WithContext(ctx).Create(entity)
			}
		}
	}

	// 可以继续解析其他类型的实体...
}

// InitBuiltinTemplates 初始化内置模板
func (s *Service) InitBuiltinTemplates(ctx context.Context) error {
	templates := []SettingTemplate{
		{
			ID:          uuid.New().String(),
			Name:        "玄幻世界模板",
			Description: "适用于东方玄幻、修真、仙侠类作品",
			Genre:       GenreFantasy,
			IsBuiltin:   true,
			IsActive:    true,
			Structure: map[string]any{
				"world":        []string{"世界名称", "世界背景", "核心规则"},
				"power_system": []string{"修炼体系", "境界划分", "特殊能力"},
				"factions":     []string{"势力名称", "势力描述", "势力立场"},
				"geography":    []string{"区域划分", "特殊地点"},
			},
		},
		{
			ID:          uuid.New().String(),
			Name:        "都市异能模板",
			Description: "适用于都市、异能、超能力类作品",
			Genre:       GenreModern,
			IsBuiltin:   true,
			IsActive:    true,
			Structure: map[string]any{
				"world":        []string{"背景设定", "异能来源"},
				"power_system": []string{"能力分类", "等级划分"},
				"organizations": []string{"组织名称", "组织目的"},
			},
		},
		{
			ID:          uuid.New().String(),
			Name:        "科幻世界模板",
			Description: "适用于星际、机甲、赛博朋克类作品",
			Genre:       GenreSciFi,
			IsBuiltin:   true,
			IsActive:    true,
			Structure: map[string]any{
				"world":      []string{"时代背景", "科技水平", "社会结构"},
				"technology": []string{"核心科技", "日常科技"},
				"species":    []string{"种族/文明", "外星生物"},
				"factions":   []string{"势力/国家", "企业/组织"},
			},
		},
	}

	for _, t := range templates {
		var existing SettingTemplate
		if err := s.db.WithContext(ctx).Where("name = ? AND is_builtin = ?", t.Name, true).First(&existing).Error; err != nil {
			s.db.WithContext(ctx).Create(&t)
		}
	}

	return nil
}


// ============================================================================
// 侧边栏快速查阅
// ============================================================================

// GetWorkSettingsSummary 获取作品设定摘要（用于侧边栏）
func (s *Service) GetWorkSettingsSummary(ctx context.Context, workID string) (*WorkSettingsSummary, error) {
	// 获取作品的世界观设定
	var settings []WorldSetting
	if err := s.db.WithContext(ctx).
		Where("work_id = ?", workID).
		Order("updated_at DESC").
		Find(&settings).Error; err != nil {
		return nil, err
	}

	if len(settings) == 0 {
		return &WorkSettingsSummary{
			WorkID:   workID,
			Entities: []EntitySummary{},
		}, nil
	}

	// 使用最新的设定
	settingID := settings[0].ID

	// 获取所有实体并按类型分组
	var entities []SettingEntity
	if err := s.db.WithContext(ctx).
		Where("setting_id = ?", settingID).
		Order("created_at ASC").
		Find(&entities).Error; err != nil {
		return nil, err
	}

	// 转换为摘要格式
	summaries := make([]EntitySummary, len(entities))
	for i, e := range entities {
		summaries[i] = EntitySummary{
			ID:          e.ID,
			Type:        string(e.Type),
			Name:        e.Name,
			Description: e.Description,
			Category:    e.Category,
		}
	}

	return &WorkSettingsSummary{
		WorkID:      workID,
		SettingID:   settingID,
		SettingName: settings[0].Name,
		Entities:    summaries,
	}, nil
}

// SearchEntitiesInWork 搜索作品中的实体（用于侧边栏搜索）
func (s *Service) SearchEntitiesInWork(ctx context.Context, workID, keyword string, entityType *string) ([]SettingEntity, error) {
	// 获取作品的最新设定
	var setting WorldSetting
	if err := s.db.WithContext(ctx).
		Where("work_id = ?", workID).
		Order("updated_at DESC").
		First(&setting).Error; err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).Where("setting_id = ?", setting.ID)

	if keyword != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if entityType != nil {
		query = query.Where("type = ?", *entityType)
	}

	var entities []SettingEntity
	if err := query.Order("created_at ASC").Find(&entities).Error; err != nil {
		return nil, err
	}

	return entities, nil
}
