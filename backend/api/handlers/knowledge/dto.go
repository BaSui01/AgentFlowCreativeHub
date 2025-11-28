package knowledge

import (
	"backend/api/handlers/common"
)

// ========== 知识库 ==========

// CreateKBRequest 创建知识库请求
type CreateKBRequest struct {
	Name                  string                 `json:"name" binding:"required,min=1,max=200"`
	Description           string                 `json:"description"`
	Type                  string                 `json:"type" binding:"required,oneof=document url api database"`
	VisibilityScope       string                 `json:"visibilityScope"`
	DefaultEmbeddingModel string                 `json:"defaultEmbeddingModel"`
	Config                map[string]interface{} `json:"config"`
	Metadata              map[string]interface{} `json:"metadata"`
}

// UpdateKBRequest 更新知识库请求
type UpdateKBRequest struct {
	Name                  *string                `json:"name" binding:"omitempty,min=1,max=200"`
	Description           *string                `json:"description"`
	Status                *string                `json:"status" binding:"omitempty,oneof=active archived"`
	VisibilityScope       *string                `json:"visibilityScope"`
	DefaultEmbeddingModel *string                `json:"defaultEmbeddingModel"`
	Config                map[string]interface{} `json:"config"`
	Metadata              map[string]interface{} `json:"metadata"`
}

// KBListResponse 知识库列表响应
type KBListResponse struct {
	Items      interface{}           `json:"items"`
	Pagination common.PaginationMeta `json:"pagination"`
}

// ========== 文档 ==========

// UpdateDocumentRequest 更新文档请求
type UpdateDocumentRequest struct {
	Title    *string                `json:"title"`
	Content  *string                `json:"content"`
	Source   *string                `json:"source"`
	Status   *string                `json:"status"`
	Metadata map[string]interface{} `json:"metadata"`
}

// DocumentListResponse 文档列表响应
type DocumentListResponse struct {
	Documents  interface{}           `json:"documents"`
	Pagination common.PaginationMeta `json:"pagination"`
}

// ========== 检索 ==========

// SearchResultItem 检索结果项
type SearchResultItem struct {
	ChunkID    string  `json:"chunkId"`
	DocumentID string  `json:"documentId"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Metadata   any     `json:"metadata,omitempty"`
}

// SearchResponse 检索响应
type SearchResponse struct {
	Results     []SearchResultItem `json:"results"`
	Total       int                `json:"total"`
	Query       string             `json:"query"`
	LatencyMs   int64              `json:"latencyMs"`
}

// ========== 分块 ==========

// ChunkListResponse 分块列表响应
type ChunkListResponse struct {
	Chunks     interface{}           `json:"chunks"`
	Pagination common.PaginationMeta `json:"pagination"`
}
