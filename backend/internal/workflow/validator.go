package workflow

import (
	"fmt"

	"backend/internal/workflow/template"
)

// Validator 工作流定义验证器
type Validator struct {
	capabilityLoader *template.AgentCapabilityLoader
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// NewValidator 创建验证器
func NewValidator(capabilityLoader *template.AgentCapabilityLoader) *Validator {
	return &Validator{
		capabilityLoader: capabilityLoader,
	}
}

// Validate 验证工作流定义
func (v *Validator) Validate(definition map[string]any) []ValidationError {
	errors := []ValidationError{}
	
	// 1. 检查必需字段
	steps, ok := definition["steps"]
	if !ok {
		errors = append(errors, ValidationError{
			Field:   "steps",
			Message: "缺少 steps 字段",
		})
		return errors
	}
	
	stepList, ok := steps.([]any)
	if !ok {
		errors = append(errors, ValidationError{
			Field:   "steps",
			Message: "steps 必须是数组",
		})
		return errors
	}
	
	if len(stepList) == 0 {
		errors = append(errors, ValidationError{
			Field:   "steps",
			Message: "至少需要一个步骤",
		})
		return errors
	}
	
	// 2. 检查每个步骤
	stepIDs := make(map[string]bool)
	for i, step := range stepList {
		stepMap, ok := step.(map[string]any)
		if !ok {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("steps[%d]", i),
				Message: "步骤格式无效",
			})
			continue
		}
		
		// 检查步骤 ID
		stepID, ok := stepMap["id"].(string)
		if !ok || stepID == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("steps[%d].id", i),
				Message: "缺少或无效的步骤 ID",
			})
		} else {
			if stepIDs[stepID] {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("steps[%d].id", i),
					Message: fmt.Sprintf("重复的步骤 ID: %s", stepID),
				})
			}
			stepIDs[stepID] = true
		}
		
		// 检查步骤类型
		stepType, ok := stepMap["type"].(string)
		if !ok {
			stepType = "agent" // 默认为 agent 类型
		}
		
		// 对于 agent 类型，检查 agent_type
		if stepType == "agent" {
			agentType, ok := stepMap["agent_type"].(string)
			if !ok || agentType == "" {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("steps[%d].agent_type", i),
					Message: "agent 类型步骤缺少 agent_type 字段",
				})
				continue
			}
			
			// 检查 agent_type 是否有效
			if v.capabilityLoader != nil {
				if _, err := v.capabilityLoader.GetCapabilities(agentType); err != nil {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("steps[%d].agent_type", i),
						Message: fmt.Sprintf("无效的 Agent 类型: %s", agentType),
					})
				}
			}
			
			// 检查 role 是否有效
			if role, ok := stepMap["role"].(string); ok && role != "" {
				if v.capabilityLoader != nil {
					if _, err := v.capabilityLoader.GetRoleCapability(agentType, role); err != nil {
						errors = append(errors, ValidationError{
							Field:   fmt.Sprintf("steps[%d].role", i),
							Message: fmt.Sprintf("无效的角色: %s/%s", agentType, role),
						})
					}
				}
			}
		}
	}
	
	// 3. 检查依赖关系
	errors = append(errors, v.validateDependencies(stepList, stepIDs)...)
	
	// 4. 检测循环依赖
	if cycleError := v.detectCyclicDependencies(stepList); cycleError != nil {
		errors = append(errors, *cycleError)
	}
	
	return errors
}

// validateDependencies 验证步骤依赖关系
func (v *Validator) validateDependencies(steps []any, stepIDs map[string]bool) []ValidationError {
	errors := []ValidationError{}
	
	for i, step := range steps {
		stepMap, ok := step.(map[string]any)
		if !ok {
			continue
		}
		
		// 检查 depends_on
		if dependsOn, ok := stepMap["depends_on"]; ok {
			var depList []string
			
			switch deps := dependsOn.(type) {
			case []any:
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok {
						depList = append(depList, depStr)
					}
				}
			case []string:
				depList = deps
			}
			
			// 检查依赖的步骤是否存在
			for _, depID := range depList {
				if !stepIDs[depID] {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("steps[%d].depends_on", i),
						Message: fmt.Sprintf("依赖的步骤不存在: %s", depID),
					})
				}
			}
		}
	}
	
	return errors
}

// detectCyclicDependencies 检测循环依赖
func (v *Validator) detectCyclicDependencies(steps []any) *ValidationError {
	// 构建依赖图
	graph := make(map[string][]string)
	stepIndex := make(map[string]int)
	
	for i, step := range steps {
		stepMap, ok := step.(map[string]any)
		if !ok {
			continue
		}
		
		stepID, ok := stepMap["id"].(string)
		if !ok || stepID == "" {
			continue
		}
		
		stepIndex[stepID] = i
		graph[stepID] = []string{}
		
		// 获取依赖列表
		if dependsOn, ok := stepMap["depends_on"]; ok {
			var depList []string
			
			switch deps := dependsOn.(type) {
			case []any:
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok {
						depList = append(depList, depStr)
					}
				}
			case []string:
				depList = deps
			}
			
			graph[stepID] = depList
		}
	}
	
	// 使用 DFS 检测环
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	for stepID := range graph {
		if !visited[stepID] {
			if cycle := v.dfsCycleDetect(stepID, graph, visited, recStack, []string{}); cycle != nil {
				return &ValidationError{
					Field:   "steps",
					Message: fmt.Sprintf("检测到循环依赖: %s", formatCycle(cycle)),
				}
			}
		}
	}
	
	return nil
}

// dfsCycleDetect DFS 检测环
func (v *Validator) dfsCycleDetect(
	node string,
	graph map[string][]string,
	visited map[string]bool,
	recStack map[string]bool,
	path []string,
) []string {
	visited[node] = true
	recStack[node] = true
	path = append(path, node)
	
	for _, neighbor := range graph[node] {
		if !visited[neighbor] {
			if cycle := v.dfsCycleDetect(neighbor, graph, visited, recStack, path); cycle != nil {
				return cycle
			}
		} else if recStack[neighbor] {
			// 找到环，返回环的路径
			cycle := []string{neighbor}
			for i := len(path) - 1; i >= 0; i-- {
				cycle = append(cycle, path[i])
				if path[i] == neighbor {
					break
				}
			}
			return cycle
		}
	}
	
	recStack[node] = false
	return nil
}

// formatCycle 格式化循环路径
func formatCycle(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	
	// 反转数组（因为 DFS 是反向记录的）
	for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
		cycle[i], cycle[j] = cycle[j], cycle[i]
	}
	
	result := ""
	for i, step := range cycle {
		if i > 0 {
			result += " -> "
		}
		result += step
	}
	
	return result
}

// ValidateQuick 快速验证（仅检查基本字段）
func (v *Validator) ValidateQuick(definition map[string]any) error {
	errors := v.Validate(definition)
	if len(errors) > 0 {
		return fmt.Errorf("工作流定义验证失败: %s", errors[0].Message)
	}
	return nil
}
