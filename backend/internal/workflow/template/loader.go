package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// TemplateLoader 模板加载器
type TemplateLoader struct {
	templates map[string]*Template
	mu        sync.RWMutex
}

// Template 工作流模板
type Template struct {
	Name        string         `yaml:"name" json:"name"`
	Description string         `yaml:"description" json:"description"`
	Category    string         `yaml:"category" json:"category"`
	Definition  map[string]any `yaml:"definition" json:"definition"`
}

// TemplateConfig 模板配置文件
type TemplateConfig struct {
	Templates map[string]*Template `yaml:"templates"`
}

// NewTemplateLoader 创建模板加载器
func NewTemplateLoader() *TemplateLoader {
	return &TemplateLoader{
		templates: make(map[string]*Template),
	}
}

// LoadFromFile 从文件加载模板
func (l *TemplateLoader) LoadFromFile(configPath string) error {
	// 读取文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取模板配置文件失败: %w", err)
	}
	
	// 解析 YAML
	var config TemplateConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析模板配置失败: %w", err)
	}
	
	// 加载模板
	l.mu.Lock()
	defer l.mu.Unlock()
	
	for key, template := range config.Templates {
		l.templates[key] = template
	}
	
	return nil
}

// LoadFromDirectory 从目录加载所有模板文件
func (l *TemplateLoader) LoadFromDirectory(dirPath string) error {
	// 遍历目录
	files, err := filepath.Glob(filepath.Join(dirPath, "*.yaml"))
	if err != nil {
		return fmt.Errorf("遍历模板目录失败: %w", err)
	}
	
	// 加载每个文件
	for _, file := range files {
		if err := l.LoadFromFile(file); err != nil {
			// 记录错误但继续加载其他文件
			continue
		}
	}
	
	return nil
}

// GetTemplate 获取模板
func (l *TemplateLoader) GetTemplate(key string) (*Template, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	template, ok := l.templates[key]
	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", key)
	}
	
	return template, nil
}

// ListTemplates 列出所有模板
func (l *TemplateLoader) ListTemplates() map[string]*Template {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	// 返回副本
	result := make(map[string]*Template, len(l.templates))
	for k, v := range l.templates {
		result[k] = v
	}
	
	return result
}

// ListByCategory 按类别列出模板
func (l *TemplateLoader) ListByCategory(category string) []*Template {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	var result []*Template
	for _, template := range l.templates {
		if template.Category == category {
			result = append(result, template)
		}
	}
	
	return result
}

// InstantiateTemplate 实例化模板
// 将模板转换为可执行的工作流定义，支持参数替换
func (l *TemplateLoader) InstantiateTemplate(key string, params map[string]any) (map[string]any, error) {
	template, err := l.GetTemplate(key)
	if err != nil {
		return nil, err
	}
	
	// 深拷贝定义
	definition := deepCopyMap(template.Definition)
	
	// 应用参数替换
	if params != nil {
		replaceParams(definition, params)
	}
	
	return definition, nil
}

// replaceParams 递归替换参数占位符
func replaceParams(data any, params map[string]any) {
	switch v := data.(type) {
	case map[string]any:
		for k, val := range v {
			if str, ok := val.(string); ok {
				v[k] = replacePlaceholders(str, params)
			} else {
				replaceParams(val, params)
			}
		}
	case []any:
		for i, val := range v {
			if str, ok := val.(string); ok {
				v[i] = replacePlaceholders(str, params)
			} else {
				replaceParams(val, params)
			}
		}
	}
}

// replacePlaceholders 替换字符串中的占位符
func replacePlaceholders(str string, params map[string]any) string {
	// 简单实现：替换 {{key}} 格式的占位符
	// 更复杂的可以使用正则表达式
	result := str
	for key, value := range params {
		placeholder := fmt.Sprintf("{{user_input.%s}}", key)
		if val, ok := value.(string); ok {
			result = strings.Replace(result, placeholder, val, -1)
		} else {
			result = strings.Replace(result, placeholder, fmt.Sprint(value), -1)
		}
	}
	return result
}

