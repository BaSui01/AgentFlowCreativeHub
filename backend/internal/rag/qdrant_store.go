package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// QdrantOptions 初始化 Qdrant 向量存储的配置
type QdrantOptions struct {
	Endpoint            string
	APIKey              string
	Collection          string
	VectorDimension     int
	Distance            string
	TimeoutSeconds      int
	HTTPClient          *http.Client
	SkipCollectionCheck bool
}

// QdrantStore 基于 Qdrant HTTP API 的向量存储实现
type QdrantStore struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	collection string
	vectorSize int
	distance   string
	skipEnsure bool
	ensureOnce sync.Once
	ensureErr  error
}

// NewQdrantStore 创建 Qdrant 向量存储实例
func NewQdrantStore(opts QdrantOptions) (*QdrantStore, error) {
	baseURL := strings.TrimSpace(opts.Endpoint)
	if baseURL == "" {
		return nil, fmt.Errorf("qdrant endpoint 不能为空")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	collection := opts.Collection
	if collection == "" {
		collection = "agentflow_chunks"
	}

	vectorSize := opts.VectorDimension
	if vectorSize <= 0 {
		vectorSize = 1536
	}

	distance := opts.Distance
	if distance == "" {
		distance = "Cosine"
	}

	timeout := opts.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: time.Duration(timeout) * time.Second}
	}

	store := &QdrantStore{
		client:     client,
		baseURL:    baseURL,
		apiKey:     opts.APIKey,
		collection: collection,
		vectorSize: vectorSize,
		distance:   distance,
		skipEnsure: opts.SkipCollectionCheck,
	}

	if err := store.ensureCollection(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

// AddVectors 写入或更新一批向量
func (s *QdrantStore) AddVectors(ctx context.Context, vectors []*Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}

	points := make([]qdrantPoint, 0, len(vectors))
	for _, vec := range vectors {
		if vec == nil {
			continue
		}
		if len(vec.Embedding) != s.vectorSize {
			return fmt.Errorf("向量维度不匹配: 期望 %d 实际 %d", s.vectorSize, len(vec.Embedding))
		}

		payload := map[string]any{
			"knowledge_base_id":  vec.KnowledgeBaseID,
			"document_id":        vec.DocumentID,
			"content":            vec.Content,
			"chunk_index":        vec.ChunkIndex,
			"chunk_hash":         vec.ContentHash,
			"token_count":        vec.TokenCount,
			"metadata":           vec.Metadata,
			"embedding_model":    vec.EmbeddingModel,
			"embedding_provider": vec.EmbeddingProvider,
		}
		if vec.TenantID != "" {
			payload["tenant_id"] = vec.TenantID
		}

		points = append(points, qdrantPoint{
			ID:      vec.ChunkID,
			Vector:  vec.Embedding,
			Payload: payload,
		})
	}

	req := upsertPointsRequest{Points: points}
	var resp qdrantOperationResponse
	if err := s.doRequest(ctx, http.MethodPut, s.pointsURL("?wait=true"), req, &resp); err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("qdrant upsert 失败: %s", resp.Error)
	}
	return nil
}

// Search 在指定知识库内执行相似度检索
func (s *QdrantStore) Search(ctx context.Context, knowledgeBaseID string, queryVector []float32, topK int) ([]*SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("查询向量不能为空")
	}
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	if topK <= 0 {
		topK = 5
	}

	req := searchRequest{
		Vector:      queryVector,
		Limit:       topK,
		WithPayload: true,
		Filter:      mustMatchFilter(map[string]string{"knowledge_base_id": knowledgeBaseID}),
	}

	var resp searchResponse
	if err := s.doRequest(ctx, http.MethodPost, s.collectionPath("/points/search"), req, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("qdrant search 失败: %s", resp.Error)
	}

	results := make([]*SearchResult, 0, len(resp.Result))
	for _, item := range resp.Result {
		payload := item.Payload
		content, _ := payload["content"].(string)
		chunkIndex := toInt(payload["chunk_index"])
		metadata, _ := payload["metadata"].(map[string]any)

		results = append(results, &SearchResult{
			ChunkID:         fmt.Sprint(item.ID),
			KnowledgeBaseID: stringFromPayload(payload, "knowledge_base_id"),
			DocumentID:      stringFromPayload(payload, "document_id"),
			Content:         content,
			ChunkIndex:      chunkIndex,
			Score:           item.Score,
			Similarity:      item.Score,
			Metadata:        metadata,
		})
	}

	return results, nil
}

// DeleteVectors 根据 chunkID 删除向量
func (s *QdrantStore) DeleteVectors(ctx context.Context, chunkIDs []string) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}

	req := deletePointsRequest{Points: chunkIDs}
	var resp qdrantOperationResponse
	if err := s.doRequest(ctx, http.MethodPost, s.collectionPath("/points/delete?wait=true"), req, &resp); err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("qdrant delete 失败: %s", resp.Error)
	}
	return nil
}

// DeleteByDocument 删除指定文档的所有向量
func (s *QdrantStore) DeleteByDocument(ctx context.Context, knowledgeBaseID, documentID string) error {
	if documentID == "" {
		return nil
	}
	return s.deleteByFilter(ctx, map[string]string{
		"knowledge_base_id": knowledgeBaseID,
		"document_id":       documentID,
	})
}

