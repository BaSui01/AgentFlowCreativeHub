package tenant

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PermissionDefinition 描述系统预置权限
type PermissionDefinition struct {
	Code        string
	Name        string
	Category    string
	Resource    string
	Action      string
	Description string
	Locales     map[string]PermissionLocale
}

// PermissionLocale 记录多语言名称/描述
type PermissionLocale struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PermissionCatalogItem 用于对外展示的权限条目
type PermissionCatalogItem struct {
	Code        string                      `json:"code"`
	Category    string                      `json:"category"`
	Resource    string                      `json:"resource"`
	Action      string                      `json:"action"`
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Locales     map[string]PermissionLocale `json:"locales"`
}

// PermissionCatalogData 描述权限字典响应
type PermissionCatalogData struct {
	Version        string                       `json:"version"`
	Items          []PermissionCatalogItem      `json:"items"`
	CategoryLabels map[string]map[string]string `json:"categoryLabels"`
	CategoryOrder  []string                     `json:"categoryOrder"`
}

const permissionCatalogVersion = "2025-11-22"

var permissionCategoryLabels = map[string]map[string]string{
	"tenant": {
		"zh-CN": "租户与账号",
		"en-US": "Tenant & Accounts",
	},
	"workspace": {
		"zh-CN": "工作区",
		"en-US": "Workspace",
	},
	"workflow": {
		"zh-CN": "工作流",
		"en-US": "Workflow",
	},
	"knowledge": {
		"zh-CN": "知识库",
		"en-US": "Knowledge",
	},
	"command": {
		"zh-CN": "命令控制",
		"en-US": "Command Center",
	},
}

var permissionCategoryOrder = []string{"tenant", "workspace", "workflow", "knowledge", "command"}

var systemPermissionCatalog = []PermissionDefinition{
	{
		Code:        "tenant:manage_roles",
		Name:        "管理角色与权限",
		Category:    "tenant",
		Resource:    "tenant",
		Action:      "manage_roles",
		Description: "创建、更新、删除角色并配置权限",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "管理角色与权限", Description: "创建、更新、删除角色并配置权限"},
			"en-US": {Name: "Manage roles and permissions", Description: "Create, update or delete roles and assign permissions."},
		},
	},
	{
		Code:        "tenant:manage_users",
		Name:        "管理租户用户",
		Category:    "tenant",
		Resource:    "tenant",
		Action:      "manage_users",
		Description: "邀请或移除租户成员",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "管理租户用户", Description: "邀请或移除租户成员"},
			"en-US": {Name: "Manage tenant users", Description: "Invite or remove tenant members."},
		},
	},
	{
		Code:        "workspace:read",
		Name:        "查看工作区文件",
		Category:    "workspace",
		Resource:    "workspace",
		Action:      "read",
		Description: "浏览文件树与历史",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "查看工作区文件", Description: "浏览文件树与历史"},
			"en-US": {Name: "View workspace files", Description: "Browse workspace files and revisions."},
		},
	},
	{
		Code:        "workspace:write",
		Name:        "编辑工作区文件",
		Category:    "workspace",
		Resource:    "workspace",
		Action:      "write",
		Description: "创建、修改、删除文件",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "编辑工作区文件", Description: "创建、修改、删除文件"},
			"en-US": {Name: "Edit workspace files", Description: "Create, update or delete files."},
		},
	},
	{
		Code:        "workspace:review",
		Name:        "审核暂存区",
		Category:    "workspace",
		Resource:    "workspace",
		Action:      "review",
		Description: "批准或驳回暂存内容",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "审核暂存区", Description: "批准或驳回暂存内容"},
			"en-US": {Name: "Review staging", Description: "Approve or reject staged changes."},
		},
	},
	{
		Code:        "workflow:create",
		Name:        "创建工作流",
		Category:    "workflow",
		Resource:    "workflow",
		Action:      "create",
		Description: "新建与发布自动化工作流",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "创建工作流", Description: "新建与发布自动化工作流"},
			"en-US": {Name: "Create workflows", Description: "Create and publish workflow automations."},
		},
	},
	{
		Code:        "workflow:approve",
		Name:        "审批工作流执行",
		Category:    "workflow",
		Resource:    "workflow",
		Action:      "approve",
		Description: "审批需要人工确认的工作流步骤",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "审批工作流执行", Description: "审批需要人工确认的工作流步骤"},
			"en-US": {Name: "Approve workflow", Description: "Approve workflow steps requiring review."},
		},
	},
	{
		Code:        "workflow:execute",
		Name:        "触发工作流运行",
		Category:    "workflow",
		Resource:    "workflow",
		Action:      "execute",
		Description: "手动触发或重试工作流",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "触发工作流运行", Description: "手动触发或重试工作流"},
			"en-US": {Name: "Execute workflow", Description: "Trigger or retry workflows manually."},
		},
	},
	{
		Code:        "kb:manage",
		Name:        "管理知识库",
		Category:    "knowledge",
		Resource:    "knowledge",
		Action:      "manage",
		Description: "导入、删除知识库文档",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "管理知识库", Description: "导入、删除知识库文档"},
			"en-US": {Name: "Manage knowledge base", Description: "Import or delete knowledge documents."},
		},
	},
	{
		Code:        "commands:execute",
		Name:        "执行智能体命令",
		Category:    "command",
		Resource:    "command",
		Action:      "execute",
		Description: "在命令面板提交执行任务",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "执行智能体命令", Description: "在命令面板提交执行任务"},
			"en-US": {Name: "Execute agent commands", Description: "Submit commands from the console."},
		},
	},
	{
		Code:        "commands:admin",
		Name:        "查看命令队列",
		Category:    "command",
		Resource:    "command",
		Action:      "admin",
		Description: "查看或终止命令执行队列中的任务",
		Locales: map[string]PermissionLocale{
			"zh-CN": {Name: "查看命令队列", Description: "查看或终止命令执行队列中的任务"},
			"en-US": {Name: "Administer commands", Description: "Inspect or terminate queued commands."},
		},
	},
}

