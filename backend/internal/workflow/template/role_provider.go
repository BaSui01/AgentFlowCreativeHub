package template

import (
	"fmt"
	"sync"
)

// RoleProvider Agent 角色提供者
// 提供角色到 System Prompt 的映射
type RoleProvider struct {
	capabilityLoader *AgentCapabilityLoader
	mu               sync.RWMutex
}

// NewRoleProvider 创建角色提供者
func NewRoleProvider(capabilityLoader *AgentCapabilityLoader) *RoleProvider {
	return &RoleProvider{
		capabilityLoader: capabilityLoader,
	}
}

// GetSystemPrompt 获取角色的 System Prompt
func (p *RoleProvider) GetSystemPrompt(agentType, role string) (string, error) {
	if p.capabilityLoader == nil {
		return "", fmt.Errorf("能力加载器未初始化")
	}
	
	// 获取角色能力
	roleCapability, err := p.capabilityLoader.GetRoleCapability(agentType, role)
	if err != nil {
		return "", fmt.Errorf("获取角色能力失败: %w", err)
	}
	
	return roleCapability.SystemPrompt, nil
}

// GetRecommendedConfig 获取角色的推荐配置
func (p *RoleProvider) GetRecommendedConfig(agentType, role string) (map[string]any, error) {
	if p.capabilityLoader == nil {
		return nil, fmt.Errorf("能力加载器未初始化")
	}
	
	// 获取角色能力
	roleCapability, err := p.capabilityLoader.GetRoleCapability(agentType, role)
	if err != nil {
		return nil, fmt.Errorf("获取角色能力失败: %w", err)
	}
	
	return roleCapability.RecommendedConfig, nil
}

// ValidateRole 验证角色是否有效
func (p *RoleProvider) ValidateRole(agentType, role string) bool {
	if p.capabilityLoader == nil {
		return false
	}
	
	_, err := p.capabilityLoader.GetRoleCapability(agentType, role)
	return err == nil
}

// GetInputFields 获取角色的输入字段
func (p *RoleProvider) GetInputFields(agentType, role string) ([]string, error) {
	if p.capabilityLoader == nil {
		return nil, fmt.Errorf("能力加载器未初始化")
	}
	
	roleCapability, err := p.capabilityLoader.GetRoleCapability(agentType, role)
	if err != nil {
		return nil, err
	}
	
	return roleCapability.InputFields, nil
}

// GetOutputFields 获取角色的输出字段
func (p *RoleProvider) GetOutputFields(agentType, role string) ([]string, error) {
	if p.capabilityLoader == nil {
		return nil, fmt.Errorf("能力加载器未初始化")
	}
	
	roleCapability, err := p.capabilityLoader.GetRoleCapability(agentType, role)
	if err != nil {
		return nil, err
	}
	
	return roleCapability.OutputFields, nil
}
