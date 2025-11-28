package api

import (
	"backend/internal/auth"
	middlewarepkg "backend/internal/middleware"

	"backend/internal/logger"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(router *gin.Engine, container *AppContainer, handlers *Handlers) {
	// 认证 API（公开，不需要 JWT）
	registerAuthRoutes(router, handlers)

	// 主 API 组（向后兼容）
	api := router.Group("/api")
	api.Use(auth.AuthMiddleware(container.JWTService), middlewarepkg.GinTenantContextMiddleware(logger.Get()))
	registerAPIRoutes(api, container, handlers)

	// 版本化 API 组
	apiV1 := router.Group("/api/v1")
	apiV1.Use(auth.AuthMiddleware(container.JWTService), middlewarepkg.GinTenantContextMiddleware(logger.Get()))
	registerAPIRoutes(apiV1, container, handlers)
}

// registerAuthRoutes 注册认证相关路由（公开）
func registerAuthRoutes(router *gin.Engine, h *Handlers) {
	authGroup := router.Group("/api/auth")
	{
		authGroup.POST("/login", h.Auth.Login)
		authGroup.POST("/register", h.Auth.Register)
		authGroup.POST("/refresh", h.Auth.Refresh)
		authGroup.POST("/logout", h.Auth.Logout)
		authGroup.POST("/forgot-password", h.Auth.ForgotPassword)
		authGroup.POST("/reset-password", h.Auth.ResetPassword)
		authGroup.GET("/oauth/:provider", h.Auth.GetOAuth2AuthURL)
		authGroup.POST("/oauth/:provider/callback", h.Auth.OAuth2Callback)
	}
}

// registerAPIRoutes 注册需要认证的 API 路由
func registerAPIRoutes(apiGroup *gin.RouterGroup, c *AppContainer, h *Handlers) {
	// WebSocket
	apiGroup.GET("/ws/notifications", h.Notification.Connect)

	// 权限守卫
	manageRolesGuard := c.PermMiddleware.RequirePermission("tenant", "manage_roles")
	commandAdminGuard := c.PermMiddleware.RequirePermission("command", "admin")
	adminGuard := c.PermMiddleware.RequireAdmin()

	// 租户管理
	registerTenantRoutes(apiGroup, h, manageRolesGuard)

	// 通知配置
	registerNotificationRoutes(apiGroup, h)
	
	// 缓存统计
	registerCacheRoutes(apiGroup, h, adminGuard)

	// 代码搜索
	registerCodeSearchRoutes(apiGroup, h, adminGuard)

	// Agent 管理
	registerAgentRoutes(apiGroup, h, adminGuard)

	// 工作流管理
	registerWorkflowRoutes(apiGroup, h, adminGuard)
	
	// 审计日志
	registerAuditRoutes(apiGroup, h, adminGuard)

	// 知识库管理
	registerKnowledgeRoutes(apiGroup, h)

	// 工具管理
	registerToolRoutes(apiGroup, h, adminGuard)

	// 文件管理
	registerFileRoutes(apiGroup, h)

	// 命令管理
	registerCommandRoutes(apiGroup, h, commandAdminGuard)

	// 工作空间
	registerWorkspaceRoutes(apiGroup, h)

	// 写作分析与统计
	registerAnalyticsRoutes(apiGroup, h)

	// 拆书系统
	registerBookParserRoutes(apiGroup, h)

	// 积分系统
	registerCreditsRoutes(apiGroup, h, adminGuard)

	// 计费系统
	registerBillingRoutes(apiGroup, h, adminGuard)

	// 内容审核系统
	registerModerationRoutes(apiGroup, h, adminGuard)

	// 世界观构建系统
	registerWorldBuilderRoutes(apiGroup, h)

	// 订阅系统
	registerSubscriptionRoutes(apiGroup, h, adminGuard)

	// 内容管理系统
	registerContentRoutes(apiGroup, h, adminGuard)

	// 合规管理系统
	registerComplianceRoutes(apiGroup, h, adminGuard)

	// 片段管理系统
	registerFragmentRoutes(apiGroup, h)

	// 多模型抽卡系统
	registerMultiModelRoutes(apiGroup, h)

	// 剧情推演系统
	registerPlotRoutes(apiGroup, h)

	// 消息管理系统
	registerMessageRoutes(apiGroup, h)

	// 工作空间模板系统
	registerWorkspaceTemplateRoutes(apiGroup, h)

	// 工具市场
	registerMarketplaceRoutes(apiGroup, h, adminGuard)
}




func registerTenantRoutes(apiGroup *gin.RouterGroup, h *Handlers, manageRolesGuard gin.HandlerFunc) {
	tenants := apiGroup.Group("/tenants")
	{
		tenants.POST("", h.Tenant.CreateTenant)
		tenants.GET("", h.Tenant.ListTenants)
		tenants.GET("/:id", h.Tenant.GetTenant)
		tenants.PUT("/:id", h.Tenant.UpdateTenant)
		tenants.DELETE("/:id", h.Tenant.DeleteTenant)
	}

	// 租户用户管理
	tenantUsers := tenants.Group("/:id/users")
	{
		tenantUsers.POST("", h.Tenant.CreateUser)
		tenantUsers.GET("", h.Tenant.ListUsers)
	}

	// 兼容旧路由
	tenantUsersLegacy := apiGroup.Group("/tenant/users")
	{
		tenantUsersLegacy.POST("", h.Tenant.CreateUser)
		tenantUsersLegacy.GET("", h.Tenant.ListUsers)
	}

	// 租户角色管理
	tenantRoles := tenants.Group("/:id/roles")
	tenantRoles.Use(manageRolesGuard)
	{
		tenantRoles.POST("", h.Tenant.CreateRole)
		tenantRoles.PUT("", h.Tenant.UpdateRole)
		tenantRoles.GET("", h.Tenant.ListRoles)
		tenantRoles.DELETE("/:roleId", h.Tenant.DeleteRole)
	}

	// 兼容旧路由
	tenantRolesLegacy := apiGroup.Group("/tenant/roles")
	tenantRolesLegacy.Use(manageRolesGuard)
	{
		tenantRolesLegacy.POST("", h.Tenant.CreateRole)
		tenantRolesLegacy.PUT("", h.Tenant.UpdateRole)
	}

	// 用户角色管理
	tenantUserRoles := tenants.Group("/:id/users/:userId/roles")
	tenantUserRoles.Use(manageRolesGuard)
	{
		tenantUserRoles.GET("", h.Tenant.ListUserRoles)
		tenantUserRoles.PUT("", h.Tenant.ReplaceUserRoles)
	}

	// 租户配置管理
	tenantConfig := apiGroup.Group("/tenant/config")
	{
		tenantConfig.GET("", h.Tenant.GetConfig)
		tenantConfig.PUT("", h.Tenant.UpdateConfig)
	}

	// 权限查询
	apiGroup.GET("/tenant/permissions", manageRolesGuard, h.Tenant.ListPermissions)
	apiGroup.GET("/tenant/permissions/catalog", manageRolesGuard, h.Tenant.GetPermissionCatalog)
	apiGroup.GET("/tenant/audit-logs", manageRolesGuard, h.Audit.GetTenantAuditLogs)
}

func registerModelRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	modelsGroup := apiGroup.Group("/models")
	{
		// 查询接口（用户可用）
		modelsGroup.GET("", h.Model.ListModels)
		modelsGroup.GET("/:id", h.Model.GetModel)
		modelsGroup.GET("/:id/stats", h.Model.GetModelStats)

		// 管理接口（需要管理员权限）
		modelsGroup.POST("", adminGuard, h.Model.CreateModel)
		modelsGroup.PUT("/:id", adminGuard, h.Model.UpdateModel)
		modelsGroup.DELETE("/:id", adminGuard, h.Model.DeleteModel)

		credentials := modelsGroup.Group("/:id/credentials")
		{
			credentials.GET("", h.Model.ListModelCredentials)
			credentials.POST("", adminGuard, h.Model.CreateModelCredential)
			credentials.DELETE("/:credentialId", adminGuard, h.Model.DeleteModelCredential)
		}

		modelsGroup.POST("/seed", adminGuard, h.Model.SeedDefaultModels)
		modelsGroup.POST("/discover/:provider", adminGuard, h.Model.DiscoverModels)
		modelsGroup.POST("/discover-all", adminGuard, h.Model.DiscoverAllModels)
	}
}

func registerTemplateRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	templatesGroup := apiGroup.Group("/templates")
	{
		templatesGroup.GET("", h.Template.ListTemplates)
		templatesGroup.GET("/:id", h.Template.GetTemplate)
		templatesGroup.POST("", h.Template.CreateTemplate)
		templatesGroup.PUT("/:id", h.Template.UpdateTemplate)
		templatesGroup.DELETE("/:id", h.Template.DeleteTemplate)
		templatesGroup.POST("/:id/versions", h.Template.CreateVersion)
		templatesGroup.GET("/:id/versions/latest", h.Template.GetLatestVersion)
		templatesGroup.POST("/:id/render", h.Template.RenderTemplate)
	}
}

func registerAgentRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	agentsGroup := apiGroup.Group("/agents")
	{
		// 用户接口（查询、执行）
		agentsGroup.GET("", h.Agent.ListAgentConfigs)
		agentsGroup.GET("/:id", h.Agent.GetAgentConfig)
		agentsGroup.GET("/types/:type", h.Agent.GetAgentByType)
		agentsGroup.GET("/capabilities", h.WfTemplate.GetAgentCapabilities)
		agentsGroup.GET("/capabilities/:agent_type/:role", h.WfTemplate.GetRoleCapability)

		// Agent 执行
		agentsGroup.POST("/:id/execute", h.AgentExecute.Execute)
		agentsGroup.POST("/:id/execute-stream", h.AgentExecute.ExecuteStream)
		agentsGroup.POST("/:id/run", h.AgentExecute.ExecuteAsync)
		agentsGroup.POST("/types/:type/execute", h.AgentExecute.ExecuteByType)
		agentsGroup.POST("/types/:type/execute-stream", h.AgentExecute.ExecuteByTypeStream)
		
		// Agent 执行历史
		agentsGroup.GET("/:id/executions", h.AgentExecute.ListExecutions)

		// 管理员接口（Agent配置管理）
		agentsGroup.POST("", adminGuard, h.Agent.CreateAgentConfig)
		agentsGroup.PUT("/:id", adminGuard, h.Agent.UpdateAgentConfig)
		agentsGroup.DELETE("/:id", adminGuard, h.Agent.DeleteAgentConfig)
		agentsGroup.POST("/seed", adminGuard, h.Agent.SeedDefaultAgents)

		// Agent性能分析（管理员接口）
		perfGroup := agentsGroup.Group("/performance")
		perfGroup.Use(adminGuard)
		{
			perfGroup.GET("/summary", h.AgentPerformance.GetPerformanceSummary)
			perfGroup.GET("/stats", h.AgentPerformance.GetAgentStats)
			perfGroup.GET("/trend", h.AgentPerformance.GetDailyTrend)
			perfGroup.GET("/comparison", h.AgentPerformance.GetAgentComparison)
			perfGroup.GET("/top", h.AgentPerformance.GetTopAgents)
			perfGroup.GET("/slowest", h.AgentPerformance.GetSlowestRuns)
			perfGroup.GET("/failed", h.AgentPerformance.GetFailedRuns)
			perfGroup.GET("/tokens", h.AgentPerformance.GetTokenUsageTrend)
		}
	}
}

func registerWorkflowRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	workflowsGroup := apiGroup.Group("/workflows")
	{
		// 用户接口（查询、执行）
		workflowsGroup.GET("", h.Workflow.ListWorkflows)
		workflowsGroup.GET("/:id", h.Workflow.GetWorkflow)
		workflowsGroup.POST("", h.Workflow.CreateWorkflow)
		workflowsGroup.PUT("/:id", h.Workflow.UpdateWorkflow)
		workflowsGroup.DELETE("/:id", h.Workflow.DeleteWorkflow)
		workflowsGroup.POST("/validate", h.Workflow.ValidateWorkflow)
		workflowsGroup.GET("/:id/stats", h.Workflow.GetWorkflowStats)
		workflowsGroup.GET("/example", h.Workflow.GetExampleWorkflow)

		// 工作流执行
		workflowsGroup.POST("/:id/execute", h.WfExecute.ExecuteWorkflow)
		workflowsGroup.GET("/:id/executions", h.WfExecute.ListExecutions)

		// 自动化工作流（可选）
		if h.Automation != nil {
			workflowsGroup.POST("/:id/execute/automated", h.Automation.ExecuteAutomated)
			workflowsGroup.GET("/executions/:id/state", h.Automation.GetExecutionState)
			workflowsGroup.GET("/executions/:id/automation-logs", h.Automation.GetAutomationLogs)

			// 管理员接口（审批管理）
			approvals := workflowsGroup.Group("/approvals")
			approvals.Use(adminGuard)
			{
				approvals.GET("/pending", h.Automation.GetApprovalRequests)
				approvals.POST("/:id/approve", h.Automation.ApproveRequest)
				approvals.POST("/:id/reject", h.Automation.RejectRequest)
				approvals.POST("/:id/resend", h.Automation.ResendNotification)
			}
		}
	}

	// 执行记录
	executions := apiGroup.Group("/executions")
	{
		executions.GET("/:id", h.WfExecute.GetExecution)
	}
}

func registerAuditRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	auditGroup := apiGroup.Group("/audit")
	{
		// 用户接口（查看自己的活动）
		auditGroup.GET("/my-activity", h.Audit.GetMyActivity)

		// 管理员接口（查询所有日志、查看其他用户活动、导出）
		auditGroup.POST("/logs/query", adminGuard, h.Audit.QueryLogs)
		auditGroup.GET("/logs/:id", adminGuard, h.Audit.GetLog)
		auditGroup.GET("/users/:userID/activity", adminGuard, h.Audit.GetUserActivity)
		auditGroup.POST("/logs/export", adminGuard, h.Audit.ExportLogs)
	}
}

func registerKnowledgeRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	kbGroup := apiGroup.Group("/knowledge-bases")
	{
		kbGroup.POST("", h.KB.Create)
		kbGroup.GET("", h.KB.List)
		kbGroup.GET("/:id", h.KB.Get)
		kbGroup.PUT("/:id", h.KB.Update)
		kbGroup.DELETE("/:id", h.KB.Delete)

		// 文档管理
		kbGroup.POST("/:id/documents", h.Document.Upload)
		kbGroup.POST("/:id/documents/text", h.Document.CreateTextDocument)
		kbGroup.GET("/:id/documents", h.Document.ListDocuments)

		// 检索
		kbGroup.POST("/:id/search", h.Search.Search)
		kbGroup.POST("/:id/context", h.Search.GetContext)
	}

	// 文档独立路由
	docsGroup := apiGroup.Group("/documents")
	{
		docsGroup.GET("/:id", h.Document.GetDocument)
		docsGroup.PUT("/:id", h.Document.UpdateDocument)
		docsGroup.DELETE("/:id", h.Document.DeleteDocument)
		docsGroup.GET("/:id/chunks", h.Document.ListChunks)
	}
}

func registerToolRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	toolsGroup := apiGroup.Group("/tools")
	{
		// 用户接口（查询、执行）
		toolsGroup.GET("", h.Tool.ListTools)
		toolsGroup.GET("/categories/:category", h.Tool.ListToolsByCategory)
		toolsGroup.GET("/:name", h.Tool.GetTool)
		toolsGroup.POST("/:name/execute", h.Tool.ExecuteTool)

		// 管理员接口（工具注册/注销）
		toolsGroup.POST("/register", adminGuard, h.Tool.RegisterTool)
		toolsGroup.DELETE("/:name", adminGuard, h.Tool.UnregisterTool)
	}
}

func registerFileRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	filesGroup := apiGroup.Group("/files")
	{
		filesGroup.GET("/tree", h.Files.GetTree)
		filesGroup.GET("/content", h.Files.GetContent)
		filesGroup.POST("", h.Files.CreateFile)
		filesGroup.PATCH("", h.Files.PatchFile)
		filesGroup.DELETE("/:id", h.Files.DeleteFile)
		filesGroup.POST("/search", h.Files.SearchFiles)
		filesGroup.GET("/history/:nodeId", h.Files.GetHistory)
		filesGroup.POST("/revert", h.Files.RevertFile)
		filesGroup.GET("/diff", h.Files.DiffVersions)

		// 文件上传下载
		filesGroup.POST("/upload", h.Files.UploadFile)
		filesGroup.POST("/upload/initiate", h.Files.InitiateUpload)
		filesGroup.POST("/upload/:uploadId/chunk", h.Files.UploadChunk)
		filesGroup.POST("/upload/:uploadId/complete", h.Files.CompleteUpload)
		filesGroup.GET("/download/:nodeId", h.Files.DownloadFile)
		filesGroup.GET("/preview/:nodeId", h.Files.GetPreview)
		filesGroup.GET("/uploads", h.Files.ListUploads)
		filesGroup.DELETE("/uploads/:uploadId", h.Files.DeleteUpload)
	}
}

