package main

// @title AgentFlow Creative Hub API
// @version 1.0
// @description API documentation for AgentFlow Creative Hub
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /
// @schemes http

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"backend/api"
	docs "backend/api/docs"
	"backend/internal/config"
	"backend/internal/infra"
	"backend/internal/logger"
	"backend/internal/tenant"
	"backend/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// @title AgentFlowCreativeHub API
// @version 1.0
// @description 多 Agent 协作创作平台 API
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// 0. 统一加载 .env，便于集中管理 APP_* 环境变量
	loadEnvFile()

	// 获取环境变量
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	// 1. 加载配置
	cfg, err := config.Load(env, "")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	docs.SwaggerInfo.Host = fmt.Sprintf("localhost:%d", cfg.Server.Port)
	docs.SwaggerInfo.BasePath = "/"

	// 2. 初始化日志
	if err := logger.Init(cfg.Log.Level, cfg.Log.Format, cfg.Log.OutputPath); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("应用启动中...",
		zap.String("env", env),
		zap.String("mode", cfg.Server.Mode),
	)

	// 3. 初始化数据库
	db, err := infra.InitDatabase(&cfg.Database)
	if err != nil {
		logger.Fatal("初始化数据库失败", zap.Error(err))
	}
	defer infra.CloseDatabase()

	// 4. 执行数据库迁移（根据配置）
	if cfg.Database.AutoMigrate {
		if err := runMigrations(db); err != nil {
			logger.Fatal("数据库迁移失败", zap.Error(err))
		}
	} else {
		logger.Info("跳过自动迁移（配置已禁用）")
	}

	// 5. 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 6. 创建路由
	router, workerServer := api.SetupRouter(db, cfg)

	// 7. 创建 HTTP 服务器
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// 8. 启动服务器（goroutine）
	go func() {
		logger.Info("HTTP 服务器启动",
			zap.Int("port", cfg.Server.Port),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	// 启动 Worker 服务器 (goroutine)
	go func() {
		if err := workerServer.Run(); err != nil {
			logger.Fatal("Worker 服务器启动失败", zap.Error(err))
		}
	}()

	// 9. 优雅关闭
	gracefulShutdown(server, workerServer)
}

// loadEnvFile 依次尝试加载当前目录及上级目录的 .env 文件
func loadEnvFile() {
	if path := resolveEnvPath(); path != "" {
		if err := godotenv.Load(path); err != nil {
			fmt.Printf("加载环境变量文件 %s 失败: %v\n", path, err)
		} else {
			fmt.Printf("已加载环境变量文件: %s\n", path)
		}
	} else {
		fmt.Println("未找到 .env 文件，将仅使用系统环境变量和 config/* 配置")
	}
}

// resolveEnvPath 尝试从当前工作目录、可执行文件目录向上查找根目录 .env
func resolveEnvPath() string {
	candidates := collectEnvCandidates()
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func collectEnvCandidates() []string {
	seen := make(map[string]struct{})
	var candidates []string
	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	traverse := func(start string) {
		dir := filepath.Clean(start)
		for i := 0; i < 8; i++ {
			if dir == "" || dir == string(filepath.Separator) || dir == "." {
				break
			}
			add(filepath.Join(dir, ".env"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if wd, err := os.Getwd(); err == nil {
		traverse(wd)
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		traverse(exeDir)
	}

	return candidates
}

// runMigrations 执行数据库迁移
func runMigrations(db interface{}) error {
	gormDB, ok := db.(*gorm.DB)
	if !ok {
		logger.Info("跳过自动迁移（非 GORM 数据库）")
		return nil
	}

	logger.Info("执行核心表自动迁移...")

	// 迁移核心 tenant 相关表
	if err := gormDB.AutoMigrate(
		&tenant.Tenant{},
		&tenant.User{},
		&tenant.Role{},
		&tenant.Permission{},
		&tenant.RolePermission{},
		&tenant.UserRole{},
	); err != nil {
		return fmt.Errorf("迁移 tenant 表失败: %w", err)
	}

	logger.Info("核心表迁移完成")
	return nil
}

// gracefulShutdown 优雅关闭
func gracefulShutdown(server *http.Server, workerServer *worker.Server) {
	// 监听中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("服务器关闭异常", zap.Error(err))
	}

	// 关闭 Worker 服务器
	workerServer.Shutdown()

	// 关闭数据库连接
	if err := infra.CloseDatabase(); err != nil {
		logger.Error("数据库关闭异常", zap.Error(err))
	}

	logger.Info("服务器已安全关闭")
}
