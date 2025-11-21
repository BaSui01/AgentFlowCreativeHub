package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	_ "backend/api/docs"
	"backend/api/handlers/agents"
	auditHandlers "backend/api/handlers/audit"
	authHandlers "backend/api/handlers/auth"
	knowledgeHandlers "backend/api/handlers/knowledge"
	"backend/api/handlers/models"
	notificationHandlers "backend/api/handlers/notifications"
	"backend/api/handlers/templates"
	tenantHandlers "backend/api/handlers/tenant"
	toolHandlers "backend/api/handlers/tools"
	"backend/api/handlers/workflows"
	workspaceHandlers "backend/api/handlers/workspace"
	auditpkg "backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/logger"
	"backend/internal/metrics"
	middlewarepkg "backend/internal/middleware"

	agentSvc "backend/internal/agent"
	"backend/internal/agent/runtime"
	"backend/internal/ai"
	modelSvc "backend/internal/models"
	"backend/internal/notification"
	"backend/internal/rag"
	templateSvc "backend/internal/template"
	tenantSvc "backend/internal/tenant"
	"backend/internal/tools"
	"backend/internal/tools/builtin"
	workspaceSvc "backend/internal/workspace"
	workflowSvc "backend/internal/workflow"
	"backend/internal/workflow/approval"
	"backend/internal/workflow/executor"
	workflowTpl "backend/internal/workflow/template"

	"backend/internal/infra/queue"
	"backend/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SetupRouter 设置并返回 Gin 路由和 Worker 服务器