func registerCommandRoutes(apiGroup *gin.RouterGroup, h *Handlers, commandAdminGuard gin.HandlerFunc) {
	commandsGroup := apiGroup.Group("/commands")
	{
		commandsGroup.POST("/execute", h.Commands.Execute)
		commandsGroup.GET("/:id", h.Commands.Get)
		commandsGroup.GET("", commandAdminGuard, h.Commands.List)
	}
}

func registerWorkspaceRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	workspaceGroup := apiGroup.Group("/workspace")
	{
		workspaceGroup.GET("/tree", h.Workspace.GetTree)
		workspaceGroup.POST("/folders", h.Workspace.CreateFolder)
		workspaceGroup.PATCH("/nodes/:id", h.Workspace.RenameNode)
		workspaceGroup.DELETE("/nodes/:id", h.Workspace.DeleteNode)
		workspaceGroup.GET("/files/:id", h.Workspace.GetFile)
		workspaceGroup.PUT("/files/:id", h.Workspace.UpdateFile)
		workspaceGroup.GET("/staging", h.Workspace.ListStaging)
		workspaceGroup.POST("/staging", h.Workspace.CreateStaging)
		workspaceGroup.POST("/staging/:id/review", h.Workspace.ReviewStaging)
		workspaceGroup.POST("/context-links", h.Workspace.AttachContext)

		// 内容管理增强 API
		workspaceGroup.GET("/outline/:workId", h.Workspace.GetOutlineView)
		workspaceGroup.POST("/batch/sort", h.Workspace.BatchUpdateSortOrder)
		workspaceGroup.POST("/batch/move", h.Workspace.BatchMoveNodes)
		workspaceGroup.POST("/batch/delete", h.Workspace.BatchDeleteNodes)
		workspaceGroup.POST("/batch/copy", h.Workspace.BatchCopyNodes)
		workspaceGroup.POST("/autosave", h.Workspace.AutoSaveContent)
		workspaceGroup.POST("/import/text", h.Workspace.ImportFromText)

		// 智能体产出物
		workspaceGroup.GET("/artifact-types", h.Artifact.GetArtifactTypes)
		workspaceGroup.POST("/artifacts", h.Artifact.CreateArtifact)

		// 智能体工作空间
		workspaceGroup.POST("/agents", h.Artifact.EnsureAgentWorkspace)
		workspaceGroup.GET("/agents/:agentId", h.Artifact.GetAgentWorkspace)
		workspaceGroup.GET("/agents/:agentId/artifacts", h.Artifact.ListAgentArtifacts)

		// 会话工作空间
		workspaceGroup.POST("/sessions", h.Artifact.EnsureSessionWorkspace)
		workspaceGroup.GET("/sessions/:sessionId/artifacts", h.Artifact.ListSessionArtifacts)

		// 工作空间导出
		workspaceGroup.POST("/export", h.Workspace.ExportWorkspace)
	}
}

func registerNotificationRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	notifGroup := apiGroup.Group("/notifications")
	{
		// 用户通知偏好管理
		notifGroup.GET("/preferences", h.NotificationConfig.GetUserPreferences)
		notifGroup.PUT("/preferences", h.NotificationConfig.UpdateUserPreferences)
		notifGroup.POST("/preferences/reset", h.NotificationConfig.ResetUserPreferences)

		// 通知渠道管理
		notifGroup.GET("/channels", h.NotificationConfig.GetEnabledChannels)
		notifGroup.POST("/channels/:channel/enable", h.NotificationConfig.EnableChannel)
		notifGroup.POST("/channels/:channel/disable", h.NotificationConfig.DisableChannel)

		// 通知分类订阅管理
		notifGroup.POST("/categories/:category/subscribe", h.NotificationConfig.SubscribeCategory)
		notifGroup.POST("/categories/:category/unsubscribe", h.NotificationConfig.UnsubscribeCategory)
	}

	// Webhook管理（需要Webhook handler）
	if h.Webhook != nil {
		webhookGroup := apiGroup.Group("/webhooks")
		{
			webhookGroup.GET("/event-types", h.Webhook.GetEventTypes)
			webhookGroup.GET("", h.Webhook.ListEndpoints)
			webhookGroup.POST("", h.Webhook.RegisterEndpoint)
			webhookGroup.PUT("/:id", h.Webhook.UpdateEndpoint)
			webhookGroup.DELETE("/:id", h.Webhook.DeleteEndpoint)
			webhookGroup.PUT("/:id/active", h.Webhook.SetActive)
			webhookGroup.POST("/:id/test", h.Webhook.TestWebhook)
		}
	}
}

// registerCacheRoutes 注册缓存统计路由
func registerCacheRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	if h.Cache == nil {
		return
	}
	
	cacheGroup := apiGroup.Group("/cache")
	{
		// 管理员接口（缓存统计、健康检查）
		cacheGroup.GET("/stats", adminGuard, h.Cache.GetStats)
		cacheGroup.GET("/health", adminGuard, h.Cache.GetHealth)
	}
}

