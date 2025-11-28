package workflows

import (
	"backend/internal/workflow"
)

// ========== 审批相关 ==========

// ApprovalActionRequest 审批操作请求
type ApprovalActionRequest struct {
	Comment string `json:"comment"`
}

// ApprovalListResponse 审批列表响应
type ApprovalListResponse struct {
	Approvals []*workflow.ApprovalRequest `json:"approvals"`
	Total     int                         `json:"total"`
}

// ========== 审批规则 ==========

// CreateApprovalRuleRequest 创建审批规则请求
type CreateApprovalRuleRequest struct {
	WorkflowID  string         `json:"workflowId"`
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Conditions  map[string]any `json:"conditions" binding:"required"`
	Actions     map[string]any `json:"actions" binding:"required"`
	Priority    int            `json:"priority"`
	IsActive    bool           `json:"isActive"`
}

// UpdateApprovalRuleRequest 更新审批规则请求
type UpdateApprovalRuleRequest struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Conditions  map[string]any `json:"conditions"`
	Actions     map[string]any `json:"actions"`
	Priority    *int           `json:"priority"`
	IsActive    *bool          `json:"isActive"`
}

// ========== 工作流模板 ==========

// CreateWorkflowTemplateRequest 创建工作流模板请求
type CreateWorkflowTemplateRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Definition  map[string]any `json:"definition" binding:"required"`
	IsPublic    bool           `json:"isPublic"`
}

// UpdateWorkflowTemplateRequest 更新工作流模板请求
type UpdateWorkflowTemplateRequest struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Category    *string        `json:"category"`
	Definition  map[string]any `json:"definition"`
	IsPublic    *bool          `json:"isPublic"`
}