// getNestedValue 获取嵌套键的值（如 "user_input.topic"）
func getNestedValue(params map[string]any, key string) any {
	parts := strings.Split(key, ".")
	var current any = params
	
	for _, part := range parts {
		if m, ok := current.(map[string]any); ok {
			current = m[part]
		} else {
			return nil
		}
	}
	
	return current
}

// deepCopyMap 深拷贝 map
func deepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[k] = deepCopyMap(val)
		case []any:
			dst[k] = deepCopySlice(val)
		default:
			dst[k] = v
		}
	}
	
	return dst
}

// deepCopySlice 深拷贝 slice
func deepCopySlice(src []any) []any {
	if src == nil {
		return nil
	}
	
	dst := make([]any, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[i] = deepCopyMap(val)
		case []any:
			dst[i] = deepCopySlice(val)
		default:
			dst[i] = v
		}
	}
	
	return dst
}

// AgentCapabilityLoader Agent 能力加载器
type AgentCapabilityLoader struct {
	capabilities map[string]*AgentCapabilitySet
	mu           sync.RWMutex
}

// AgentCapabilitySet Agent 能力集合
type AgentCapabilitySet struct {
	Description string                    `yaml:"description" json:"description"`
	Roles       []*AgentRoleCapability    `yaml:"roles" json:"roles"`
}

// AgentRoleCapability Agent 角色能力
type AgentRoleCapability struct {
	Role              string         `yaml:"role" json:"role"`
	Name              string         `yaml:"name" json:"name"`
	Description       string         `yaml:"description" json:"description"`
	InputFields       []string       `yaml:"input_fields" json:"input_fields"`
	OutputFields      []string       `yaml:"output_fields" json:"output_fields"`
	SystemPrompt      string         `yaml:"system_prompt" json:"system_prompt"`
	RecommendedConfig map[string]any `yaml:"recommended_config" json:"recommended_config"`
}

// AgentCapabilityConfig Agent 能力配置文件
type AgentCapabilityConfig struct {
	Capabilities map[string]*AgentCapabilitySet `yaml:"capabilities"`
}

// NewAgentCapabilityLoader 创建 Agent 能力加载器
func NewAgentCapabilityLoader() *AgentCapabilityLoader {
	return &AgentCapabilityLoader{
		capabilities: make(map[string]*AgentCapabilitySet),
	}
}

// LoadFromFile 从文件加载能力配置
func (l *AgentCapabilityLoader) LoadFromFile(configPath string) error {
	// 读取文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取能力配置文件失败: %w", err)
	}
	
	// 解析 YAML
	var config AgentCapabilityConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析能力配置失败: %w", err)
	}
	
	// 加载能力
	l.mu.Lock()
	defer l.mu.Unlock()
	
	for agentType, capabilitySet := range config.Capabilities {
		l.capabilities[agentType] = capabilitySet
	}
	
	return nil
}

// GetCapabilities 获取指定 Agent 的能力集合
func (l *AgentCapabilityLoader) GetCapabilities(agentType string) (*AgentCapabilitySet, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	capabilitySet, ok := l.capabilities[agentType]
	if !ok {
		return nil, fmt.Errorf("Agent 类型不存在: %s", agentType)
	}
	
	return capabilitySet, nil
}

// GetRoleCapability 获取指定角色的能力
func (l *AgentCapabilityLoader) GetRoleCapability(agentType, role string) (*AgentRoleCapability, error) {
	capabilitySet, err := l.GetCapabilities(agentType)
	if err != nil {
		return nil, err
	}
	
	for _, roleCapability := range capabilitySet.Roles {
		if roleCapability.Role == role {
			return roleCapability, nil
		}
	}
	
	return nil, fmt.Errorf("角色不存在: %s/%s", agentType, role)
}

// ListAllCapabilities 列出所有能力
func (l *AgentCapabilityLoader) ListAllCapabilities() map[string]*AgentCapabilitySet {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	// 返回副本
	result := make(map[string]*AgentCapabilitySet, len(l.capabilities))
	for k, v := range l.capabilities {
		result[k] = v
	}
	
	return result
}
