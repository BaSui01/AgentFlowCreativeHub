package runtime

import (
	"context"
	"fmt"
	"time"

	"backend/internal/rag"

	"github.com/google/uuid"
)

// MemoryService Agent 记忆服务接口
type MemoryService interface {
	// Add 添加记忆
	Add(ctx context.Context, kbID string, content string, metadata map[string]any) error
	// Search 检索记忆
	Search(ctx context.Context, kbID string, query string, limit int) ([]MemoryResult, error)
}

// MemoryResult 记忆检索结果
type MemoryResult struct {
	Content   string
	Score     float64
	Timestamp time.Time
	Metadata  map[string]any
}

// RAGMemoryService 基于 RAG 的记忆实现
type RAGMemoryService struct {
	vectorStore       rag.VectorStore
	embeddingProvider rag.EmbeddingProvider
}

// NewRAGMemoryService 创建 RAG 记忆服务
func NewRAGMemoryService(store rag.VectorStore, provider rag.EmbeddingProvider) *RAGMemoryService {
	return &RAGMemoryService{
		vectorStore:       store,
		embeddingProvider: provider,
	}
}

func (m *RAGMemoryService) Add(ctx context.Context, kbID string, content string, metadata map[string]any) error {
	// 1. 生成 Embedding
	embeddings, err := m.embeddingProvider.EmbedBatch(ctx, []string{content})
	if err != nil {
		return fmt.Errorf("embedding generation failed: %w", err)
	}

	if len(embeddings) == 0 {
		return fmt.Errorf("no embeddings generated")
	}

	// 2. 构造 Vector
	chunkID := uuid.New().String()
	docID := uuid.New().String() // 虚拟文档 ID

	vec := &rag.Vector{
		ChunkID:           chunkID,
		KnowledgeBaseID:   kbID,
		DocumentID:        docID,
		Content:           content,
		ChunkIndex:        0,
		Embedding:         embeddings[0],
		Metadata:          metadata,
		TokenCount:        len(content) / 4,
		EmbeddingModel:    m.embeddingProvider.GetModel(),
		EmbeddingProvider: m.embeddingProvider.GetProviderName(),
	}

	// 3. 存储
	return m.vectorStore.AddVectors(ctx, []*rag.Vector{vec})
}

func (m *RAGMemoryService) Search(ctx context.Context, kbID string, query string, limit int) ([]MemoryResult, error) {
	// 1. 生成查询向量
	embeddings, err := m.embeddingProvider.EmbedBatch(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("query embedding generation failed: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no query embeddings generated")
	}

	// 2. 搜索
	searchResults, err := m.vectorStore.Search(ctx, kbID, embeddings[0], limit)
	if err != nil {
		return nil, err
	}

	var results []MemoryResult
	for _, r := range searchResults {
		results = append(results, MemoryResult{
			Content:  r.Content,
			Score:    r.Score,
			Metadata: r.Metadata,
		})
	}
	return results, nil
}
