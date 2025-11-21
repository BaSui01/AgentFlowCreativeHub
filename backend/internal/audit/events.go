package audit

// EventType 审计事件类型
type EventType string

// 认证相关事件
const (
	EventUserLogin         EventType = "user.login"           // 用户登录
	EventUserLoginFailed   EventType = "user.login.failed"    // 用户登录失败
	EventUserLogout        EventType = "user.logout"          // 用户登出
	EventUserRegister      EventType = "user.register"        // 用户注册
	EventTokenRefresh      EventType = "user.token.refresh"   // 令牌刷新
	EventSessionRevoke     EventType = "user.session.revoke"  // 会话撤销
	EventPasswordChange    EventType = "user.password.change" // 修改密码
	EventPasswordReset     EventType = "user.password.reset"  // 重置密码
)

// 资源管理事件
const (
	EventModelCreate EventType = "model.create" // 创建模型
	EventModelUpdate EventType = "model.update" // 更新模型
	EventModelDelete EventType = "model.delete" // 删除模型
	EventModelView   EventType = "model.view"   // 查看模型

	EventTemplateCreate EventType = "template.create" // 创建模板
	EventTemplateUpdate EventType = "template.update" // 更新模板
	EventTemplateDelete EventType = "template.delete" // 删除模板
	EventTemplateRender EventType = "template.render" // 渲染模板

	EventAgentCreate  EventType = "agent.create"  // 创建 Agent
	EventAgentUpdate  EventType = "agent.update"  // 更新 Agent
	EventAgentDelete  EventType = "agent.delete"  // 删除 Agent
	EventAgentExecute EventType = "agent.execute" // 执行 Agent

	EventWorkflowCreate  EventType = "workflow.create"  // 创建工作流
	EventWorkflowUpdate  EventType = "workflow.update"  // 更新工作流
	EventWorkflowDelete  EventType = "workflow.delete"  // 删除工作流
	EventWorkflowExecute EventType = "workflow.execute" // 执行工作流
)

// 系统管理事件
const (
	EventTenantCreate      EventType = "tenant.create"        // 创建租户
	EventTenantUpdate      EventType = "tenant.update"        // 更新租户
	EventTenantDelete      EventType = "tenant.delete"        // 删除租户
	EventUserCreate        EventType = "tenant.user.create"   // 创建用户
	EventUserUpdate        EventType = "tenant.user.update"   // 更新用户
	EventUserDelete        EventType = "tenant.user.delete"   // 删除用户
	EventPermissionGrant   EventType = "permission.grant"     // 授予权限
	EventPermissionRevoke  EventType = "permission.revoke"    // 撤销权限
	EventSettingUpdate     EventType = "setting.update"       // 更新设置
	EventAPIKeyCreate      EventType = "apikey.create"        // 创建 API Key
	EventAPIKeyRevoke      EventType = "apikey.revoke"        // 撤销 API Key
)

// 数据访问事件
const (
	EventDataExport EventType = "data.export" // 数据导出
	EventDataImport EventType = "data.import" // 数据导入
	EventDataQuery  EventType = "data.query"  // 数据查询
	EventDataDelete EventType = "data.delete" // 数据删除
)

// 安全事件（虽然安全优先级低，但审计日志仍需记录）
const (
	EventSecurityViolation    EventType = "security.violation"     // 安全违规
	EventUnauthorizedAccess   EventType = "security.unauthorized"  // 未授权访问
	EventSuspiciousActivity   EventType = "security.suspicious"    // 可疑活动
	EventRateLimitExceeded    EventType = "security.ratelimit"     // 超过速率限制
)

// EventCategory 事件分类
type EventCategory string

const (
	CategoryAuth     EventCategory = "auth"     // 认证授权
	CategoryResource EventCategory = "resource" // 资源管理
	CategorySystem   EventCategory = "system"   // 系统管理
	CategoryData     EventCategory = "data"     // 数据访问
	CategorySecurity EventCategory = "security" // 安全相关
)

