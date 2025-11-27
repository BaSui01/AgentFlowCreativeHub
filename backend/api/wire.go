package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"backend/api/handlers/agents"
	auditHandlers "backend/api/handlers/audit"
	authHandlers "backend/api/handlers/auth"
	commandHandlers "backend/api/handlers/commands"
	filesHandlers "backend/api/handlers/files"
	knowledgeHandlers "backend/api/handlers/knowledge"
	"backend/api/handlers/models"
	notificationHandlers "backend/api/handlers/notifications"
	"backend/api/handlers/templates"
	tenantHandlers "backend/api/handlers/tenant"
	toolHandlers "backend/api/handlers/tools"
	"backend/api/handlers/workflows"
	workspaceHandlers "backend/api/handlers/workspace"

	agentSvc "backend/internal/agent"
	"backend/internal/agent/runtime"
	"backend/internal/ai"
	auditpkg "backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/command"
	"backend/internal/config"
	"backend/internal/infra/queue"
	"backend/internal/logger"
	modelSvc "backend/internal/models"
	"backend/internal/notification"
	"backend/internal/rag"
	templateSvc "backend/internal/template"
	tenantSvc "backend/internal/tenant"
	"backend/internal/tools"
	"backend/internal/tools/builtin"
	"backend/internal/worker"
	workflowSvc "backend/internal/workflow"
	"backend/internal/workflow/approval"
	"backend/internal/workflow/executor"
	workflowTpl "backend/internal/workflow/template"
	workspaceSvc "backend/internal/workspace"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AppContainer 应用容器，集中管理所有服务依赖
type AppContainer struct {
	// 基础设施
	DB          *gorm.DB
	Config      *config.Config
	RedisClient *redis.Client
	QueueClient queue.Client

	// 认证相关
	JWTService    *auth.JWTService
	OAuth2Service *auth.OAuth2Service
	IdentityStore *auth.IdentityStore
	StateStore    auth.StateStore

	// 租户相关
	TenantService tenantSvc.TenantService
	UserService   tenantSvc.UserService
	RoleService   tenantSvc.RoleService
	ConfigService tenantSvc.TenantConfigService
	Hasher        *BcryptHasher

	// 核心服务 (使用接口类型以提升可测试性和可维护性)
	ModelService           modelSvc.ModelServiceInterface
	ModelCredentialService modelSvc.ModelCredentialServiceInterface
	ModelDiscoveryService  modelSvc.ModelDiscoveryServiceInterface
	TemplateService        *templateSvc.TemplateService // TODO: 待接口化
	AgentService           agentSvc.AgentServiceInterface
	WorkflowService        workflowSvc.WorkflowServiceInterface
	WorkspaceService       *workspaceSvc.Service // TODO: 待接口化
	SessionService         modelSvc.SessionServiceInterface
	AuditService           modelSvc.AuditLogServiceInterface
	CommandService         *command.Service // TODO: 待接口化

	// RAG 相关
	KBService   modelSvc.KnowledgeBaseServiceInterface
	DocService  modelSvc.DocumentServiceInterface
	RAGService  *rag.RAGService   // TODO: 待接口化
	VectorStore rag.VectorStore

	// Agent 运行时
	AgentRegistry *runtime.Registry
	AsyncClient   *runtime.AsyncClient
	ClientFactory *ai.ClientFactory

	// 工具相关
	ToolRegistry *tools.ToolRegistry
	ToolExecutor *tools.ToolExecutor

	// 工作流
	WorkflowEngine   *executor.Engine
	AutomationEngine *executor.AutomationEngine
	ApprovalManager  *approval.Manager

	// 通知
	WSHub                      *notification.WebSocketHub
	MultiNotifier              *notification.MultiNotifier
	NotificationConfigService  notification.NotificationConfigServiceInterface

	// 权限
	PermMiddleware *auth.MiddlewareFactory

	// 工作流模板
	WorkflowInitializer *workflowTpl.SystemInitializer

	// Worker
	WorkerServer *worker.Server
}

