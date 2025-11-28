package rag

import (
	"context"
)

// SimpleKeywordSearcher 简单的内存关键词检索实现 (Mock)
// 在生产环境中应替换为 Elasticsearch 或 Bleve
type SimpleKeywordSearcher struct {
	vectorStore VectorStore
}

func NewSimpleKeywordSearcher(vs VectorStore) *SimpleKeywordSearcher {
	return &SimpleKeywordSearcher{vectorStore: vs}
}

func (s *SimpleKeywordSearcher) SearchKeywords(ctx context.Context, kbID string, query string, topK int) ([]*SearchResult, error) {
	// 注意：这只是一个 Mock 实现，实际上 VectorStore 接口不支持纯文本搜索
	// 在真实场景中，这里会调用 ES 的 Search API
	// 使用 PGKeywordSearcher (keyword_searcher.go) 提供 PostgreSQL 全文搜索实现
	return []*SearchResult{}, nil
}
