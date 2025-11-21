package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAutomatedTaskExecutor_shouldAutoApprove(t *testing.T) {
	execCtx := &ExecutionContext{
		WorkflowID:  "wf-1",
		ExecutionID: "exe-1",
		Data: map[string]any{
			"step_1": map[string]any{
				"output": map[string]any{
					"quality": 95,
				},
			},
		},
	}
	task := &Task{
		Step: &StepDefinition{
			ID: "approval_step",
			ApprovalConfig: &ApprovalConfig{
				AutoApproveIf: &Condition{Expression: "{{step_1.output.quality}} >= 90"},
			},
		},
		Context: execCtx,
	}
	executor := &AutomatedTaskExecutor{}
	approved, err := executor.shouldAutoApprove(context.Background(), task)
	require.NoError(t, err)
	require.True(t, approved)
}

func TestAutomatedTaskExecutor_shouldAutoApproveFalse(t *testing.T) {
	execCtx := &ExecutionContext{Data: map[string]any{"score": 50}}
	task := &Task{
		Step: &StepDefinition{
			ID: "approval_step",
			ApprovalConfig: &ApprovalConfig{
				AutoApproveIf: &Condition{Expression: "{{score}} > 80"},
			},
		},
		Context: execCtx,
	}
	executor := &AutomatedTaskExecutor{}
	approved, err := executor.shouldAutoApprove(context.Background(), task)
	require.NoError(t, err)
	require.False(t, approved)
}
