package workflow

import "context"

// ============================================================================
// WorkflowService 接口定义
// ============================================================================

// WorkflowServiceInterface 工作流管理服务接口
type WorkflowServiceInterface interface {
	// CreateWorkflow 创建工作流
	CreateWorkflow(ctx context.Context, req *CreateWorkflowRequest) (*Workflow, error)

	// GetWorkflow 查询工作流
	GetWorkflow(ctx context.Context, tenantID, workflowID string) (*Workflow, error)

	// ListWorkflows 查询工作流列表
	ListWorkflows(ctx context.Context, tenantID string, page, pageSize int) ([]*Workflow, int64, error)

	// UpdateWorkflow 更新工作流
	UpdateWorkflow(ctx context.Context, tenantID, workflowID string, req *UpdateWorkflowRequest) (*Workflow, error)

	// DeleteWorkflow 删除工作流
	DeleteWorkflow(ctx context.Context, tenantID, workflowID, operatorID string) error

	// ValidateWorkflow 验证工作流定义
	ValidateWorkflow(ctx context.Context, definition map[string]any) error

	// CloneWorkflow 克隆工作流
	CloneWorkflow(ctx context.Context, tenantID, workflowID, newName string) (*Workflow, error)

	// ExportWorkflow 导出工作流定义
	ExportWorkflow(ctx context.Context, tenantID, workflowID string) ([]byte, error)

	// ImportWorkflow 导入工作流定义
	ImportWorkflow(ctx context.Context, tenantID string, data []byte) (*Workflow, error)

	// GetWorkflowStats 获取工作流统计信息
	GetWorkflowStats(ctx context.Context, tenantID, workflowID string) (*WorkflowStats, error)
}

// WorkflowStats 工作流统计信息
type WorkflowStats struct {
	TotalExecutions     int64   `json:"total_executions"`
	SuccessfulExecutions int64   `json:"successful_executions"`
	FailedExecutions    int64   `json:"failed_executions"`
	AverageExecutionTime float64 `json:"average_execution_time"` // 秒
	LastExecutionAt     string  `json:"last_execution_at"`
}