type Handlers struct {
	Model              *models.ModelHandler
	Template           *templates.TemplateHandler
	Agent              *agents.AgentHandler
	AgentExecute       *agents.AgentExecuteHandler
	Workflow           *workflows.WorkflowHandler
	WfExecute          *workflows.WorkflowExecuteHandler
	Automation         *workflows.AutomationHandler
	WfTemplate         *workflows.TemplateHandler
	Workspace          *workspaceHandlers.Handler
	Artifact           *workspaceHandlers.ArtifactHandler
	Commands           *commandHandlers.Handler
	Files              *filesHandlers.Handler
	Auth               *authHandlers.AuthHandler
	Audit              *auditHandlers.AuditHandler
	Tenant             *tenantHandlers.TenantHandler
	KB                 *knowledgeHandlers.KBHandler
	Document           *knowledgeHandlers.DocumentHandler
	Search             *knowledgeHandlers.SearchHandler
	Tool               *toolHandlers.ToolHandler
	Notification       *notificationHandlers.WebSocketHandler
	NotificationConfig *notificationHandlers.NotificationConfigHandler
}


// InitContainer 初始化应用容器
func InitContainer(db *gorm.DB, cfg *config.Config) (*AppContainer, error) {
	container := &AppContainer{
		DB:     db,
		Config: cfg,
	}

	// 初始化 Redis
	if err := container.initRedis(cfg); err != nil {
		return nil, err
	}

	// 初始化认证服务
	if err := container.initAuth(cfg); err != nil {
		return nil, err
	}

	// 初始化租户服务
	if err := container.initTenant(db); err != nil {
		return nil, err
	}

	// 初始化核心服务
	if err := container.initCoreServices(db, cfg); err != nil {
		return nil, err
	}

	// 初始化 Agent 运行时
	if err := container.initAgentRuntime(db, cfg); err != nil {
		return nil, err
	}

	// 初始化工作流
	if err := container.initWorkflow(db, cfg); err != nil {
		return nil, err
	}

	// 初始化通知系统
	container.initNotification()

	// 初始化权限检查器
	container.initPermissions()

	// 初始化 Worker
	container.initWorker(cfg)

	return container, nil
}

// InitHandlers 初始化所有 Handlers
func (c *AppContainer) InitHandlers() *Handlers {
	h := &Handlers{}

	h.Model = models.NewModelHandler(c.ModelService, c.ModelDiscoveryService, c.ModelCredentialService)
	h.Template = templates.NewTemplateHandler(c.TemplateService)
	h.Agent = agents.NewAgentHandler(c.AgentService)
	h.AgentExecute = agents.NewAgentExecuteHandler(c.AgentRegistry, c.AsyncClient)
	h.Workflow = workflows.NewWorkflowHandler(c.WorkflowService)
	h.WfExecute = workflows.NewWorkflowExecuteHandler(c.WorkflowEngine, c.DB)
	h.Workspace = workspaceHandlers.NewHandler(c.WorkspaceService, c.ToolExecutor, c.AgentRegistry)
	h.Artifact = workspaceHandlers.NewArtifactHandler(c.WorkspaceService)
	h.Commands = commandHandlers.NewHandler(c.CommandService, c.AsyncClient)
	h.Files = filesHandlers.NewHandler(c.WorkspaceService)
	h.Auth = authHandlers.NewAuthHandler(c.JWTService, c.OAuth2Service, c.SessionService, c.AuditService, c.DB, c.IdentityStore, c.StateStore)
	h.Audit = auditHandlers.NewAuditHandler(c.AuditService)
	h.Tenant = tenantHandlers.NewTenantHandler(c.TenantService, c.UserService, c.RoleService, c.ConfigService, c.Hasher)
	h.KB = knowledgeHandlers.NewKBHandler(c.KBService)
	h.Document = knowledgeHandlers.NewDocumentHandler(c.DocService, c.KBService, c.RAGService)
	h.Search = knowledgeHandlers.NewSearchHandler(c.KBService, c.RAGService)
	h.Tool = toolHandlers.NewToolHandler(c.ToolRegistry, c.ToolExecutor, c.DB)
	h.Notification = notificationHandlers.NewWebSocketHandler(c.WSHub)
	h.NotificationConfig = notificationHandlers.NewNotificationConfigHandler(c.NotificationConfigService)

	// 自动化 Handler（需要 Redis）
	if c.RedisClient != nil && c.AutomationEngine != nil {
		h.Automation = workflows.NewAutomationHandler(c.DB, c.RedisClient, c.AutomationEngine, c.ApprovalManager)
	}

	// 工作流模板 Handler
	h.WfTemplate = workflows.NewTemplateHandler(c.DB, c.WorkflowInitializer.GetTemplateLoader(), c.WorkflowInitializer.GetCapabilityLoader())

	return h
}

