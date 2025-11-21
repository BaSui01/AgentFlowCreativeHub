package rag

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// PGVectorStore 基于PostgreSQL pgvector扩展的向量存储实现
type PGVectorStore struct {
	db *gorm.DB
}

// NewPGVectorStore 创建新的pgvector存储实例
func NewPGVectorStore(db *gorm.DB) (*PGVectorStore, error) {
	store := &PGVectorStore{
		db: db,
	}

	// 确保pgvector扩展已启用
	if err := store.ensureExtension(); err != nil {
		return nil, fmt.Errorf("确保pgvector扩展失败: %w", err)
	}

	return store, nil
}

// ensureExtension 确保pgvector扩展已启用
func (s *PGVectorStore) ensureExtension() error {
	return s.db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error
}

// AddVectors 添加向量到存储
// vectors: 要添加的向量列表
// 返回: 成功添加的向量数量和错误
func (s *PGVectorStore) AddVectors(ctx context.Context, vectors []*Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	// 使用事务批量插入
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, vec := range vectors {
			// 创建知识片段记录
			chunk := &KnowledgeChunk{
				ID:                vec.ChunkID,
				KnowledgeBaseID:   vec.KnowledgeBaseID,
				DocumentID:        vec.DocumentID,
				Content:           vec.Content,
				ContentHash:       vec.ContentHash,
				ChunkIndex:        vec.ChunkIndex,
				StartOffset:       vec.StartOffset,
				EndOffset:         vec.EndOffset,
				TokenCount:        vec.TokenCount,
				Embedding:         vectorToString(vec.Embedding), // 使用转换函数
				EmbeddingModel:    vec.EmbeddingModel,
				EmbeddingProvider: vec.EmbeddingProvider,
				MetadataRaw:       vec.Metadata,
			}

			if err := tx.Create(chunk).Error; err != nil {
				return fmt.Errorf("创建知识片段失败: %w", err)
			}
		}
		return nil
	})
}

// Search 执行向量相似度搜索
// ctx: 上下文
// kbID: 知识库ID
// queryVector: 查询向量
// topK: 返回结果数量
// 返回: 搜索结果和错误
func (s *PGVectorStore) Search(ctx context.Context, kbID string, queryVector []float32, topK int) ([]*SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("查询向量不能为空")
	}

	if topK <= 0 {
		topK = 5
	}

	// 构建向量字符串 '[0.1, 0.2, ...]'
	vectorStr := vectorToString(queryVector)

	// 使用余弦相似度搜索
	// 1 - (embedding <=> query_vector) 计算余弦相似度
	// <=> 是pgvector的余弦距离操作符
	query := `
		SELECT 
			id,
			knowledge_base_id,
			document_id,
			content,
			chunk_index,
			metadata,
			1 - (embedding <=> $1::vector) AS similarity
		FROM knowledge_chunks
		WHERE knowledge_base_id = $2
			AND deleted_at IS NULL
		ORDER BY embedding <=> $1::vector
		LIMIT $3
	`

	var results []struct {
		ID              string                 `gorm:"column:id"`
		KnowledgeBaseID string                 `gorm:"column:knowledge_base_id"`
		DocumentID      string                 `gorm:"column:document_id"`
		Content         string                 `gorm:"column:content"`
		ChunkIndex      int                    `gorm:"column:chunk_index"`
		Metadata        map[string]interface{} `gorm:"column:metadata;type:jsonb"`
		Similarity      float64                `gorm:"column:similarity"`
	}

	if err := s.db.WithContext(ctx).Raw(query, vectorStr, kbID, topK).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	// 转换结果
	searchResults := make([]*SearchResult, 0, len(results))
	for _, r := range results {
		searchResults = append(searchResults, &SearchResult{
			ChunkID:         r.ID,
			KnowledgeBaseID: r.KnowledgeBaseID,
			DocumentID:      r.DocumentID,
			Content:         r.Content,
			ChunkIndex:      r.ChunkIndex,
			Similarity:      r.Similarity,
			Score:           r.Similarity,
			Metadata:        r.Metadata,
		})
	}

	return searchResults, nil
}

// DeleteVectors 删除指定向量
func (s *PGVectorStore) DeleteVectors(ctx context.Context, chunkIDs []string) error {
	if len(chunkIDs) == 0 {
		return nil
	}

	// 软删除
	return s.db.WithContext(ctx).
		Model(&KnowledgeChunk{}).
		Where("id IN ?", chunkIDs).
		Update("deleted_at", gorm.Expr("NOW()")).
		Error
}

// DeleteByDocument 删除指定文档的所有向量
func (s *PGVectorStore) DeleteByDocument(ctx context.Context, knowledgeBaseID, documentID string) error {
	return s.db.WithContext(ctx).
		Model(&KnowledgeChunk{}).
		Where("document_id = ?", documentID).
		Update("deleted_at", gorm.Expr("NOW()")).
		Error
}

// DeleteByKnowledgeBase 删除指定知识库的所有向量
func (s *PGVectorStore) DeleteByKnowledgeBase(ctx context.Context, kbID string) error {
	return s.db.WithContext(ctx).
		Model(&KnowledgeChunk{}).
		Where("knowledge_base_id = ?", kbID).
		Update("deleted_at", gorm.Expr("NOW()")).
		Error
}

// GetStats 获取向量存储统计信息
func (s *PGVectorStore) GetStats(ctx context.Context, kbID string) (*VectorStoreStats, error) {
	var stats VectorStoreStats

	// 查询向量数量
	if err := s.db.WithContext(ctx).
		Model(&KnowledgeChunk{}).
		Where("knowledge_base_id = ? AND deleted_at IS NULL", kbID).
		Count(&stats.TotalVectors).Error; err != nil {
		return nil, fmt.Errorf("查询向量数量失败: %w", err)
	}

	// 查询文档数量
	if err := s.db.WithContext(ctx).
		Model(&KnowledgeChunk{}).
		Where("knowledge_base_id = ? AND deleted_at IS NULL", kbID).
		Distinct("document_id").
		Count(&stats.TotalDocuments).Error; err != nil {
		return nil, fmt.Errorf("查询文档数量失败: %w", err)
	}
	return &stats, nil
}

// vectorToString 将向量转换为PostgreSQL向量字符串格式
func vectorToString(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}

	result := "["
	for i, v := range vec {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%f", v)
	}
	result += "]"
	return result
}
