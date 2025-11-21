package executor

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"backend/internal/audit"
	"backend/internal/worker/tasks"
	"backend/internal/workflow"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeQueueClient struct {
	enqueueErr  error
	lastPayload tasks.ExecuteWorkflowPayload
}

func (f *fakeQueueClient) EnqueueProcessDocument(string) error { return nil }

func (f *fakeQueueClient) EnqueueExecuteWorkflow(payload tasks.ExecuteWorkflowPayload) error {
	f.lastPayload = payload
	return f.enqueueErr
}

func (f *fakeQueueClient) Close() error { return nil }

type noopAuditService struct{}

func (noopAuditService) RecordTokenUsage(*audit.TokenUsage) error { return nil }

func setupEngineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:engine_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("初始化 sqlite 失败: %v", err)
	}
	if err := db.AutoMigrate(&workflow.Workflow{}, &workflow.WorkflowExecution{}); err != nil {
		t.Fatalf("迁移 schema 失败: %v", err)
	}
	return db
}

func createTestWorkflow(t *testing.T, db *gorm.DB, tenantID, workflowID string) {
	t.Helper()
	w := &workflow.Workflow{
		ID:          workflowID,
		TenantID:    tenantID,
		Name:        "Demo",
		Description: "",
		Definition:  workflow.WorkflowDefinition{},
		Version:     "v1",
		Visibility:  "personal",
	}
	if err := db.Create(w).Error; err != nil {
		t.Fatalf("写入 workflow 失败: %v", err)
	}
}

func TestEngineExecuteWorkflowNotFound(t *testing.T) {
	db := setupEngineTestDB(t)
	queueClient := &fakeQueueClient{}
	engine := NewEngine(db, nil, queueClient, noopAuditService{})
	ctx := context.Background()
	_, err := engine.Execute(ctx, "wf-missing", "tenant-1", "user-1", map[string]any{"k": "v"})
	if err == nil || err.Error() == "" {
		t.Fatalf("未找到工作流时应返回错误")
	}
}

func TestEngineExecuteEnqueueFailureMarksExecutionFailed(t *testing.T) {
	db := setupEngineTestDB(t)
	createTestWorkflow(t, db, "tenant-1", "wf-1")
	queueClient := &fakeQueueClient{enqueueErr: errors.New("queue down")}
	engine := NewEngine(db, nil, queueClient, noopAuditService{})
	ctx := context.Background()
	_, err := engine.Execute(ctx, "wf-1", "tenant-1", "user-1", map[string]any{"hello": "world"})
	if err == nil {
		t.Fatalf("队列失败时应返回错误")
	}
	var exec workflow.WorkflowExecution
	if err := db.Where("workflow_id = ?", "wf-1").First(&exec).Error; err != nil {
		t.Fatalf("未找到执行记录: %v", err)
	}
	if exec.Status != "failed" {
		t.Fatalf("应标记为 failed, 实际 %s", exec.Status)
	}
	if exec.ErrorMessage == "" {
		t.Fatalf("失败原因应写入 error_message")
	}
}

func TestEngineExecuteSuccessCreatesQueuedRecord(t *testing.T) {
	db := setupEngineTestDB(t)
	createTestWorkflow(t, db, "tenant-1", "wf-2")
	queueClient := &fakeQueueClient{}
	engine := NewEngine(db, nil, queueClient, noopAuditService{})
	ctx := context.Background()
	result, err := engine.Execute(ctx, "wf-2", "tenant-1", "user-xyz", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("返回状态应为 queued, got %s", result.Status)
	}
	if queueClient.lastPayload.ExecutionID != result.ExecutionID {
		t.Fatalf("入队 payload execution_id 不一致")
	}
	var exec workflow.WorkflowExecution
	if err := db.Where("id = ?", result.ExecutionID).First(&exec).Error; err != nil {
		t.Fatalf("未找到执行记录: %v", err)
	}
	if exec.Status != "queued" {
		t.Fatalf("执行状态应为 queued, got %s", exec.Status)
	}
	if exec.TenantID != "tenant-1" || exec.UserID != "user-xyz" {
		t.Fatalf("执行记录字段不正确: %+v", exec)
	}
}
