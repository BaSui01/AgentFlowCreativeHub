package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"backend/internal/worker/tasks"

	"github.com/hibiken/asynq"
	"go.uber.org/zap/zaptest"
)

type fakeRunner struct {
	called bool
	execID string
	retErr error
}

func (f *fakeRunner) RunExecution(ctx context.Context, executionID string) error {
	f.called = true
	f.execID = executionID
	return f.retErr
}

func TestWorkflowHandlerHandleExecuteWorkflow_Success(t *testing.T) {
	runner := &fakeRunner{}
	h := NewWorkflowHandler(runner, zaptest.NewLogger(t))
	ctx := context.Background()
	payload, _ := json.Marshal(tasks.ExecuteWorkflowPayload{ExecutionID: "exec-1"})
	task := asynq.NewTask(tasks.TypeExecuteWorkflow, payload)
	if err := h.HandleExecuteWorkflow(ctx, task); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !runner.called || runner.execID != "exec-1" {
		t.Fatalf("runner not invoked correctly: called=%v id=%s", runner.called, runner.execID)
	}
}

func TestWorkflowHandlerHandleExecuteWorkflow_RunError(t *testing.T) {
	expectedErr := errors.New("boom")
	runner := &fakeRunner{retErr: expectedErr}
	h := NewWorkflowHandler(runner, zaptest.NewLogger(t))
	ctx := context.Background()
	payload, _ := json.Marshal(tasks.ExecuteWorkflowPayload{ExecutionID: "exec-2"})
	task := asynq.NewTask(tasks.TypeExecuteWorkflow, payload)
	if err := h.HandleExecuteWorkflow(ctx, task); !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestWorkflowHandlerHandleExecuteWorkflow_InvalidPayload(t *testing.T) {
	runner := &fakeRunner{}
	h := NewWorkflowHandler(runner, zaptest.NewLogger(t))
	ctx := context.Background()
	task := asynq.NewTask(tasks.TypeExecuteWorkflow, []byte("not-json"))
	if err := h.HandleExecuteWorkflow(ctx, task); err == nil {
		t.Fatalf("expected error for invalid payload")
	}
	if runner.called {
		t.Fatalf("runner should not be called when payload invalid")
	}
}
