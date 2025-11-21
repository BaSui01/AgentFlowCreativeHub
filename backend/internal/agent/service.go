package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backend/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentService Agent 配置管理服务
type AgentService struct {
	db *gorm.DB
}

// NewAgentService 创建 AgentService 实例
func NewAgentService(db *gorm.DB) *AgentService {
	return &AgentService{db: db}
}

// ListAgentConfigsRequest 查询 Agent 配置列表请求
type ListAgentConfigsRequest struct {
	TenantID  string
	AgentType string // writer, reviewer, planner, formatter, translator, analyzer, researcher
	Page      int
	PageSize  int
}

// ListAgentConfigsResponse 查询 Agent 配置列表响应
type ListAgentConfigsResponse struct {
	Agents     []*AgentConfig `json:"agents"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
}

// ListAgentConfigs 查询 Agent 配置列表
func (s *AgentService) ListAgentConfigs(ctx context.Context, req *ListAgentConfigsRequest) (*ListAgentConfigsResponse, error) {
	// 构建查询
	query := s.db.WithContext(ctx).
		Model(&AgentConfig{}).
		Scopes(common.NotDeleted())

	// 租户过滤
	if req.TenantID != "" {
		query = query.Where("tenant_id = ?", req.TenantID)
	}

	// Agent 类型过滤
	if req.AgentType != "" {
		query = query.Where("agent_type = ?", req.AgentType)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("统计 Agent 配置数量失败: %w", err)
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
	var agents []*AgentConfig
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("查询 Agent 配置列表失败: %w", err)
	}

	// 计算总页数
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &ListAgentConfigsResponse{
		Agents:     agents,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetAgentConfig 查询单个 Agent 配置
func (s *AgentService) GetAgentConfig(ctx context.Context, tenantID, agentID string) (*AgentConfig, error) {
	var agent AgentConfig
	if err := s.db.WithContext(ctx).
		Scopes(common.NotDeleted()).
		Where("id = ? AND tenant_id = ?", agentID, tenantID).
		First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Agent 配置不存在")
		}
		return nil, fmt.Errorf("查询 Agent 配置失败: %w", err)
	}
	return &agent, nil
}

// CreateAgentConfigRequest 创建 Agent 配置请求
type CreateAgentConfigRequest struct {
	TenantID          string
	AgentType         string
	Name              string
	Description       string
	ModelID           string
	SecondaryModelID  string
	FallbackStrategy  string
	FallbackTimeoutMs int
	PromptTemplateID  string
	SystemPrompt      string
	Temperature       float64
	MaxTokens         int
	ExtraConfig       map[string]any
}

// CreateAgentConfig 创建 Agent 配置
func (s *AgentService) CreateAgentConfig(ctx context.Context, req *CreateAgentConfigRequest) (*AgentConfig, error) {
	// 验证必填字段
	if req.Name == "" {
		return nil, fmt.Errorf("Agent 名称不能为空")
	}
	if req.AgentType == "" {
		return nil, fmt.Errorf("Agent 类型不能为空")
	}
	if req.ModelID == "" {
		return nil, fmt.Errorf("模型 ID 不能为空")
	}

	// 验证 Agent 类型
	validTypes := map[string]bool{
		"writer":     true,
		"reviewer":   true,
		"planner":    true,
		"formatter":  true,
		"translator": true,
		"analyzer":   true,
		"researcher": true,
	}
	if !validTypes[req.AgentType] {
		return nil, fmt.Errorf("无效的 Agent 类型: %s", req.AgentType)
	}

	// 设置默认值
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 2048
	}

	fallbackStrategy := strings.TrimSpace(req.FallbackStrategy)
	if fallbackStrategy == "" {
		fallbackStrategy = "auto"
	}
	allowedStrategies := map[string]bool{
		"auto":        true,
		"immediate":   true,
		"after_retry": true,
		"manual":      true,
	}
	if !allowedStrategies[fallbackStrategy] {
		return nil, fmt.Errorf("无效的降级策略: %s", fallbackStrategy)
	}
	if req.FallbackTimeoutMs < 0 {
		req.FallbackTimeoutMs = 0
	}

	// 创建 Agent 配置
	agent := &AgentConfig{
		ID:                uuid.New().String(),
		TenantID:          req.TenantID,
		AgentType:         req.AgentType,
		Name:              req.Name,
		Description:       req.Description,
		PrimaryModelID:    req.ModelID,
		SecondaryModelID:  req.SecondaryModelID,
		FallbackStrategy:  fallbackStrategy,
		FallbackTimeoutMs: req.FallbackTimeoutMs,
		PromptTemplateID:  req.PromptTemplateID,
		SystemPrompt:      req.SystemPrompt,
		Temperature:       req.Temperature,
		MaxTokens:         req.MaxTokens,
		ExtraConfig:       req.ExtraConfig,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(agent).Error; err != nil {
		return nil, fmt.Errorf("创建 Agent 配置失败: %w", err)
	}

	hydrateAgentComputedFields(agent)
	return agent, nil
}

// UpdateAgentConfigRequest 更新 Agent 配置请求
type UpdateAgentConfigRequest struct {
	Name              *string
	Description       *string
	ModelID           *string
	SecondaryModelID  *string
	FallbackStrategy  *string
	FallbackTimeoutMs *int
	PromptTemplateID  *string
	SystemPrompt      *string
	Temperature       *float64
	MaxTokens         *int
	ExtraConfig       map[string]any
}

// UpdateAgentConfig 更新 Agent 配置
func (s *AgentService) UpdateAgentConfig(ctx context.Context, tenantID, agentID string, req *UpdateAgentConfigRequest) (*AgentConfig, error) {
	// 查询 Agent 配置
	agent, err := s.GetAgentConfig(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}

	// 更新字段
	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ModelID != nil {
		updates["primary_model_id"] = *req.ModelID
	}
	if req.SecondaryModelID != nil {
		updates["secondary_model_id"] = *req.SecondaryModelID
	}
	if req.FallbackStrategy != nil {
		updates["fallback_strategy"] = *req.FallbackStrategy
	}
	if req.FallbackTimeoutMs != nil {
		updates["fallback_timeout_ms"] = *req.FallbackTimeoutMs
	}
	if req.PromptTemplateID != nil {
		updates["prompt_template_id"] = *req.PromptTemplateID
	}
	if req.SystemPrompt != nil {
		updates["system_prompt"] = *req.SystemPrompt
	}
	if req.Temperature != nil {
		updates["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		updates["max_tokens"] = *req.MaxTokens
	}
	if req.ExtraConfig != nil {
		updates["extra_config"] = req.ExtraConfig
	}
	updates["updated_at"] = time.Now().UTC()

	// 执行更新
	if err := s.db.WithContext(ctx).
		Model(agent).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新 Agent 配置失败: %w", err)
	}

	// 重新查询返回最新数据
	agent, err = s.GetAgentConfig(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// DeleteAgentConfig 软删除 Agent 配置
func (s *AgentService) DeleteAgentConfig(ctx context.Context, tenantID, agentID, operatorID string) error {
	// 查询 Agent 配置
	agent, err := s.GetAgentConfig(ctx, tenantID, agentID)
	if err != nil {
		return err
	}

	// 软删除
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).
		Model(agent).
		Updates(map[string]any{
			"deleted_at": now,
			"deleted_by": operatorID,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("删除 Agent 配置失败: %w", err)
	}

	return nil
}

// SeedDefaultAgents 初始化预置 Agent 配置
func (s *AgentService) SeedDefaultAgents(ctx context.Context, tenantID, defaultModelID string) error {
	defaultAgents := []*CreateAgentConfigRequest{
		{
			TenantID:    tenantID,
			AgentType:   "writer",
			Name:        "内容写作 Agent",
			Description: "专注于创作高质量的内容",
			ModelID:     defaultModelID,
			SystemPrompt: "你是一个专业的内容创作者，擅长撰写清晰、有吸引力的文章。" +
				"请根据用户提供的主题和要求，创作高质量的内容。",
			Temperature: 0.8,
			MaxTokens:   4096,
		},
		{
			TenantID:    tenantID,
			AgentType:   "reviewer",
			Name:        "内容审校 Agent",
			Description: "审校内容的准确性、一致性和可读性",
			ModelID:     defaultModelID,
			SystemPrompt: "你是一个专业的内容审校者，擅长发现文本中的错误和改进空间。" +
				"请仔细审查提供的内容，指出问题并给出修改建议。",
			Temperature: 0.3,
			MaxTokens:   2048,
		},
		{
			TenantID:    tenantID,
			AgentType:   "formatter",
			Name:        "格式化 Agent",
			Description: "格式化内容，统一风格",
			ModelID:     defaultModelID,
			SystemPrompt: "你是一个专业的内容格式化工具，擅长将内容整理为统一的格式和风格。" +
				"请将提供的内容格式化，保持专业和一致性。",
			Temperature: 0.1,
			MaxTokens:   2048,
		},
	}

	// 批量创建
	for _, req := range defaultAgents {
		if _, err := s.CreateAgentConfig(ctx, req); err != nil {
			return fmt.Errorf("创建预置 Agent 失败 [%s]: %w", req.Name, err)
		}
	}

	return nil
}

// GetAgentByType 根据类型获取默认 Agent 配置
func (s *AgentService) GetAgentByType(ctx context.Context, tenantID, agentType string) (*AgentConfig, error) {
	var agent AgentConfig
	if err := s.db.WithContext(ctx).
		Scopes(common.NotDeleted()).
		Where("tenant_id = ? AND agent_type = ?", tenantID, agentType).
		Order("created_at DESC").
		First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("未找到 %s 类型的 Agent 配置", agentType)
		}
		return nil, fmt.Errorf("查询 Agent 配置失败: %w", err)
	}
	return &agent, nil
}

// hydrateAgentComputedFields 填充 Agent 的计算字段
func hydrateAgentComputedFields(agent *AgentConfig) {
	if agent == nil {
		return
	}

	// ModelID 是主模型 ID 的别名（为了向后兼容）
	agent.ModelID = agent.PrimaryModelID

	// 判断是否启用降级
	// 如果有次要模型且最近发生过降级，则认为降级处于激活状态
	agent.FallbackActive = false
	if agent.SecondaryModelID != "" && agent.LastFallbackAt != nil {
		// 如果最近 5 分钟内发生过降级，认为降级仍然激活
		if time.Since(*agent.LastFallbackAt) < 5*time.Minute {
			agent.FallbackActive = true
		}
	}

	// ActiveModelID 是当前实际使用的模型 ID
	if agent.FallbackActive {
		agent.ActiveModelID = agent.SecondaryModelID
	} else {
		agent.ActiveModelID = agent.PrimaryModelID
	}
}
