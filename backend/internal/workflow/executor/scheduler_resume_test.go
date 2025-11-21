package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchedulerResumeFromPreviousResults 测试 Scheduler 从之前的结果恢复
func TestSchedulerResumeFromPreviousResults(t *testing.T) {
	// 1. 创建一个简单的 DAG: step1 -> step2 -> step3
	dag := &DAG{
		Nodes: map[string]*Node{
			"step1": {
				ID:           "step1",
				Step:         &StepDefinition{ID: "step1", Type: "test"},
				Dependencies: []string{},
				Dependents:   []string{"step2"},
			},
			"step2": {
				ID:           "step2",
				Step:         &StepDefinition{ID: "step2", Type: "test"},
				Dependencies: []string{"step1"},
				Dependents:   []string{"step3"},
			},
			"step3": {
				ID:           "step3",
				Step:         &StepDefinition{ID: "step3", Type: "test"},
				Dependencies: []string{"step2"},
				Dependents:   []string{},
			},
		},
	}

	// 2. 创建 mock executor，只执行 step2 和 step3
	executor := &mockTaskExecutor{
		results: map[string]*TaskResult{
			"step2": {ID: "step2", Status: "success", Output: "step2_output"},
			"step3": {ID: "step3", Status: "success", Output: "step3_output"},
		},
	}

	// 3. 创建 Scheduler
	scheduler := NewScheduler(dag, executor, 5)

	// 4. 准备 previousResults（step1 已完成）
	previousResults := map[string]*TaskResult{
		"step1": {
			ID:     "step1",
			Status: "success",
			Output: "step1_output",
		},
	}

	// 5. 创建执行上下文
	execCtx := NewExecutionContext("test-workflow", "test-execution", "test-tenant", "test-user")

	// 6. 恢复执行
	ctx := context.Background()
	results, err := scheduler.Resume(ctx, execCtx, previousResults)

	// 7. 验证结果
	require.NoError(t, err)
	assert.Equal(t, 3, len(results), "应该有 3 个任务结果")

	// 验证所有步骤都成功
	assert.Equal(t, "success", results["step1"].Status, "step1 应该成功")
	assert.Equal(t, "success", results["step2"].Status, "step2 应该成功")
	assert.Equal(t, "success", results["step3"].Status, "step3 应该成功")

	// 验证输出
	assert.Equal(t, "step1_output", results["step1"].Output)
	assert.Equal(t, "step2_output", results["step2"].Output)
	assert.Equal(t, "step3_output", results["step3"].Output)
}

// TestSchedulerResumeWithPausedTask 测试 Scheduler 遇到暂停任务时的行为
func TestSchedulerResumeWithPausedTask(t *testing.T) {
	// 1. 创建 DAG: step1 -> step2 -> step3
	dag := &DAG{
		Nodes: map[string]*Node{
			"step1": {
				ID:           "step1",
				Step:         &StepDefinition{ID: "step1", Type: "test"},
				Dependencies: []string{},
				Dependents:   []string{"step2"},
			},
			"step2": {
				ID:           "step2",
				Step:         &StepDefinition{ID: "step2", Type: "test"},
				Dependencies: []string{"step1"},
				Dependents:   []string{"step3"},
			},
			"step3": {
				ID:           "step3",
				Step:         &StepDefinition{ID: "step3", Type: "test"},
				Dependencies: []string{"step2"},
				Dependents:   []string{},
			},
		},
	}

	// 2. 创建 mock executor，step2 返回 paused
	executor := &mockTaskExecutor{
		results: map[string]*TaskResult{
			"step2": {ID: "step2", Status: "paused", Output: nil},
			// step3 不应该被执行
		},
	}

	// 3. 创建 Scheduler
	scheduler := NewScheduler(dag, executor, 5)

	// 4. 准备 previousResults（step1 已完成）
	previousResults := map[string]*TaskResult{
		"step1": {
			ID:     "step1",
			Status: "success",
			Output: "step1_output",
		},
	}

	// 5. 创建执行上下文
	execCtx := NewExecutionContext("test-workflow", "test-execution", "test-tenant", "test-user")

	// 6. 恢复执行
	ctx := context.Background()
	results, err := scheduler.Resume(ctx, execCtx, previousResults)

	// 7. 验证结果
	require.NoError(t, err)

	// 应该只有 step1 和 step2 的结果
	assert.Equal(t, 2, len(results), "应该只有 2 个任务结果")
	assert.Equal(t, "success", results["step1"].Status)
	assert.Equal(t, "paused", results["step2"].Status)

	// step3 不应该被执行
	_, hasStep3 := results["step3"]
	assert.False(t, hasStep3, "step3 不应该被执行")
}

// TestSchedulerResumeMultipleBranches 测试 Scheduler 恢复多分支 DAG
func TestSchedulerResumeMultipleBranches(t *testing.T) {
	// 1. 创建分支 DAG:
	//     step1
	//    /     \
	// step2   step3
	//    \     /
	//     step4
	dag := &DAG{
		Nodes: map[string]*Node{
			"step1": {
				ID:           "step1",
				Step:         &StepDefinition{ID: "step1", Type: "test"},
				Dependencies: []string{},
				Dependents:   []string{"step2", "step3"},
			},
			"step2": {
				ID:           "step2",
				Step:         &StepDefinition{ID: "step2", Type: "test"},
				Dependencies: []string{"step1"},
				Dependents:   []string{"step4"},
			},
			"step3": {
				ID:           "step3",
				Step:         &StepDefinition{ID: "step3", Type: "test"},
				Dependencies: []string{"step1"},
				Dependents:   []string{"step4"},
			},
			"step4": {
				ID:           "step4",
				Step:         &StepDefinition{ID: "step4", Type: "test"},
				Dependencies: []string{"step2", "step3"},
				Dependents:   []string{},
			},
		},
	}

	// 2. 创建 mock executor
	executor := &mockTaskExecutor{
		results: map[string]*TaskResult{
			"step2": {ID: "step2", Status: "success", Output: "step2_output"},
			"step3": {ID: "step3", Status: "success", Output: "step3_output"},
			"step4": {ID: "step4", Status: "success", Output: "step4_output"},
		},
	}

	// 3. 创建 Scheduler
	scheduler := NewScheduler(dag, executor, 5)

	// 4. 准备 previousResults（step1 已完成）
	previousResults := map[string]*TaskResult{
		"step1": {
			ID:     "step1",
			Status: "success",
			Output: "step1_output",
		},
	}

	// 5. 创建执行上下文
	execCtx := NewExecutionContext("test-workflow", "test-execution", "test-tenant", "test-user")

	// 6. 恢复执行
	ctx := context.Background()
	results, err := scheduler.Resume(ctx, execCtx, previousResults)

	// 7. 验证结果
	require.NoError(t, err)
	assert.Equal(t, 4, len(results), "应该有 4 个任务结果")

	// 验证所有步骤都成功
	for stepID, result := range results {
		assert.Equal(t, "success", result.Status, "步骤 %s 应该成功", stepID)
	}
}

// Mock 实现

type mockTaskExecutor struct {
	results map[string]*TaskResult
}

func (m *mockTaskExecutor) ExecuteTask(ctx context.Context, task *Task) (*TaskResult, error) {
	if result, ok := m.results[task.ID]; ok {
		return result, nil
	}
	// 默认返回成功
	return &TaskResult{
		ID:     task.ID,
		Status: "success",
		Output: map[string]any{"result": "ok"},
	}, nil
}

