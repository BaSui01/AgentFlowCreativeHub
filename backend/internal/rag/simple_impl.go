package rag

import (
	"context"
	"sort"
	"strings"
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
	
	// 为了演示，我们这里返回空结果，或者假设底层存储支持 Keyword
	// 如果使用 pgvector，可以使用 tsvector 及其相关操作来实现
	
	return []*SearchResult{}, nil
}

// SimpleReranker 简单的重排序实现 (Mock)
// 在生产环境中应替换为 Cohere Rerank 或 BGE-Reranker
type SimpleReranker struct {}

func NewSimpleReranker() *SimpleReranker {
	return &SimpleReranker{}
}

func (r *SimpleReranker) Rerank(ctx context.Context, query string, documents []*SearchResult, topK int) ([]*SearchResult, error) {
	// 简单的 Mock 实现：根据 Content 长度排序（仅作演示）
	// 真实场景应调用 Rerank 模型 API
	
	// 复制切片以避免修改原始数据
	reranked := make([]*SearchResult, len(documents))
	copy(reranked, documents)
	
	// 模拟重排序：这里简单地假设包含 Query 关键词的排在前面
	queryTerms := strings.Fields(strings.ToLower(query))
	
	sort.Slice(reranked, func(i, j int) bool {
		scoreI := calculateSimpleScore(reranked[i].Content, queryTerms)
		scoreJ := calculateSimpleScore(reranked[j].Content, queryTerms)
		
		// 如果分数相同，保留原始顺序（稳定排序）
		// 但这里 sort.Slice 不保证稳定
		if scoreI == scoreJ {
			return reranked[i].Score > reranked[j].Score // 原始分数
		}
		return scoreI > scoreJ
	})
	
	if len(reranked) > topK {
		reranked = reranked[:topK]
	}
	
	return reranked, nil
}

func calculateSimpleScore(content string, terms []string) float64 {
	contentLower := strings.ToLower(content)
	score := 0.0
	for _, term := range terms {
		if strings.Contains(contentLower, term) {
			score += 1.0
		}
	}
	return score
}
