package api

import (
	_ "backend/api/docs"
	auditpkg "backend/internal/audit"
	"backend/internal/config"
	"backend/internal/logger"
	"backend/internal/metrics"
	"backend/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SetupRouter 设置并返回 Gin 路由和 Worker 服务器
func SetupRouter(db *gorm.DB, cfg *config.Config) (*gin.Engine, *worker.Server) {
	router := gin.New()

	// 初始化应用容器（依赖注入）
	container, err := InitContainer(db, cfg)
	if err != nil {
		logger.Fatal("初始化应用容器失败", zap.Error(err))
	}

	// 初始化所有 Handlers
	handlers := container.InitHandlers()

	// 全局中间件
	router.Use(gin.Recovery())
	router.Use(RequestLogger())
	router.Use(CORS())
	router.Use(metrics.PrometheusMiddleware())
	router.Use(auditpkg.AuditMiddleware(container.AuditService))

	// 公开端点（不需要认证）
	router.GET("/health", HealthCheck(db))
	router.GET("/ready", ReadinessCheck(db))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 注册 API 路由
	RegisterRoutes(router, container, handlers)

	return router, container.WorkerServer
}
