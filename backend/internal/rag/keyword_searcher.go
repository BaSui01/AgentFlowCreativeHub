package rag

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// PGKeywordSearcher PostgreSQL 全文搜索实现 (BM25)
type PGKeywordSearcher struct {
	db *gorm.DB
}

// NewPGKeywordSearcher 创建 PostgreSQL 关键词检索器
func NewPGKeywordSearcher(db *gorm.DB) *PGKeywordSearcher {
	return &PGKeywordSearcher{db: db}
}

// SearchKeywords 使用 PostgreSQL 全文搜索
func (s *PGKeywordSearcher) SearchKeywords(ctx context.Context, kbID string, query string, topK int) ([]*SearchResult, error) {
	if topK <= 0 {
		topK = 10
	}

	// 将查询转换为 tsquery 格式
	tsQuery := s.buildTSQuery(query)

	var results []*SearchResult

	// 使用 PostgreSQL ts_rank_cd 进行 BM25 风格的排序
	sql := `
		SELECT 
			chunk_id,
			document_id,
			content,
			ts_rank_cd(to_tsvector('simple', content), to_tsquery('simple', ?)) as score
		FROM knowledge_vectors
		WHERE knowledge_base_id = ?
			AND to_tsvector('simple', content) @@ to_tsquery('simple', ?)
		ORDER BY score DESC
		LIMIT ?
	`

	rows, err := s.db.WithContext(ctx).Raw(sql, tsQuery, kbID, tsQuery, topK).Rows()
	if err != nil {
		return nil, fmt.Errorf("关键词搜索失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ChunkID, &r.DocumentID, &r.Content, &r.Score); err != nil {
			continue
		}
		results = append(results, &r)
	}

	return results, nil
}

// buildTSQuery 构建 tsquery 查询
func (s *PGKeywordSearcher) buildTSQuery(query string) string {
	// 分词并用 & 连接 (AND 语义)
	words := strings.Fields(query)
	if len(words) == 0 {
		return query
	}

	// 转义特殊字符
	escaped := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.ReplaceAll(w, "'", "''")
		w = strings.ReplaceAll(w, "\\", "\\\\")
		if w != "" {
			escaped = append(escaped, w)
		}
	}

	// 用 | (OR) 连接，提高召回率
	return strings.Join(escaped, " | ")
}

// EnsureFullTextIndex 确保全文索引存在
func (s *PGKeywordSearcher) EnsureFullTextIndex(ctx context.Context) error {
	// 创建 GIN 索引用于全文搜索
	sql := `
		CREATE INDEX IF NOT EXISTS idx_knowledge_vectors_content_fts 
		ON knowledge_vectors 
		USING GIN (to_tsvector('simple', content))
	`
	return s.db.WithContext(ctx).Exec(sql).Error
}

// SearchKeywordsWithHighlight 带高亮的关键词搜索
func (s *PGKeywordSearcher) SearchKeywordsWithHighlight(ctx context.Context, kbID string, query string, topK int) ([]*SearchResult, error) {
	if topK <= 0 {
		topK = 10
	}

	tsQuery := s.buildTSQuery(query)

	var results []*SearchResult

	sql := `
		SELECT 
			chunk_id,
			document_id,
			ts_headline('simple', content, to_tsquery('simple', ?), 
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=20') as content,
			ts_rank_cd(to_tsvector('simple', content), to_tsquery('simple', ?)) as score
		FROM knowledge_vectors
		WHERE knowledge_base_id = ?
			AND to_tsvector('simple', content) @@ to_tsquery('simple', ?)
		ORDER BY score DESC
		LIMIT ?
	`

	rows, err := s.db.WithContext(ctx).Raw(sql, tsQuery, tsQuery, kbID, tsQuery, topK).Rows()
	if err != nil {
		return nil, fmt.Errorf("关键词搜索失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ChunkID, &r.DocumentID, &r.Content, &r.Score); err != nil {
			continue
		}
		results = append(results, &r)
	}

	return results, nil
}
