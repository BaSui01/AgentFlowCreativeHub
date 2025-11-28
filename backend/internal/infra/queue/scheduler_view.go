package queue

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// SchedulerViewService 任务调度可视化服务
type SchedulerViewService struct {
	queues      map[string]QueueInspector
	mu          sync.RWMutex
	historySize int
	history     []TaskHistoryEntry
	historyMu   sync.RWMutex
}

// QueueInspector 队列检查接口
type QueueInspector interface {
	Name() string
	Size() int64
	GetPendingTasks(limit int) ([]*TaskInfo, error)
	GetProcessingTasks() ([]*TaskInfo, error)
	GetCompletedTasks(limit int) ([]*TaskInfo, error)
	GetFailedTasks(limit int) ([]*TaskInfo, error)
	GetTaskByID(id string) (*TaskInfo, error)
	GetStats() *QueueStats
}

// TaskInfo 任务信息
type TaskInfo struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Priority    int            `json:"priority"`
	Status      TaskStatus     `json:"status"`
	Payload     map[string]any `json:"payload,omitempty"`
	Result      any            `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	RetryCount  int            `json:"retry_count"`
	MaxRetries  int            `json:"max_retries"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Timeout     time.Duration  `json:"timeout,omitempty"`
	WorkerID    string         `json:"worker_id,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskProcessing TaskStatus = "processing"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskCancelled  TaskStatus = "cancelled"
	TaskTimeout    TaskStatus = "timeout"
)

// QueueStats 队列统计
type QueueStats struct {
	Name            string        `json:"name"`
	Pending         int64         `json:"pending"`
	Processing      int64         `json:"processing"`
	Completed       int64         `json:"completed"`
	Failed          int64         `json:"failed"`
	TotalProcessed  int64         `json:"total_processed"`
	AvgProcessTime  time.Duration `json:"avg_process_time"`
	TasksPerSecond  float64       `json:"tasks_per_second"`
	OldestPending   *time.Time    `json:"oldest_pending,omitempty"`
	WorkerCount     int           `json:"worker_count"`
}

// TaskHistoryEntry 任务历史记录
type TaskHistoryEntry struct {
	TaskID      string        `json:"task_id"`
	QueueName   string        `json:"queue_name"`
	Type        string        `json:"type"`
	Status      TaskStatus    `json:"status"`
	Duration    time.Duration `json:"duration"`
	CompletedAt time.Time     `json:"completed_at"`
}

// NewSchedulerViewService 创建调度可视化服务
func NewSchedulerViewService(historySize int) *SchedulerViewService {
	if historySize <= 0 {
		historySize = 1000
	}
	return &SchedulerViewService{
		queues:      make(map[string]QueueInspector),
		historySize: historySize,
		history:     make([]TaskHistoryEntry, 0, historySize),
	}
}

// RegisterQueue 注册队列
func (s *SchedulerViewService) RegisterQueue(queue QueueInspector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queues[queue.Name()] = queue
}

// UnregisterQueue 注销队列
func (s *SchedulerViewService) UnregisterQueue(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.queues, name)
}

// GetOverview 获取总览
func (s *SchedulerViewService) GetOverview(ctx context.Context) *SchedulerOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	overview := &SchedulerOverview{
		Queues:      make([]QueueStats, 0, len(s.queues)),
		Timestamp:   time.Now(),
	}

	for _, queue := range s.queues {
		stats := queue.GetStats()
		overview.Queues = append(overview.Queues, *stats)
		overview.TotalPending += stats.Pending
		overview.TotalProcessing += stats.Processing
		overview.TotalCompleted += stats.Completed
		overview.TotalFailed += stats.Failed
	}

	return overview
}

// SchedulerOverview 调度器总览
type SchedulerOverview struct {
	Queues          []QueueStats `json:"queues"`
	TotalPending    int64        `json:"total_pending"`
	TotalProcessing int64        `json:"total_processing"`
	TotalCompleted  int64        `json:"total_completed"`
	TotalFailed     int64        `json:"total_failed"`
	Timestamp       time.Time    `json:"timestamp"`
}

// GetQueueDetails 获取队列详情
func (s *SchedulerViewService) GetQueueDetails(ctx context.Context, queueName string) (*QueueDetails, error) {
	s.mu.RLock()
	queue, ok := s.queues[queueName]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("queue not found: %s", queueName)
	}

	details := &QueueDetails{
		Stats: queue.GetStats(),
	}

	// 获取待处理任务
	if pending, err := queue.GetPendingTasks(100); err == nil {
		details.PendingTasks = pending
	}

	// 获取处理中任务
	if processing, err := queue.GetProcessingTasks(); err == nil {
		details.ProcessingTasks = processing
	}

	// 获取最近完成的任务
	if completed, err := queue.GetCompletedTasks(20); err == nil {
		details.RecentCompleted = completed
	}

	// 获取最近失败的任务
	if failed, err := queue.GetFailedTasks(20); err == nil {
		details.RecentFailed = failed
	}

	return details, nil
}

// QueueDetails 队列详情
type QueueDetails struct {
	Stats           *QueueStats `json:"stats"`
	PendingTasks    []*TaskInfo `json:"pending_tasks"`
	ProcessingTasks []*TaskInfo `json:"processing_tasks"`
	RecentCompleted []*TaskInfo `json:"recent_completed"`
	RecentFailed    []*TaskInfo `json:"recent_failed"`
}

// GetTask 获取任务详情
func (s *SchedulerViewService) GetTask(ctx context.Context, queueName, taskID string) (*TaskInfo, error) {
	s.mu.RLock()
	queue, ok := s.queues[queueName]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("queue not found: %s", queueName)
	}

	return queue.GetTaskByID(taskID)
}

// SearchTasks 搜索任务
func (s *SchedulerViewService) SearchTasks(ctx context.Context, filter TaskFilter) ([]*TaskInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*TaskInfo, 0)

	for queueName, queue := range s.queues {
		if filter.QueueName != "" && filter.QueueName != queueName {
			continue
		}

		// 搜索各状态的任务
		if filter.Status == "" || filter.Status == TaskPending {
			if tasks, err := queue.GetPendingTasks(filter.Limit); err == nil {
				results = append(results, s.filterTasks(tasks, filter)...)
			}
		}

		if filter.Status == "" || filter.Status == TaskProcessing {
			if tasks, err := queue.GetProcessingTasks(); err == nil {
				results = append(results, s.filterTasks(tasks, filter)...)
			}
		}

		if filter.Status == "" || filter.Status == TaskCompleted {
			if tasks, err := queue.GetCompletedTasks(filter.Limit); err == nil {
				results = append(results, s.filterTasks(tasks, filter)...)
			}
		}

		if filter.Status == "" || filter.Status == TaskFailed {
			if tasks, err := queue.GetFailedTasks(filter.Limit); err == nil {
				results = append(results, s.filterTasks(tasks, filter)...)
			}
		}
	}

	// 排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// 限制数量
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// TaskFilter 任务过滤器
type TaskFilter struct {
	QueueName string
	Status    TaskStatus
	Type      string
	Tags      []string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
}

func (s *SchedulerViewService) filterTasks(tasks []*TaskInfo, filter TaskFilter) []*TaskInfo {
	filtered := make([]*TaskInfo, 0)

	for _, task := range tasks {
		// 类型过滤
		if filter.Type != "" && task.Type != filter.Type {
			continue
		}

		// 时间过滤
		if filter.StartTime != nil && task.CreatedAt.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && task.CreatedAt.After(*filter.EndTime) {
			continue
		}

		// 标签过滤
		if len(filter.Tags) > 0 && !hasAllTags(task.Tags, filter.Tags) {
			continue
		}

		filtered = append(filtered, task)
	}

	return filtered
}

func hasAllTags(taskTags, filterTags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range taskTags {
		tagSet[t] = true
	}
	for _, t := range filterTags {
		if !tagSet[t] {
			return false
		}
	}
	return true
}

// RecordTaskCompletion 记录任务完成
func (s *SchedulerViewService) RecordTaskCompletion(entry TaskHistoryEntry) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	s.history = append(s.history, entry)

	// 限制历史大小
	if len(s.history) > s.historySize {
		s.history = s.history[len(s.history)-s.historySize:]
	}
}

// GetTaskHistory 获取任务历史
func (s *SchedulerViewService) GetTaskHistory(limit int) []TaskHistoryEntry {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}

	// 返回最近的历史
	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]TaskHistoryEntry, limit)
	copy(result, s.history[start:])

	// 反转顺序（最新的在前）
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetThroughputMetrics 获取吞吐量指标
func (s *SchedulerViewService) GetThroughputMetrics(duration time.Duration) *ThroughputMetrics {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	cutoff := time.Now().Add(-duration)
	metrics := &ThroughputMetrics{
		Duration:  duration,
		ByQueue:   make(map[string]int64),
		ByType:    make(map[string]int64),
		ByStatus:  make(map[string]int64),
	}

	var totalDuration time.Duration
	for _, entry := range s.history {
		if entry.CompletedAt.Before(cutoff) {
			continue
		}

		metrics.Total++
		metrics.ByQueue[entry.QueueName]++
		metrics.ByType[entry.Type]++
		metrics.ByStatus[string(entry.Status)]++
		totalDuration += entry.Duration
	}

	if metrics.Total > 0 {
		metrics.AvgDuration = totalDuration / time.Duration(metrics.Total)
		metrics.TasksPerSecond = float64(metrics.Total) / duration.Seconds()
	}

	return metrics
}

// ThroughputMetrics 吞吐量指标
type ThroughputMetrics struct {
	Duration       time.Duration      `json:"duration"`
	Total          int64              `json:"total"`
	TasksPerSecond float64            `json:"tasks_per_second"`
	AvgDuration    time.Duration      `json:"avg_duration"`
	ByQueue        map[string]int64   `json:"by_queue"`
	ByType         map[string]int64   `json:"by_type"`
	ByStatus       map[string]int64   `json:"by_status"`
}

// GetQueueNames 获取所有队列名称
func (s *SchedulerViewService) GetQueueNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.queues))
	for name := range s.queues {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ExportStats 导出统计数据
func (s *SchedulerViewService) ExportStats() map[string]any {
	overview := s.GetOverview(context.Background())
	throughput := s.GetThroughputMetrics(time.Hour)

	return map[string]any{
		"overview":   overview,
		"throughput": throughput,
		"history":    s.GetTaskHistory(100),
		"queues":     s.GetQueueNames(),
	}
}
