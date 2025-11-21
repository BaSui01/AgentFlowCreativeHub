package audit

import "time"

// TokenUsage 记录一次 Token 消耗
type TokenUsage struct {
	ID            string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID      string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID        string    `json:"userId" gorm:"type:uuid;index"`
	
	// 上下文关联
	WorkflowID    string    `json:"workflowId" gorm:"type:uuid;index"`
	ExecutionID   string    `json:"executionId" gorm:"type:uuid;index"`
	StepID        string    `json:"stepId" gorm:"size:100"`
	AgentID       string    `json:"agentId" gorm:"type:uuid"`
	
	// 模型信息
	Model         string    `json:"model" gorm:"size:100;not null"`
	Provider      string    `json:"provider" gorm:"size:50;not null"`
	
	// Token 统计
	PromptTokens  int       `json:"promptTokens" gorm:"not null"`
	CompletionTokens int    `json:"completionTokens" gorm:"not null"`
	TotalTokens   int       `json:"totalTokens" gorm:"not null"`
	
	// 成本（预估，单位：美元）
	EstimatedCost float64   `json:"estimatedCost" gorm:"type:decimal(10,6)"`
	
	// 时间戳
	CreatedAt     time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
}

// AuditService 审计服务接口
type AuditService interface {
	RecordTokenUsage(usage *TokenUsage) error
}