// registerCodeSearchRoutes 注册代码搜索路由
func registerCodeSearchRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	if h.CodeSearch == nil {
		return
	}
	
	codeSearchGroup := apiGroup.Group("/codesearch")
	{
		// 用户接口（搜索功能）
		codeSearchGroup.POST("/symbols", h.CodeSearch.SearchSymbols)
		codeSearchGroup.POST("/definition", h.CodeSearch.FindDefinition)
		codeSearchGroup.POST("/references", h.CodeSearch.FindReferences)
		codeSearchGroup.POST("/text", h.CodeSearch.TextSearch)
		codeSearchGroup.GET("/outline", h.CodeSearch.GetFileOutline)
		codeSearchGroup.POST("/semantic", h.CodeSearch.SemanticSearch)
		codeSearchGroup.GET("/index/status", h.CodeSearch.GetIndexStatus)

		// 管理员接口（索引管理、配置）
		codeSearchGroup.POST("/index", adminGuard, h.CodeSearch.BuildIndex)
		codeSearchGroup.PUT("/config/base-path", adminGuard, h.CodeSearch.SetBasePath)
	}
}

// registerAnalyticsRoutes 注册写作分析与统计路由
func registerAnalyticsRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	if h.Analytics == nil {
		return
	}

	analyticsGroup := apiGroup.Group("/analytics")
	{
		// 作者仪表盘
		analyticsGroup.GET("/dashboard", h.Analytics.GetAuthorDashboard)

		// 月度报告
		analyticsGroup.GET("/monthly-report", h.Analytics.GetMonthlyReport)

		// Token 消耗趋势
		analyticsGroup.GET("/token-trend", h.Analytics.GetTokenTrend)

		// 功能使用分布
		analyticsGroup.GET("/feature-usage", h.Analytics.GetFeatureUsage)

		// 模型偏好分析
		analyticsGroup.GET("/model-preference", h.Analytics.GetModelPreference)

		// 写作效率分析
		analyticsGroup.GET("/writing-efficiency", h.Analytics.GetWritingEfficiency)

		// 写作习惯分析
		analyticsGroup.GET("/writing-habits", h.Analytics.GetWritingHabits)
		// 近期活动记录
		analyticsGroup.GET("/recent-activities", h.Analytics.GetRecentActivities)

		// 报告导出
		analyticsGroup.POST("/reports/export", h.Analytics.ExportReport)
	}
}


// registerBookParserRoutes 注册拆书系统路由
func registerBookParserRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	if h.BookParser == nil {
		return
	}

	bookParserGroup := apiGroup.Group("/bookparser")
	{
		// 获取支持的分析维度
		bookParserGroup.GET("/dimensions", h.BookParser.GetDimensions)

		// 任务管理
		bookParserGroup.POST("/tasks", h.BookParser.CreateTask)
		bookParserGroup.GET("/tasks", h.BookParser.ListTasks)
		bookParserGroup.GET("/tasks/:id", h.BookParser.GetTask)
		bookParserGroup.GET("/tasks/:id/progress", h.BookParser.GetTaskProgress)
		bookParserGroup.GET("/tasks/:id/results", h.BookParser.GetTaskResults)
		bookParserGroup.POST("/tasks/:id/cancel", h.BookParser.CancelTask)

		// 知识库搜索
		bookParserGroup.POST("/knowledge/search", h.BookParser.SearchKnowledge)
	}
}

// registerCreditsRoutes 注册积分系统路由
func registerCreditsRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	if h.Credits == nil {
		return
	}

	creditsGroup := apiGroup.Group("/credits")
	{
		// 用户接口
		creditsGroup.GET("/balance", h.Credits.GetBalance)
		creditsGroup.GET("/transactions", h.Credits.ListTransactions)
		creditsGroup.PUT("/warn-threshold", h.Credits.UpdateWarnThreshold)

		// 管理员接口
		creditsGroup.GET("/users/:userId/balance", adminGuard, h.Credits.GetUserBalance)
		creditsGroup.POST("/recharge", adminGuard, h.Credits.Recharge)
		creditsGroup.POST("/gift", adminGuard, h.Credits.Gift)
		creditsGroup.GET("/stats", adminGuard, h.Credits.GetStats)
		creditsGroup.GET("/users", adminGuard, h.Credits.ListUserSummaries)
		creditsGroup.GET("/export", adminGuard, h.Credits.ExportTransactions)
	}
}

// registerSubscriptionRoutes 注册订阅系统路由
func registerSubscriptionRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	if h.Subscription == nil {
		return
	}

	subGroup := apiGroup.Group("/subscription")
	{
		// 用户接口（查看套餐、订阅操作）
		subGroup.GET("/plans", h.Subscription.ListPlans)
		subGroup.GET("/plans/:id", h.Subscription.GetPlan)
		subGroup.POST("/subscribe", h.Subscription.Subscribe)
		subGroup.GET("/current", h.Subscription.GetCurrentSubscription)
		subGroup.GET("/history", h.Subscription.ListUserSubscriptions)
		subGroup.POST("/cancel", h.Subscription.CancelSubscription)
		subGroup.POST("/renew", h.Subscription.RenewSubscription)
		subGroup.POST("/change-plan", h.Subscription.ChangePlan)
		subGroup.POST("/trial/start", h.Subscription.StartTrial)
		subGroup.POST("/trial/convert", h.Subscription.ConvertTrial)

		// 管理员接口（套餐管理）
		subGroup.POST("/plans", adminGuard, h.Subscription.CreatePlan)
		subGroup.PUT("/plans/:id", adminGuard, h.Subscription.UpdatePlan)
		subGroup.DELETE("/plans/:id", adminGuard, h.Subscription.DeletePlan)

		// 管理员接口（查看其他用户订阅）
		subGroup.GET("/users/:userId", adminGuard, h.Subscription.GetUserSubscription)

		// 管理员接口（统计）
		subGroup.GET("/stats", adminGuard, h.Subscription.GetStats)
		subGroup.GET("/expiring", adminGuard, h.Subscription.CheckExpiring)
	}
}

