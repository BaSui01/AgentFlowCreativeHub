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
	analyticsHandlers "backend/api/handlers/analytics"
	apikeyHandlers "backend/api/handlers/apikey"
	billingHandlers "backend/api/handlers/billing"
	bookparserHandlers "backend/api/handlers/bookparser"
	moderationHandlers "backend/api/handlers/moderation"
	worldbuilderHandlers "backend/api/handlers/worldbuilder"
	creditsHandlers "backend/api/handlers/credits"
	complianceHandlers "backend/api/handlers/compliance"
	contentHandlers "backend/api/handlers/content"
	subscriptionHandlers "backend/api/handlers/subscription"
	fragmentHandlers "backend/api/handlers/fragment"
	kbsharingHandlers "backend/api/handlers/kbsharing"
	memoHandlers "backend/api/handlers/memo"
	metricsHandlers "backend/api/handlers/metrics"
	multimodelHandlers "backend/api/handlers/multimodel"
	plotHandlers "backend/api/handlers/plot"
	userHandlers "backend/api/handlers/user"
	marketplaceHandlers "backend/internal/tools/marketplace"
	auditHandlers "backend/api/handlers/audit"
	authHandlers "backend/api/handlers/auth"
	cacheHandlers "backend/api/handlers/cache"
	codesearchHandlers "backend/api/handlers/codesearch"
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
	"backend/internal/analytics"
	"backend/internal/billing"
	"backend/internal/bookparser"
	"backend/internal/compliance"
	"backend/internal/content"
	"backend/internal/credits"
	"backend/internal/fragment"
	"backend/internal/multimodel"
	"backend/internal/plot"
	"backend/internal/memo"
	"backend/internal/metrics"
	"backend/internal/moderation"
	"backend/internal/user"
	"backend/internal/worldbuilder"
	"backend/internal/subscription"
	auditpkg "backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/cache"
	"backend/internal/codesearch"
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
	RedisClient redis.UniversalClient
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

	// 核心服务
	ModelService           *modelSvc.ModelService
	ModelCredentialService *modelSvc.ModelCredentialService
	ModelQuotaService      *modelSvc.ModelQuotaService
	ModelDiscoveryService  *modelSvc.ModelDiscoveryService
	TemplateService        *templateSvc.TemplateService
	AgentService           *agentSvc.AgentService
	WorkflowService        *workflowSvc.WorkflowService
	WorkspaceService       *workspaceSvc.Service
	SessionService         *modelSvc.SessionService
	AuditService           *modelSvc.AuditLogService
	CommandService         *command.Service

	// RAG 相关
	KBService   *modelSvc.KnowledgeBaseService
	DocService  *modelSvc.DocumentService
	RAGService  *rag.RAGService   // TODO: 待接口化
	VectorStore rag.VectorStore

	// Agent 运行时
	AgentRegistry *runtime.Registry
	AsyncClient   *runtime.AsyncClient
	ClientFactory *ai.ClientFactory
	
	// 缓存
	DiskCache     *cache.DiskCache        // L3硬盘缓存
	CacheMonitor  *cache.CacheMonitor     // 缓存监控服务

	// 代码搜索
	ACECodeSearchService      *codesearch.ACECodeSearchService
	CodebaseSearchService     *codesearch.CodebaseSearchService

	// 分析统计
	AnalyticsService *analytics.Service

	// 拆书服务
	BookParserService *bookparser.Service

	// 积分服务
	CreditsService *credits.Service

	// 计费服务
	BillingService *billing.Service

	// 内容审核服务
	ModerationService *moderation.Service

	// 世界观构建服务
	WorldBuilderService *worldbuilder.Service

	// 订阅服务
	SubscriptionService *subscription.Service

	// 内容管理服务
	ContentService *content.Service

	// 合规服务
	ComplianceService *compliance.Service

	// 片段管理服务
	FragmentService *fragment.Service

	// 多模型服务
	MultiModelService *multimodel.Service

	// 剧情推演服务
	PlotService *plot.Service

	// 消息服务
	MessageService *notification.MessageService

	// 工作空间模板服务
	WorkspaceTemplateService *workspaceSvc.TemplateService

	// 工具市场服务
	MarketplaceService *marketplaceHandlers.Service

	// API Key 服务
	APIKeyService *auth.APIKeyService

	// 用户资料服务
	UserProfileService *user.ProfileService

	// 指标统计服务
	MetricsService metrics.MetricsServiceInterface

	// 知识库共享服务
	KBSharingService *rag.KBSharingService

	// 备忘录服务
	MemoService *memo.MemoService

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
	WebhookService             *notification.WebhookService
	EmailService               *notification.EmailService

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
	AgentPerformance   *agents.PerformanceHandler
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
	Webhook            *notificationHandlers.WebhookHandler
	Cache              *cacheHandlers.CacheHandler
	CodeSearch         *codesearchHandlers.Handler
	Analytics          *analyticsHandlers.DashboardHandler
	BookParser         *bookparserHandlers.Handler
	Credits            *creditsHandlers.Handler
	Billing            *billingHandlers.Handler
	Moderation         *moderationHandlers.Handler
	WorldBuilder       *worldbuilderHandlers.Handler
	Subscription       *subscriptionHandlers.Handler
	Content            *contentHandlers.Handler
	Compliance         *complianceHandlers.Handler
	Fragment           *fragmentHandlers.Handler
	MultiModel         *multimodelHandlers.Handler
	Plot               *plotHandlers.Handler
	Marketplace        *marketplaceHandlers.Handler
	Message            *notificationHandlers.MessageHandler
	WorkspaceTemplate  *workspaceHandlers.TemplateHandler
	APIKey             *apikeyHandlers.Handler
	User               *userHandlers.Handler
	Metrics            *metricsHandlers.Handler
	KBSharing          *kbsharingHandlers.Handler
	Memo               *memoHandlers.Handler
	Quota              *models.QuotaHandler
	ApprovalRule       *workflows.ApprovalRuleHandler
}

