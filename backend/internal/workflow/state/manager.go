package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// WorkflowState 工作流状态
type WorkflowState struct {
	ExecutionID      string         `json:"execution_id"`
	Mode             string         `json:"mode"` // auto、semi_auto、manual
	CurrentStep      string         `json:"current_step"`
	CurrentRound     int            `json:"current_round"`
	MaxRounds        int            `json:"max_rounds"`
	StepResults      map[string]any `json:"step_results"`
	PendingApprovals []string       `json:"pending_approvals"` // Approval Request IDs
	Status           string         `json:"status"`            // running、paused、completed、failed
	Metadata         map[string]any `json:"metadata"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// StateManager 状态管理器
type StateManager struct {
	redis redis.UniversalClient
}

// NewStateManager 创建状态管理器
func NewStateManager(redisClient redis.UniversalClient) *StateManager {
	return &StateManager{
		redis: redisClient,
	}
}

// GetState 获取工作流状态
func (m *StateManager) GetState(ctx context.Context, executionID string) (*WorkflowState, error) {
	key := m.stateKey(executionID)

	data, err := m.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// 状态不存在，创建新状态
			return &WorkflowState{
				ExecutionID: executionID,
				Status:      "running",
				StepResults: make(map[string]any),
				Metadata:    make(map[string]any),
				UpdatedAt:   time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("获取状态失败: %w", err)
	}

	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("解析状态失败: %w", err)
	}

	return &state, nil
}

// SaveState 保存工作流状态
func (m *StateManager) SaveState(ctx context.Context, state *WorkflowState) error {
	state.UpdatedAt = time.Now()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}

	key := m.stateKey(state.ExecutionID)

	// 保存 24 小时
	if err := m.redis.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("保存状态失败: %w", err)
	}

	return nil
}

// UpdateState 更新工作流状态
func (m *StateManager) UpdateState(ctx context.Context, executionID string, updates map[string]any) error {
	state, err := m.GetState(ctx, executionID)
	if err != nil {
		return err
	}

	// 应用更新
	for key, value := range updates {
		switch key {
		case "current_step":
			if v, ok := value.(string); ok {
				state.CurrentStep = v
			}
		case "current_round":
			if v, ok := value.(int); ok {
				state.CurrentRound = v
			}
		case "status":
			if v, ok := value.(string); ok {
				state.Status = v
			}
		case "step_result":
			if result, ok := value.(map[string]any); ok {
				if stepID, ok := result["step_id"].(string); ok {
					state.StepResults[stepID] = result["output"]
				}
			}
		case "add_approval":
			if approvalID, ok := value.(string); ok {
				state.PendingApprovals = append(state.PendingApprovals, approvalID)
				state.Status = "paused" // 有待审批的请求时暂停
			}
		case "remove_approval":
			if approvalID, ok := value.(string); ok {
				state.PendingApprovals = removeFromSlice(state.PendingApprovals, approvalID)
				if len(state.PendingApprovals) == 0 {
					state.Status = "running" // 所有审批完成后继续运行
				}
			}
		case "metadata":
			if metadata, ok := value.(map[string]any); ok {
				for k, v := range metadata {
					state.Metadata[k] = v
				}
			}
		case "step_results":
			if stepResults, ok := value.(map[string]any); ok {
				for k, v := range stepResults {
					state.StepResults[k] = v
				}
			}
		}
	}

	// 保存更新后的状态
	return m.SaveState(ctx, state)
}

// IncrementRound 增加轮次计数
func (m *StateManager) IncrementRound(ctx context.Context, executionID string) (int, error) {
	state, err := m.GetState(ctx, executionID)
	if err != nil {
		return 0, err
	}

	state.CurrentRound++

	if err := m.SaveState(ctx, state); err != nil {
		return 0, err
	}

	return state.CurrentRound, nil
}

// ShouldStop 判断是否应该停止执行
func (m *StateManager) ShouldStop(ctx context.Context, executionID string) (bool, string, error) {
	state, err := m.GetState(ctx, executionID)
	if err != nil {
		return false, "", err
	}

	// 检查是否暂停
	if state.Status == "paused" {
		return true, "workflow_paused", nil
	}

	// 检查是否达到最大轮次
	if state.MaxRounds > 0 && state.CurrentRound >= state.MaxRounds {
		return true, "max_rounds_reached", nil
	}

	return false, "", nil
}

// DeleteState 删除工作流状态
func (m *StateManager) DeleteState(ctx context.Context, executionID string) error {
	key := m.stateKey(executionID)
	return m.redis.Del(ctx, key).Err()
}

// stateKey 生成 Redis key
func (m *StateManager) stateKey(executionID string) string {
	return fmt.Sprintf("workflow:state:%s", executionID)
}

// 辅助函数

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != item {
			result = append(result, v)
		}
	}
	return result
}