// registerBillingRoutes 注册计费系统路由
func registerBillingRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	if h.Billing == nil {
		return
	}

	billingGroup := apiGroup.Group("/billing")
	{
		// 用户接口（查询定价、成本预估）
		billingGroup.GET("/pricings", h.Billing.ListPricings)
		billingGroup.GET("/pricings/query", h.Billing.GetPricing)
		billingGroup.POST("/estimate", h.Billing.EstimateCost)
		billingGroup.POST("/calculator", h.Billing.CalculateTokenCost)
		billingGroup.GET("/trend", h.Billing.GetCostTrend)

		// 管理员接口（定价管理）
		billingGroup.POST("/pricings", adminGuard, h.Billing.CreatePricing)
		billingGroup.PUT("/pricings/:id", adminGuard, h.Billing.UpdatePricing)
		billingGroup.DELETE("/pricings/:id", adminGuard, h.Billing.DeletePricing)

		// 管理员接口（成本报表）
		billingGroup.POST("/reports", adminGuard, h.Billing.GenerateCostReport)

		// 管理员接口（成本告警配置）
		billingGroup.GET("/alerts", adminGuard, h.Billing.ListAlerts)
		billingGroup.GET("/alerts/:id", adminGuard, h.Billing.GetAlert)
		billingGroup.POST("/alerts", adminGuard, h.Billing.CreateAlert)
		billingGroup.PUT("/alerts/:id", adminGuard, h.Billing.UpdateAlert)
		billingGroup.DELETE("/alerts/:id", adminGuard, h.Billing.DeleteAlert)
		billingGroup.POST("/alerts/check", adminGuard, h.Billing.CheckAlerts)

		// 管理员接口（计费审计）
		billingGroup.POST("/audit", adminGuard, h.Billing.QueryBillingAudit)

		// 管理员接口（定价策略）
		billingGroup.GET("/strategies", adminGuard, h.Billing.ListStrategies)
		billingGroup.POST("/strategies", adminGuard, h.Billing.CreateStrategy)
		billingGroup.PUT("/strategies/:id", adminGuard, h.Billing.UpdateStrategy)
		billingGroup.DELETE("/strategies/:id", adminGuard, h.Billing.DeleteStrategy)
	}
}

// registerContentRoutes 注册内容管理路由
func registerContentRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	if h.Content == nil {
		return
	}

	contentGroup := apiGroup.Group("/content")
	{
	// 用户接口（作品浏览、发布、举报）
		contentGroup.GET("/works/recommend", h.Content.GetRecommendWorks)
		contentGroup.GET("/works", h.Content.ListWorks)
		contentGroup.POST("/works/search", h.Content.SearchWorks)
		contentGroup.POST("/works", h.Content.PublishWork)
		contentGroup.GET("/works/:id", h.Content.GetWork)
		contentGroup.PUT("/works/:id", h.Content.UpdateWork)
		contentGroup.DELETE("/works/:id", h.Content.DeleteWork)
		contentGroup.GET("/categories", h.Content.ListCategories)
		contentGroup.GET("/tags", h.Content.ListTags)
		contentGroup.POST("/reports", h.Content.CreateReport)

		// 评论和点赞
		contentGroup.GET("/works/:id/comments", h.Content.ListComments)
		contentGroup.POST("/works/:id/comments", h.Content.CreateComment)
		contentGroup.POST("/works/:id/like", h.Content.ToggleLike)
		
		// 内容管理（管理员）
		contentGroup.POST("/categories", adminGuard, h.Content.CreateCategory)
		contentGroup.PUT("/categories/:id", adminGuard, h.Content.UpdateCategory)
		contentGroup.DELETE("/categories/:id", adminGuard, h.Content.DeleteCategory)

		// 管理员接口（标签管理）
		contentGroup.PUT("/tags/:id/hot", adminGuard, h.Content.SetHotTag)

		// 管理员接口（举报处理）
		contentGroup.GET("/reports", adminGuard, h.Content.ListReports)
		contentGroup.POST("/reports/:id/handle", adminGuard, h.Content.HandleReport)

		// 管理员接口（统计）
		contentGroup.GET("/stats", adminGuard, h.Content.GetContentStats)
	}
}

// registerModerationRoutes 注册内容审核路由
func registerModerationRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	if h.Moderation == nil {
		return
	}

	moderationGroup := apiGroup.Group("/moderation")
	{
		// 用户接口
		moderationGroup.POST("/submit", h.Moderation.SubmitContent)
		moderationGroup.POST("/filter", h.Moderation.FilterContent)

		// 审核员/管理员接口（任务处理）
		moderationGroup.GET("/tasks", adminGuard, h.Moderation.ListTasks)
		moderationGroup.GET("/tasks/:id", adminGuard, h.Moderation.GetTask)
		moderationGroup.GET("/tasks/:id/records", adminGuard, h.Moderation.GetTaskRecords)
		moderationGroup.POST("/tasks/:id/assign", adminGuard, h.Moderation.AssignTask)
		moderationGroup.GET("/queue", adminGuard, h.Moderation.GetPendingQueue)
		moderationGroup.POST("/review", adminGuard, h.Moderation.ReviewTask)

		// 管理员接口（敏感词管理）
		moderationGroup.GET("/words", adminGuard, h.Moderation.ListSensitiveWords)
		moderationGroup.POST("/words", adminGuard, h.Moderation.AddSensitiveWord)
		moderationGroup.POST("/words/batch", adminGuard, h.Moderation.BatchAddWords)
		moderationGroup.DELETE("/words/:id", adminGuard, h.Moderation.DeleteSensitiveWord)

		// 管理员接口（审核规则管理）
		moderationGroup.GET("/rules", adminGuard, h.Moderation.ListRules)
		moderationGroup.POST("/rules", adminGuard, h.Moderation.CreateRule)
		moderationGroup.PUT("/rules/:id", adminGuard, h.Moderation.UpdateRule)
		moderationGroup.DELETE("/rules/:id", adminGuard, h.Moderation.DeleteRule)

		// 管理员接口（统计）
		moderationGroup.GET("/stats", adminGuard, h.Moderation.GetStats)
	}
}

func registerComplianceRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	complianceGroup := apiGroup.Group("/compliance")
	{
		// 用户接口（实名认证提交、查询自己的认证状态）
		complianceGroup.POST("/verifications", h.Compliance.SubmitVerification)
		complianceGroup.GET("/verifications/me", h.Compliance.GetMyVerification)
		complianceGroup.GET("/ratings/:contentId", h.Compliance.GetContentRating)
		complianceGroup.POST("/copyrights", h.Compliance.RegisterCopyright)
		complianceGroup.GET("/copyrights/:contentId", h.Compliance.GetCopyrightRecord)

		// 管理员接口（认证审核）
		complianceGroup.GET("/verifications", adminGuard, h.Compliance.ListVerifications)
		complianceGroup.POST("/verifications/:id/review", adminGuard, h.Compliance.ReviewVerification)

		// 管理员接口（内容分级）
		complianceGroup.POST("/ratings", adminGuard, h.Compliance.SetContentRating)

		// 管理员接口（合规检查）
		complianceGroup.POST("/checks", adminGuard, h.Compliance.RunComplianceCheck)
		complianceGroup.GET("/checks", adminGuard, h.Compliance.ListComplianceChecks)

		// 管理员接口（风险提示）
		complianceGroup.GET("/alerts", adminGuard, h.Compliance.ListRiskAlerts)
		complianceGroup.POST("/alerts/:id/resolve", adminGuard, h.Compliance.ResolveRiskAlert)

		// 管理员接口（合规报告）
		complianceGroup.POST("/reports", adminGuard, h.Compliance.GenerateComplianceReport)
		complianceGroup.GET("/reports", adminGuard, h.Compliance.ListComplianceReports)
		complianceGroup.GET("/reports/:id", adminGuard, h.Compliance.GetComplianceReport)

		// 管理员接口（统计）
		complianceGroup.GET("/stats", adminGuard, h.Compliance.GetComplianceStats)
	}
}

// registerWorldBuilderRoutes 注册世界观构建路由
func registerWorldBuilderRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	if h.WorldBuilder == nil {
		return
	}

	wbGroup := apiGroup.Group("/worldbuilder")
	{
		// 世界观设定 CRUD
		wbGroup.POST("/settings", h.WorldBuilder.CreateSetting)
		wbGroup.GET("/settings", h.WorldBuilder.ListSettings)
		wbGroup.GET("/settings/:id", h.WorldBuilder.GetSetting)
		wbGroup.PUT("/settings/:id", h.WorldBuilder.UpdateSetting)
		wbGroup.DELETE("/settings/:id", h.WorldBuilder.DeleteSetting)

		// AI 生成
		wbGroup.POST("/generate", h.WorldBuilder.GenerateSetting)
		wbGroup.POST("/modify", h.WorldBuilder.ModifySetting)
		wbGroup.POST("/generate/character", h.WorldBuilder.GenerateCharacter)
		wbGroup.POST("/generate/relations", h.WorldBuilder.GenerateRelations)

		// 实体管理
		wbGroup.POST("/entities", h.WorldBuilder.CreateEntity)
		wbGroup.GET("/entities", h.WorldBuilder.ListEntities)
		wbGroup.GET("/entities/:id", h.WorldBuilder.GetEntity)
		wbGroup.PUT("/entities/:id", h.WorldBuilder.UpdateEntity)
		wbGroup.DELETE("/entities/:id", h.WorldBuilder.DeleteEntity)

		// 关系管理
		wbGroup.POST("/relations", h.WorldBuilder.CreateRelation)
		wbGroup.GET("/relations/graph", h.WorldBuilder.GetRelationGraph)
		wbGroup.DELETE("/relations/:id", h.WorldBuilder.DeleteRelation)

		// 版本管理
		wbGroup.GET("/versions", h.WorldBuilder.GetVersionHistory)
		wbGroup.GET("/versions/:id", h.WorldBuilder.GetVersion)
		wbGroup.POST("/versions/revert", h.WorldBuilder.RevertToVersion)
		wbGroup.GET("/versions/diff", h.WorldBuilder.DiffVersions)

		// 模板管理
		wbGroup.GET("/templates", h.WorldBuilder.ListTemplates)
		wbGroup.POST("/templates", h.WorldBuilder.CreateTemplate)

		// 统计
		wbGroup.GET("/stats", h.WorldBuilder.GetStats)
	}
}

// registerFragmentRoutes 注册片段管理路由
func registerFragmentRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	fragments := apiGroup.Group("/fragments")
	{
		// 基础 CRUD
		fragments.POST("", h.Fragment.CreateFragment)
		fragments.GET("", h.Fragment.ListFragments)
		fragments.GET("/:id", h.Fragment.GetFragment)
		fragments.PUT("/:id", h.Fragment.UpdateFragment)
		fragments.DELETE("/:id", h.Fragment.DeleteFragment)

		// 批量操作
		fragments.POST("/batch", h.Fragment.BatchOperation)
		
		// 完成片段
		fragments.POST("/:id/complete", h.Fragment.CompleteFragment)

		// 统计
		fragments.GET("/stats", h.Fragment.GetStats)
	}
}