// DeleteByKnowledgeBase 删除知识库下全部向量
func (s *QdrantStore) DeleteByKnowledgeBase(ctx context.Context, knowledgeBaseID string) error {
	return s.deleteByFilter(ctx, map[string]string{"knowledge_base_id": knowledgeBaseID})
}

// GetStats 查询指定知识库的向量数量
func (s *QdrantStore) GetStats(ctx context.Context, knowledgeBaseID string) (*VectorStoreStats, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}

	req := countRequest{
		Filter: mustMatchFilter(map[string]string{"knowledge_base_id": knowledgeBaseID}),
	}
	var resp countResponse
	if err := s.doRequest(ctx, http.MethodPost, s.collectionPath("/points/count"), req, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("qdrant count 失败: %s", resp.Error)
	}

	return &VectorStoreStats{
		TotalVectors:   resp.Result.Count,
		TotalDocuments: 0, // Qdrant 不直接提供文档统计
	}, nil
}

// --- 内部辅助 ---

func (s *QdrantStore) collectionPath(path string) string {
	return fmt.Sprintf("/collections/%s%s", url.PathEscape(s.collection), path)
}

func (s *QdrantStore) pointsURL(query string) string {
	return fmt.Sprintf("/collections/%s/points%s", url.PathEscape(s.collection), query)
}

func (s *QdrantStore) ensureCollection(ctx context.Context) error {
	if s.skipEnsure {
		return nil
	}
	s.ensureOnce.Do(func() {
		// 先尝试探测集合
		var resp qdrantOperationResponse
		err := s.doRequest(ctx, http.MethodGet, s.collectionPath(""), nil, &resp)
		if err == nil && resp.Status == "ok" {
			s.ensureErr = nil
			return
		}

		createReq := createCollectionRequest{
			Vectors: qdrantVectorParams{
				Size:     s.vectorSize,
				Distance: s.distance,
			},
		}
		s.ensureErr = s.doRequest(ctx, http.MethodPut, s.collectionPath(""), createReq, &resp)
		if s.ensureErr == nil && resp.Status != "ok" {
			s.ensureErr = fmt.Errorf("创建 Qdrant 集合失败: %s", resp.Error)
		}
	})
	return s.ensureErr
}

func (s *QdrantStore) deleteByFilter(ctx context.Context, conditions map[string]string) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	filter := mustMatchFilter(conditions)
	req := deletePointsRequest{Filter: filter}
	var resp qdrantOperationResponse
	if err := s.doRequest(ctx, http.MethodPost, s.collectionPath("/points/delete?wait=true"), req, &resp); err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("qdrant delete 失败: %s", resp.Error)
	}
	return nil
}

func (s *QdrantStore) doRequest(ctx context.Context, method, path string, payload any, dest any) error {
	var bodyReader *bytes.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("序列化请求失败: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	fullURL := s.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("qdrant API 错误: %s (%d)", errBody["status"], resp.StatusCode)
	}

	if dest == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func mustMatchFilter(values map[string]string) *qdrantFilter {
	if len(values) == 0 {
		return nil
	}
	must := make([]fieldCondition, 0, len(values))
	for k, v := range values {
		if v == "" {
			continue
		}
		must = append(must, fieldCondition{
			Key:   k,
			Match: fieldMatch{Value: v},
		})
	}
	if len(must) == 0 {
		return nil
	}
	return &qdrantFilter{Must: must}
}

func stringFromPayload(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	case string:
		var iv int
		fmt.Sscanf(n, "%d", &iv)
		return iv
	default:
		return 0
	}
}

// --- Qdrant API payloads ---

type qdrantVectorParams struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

type createCollectionRequest struct {
	Vectors qdrantVectorParams `json:"vectors"`
}

type qdrantPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

type upsertPointsRequest struct {
	Points []qdrantPoint `json:"points"`
}

type fieldCondition struct {
	Key   string     `json:"key"`
	Match fieldMatch `json:"match"`
}

type fieldMatch struct {
	Value any `json:"value"`
}

type qdrantFilter struct {
	Must []fieldCondition `json:"must,omitempty"`
}

type deletePointsRequest struct {
	Points []string      `json:"points,omitempty"`
	Filter *qdrantFilter `json:"filter,omitempty"`
}

type searchRequest struct {
	Vector         []float32     `json:"vector"`
	Limit          int           `json:"limit"`
	WithPayload    bool          `json:"with_payload"`
	ScoreThreshold *float64      `json:"score_threshold,omitempty"`
	Filter         *qdrantFilter `json:"filter,omitempty"`
}

type searchResponse struct {
	Status string              `json:"status"`
	Result []searchResultEntry `json:"result"`
	Error  string              `json:"error"`
}

type searchResultEntry struct {
	ID      any            `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

type qdrantOperationResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type countRequest struct {
	Filter *qdrantFilter `json:"filter,omitempty"`
}

type countResponse struct {
	Status string `json:"status"`
	Result struct {
		Count int64 `json:"count"`
	} `json:"result"`
	Error string `json:"error"`
}