// shouldAutoMigrate 检查是否应该执行自动迁移
func (c *AppContainer) shouldAutoMigrate() bool {
	return c.Config != nil && c.Config.Database.AutoMigrate
}

// autoMigrate 条件执行自动迁移
func (c *AppContainer) autoMigrate(migrator interface{ AutoMigrate() error }, name string) {
	if !c.shouldAutoMigrate() {
		return
	}
	if err := migrator.AutoMigrate(); err != nil {
		logger.Warn(name+"表迁移失败", zap.Error(err))
	}
}

// autoMigrateDB 条件执行 GORM 自动迁移
func (c *AppContainer) autoMigrateDB(db *gorm.DB, name string, models ...interface{}) {
	if !c.shouldAutoMigrate() {
		return
	}
	if err := db.AutoMigrate(models...); err != nil {
		logger.Warn(name+"表迁移失败", zap.Error(err))
	}
}

// InitContainer 初始化依赖注入容器
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
	h.AgentPerformance = agents.NewPerformanceHandler(agentSvc.NewPerformanceService(c.DB))
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
	h.Webhook = notificationHandlers.NewWebhookHandler(c.WebhookService)
	
	// 缓存统计 Handler
	if c.DiskCache != nil {
		h.Cache = cacheHandlers.NewCacheHandler(c.DiskCache)
	}

	// 代码搜索 Handler
	h.CodeSearch = codesearchHandlers.NewHandler(c.ACECodeSearchService, c.CodebaseSearchService)

	// 分析统计 Handler
	h.Analytics = analyticsHandlers.NewDashboardHandler(c.AnalyticsService)

	// 拆书服务 Handler
	h.BookParser = bookparserHandlers.NewHandler(c.BookParserService)

	// 积分 Handler
	h.Credits = creditsHandlers.NewHandler(c.CreditsService)

	// 计费 Handler
	h.Billing = billingHandlers.NewHandler(c.BillingService)

	// 内容审核 Handler
	h.Moderation = moderationHandlers.NewHandler(c.ModerationService)

	// 世界观构建 Handler
	h.WorldBuilder = worldbuilderHandlers.NewHandler(c.WorldBuilderService)

	// 订阅 Handler
	h.Subscription = subscriptionHandlers.NewHandler(c.SubscriptionService)

	// 内容管理 Handler
	h.Content = contentHandlers.NewHandler(c.ContentService)

	// 合规管理 Handler
	h.Compliance = complianceHandlers.NewHandler(c.ComplianceService)

	// 片段管理 Handler
	h.Fragment = fragmentHandlers.NewHandler(c.FragmentService)

	// 多模型抽卡 Handler
	h.MultiModel = multimodelHandlers.NewHandler(c.MultiModelService)

	// 剧情推演 Handler
	h.Plot = plotHandlers.NewHandler(c.PlotService)

	// 工具市场 Handler
	h.Marketplace = marketplaceHandlers.NewHandler(c.MarketplaceService)

	// 消息 Handler
	if c.MessageService != nil {
		h.Message = notificationHandlers.NewMessageHandler(c.MessageService)
	}

	// 工作空间模板 Handler
	if c.WorkspaceTemplateService != nil {
		h.WorkspaceTemplate = workspaceHandlers.NewTemplateHandler(c.WorkspaceTemplateService)
	}

	// 自动化 Handler（需要 Redis）
	if c.RedisClient != nil && c.AutomationEngine != nil {
		h.Automation = workflows.NewAutomationHandler(c.DB, c.RedisClient, c.AutomationEngine, c.ApprovalManager)
	}

	// 工作流模板 Handler
	h.WfTemplate = workflows.NewTemplateHandler(c.DB, c.WorkflowInitializer.GetTemplateLoader(), c.WorkflowInitializer.GetCapabilityLoader())

	// API Key Handler
	if c.APIKeyService != nil {
		h.APIKey = apikeyHandlers.NewHandler(c.APIKeyService)
	}

	// 用户资料 Handler
	if c.UserProfileService != nil {
		h.User = userHandlers.NewHandler(c.UserProfileService)
	}

	// 指标统计 Handler
	if c.MetricsService != nil {
		h.Metrics = metricsHandlers.NewHandler(c.MetricsService)
	}

	// 知识库共享 Handler
	if c.KBSharingService != nil {
		h.KBSharing = kbsharingHandlers.NewHandler(c.KBSharingService)
	}

	// 备忘录 Handler
	if c.MemoService != nil {
		h.Memo = memoHandlers.NewHandler(c.MemoService)
	}

	// 配额管理 Handler
	if c.ModelQuotaService != nil {
		h.Quota = models.NewQuotaHandler(c.ModelQuotaService)
	}

	// 审批规则 Handler
	h.ApprovalRule = workflows.NewApprovalRuleHandler(c.DB)

	return h
}