func SetupRouter(db *gorm.DB, cfg *config.Config) (*gin.Engine, *worker.Server) {
	router := gin.New()

	// 统一归一化 Redis 配置，优先使用 cfg.Redis，再回退到环境变量
	redisCfg := normalizeRedisConfig(cfg.Redis)
	cfg.Redis = redisCfg

	// 初始化队列客户端
	queueClient := queue.NewClient(redisCfg)

	// 初始化 Redis 客户端（OAuth2 state、自动化状态等）
	var redisClient *redis.Client
	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port),
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
		MaintNotificationsConfig: &maintnotifications.Config{ // Redis 服务器不支持 maint_notifications
			Mode: maintnotifications.ModeDisabled,
		},
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Redis 不可用，自动化审批与 OAuth2 状态将退回内存实现", zap.Error(err))
		redisClient = nil
	}

	// 初始化认证服务
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	if appEnv == "" {
		appEnv = "dev"
	}
	jwtSecretKey := strings.TrimSpace(os.Getenv("JWT_SECRET_KEY"))
	if jwtSecretKey == "" {
		// 生产模式必须显式配置密钥，防止使用弱默认值
		if strings.EqualFold(cfg.Server.Mode, "release") || strings.EqualFold(appEnv, "prod") || strings.EqualFold(appEnv, "production") {
			logger.Fatal("JWT_SECRET_KEY 未配置，生产环境禁止使用默认密钥")
		}
		jwtSecretKey = "default_jwt_secret_key_change_in_production" // 本地/测试默认值，需明确提示
		logger.Warn("JWT_SECRET_KEY 未配置，已回退为开发默认值，请在生产环境设置强随机密钥")
	}
	jwtService := auth.NewJWTService(jwtSecretKey, "AgentFlowCreativeHub", redisClient)

	// 初始化 OAuth2 服务
	oauth2Service := auth.NewOAuth2Service()

	// 注册 OAuth2 提供商（如果配置了）
	if googleClientID := os.Getenv("GOOGLE_CLIENT_ID"); googleClientID != "" {
		oauth2Service.RegisterProvider(auth.ProviderGoogle, &auth.OAuth2Config{
			ClientID:     googleClientID,
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		})
	}

	if githubClientID := os.Getenv("GITHUB_CLIENT_ID"); githubClientID != "" {
		oauth2Service.RegisterProvider(auth.ProviderGitHub, &auth.OAuth2Config{
			ClientID:     githubClientID,
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GITHUB_REDIRECT_URL"),
		})
	}

	// 初始化会话服务
	sessionService := modelSvc.NewSessionService(db)

	// 初始化审计日志服务
	auditService := modelSvc.NewAuditLogService(db)

	// 初始化租户服务组件
	idGen := &UUIDGenerator{}
	hasher := &BcryptHasher{}
	auditAdapter := &TenantAuditAdapter{svc: auditService}

	// 获取底层 SQL DB（repository 使用标准 SQL 接口）
	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("获取底层数据库连接失败", zap.Error(err))
	}

	tenantRepo := tenantSvc.NewTenantRepository(sqlDB)
	userRepo := tenantSvc.NewUserRepository(sqlDB)
	roleRepo := tenantSvc.NewRoleRepository(sqlDB)
	userRoleRepo := tenantSvc.NewUserRoleRepository(sqlDB)
	rolePermRepo := tenantSvc.NewRolePermissionRepository(sqlDB)
	permRepo := tenantSvc.NewPermissionRepository(sqlDB)
	configRepo := tenantSvc.NewTenantConfigRepository(sqlDB)
	configCache := tenantSvc.NewInMemoryTenantConfigCache(0) // 默认无 TTL

	// 初始化租户相关 Services
	tenantService := tenantSvc.NewTenantService(tenantRepo, userRepo, nil, idGen, nil, auditAdapter)
	userService := tenantSvc.NewUserService(userRepo, hasher, auditAdapter)
	roleService := tenantSvc.NewRoleService(roleRepo, userRoleRepo, rolePermRepo, permRepo, idGen, auditAdapter)
	configService := tenantSvc.NewTenantConfigService(configRepo, configCache, auditAdapter)

	// 全局中间件
	router.Use(gin.Recovery())
	router.Use(RequestLogger())
	router.Use(CORS())

	// Prometheus 指标收集中间件
	router.Use(metrics.PrometheusMiddleware())

	// 审计中间件（记录所有请求）
	router.Use(auditpkg.AuditMiddleware(auditService))

	// 公开端点（不需要认证）
	router.GET("/health", HealthCheck(db))
	router.GET("/ready", ReadinessCheck(db))

	// Prometheus 指标端点
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Swagger 文档
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 初始化 Services
	modelService := modelSvc.NewModelService(db)
	modelCredentialService := modelSvc.NewModelCredentialService(db)
	templateService := templateSvc.NewTemplateService(db)
	agentService := agentSvc.NewAgentService(db)
	workflowService := workflowSvc.NewWorkflowService(db)
	workspaceService := workspaceSvc.NewService(db)

	// 初始化 AI 模型适配层
	dbLogger := ai.NewDBLogger(db)
	clientFactory := ai.NewClientFactory(db, dbLogger)

	// 初始化模型发现服务
	modelDiscoveryService := modelSvc.NewModelDiscoveryService(db, nil)

	// 初始化 RAG 相关服务
	kbService := modelSvc.NewKnowledgeBaseService(db)
	docService := modelSvc.NewDocumentService(db)

	// 初始化向量存储与向量化组件
	vectorStore, err := initVectorStore(cfg, db)
	if err != nil {
		logger.Fatal("Failed to init vector store", zap.Error(err))
	}
	embeddingProvider := rag.NewOpenAIEmbeddingProvider(os.Getenv("OPENAI_API_KEY"), "")
	chunker := rag.NewChunker(500, 50)
	ragService := rag.NewRAGService(db, vectorStore, embeddingProvider, chunker, queueClient)
	
	// 注入 PostgreSQL 关键词检索器 (Hybrid Search)
	keywordSearcher := rag.NewPostgresKeywordSearcher(db)
	ragService.WithKeywordSearcher(keywordSearcher)

	// 初始化 Agent 运行时(通过 ModelProvider 接入模型能力)
	agentRegistry := runtime.NewRegistry(db, clientFactory)
	// 设置 RAG Helper 到 Agent Registry（启用知识库增强）
	ragHelper := runtime.NewRAGHelper(ragService)
	agentRegistry.SetRAGHelper(ragHelper)

	// 初始化 Memory 服务
	memoryService := runtime.NewRAGMemoryService(vectorStore, embeddingProvider)
	agentRegistry.SetMemoryService(memoryService)

	// 初始化 Agent 日志服务 (Postgres)
	logService := runtime.NewPGLogService(db)

	// 初始化异步任务客户端与 Worker，走统一 Redis 配置
	asyncClient := runtime.NewAsyncClient(redisCfg)

	// 初始化异步 Worker（直接复用归一化配置）
	// AsyncWorker 需要 Registry 而不是 AgentService
	asyncWorker := runtime.NewAsyncWorker(redisCfg, agentRegistry, logService)
	if err := asyncWorker.Start(); err != nil {
		logger.Error("Failed to start AsyncWorker", zap.Error(err))
	}

	workflowMaxConcurrency := 0
	if value := strings.TrimSpace(os.Getenv("WORKFLOW_MAX_CONCURRENCY")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			workflowMaxConcurrency = parsed
		} else {
			logger.Warn("无效的 WORKFLOW_MAX_CONCURRENCY，使用默认值", zap.String("value", value), zap.Error(err))
		}
	}

	// 初始化 Token 审计服务
	tokenAuditService := auditpkg.NewTokenAuditService(db)

	// 初始化工作流执行引擎
	workflowEngine := executor.NewEngine(db, agentRegistry, queueClient, tokenAuditService, executor.WithMaxConcurrency(workflowMaxConcurrency))

	// 初始化工具注册表和执行器
	toolRegistry := tools.NewToolRegistry()

	// 注册所有内置工具
	if err := builtin.RegisterAll(toolRegistry); err != nil {
		logger.Error("Failed to register builtin tools", zap.Error(err))
	}
	workspaceTool := builtin.NewWorkspacePublishTool(workspaceService)
	if err := toolRegistry.Register(workspaceTool.GetDefinition().Name, workspaceTool, workspaceTool.GetDefinition()); err != nil {
		logger.Error("Failed to register workspace tool", zap.Error(err))
	}

	// 初始化工具执行器
	toolExecutor := tools.NewToolExecutor(toolRegistry, db)

	// 初始化 ToolHelper 并注入到 AgentRegistry
	toolHelper := runtime.NewToolHelper(toolRegistry, toolExecutor)
	agentRegistry.SetToolHelper(toolHelper)

	// Redis 客户端已在 SetupRouter 开头初始化

	var offlineStore notification.OfflineStore = notification.NewMemoryOfflineStore(100)
	if redisClient != nil {
		offlineStore = notification.NewRedisOfflineStore(redisClient, 200, time.Hour)
	}
	wsHub := notification.NewWebSocketHub(notification.WithOfflineStore(offlineStore))
	multiNotifier := notification.NewMultiNotifier(nil, nil, wsHub)
	targetResolver := approval.NewConfigTargetResolver(db, configService)
	approvalManager := approval.NewManager(
		db,
		approval.WithNotifier(multiNotifier),
		approval.WithTargetResolver(targetResolver),
	)

	identityStore := auth.NewIdentityStore(db, os.Getenv("DEFAULT_TENANT_ID"))
	var stateStore auth.StateStore
	if redisClient != nil {
		stateStore = auth.NewRedisStateStore(redisClient)
	} else {
		stateStore = auth.NewMemoryStateStore(10 * time.Minute)
	}

	// 初始化 Handlers
	modelHandler := models.NewModelHandler(modelService, modelDiscoveryService, modelCredentialService)
	templateHandler := templates.NewTemplateHandler(templateService)
	agentHandler := agents.NewAgentHandler(agentService)
	// 注入 AsyncClient
	agentExecuteHandler := agents.NewAgentExecuteHandler(agentRegistry, asyncClient)
	workflowHandler := workflows.NewWorkflowHandler(workflowService)
	workflowExecuteHandler := workflows.NewWorkflowExecuteHandler(workflowEngine, db)
	workspaceHandler := workspaceHandlers.NewHandler(workspaceService, toolExecutor, agentRegistry)

	var automationEngine *executor.AutomationEngine
	var automationHandler *workflows.AutomationHandler
	if redisClient != nil {
		automationEngine = executor.NewAutomationEngine(db, redisClient, agentRegistry, queueClient, tokenAuditService, approvalManager, executor.WithMaxConcurrency(workflowMaxConcurrency))
		automationEngine.SetApprovalNotifier(multiNotifier)
		automationHandler = workflows.NewAutomationHandler(db, redisClient, automationEngine, approvalManager)
	} else {
		logger.Warn("自动化工作流状态管理已禁用，原因：Redis 未连接")
	}
	authHandler := authHandlers.NewAuthHandler(jwtService, oauth2Service, sessionService, auditService, db, identityStore, stateStore)
	auditHandler := auditHandlers.NewAuditHandler(auditService)
	tenantHandler := tenantHandlers.NewTenantHandler(tenantService, userService, roleService, configService, hasher)

	// RAG 相关 Handlers
	kbHandler := knowledgeHandlers.NewKBHandler(kbService)
	documentHandler := knowledgeHandlers.NewDocumentHandler(docService, kbService, ragService)
	searchHandler := knowledgeHandlers.NewSearchHandler(kbService, ragService)

	// 工具相关 Handlers
	toolHandler := toolHandlers.NewToolHandler(toolRegistry, toolExecutor, db)
	notificationHandler := notificationHandlers.NewWebSocketHandler(wsHub)

	// 初始化权限检查器
	permChecker := auth.NewDatabasePermissionChecker(roleService)
	_ = auth.NewMiddlewareFactory(permChecker) // 暂时未使用，但已初始化供后续使用

	// 加载工作流模板与 Agent 能力配置
	workflowInitializer := workflowTpl.NewSystemInitializer()
	if err := workflowInitializer.Initialize("config"); err != nil {
		logger.Warn("初始化工作流模板/能力失败", zap.Error(err))
	}
	wfTemplateHandler := workflows.NewTemplateHandler(db, workflowInitializer.GetTemplateLoader(), workflowInitializer.GetCapabilityLoader())

	// 路由注册器，方便同时挂载 /api 与 /api/v1
	registerAPIRoutes := func(apiGroup *gin.RouterGroup) {
		apiGroup.GET("/ws/notifications", notificationHandler.Connect)

		// 租户管理
		tenants := apiGroup.Group("/tenants")
		{
			tenants.POST("", tenantHandler.CreateTenant)
			tenants.GET("", tenantHandler.ListTenants)
			tenants.GET("/:id", tenantHandler.GetTenant)
			tenants.PUT("/:id", tenantHandler.UpdateTenant)
			tenants.DELETE("/:id", tenantHandler.DeleteTenant)
		}

		// 租户用户管理（建议使用层级式路由）
		tenantUsers := tenants.Group("/:id/users")
		{
			tenantUsers.POST("", tenantHandler.CreateUser)
			tenantUsers.GET("", tenantHandler.ListUsers)
		}
		// 兼容旧路由 /tenant/users
		tenantUsersLegacy := apiGroup.Group("/tenant/users")
		{
			tenantUsersLegacy.POST("", tenantHandler.CreateUser)
			tenantUsersLegacy.GET("", tenantHandler.ListUsers)
		}

		// 租户角色管理
		tenantRoles := tenants.Group("/:id/roles")
		{
			tenantRoles.POST("", tenantHandler.CreateRole)
			tenantRoles.PUT("", tenantHandler.UpdateRole)
		}
		// 兼容旧路由 /tenant/roles
		tenantRolesLegacy := apiGroup.Group("/tenant/roles")
		{
			tenantRolesLegacy.POST("", tenantHandler.CreateRole)
			tenantRolesLegacy.PUT("", tenantHandler.UpdateRole)
		}

		// 租户配置管理
		tenantConfig := apiGroup.Group("/tenant/config")
		{
			tenantConfig.GET("", tenantHandler.GetConfig)
			tenantConfig.PUT("", tenantHandler.UpdateConfig)
		}

		// 权限查询
		apiGroup.GET("/tenant/permissions", tenantHandler.ListPermissions)

		// 模型管理 API
		modelsGroup := apiGroup.Group("/models")
		{
			modelsGroup.GET("", modelHandler.ListModels)
			modelsGroup.GET("/:id", modelHandler.GetModel)
			modelsGroup.POST("", modelHandler.CreateModel)
			modelsGroup.PUT("/:id", modelHandler.UpdateModel)
			modelsGroup.DELETE("/:id", modelHandler.DeleteModel)
			modelsGroup.GET("/:id/stats", modelHandler.GetModelStats)
			credentials := modelsGroup.Group("/:id/credentials")
			{
				credentials.GET("", modelHandler.ListModelCredentials)
				credentials.POST("", modelHandler.CreateModelCredential)
				credentials.DELETE("/:credentialId", modelHandler.DeleteModelCredential)
			}
			modelsGroup.POST("/seed", modelHandler.SeedDefaultModels)

			// 模型自动发现 API
			modelsGroup.POST("/discover/:provider", modelHandler.DiscoverModels)
			modelsGroup.POST("/discover-all", modelHandler.DiscoverAllModels)
		}

		// 模板管理 API
		templatesGroup := apiGroup.Group("/templates")
		{
			templatesGroup.GET("", templateHandler.ListTemplates)
			templatesGroup.GET("/:id", templateHandler.GetTemplate)
			templatesGroup.POST("", templateHandler.CreateTemplate)
			templatesGroup.PUT("/:id", templateHandler.UpdateTemplate)
			templatesGroup.DELETE("/:id", templateHandler.DeleteTemplate)
			templatesGroup.POST("/:id/versions", templateHandler.CreateVersion)
			templatesGroup.GET("/:id/versions/latest", templateHandler.GetLatestVersion)
			templatesGroup.POST("/:id/render", templateHandler.RenderTemplate)
		}

		// Agent 管理 API
		agentsGroup := apiGroup.Group("/agents")
		{
			agentsGroup.GET("", agentHandler.ListAgentConfigs)
			agentsGroup.GET("/:id", agentHandler.GetAgentConfig)
			agentsGroup.POST("", agentHandler.CreateAgentConfig)
			agentsGroup.PUT("/:id", agentHandler.UpdateAgentConfig)
			agentsGroup.DELETE("/:id", agentHandler.DeleteAgentConfig)
			agentsGroup.GET("/types/:type", agentHandler.GetAgentByType)
			agentsGroup.POST("/seed", agentHandler.SeedDefaultAgents)

			// Agent 能力目录
			agentsGroup.GET("/capabilities", wfTemplateHandler.GetAgentCapabilities)
			agentsGroup.GET("/capabilities/:agent_type/:role", wfTemplateHandler.GetRoleCapability)

			// Agent 执行 API
			agentsGroup.POST("/:id/execute", agentExecuteHandler.Execute)
			agentsGroup.POST("/:id/execute-stream", agentExecuteHandler.ExecuteStream)
			// 异步执行路由
			agentsGroup.POST("/:id/run", agentExecuteHandler.ExecuteAsync)

			agentsGroup.POST("/types/:type/execute", agentExecuteHandler.ExecuteByType)
			agentsGroup.POST("/types/:type/execute-stream", agentExecuteHandler.ExecuteByTypeStream)
		}

		// 工作流管理 API
		workflowsGroup := apiGroup.Group("/workflows")
		{
			workflowsGroup.GET("", workflowHandler.ListWorkflows)
			workflowsGroup.GET("/:id", workflowHandler.GetWorkflow)
			workflowsGroup.POST("", workflowHandler.CreateWorkflow)
			workflowsGroup.PUT("/:id", workflowHandler.UpdateWorkflow)
			workflowsGroup.DELETE("/:id", workflowHandler.DeleteWorkflow)
			workflowsGroup.POST("/validate", workflowHandler.ValidateWorkflow)
			workflowsGroup.GET("/:id/stats", workflowHandler.GetWorkflowStats)
			workflowsGroup.GET("/example", workflowHandler.GetExampleWorkflow)

			// 工作流执行 API
			workflowsGroup.POST("/:id/execute", workflowExecuteHandler.ExecuteWorkflow)
			workflowsGroup.GET("/:id/executions", workflowExecuteHandler.ListExecutions)
			if automationHandler != nil {
				workflowsGroup.POST("/:id/execute/automated", automationHandler.ExecuteAutomated)
				approvals := workflowsGroup.Group("/approvals")
				{
					approvals.GET("/pending", automationHandler.GetApprovalRequests)
					approvals.POST("/:id/approve", automationHandler.ApproveRequest)
					approvals.POST("/:id/reject", automationHandler.RejectRequest)
					approvals.POST("/:id/resend", automationHandler.ResendNotification)
				}
				workflowsGroup.GET("/executions/:id/state", automationHandler.GetExecutionState)
				workflowsGroup.GET("/executions/:id/automation-logs", automationHandler.GetAutomationLogs)
			}
		}

		// 执行记录 API
		executions := apiGroup.Group("/executions")
		{
			executions.GET("/:id", workflowExecuteHandler.GetExecution)
		}

		// 审计日志 API（需要管理员权限或仅查看自己的日志）
		auditGroup := apiGroup.Group("/audit")
		{
			auditGroup.POST("/logs/query", auditHandler.QueryLogs)
			auditGroup.GET("/logs/:id", auditHandler.GetLog)
			auditGroup.GET("/users/:userID/activity", auditHandler.GetUserActivity)
			auditGroup.GET("/my-activity", auditHandler.GetMyActivity)
		}

		// 知识库管理 API
		kbGroup := apiGroup.Group("/knowledge-bases")
		{
			kbGroup.POST("", kbHandler.Create)
			kbGroup.GET("", kbHandler.List)
			kbGroup.GET("/:id", kbHandler.Get)
			kbGroup.PUT("/:id", kbHandler.Update)
			kbGroup.DELETE("/:id", kbHandler.Delete)

			// 文档管理 API（挂载在知识库下）
			kbGroup.POST("/:id/documents", documentHandler.Upload)
			kbGroup.POST("/:id/documents/text", documentHandler.CreateTextDocument)
			kbGroup.GET("/:id/documents", documentHandler.ListDocuments)

			// 检索 API
			kbGroup.POST("/:id/search", searchHandler.Search)
			kbGroup.POST("/:id/context", searchHandler.GetContext)
		}

		// 文档 API（独立路由）
		docsGroup := apiGroup.Group("/documents")
		{
			docsGroup.GET("/:id", documentHandler.GetDocument)
			docsGroup.DELETE("/:id", documentHandler.DeleteDocument)
			docsGroup.GET("/:id/chunks", documentHandler.ListChunks)
		}

		// 工具管理 API
		toolsGroup := apiGroup.Group("/tools")
		{
			toolsGroup.GET("", toolHandler.ListTools)
			toolsGroup.POST("/register", toolHandler.RegisterTool)
			toolsGroup.GET("/categories/:category", toolHandler.ListToolsByCategory)
			toolsGroup.GET("/:name", toolHandler.GetTool)
			toolsGroup.DELETE("/:name", toolHandler.UnregisterTool)
			toolsGroup.POST("/:name/execute", toolHandler.ExecuteTool)
		}

		workspaceGroup := apiGroup.Group("/workspace")
		{
			workspaceGroup.GET("/tree", workspaceHandler.GetTree)
			workspaceGroup.POST("/folders", workspaceHandler.CreateFolder)
			workspaceGroup.PATCH("/nodes/:id", workspaceHandler.RenameNode)
			workspaceGroup.DELETE("/nodes/:id", workspaceHandler.DeleteNode)
			workspaceGroup.GET("/files/:id", workspaceHandler.GetFile)
			workspaceGroup.PUT("/files/:id", workspaceHandler.UpdateFile)
			workspaceGroup.GET("/staging", workspaceHandler.ListStaging)
			workspaceGroup.POST("/staging", workspaceHandler.CreateStaging)
			workspaceGroup.POST("/staging/:id/approve", workspaceHandler.ApproveStaging)
			workspaceGroup.POST("/staging/:id/reject", workspaceHandler.RejectStaging)
			workspaceGroup.POST("/context-links", workspaceHandler.AttachContext)
		}
	}

	// 认证 API（公开，不需要 JWT）
	authGroup := router.Group("/api/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)
		authGroup.POST("/logout", authHandler.Logout)
		authGroup.GET("/oauth/:provider", authHandler.GetOAuth2AuthURL)
		authGroup.POST("/oauth/:provider/callback", authHandler.OAuth2Callback)
	}

	// 主 API 组（向后兼容）
	api := router.Group("/api")
	api.Use(auth.AuthMiddleware(jwtService), middlewarepkg.GinTenantContextMiddleware(logger.Get()))
	registerAPIRoutes(api)

	// 版本化 API 组
	apiV1 := router.Group("/api/v1")
	apiV1.Use(auth.AuthMiddleware(jwtService), middlewarepkg.GinTenantContextMiddleware(logger.Get()))
	registerAPIRoutes(apiV1)

	// 初始化 Worker 服务器
	workerServer := worker.NewServer(redisCfg, ragService, workflowEngine, logger.Get())

	return router, workerServer
}

