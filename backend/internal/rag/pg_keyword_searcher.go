package rag

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// PostgresKeywordSearcher 基于 PostgreSQL tsvector 的关键词检索器
type PostgresKeywordSearcher struct {
	db *gorm.DB
}

// NewPostgresKeywordSearcher 创建 Postgres 关键词检索器
func NewPostgresKeywordSearcher(db *gorm.DB) *PostgresKeywordSearcher {
	return &PostgresKeywordSearcher{db: db}
}

// SearchKeywords 执行关键词检索
func (s *PostgresKeywordSearcher) SearchKeywords(ctx context.Context, kbID string, query string, topK int) ([]*SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return []*SearchResult{}, nil
	}

	// 将查询转换为 tsquery 格式 (简单的 OR 连接)
	// "AI Agent" -> "AI | Agent"
	terms := strings.Fields(query)
	tsQuery := strings.Join(terms, " | ")

	var results []*SearchResult

	// 使用 gorm 执行原生 SQL 查询
	// 注意：我们需要在 knowledge_chunks 表上创建 tsvector 索引
	// 这里假设 knowledge_chunks 表有 content 字段
	// ts_rank_cd 用于计算相关性评分

	// 为了性能，建议在表中增加一个 tsvector 类型的列 `content_vector` 并建立 GIN 索引
	// 但为了简化，这里演示直接对 content 进行转换 (性能较低，适用于中小规模)

	err := s.db.WithContext(ctx).Raw(`
		SELECT 
			id as chunk_id,
			knowledge_base_id,
			document_id,
			content,
			chunk_index,
			ts_rank_cd(to_tsvector('chinese', content), plainto_tsquery('chinese', ?)) as score,
			metadata_raw as metadata
		FROM knowledge_chunks
		WHERE 
			knowledge_base_id = ? AND
			deleted_at IS NULL AND
			to_tsvector('chinese', content) @@ plainto_tsquery('chinese', ?)
		ORDER BY score DESC
		LIMIT ?
	`, tsQuery, kbID, tsQuery, topK).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("keyword search failed: %w", err)
	}

	// 归一化分数 (0-1范围大致映射)
	// BM25/ts_rank 分数没有上限，这里简单处理，或者直接返回原始分
	// RRF 融合时只看排名，不看绝对分数，所以直接返回即可

	return results, nil
}
