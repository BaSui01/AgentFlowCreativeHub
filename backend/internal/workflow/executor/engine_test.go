package executor

import "testing"

// TestEngineDefaultConcurrency 确保默认并发为 5
func TestEngineDefaultConcurrency(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil)
	if e.MaxConcurrency() != 5 {
		t.Fatalf("expected default concurrency 5, got %d", e.MaxConcurrency())
	}
}

// TestEngineCustomConcurrency 确保配置生效
func TestEngineCustomConcurrency(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil, WithMaxConcurrency(8))
	if e.MaxConcurrency() != 8 {
		t.Fatalf("expected concurrency 8, got %d", e.MaxConcurrency())
	}
}

// TestEngineInvalidConcurrency 确保非法配置被忽略
func TestEngineInvalidConcurrency(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil, WithMaxConcurrency(0))
	if e.MaxConcurrency() != 5 {
		t.Fatalf("expected fallback concurrency 5, got %d", e.MaxConcurrency())
	}
}
