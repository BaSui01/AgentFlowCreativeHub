package runtime

import (
	"context"
	"testing"

	"backend/internal/agent"
	"backend/internal/ai"
	"backend/internal/rag"
)

// fakeRetriever 测试用检索器
type fakeRetriever struct {
	resp      *rag.SearchResponse
	err       error
	called    bool
	lastReq   *rag.SearchRequest
}

func (f *fakeRetriever) Search(ctx context.Context, req *rag.SearchRequest) (*rag.SearchResponse, error) {
	f.called = true
	f.lastReq = req
	return f.resp, f.err
}

// fakeModelClient 测试用模型客户端,仅实现 ChatCompletion
type fakeModelClient struct {
	lastReq *ai.ChatCompletionRequest
}

func (f *fakeModelClient) ChatCompletion(ctx context.Context, req *ai.ChatCompletionRequest) (*ai.ChatCompletionResponse, error) {
	f.lastReq = req
	return &ai.ChatCompletionResponse{Content: "summary-from-model"}, nil
}

// 未在测试中使用的方法使用空实现即可
func (f *fakeModelClient) ChatCompletionStream(ctx context.Context, req *ai.ChatCompletionRequest) (<-chan ai.StreamChunk, <-chan error) {
	ch := make(chan ai.StreamChunk)
	errCh := make(chan error)
	close(ch)
	close(errCh)
	return ch, errCh
}

func (f *fakeModelClient) Embedding(ctx context.Context, req *ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	return nil, nil
}

func (f *fakeModelClient) Name() string { return "fake" }

func (f *fakeModelClient) Close() error { return nil }

// TestEnrichWithKnowledge_NoneMode 当 rag_mode=none 时不应调用检索
func TestEnrichWithKnowledge_NoneMode(t *testing.T) {
	retriever := &fakeRetriever{}
	helper := NewRAGHelper(retriever)

	cfg := &agent.AgentConfig{
		TenantID:        "tenant",
		KnowledgeBaseID: "kb",
		RAGEnabled:      true,
		RAGTopK:         3,
		RAGMinScore:     0.5,
		ExtraConfig: map[string]any{
			"rag_mode": string(RAGModeNone),
		},
	}

	input := &AgentInput{Content: "question"}

	ctx := context.Background()
	out, err := helper.EnrichWithKnowledge(ctx, cfg, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != input {
		t.Fatalf("expect input returned unchanged when RAG disabled")
	}
	if retriever.called {
		t.Fatalf("retriever should not be called when rag_mode=none")
	}
}

// TestEnrichWithKnowledge_StuffMode_InsertContext 验证默认 stuff 模式会注入知识上下文
func TestEnrichWithKnowledge_StuffMode_InsertContext(t *testing.T) {
	retriever := &fakeRetriever{
		resp: &rag.SearchResponse{
			Results: []*rag.SearchResult{
				{Content: "doc1", Score: 0.9},
			},
			Query: "q",
			TopK:  1,
		},
	}
	helper := NewRAGHelper(retriever)

	cfg := &agent.AgentConfig{
		TenantID:        "tenant",
		KnowledgeBaseID: "kb",
		RAGEnabled:      true,
		RAGTopK:         1,
		RAGMinScore:     0.5,
	}

	input := &AgentInput{Content: "question"}
	ctx := context.Background()
	out, err := helper.EnrichWithKnowledge(ctx, cfg, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !retriever.called {
		t.Fatalf("retriever should be called in stuff mode")
	}
	if out.Context == nil || out.Context.Data == nil {
		t.Fatalf("expect context data injected")
	}
	val, ok := out.Context.Data["knowledge_context"].(string)
	if !ok || val == "" {
		t.Fatalf("expect non-empty knowledge_context, got %#v", out.Context.Data["knowledge_context"])
	}
}
