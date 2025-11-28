package command

import (
	"context"
	"testing"
	"time"

	"backend/internal/workspace"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCommandTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:command_service?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&workspace.WorkspaceNode{},
		&workspace.WorkspaceFile{},
		&workspace.WorkspaceFileVersion{},
		&workspace.WorkspaceStagingFile{},
		&workspace.WorkspaceContextLink{},
		&CommandRequest{},
	))
	return db
}

func TestExecuteCommandDedup(t *testing.T) {
	ctx := context.Background()
	db := setupCommandTestDB(t)
	workspaceSvc := workspace.NewService(db)
	require.NoError(t, workspaceSvc.EnsureDefaults(ctx, "tenant-x", "user"))
	service := NewService(db, workspaceSvc)

	input := &ExecuteCommandInput{
		TenantID:    "tenant-x",
		UserID:      "user",
		AgentID:     "agent-1",
		CommandType: "summarize",
		Content:     "hello world",
	}
	res, err := service.ExecuteCommand(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, res.Request)
	require.True(t, res.NewlyCreated)

	// 重复提交应返回相同请求，不再入队
	res2, err := service.ExecuteCommand(ctx, input)
	require.NoError(t, err)
	require.Equal(t, res.Request.ID, res2.Request.ID)
	require.False(t, res2.NewlyCreated)
}

func TestMarkCompleted(t *testing.T) {
	ctx := context.Background()
	db := setupCommandTestDB(t)
	workspaceSvc := workspace.NewService(db)
	service := NewService(db, workspaceSvc)
	req := &CommandRequest{
		TenantID:    "tenant-b",
		AgentID:     "agent-1",
		CommandType: "draft",
		Status:      "queued",
	}
	require.NoError(t, db.Create(req).Error)

	service.MarkRunning(ctx, req.ID)
	service.MarkCompleted(ctx, req.ID, "输出", 1234, 99)

	stored, err := service.GetCommand(ctx, "tenant-b", req.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", stored.Status)
	require.Equal(t, 99, stored.TokenCost)
	require.Equal(t, 1234, stored.LatencyMs)
}

func TestDeadlineComputation(t *testing.T) {
	db := setupCommandTestDB(t)
	workspaceSvc := workspace.NewService(db)
	service := NewService(db, workspaceSvc)
	deadline := service.computeDeadline(&ExecuteCommandInput{DeadlineMs: 60000})
	require.NotNil(t, deadline)
	require.WithinDuration(t, time.Now().Add(60*time.Second), *deadline, 2*time.Second)
}

func TestQueuePositionAssign(t *testing.T) {
	ctx := context.Background()
	db := setupCommandTestDB(t)
	workspaceSvc := workspace.NewService(db)
	service := NewService(db, workspaceSvc)
	input := &ExecuteCommandInput{
		TenantID: "tenant-q",
		UserID:   "reviewer",
		AgentID:  "agent-queue",
		Content:  "first command",
	}
	res1, err := service.ExecuteCommand(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 1, res1.Request.QueuePosition)
	res2, err := service.ExecuteCommand(ctx, &ExecuteCommandInput{
		TenantID: "tenant-q",
		UserID:   "reviewer",
		AgentID:  "agent-queue",
		Content:  "second command",
	})
	require.NoError(t, err)
	require.Equal(t, 2, res2.Request.QueuePosition)
}

func TestListCommandsFilters(t *testing.T) {
	ctx := context.Background()
	db := setupCommandTestDB(t)
	workspaceSvc := workspace.NewService(db)
	service := NewService(db, workspaceSvc)
	now := time.Now()
	seed := []CommandRequest{
		{ID: "cmd-1", TenantID: "tenant-list", AgentID: "agent-a", Status: "completed", CommandType: "draft", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "cmd-2", TenantID: "tenant-list", AgentID: "agent-b", Status: "failed", CommandType: "summarize", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "cmd-3", TenantID: "tenant-list", AgentID: "agent-a", Status: "queued", CommandType: "translate", CreatedAt: now},
	}
	for _, item := range seed {
		req := item
		require.NoError(t, db.WithContext(ctx).Create(&req).Error)
	}
	items, total, err := service.ListCommands(ctx, "tenant-list", ListCommandsParams{Status: "completed", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "cmd-1", items[0].ID)
	items, total, err = service.ListCommands(ctx, "tenant-list", ListCommandsParams{AgentID: "agent-a", Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, items, 1)
	require.Equal(t, "cmd-1", items[0].ID)
}