// --- 内部初始化方法 ---

func (c *AppContainer) initRedis(cfg *config.Config) error {
	redisCfg := normalizeRedisConfig(cfg.Redis)
	cfg.Redis = redisCfg
	c.QueueClient = queue.NewClient(redisCfg)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port),
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Redis 不可用，自动化审批与 OAuth2 状态将退回内存实现", zap.Error(err))
		c.RedisClient = nil
	} else {
		c.RedisClient = redisClient
	}

	return nil
}

func (c *AppContainer) initAuth(cfg *config.Config) error {
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	if appEnv == "" {
		appEnv = "dev"
	}

	jwtSecretKey := strings.TrimSpace(os.Getenv("JWT_SECRET_KEY"))
	if jwtSecretKey == "" {
		if strings.EqualFold(cfg.Server.Mode, "release") || strings.EqualFold(appEnv, "prod") || strings.EqualFold(appEnv, "production") {
			logger.Fatal("JWT_SECRET_KEY 未配置，生产环境禁止使用默认密钥")
		}
		jwtSecretKey = "default_jwt_secret_key_change_in_production"
		logger.Warn("JWT_SECRET_KEY 未配置，已回退为开发默认值，请在生产环境设置强随机密钥")
	}

	c.JWTService = auth.NewJWTService(jwtSecretKey, "AgentFlowCreativeHub", c.RedisClient)
	c.OAuth2Service = auth.NewOAuth2Service()

	// 注册 OAuth2 提供商
	if googleClientID := os.Getenv("GOOGLE_CLIENT_ID"); googleClientID != "" {
		c.OAuth2Service.RegisterProvider(auth.ProviderGoogle, &auth.OAuth2Config{
			ClientID:     googleClientID,
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		})
	}
	if githubClientID := os.Getenv("GITHUB_CLIENT_ID"); githubClientID != "" {
		c.OAuth2Service.RegisterProvider(auth.ProviderGitHub, &auth.OAuth2Config{
			ClientID:     githubClientID,
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GITHUB_REDIRECT_URL"),
		})
	}

	c.IdentityStore = auth.NewIdentityStore(c.DB, os.Getenv("DEFAULT_TENANT_ID"))
	if c.RedisClient != nil {
		c.StateStore = auth.NewRedisStateStore(c.RedisClient)
	} else {
		c.StateStore = auth.NewMemoryStateStore(10 * time.Minute)
	}

	return nil
}

func (c *AppContainer) initTenant(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层数据库连接失败: %w", err)
	}

	c.SessionService = modelSvc.NewSessionService(db)
	c.AuditService = modelSvc.NewAuditLogService(db)

	idGen := &UUIDGenerator{}
	c.Hasher = &BcryptHasher{}
	auditAdapter := &TenantAuditAdapter{svc: c.AuditService}

	tenantRepo := tenantSvc.NewTenantRepository(sqlDB)
	userRepo := tenantSvc.NewUserRepository(sqlDB)
	roleRepo := tenantSvc.NewRoleRepository(sqlDB)
	userRoleRepo := tenantSvc.NewUserRoleRepository(sqlDB)
	rolePermRepo := tenantSvc.NewRolePermissionRepository(sqlDB)
	permRepo := tenantSvc.NewPermissionRepository(sqlDB)
	configRepo := tenantSvc.NewTenantConfigRepository(sqlDB)
	configCache := tenantSvc.NewInMemoryTenantConfigCache(0)

	c.TenantService = tenantSvc.NewTenantService(tenantRepo, userRepo, nil, idGen, nil, auditAdapter)
	c.UserService = tenantSvc.NewUserService(userRepo, c.Hasher, auditAdapter)
	c.RoleService = tenantSvc.NewRoleService(roleRepo, userRoleRepo, rolePermRepo, permRepo, idGen, auditAdapter)
	c.ConfigService = tenantSvc.NewTenantConfigService(configRepo, configCache, auditAdapter)

	if err := tenantSvc.EnsureSystemPermissions(db); err != nil {
		return fmt.Errorf("初始化系统权限失败: %w", err)
	}

	return nil
}