// --- 内部初始化方法 ---

func (c *AppContainer) initRedis(cfg *config.Config) error {
	redisCfg := normalizeRedisConfig(cfg.Redis)
	cfg.Redis = redisCfg
	c.QueueClient = queue.NewClient(redisCfg)

	// 根据模式创建 Redis 客户端
	mode := redisCfg.Mode
	if mode == "" {
		mode = "standalone"
	}

	var redisClient redis.UniversalClient

	switch mode {
	case "standalone":
		redisClient = redis.NewClient(&redis.Options{
			Addr:         fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port),
			Password:     redisCfg.Password,
			DB:           redisCfg.DB,
			PoolSize:     redisCfg.PoolSize,
			MinIdleConns: redisCfg.MinIdleConns,
			MaintNotificationsConfig: &maintnotifications.Config{
				Mode: maintnotifications.ModeDisabled,
			},
		})
		logger.Info("Redis 单节点模式初始化", zap.String("addr", fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port)))

	case "sentinel":
		if redisCfg.MasterName == "" || len(redisCfg.SentinelAddrs) == 0 {
			return fmt.Errorf("哨兵模式需要配置 master_name 和 sentinel_addrs")
		}
		redisClient = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       redisCfg.MasterName,
			SentinelAddrs:    redisCfg.SentinelAddrs,
			SentinelPassword: redisCfg.SentinelPassword,
			Password:         redisCfg.Password,
			DB:               redisCfg.DB,
			PoolSize:         redisCfg.PoolSize,
			MinIdleConns:     redisCfg.MinIdleConns,
		})
		logger.Info("Redis 哨兵模式初始化", zap.String("master", redisCfg.MasterName), zap.Strings("sentinels", redisCfg.SentinelAddrs))

	case "cluster":
		if len(redisCfg.ClusterAddrs) == 0 {
			return fmt.Errorf("集群模式需要配置 cluster_addrs")
		}
		redisClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        redisCfg.ClusterAddrs,
			Password:     redisCfg.Password,
			PoolSize:     redisCfg.PoolSize,
			MinIdleConns: redisCfg.MinIdleConns,
		})
		logger.Info("Redis 集群模式初始化", zap.Strings("addrs", redisCfg.ClusterAddrs))

	default:
		return fmt.Errorf("不支持的 Redis 模式: %s (可选: standalone, sentinel, cluster)", mode)
	}

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Redis 不可用，自动化审批与 OAuth2 状态将退回内存实现", zap.Error(err))
		c.RedisClient = nil
	} else {
		c.RedisClient = redisClient
		logger.Info("Redis 连接成功", zap.String("mode", mode))
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
	c.ModelQuotaService = modelSvc.NewModelQuotaService(db)
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

	// 代码搜索服务
	codeSearchBasePath := strings.TrimSpace(os.Getenv("CODE_SEARCH_BASE_PATH"))
	if codeSearchBasePath == "" {
		codeSearchBasePath = "."
	}
	c.ACECodeSearchService = codesearch.NewACECodeSearchService(codeSearchBasePath)
	
	// 语义代码搜索（需要 Embedding Provider）
	embProvider := rag.NewOpenAIEmbeddingProvider(os.Getenv("OPENAI_API_KEY"), "")
	c.CodebaseSearchService = codesearch.NewCodebaseSearchService(codeSearchBasePath, embProvider)

	// 分析统计服务
	c.AnalyticsService = analytics.NewService(db)

	return nil
}

