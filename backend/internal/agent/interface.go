package agent

import "context"

// ============================================================================
// AgentService 接口定义
// ============================================================================

// AgentServiceInterface Agent配置管理服务接口
type AgentServiceInterface interface {
	// CreateAgentConfig 创建Agent配置
	CreateAgentConfig(ctx context.Context, req *CreateAgentConfigRequest) (*AgentConfig, error)

	// GetAgentConfig 查询Agent配置
	GetAgentConfig(ctx context.Context, tenantID, agentID string) (*AgentConfig, error)

	// ListAgentConfigs 查询Agent配置列表
	ListAgentConfigs(ctx context.Context, tenantID string, page, pageSize int) ([]*AgentConfig, int64, error)

	// UpdateAgentConfig 更新Agent配置
	UpdateAgentConfig(ctx context.Context, tenantID, agentID string, req *UpdateAgentConfigRequest) (*AgentConfig, error)

	// DeleteAgentConfig 删除Agent配置
	DeleteAgentConfig(ctx context.Context, tenantID, agentID, operatorID string) error

	// GetAgentByType 根据类型获取Agent配置
	GetAgentByType(ctx context.Context, tenantID, agentType string) (*AgentConfig, error)

	// InitializeDefaultAgents 初始化默认Agent配置
	InitializeDefaultAgents(ctx context.Context, tenantID string) error
}
