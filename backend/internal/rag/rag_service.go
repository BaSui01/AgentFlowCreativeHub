package rag

import (
	"context"
	"fmt"
	"io"
	"time"

	"backend/internal/infra/queue"
	"backend/internal/metrics"
	"backend/internal/rag/parsers"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Retriever 定义检索接口,便于后续替换不同向量存储或检索实现
type Retriever interface {
	Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
}

// KeywordSearcher 定义关键词检索接口 (BM25)
type KeywordSearcher interface {
	SearchKeywords(ctx context.Context, kbID string, query string, topK int) ([]*SearchResult, error)
}

// Reranker 定义重排序接口
type Reranker interface {
	Rerank(ctx context.Context, query string, documents []*SearchResult, topK int) ([]*SearchResult, error)
}

// RAGService RAG知识库服务,实现 Retriever 接口
type RAGService struct {
	db                *gorm.DB
	vectorStore       VectorStore
	embeddingProvider EmbeddingProvider
	chunker           *Chunker
	parserRegistry    *parsers.ParserRegistry
	queueClient       queue.Client

	// Optional components for advanced RAG
	keywordSearcher KeywordSearcher
	reranker        Reranker
}

// NewRAGService 创建RAG服务实例
func NewRAGService(
	db *gorm.DB,
	vectorStore VectorStore,
	embeddingProvider EmbeddingProvider,
	chunker *Chunker,
	queueClient queue.Client,
) *RAGService {
	return &RAGService{
		db:                db,
		vectorStore:       vectorStore,
		embeddingProvider: embeddingProvider,
		chunker:           chunker,
		parserRegistry:    parsers.NewParserRegistry(),
		queueClient:       queueClient,
	}
}

// WithKeywordSearcher 配置关键词检索器
func (s *RAGService) WithKeywordSearcher(ks KeywordSearcher) *RAGService {
	s.keywordSearcher = ks
	return s
}

// WithReranker 配置重排序器
func (s *RAGService) WithReranker(r Reranker) *RAGService {
	s.reranker = r
	return s
}

// UploadDocumentRequest 上传文档请求
type UploadDocumentRequest struct {
	KnowledgeBaseID string
	TenantID        string
	UserID          string
	FileName        string
	FileSize        int64
	ContentType     string
	Reader          io.Reader
}

// UploadDocumentResponse 上传文档响应
type UploadDocumentResponse struct {
	DocumentID string
	FileName   string
	Status     string
	Message    string
}

// UploadDocument 上传并处理文档
func (s *RAGService) UploadDocument(ctx context.Context, req *UploadDocumentRequest) (*UploadDocumentResponse, error) {
	// 1. 验证知识库是否存在
	var kb KnowledgeBase
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", req.KnowledgeBaseID, req.TenantID).
		First(&kb).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("知识库不存在")
		}
		return nil, fmt.Errorf("查询知识库失败: %w", err)
	}

	// 2. 解析文档内容
	content, err := s.parseDocument(req.FileName, req.Reader)
	if err != nil {
		return nil, fmt.Errorf("解析文档失败: %w", err)
	}

	// 3. 创建文档记录
	doc := &KnowledgeDocument{
		ID:              uuid.New().String(),
		KnowledgeBaseID: req.KnowledgeBaseID,
		TenantID:        req.TenantID,
		Title:           req.FileName,
		FileName:        req.FileName,
		FileSize:        req.FileSize,
		ContentType:     req.ContentType,
		SourceType:      "upload",
		SourceURI:       fmt.Sprintf("file://%s", req.FileName),
		Version:         "v1",
		FileMimeType:    req.ContentType,
		Content:         content,
		Status:          "processing",
		UploadedBy:      req.UserID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(doc).Error; err != nil {
		return nil, fmt.Errorf("创建文档记录失败: %w", err)
	}

	// 4. 异步处理文档(分块 + 向量化) -> 改为入队
	if err := s.queueClient.EnqueueProcessDocument(doc.ID); err != nil {
		// 入队失败，更新状态
		_ = s.updateDocumentStatus(ctx, doc.ID, "failed", fmt.Sprintf("任务入队失败: %v", err))
		return nil, fmt.Errorf("任务入队失败: %w", err)
	}

	return &UploadDocumentResponse{
		DocumentID: doc.ID,
		FileName:   doc.FileName,
		Status:     "processing",
		Message:    "文档上传成功,正在处理中",
	}, nil
}