func (c *AppContainer) initAgentRuntime(db *gorm.DB, cfg *config.Config) error {
	// 初始化硬盘缓存(L3)
	var diskCache *cache.DiskCache
	if cfg.Cache.Disk.Enabled {
		ttl, err := time.ParseDuration(cfg.Cache.Disk.TTL)
		if err != nil {
			logger.Warn("解析缓存TTL失败，使用默认值720h", zap.Error(err))
			ttl = 720 * time.Hour
		}
		
		diskCache, err = cache.NewDiskCache(
			cfg.Cache.Disk.DBPath,
			ttl,
			cfg.Cache.Disk.MaxSizeGB,
		)
		if err != nil {
			logger.Error("初始化硬盘缓存失败", zap.Error(err))
			// 不阻断启动，继续使用nil diskCache
		} else {
			logger.Info("硬盘缓存初始化成功",
				zap.String("db_path", cfg.Cache.Disk.DBPath),
				zap.Int("max_size_gb", cfg.Cache.Disk.MaxSizeGB),
				zap.Duration("ttl", ttl),
		)
			
			// TODO: 修复CacheMonitor功能
			// 启动缓存监控服务
			// monitorInterval := 5 * time.Minute // 默认5分钟
			// if cfg.Cache.Disk.MonitorEnabled {
			// 	if cfg.Cache.Disk.MonitorInterval != "" {
			// 		if interval, err := time.ParseDuration(cfg.Cache.Disk.MonitorInterval); err == nil {
			// 			monitorInterval = interval
			// 		} else {
			// 			logger.Warn("解析缓存监控间隔失败，使用默认值5m", zap.Error(err))
			// 		}
			// 	}
			// 	c.CacheMonitor = cache.NewCacheMonitor(logger.Get())
			// 	logger.Info("缓存监控服务已启动", zap.Duration("interval", monitorInterval))
			// }
		}
	}
	c.DiskCache = diskCache
	
	dbLogger := ai.NewDBLogger(db)
	c.ClientFactory = ai.NewClientFactory(db, dbLogger, diskCache)


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

	// 拆书服务（依赖 AgentRegistry 和 RAGService）
	c.BookParserService = bookparser.NewService(db, c.AgentRegistry, c.RAGService)
	if err := c.BookParserService.AutoMigrate(); err != nil {
		logger.Warn("拆书服务表迁移失败", zap.Error(err))
	}

	// 积分服务
	c.CreditsService = credits.NewService(db)
	// 自动迁移积分表
	if err := db.AutoMigrate(&credits.CreditAccount{}, &credits.CreditTransaction{}, &credits.CreditPricing{}); err != nil {
		logger.Warn("积分服务表迁移失败", zap.Error(err))
	}

	// 计费服务
	metricsService := metrics.NewMetricsService(db)
	c.BillingService = billing.NewService(db, c.CreditsService, metricsService)
	if err := c.BillingService.AutoMigrate(); err != nil {
		logger.Warn("计费服务表迁移失败", zap.Error(err))
	}

	// 内容审核服务
	c.ModerationService = moderation.NewService(db, c.AgentRegistry)
	if err := c.ModerationService.AutoMigrate(); err != nil {
		logger.Warn("内容审核服务表迁移失败", zap.Error(err))
	}

	// 世界观构建服务
	c.WorldBuilderService = worldbuilder.NewService(db, c.AgentRegistry)
	if err := c.WorldBuilderService.AutoMigrate(); err != nil {
		logger.Warn("世界观构建服务表迁移失败", zap.Error(err))
	}
	// 初始化内置模板
	c.WorldBuilderService.InitBuiltinTemplates(context.Background())

	// 订阅服务
	c.SubscriptionService = subscription.NewService(db)
	// 自动迁移订阅表
	if err := db.AutoMigrate(&subscription.SubscriptionPlan{}, &subscription.UserSubscription{}, &subscription.SubscriptionHistory{}); err != nil {
		logger.Warn("订阅服务表迁移失败", zap.Error(err))
	}

	// 内容管理服务
	c.ContentService = content.NewService(db)
	if err := c.ContentService.AutoMigrate(); err != nil {
		logger.Warn("内容管理服务表迁移失败", zap.Error(err))
	}

	// 合规服务
	c.ComplianceService = compliance.NewService(db)
	if err := c.ComplianceService.AutoMigrate(); err != nil {
		logger.Warn("合规服务表迁移失败", zap.Error(err))
	}

	// 片段管理服务
	c.FragmentService = fragment.NewService(db)
	if err := db.AutoMigrate(&fragment.Fragment{}); err != nil {
		logger.Warn("片段管理服务表迁移失败", zap.Error(err))
	}

	// 多模型抽卡服务（依赖 AgentRegistry 和 ModelService）
	c.MultiModelService = multimodel.NewService(db, c.AgentRegistry, c.ModelService)
	if err := db.AutoMigrate(&multimodel.DrawHistory{}); err != nil {
		logger.Warn("多模型服务表迁移失败", zap.Error(err))
	}

	// 剧情推演服务（依赖 AgentRegistry 和 WorkspaceService）
	c.PlotService = plot.NewService(db, c.AgentRegistry, c.WorkspaceService)
	if err := db.AutoMigrate(&plot.PlotRecommendation{}); err != nil {
		logger.Warn("剧情推演服务表迁移失败", zap.Error(err))
	}

	// 消息服务
	c.MessageService = notification.NewMessageService(db)
	if err := c.MessageService.AutoMigrate(); err != nil {
		logger.Warn("消息服务表迁移失败", zap.Error(err))
	}

	// 工作空间模板服务
	c.WorkspaceTemplateService = workspaceSvc.NewTemplateService(db, c.WorkspaceService)
	if err := c.WorkspaceTemplateService.AutoMigrate(); err != nil {
		logger.Warn("工作空间模板服务表迁移失败", zap.Error(err))
	}
	// 初始化内置模板
	c.WorkspaceTemplateService.InitBuiltinTemplates(context.Background())

	// 工具市场服务
	c.MarketplaceService = marketplaceHandlers.NewService(db)
	if err := c.MarketplaceService.AutoMigrate(); err != nil {
		logger.Warn("工具市场服务表迁移失败", zap.Error(err))
	}

	// API Key 服务
	c.APIKeyService = auth.NewAPIKeyService(db)

	// 指标统计服务
	c.MetricsService = metrics.NewMetricsService(db)

	// 知识库共享服务
	c.KBSharingService = rag.NewKBSharingService(nil, nil, 5*time.Minute)

	// 备忘录服务
	c.MemoService = memo.NewMemoService(nil)

	// 用户资料服务
	c.UserProfileService = user.NewProfileService(nil)

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

	// 初始化Webhook服务
	c.WebhookService = notification.NewWebhookService(3)

	// 初始化邮件服务
	emailConfig := &notification.EmailServiceConfig{
		SMTPHost:    os.Getenv("SMTP_HOST"),
		SMTPPort:    587,
		Username:    os.Getenv("SMTP_USERNAME"),
		Password:    os.Getenv("SMTP_PASSWORD"),
		FromAddress: os.Getenv("SMTP_FROM_ADDRESS"),
		FromName:    os.Getenv("SMTP_FROM_NAME"),
		UseTLS:      os.Getenv("SMTP_USE_TLS") != "false",
		MaxRetries:  3,
		QueueSize:   1000,
		Workers:     3,
	}
	if port := os.Getenv("SMTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			emailConfig.SMTPPort = p
		}
	}
	c.EmailService = notification.NewEmailService(c.DB, emailConfig)

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

