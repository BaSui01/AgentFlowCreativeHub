package rag

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"backend/internal/worker/tasks"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeEmbeddingProvider struct{}

func (fakeEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{float32(len(text))}, nil
}

func (fakeEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i, txt := range texts {
		res[i] = []float32{float32(len(txt))}
	}
	return res, nil
}

func (fakeEmbeddingProvider) GetModel() string        { return "test-model" }
func (fakeEmbeddingProvider) GetProviderName() string { return "test-provider" }

type fakeVectorStore struct {
	added       []*Vector
	searchReply []*SearchResult
}

func (f *fakeVectorStore) AddVectors(ctx context.Context, vectors []*Vector) error {
	f.added = append(f.added, vectors...)
	return nil
}

func (f *fakeVectorStore) Search(ctx context.Context, knowledgeBaseID string, queryVector []float32, topK int) ([]*SearchResult, error) {
	if len(f.searchReply) > 0 {
		return f.searchReply, nil
	}
	results := make([]*SearchResult, 0, len(f.added))
	for _, v := range f.added {
		results = append(results, &SearchResult{
			ChunkID:         v.ChunkID,
			KnowledgeBaseID: v.KnowledgeBaseID,
			DocumentID:      v.DocumentID,
			Content:         v.Content,
			ChunkIndex:      v.ChunkIndex,
			Score:           0.9,
			Metadata:        v.Metadata,
		})
	}
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (f *fakeVectorStore) DeleteVectors(ctx context.Context, chunkIDs []string) error     { return nil }
func (f *fakeVectorStore) DeleteByDocument(ctx context.Context, kbID, docID string) error { return nil }
func (f *fakeVectorStore) DeleteByKnowledgeBase(ctx context.Context, kbID string) error   { return nil }
func (f *fakeVectorStore) GetStats(ctx context.Context, kbID string) (*VectorStoreStats, error) {
	return &VectorStoreStats{}, nil
}

type fakeQueueClient struct {
	docIDs []string
}

func (f *fakeQueueClient) EnqueueProcessDocument(documentID string) error {
	f.docIDs = append(f.docIDs, documentID)
	return nil
}

func (f *fakeQueueClient) EnqueueExecuteWorkflow(payload tasks.ExecuteWorkflowPayload) error {
	return nil
}
func (f *fakeQueueClient) Close() error { return nil }

func setupRAGTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:rag_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&KnowledgeBase{}, &KnowledgeDocument{}))
	return db
}

func TestRAGService_UploadProcessSearch(t *testing.T) {
	ctx := context.Background()
	db := setupRAGTestDB(t)
	vectorStore := &fakeVectorStore{}
	queueClient := &fakeQueueClient{}
	svc := NewRAGService(db, vectorStore, fakeEmbeddingProvider{}, NewChunker(200, 20), queueClient)

	base := &KnowledgeBase{
		ID:                    "kb-test",
		TenantID:              "tenant-1",
		Name:                  "测试知识库",
		VisibilityScope:       "tenant",
		DefaultEmbeddingModel: "test-model",
		Status:                "active",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	require.NoError(t, db.Create(base).Error)

	uploadResp, err := svc.UploadDocument(ctx, &UploadDocumentRequest{
		KnowledgeBaseID: base.ID,
		TenantID:        base.TenantID,
		UserID:          "user-1",
		FileName:        "sample.txt",
		ContentType:     "text/plain",
		Reader:          strings.NewReader("Hello world. 集成测试覆盖整条流程。"),
	})
	require.NoError(t, err)
	if len(queueClient.docIDs) != 1 || queueClient.docIDs[0] != uploadResp.DocumentID {
		t.Fatalf("上传后应将文档入队: %+v", queueClient.docIDs)
	}

	require.NoError(t, svc.ProcessDocument(ctx, uploadResp.DocumentID))

	var doc KnowledgeDocument
	require.NoError(t, db.Where("id = ?", uploadResp.DocumentID).First(&doc).Error)
	if doc.Status != "completed" {
		t.Fatalf("文档状态应更新为 completed, 实际 %s", doc.Status)
	}
	if len(vectorStore.added) == 0 {
		t.Fatalf("分块向量应写入 vector store")
	}

	searchResp, err := svc.Search(ctx, &SearchRequest{
		KnowledgeBaseID: base.ID,
		TenantID:        base.TenantID,
		Query:           "Hello",
		TopK:            3,
	})
	require.NoError(t, err)
	if len(searchResp.Results) == 0 {
		t.Fatalf("检索结果不应为空")
	}
}
