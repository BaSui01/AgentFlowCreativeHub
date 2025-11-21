package workspace

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWorkspaceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:workspace_service?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&WorkspaceNode{}, &WorkspaceFile{}, &WorkspaceFileVersion{}, &WorkspaceStagingFile{}, &WorkspaceContextLink{}))
	return db
}

func TestEnsureDefaultsAndTree(t *testing.T) {
	ctx := context.Background()
	db := setupWorkspaceTestDB(t)
	svc := NewService(db)
	require.NoError(t, svc.EnsureDefaults(ctx, "tenant-a", "user-1"))
	tree, err := svc.ListTree(ctx, "tenant-a")
	require.NoError(t, err)
	require.True(t, len(tree) >= 4)
}

func TestPublishStagingFile(t *testing.T) {
	ctx := context.Background()
	db := setupWorkspaceTestDB(t)
	svc := NewService(db)
	require.NoError(t, svc.EnsureDefaults(ctx, "tenant-b", "user"))
	staging, err := svc.CreateStagingFile(ctx, &CreateStagingRequest{
		TenantID:  "tenant-b",
		FileType:  "outline",
		Content:   "# 测试大纲\n内容",
		Summary:   "概述",
		AgentID:   "agent-1",
		AgentName: "写作助手",
		Command:   "@writer",
		CreatedBy: "user",
	})
	require.NoError(t, err)
	file, version, err := svc.PublishStagingFile(ctx, "tenant-b", staging.ID, "reviewer")
	require.NoError(t, err)
	require.NotEmpty(t, file.ID)
	require.NotEmpty(t, version.ID)
	updated, err := svc.ListStagingFiles(ctx, &ListStagingRequest{TenantID: "tenant-b", Status: "approved"})
	require.NoError(t, err)
	require.Equal(t, 1, len(updated))
}

func TestRejectStagingAddsMetadata(t *testing.T) {
	ctx := context.Background()
	db := setupWorkspaceTestDB(t)
	svc := NewService(db)
	require.NoError(t, svc.EnsureDefaults(ctx, "tenant-c", "user"))
	staging, err := svc.CreateStagingFile(ctx, &CreateStagingRequest{
		TenantID:  "tenant-c",
		FileType:  "draft",
		Content:   "内容",
		CreatedBy: "user",
	})
	require.NoError(t, err)
	require.NoError(t, svc.RejectStagingFile(ctx, "tenant-c", staging.ID, "auditor", "缺少关键信息"))
	var stored WorkspaceStagingFile
	require.NoError(t, db.Where("id = ?", staging.ID).First(&stored).Error)
	require.Equal(t, "rejected", stored.Status)
	data, _ := json.Marshal(stored.Metadata)
	require.Contains(t, string(data), "review_note")
}
