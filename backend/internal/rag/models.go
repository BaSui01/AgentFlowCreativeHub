package rag

import "time"

// KnowledgeBase represents a logical knowledge base within a tenant.
type KnowledgeBase struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;not null;index"`

	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`

	// 可见性
	VisibilityScope string `json:"visibilityScope" gorm:"size:50;not null;default:tenant"`

	// 配置
	DefaultEmbeddingModel string `json:"defaultEmbeddingModel" gorm:"size:100"`

	// 统计信息
	DocumentCount int   `json:"documentCount" gorm:"default:0"`
	ChunkCount    int   `json:"chunkCount" gorm:"default:0"`
	TotalSize     int64 `json:"totalSize" gorm:"default:0"` // 字节

	// 状态
	Status string `json:"status" gorm:"size:50;not null;default:active"`

	// 创建人/更新人
	CreatedBy string `json:"createdBy" gorm:"size:100"`
	UpdatedBy string `json:"updatedBy" gorm:"size:100"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}

// KnowledgeDocument represents a single document inside a knowledge base.
type KnowledgeDocument struct {
	ID              string `json:"id" gorm:"primaryKey;type:uuid"`
	KnowledgeBaseID string `json:"knowledgeBaseId" gorm:"type:uuid;not null;index"`
	TenantID        string `json:"tenantId" gorm:"type:uuid;not null;index"`

	// 文档信息
	Title    string `json:"title" gorm:"size:500"`
	FileName string `json:"fileName" gorm:"size:500"`
	Author   string `json:"author" gorm:"size:255"`

	// 文件信息
	SourceType   string `json:"sourceType" gorm:"size:50;not null"`   // pdf, markdown, txt, url
	SourceURI    string `json:"sourceUri" gorm:"type:text;not null"`
	ContentType  string `json:"contentType" gorm:"size:100"`
	Content      string `json:"content" gorm:"type:text"`
	FileSize     int64  `json:"fileSize"`
	FileMimeType string `json:"fileMimeType" gorm:"size:100"`
	FileHash     string `json:"fileHash" gorm:"size:64;index"` // SHA-256

	// 版本与状态
	Version string `json:"version" gorm:"size:50"`
	Status  string `json:"status" gorm:"size:50;not null;default:pending"` // pending, processing, indexed, failed

	// 处理信息
	UploadedBy   string     `json:"uploadedBy" gorm:"size:100"`
	ProcessedAt  *time.Time `json:"processedAt"`
	ErrorMessage string     `json:"errorMessage" gorm:"type:text"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}

// KnowledgeChunk is a content chunk derived from a document.
type KnowledgeChunk struct {
	ID              string `json:"id" gorm:"primaryKey;type:uuid"`
	DocumentID      string `json:"documentId" gorm:"type:uuid;not null;index"`
	KnowledgeBaseID string `json:"knowledgeBaseId" gorm:"type:uuid;not null;index"`

	ChunkIndex  int    `json:"chunkIndex" gorm:"not null"`
	Content     string `json:"content" gorm:"type:text;not null"`
	ContentHash string `json:"contentHash" gorm:"size:64;index"`
	TokenCount  int    `json:"tokenCount" gorm:"default:0"`

	// 位置信息
	StartOffset int `json:"startOffset" gorm:"column:start_pos"`
	EndOffset   int `json:"endOffset" gorm:"column:end_pos"`

	// 向量（PostgreSQL pgvector 类型）
	Embedding         string         `json:"-" gorm:"type:vector(1536)"` // OpenAI embedding 维度
	EmbeddingModel    string         `json:"embeddingModel" gorm:"size:100"`
	EmbeddingProvider string         `json:"embeddingProvider" gorm:"size:50"`
	MetadataRaw       map[string]any `json:"metadata" gorm:"type:jsonb;serializer:json"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
}

// RagQueryLog captures a single RAG query for observability and analysis.
type RagQueryLog struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenantId"`
	UserID            string    `json:"userId"`
	KnowledgeBaseIDs  []string  `json:"knowledgeBaseIds"`
	TopK              int       `json:"topK"`
	ScoreThreshold    *float64  `json:"scoreThreshold,omitempty"`
	RetrievedCount    int       `json:"retrievedCount"`
	AverageScore      *float64  `json:"avgScore,omitempty"`
	LatencyMs         int       `json:"latencyMs"`
	TraceID           string    `json:"traceId"`
	SessionID         string    `json:"sessionId"`
	CreatedAt         time.Time `json:"createdAt"`
}

// Tool represents a logical tool that can be invoked by agents.
type Tool struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID string `json:"tenantId" gorm:"type:uuid;index"` // 空表示系统工具

	Name        string `json:"name" gorm:"size:255;not null"`
	Category    string `json:"category" gorm:"size:64;not null"`
	Description string `json:"description" gorm:"type:text"`

	InputSchema      map[string]any `json:"inputSchema" gorm:"type:jsonb;not null;serializer:json"`
	OutputSchema     map[string]any `json:"outputSchema" gorm:"type:jsonb;not null;serializer:json"`
	SensitivityLevel string         `json:"sensitivityLevel" gorm:"size:32;not null;default:normal"`
	Status           string         `json:"status" gorm:"size:32;not null;default:active"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
}

// ToolVersion describes a concrete implementation of a tool.
type ToolVersion struct {
	ID        string         `json:"id"`
	ToolID    string         `json:"toolId"`
	Version   string         `json:"version"`
	ImplType  string         `json:"implType"`
	ImplRef   string         `json:"implRef"`
	ConfigRaw map[string]any `json:"config"`
	CreatedAt time.Time      `json:"createdAt"`
}

// ToolCallLog stores a single tool invocation record.
type ToolCallLog struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenantId"`
	UserID         string    `json:"userId"`
	ToolID         string    `json:"toolId"`
	ToolVersionID  string    `json:"toolVersionId"`
	Status         string    `json:"status"`
	LatencyMs      int       `json:"latencyMs"`
	ErrorCode      string    `json:"errorCode"`
	ErrorMessage   string    `json:"errorMessage"`
	TraceID        string    `json:"traceId"`
	CreatedAt      time.Time `json:"createdAt"`
}
