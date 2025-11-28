package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Priority 任务优先级
type Priority int

const (
	PriorityLow      Priority = 1
	PriorityNormal   Priority = 5
	PriorityHigh     Priority = 10
	PriorityCritical Priority = 20
)

// PriorityQueueClient 支持优先级的队列客户端
type PriorityQueueClient struct {
	client *asynq.Client
	inspector *asynq.Inspector
}

// NewPriorityQueueClient 创建优先级队列客户端
func NewPriorityQueueClient(redisAddr string) *PriorityQueueClient {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})
	return &PriorityQueueClient{
		client: client,
		inspector: inspector,
	}
}

// TaskOptions 任务选项
type TaskOptions struct {
	Priority   Priority      // 优先级
	MaxRetry   int           // 最大重试次数
	Timeout    time.Duration // 超时时间
	Deadline   time.Time     // 截止时间
	Unique     time.Duration // 去重窗口
	TaskID     string        // 任务 ID (用于取消)
	ProcessAt  time.Time     // 延迟执行时间
	Retention  time.Duration // 完成后保留时间
}

// DefaultTaskOptions 默认任务选项
func DefaultTaskOptions() *TaskOptions {
	return &TaskOptions{
		Priority: PriorityNormal,
		MaxRetry: 3,
		Timeout:  30 * time.Minute,
	}
}

