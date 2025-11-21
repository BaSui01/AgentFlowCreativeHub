## 目标
1. 补齐工作流执行链路的测试与鲁棒性（Asynq worker、执行查询 API）。
2. 完成审批通知与自动通过逻辑，使自动化执行节点可真正暂停/恢复。
3. 为 OAuth2/state/session 链路建立端到端验证与指标，确保回归可观测。
4. 补齐待办扫描工具，为回归矩阵提供数据基础。

## 工作流 Worker / Handler 测试
1. **WorkflowHandler 抽象化依赖**
   - 在 `internal/worker/handlers/workflow_handler.go` 中引入 `WorkflowRunner` 接口，仅暴露 `RunExecution(ctx, execID)`，并让 `executor.Engine` 实现。
   - handler 结构体改为依赖该接口，便于注入 mock。
2. **Worker handler 单测** (`internal/worker/handlers/workflow_handler_test.go`)
   - 使用自定义 fake runner 记录调用次数与注入错误，构造 asynq.Task 载荷（含 execution_id/workflow_id）。
   - 覆盖场景：成功（RunExecution 调用一次）、engine 返回错误（handler 返回 error 以便 Asynq 重试）、payload JSON 无效（直接返回解码错误）。
3. **WorkflowExecuteHandler httptest** (`api/handlers/workflows/execute_handler_test.go`)
   - 使用 sqlite-in-memory GORM 初始化，并批量插入不同 tenant 的 `WorkflowExecution`/`WorkflowTask`。
   - 用 gin TestMode 构造请求：
     - `GetExecution`：同租户成功返回任务列表、不同租户 404。
     - `ListExecutions`：`status` 过滤 + 分页字段校验；确保其它租户数据不可见；验证 `total`/`total_page` 计算正确。
   - 为 handler 添加小型工厂（注入 fake engine 以避免真实执行）。

## 审批通知 sendNotification + WebSocket
1. **通知器接入**
   - 在 `approval.Manager` 中注入 `notification.MultiNotifier`（通过 `SetNotifier`），并在 `sendNotification` 里从数据库加载 Workflow 名称（`workflow.Workflow`），完善 `WorkflowName/StepName`。
   - 将 email 接收人/ webhook URL 委托给配置或审批输入：扩展 `ApprovalRequestInput` 以允许 `NotifyTargets map[string]string` （如 email/webhook）。
2. **WebSocket 支持**
   - 在 `notification` 包新增 `hub.go`：维护租户/用户 -> 连接 map，使用 gorilla/websocket 或 channel 抽象；`WebSocketNotifier.Send` 将消息广播给在线连接，不在线则静默。
   - 在 API 层新增 `/api/ws/notifications`（需要 JWT），创建连接后将 `tenant_id/user_id` 注册到 hub。
3. **sendNotification 实现**
   - 根据 `approval.NotifyChannels` 构造通知体并调用 hub；捕获错误写入 logger。
   - 补充单元测试：使用 stub notifier 验证 email/webhook/websocket 分支被调用。

## 审批自动通过条件
1. **表达式评估**
   - 在 `AutomatedTaskExecutor.handleApproval` 中，当 `ApprovalConfig.AutoApproveIf` 存在时，使用现有 `ConditionExecutor` 评估表达式（利用任务上下文 `execCtx`），若满足则直接返回 true 并记录审计（新增 `approval.Manager.AutoApprove` 轻量方法）。
2. **状态更新**
   - 为 `workflow.ApprovalRequest` 增加 `AutoApproved bool` 字段；`Manager` 在自动通过时写入 `resolved_at`/`approved_by = "system"` 并跳过通知。
3. **测试**
   - 在 `automation_engine_test.go` / 新增文件中模拟步骤输出并验证：条件满足时不调用 `CreateApprovalRequest`，不满足时依旧走人工审批。

## OAuth2 / State / Session E2E + 指标
1. **测试基建**
   - 在 `api/handlers/auth` 或 `internal/auth` 下新增 `auth_flow_test.go`：
     - 使用 httptest + sqlite + MemoryStateStore + fake OAuth provider（实现 `ExchangeCode` 接口）模拟完整流程：`/oauth/:provider` -> state 保存 -> callback -> session creation -> refresh -> logout。
     - 验证：state 只能消费一次、`SessionService.RotateRefreshToken` 被调用、审计日志有记录。
2. **指标** (`internal/metrics/auth.go`)
   - 新增 Prometheus 统计：`auth_oauth2_requests_total{provider}`, `auth_sessions_active`, `auth_state_validation_failures_total`。
   - 在 Auth handler 中埋点：state 校验失败、OAuth 成功/失败、session 创建/撤销。
3. **回归脚本**
   - 在 `Makefile` or `scripts/auth_e2e.sh` 添加命令运行上述 go test，并输出指标抓取说明。

## TODO 扫描与回归矩阵
1. **扫描器实现**
   - 新建 `cmd/tools/todo_report/main.go`：
     - 使用 filepath.Walk 读取仓库（可接收 `--include`/`--exclude`），匹配 `TODO`/`FIXME`，解析责任人标签如 `TODO(@owner):`。
     - 输出 JSON + Markdown（`docs/regression/todo_report.md`）含文件/行/上下文。
2. **回归矩阵文档**
   - 自动生成 `docs/regression/matrix.md`，列出模块（Auth/Workflow/Approval/Tools）与测试覆盖情况，基于扫描结果和手工配置。
3. **CI Hook（可选）**
   - 在 `package.json` 或 `Makefile` 中添加 `todo-report` 目标，供后续 pipeline 使用。

## 验证计划
- `go test ./backend/internal/worker/... ./backend/api/handlers/workflows ./backend/internal/workflow/... ./backend/api/handlers/auth`（包含新增 E2E）。
- WebSocket hub 通过单元测试模拟多连接广播；OAuth 指标通过暴露 prometheus registry 并断言 Counter/Gauge 值。
- 文档输出后通过 lint/markdownlint（如有）确保格式正确。