func (c *AppContainer) initCoreServices(db *gorm.DB, cfg *config.Config) error {
	c.ModelService = modelSvc.NewModelService(db)
	c.ModelCredentialService = modelSvc.NewModelCredentialService(db)
	c.ModelDiscoveryService = modelSvc.NewModelDiscoveryService(db, nil)
	c.TemplateService = templateSvc.NewTemplateService(db)
	c.AgentService = agentSvc.NewAgentService(db)
	c.WorkflowService = workflowSvc.NewWorkflowService(db)
	c.WorkspaceService = workspaceSvc.NewService(db)

	// RAG 相关
	c.KBService = modelSvc.NewKnowledgeBaseService(db)
	c.DocService = modelSvc.NewDocumentService(db)

	var err error
	c.VectorStore, err = initVectorStore(cfg, db)
	if err != nil {
		return fmt.Errorf("初始化向量存储失败: %w", err)
	}

	embeddingProvider := rag.NewOpenAIEmbeddingProvider(os.Getenv("OPENAI_API_KEY"), "")
	chunker := rag.NewChunker(500, 50)
	c.RAGService = rag.NewRAGService(db, c.VectorStore, embeddingProvider, chunker, c.QueueClient)
	keywordSearcher := rag.NewPostgresKeywordSearcher(db)
	c.RAGService.WithKeywordSearcher(keywordSearcher)

	// 命令服务
	c.CommandService = command.NewService(db, c.WorkspaceService)
	if err := c.CommandService.AutoMigrate(); err != nil {
		return fmt.Errorf("初始化命令请求表失败: %w", err)
	}

	return nil
}

func (c *AppContainer) initAgentRuntime(db *gorm.DB, cfg *config.Config) error {
	dbLogger := ai.NewDBLogger(db)
	c.ClientFactory = ai.NewClientFactory(db, dbLogger)

	c.AgentRegistry = runtime.NewRegistry(db, c.ClientFactory)

	ragHelper := runtime.NewRAGHelper(c.RAGService)
	c.AgentRegistry.SetRAGHelper(ragHelper)

	embeddingProvider := rag.NewOpenAIEmbeddingProvider(os.Getenv("OPENAI_API_KEY"), "")
	memoryService := runtime.NewRAGMemoryService(c.VectorStore, embeddingProvider)
	c.AgentRegistry.SetMemoryService(memoryService)

	logService := runtime.NewPGLogService(db)

	c.AsyncClient = runtime.NewAsyncClient(cfg.Redis)
	asyncWorker := runtime.NewAsyncWorker(cfg.Redis, c.AgentRegistry, logService, c.CommandService)
	if err := asyncWorker.Start(); err != nil {
		logger.Error("Failed to start AsyncWorker", zap.Error(err))
	}

	// 工具注册
	c.ToolRegistry = tools.NewToolRegistry()
	if err := builtin.RegisterAll(c.ToolRegistry); err != nil {
		logger.Error("Failed to register builtin tools", zap.Error(err))
	}

	workspaceTool := builtin.NewWorkspacePublishTool(c.WorkspaceService)
	if err := c.ToolRegistry.Register(workspaceTool.GetDefinition().Name, workspaceTool, workspaceTool.GetDefinition()); err != nil {
		logger.Error("Failed to register workspace tool", zap.Error(err))
	}

	mcpBasePath := strings.TrimSpace(os.Getenv("MCP_BASE_PATH"))
	if mcpBasePath == "" {
		mcpBasePath = "."
	}
	if err := builtin.RegisterMCPTools(c.ToolRegistry, mcpBasePath, nil, nil, nil); err != nil {
		logger.Error("Failed to register MCP tools", zap.Error(err))
	}

	c.ToolExecutor = tools.NewToolExecutor(c.ToolRegistry, db)

	toolHelper := runtime.NewToolHelper(c.ToolRegistry, c.ToolExecutor)
	c.AgentRegistry.SetToolHelper(toolHelper)

	return nil
}