// Enqueue 入队任务
func (c *PriorityQueueClient) Enqueue(ctx context.Context, taskType string, payload interface{}, opts *TaskOptions) (*asynq.TaskInfo, error) {
	if opts == nil {
		opts = DefaultTaskOptions()
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(taskType, data)

	// 构建 asynq 选项
	var asynqOpts []asynq.Option

	// 队列名称基于优先级
	queue := c.priorityToQueue(opts.Priority)
	asynqOpts = append(asynqOpts, asynq.Queue(queue))

	if opts.MaxRetry > 0 {
		asynqOpts = append(asynqOpts, asynq.MaxRetry(opts.MaxRetry))
	}

	if opts.Timeout > 0 {
		asynqOpts = append(asynqOpts, asynq.Timeout(opts.Timeout))
	}

	if !opts.Deadline.IsZero() {
		asynqOpts = append(asynqOpts, asynq.Deadline(opts.Deadline))
	}

	if opts.Unique > 0 {
		asynqOpts = append(asynqOpts, asynq.Unique(opts.Unique))
	}

	if opts.TaskID != "" {
		asynqOpts = append(asynqOpts, asynq.TaskID(opts.TaskID))
	}

	if !opts.ProcessAt.IsZero() {
		asynqOpts = append(asynqOpts, asynq.ProcessAt(opts.ProcessAt))
	}

	if opts.Retention > 0 {
		asynqOpts = append(asynqOpts, asynq.Retention(opts.Retention))
	}

	return c.client.EnqueueContext(ctx, task, asynqOpts...)
}

// EnqueueWorkflow 入队工作流任务
func (c *PriorityQueueClient) EnqueueWorkflow(ctx context.Context, workflowID string, priority Priority) (*asynq.TaskInfo, error) {
	return c.Enqueue(ctx, "workflow:execute", map[string]string{
		"workflow_id": workflowID,
	}, &TaskOptions{
		Priority: priority,
		MaxRetry: 3,
		Timeout:  time.Hour,
		TaskID:   "workflow:" + workflowID,
	})
}

// EnqueueRAGIndex 入队 RAG 索引任务
func (c *PriorityQueueClient) EnqueueRAGIndex(ctx context.Context, documentID string, priority Priority) (*asynq.TaskInfo, error) {
	return c.Enqueue(ctx, "rag:index", map[string]string{
		"document_id": documentID,
	}, &TaskOptions{
		Priority: priority,
		MaxRetry: 3,
		Timeout:  30 * time.Minute,
		TaskID:   "rag:" + documentID,
	})
}

// CancelTask 取消任务
func (c *PriorityQueueClient) CancelTask(ctx context.Context, taskID string) error {
	// 尝试从所有队列中删除
	queues := []string{"critical", "high", "default", "low"}
	for _, q := range queues {
		err := c.inspector.DeleteTask(q, taskID)
		if err == nil {
			return nil
		}
		if err != asynq.ErrTaskNotFound {
			continue
		}
	}
	return fmt.Errorf("task not found: %s", taskID)
}

// GetTaskInfo 获取任务信息
func (c *PriorityQueueClient) GetTaskInfo(ctx context.Context, taskID string) (*asynq.TaskInfo, error) {
	queues := []string{"critical", "high", "default", "low"}
	for _, q := range queues {
		info, err := c.inspector.GetTaskInfo(q, taskID)
		if err == nil {
			return info, nil
		}
	}
	return nil, fmt.Errorf("task not found: %s", taskID)
}

// ListPendingTasks 列出待处理任务
func (c *PriorityQueueClient) ListPendingTasks(ctx context.Context, queue string, page, pageSize int) ([]*asynq.TaskInfo, int, error) {
	tasks, err := c.inspector.ListPendingTasks(queue, asynq.PageSize(pageSize), asynq.Page(page))
	if err != nil {
		return nil, 0, err
	}

	// 获取总数
	info, err := c.inspector.GetQueueInfo(queue)
	if err != nil {
		return tasks, len(tasks), nil
	}

	return tasks, info.Pending, nil
}

// ListActiveTasks 列出正在执行的任务
func (c *PriorityQueueClient) ListActiveTasks(ctx context.Context, queue string) ([]*asynq.TaskInfo, error) {
	return c.inspector.ListActiveTasks(queue)
}

// GetQueueStats 获取队列统计
func (c *PriorityQueueClient) GetQueueStats(ctx context.Context) (map[string]*AsynqQueueStats, error) {
	queues := []string{"critical", "high", "default", "low"}
	stats := make(map[string]*AsynqQueueStats)

	for _, q := range queues {
		info, err := c.inspector.GetQueueInfo(q)
		if err != nil {
			continue
		}
		stats[q] = &AsynqQueueStats{
			Queue:      q,
			Pending:    info.Pending,
			Active:     info.Active,
			Scheduled:  info.Scheduled,
			Retry:      info.Retry,
			Archived:   info.Archived,
			Completed:  info.Completed,
			Processed:  info.Processed,
			Failed:     info.Failed,
		}
	}

	return stats, nil
}

// AsynqQueueStats Asynq队列统计
type AsynqQueueStats struct {
	Queue     string `json:"queue"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
	Completed int    `json:"completed"`
	Processed int    `json:"processed"`
	Failed    int    `json:"failed"`
}

// priorityToQueue 优先级转队列名
func (c *PriorityQueueClient) priorityToQueue(p Priority) string {
	switch {
	case p >= PriorityCritical:
		return "critical"
	case p >= PriorityHigh:
		return "high"
	case p >= PriorityNormal:
		return "default"
	default:
		return "low"
	}
}

// Close 关闭客户端
func (c *PriorityQueueClient) Close() error {
	return c.client.Close()
}

// PriorityQueueServer 优先级队列服务器配置
type PriorityQueueServerConfig struct {
	Concurrency int // 并发数
	Queues      map[string]int // 队列优先级权重
}

// DefaultPriorityQueueServerConfig 默认服务器配置
func DefaultPriorityQueueServerConfig() *PriorityQueueServerConfig {
	return &PriorityQueueServerConfig{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6, // 最高权重
			"high":     4,
			"default":  2,
			"low":      1,
		},
	}
}

// ToAsynqConfig 转换为 asynq 配置
func (c *PriorityQueueServerConfig) ToAsynqConfig() asynq.Config {
	return asynq.Config{
		Concurrency: c.Concurrency,
		Queues:      c.Queues,
	}
}