// registerMultiModelRoutes 注册多模型抽卡路由
func registerMultiModelRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	multimodel := apiGroup.Group("/multimodel")
	{
		// 多模型抽卡
		multimodel.POST("/draw", h.MultiModel.Draw)
		
		// 结果管理
		multimodel.GET("/draws", h.MultiModel.ListDrawHistory)
		multimodel.GET("/draws/:id", h.MultiModel.GetDrawHistory)
		multimodel.DELETE("/draws/:id", h.MultiModel.DeleteDrawHistory)
		
		// 重新生成
		multimodel.POST("/regenerate", h.MultiModel.Regenerate)
		
		// 统计
		multimodel.GET("/stats", h.MultiModel.GetStats)
	}
}

// registerPlotRoutes 注册剧情推演路由
func registerPlotRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	plot := apiGroup.Group("/plot")
	{
		// 剧情推演
		plot.POST("/recommendations", h.Plot.CreatePlotRecommendation)
		
		// 一键应用
		plot.POST("/apply", h.Plot.ApplyPlotToChapter)
		
		// 历史管理
		plot.GET("/recommendations", h.Plot.ListPlotRecommendations)
		plot.GET("/recommendations/:id", h.Plot.GetPlotRecommendation)
		plot.PUT("/recommendations/:id", h.Plot.UpdatePlotRecommendation)
		plot.DELETE("/recommendations/:id", h.Plot.DeletePlotRecommendation)
		
		// 统计
		plot.GET("/stats", h.Plot.GetStats)
	}
}

// registerMessageRoutes 注册消息管理路由
func registerMessageRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	if h.Message == nil {
		return
	}

	messages := apiGroup.Group("/messages")
	{
		// 消息列表和详情
		messages.GET("", h.Message.ListMessages)
		messages.GET("/:id", h.Message.GetMessage)
		
		// 已读状态管理
		messages.POST("/:id/read", h.Message.MarkAsRead)
		messages.POST("/:id/unread", h.Message.MarkAsUnread)
		messages.POST("/read-all", h.Message.MarkAllAsRead)
		messages.POST("/batch-read", h.Message.BatchMarkAsRead)
		
		// 未读数量
		messages.GET("/unread-count", h.Message.GetUnreadCount)
		messages.GET("/unread-count/by-category", h.Message.GetUnreadCountByCategory)
		
		// 删除
		messages.DELETE("/:id", h.Message.DeleteMessage)
		messages.POST("/batch-delete", h.Message.BatchDeleteMessages)
		
		// 统计
		messages.GET("/stats", h.Message.GetStats)
	}
}

// registerWorkspaceTemplateRoutes 注册工作空间模板路由
func registerWorkspaceTemplateRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	if h.WorkspaceTemplate == nil {
		return
	}

	templates := apiGroup.Group("/workspace/templates")
	{
		// 模板 CRUD
		templates.GET("", h.WorkspaceTemplate.ListTemplates)
		templates.GET("/:id", h.WorkspaceTemplate.GetTemplate)
		templates.POST("", h.WorkspaceTemplate.CreateTemplate)
		templates.PUT("/:id", h.WorkspaceTemplate.UpdateTemplate)
		templates.DELETE("/:id", h.WorkspaceTemplate.DeleteTemplate)
		
		// 应用模板
		templates.POST("/apply", h.WorkspaceTemplate.ApplyTemplate)
	}
}

// registerMarketplaceRoutes 注册工具市场路由
func registerMarketplaceRoutes(apiGroup *gin.RouterGroup, h *Handlers, adminGuard gin.HandlerFunc) {
	if h.Marketplace == nil {
		return
	}

	marketplace := apiGroup.Group("/marketplace")
	{
		// 公开接口（无需认证）
		marketplace.GET("/packages", h.Marketplace.ListPackages)
		marketplace.GET("/packages/:id", h.Marketplace.GetPackage)
		marketplace.POST("/search", h.Marketplace.Search)
		marketplace.GET("/stats", h.Marketplace.GetStats)
		marketplace.GET("/packages/:id/versions", h.Marketplace.ListVersions)
		marketplace.GET("/packages/:id/versions/:version", h.Marketplace.GetVersion)
		marketplace.GET("/packages/:id/ratings", h.Marketplace.ListRatings)

		// 用户接口（需要认证）
		marketplace.POST("/publish", h.Marketplace.Publish)
		marketplace.PUT("/packages/:id", h.Marketplace.UpdatePackage)
		marketplace.DELETE("/packages/:id", h.Marketplace.DeletePackage)
		marketplace.POST("/packages/:id/versions", h.Marketplace.PublishVersion)
		marketplace.POST("/packages/:id/rate", h.Marketplace.Rate)
		marketplace.POST("/packages/:id/install", h.Marketplace.Install)
		marketplace.POST("/packages/:id/uninstall", h.Marketplace.Uninstall)
		marketplace.GET("/installed", h.Marketplace.ListInstalled)

		// 管理员接口
		admin := marketplace.Group("/admin")
		admin.Use(adminGuard)
		{
			admin.GET("/pending", h.Marketplace.ListPendingPackages)
			admin.POST("/packages/:id/approve", h.Marketplace.ApprovePackage)
			admin.POST("/packages/:id/reject", h.Marketplace.RejectPackage)
			admin.POST("/packages/:id/deprecate", h.Marketplace.DeprecatePackage)
		}
	}
}
