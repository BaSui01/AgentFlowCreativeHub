package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"backend/internal/command"
	"backend/internal/config"

	"github.com/hibiken/asynq"
)

// AsyncWorker 异步任务 Worker
type AsyncWorker struct {
	server   *asynq.Server
	registry *Registry
	logSvc   AgentLogService
	command  *command.Service
}

// NewAsyncWorker 创建 Worker
func NewAsyncWorker(redisCfg config.RedisConfig, registry *Registry, logSvc AgentLogService, commandSvc *command.Service) *AsyncWorker {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     formatRedisAddr(redisCfg),
			Password: redisCfg.Password,
			DB:       redisCfg.DB,
		},
		asynq.Config{
			Concurrency: 10, // 10 个并发 Worker
			Queues: map[string]int{
				"default": 1,
			},
		},
	)

	return &AsyncWorker{
		server:   srv,
		registry: registry,
		logSvc:   logSvc,
		command:  commandSvc,
	}
}

// Start 启动 Worker
func (w *AsyncWorker) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeAgentRun, w.HandleAgentRunTask)

	// 异步启动
	go func() {
		if err := w.server.Run(mux); err != nil {
			log.Printf("could not run asynq server: %v", err)
		}
	}()

	return nil
}

// Stop 停止 Worker
func (w *AsyncWorker) Stop() {
	w.server.Shutdown()
}

// HandleAgentRunTask 处理 Agent 执行任务
func (w *AsyncWorker) HandleAgentRunTask(ctx context.Context, t *asynq.Task) error {
	var p AgentRunPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	log.Printf("Processing agent run task: AgentID=%s, UserID=%s", p.AgentID, p.UserID)
	if w.command != nil && p.CommandID != "" {
		w.command.MarkRunning(ctx, p.CommandID)
	}

	// 1. 创建初始运行日志
	runLog := &AgentRunLog{
		ID:        t.ResultWriter().TaskID(), // 使用 Task ID 作为 Run ID
		AgentID:   p.AgentID,
		TraceID:   p.TraceID,
		Status:    "running",
		Input:     p.Input,
		CreatedAt: time.Now(),
	}
	_ = w.logSvc.CreateRun(ctx, runLog)

	// 2. 构造 Input
	input := &AgentInput{
		Content:     p.Input,
		Variables:   p.Variables,
		ExtraParams: p.ContextData,
		Context: &AgentContext{
			TenantID: p.TenantID,
			UserID:   p.UserID,
			TraceID:  &p.TraceID,
			// SessionID: nil // 异步任务暂不支持 Session 延续，或者需要 Payload 传递
		},
	}

	// 3. 执行 Agent
	result, err := w.registry.Execute(ctx, p.TenantID, p.AgentID, input)

	finishedAt := time.Now()
	runLog.FinishedAt = &finishedAt

	if err != nil {
		runLog.Status = "failed"
		runLog.Error = err.Error()
		_ = w.logSvc.UpdateRun(ctx, runLog)
		if w.command != nil && p.CommandID != "" {
			w.command.MarkFailed(ctx, p.CommandID, err.Error())
		}
		return fmt.Errorf("agent execution failed: %w", err)
	}

	// 4. 更新日志为成功
	runLog.Status = "completed"
	runLog.Output = result.Output
	if result.Usage != nil {
		runLog.TotalTokens = result.Usage.TotalTokens
	}
	runLog.LatencyMs = result.LatencyMs

	_ = w.logSvc.UpdateRun(ctx, runLog)
	if w.command != nil && p.CommandID != "" {
		runOutput := result.Output
		tokenCost := 0
		if result.Usage != nil {
			tokenCost = result.Usage.TotalTokens
		}
		w.command.MarkCompleted(ctx, p.CommandID, runOutput, int(result.LatencyMs), tokenCost)
	}

	return nil
}
