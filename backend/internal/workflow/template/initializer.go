package template

import (
	"log"
	"os"
	"path/filepath"
)

// SystemInitializer 系统初始化器
type SystemInitializer struct {
	templateLoader   *TemplateLoader
	capabilityLoader *AgentCapabilityLoader
	roleProvider     *RoleProvider
}

// NewSystemInitializer 创建系统初始化器
func NewSystemInitializer() *SystemInitializer {
	return &SystemInitializer{
		templateLoader:   NewTemplateLoader(),
		capabilityLoader: NewAgentCapabilityLoader(),
	}
}

// Initialize 初始化系统
func (i *SystemInitializer) Initialize(configDir string) error {
	// 1. 加载工作流模板
	templatePath := filepath.Join(configDir, "workflow_templates.yaml")
	if err := i.loadTemplates(templatePath); err != nil {
		log.Printf("警告: 加载工作流模板失败: %v", err)
	} else {
		templates := i.templateLoader.ListTemplates()
		log.Printf("成功加载 %d 个工作流模板", len(templates))
	}
	
	// 2. 加载 Agent 能力配置
	capabilityPath := filepath.Join(configDir, "agent_capabilities.yaml")
	if err := i.loadCapabilities(capabilityPath); err != nil {
		log.Printf("警告: 加载 Agent 能力配置失败: %v", err)
	} else {
		capabilities := i.capabilityLoader.ListAllCapabilities()
		log.Printf("成功加载 %d 种 Agent 能力", len(capabilities))
	}
	
	// 3. 初始化角色提供者
	i.roleProvider = NewRoleProvider(i.capabilityLoader)
	
	log.Println("工作流系统初始化完成")
	return nil
}

// loadTemplates 加载模板
func (i *SystemInitializer) loadTemplates(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // 文件不存在，跳过
	}
	
	return i.templateLoader.LoadFromFile(path)
}

// loadCapabilities 加载能力配置
func (i *SystemInitializer) loadCapabilities(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // 文件不存在，跳过
	}
	
	return i.capabilityLoader.LoadFromFile(path)
}

// GetTemplateLoader 获取模板加载器
func (i *SystemInitializer) GetTemplateLoader() *TemplateLoader {
	return i.templateLoader
}

// GetCapabilityLoader 获取能力加载器
func (i *SystemInitializer) GetCapabilityLoader() *AgentCapabilityLoader {
	return i.capabilityLoader
}

// GetRoleProvider 获取角色提供者
func (i *SystemInitializer) GetRoleProvider() *RoleProvider {
	return i.roleProvider
}
