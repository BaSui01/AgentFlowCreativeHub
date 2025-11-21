package rag

import "context"

// Vector 描述一条需要写入向量存储的知识片段。
type Vector struct {
	ChunkID           string
	KnowledgeBaseID   string
	DocumentID        string
	TenantID          string
	Content           string
	ContentHash       string
	ChunkIndex        int
	StartOffset       int
	EndOffset         int
	TokenCount        int
	Embedding         []float32
	EmbeddingModel    string
	EmbeddingProvider string
	Metadata          map[string]any
}

// SearchResult 描述一次相似度检索的返回结果。
type SearchResult struct {
	ChunkID         string                 `json:"chunk_id"`
	KnowledgeBaseID string                 `json:"knowledge_base_id"`
	DocumentID      string                 `json:"document_id"`
	Content         string                 `json:"content"`
	ChunkIndex      int                    `json:"chunk_index"`
	Similarity      float64                `json:"similarity"`
	Score           float64                `json:"score"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// VectorStoreStats 记录某个知识库在向量存储中的统计信息。
type VectorStoreStats struct {
	TotalVectors   int64
	TotalDocuments int64
}

// VectorStore 抽象向量写入、检索与删除功能，可由不同后端实现（pgvector、Qdrant 等）。
type VectorStore interface {
	AddVectors(ctx context.Context, vectors []*Vector) error
	Search(ctx context.Context, knowledgeBaseID string, queryVector []float32, topK int) ([]*SearchResult, error)
	DeleteVectors(ctx context.Context, chunkIDs []string) error
	DeleteByDocument(ctx context.Context, knowledgeBaseID, documentID string) error
	DeleteByKnowledgeBase(ctx context.Context, knowledgeBaseID string) error
	GetStats(ctx context.Context, knowledgeBaseID string) (*VectorStoreStats, error)
}
