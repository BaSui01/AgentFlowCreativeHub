package rag

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"
)

// SimpleReranker 简单的重排序器 (基于关键词重叠度 + 位置权重)
type SimpleReranker struct {
	// 权重配置
	KeywordWeight  float64 // 关键词匹配权重
	PositionWeight float64 // 位置权重 (关键词在文档中的位置)
	LengthPenalty  float64 // 长度惩罚因子
}

// NewSimpleReranker 创建简单重排序器
func NewSimpleReranker() *SimpleReranker {
	return &SimpleReranker{
		KeywordWeight:  0.6,
		PositionWeight: 0.3,
		LengthPenalty:  0.1,
	}
}

// Rerank 对搜索结果进行重排序
func (r *SimpleReranker) Rerank(ctx context.Context, query string, documents []*SearchResult, topK int) ([]*SearchResult, error) {
	if len(documents) == 0 {
		return documents, nil
	}

	// 提取查询关键词
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return documents[:min(topK, len(documents))], nil
	}

	// 计算每个文档的重排序分数
	type scoredDoc struct {
		doc   *SearchResult
		score float64
	}

	scored := make([]scoredDoc, len(documents))
	for i, doc := range documents {
		score := r.computeScore(queryTerms, doc.Content, doc.Score)
		scored[i] = scoredDoc{doc: doc, score: score}
	}

	// 按分数降序排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// 返回 TopK 结果
	result := make([]*SearchResult, 0, topK)
	for i := 0; i < len(scored) && i < topK; i++ {
		doc := scored[i].doc
		doc.Score = scored[i].score // 更新分数
		result = append(result, doc)
	}

	return result, nil
}

// computeScore 计算综合分数
func (r *SimpleReranker) computeScore(queryTerms []string, content string, originalScore float64) float64 {
	docTerms := tokenize(content)
	if len(docTerms) == 0 {
		return originalScore * 0.5
	}

	// 1. 关键词匹配分数 (Jaccard + TF)
	keywordScore := r.keywordMatchScore(queryTerms, docTerms)

	// 2. 位置分数 (关键词出现位置越靠前越好)
	positionScore := r.positionScore(queryTerms, content)

	// 3. 长度惩罚 (太长或太短都不好)
	lengthScore := r.lengthScore(len(docTerms))

	// 综合分数 = 原始分数 * 0.3 + 重排序分数 * 0.7
	rerankScore := r.KeywordWeight*keywordScore +
		r.PositionWeight*positionScore +
		r.LengthPenalty*lengthScore

	return originalScore*0.3 + rerankScore*0.7
}

// keywordMatchScore 计算关键词匹配分数
func (r *SimpleReranker) keywordMatchScore(queryTerms, docTerms []string) float64 {
	if len(queryTerms) == 0 {
		return 0
	}

	// 构建文档词频映射
	docTermFreq := make(map[string]int)
	for _, term := range docTerms {
		docTermFreq[strings.ToLower(term)]++
	}

	// 计算匹配分数
	matchCount := 0
	totalTF := 0.0
	for _, qTerm := range queryTerms {
		qTermLower := strings.ToLower(qTerm)
		if freq, ok := docTermFreq[qTermLower]; ok {
			matchCount++
			// TF-IDF 简化: log(1 + tf)
			totalTF += math.Log(1 + float64(freq))
		}
	}

	// Jaccard 系数
	jaccard := float64(matchCount) / float64(len(queryTerms))

	// 综合: Jaccard * 0.5 + TF * 0.5
	return jaccard*0.5 + math.Min(totalTF/float64(len(queryTerms)), 1.0)*0.5
}

// positionScore 计算位置分数
func (r *SimpleReranker) positionScore(queryTerms []string, content string) float64 {
	if len(queryTerms) == 0 || len(content) == 0 {
		return 0
	}

	contentLower := strings.ToLower(content)
	totalScore := 0.0

	for _, term := range queryTerms {
		termLower := strings.ToLower(term)
		pos := strings.Index(contentLower, termLower)
		if pos >= 0 {
			// 位置越靠前分数越高 (指数衰减)
			posRatio := float64(pos) / float64(len(content))
			totalScore += math.Exp(-2 * posRatio) // e^(-2x), x=0 时为 1, x=0.5 时约 0.37
		}
	}

	return totalScore / float64(len(queryTerms))
}

// lengthScore 计算长度分数
func (r *SimpleReranker) lengthScore(docLength int) float64 {
	// 理想长度范围: 50-500 词
	idealMin := 50
	idealMax := 500

	if docLength < idealMin {
		return float64(docLength) / float64(idealMin)
	}
	if docLength > idealMax {
		return float64(idealMax) / float64(docLength)
	}
	return 1.0
}

// tokenize 分词 (简单实现)
func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current.WriteRune(r)
		} else if unicode.Is(unicode.Han, r) {
			// 中文字符单独作为 token
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// CrossEncoderReranker 基于 Cross-Encoder 的重排序器 (需要外部 API)
type CrossEncoderReranker struct {
	apiEndpoint string
	apiKey      string
	model       string
}

// NewCrossEncoderReranker 创建 Cross-Encoder 重排序器
func NewCrossEncoderReranker(endpoint, apiKey, model string) *CrossEncoderReranker {
	return &CrossEncoderReranker{
		apiEndpoint: endpoint,
		apiKey:      apiKey,
		model:       model,
	}
}

// Rerank 使用 Cross-Encoder API 进行重排序
func (r *CrossEncoderReranker) Rerank(ctx context.Context, query string, documents []*SearchResult, topK int) ([]*SearchResult, error) {
	// TODO: 实现外部 API 调用 (Cohere, Jina, 或自建服务)
	// 目前降级到简单重排序
	simple := NewSimpleReranker()
	return simple.Rerank(ctx, query, documents, topK)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
