package executor

import "testing"

func TestConditionExecutorFunctions(t *testing.T) {
	execCtx := &ExecutionContext{
		Data: map[string]any{
			"score":   map[string]any{"quality": 95},
			"numbers": []any{1, 2, 3},
		},
	}
	cond := NewConditionExecutor(nil)

	result, err := cond.EvaluateCondition("any_of({{score.quality}} > 100, {{score.quality}} >= 90)", execCtx)
	if err != nil || !result {
		t.Fatalf("any_of should be true, err=%v", err)
	}

	result, err = cond.EvaluateCondition("all_of({{score.quality}} >= 90, {{score.quality}} < 100)", execCtx)
	if err != nil || !result {
		t.Fatalf("all_of should be true, err=%v", err)
	}

	result, err = cond.EvaluateCondition("contains({{numbers}}, 2)", execCtx)
	if err != nil || !result {
		t.Fatalf("contains should be true, err=%v", err)
	}

	length, err := cond.resolveValue("len({{numbers}})", execCtx)
	if err != nil {
		t.Fatalf("len resolve failed: %v", err)
	}
	if length.(float64) != 3 {
		t.Fatalf("expected length 3, got %v", length)
	}
}