// EnsureSystemPermissions 将预置权限写入数据库（幂等）
func EnsureSystemPermissions(db *gorm.DB) error {
	for _, def := range systemPermissionCatalog {
		if err := upsertPermission(db, def); err != nil {
			return err
		}
	}
	return nil
}

func upsertPermission(db *gorm.DB, def PermissionDefinition) error {
	const q = `
	INSERT INTO permissions (id, tenant_id, code, name, category, resource, action, description, created_at, updated_at)
	VALUES (?, NULL, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	ON CONFLICT (code) DO UPDATE
	SET name = EXCLUDED.name,
	    category = EXCLUDED.category,
	    resource = EXCLUDED.resource,
	    action = EXCLUDED.action,
	    description = EXCLUDED.description,
	    updated_at = NOW()
	`
	return db.Exec(q, uuid.NewString(), def.Code, def.Name, def.Category, def.Resource, def.Action, def.Description).Error
}

// GetPermissionCatalog 返回结构化权限字典
func GetPermissionCatalog() PermissionCatalogData {
	items := make([]PermissionCatalogItem, 0, len(systemPermissionCatalog))
	for _, def := range systemPermissionCatalog {
		items = append(items, PermissionCatalogItem{
			Code:        def.Code,
			Category:    def.Category,
			Resource:    def.Resource,
			Action:      def.Action,
			Name:        def.Name,
			Description: def.Description,
			Locales:     def.Locales,
		})
	}
	return PermissionCatalogData{
		Version:        permissionCatalogVersion,
		Items:          items,
		CategoryLabels: permissionCategoryLabels,
		CategoryOrder:  permissionCategoryOrder,
	}
}

// EnsureTenantDefaultRoles 后续可扩展：根据权限为新租户创建默认角色
func EnsureTenantDefaultRoles(db *gorm.DB, tenantID, adminUserID string) error {
	// 暂以占位，后续 phases 可实现更复杂逻辑
	_ = db
	_ = tenantID
	_ = adminUserID
	return nil
}