// ProcessDocument 处理文档(分块 + 向量化)
func (s *RAGService) ProcessDocument(ctx context.Context, documentID string) error {
	// 1. 查询文档
	var doc KnowledgeDocument
	if err := s.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", documentID).
		First(&doc).Error; err != nil {
		return fmt.Errorf("查询文档失败: %w", err)
	}

	// 2. 分块
	chunks, err := s.chunker.ChunkDocument(doc.Content)
	if err != nil {
		return fmt.Errorf("文档分块失败: %w", err)
	}

	// 3. 批量向量化
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	embeddings, err := s.embeddingProvider.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("向量化失败: %w", err)
	}

	// 4. 准备向量数据
	vectors := make([]*Vector, len(chunks))
	for i, chunk := range chunks {
		vectors[i] = &Vector{
			ChunkID:           uuid.New().String(),
			KnowledgeBaseID:   doc.KnowledgeBaseID,
			DocumentID:        doc.ID,
			TenantID:          doc.TenantID,
			Content:           chunk.Content,
			ContentHash:       chunk.ContentHash,
			ChunkIndex:        chunk.ChunkIndex,
			StartOffset:       chunk.StartOffset,
			EndOffset:         chunk.EndOffset,
			TokenCount:        chunk.TokenCount,
			Embedding:         embeddings[i],
			EmbeddingModel:    s.embeddingProvider.GetModel(),
			EmbeddingProvider: s.embeddingProvider.GetProviderName(),
			Metadata: map[string]interface{}{
				"file_name": doc.FileName,
			},
		}
	}

	// 5. 存储向量
	if err := s.vectorStore.AddVectors(ctx, vectors); err != nil {
		return fmt.Errorf("存储向量失败: %w", err)
	}

	// 6. 更新文档状态
	if err := s.updateDocumentStatus(ctx, documentID, "completed", ""); err != nil {
		return fmt.Errorf("更新文档状态失败: %w", err)
	}

	// 7. 更新文档统计
	if err := s.updateDocumentStats(ctx, documentID, len(chunks), len(doc.Content)); err != nil {
		return fmt.Errorf("更新文档统计失败: %w", err)
	}

	return nil
}

// SearchRequest 搜索请求
type SearchRequest struct {
	KnowledgeBaseID string
	TenantID        string
	Query           string
	TopK            int

	// 增强选项
	EnableHybrid   bool    // 启用混合检索
	EnableRerank   bool    // 启用重排序
	RerankTopK     int     // 重排序 TopK
	ScoreThreshold float64 // 最小相关度阈值
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Results []*SearchResult
	Query   string
	TopK    int
}

// Search 语义搜索
func (s *RAGService) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	// 记录开始时间
	start := time.Now()

	// 1. 验证知识库
	var kb KnowledgeBase
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", req.KnowledgeBaseID, req.TenantID).
		First(&kb).Error; err != nil {
		// 记录失败指标
		metrics.RAGSearchesTotal.WithLabelValues(req.KnowledgeBaseID, "failed").Inc()
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("知识库不存在")
		}
		return nil, fmt.Errorf("查询知识库失败: %w", err)
	}

	// 2. 向量化查询
	queryEmbedding, err := s.embeddingProvider.Embed(ctx, req.Query)
	if err != nil {
		// 记录失败指标
		metrics.RAGSearchesTotal.WithLabelValues(req.KnowledgeBaseID, "failed").Inc()
		return nil, fmt.Errorf("查询向量化失败: %w", err)
	}

	// 3. 向量搜索
	vectorResults, err := s.vectorStore.Search(ctx, req.KnowledgeBaseID, queryEmbedding, req.TopK)
	if err != nil {
		// 记录失败指标
		metrics.RAGSearchesTotal.WithLabelValues(req.KnowledgeBaseID, "failed").Inc()
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	finalResults := vectorResults

	// 4. 关键词检索 (混合检索)
	if req.EnableHybrid && s.keywordSearcher != nil {
		keywordResults, err := s.keywordSearcher.SearchKeywords(ctx, req.KnowledgeBaseID, req.Query, req.TopK)
		if err == nil {
			// RRF 融合 (Reciprocal Rank Fusion)
			finalResults = s.fusionResults(vectorResults, keywordResults, 60)
		} else {
			// 记录警告但不中断
			fmt.Printf("Keyword search failed: %v\n", err)
		}
	}

	// 5. 重排序 (Rerank)
	if req.EnableRerank && s.reranker != nil && len(finalResults) > 0 {
		rerankLimit := req.RerankTopK
		if rerankLimit <= 0 {
			rerankLimit = req.TopK // 默认与 TopK 一致
		}

		reranked, err := s.reranker.Rerank(ctx, req.Query, finalResults, rerankLimit)
		if err == nil {
			finalResults = reranked
		} else {
			fmt.Printf("Rerank failed: %v\n", err)
		}
	}

	// 6. 阈值过滤
	if req.ScoreThreshold > 0 {
		filtered := make([]*SearchResult, 0, len(finalResults))
		for _, r := range finalResults {
			if r.Score >= req.ScoreThreshold {
				filtered = append(filtered, r)
			}
		}
		finalResults = filtered
	}

	// Prometheus 指标：记录搜索成功
	duration := time.Since(start).Seconds()
	metrics.RAGSearchDuration.WithLabelValues(req.KnowledgeBaseID).Observe(duration)
	metrics.RAGSearchesTotal.WithLabelValues(req.KnowledgeBaseID, "success").Inc()

	// 记录结果数量
	if len(finalResults) > 0 {
		metrics.RAGSearchResults.WithLabelValues(req.KnowledgeBaseID).Observe(float64(len(finalResults)))
	}

	return &SearchResponse{
		Results: finalResults,
		Query:   req.Query,
		TopK:    req.TopK,
	}, nil
}

