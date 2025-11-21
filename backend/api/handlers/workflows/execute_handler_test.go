package workflows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/audit"
	"backend/internal/worker/tasks"
	"backend/internal/workflow"
	"backend/internal/workflow/executor"

	"github.com/gin-gonic/gin"
	sqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupWorkflowTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&workflow.Workflow{}, &workflow.WorkflowExecution{}, &workflow.WorkflowTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

type fakeWorkflowQueue struct {
	err  error
	last tasks.ExecuteWorkflowPayload
}

func (f *fakeWorkflowQueue) EnqueueProcessDocument(string) error { return nil }

func (f *fakeWorkflowQueue) EnqueueExecuteWorkflow(payload tasks.ExecuteWorkflowPayload) error {
	f.last = payload
	return f.err
}

func (f *fakeWorkflowQueue) Close() error { return nil }

type noopAudit struct{}

func (noopAudit) RecordTokenUsage(*audit.TokenUsage) error { return nil }

func insertWorkflowRecord(t *testing.T, db *gorm.DB, workflowID, tenantID string) {
	t.Helper()
	wf := &workflow.Workflow{
		ID:         workflowID,
		TenantID:   tenantID,
		Name:       "Demo",
		Definition: workflow.WorkflowDefinition{},
		Version:    "v1",
		Visibility: "personal",
	}
	if err := db.Create(wf).Error; err != nil {
		t.Fatalf("insert workflow failed: %v", err)
	}
}

func TestWorkflowExecuteHandler_ExecuteWorkflowSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWorkflowTestDB(t)
	insertWorkflowRecord(t, db, "wf-handler", "tenant-1")
	queueClient := &fakeWorkflowQueue{}
	engine := executor.NewEngine(db, nil, queueClient, noopAudit{})
	h := NewWorkflowExecuteHandler(engine, db)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := []byte(`{"input":{"q":"hi"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/wf-handler/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "wf-handler"}}
	c.Set("tenant_id", "tenant-1")
	c.Set("user_id", "user-1")

	h.ExecuteWorkflow(c)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if queueClient.last.WorkflowID != "wf-handler" {
		t.Fatalf("queue payload not captured")
	}
}

func TestWorkflowExecuteHandler_ExecuteWorkflowEngineError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWorkflowTestDB(t)
	insertWorkflowRecord(t, db, "wf-handler-err", "tenant-1")
	queueClient := &fakeWorkflowQueue{err: fmt.Errorf("queue down")}
	engine := executor.NewEngine(db, nil, queueClient, noopAudit{})
	h := NewWorkflowExecuteHandler(engine, db)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/wf-handler-err/execute", bytes.NewReader([]byte(`{"input":{}}`)))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "wf-handler-err"}}
	c.Set("tenant_id", "tenant-1")
	c.Set("user_id", "user-1")

	h.ExecuteWorkflow(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestWorkflowExecuteHandler_GetExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWorkflowTestDB(t)
	exec := &workflow.WorkflowExecution{ID: "exec-1", TenantID: "tenant-1", WorkflowID: "wf", Status: "completed"}
	task := &workflow.WorkflowTask{ID: "task-1", ExecutionID: "exec-1", StepID: "step", AgentType: "writer", Status: "completed"}
	if err := db.Create(exec).Error; err != nil {
		t.Fatalf("create exec: %v", err)
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	h := NewWorkflowExecuteHandler(nil, db)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/executions/exec-1", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "exec-1"}}
	c.Set("tenant_id", "tenant-1")

	h.GetExecution(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if resp["execution"].(map[string]any)["id"].(string) != "exec-1" {
		t.Fatalf("unexpected execution id")
	}
	tasksVal := resp["tasks"].([]any)
	if len(tasksVal) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasksVal))
	}
}

func TestWorkflowExecuteHandler_GetExecution_TenantIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWorkflowTestDB(t)
	_ = db.Create(&workflow.WorkflowExecution{ID: "exec-2", TenantID: "tenant-1", WorkflowID: "wf", Status: "running"})
	h := NewWorkflowExecuteHandler(nil, db)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/executions/exec-2", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "exec-2"}}
	c.Set("tenant_id", "tenant-2")

	h.GetExecution(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for other tenant, got %d", w.Code)
	}
}

func TestWorkflowExecuteHandler_ListExecutions_FilterAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWorkflowTestDB(t)
	execs := []workflow.WorkflowExecution{
		{ID: "exec-a", TenantID: "tenant-1", WorkflowID: "wf", Status: "completed"},
		{ID: "exec-b", TenantID: "tenant-1", WorkflowID: "wf", Status: "failed"},
		{ID: "exec-c", TenantID: "other", WorkflowID: "wf", Status: "completed"},
	}
	for _, e := range execs {
		e := e
		if err := db.Create(&e).Error; err != nil {
			t.Fatalf("create exec: %v", err)
		}
	}

	h := NewWorkflowExecuteHandler(nil, db)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/workflows/wf/executions?page=1&page_size=1&status=completed", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "wf"}}
	c.Set("tenant_id", "tenant-1")

	h.ListExecutions(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Executions []map[string]any `json:"executions"`
		Pagination map[string]any   `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if len(resp.Executions) != 1 {
		t.Fatalf("expected 1 execution page, got %d", len(resp.Executions))
	}
	if total := resp.Pagination["total"].(float64); total != 1 {
		t.Fatalf("expected total 1 for filtered tenant/status, got %v", total)
	}
	if resp.Executions[0]["id"].(string) != "exec-a" {
		t.Fatalf("expected exec-a, got %s", resp.Executions[0]["id"])
	}
}