// RequestLogger 请求日志中间件
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录日志
		logger.Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowedOrigins := getEnvList("CORS_ALLOW_ORIGINS")
		origin := c.GetHeader("Origin")
		switch {
		case len(allowedOrigins) == 0:
			// 开发缺省：全部放行
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		case origin != "" && stringInSlice(origin, allowedOrigins):
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		default:
			// 未匹配则不设置 Allow-Origin，浏览器将拦截
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		allowedHeaders := defaultIfEmpty(
			getEnvList("CORS_ALLOW_HEADERS"),
			[]string{
				"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization",
				"Accept", "Origin", "Cache-Control", "X-Requested-With",
			},
		)
		c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))

		allowedMethods := defaultIfEmpty(
			getEnvList("CORS_ALLOW_METHODS"),
			[]string{"POST", "OPTIONS", "GET", "PUT", "DELETE", "PATCH"},
		)
		c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
		c.Writer.Header().Set("Access-Control-Max-Age", "600")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// ReadinessResponse 就绪检查响应
type ReadinessResponse struct {
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
	Database string `json:"database,omitempty"`
}

// HealthCheck 健康检查
// @Summary 服务健康检查
// @Description 返回基础健康状态，可供监控探针使用
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func HealthCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "AgentFlowCreativeHub",
		})
	}
}