// fusionResults 实现 RRF 融合算法
func (s *RAGService) fusionResults(listA, listB []*SearchResult, k float64) []*SearchResult {
	scores := make(map[string]float64)
	docMap := make(map[string]*SearchResult)

	// 处理 List A (Vector)
	for rank, item := range listA {
		docMap[item.ChunkID] = item
		scores[item.ChunkID] += 1.0 / (k + float64(rank+1))
	}

	// 处理 List B (Keyword)
	for rank, item := range listB {
		if _, exists := docMap[item.ChunkID]; !exists {
			docMap[item.ChunkID] = item
		}
		scores[item.ChunkID] += 1.0 / (k + float64(rank+1))
	}

	// 转换回列表并排序
	results := make([]*SearchResult, 0, len(scores))
	for id, score := range scores {
		item := docMap[id]
		// 更新融合后的分数
		item.Score = score
		results = append(results, item)
	}

	// 排序 (Score 降序)
	// 这里简单使用冒泡，实际应使用 sort.Slice
	for i := 0; i < len(results)-1; i++ {
		for j := 0; j < len(results)-i-1; j++ {
			if results[j].Score < results[j+1].Score {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}

	return results
}

// DeleteDocument 删除文档
func (s *RAGService) DeleteDocument(ctx context.Context, documentID, tenantID string) error {
	// 1. 验证文档
	var doc KnowledgeDocument
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", documentID, tenantID).
		First(&doc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("文档不存在")
		}
		return fmt.Errorf("查询文档失败: %w", err)
	}

	// 2. 删除向量
	if err := s.vectorStore.DeleteByDocument(ctx, doc.KnowledgeBaseID, documentID); err != nil {
		return fmt.Errorf("删除向量失败: %w", err)
	}

	// 3. 软删除文档
	if err := s.db.WithContext(ctx).
		Model(&KnowledgeDocument{}).
		Where("id = ?", documentID).
		Update("deleted_at", time.Now()).Error; err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}

	return nil
}

// ListDocuments 列出文档
func (s *RAGService) ListDocuments(ctx context.Context, kbID, tenantID string) ([]*KnowledgeDocument, error) {
	var docs []*KnowledgeDocument
	if err := s.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND tenant_id = ? AND deleted_at IS NULL", kbID, tenantID).
		Order("created_at DESC").
		Find(&docs).Error; err != nil {
		return nil, fmt.Errorf("查询文档列表失败: %w", err)
	}

	return docs, nil
}

// parseDocument 解析文档内容
func (s *RAGService) parseDocument(fileName string, reader io.Reader) (string, error) {
	return s.parserRegistry.Parse(fileName, reader)
}

// updateDocumentStatus 更新文档状态
func (s *RAGService) updateDocumentStatus(ctx context.Context, documentID, status, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}

	return s.db.WithContext(ctx).
		Model(&KnowledgeDocument{}).
		Where("id = ?", documentID).
		Updates(updates).Error
}

// updateDocumentStats 更新文档统计信息
func (s *RAGService) updateDocumentStats(ctx context.Context, documentID string, chunkCount, charCount int) error {
    updates := map[string]interface{}{
        "updated_at": time.Now(),
    }
    // 仅当列存在时才更新，便于在 sqlite 测试环境下运行
    migrator := s.db.Migrator()
    if migrator.HasColumn(&KnowledgeDocument{}, "chunk_count") {
        updates["chunk_count"] = chunkCount
    }
    if migrator.HasColumn(&KnowledgeDocument{}, "char_count") {
        updates["char_count"] = charCount
    }
    return s.db.WithContext(ctx).
        Model(&KnowledgeDocument{}).
        Where("id = ?", documentID).
        Updates(updates).Error
}