// GetEventCategory 获取事件分类
func GetEventCategory(eventType EventType) EventCategory {
	switch eventType {
	case EventUserLogin, EventUserLoginFailed, EventUserLogout, EventUserRegister,
		EventTokenRefresh, EventSessionRevoke, EventPasswordChange, EventPasswordReset:
		return CategoryAuth

	case EventModelCreate, EventModelUpdate, EventModelDelete, EventModelView,
		EventTemplateCreate, EventTemplateUpdate, EventTemplateDelete, EventTemplateRender,
		EventAgentCreate, EventAgentUpdate, EventAgentDelete, EventAgentExecute,
		EventWorkflowCreate, EventWorkflowUpdate, EventWorkflowDelete, EventWorkflowExecute:
		return CategoryResource

	case EventTenantCreate, EventTenantUpdate, EventTenantDelete,
		EventUserCreate, EventUserUpdate, EventUserDelete,
		EventPermissionGrant, EventPermissionRevoke,
		EventSettingUpdate, EventAPIKeyCreate, EventAPIKeyRevoke:
		return CategorySystem

	case EventDataExport, EventDataImport, EventDataQuery, EventDataDelete:
		return CategoryData

	case EventSecurityViolation, EventUnauthorizedAccess, EventSuspiciousActivity, EventRateLimitExceeded:
		return CategorySecurity

	default:
		return CategorySystem
	}
}

// EventLevel 事件级别
type EventLevel string

const (
	LevelInfo    EventLevel = "info"    // 信息
	LevelWarning EventLevel = "warning" // 警告
	LevelError   EventLevel = "error"   // 错误
	LevelCritical EventLevel = "critical" // 严重
)

// GetEventLevel 获取事件级别（基于事件类型推断）
func GetEventLevel(eventType EventType) EventLevel {
	switch eventType {
	case EventUserLoginFailed, EventUnauthorizedAccess, EventSuspiciousActivity,
		EventRateLimitExceeded:
		return LevelWarning

	case EventSecurityViolation, EventDataDelete, EventTenantDelete,
		EventPermissionRevoke, EventSessionRevoke:
		return LevelError

	default:
		return LevelInfo
	}
}

// AuditMetadata 审计元数据
type AuditMetadata struct {
	ResourceType string                 `json:"resource_type,omitempty"` // 资源类型
	ResourceID   string                 `json:"resource_id,omitempty"`   // 资源 ID
	Action       string                 `json:"action,omitempty"`        // 操作名称
	Changes      map[string]interface{} `json:"changes,omitempty"`       // 变更内容
	OldValue     interface{}            `json:"old_value,omitempty"`     // 旧值
	NewValue     interface{}            `json:"new_value,omitempty"`     // 新值
	Reason       string                 `json:"reason,omitempty"`        // 操作原因
	Duration     int64                  `json:"duration,omitempty"`      // 执行时长（毫秒）
	ErrorMessage string                 `json:"error_message,omitempty"` // 错误消息
}

// EventDescription 获取事件描述
func GetEventDescription(eventType EventType) string {
	descriptions := map[EventType]string{
		EventUserLogin:       "用户登录",
		EventUserLoginFailed: "用户登录失败",
		EventUserLogout:      "用户登出",
		EventUserRegister:    "用户注册",
		EventTokenRefresh:    "刷新访问令牌",
		EventSessionRevoke:   "撤销用户会话",
		EventPasswordChange:  "修改密码",
		EventPasswordReset:   "重置密码",

		EventModelCreate: "创建模型",
		EventModelUpdate: "更新模型",
		EventModelDelete: "删除模型",
		EventModelView:   "查看模型",

		EventTemplateCreate: "创建模板",
		EventTemplateUpdate: "更新模板",
		EventTemplateDelete: "删除模板",
		EventTemplateRender: "渲染模板",

		EventAgentCreate:  "创建 Agent",
		EventAgentUpdate:  "更新 Agent",
		EventAgentDelete:  "删除 Agent",
		EventAgentExecute: "执行 Agent",

		EventWorkflowCreate:  "创建工作流",
		EventWorkflowUpdate:  "更新工作流",
		EventWorkflowDelete:  "删除工作流",
		EventWorkflowExecute: "执行工作流",

		EventTenantCreate:    "创建租户",
		EventTenantUpdate:    "更新租户",
		EventTenantDelete:    "删除租户",
		EventUserCreate:      "创建用户",
		EventUserUpdate:      "更新用户",
		EventUserDelete:      "删除用户",
		EventPermissionGrant: "授予权限",
		EventPermissionRevoke: "撤销权限",
		EventSettingUpdate:   "更新设置",
		EventAPIKeyCreate:    "创建 API Key",
		EventAPIKeyRevoke:    "撤销 API Key",

		EventDataExport: "导出数据",
		EventDataImport: "导入数据",
		EventDataQuery:  "查询数据",
		EventDataDelete: "删除数据",

		EventSecurityViolation:  "安全违规",
		EventUnauthorizedAccess: "未授权访问",
		EventSuspiciousActivity: "可疑活动",
		EventRateLimitExceeded:  "超过速率限制",
	}

	if desc, exists := descriptions[eventType]; exists {
		return desc
	}
	return string(eventType)
}
