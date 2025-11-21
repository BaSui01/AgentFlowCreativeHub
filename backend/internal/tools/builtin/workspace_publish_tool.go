package builtin

import (
	"context"
	"errors"
	"fmt"

	"backend/internal/tools"
	"backend/internal/workspace"
)

// WorkspacePublishToolName 工具名称
const WorkspacePublishToolName = "workspace.publish_file"

// WorkspacePublishTool 审核发布工具
type WorkspacePublishTool struct {
	svc *workspace.Service
}

// NewWorkspacePublishTool 构造函数
func NewWorkspacePublishTool(svc *workspace.Service) *WorkspacePublishTool {
	return &WorkspacePublishTool{svc: svc}
}

// GetDefinition 工具定义
func (t *WorkspacePublishTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        WorkspacePublishToolName,
		DisplayName: "发布暂存文件",
		Description: "将待审核的文件移动到正式目录，并生成最新版本",
		Category:    "workspace",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tenant_id":   map[string]any{"type": "string", "description": "租户ID"},
				"staging_id":  map[string]any{"type": "string", "description": "暂存记录ID"},
				"reviewer_id": map[string]any{"type": "string", "description": "审核人"},
			},
			"required": []string{"tenant_id", "staging_id", "reviewer_id"},
		},
	}
}

// Validate 校验参数
func (t *WorkspacePublishTool) Validate(input map[string]any) error {
	if input == nil {
		return errors.New("缺少参数")
	}
	if input["tenant_id"] == nil || fmt.Sprint(input["tenant_id"]) == "" {
		return errors.New("tenant_id 不能为空")
	}
	if input["staging_id"] == nil || fmt.Sprint(input["staging_id"]) == "" {
		return errors.New("staging_id 不能为空")
	}
	if input["reviewer_id"] == nil || fmt.Sprint(input["reviewer_id"]) == "" {
		return errors.New("reviewer_id 不能为空")
	}
	return nil
}

// Execute 执行工具
func (t *WorkspacePublishTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	if err := t.Validate(input); err != nil {
		return nil, err
	}
	tenantID := fmt.Sprint(input["tenant_id"])
	stagingID := fmt.Sprint(input["staging_id"])
	reviewerID := fmt.Sprint(input["reviewer_id"])
	file, version, err := t.svc.PublishStagingFile(ctx, tenantID, stagingID, reviewerID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"fileId":       file.ID,
		"versionId":    version.ID,
		"nodeId":       file.NodeID,
		"reviewStatus": file.ReviewStatus,
	}, nil
}
