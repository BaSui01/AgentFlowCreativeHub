package workflows

import (
	"encoding/json"
	"fmt"

	"backend/internal/workflow"
)

// mapToWorkflowDefinition 将通用 map 转换为 workflow.WorkflowDefinition
func mapToWorkflowDefinition(data map[string]any) (workflow.WorkflowDefinition, error) {
	var def workflow.WorkflowDefinition
	bytes, err := json.Marshal(data)
	if err != nil {
		return def, fmt.Errorf("编码工作流定义失败: %w", err)
	}
	if err := json.Unmarshal(bytes, &def); err != nil {
		return def, fmt.Errorf("解析工作流定义失败: %w", err)
	}
	return def, nil
}
