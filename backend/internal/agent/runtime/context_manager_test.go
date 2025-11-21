package runtime

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type mockSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{sessions: make(map[string]*Session)}
}

func (m *mockSessionStore) Get(_ context.Context, sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return nil, ErrSessionNotFound
}

func (m *mockSessionStore) Save(_ context.Context, session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := *session
	m.sessions[session.ID] = &copied
	return nil
}

func (m *mockSessionStore) Delete(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
	return nil
}

func newTestContextManager() (*ContextManager, *mockSessionStore) {
	store := newMockSessionStore()
	return NewContextManager(store), store
}

func TestCreateAndGetSession(t *testing.T) {
	cm, store := newTestContextManager()
	ctx := context.Background()
	sessionID := "sess-create"

	if _, err := cm.CreateSession(ctx, "tenant", "user", sessionID); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	stored, err := cm.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if stored.ID != sessionID || stored.TenantID != "tenant" || stored.UserID != "user" {
		t.Fatalf("unexpected session data: %#v", stored)
	}

	if len(store.sessions) != 1 {
		t.Fatalf("session not saved: %d", len(store.sessions))
	}
}

func TestAddMessageAndHistoryLimit(t *testing.T) {
	cm, _ := newTestContextManager()
	ctx := context.Background()
	sessionID := "sess-history"

	if _, err := cm.CreateSession(ctx, "tenant", "user", sessionID); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	for i := 1; i <= 5; i++ {
		if err := cm.AddMessage(ctx, sessionID, "user", fmt.Sprintf("m%d", i)); err != nil {
			t.Fatalf("AddMessage failed: %v", err)
		}
	}

	input := &AgentInput{}
	if err := cm.EnrichInput(ctx, input, sessionID, 3, 0, "gpt-3.5-turbo"); err != nil {
		t.Fatalf("EnrichInput failed: %v", err)
	}

	if len(input.History) != 3 {
		t.Fatalf("expect 3 messages, got %d", len(input.History))
	}
	if input.History[0].Content != "m3" || input.History[2].Content != "m5" {
		t.Fatalf("unexpected history slice: %#v", input.History)
	}
}

func TestEnrichInputSummaryMode(t *testing.T) {
	cm, _ := newTestContextManager()
	ctx := context.Background()
	sessionID := "sess-summary"

	if _, err := cm.CreateSession(ctx, "tenant", "user", sessionID); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	for i := 1; i <= 4; i++ {
		if err := cm.AddMessage(ctx, sessionID, "user", fmt.Sprintf("m%d", i)); err != nil {
			t.Fatalf("AddMessage failed: %v", err)
		}
	}

	summary := "对话摘要内容"
	if err := cm.SetData(ctx, sessionID, memorySummaryKey, summary); err != nil {
		t.Fatalf("SetData failed: %v", err)
	}

	input := &AgentInput{
		ExtraParams: map[string]any{
			"memory_mode": memoryModeSummary,
		},
	}

	if err := cm.EnrichInput(ctx, input, sessionID, 2, 0, "gpt-3.5-turbo"); err != nil {
		t.Fatalf("EnrichInput failed: %v", err)
	}

	if len(input.History) != 3 {
		t.Fatalf("expect summary + 2 messages, got %d", len(input.History))
	}
	if input.History[0].Role != "system" || input.History[0].Content != summary {
		t.Fatalf("summary not injected: %#v", input.History[0])
	}
	if input.History[1].Content != "m3" || input.History[2].Content != "m4" {
		t.Fatalf("unexpected tail: %#v", input.History[1:])
	}
}

func TestAddMessageMissingSession(t *testing.T) {
	cm, _ := newTestContextManager()
	ctx := context.Background()

	if err := cm.AddMessage(ctx, "missing", "user", "content"); err == nil {
		t.Fatalf("expected error when session missing")
	} else if err != ErrSessionNotFound {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetAndGetData(t *testing.T) {
	cm, _ := newTestContextManager()
	ctx := context.Background()
	sessionID := "sess-data"

	if _, err := cm.CreateSession(ctx, "tenant", "user", sessionID); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if err := cm.SetData(ctx, sessionID, "k", "v"); err != nil {
		t.Fatalf("SetData failed: %v", err)
	}

	val, err := cm.GetData(ctx, sessionID, "k")
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	if val.(string) != "v" {
		t.Fatalf("unexpected value: %v", val)
	}
}
