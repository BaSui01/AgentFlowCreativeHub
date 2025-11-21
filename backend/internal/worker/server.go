package worker

import (
	"context"
	"fmt"

	"backend/internal/config"
	"backend/internal/rag"
	"backend/internal/worker/handlers"
	"backend/internal/worker/tasks"
	"backend/internal/workflow/executor"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

type Server struct {
	server *asynq.Server
	mux    *asynq.ServeMux
	logger *zap.Logger
}

func NewServer(
	cfg config.RedisConfig,
	ragService *rag.RAGService,
	workflowEngine *executor.Engine,
	logger *zap.Logger,
) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
		},
		asynq.Config{
			Concurrency: 10, // 并发 worker 数
			Queues: map[string]int{
				"workflow": 6, // 工作流优先级高
				"rag":      3, // RAG 优先级中
				"default":  1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error("任务执行失败",
					zap.String("type", task.Type()),
					zap.Error(err),
				)
			}),
		},
	)

	mux := asynq.NewServeMux()

	// 注册 RAG 处理器
	ragHandler := handlers.NewRAGHandler(ragService, logger)
	mux.HandleFunc(tasks.TypeProcessDocument, ragHandler.HandleProcessDocument)

	// 注册 Workflow 处理器
	workflowHandler := handlers.NewWorkflowHandler(workflowEngine, logger)
	mux.HandleFunc(tasks.TypeExecuteWorkflow, workflowHandler.HandleExecuteWorkflow)

	return &Server{
		server: srv,
		mux:    mux,
		logger: logger,
	}
}

// Run 启动 Worker 服务器
func (s *Server) Run() error {
	s.logger.Info("Worker 服务器启动中...")
	return s.server.Run(s.mux)
}

// Start 非阻塞启动
func (s *Server) Start() error {
	s.logger.Info("Worker 服务器启动中 (后台)...")
	return s.server.Start(s.mux)
}

// Shutdown 停止 Worker 服务器
func (s *Server) Shutdown() {
	s.logger.Info("Worker 服务器停止中...")
	s.server.Shutdown()
}
