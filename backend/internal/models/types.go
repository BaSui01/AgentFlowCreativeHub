package models

import "time"

// ============================================================================
// 统计类型
// ============================================================================

// ModelStats 模型使用统计
type ModelStats struct {
	ModelID        string    `json:"modelId"`
	TotalCalls     int64     `json:"totalCalls"`
	TotalTokens    int64     `json:"totalTokens"`
	TotalCost      float64   `json:"totalCost"`
	AvgLatencyMs   float64   `json:"avgLatencyMs"`
	SuccessRate    float64   `json:"successRate"`
	LastCalledAt   time.Time `json:"lastCalledAt"`
}

// ============================================================================
// Credential相关请求类型
// ============================================================================

// CreateCredentialRequest 创建凭证请求
type CreateCredentialRequest struct {
	TenantID     string            `json:"tenantId" binding:"required"`
	ModelID      string            `json:"modelId" binding:"required"`
	Provider     string            `json:"provider" binding:"required"`
	Name         string            `json:"name" binding:"required"`
	APIKey       string            `json:"apiKey" binding:"required"`
	BaseURL      string            `json:"baseUrl"`
	ExtraHeaders map[string]string `json:"extraHeaders"`
}

// UpdateCredentialRequest 更新凭证请求
type UpdateCredentialRequest struct {
	Name         string            `json:"name"`
	APIKey       string            `json:"apiKey"`
	BaseURL      string            `json:"baseUrl"`
	ExtraHeaders map[string]string `json:"extraHeaders"`
	Status       string            `json:"status"`
}

// ============================================================================
// Session相关请求类型
// ============================================================================

// CreateSessionRequest 创建会话请求
type CreateSessionRequest struct {
	TenantID    string         `json:"tenantId" binding:"required"`
	UserID      string         `json:"userId" binding:"required"`
	Title       string         `json:"title"`
	ModelID     string         `json:"modelId"`
	SystemPrompt string        `json:"systemPrompt"`
	Metadata    map[string]any `json:"metadata"`
}

// UpdateSessionRequest 更新会话请求
type UpdateSessionRequest struct {
	Title        string         `json:"title"`
	SystemPrompt string         `json:"systemPrompt"`
	Metadata     map[string]any `json:"metadata"`
	Status       string         `json:"status"`
}

// Message 消息类型
type Message struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId"`
	Role      string         `json:"role"` // user, assistant, system
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"createdAt"`
}

// ============================================================================
// AuditLog相关请求类型
// ============================================================================

// ListAuditLogsRequest 查询审计日志请求
type ListAuditLogsRequest struct {
	TenantID   string     `json:"tenantId" binding:"required"`
	UserID     string     `json:"userId"`
	Action     string     `json:"action"`
	Resource   string     `json:"resource"`
	StartTime  *time.Time `json:"startTime"`
	EndTime    *time.Time `json:"endTime"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}

// ============================================================================
// KnowledgeBase相关请求类型
// ============================================================================

// CreateKnowledgeBaseRequest 创建知识库请求
type CreateKnowledgeBaseRequest struct {
	TenantID      string            `json:"tenantId" binding:"required"`
	Name          string            `json:"name" binding:"required"`
	Description   string            `json:"description"`
	EmbeddingModel string           `json:"embeddingModel" binding:"required"`
	ChunkSize     int               `json:"chunkSize"`
	ChunkOverlap  int               `json:"chunkOverlap"`
	Metadata      map[string]string `json:"metadata"`
}

// UpdateKnowledgeBaseRequest 更新知识库请求
type UpdateKnowledgeBaseRequest struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	ChunkSize     int               `json:"chunkSize"`
	ChunkOverlap  int               `json:"chunkOverlap"`
	Metadata      map[string]string `json:"metadata"`
	Status        string            `json:"status"`
}

// ============================================================================
// Document相关请求类型
// ============================================================================

// CreateDocumentRequest 创建文档请求
type CreateDocumentRequest struct {
	TenantID        string            `json:"tenantId" binding:"required"`
	KnowledgeBaseID string            `json:"knowledgeBaseId" binding:"required"`
	Title           string            `json:"title" binding:"required"`
	Content         string            `json:"content"`
	ContentType     string            `json:"contentType"`  // MIME type
	Type            string            `json:"type"`         // text, file
	Source          string            `json:"source"`       // 来源
	SourceType      string            `json:"sourceType"`   // file, manual, url
	FileSize        int64             `json:"fileSize"`     // 文件大小
	CreatedBy       string            `json:"createdBy"`    // 创建者
	Metadata        map[string]string `json:"metadata"`
}

// UpdateDocumentRequest 更新文档请求
type UpdateDocumentRequest struct {
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata"`
	Status   string            `json:"status"`
}
