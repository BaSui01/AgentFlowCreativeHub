package approval

import (
	"context"
	"testing"
	"time"

	workflowpkg "backend/internal/workflow"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestManagerCheckAutoApproval(t *testing.T) {
	mgr := &Manager{}
	ctx := context.Background()
	output := map[string]any{
		"quality_score": 95.0,
		"nested": map[string]any{
			"score": 88,
		},
	}

	approved, err := mgr.CheckAutoApproval(ctx, output, "{{quality_score}} >= 90")
	require.NoError(t, err)
	require.True(t, approved)

	approved, err = mgr.CheckAutoApproval(ctx, output, "{{nested.score}} == 88")
	require.NoError(t, err)
	require.True(t, approved)

	approved, err = mgr.CheckAutoApproval(ctx, output, "{{nested.score}} > 90")
	require.NoError(t, err)
	require.False(t, approved)
}

func TestManagerApprovePublishesEvent(t *testing.T) {
	db := openTestDB(t)
	bus := NewApprovalEventBus(nil)
	mgr := NewManager(db, WithEventBus(bus))

	approval := &workflowpkg.ApprovalRequest{
		ID:          "ap-1",
		TenantID:    "tenant-1",
		ExecutionID: "exec-1",
		WorkflowID:  "wf-1",
		StepID:      "step-1",
		Status:      "pending",
		Type:        "required",
		RequestedBy: "user-1",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(approval).Error)

	ctx := context.Background()
	eventCh, cancel := mgr.SubscribeApproval("ap-1")
	require.NotNil(t, eventCh)
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	require.NoError(t, mgr.ApproveRequest(ctx, "ap-1", "approver-1", "looks good"))

	select {
	case evt := <-eventCh:
		require.Equal(t, "approved", evt.Status)
		require.Equal(t, "approver-1", evt.ApprovedBy)
	case <-time.After(time.Second):
		t.Fatal("did not receive approval event")
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&workflowpkg.ApprovalRequest{}))
	return db
}
