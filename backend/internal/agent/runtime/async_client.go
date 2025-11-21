package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backend/internal/config"

	"github.com/hibiken/asynq"
)

const (
	TypeAgentRun = "agent:run"
)

// AsyncClient 异步任务客户端
type AsyncClient struct {
	client *asynq.Client
}

// NewAsyncClient 创建异步客户端
func NewAsyncClient(redisCfg config.RedisConfig) *AsyncClient {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     formatRedisAddr(redisCfg),
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	return &AsyncClient{client: client}
}

func formatRedisAddr(cfg config.RedisConfig) string {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = "localhost"
	}
	port := cfg.Port
	if port == 0 {
		port = 6379
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// AgentRunPayload 任务载荷
type AgentRunPayload struct {
	AgentID     string         `json:"agent_id"`
	Input       string         `json:"input"`
	Variables   map[string]any `json:"variables"`
	ContextData map[string]any `json:"context_data"`
	TraceID     string         `json:"trace_id"`
	UserID      string         `json:"user_id"`
	TenantID    string         `json:"tenant_id"`
}

// EnqueueAgentRun 将 Agent 执行任务加入队列
func (c *AsyncClient) EnqueueAgentRun(ctx context.Context, payload *AgentRunPayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	task := asynq.NewTask(TypeAgentRun, data)

	// 任务默认保留 1 小时，超时 30 分钟
	info, err := c.client.EnqueueContext(ctx, task,
		asynq.Retention(1*time.Hour),
		asynq.Timeout(30*time.Minute))

	if err != nil {
		return "", fmt.Errorf("could not enqueue agent run task: %w", err)
	}

	return info.ID, nil
}

// Close 关闭客户端
func (c *AsyncClient) Close() error {
	return c.client.Close()
}