// ReadinessCheck 就绪检查
// @Summary 服务就绪检查
// @Description 包含数据库连通性结果，用于判断可接收请求
// @Tags System
// @Produce json
// @Success 200 {object} ReadinessResponse
// @Failure 503 {object} ReadinessResponse
// @Router /ready [get]
func ReadinessCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查数据库连接
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{
				"status": "not_ready",
				"reason": "database connection error",
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{
				"status": "not_ready",
				"reason": "database ping failed",
			})
			return
		}

		c.JSON(200, gin.H{
			"status":   "ready",
			"database": "connected",
		})
	}
}

// getEnvList 读取逗号分隔的环境变量列表
func getEnvList(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var res []string
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			res = append(res, v)
		}
	}
	return res
}

// stringInSlice 判断字符串是否存在
func stringInSlice(target string, list []string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// defaultIfEmpty 返回非空列表或默认值
func defaultIfEmpty(list []string, def []string) []string {
	if len(list) == 0 {
		return def
	}
	return list
}

func normalizeRedisConfig(cfg config.RedisConfig) config.RedisConfig {
	resolved := cfg
	resolved.Host = strings.TrimSpace(resolved.Host)
	if resolved.Host == "" {
		if addr := strings.TrimSpace(os.Getenv("REDIS_ADDR")); addr != "" {
			host, port := parseRedisAddr(addr)
			if host != "" {
				resolved.Host = host
			}
			if resolved.Port == 0 && port > 0 {
				resolved.Port = port
			}
		}
	}
	if resolved.Host == "" {
		resolved.Host = "localhost"
	}
	if resolved.Port == 0 {
		resolved.Port = 6379
	}
	return resolved
}

func parseRedisAddr(addr string) (string, int) {
	if strings.TrimSpace(addr) == "" {
		return "", 0
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return strings.TrimSpace(addr), 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, 0
	}
	return host, port
}

// --- Helpers for Tenant Service Dependency Injection ---

func initVectorStore(cfg *config.Config, db *gorm.DB) (rag.VectorStore, error) {
	if cfg != nil {
		vsType := strings.ToLower(strings.TrimSpace(cfg.RAG.VectorStore.Type))
		
		// 显式指定使用 Qdrant
		if vsType == "qdrant" {
			qcfg := cfg.RAG.VectorStore.Qdrant
			if strings.TrimSpace(qcfg.Endpoint) == "" {
				return nil, fmt.Errorf("未配置 Qdrant endpoint")
			}
			opts := rag.QdrantOptions{
				Endpoint:        qcfg.Endpoint,
				APIKey:          qcfg.APIKey,
				Collection:      qcfg.Collection,
				VectorDimension: qcfg.VectorDimension,
				Distance:        qcfg.Distance,
				TimeoutSeconds:  qcfg.TimeoutSeconds,
			}
			return rag.NewQdrantStore(opts)
		}
	}
	// 默认使用 PGVector
	return rag.NewPGVectorStore(db)
}

type UUIDGenerator struct{}

func (g *UUIDGenerator) NewID() (string, error) {
	return uuid.New().String(), nil
}

type BcryptHasher struct{}

func (h *BcryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

type TenantAuditAdapter struct {
	svc *modelSvc.AuditLogService
}

func (a *TenantAuditAdapter) LogAction(ctx context.Context, tc tenantSvc.TenantContext, action, resource string, details any) {
	// 转换 details 为 metadata map
	metadata := make(map[string]interface{})
	if details != nil {
		// 简单的尝试转换，如果类型不匹配可能会有问题，生产环境建议更严谨
		if m, ok := details.(map[string]interface{}); ok {
			metadata = m
		} else {
			// 尝试 JSON 序列化再反序列化
			if b, err := json.Marshal(details); err == nil {
				_ = json.Unmarshal(b, &metadata)
			}
		}
	}

	// 记录审计日志
	log := &modelSvc.AuditLog{
		TenantID:      tc.TenantID,
		UserID:        tc.UserID,
		EventType:     action,
		EventCategory: resource,
		EventLevel:    "info",
		Description:   "Action: " + action + " on " + resource,
		Metadata:      metadata,
		CreatedAt:     time.Now().UTC(),
	}

	// 异步写入，忽略错误以避免阻塞业务流程
	// 在实际生产中，可能希望使用带缓冲的 channel 或专门的 logging pipeline
	go func() {
		// 创建一个新的 context，因为原来的 request context 可能已取消
		_ = a.svc.CreateLog(context.Background(), log)
	}()
}
