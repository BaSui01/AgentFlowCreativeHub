package tasks

// Task Types
const (
	TypeProcessDocument  = "rag:process_document"
	TypeExecuteWorkflow  = "workflow:execute"
)

// ProcessDocumentPayload RAG文档处理任务载荷
type ProcessDocumentPayload struct {
	DocumentID string `json:"document_id"`
}

// ExecuteWorkflowPayload 工作流执行任务载荷
type ExecuteWorkflowPayload struct {
	ExecutionID string         `json:"execution_id"`
	WorkflowID  string         `json:"workflow_id"`
	TenantID    string         `json:"tenant_id"`
	UserID      string         `json:"user_id"`
	Input       map[string]any `json:"input"`
}
