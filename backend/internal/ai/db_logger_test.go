package ai

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// initTestDB 创建内存数据库用于测试
func initTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:db_logger_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	schema := `
		CREATE TABLE ai_call_logs (
			id TEXT PRIMARY KEY,
			tenant_id TEXT,
			user_id TEXT,
			model_provider TEXT,
			model_name TEXT,
			request_tokens INTEGER,
			response_tokens INTEGER,
			total_tokens INTEGER,
			latency_ms INTEGER,
			cost REAL,
			status TEXT,
			error_message TEXT,
			metadata JSON,
			created_at TIMESTAMP
		);
	`
	if err := db.Exec(schema).Error; err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return db
}

func TestDBLoggerLogPersistsProvider(t *testing.T) {
	db := initTestDB(t)
	logger := NewDBLogger(db)
	log := &ModelCallLog{
		TenantID:      "tenant-1",
		UserID:        "user-1",
		ModelID:       "model-uuid",
		ModelProvider: "openai",
		ModelName:     "gpt-4o",
		PromptTokens:  10,
		TotalTokens:   20,
	}
	if err := logger.Log(context.Background(), log); err != nil {
		t.Fatalf("log failed: %v", err)
	}
	var stored struct {
		ModelProvider string
		ModelName     string
	}
	if err := db.WithContext(context.Background()).
		Table("ai_call_logs").
		Select("model_provider, model_name").
		First(&stored).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if stored.ModelProvider != "openai" {
		t.Fatalf("expected provider openai, got %s", stored.ModelProvider)
	}
	if stored.ModelName != "gpt-4o" {
		t.Fatalf("expected model name gpt-4o, got %s", stored.ModelName)
	}
}

func TestDBLoggerLogFallbacksToModelID(t *testing.T) {
	db := initTestDB(t)
	logger := NewDBLogger(db)
	log := &ModelCallLog{
		TenantID:     "tenant-2",
		UserID:       "user-2",
		ModelID:      "model-xyz",
		PromptTokens: 5,
		TotalTokens:  5,
	}
	if err := logger.Log(context.Background(), log); err != nil {
		t.Fatalf("log failed: %v", err)
	}
	var stored struct {
		ModelProvider string
		ModelName     string
	}
	if err := db.WithContext(context.Background()).
		Table("ai_call_logs").
		Select("model_provider, model_name").
		First(&stored).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if stored.ModelProvider != "model-xyz" {
		t.Fatalf("expected fallback provider model-xyz, got %s", stored.ModelProvider)
	}
	if stored.ModelName != "model-xyz" {
		t.Fatalf("expected fallback name model-xyz, got %s", stored.ModelName)
	}
}
