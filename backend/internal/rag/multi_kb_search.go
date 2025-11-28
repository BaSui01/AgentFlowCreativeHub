package rag

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// MultiKBSearchRequest 多知识库搜索请求
type MultiKBSearchRequest struct {
	KnowledgeBaseIDs []string // 要搜索的知识库 ID 列表
	TenantID         string
	Query            string
	TopK             int

	// 高级选项
	EnableHybrid   bool    // 启用混合检索
	EnableRerank   bool    // 启用重排序
	RerankTopK     int     // 重排序 TopK
	ScoreThreshold float64 // 最小相关度阈值
	WeightByKB     map[string]float64 // 每个知识库的权重 (可选)
}

// MultiKBSearchResponse 多知识库搜索响应
type MultiKBSearchResponse struct {
	Results       []*MultiKBSearchResult
	Query         string
	TopK          int
	SearchedKBs   []string // 实际搜索的知识库
	FailedKBs     []string // 搜索失败的知识库
}

// MultiKBSearchResult 带知识库信息的搜索结果
type MultiKBSearchResult struct {
	*SearchResult
	KnowledgeBaseID   string  `json:"knowledgeBaseId"`
	KnowledgeBaseName string  `json:"knowledgeBaseName"`
	Weight            float64 `json:"weight"` // 权重调整后的分数
}

// MultiKBSearcher 多知识库搜索器
type MultiKBSearcher struct {
	ragService *RAGService
}

// NewMultiKBSearcher 创建多知识库搜索器
func NewMultiKBSearcher(ragService *RAGService) *MultiKBSearcher {
	return &MultiKBSearcher{ragService: ragService}
}

// Search 执行多知识库搜索
func (s *MultiKBSearcher) Search(ctx context.Context, req *MultiKBSearchRequest) (*MultiKBSearchResponse, error) {
	if len(req.KnowledgeBaseIDs) == 0 {
		return nil, fmt.Errorf("至少需要指定一个知识库")
	}

	if req.TopK <= 0 {
		req.TopK = 10
	}

	// 并发搜索所有知识库
	var wg sync.WaitGroup
	resultsChan := make(chan *kbSearchResult, len(req.KnowledgeBaseIDs))

	for _, kbID := range req.KnowledgeBaseIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			searchReq := &SearchRequest{
				KnowledgeBaseID: id,
				TenantID:        req.TenantID,
				Query:           req.Query,
				TopK:            req.TopK * 2, // 多取一些用于融合
				EnableHybrid:    req.EnableHybrid,
				EnableRerank:    false, // 先不重排，最后统一重排
				ScoreThreshold:  0,     // 先不过滤
			}

			resp, err := s.ragService.Search(ctx, searchReq)
			resultsChan <- &kbSearchResult{
				kbID:    id,
				results: resp,
				err:     err,
			}
		}(kbID)
	}

	wg.Wait()
	close(resultsChan)

	// 收集结果
	var allResults []*MultiKBSearchResult
	var searchedKBs, failedKBs []string

	// 获取知识库名称映射
	kbNames := s.getKBNames(ctx, req.TenantID, req.KnowledgeBaseIDs)

	for result := range resultsChan {
		if result.err != nil {
			failedKBs = append(failedKBs, result.kbID)
			continue
		}

		searchedKBs = append(searchedKBs, result.kbID)

		// 获取该知识库的权重
		weight := 1.0
		if req.WeightByKB != nil {
			if w, ok := req.WeightByKB[result.kbID]; ok {
				weight = w
			}
		}

		// 转换结果
		for _, r := range result.results.Results {
			allResults = append(allResults, &MultiKBSearchResult{
				SearchResult:      r,
				KnowledgeBaseID:   result.kbID,
				KnowledgeBaseName: kbNames[result.kbID],
				Weight:            r.Score * weight,
			})
		}
	}

	// 按加权分数排序
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Weight > allResults[j].Weight
	})

	// 重排序 (如果启用)
	if req.EnableRerank && s.ragService.reranker != nil && len(allResults) > 0 {
		// 转换为 SearchResult 列表
		searchResults := make([]*SearchResult, len(allResults))
		for i, r := range allResults {
			searchResults[i] = r.SearchResult
		}

		rerankLimit := req.RerankTopK
		if rerankLimit <= 0 {
			rerankLimit = req.TopK
		}

		reranked, err := s.ragService.reranker.Rerank(ctx, req.Query, searchResults, rerankLimit)
		if err == nil {
			// 更新排序后的结果
			rerankedResults := make([]*MultiKBSearchResult, 0, len(reranked))
			for _, r := range reranked {
				// 找到对应的 MultiKBSearchResult
				for _, mr := range allResults {
					if mr.ChunkID == r.ChunkID {
						mr.SearchResult.Score = r.Score
						mr.Weight = r.Score
						rerankedResults = append(rerankedResults, mr)
						break
					}
				}
			}
			allResults = rerankedResults
		}
	}

	// 阈值过滤
	if req.ScoreThreshold > 0 {
		filtered := make([]*MultiKBSearchResult, 0)
		for _, r := range allResults {
			if r.Weight >= req.ScoreThreshold {
				filtered = append(filtered, r)
			}
		}
		allResults = filtered
	}

	// 截取 TopK
	if len(allResults) > req.TopK {
		allResults = allResults[:req.TopK]
	}

	return &MultiKBSearchResponse{
		Results:     allResults,
		Query:       req.Query,
		TopK:        req.TopK,
		SearchedKBs: searchedKBs,
		FailedKBs:   failedKBs,
	}, nil
}

// kbSearchResult 单个知识库的搜索结果
type kbSearchResult struct {
	kbID    string
	results *SearchResponse
	err     error
}

// getKBNames 获取知识库名称映射
func (s *MultiKBSearcher) getKBNames(ctx context.Context, tenantID string, kbIDs []string) map[string]string {
	names := make(map[string]string)

	var kbs []KnowledgeBase
	s.ragService.db.WithContext(ctx).
		Where("id IN ? AND tenant_id = ?", kbIDs, tenantID).
		Find(&kbs)

	for _, kb := range kbs {
		names[kb.ID] = kb.Name
	}

	return names
}

// SearchWithFusion 使用 RRF 融合的多知识库搜索
func (s *MultiKBSearcher) SearchWithFusion(ctx context.Context, req *MultiKBSearchRequest) (*MultiKBSearchResponse, error) {
	// 先获取各知识库的原始结果
	resp, err := s.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	// 按知识库分组
	kbResults := make(map[string][]*MultiKBSearchResult)
	for _, r := range resp.Results {
		kbResults[r.KnowledgeBaseID] = append(kbResults[r.KnowledgeBaseID], r)
	}

	// RRF 融合
	scores := make(map[string]float64)
	resultMap := make(map[string]*MultiKBSearchResult)
	k := 60.0 // RRF 常数

	for _, results := range kbResults {
		for rank, r := range results {
			if _, exists := resultMap[r.ChunkID]; !exists {
				resultMap[r.ChunkID] = r
			}
			scores[r.ChunkID] += 1.0 / (k + float64(rank+1))
		}
	}

	// 重新排序
	fusedResults := make([]*MultiKBSearchResult, 0, len(scores))
	for id, score := range scores {
		r := resultMap[id]
		r.Weight = score
		fusedResults = append(fusedResults, r)
	}

	sort.Slice(fusedResults, func(i, j int) bool {
		return fusedResults[i].Weight > fusedResults[j].Weight
	})

	// 截取 TopK
	if len(fusedResults) > req.TopK {
		fusedResults = fusedResults[:req.TopK]
	}

	resp.Results = fusedResults
	return resp, nil
}
