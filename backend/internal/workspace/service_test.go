package workspace

import (
	"context"
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
	limited, err := svc.ListTreeWithOptions(ctx, "tenant-a", TreeListOptions{Depth: 1})
	require.NoError(t, err)
	require.NotEmpty(t, limited)
	require.Nil(t, limited[0].Children)
	cursor := limited[0].ID
	result, err := svc.ListTreeWithOptions(ctx, "tenant-a", TreeListOptions{ParentID: &cursor, Depth: 2})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, cursor, result[0].ID)
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
	updated, err := svc.ListStagingFiles(ctx, &ListStagingRequest{TenantID: "tenant-b", Status: StagingStatusArchived})
	require.NoError(t, err)
	require.Equal(t, 1, len(updated))
}

func TestReviewRejectStagingAddsMetadata(t *testing.T) {
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
	_, err = svc.ReviewStagingFile(ctx, &ReviewStagingRequest{
		TenantID:    "tenant-c",
		StagingID:   staging.ID,
		ReviewerID:  "auditor",
		Action:      ReviewActionReject,
		Reason:      "缺少关键信息",
		ReviewToken: staging.ReviewToken,
	})
	require.NoError(t, err)
	var stored WorkspaceStagingFile
	require.NoError(t, db.Where("id = ?", staging.ID).First(&stored).Error)
	require.Equal(t, StagingStatusRejected, stored.Status)
	require.Contains(t, stored.Metadata, "review_note")
}

func TestSecondaryReviewFlow(t *testing.T) {
	ctx := context.Background()
	db := setupWorkspaceTestDB(t)
	svc := NewService(db)
	staging, err := svc.CreateStagingFile(ctx, &CreateStagingRequest{
		TenantID:          "tenant-sec",
		FileType:          "draft",
		Content:           "Secondary",
		CreatedBy:         "agent",
		RequiresSecondary: true,
	})
	require.NoError(t, err)
	updated, err := svc.ReviewStagingFile(ctx, &ReviewStagingRequest{
		TenantID:    "tenant-sec",
		StagingID:   staging.ID,
		ReviewerID:  "primary",
		Action:      ReviewActionApprove,
		ReviewToken: staging.ReviewToken,
	})
	require.NoError(t, err)
	require.Equal(t, StagingStatusAwaitingSecondary, updated.Status)
	require.NotEmpty(t, updated.SecondaryReviewToken)
	final, err := svc.ReviewStagingFile(ctx, &ReviewStagingRequest{
		TenantID:    "tenant-sec",
		StagingID:   staging.ID,
		ReviewerID:  "secondary",
		Action:      ReviewActionApprove,
		ReviewToken: updated.SecondaryReviewToken,
	})
	require.NoError(t, err)
	require.Equal(t, StagingStatusArchived, final.Status)
}

func TestCreateFileHistoryAndDiff(t *testing.T) {
	ctx := context.Background()
	db := setupWorkspaceTestDB(t)
	svc := NewService(db)
	detail, err := svc.CreateFile(ctx, &CreateFileRequest{
		TenantID: "tenant-d",
		Name:     "测试文档",
		Content:  "第一版内容",
		UserID:   "author",
	})
	require.NoError(t, err)
	require.NotNil(t, detail.File)
	_, err = svc.UpdateFileContent(ctx, &UpdateFileRequest{
		TenantID: "tenant-d",
		NodeID:   detail.Node.ID,
		Content:  "第一版内容\n第二行",
		UserID:   "author",
	})
	require.NoError(t, err)
	history, err := svc.GetFileHistory(ctx, "tenant-d", detail.Node.ID, 10)
	require.NoError(t, err)
	require.True(t, len(history) >= 2)
	latest := history[0]
	older := history[len(history)-1]
	diff, err := svc.DiffFileVersions(ctx, "tenant-d", older.VersionID, latest.VersionID)
	require.NoError(t, err)
	require.NotNil(t, diff)
	require.Equal(t, older.VersionID, diff.BaseVersion.ID)
	require.NotEmpty(t, diff.Hunks)
	result, err := svc.RevertFile(ctx, "tenant-d", detail.Node.ID, older.VersionID, "reviewer")
	require.NoError(t, err)
	require.Equal(t, older.Summary, result.Version.Summary)
}

func TestSearchFiles(t *testing.T) {
	ctx := context.Background()
	db := setupWorkspaceTestDB(t)
	svc := NewService(db)
	file, err := svc.CreateFile(ctx, &CreateFileRequest{
		TenantID: "tenant-e",
		Name:     "方案",
		Content:  "重点在于测试全文检索",
		Summary:  "测试摘要",
		UserID:   "user",
	})
	require.NoError(t, err)
	require.NotNil(t, file)
	results, total, err := svc.SearchFiles(ctx, &SearchFilesRequest{
		TenantID: "tenant-e",
		Query:    "测试",
		Limit:    5,
	})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.GreaterOrEqual(t, total, int64(1))
	require.Equal(t, "方案", results[0].Name)
}

func TestUpdateFileContentVersionConflict(t *testing.T) {
	ctx := context.Background()
	db := setupWorkspaceTestDB(t)
	svc := NewService(db)
	detail, err := svc.CreateFile(ctx, &CreateFileRequest{
		TenantID: "tenant-f",
		Name:     "冲突用例",
		Content:  "v1",
		UserID:   "author",
	})
	require.NoError(t, err)
	_, err = svc.UpdateFileContent(ctx, &UpdateFileRequest{
		TenantID:          "tenant-f",
		NodeID:            detail.Node.ID,
		Content:           "v2",
		UserID:            "author",
		ExpectedVersionID: "mismatch",
	})
	require.ErrorIs(t, err, ErrFileVersionConflict)
}
