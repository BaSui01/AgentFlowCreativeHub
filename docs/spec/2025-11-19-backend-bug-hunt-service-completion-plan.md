## 🎯 目标
在工具管理 API 完成之后，继续排查剩余模块的缺陷与半成品实现，优先补齐会影响主流程的服务，确保工作流、审批、通知、OAuth2 等核心链路具备可测试、可观测的交付质量。

> 注：尝试调用 `sequential-thinking` 工具时出现连接错误（两次重试均失败），因此以下方案基于现有代码阅读与 TODO 排查结果整理。

---

## 1. 工作流执行链路回归与修复
1. **Worker 侧 RunExecution 注释未落地**  
   - 文件：`internal/worker/handlers/workflow_handler.go` 第 38 行仍保留 “需要新增” 注释，需要确认当前 `executor.Engine.RunExecution` 是否真正被调用、以及任务失败是否会重试。  
   - 计划：补齐 Asynq 任务入队/消费路径的单测（使用 `asynqtest` 或 stub engine），验证 `RunExecution` 被调用、失败时记录错误并支持重试策略。  
2. **执行记录/任务查询缺回归**  
   - 虽然 `WorkflowExecuteHandler.GetExecution/ListExecutions` 已实现，但缺乏多租户过滤、分页、状态过滤的自动化测试。  
   - 计划：基于 sqlite/memory DB 编写 handler 级别测试，覆盖：不同租户隔离、status filter、分页正确性，以及任务（`workflow.WorkflowTask`）联查。必要时补充索引或错误处理。  
3. **调度并发 TODO**  
   - `executor.NewScheduler(..., 5)` 留有 “TODO: Configurable concurrency”。  
   - 计划：增加 Engine 配置项或从环境变量读取并发度，外加单位测试验证默认值/自定义值生效。

### 交付物
- Worker handler 单元测试 + 文档化的重试策略
- Workflow execute handler 的 httptest 覆盖（get/list）
- Scheduler 并发配置化实现及测试

---

## 2. 审批 & 通知服务补完
1. **审批通知未实现**  
   - `workflow/approval/manager.go` 第 60 行 TODO：`sendNotification` 目前只是空壳。  
   - 计划：实现通知派发（调用 `internal/notification` 的 webhook / email / future websocket），并在创建审批请求时写审计/日志。  
2. **WebSocketNotifier 占位**  
   - `notification/notifier.go` 238 行起 WebSocket 通知器未实现。  
   - 计划：实现连接管理（map + mutex）、广播与断线清理，提供最小 API（Register/Unregister/Broadcast），并在审批通知里调用。  
3. **自动审批条件缺失**  
   - `workflow/executor/automation_engine.go` 中 `handleApproval` 标注 “TODO: 评估自动批准条件”。  
   - 计划：解析 `ApprovalConfig.AutoApproveIf`，支持根据任务输出/质量分数决定是否绕过人工审批，并编写单测覆盖 true/false 场景。

### 交付物
- 完整的通知通道实现（至少 webhook + websocket）
- 审批创建到通知的集成测试 & 日志验证
- 自动审批条件解析逻辑及单元测试

---

## 3. OAuth2 / Session 回归与观测
1. **StateStore & SessionService 缺回归**  
   - 新增的 `memory/redis StateStore`、`SessionService.RotateRefreshToken` 目前只有单元级覆盖。需要端到端测试验证 state 保存/消费、refresh token 轮换、会话撤销。  
   - 计划：使用 httptest + mock OAuth provider，模拟 `/oauth/:provider` → `/callback` → `/refresh` 全流程，确保 state 泄露/重放会被拒绝，并为默认 redis/memory 分支都跑通。  
2. **Metrics/日志缺失**  
   - 设计指标：state store 降级、refresh 失败次数、登录成功率；接入 `metrics` 包并暴露 Prometheus 指标，便于 bug 追踪。  
3. **安全回归脚本**  
   - 编写 `scripts/auth_regress.sh`（或 Go 测试）自动跑登录/刷新/登出，作为 CI 早期预警。

### 交付物
- OAuth2 集成测试（mock provider + redis/memory）
- 新增 Prometheus 指标（state_store_downgrade_total、refresh_fail_total 等）
- 文档/脚本说明如何运行回归测试

---

## 4. Bug 寻找方法论 & 工具化
- **TODO 扫描与优先级队列**：使用脚本扫描 `TODO`、`FIXME`，按模块/影响度标记，生成 `docs/bug-backlog.md`。
- **日志/指标对齐**：统一 logger field（trace_id, tenant_id, user_id），并在关键 handler（workflow、tools、auth）补齐 missing 字段，方便排查。
- **回归矩阵**：维护表格列出“功能 × 测试/监控覆盖”，提升对未完成服务的可见度。

---

### 时间与优先级建议
1. **P0**：工作流执行链路 + OAuth2/session 回归（直接影响主用户路径）
2. **P1**：审批/通知补完（自动化场景 & 多人协作）
3. **P2**：Bug backlog tooling、日志指标统一

如上计划获批后，可按模块依次实施，并在每个阶段结束时执行 `go test ./...` + 针对性集成测试，确保无回归。