package runtime

import (
	"context"
	"time"
)

// AgentRunLog Agent 执行日志
type AgentRunLog struct {
	ID            string          `json:"id"`
	AgentID       string          `json:"agent_id"`
	TraceID       string          `json:"trace_id"`
	Status        string          `json:"status"` // running, completed, failed
	Input         string          `json:"input"`
	Output        string          `json:"output"`
	Error         string          `json:"error,omitempty"`
	Steps         []*AgentStepLog `json:"steps"`
	TotalTokens   int             `json:"total_tokens"`
	LatencyMs     int64           `json:"latency_ms"`
	CreatedAt     time.Time       `json:"created_at"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty"`
}

// AgentStepLog Agent 执行步骤日志
type AgentStepLog struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"` // plan, think, action, observation, final_answer
	Content      string         `json:"content"`
	ToolCalls    []*ToolCallLog `json:"tool_calls,omitempty"`
	LatencyMs    int64          `json:"latency_ms"`
	Timestamp    time.Time      `json:"timestamp"`
	TokenUsage   *Usage         `json:"token_usage,omitempty"`
}

// ToolCallLog 工具调用日志
type ToolCallLog struct {
	ToolName   string `json:"tool_name"`
	Arguments  string `json:"arguments"`
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
	LatencyMs  int64  `json:"latency_ms"`
}

// AgentLogService Agent 日志服务接口
type AgentLogService interface {
	CreateRun(ctx context.Context, run *AgentRunLog) error
	UpdateRun(ctx context.Context, run *AgentRunLog) error
	AddStep(ctx context.Context, runID string, step *AgentStepLog) error
	GetRun(ctx context.Context, runID string) (*AgentRunLog, error)
	GetRunTrace(ctx context.Context, runID string) (any, error) // 返回可视化 Trace 数据
}

