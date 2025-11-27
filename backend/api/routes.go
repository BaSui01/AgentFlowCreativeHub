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
		authGroup.POST("/refresh", h.Auth.Refresh)
		authGroup.POST("/logout", h.Auth.Logout)
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

	// 租户管理
	registerTenantRoutes(apiGroup, h, manageRolesGuard)

	// 模型管理
	registerModelRoutes(apiGroup, h)

	// 模板管理
	registerTemplateRoutes(apiGroup, h)

	// Agent 管理
	registerAgentRoutes(apiGroup, h)

	// 工作流管理
	registerWorkflowRoutes(apiGroup, h)

	// 审计日志
	registerAuditRoutes(apiGroup, h)

	// 知识库管理
	registerKnowledgeRoutes(apiGroup, h)

	// 工具管理
	registerToolRoutes(apiGroup, h)

	// 文件管理
	registerFileRoutes(apiGroup, h)

	// 命令管理
	registerCommandRoutes(apiGroup, h, commandAdminGuard)

	// 工作空间
	registerWorkspaceRoutes(apiGroup, h)

	// 通知配置
	registerNotificationRoutes(apiGroup, h)
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

func registerModelRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	modelsGroup := apiGroup.Group("/models")
	{
		modelsGroup.GET("", h.Model.ListModels)
		modelsGroup.GET("/:id", h.Model.GetModel)
		modelsGroup.POST("", h.Model.CreateModel)
		modelsGroup.PUT("/:id", h.Model.UpdateModel)
		modelsGroup.DELETE("/:id", h.Model.DeleteModel)
		modelsGroup.GET("/:id/stats", h.Model.GetModelStats)

		credentials := modelsGroup.Group("/:id/credentials")
		{
			credentials.GET("", h.Model.ListModelCredentials)
			credentials.POST("", h.Model.CreateModelCredential)
			credentials.DELETE("/:credentialId", h.Model.DeleteModelCredential)
		}

		modelsGroup.POST("/seed", h.Model.SeedDefaultModels)
		modelsGroup.POST("/discover/:provider", h.Model.DiscoverModels)
		modelsGroup.POST("/discover-all", h.Model.DiscoverAllModels)
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

func registerAgentRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	agentsGroup := apiGroup.Group("/agents")
	{
		agentsGroup.GET("", h.Agent.ListAgentConfigs)
		agentsGroup.GET("/:id", h.Agent.GetAgentConfig)
		agentsGroup.POST("", h.Agent.CreateAgentConfig)
		agentsGroup.PUT("/:id", h.Agent.UpdateAgentConfig)
		agentsGroup.DELETE("/:id", h.Agent.DeleteAgentConfig)
		agentsGroup.GET("/types/:type", h.Agent.GetAgentByType)
		agentsGroup.POST("/seed", h.Agent.SeedDefaultAgents)

		// Agent 能力目录
		agentsGroup.GET("/capabilities", h.WfTemplate.GetAgentCapabilities)
		agentsGroup.GET("/capabilities/:agent_type/:role", h.WfTemplate.GetRoleCapability)

		// Agent 执行
		agentsGroup.POST("/:id/execute", h.AgentExecute.Execute)
		agentsGroup.POST("/:id/execute-stream", h.AgentExecute.ExecuteStream)
		agentsGroup.POST("/:id/run", h.AgentExecute.ExecuteAsync)
		agentsGroup.POST("/types/:type/execute", h.AgentExecute.ExecuteByType)
		agentsGroup.POST("/types/:type/execute-stream", h.AgentExecute.ExecuteByTypeStream)
	}
}

func registerWorkflowRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	workflowsGroup := apiGroup.Group("/workflows")
	{
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

			approvals := workflowsGroup.Group("/approvals")
			{
				approvals.GET("/pending", h.Automation.GetApprovalRequests)
				approvals.POST("/:id/approve", h.Automation.ApproveRequest)
				approvals.POST("/:id/reject", h.Automation.RejectRequest)
				approvals.POST("/:id/resend", h.Automation.ResendNotification)
			}

			workflowsGroup.GET("/executions/:id/state", h.Automation.GetExecutionState)
			workflowsGroup.GET("/executions/:id/automation-logs", h.Automation.GetAutomationLogs)
		}
	}

	// 执行记录
	executions := apiGroup.Group("/executions")
	{
		executions.GET("/:id", h.WfExecute.GetExecution)
	}
}

func registerAuditRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	auditGroup := apiGroup.Group("/audit")
	{
		auditGroup.POST("/logs/query", h.Audit.QueryLogs)
		auditGroup.GET("/logs/:id", h.Audit.GetLog)
		auditGroup.GET("/users/:userID/activity", h.Audit.GetUserActivity)
		auditGroup.GET("/my-activity", h.Audit.GetMyActivity)
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
		docsGroup.DELETE("/:id", h.Document.DeleteDocument)
		docsGroup.GET("/:id/chunks", h.Document.ListChunks)
	}
}

func registerToolRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	toolsGroup := apiGroup.Group("/tools")
	{
		toolsGroup.GET("", h.Tool.ListTools)
		toolsGroup.POST("/register", h.Tool.RegisterTool)
		toolsGroup.GET("/categories/:category", h.Tool.ListToolsByCategory)
		toolsGroup.GET("/:name", h.Tool.GetTool)
		toolsGroup.DELETE("/:name", h.Tool.UnregisterTool)
		toolsGroup.POST("/:name/execute", h.Tool.ExecuteTool)
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

		// 静音时段
		notifGroup.POST("/quiet-hours", h.NotificationConfig.SetQuietHours)

		// 通知历史与统计
		notifGroup.GET("/history", h.NotificationConfig.GetNotificationHistory)
		notifGroup.GET("/stats", h.NotificationConfig.GetNotificationStats)

		// 租户配置管理（需要管理员权限）
		notifGroup.GET("/config/tenant", h.NotificationConfig.GetTenantConfig)
		notifGroup.PUT("/config/tenant", h.NotificationConfig.UpdateTenantConfig)
	}
}
