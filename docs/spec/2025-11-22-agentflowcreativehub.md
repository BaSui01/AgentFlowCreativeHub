我已读取当前仓库与全局开发规范（CLAUDE.md / AGENTS.md），并确认：
- 需要保持简体中文交流
- 当前处于 Spec Mode，只能给出实现方案，不能直接改动文件或运行具有副作用的命令
- 需在实现前给出清晰规划，并在获得你确认后再进入真正实现阶段

结合你之前已进行的改动痕迹（多租户、工作区、RBAC 权限、前后端已经有较多变更），后续完整实现大致会分为几块：

1. **后端整体思路（Go）**
   - 继续沿用已存在的多租户与工作区模型，复用 `backend/internal/tenant`、`backend/internal/workspace`、`backend/internal/models` 中的抽象。
   - 引入或补全以下能力（根据你之后给的具体需求裁剪）：
     - 命令执行与审计：在 `backend/internal/command` 目录下实现命令模型、执行服务与审计记录持久化；在 `backend/api/handlers/commands` 下提供 REST API（例如：列出可用命令、在指定 workspace 中执行命令、查询命令执行历史）。
     - 文件浏览与预览：在 `backend/api/handlers/files` 下暴露按 workspace 的文件树浏览接口、文件内容读取接口、暂存区（staging）相关接口；领域逻辑放在 `backend/internal/workspace` 或单独的 `internal/files` 模块中，根据现有模式选型。
     - RBAC 权限：基于你已经新增的 `permissions_seed.go`、`permissions_seed_test.go` 和 `db/migrations/0011_rbac_permissions.sql`，实现角色-权限-资源的检查器；在 `backend/internal/auth/checker.go` 或新增 service 中落地 `PermissionGuard` 所需的后端权限标识（如 `workspace:view`, `workspace:execute_command`, `admin:tenant_manage` 等）。
   - 为上述模块在 `backend/internal/..._test.go` 中补充对应单元测试，延续项目现有测试风格（table-driven tests、使用 testify 或标准库 `testing`）。
   - 在 `backend/api/setup.go` 内挂载新路由，保持与现有路由分组（/api/tenants、/api/workspaces 等）一致的风格。

2. **前端整体思路（React + TS）**
   - 复用已有路由与布局：
     - 在 `frontend/src/app/router/AppRouter.tsx` 中接入新的后台管理页（如 `AdminPage` 或 `TenantAdminPage`），以及与工作区/权限相关的页面。
     - 保持 `MainLayout` 统一布局，新增菜单项仅在已授权用户可见（结合 `PermissionGuard`）。
   - 权限模型：
     - 基于你新增的 `use-authorization.ts` 与 `use-permission-catalog.tsx`，前端从后端拉取当前用户的权限/角色列表及权限 catalog，并在本地缓存（context 或 zustand/类似状态管理方案）中使用。
     - 提供 `PermissionGuard` 组件（已经有文件和测试骨架），接受 `anyOf` / `allOf` 等权限 key，内部使用 `useAuthorization` 进行判断，从而在页面和按钮级别进行显示/禁用控制。
   - 工作区交互：
     - 在 `frontend/src/features/workspace` 里继续完善现有组件：
       - `FileExplorer`：从新建的 files API 拉取文件树，支持点击节点后在 `FilePreview` 中展示内容。
       - `CommandConsole`：从后端 commands API 拉取命令 catalog，允许在当前 workspace 上执行命令，并在控制台展示执行日志与结果。
       - `StagingPanel`：展示当前 workspace 的暂存文件/变更，未来可扩展为与 git 或自研版本管理的集成。
   - 认证与路由守卫：
     - 利用 `auth-context.tsx` 和 `LoginPage.tsx`，在登录成功后拉取用户信息和权限列表，并将其灌入 Authorization context。
     - `AppRouter` 中针对需要权限的路由使用 `PermissionGuard` 包裹，未授权时跳转到 403 页面或展示提醒组件（可在 `shared/ui` 新增简单组件）。

3. **RBAC 权限与种子数据**
   - 数据 schema：
     - 依据 `0011_rbac_permissions.sql` 定义 `permissions`、`roles`、`role_permissions`、`user_roles` 等表；
     - 每条 permission 使用 machine-readable key（如 `workspace.read`, `workspace.command.run`, `tenant.admin.manage`），与前后端统一。
   - Seed 逻辑：
     - 在 `backend/internal/tenant/permissions_seed.go` 中实现一次性/幂等的种子数据写入逻辑（检测不存在再插入）。
     - 提供测试 `permissions_seed_test.go` 覆盖：
       - 首次执行创建所有默认权限和角色
       - 再次执行不重复创建（幂等性）
   - 权限检查：
     - 在 `backend/internal/auth/checker.go` 或新模块中实现 `HasPermission(userID, tenantID, permissionKey)` 等接口；
     - HTTP handler 中通过中间件封装权限检查（例如：`RequirePermission("workspace.command.run")`）。

4. **API 设计草案（供确认后细化）**
   - 命令相关（`/api/workspaces/{workspaceId}/commands`）：
     - `GET /catalog`：获取可用命令列表（需要 `workspace.command.view`）
     - `POST /execute`：在 workspace 内执行命令（需要 `workspace.command.run`）
     - `GET /history`：查看命令执行历史（需要 `workspace.command.history`）
   - 文件相关（`/api/workspaces/{workspaceId}/files`）：
     - `GET /tree`：获取目录树
     - `GET /content?path=...`：读取文件内容
     - （如有需要）`POST /stage`、`DELETE /stage`：管理暂存变更
   - 权限/角色相关：
     - `GET /api/admin/permissions/catalog`：前端 permission-catalog 初始化
     - `GET /api/admin/users/{id}/permissions`：当前用户实际权限（含角色展开）

5. **验证与测试计划（Spec 阶段）**
   - 后端：
     - 使用 `go test ./...` 作为基础验证命令；对新增模块补充 table-driven 单测。
     - 在引入 HTTP handler 时，以 handler 层单测 + service 层单测为主，不强行上 e2e 测试，避免过重。
   - 前端：
     - 使用 `npm test` 或 `npm run test`（需在 `frontend/package.json` 中确认）跑单测，确保新增 hooks 和组件（尤其是 `PermissionGuard`、`useAuthorization` 等）都有测试覆盖。
     - 针对关键交互（登录后看到的菜单/按钮是否按权限展示）写 1-2 个 React Testing Library 测试用例即可。

---

如果你回复“确认”或进一步补充你想优先完成的子任务（例如：先搞定 RBAC、或先完成 workspace 文件浏览等），我就会在退出 Spec Mode 后：
- 先用 Grep/Glob 深入对齐现有实现（如已有的 handler/service 模式）
- 然后按你确认的顺序逐步实现对应后端和前端代码
- 并补充/修正相应的测试与本地验证命令