package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"backend/internal/config"
	"backend/internal/worker/tasks"

	"github.com/hibiken/asynq"
)

// Client 任务队列客户端接口
type Client interface {
	EnqueueProcessDocument(documentID string) error
	EnqueueExecuteWorkflow(payload tasks.ExecuteWorkflowPayload) error
	Close() error
}

type asynqClient struct {
	client *asynq.Client
}

// NewClient 创建任务队列客户端
func NewClient(cfg config.RedisConfig) Client {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &asynqClient{client: client}
}

func (c *asynqClient) EnqueueProcessDocument(documentID string) error {
	payload, err := json.Marshal(tasks.ProcessDocumentPayload{DocumentID: documentID})
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}

	task := asynq.NewTask(tasks.TypeProcessDocument, payload)
	
	// 默认重试 3 次，超时 10 分钟
	info, err := c.client.Enqueue(task, 
		asynq.MaxRetry(3), 
		asynq.Timeout(10*time.Minute),
		asynq.Queue("rag"), // RAG 专用队列
	)
	if err != nil {
		return fmt.Errorf("enqueue task failed: %w", err)
	}
	
	_ = info // 忽略 info
	return nil
}

func (c *asynqClient) EnqueueExecuteWorkflow(payload tasks.ExecuteWorkflowPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}

	task := asynq.NewTask(tasks.TypeExecuteWorkflow, data)

	// 工作流执行可能较长，设置较长超时
	info, err := c.client.Enqueue(task,
		asynq.MaxRetry(0), // 工作流内部有重试机制，这里暂不重试
		asynq.Timeout(30*time.Minute),
		asynq.Queue("workflow"), // 工作流专用队列
	)
	if err != nil {
		return fmt.Errorf("enqueue task failed: %w", err)
	}

	_ = info
	return nil
}

func (c *asynqClient) Close() error {
	return c.client.Close()
}
