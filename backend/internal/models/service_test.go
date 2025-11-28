package models

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:model_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Model{}))
	return db
}

func TestSeedDefaultModelsCreatesGeminiDefaults(t *testing.T) {
	ctx := context.Background()
	db := setupModelTestDB(t)
	svc := NewModelService(db)
	require.NoError(t, svc.SeedDefaultModels(ctx, "tenant-seed"))
	var count int64
	require.NoError(t, db.Model(&Model{}).Where("tenant_id = ? AND provider = ?", "tenant-seed", "gemini").Count(&count).Error)
	require.Greater(t, count, int64(0), "预期至少有一个 Gemini 预置模型")
}

func TestModelDiscoverySyncSupportsGemini(t *testing.T) {
	ctx := context.Background()
	db := setupModelTestDB(t)
	discovery := NewModelDiscoveryService(db, nil)
	count, err := discovery.SyncModelsFromProvider(ctx, "tenant-discovery", "gemini")
	require.NoError(t, err)
	require.Greater(t, count, 0, "应同步到 Gemini 模型列表")
	var stored Model
	require.NoError(t, db.Where("tenant_id = ? AND provider = ?", "tenant-discovery", "gemini").First(&stored).Error)
	require.Contains(t, stored.ModelIdentifier, "gemini")
}
