package rag

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQdrantStoreAddVectors(t *testing.T) {
	reqCh := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/points") {
			body, _ := io.ReadAll(r.Body)
			reqCh <- string(body)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	store, err := NewQdrantStore(QdrantOptions{
		Endpoint:            server.URL,
		Collection:          "ut_collection",
		VectorDimension:     2,
		SkipCollectionCheck: true,
		HTTPClient:          server.Client(),
	})
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	vec := &Vector{
		ChunkID:         "chunk-1",
		KnowledgeBaseID: "kb",
		DocumentID:      "doc",
		Content:         "hello",
		ChunkIndex:      0,
		Embedding:       []float32{0.1, 0.2},
		Metadata:        map[string]any{"source": "test"},
	}

	if err := store.AddVectors(context.Background(), []*Vector{vec}); err != nil {
		t.Fatalf("add vectors: %v", err)
	}

	select {
	case payload := <-reqCh:
		var body map[string]any
		_ = json.Unmarshal([]byte(payload), &body)
		points, _ := body["points"].([]any)
		if len(points) != 1 {
			t.Fatalf("expected 1 point, got %d", len(points))
		}
	default:
		t.Fatalf("no request captured")
	}
}

func TestQdrantStoreSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/points/search") {
			_, _ = w.Write([]byte(`{"status":"ok","result":[{"id":"chunk-1","score":0.9,"payload":{"knowledge_base_id":"kb","document_id":"doc","content":"world","chunk_index":1}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	store, err := NewQdrantStore(QdrantOptions{
		Endpoint:            server.URL,
		Collection:          "ut_collection",
		VectorDimension:     2,
		SkipCollectionCheck: true,
		HTTPClient:          server.Client(),
	})
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	results, err := store.Search(context.Background(), "kb", []float32{0.1, 0.2}, 3)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "world" {
		t.Fatalf("unexpected content: %s", results[0].Content)
	}
}