func (c *AppContainer) initWorkflow(db *gorm.DB, cfg *config.Config) error {
	workflowMaxConcurrency := 0
	if value := strings.TrimSpace(os.Getenv("WORKFLOW_MAX_CONCURRENCY")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			workflowMaxConcurrency = parsed
		} else {
			logger.Warn("无效的 WORKFLOW_MAX_CONCURRENCY，使用默认值", zap.String("value", value), zap.Error(err))
		}
	}

	tokenAuditService := auditpkg.NewTokenAuditService(db)
	c.WorkflowEngine = executor.NewEngine(db, c.AgentRegistry, c.QueueClient, tokenAuditService, executor.WithMaxConcurrency(workflowMaxConcurrency))

	c.WorkflowInitializer = workflowTpl.NewSystemInitializer()
	if err := c.WorkflowInitializer.Initialize("config"); err != nil {
		logger.Warn("初始化工作流模板/能力失败", zap.Error(err))
	}

	return nil
}

func (c *AppContainer) initNotification() {
	var offlineStore notification.OfflineStore = notification.NewMemoryOfflineStore(100)
	if c.RedisClient != nil {
		offlineStore = notification.NewRedisOfflineStore(c.RedisClient, 200, time.Hour)
	}

	c.WSHub = notification.NewWebSocketHub(notification.WithOfflineStore(offlineStore))
	c.MultiNotifier = notification.NewMultiNotifier(nil, nil, c.WSHub)
	
	// 初始化通知配置服务
	c.NotificationConfigService = notification.NewNotificationConfigService(c.DB)

	targetResolver := approval.NewConfigTargetResolver(c.DB, c.ConfigService)
	c.ApprovalManager = approval.NewManager(
		c.DB,
		approval.WithNotifier(c.MultiNotifier),
		approval.WithTargetResolver(targetResolver),
	)

	// 自动化引擎（需要 Redis）
	if c.RedisClient != nil {
		workflowMaxConcurrency := 0
		if value := strings.TrimSpace(os.Getenv("WORKFLOW_MAX_CONCURRENCY")); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				workflowMaxConcurrency = parsed
			}
		}

		tokenAuditService := auditpkg.NewTokenAuditService(c.DB)
		c.AutomationEngine = executor.NewAutomationEngine(c.DB, c.RedisClient, c.AgentRegistry, c.QueueClient, tokenAuditService, c.ApprovalManager, executor.WithMaxConcurrency(workflowMaxConcurrency))
		c.AutomationEngine.SetApprovalNotifier(c.MultiNotifier)
	} else {
		logger.Warn("自动化工作流状态管理已禁用，原因：Redis 未连接")
	}
}

func (c *AppContainer) initPermissions() {
	permChecker := auth.NewDatabasePermissionChecker(c.RoleService)
	c.PermMiddleware = auth.NewMiddlewareFactory(permChecker)
}

func (c *AppContainer) initWorker(cfg *config.Config) {
	c.WorkerServer = worker.NewServer(cfg.Redis, c.RAGService, c.WorkflowEngine, logger.Get())
}

// --- 依赖注入辅助类型 ---

// UUIDGenerator UUID 生成器
type UUIDGenerator struct{}

func (g *UUIDGenerator) NewID() (string, error) {
	return uuid.New().String(), nil
}

// BcryptHasher 密码哈希器
type BcryptHasher struct{}

func (h *BcryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// TenantAuditAdapter 租户审计适配器
type TenantAuditAdapter struct {
	svc *modelSvc.AuditLogService
}

func (a *TenantAuditAdapter) LogAction(ctx context.Context, tc tenantSvc.TenantContext, action, resource string, details any) {
	metadata := make(map[string]interface{})
	if details != nil {
		if m, ok := details.(map[string]interface{}); ok {
			metadata = m
		} else {
			if b, err := json.Marshal(details); err == nil {
				_ = json.Unmarshal(b, &metadata)
			}
		}
	}

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

	go func() {
		_ = a.svc.CreateLog(context.Background(), log)
	}()
}
