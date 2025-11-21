# 🚀 MultiAgent Creative Hub - 下一步实施计划 v2.0

> **制定日期**: 2025-11-17  
> **基于**: 项目当前状态全面评估  
> **目标**: 从数据模型完成到可运行的 MVP

---

## 📊 当前状态总结

### ✅ 已完成（估计 40%）

1. **数据模型层** ✅ 100%
   - 11 个核心模块的 Go 模型（Tenant、User、Role、Workflow、Agent、Template、Model、RAG...）
   - 所有模型支持软删除、时间戳、GORM 标签
   - 4 个数据库迁移脚本

2. **基础架构层** ✅ 75%
   - 多租户中间件
   - RBAC 权限控制
   - 审计日志
   - 配置缓存
   - 软删除查询范围

3. **Service 层** ⚠️ 30%
   - ✅ TenantService、UserService、RoleService、ConfigService
   - ❌ WorkflowService、AgentService、TemplateService、ModelService

4. **API 层** ⚠️ 20%
   - ✅ 租户管理路由
   - ❌ Workflow、Agent、Template、Model 路由

### ❌ 缺失关键模块（估计 60%）

1. **AI 模型适配层** - 0%
2. **Agent 运行时** - 0%
3. **工作流编排引擎** - 0%
4. **RAG 实现** - 5%（仅接口）
5. **前端控制台** - 0%
6. **配置管理系统** - 0%（Viper 未集成）
7. **测试** - 0%

---

## 🎯 下一步计划（分 3 个 Sprint）

### 🔥 Sprint 1: 基础设施与配置 (3-5 天)

**目标**: 完善基础设施，实现可启动的后端服务

#### 任务 1.1: 完善项目基础设施 ⭐ 优先级 P0

<details>
<summary>详细任务清单</summary>

**1.1.1 完善 go.mod 和依赖管理**
- [ ] 补充缺失的依赖
  ```go
  // 需要添加的依赖
  - github.com/gin-gonic/gin (Web 框架)
  - gorm.io/gorm (ORM)
  - gorm.io/driver/postgres (PostgreSQL 驱动)
  - github.com/spf13/viper (配置管理)
  - go.uber.org/zap (日志)
  - github.com/google/uuid (UUID 生成)
  - github.com/go-redis/redis/v8 (Redis 客户端)
  ```
- [ ] 运行 `go mod tidy`
- [ ] 创建 `go.sum`

**1.1.2 配置管理系统**
- [ ] 创建 `backend/internal/config/config.go`
  - 定义 Config 结构体
  - 使用 Viper 加载配置
  - 支持多环境（dev/staging/prod）
  - 环境变量覆盖机制

- [ ] 创建配置文件
  - `config/dev.yaml` - 开发环境
  - `config/prod.yaml` - 生产环境
  - `.env.example` - 环境变量模板

**1.1.3 日志系统**
- [ ] 创建 `backend/internal/logger/logger.go`
  - 集成 Zap 结构化日志
  - 添加 TraceID 支持
  - 日志级别控制
  - JSON 格式输出

**1.1.4 数据库连接**
- [ ] 完善 `backend/internal/infra/db.go`
  - GORM 初始化
  - 连接池配置
  - 自动迁移（开发环境）
  - 健康检查

**1.1.5 主程序入口**
- [ ] 创建 `backend/cmd/server/main.go`
  - 加载配置
  - 初始化日志
  - 初始化数据库
  - 启动 HTTP 服务
  - 优雅关闭

**预计工时**: 2 天  
**交付物**: 可启动的后端服务（监听 8080 端口，支持健康检查）

</details>

---

#### 任务 1.2: 补全核心 Service 层 ⭐ 优先级 P0

<details>
<summary>详细任务清单</summary>

**1.2.1 ModelService（AI 模型管理）**
- [ ] 创建 `backend/internal/models/service.go`
  - ListModels() - 查询模型列表
  - GetModel(id) - 查询单个模型
  - CreateModel() - 创建模型配置
  - UpdateModel() - 更新模型配置
  - DeleteModel() - 软删除模型
  - SeedDefaultModels() - 初始化预置模型

**1.2.2 TemplateService（Prompt 模板管理）**
- [ ] 创建 `backend/internal/template/service.go`
  - ListTemplates() - 查询模板列表（支持过滤）
  - GetTemplate(id) - 查询单个模板
  - CreateTemplate() - 创建模板
  - UpdateTemplate() - 更新模板
  - DeleteTemplate() - 软删除模板
  - CreateVersion() - 创建模板版本
  - GetLatestVersion() - 获取最新版本
  - RenderTemplate() - 渲染模板（变量注入）

**1.2.3 AgentService（Agent 配置管理）**
- [ ] 创建 `backend/internal/agent/service.go`
  - ListAgentConfigs() - 查询 Agent 配置
  - GetAgentConfig(id) - 查询单个配置
  - CreateAgentConfig() - 创建配置
  - UpdateAgentConfig() - 更新配置
  - DeleteAgentConfig() - 软删除配置

**1.2.4 WorkflowService（工作流管理）**
- [ ] 创建 `backend/internal/workflow/service.go`
  - ListWorkflows() - 查询工作流列表
  - GetWorkflow(id) - 查询单个工作流
  - CreateWorkflow() - 创建工作流
  - UpdateWorkflow() - 更新工作流
  - DeleteWorkflow() - 软删除工作流
  - ValidateWorkflow() - 验证工作流定义

**预计工时**: 2 天  
**交付物**: 4 个完整的 Service 实现

</details>

---

#### 任务 1.3: 补全 API 路由层 ⭐ 优先级 P1

<details>
<summary>详细任务清单</summary>

**1.3.1 Models API**
- [ ] 创建 `backend/api/handlers/models/model_handler.go`
  - GET `/api/models` - 查询模型列表
  - GET `/api/models/:id` - 查询单个模型
  - POST `/api/models` - 创建模型
  - PUT `/api/models/:id` - 更新模型
  - DELETE `/api/models/:id` - 删除模型

**1.3.2 Templates API**
- [ ] 创建 `backend/api/handlers/templates/template_handler.go`
  - GET `/api/templates` - 查询模板列表
  - GET `/api/templates/:id` - 查询单个模板
  - POST `/api/templates` - 创建模板
  - PUT `/api/templates/:id` - 更新模板
  - DELETE `/api/templates/:id` - 删除模板
  - POST `/api/templates/:id/versions` - 创建版本
  - POST `/api/templates/:id/render` - 渲染模板

**1.3.3 Agents API**
- [ ] 创建 `backend/api/handlers/agents/agent_handler.go`
  - GET `/api/agents` - 查询 Agent 配置
  - GET `/api/agents/:id` - 查询单个配置
  - POST `/api/agents` - 创建配置
  - PUT `/api/agents/:id` - 更新配置
  - DELETE `/api/agents/:id` - 删除配置

**1.3.4 Workflows API**
- [ ] 创建 `backend/api/handlers/workflows/workflow_handler.go`
  - GET `/api/workflows` - 查询工作流列表
  - GET `/api/workflows/:id` - 查询单个工作流
  - POST `/api/workflows` - 创建工作流
  - PUT `/api/workflows/:id` - 更新工作流
  - DELETE `/api/workflows/:id` - 删除工作流

**1.3.5 更新 router.go**
- [ ] 注册所有新路由
- [ ] 添加中间件（TenantContext、RBAC）

**预计工时**: 1.5 天  
**交付物**: 完整的 REST API（支持 CRUD 操作）

</details>

---

### 🚀 Sprint 2: AI 模型适配与 Agent 运行时 (5-7 天)

**目标**: 实现 AI 模型调用和 Agent 执行

#### 任务 2.1: AI 模型适配层 ⭐ 优先级 P0

<details>
<summary>详细任务清单</summary>

**2.1.1 统一模型客户端接口**
- [ ] 创建 `backend/internal/ai/client.go`
  ```go
  type ModelClient interface {
      ChatCompletion(ctx, request) (response, error)
      ChatCompletionStream(ctx, request) (stream, error)
      Embedding(ctx, texts) ([][]float64, error)
  }
  ```

**2.1.2 OpenAI 适配器**
- [ ] 创建 `backend/internal/ai/openai/client.go`
  - 集成 `go-openai` SDK
  - 实现 ModelClient 接口
  - 支持 GPT-4、GPT-3.5-turbo
  - 支持流式响应（SSE）
  - Token 计数
  - 成本计算
  - 重试机制（指数退避）

**2.1.3 Claude 适配器**
- [ ] 创建 `backend/internal/ai/anthropic/client.go`
  - 集成 `anthropic-sdk-go` SDK
  - 实现 ModelClient 接口
  - 支持 Claude 3.5 Sonnet
  - 支持流式响应

**2.1.4 模型客户端工厂**
- [ ] 创建 `backend/internal/ai/factory.go`
  - 根据 Model 配置创建对应客户端
  - 支持多提供商
  - 连接池管理

**2.1.5 模型调用日志**
- [ ] 实现 ModelCallLog 自动记录
  - 拦截器模式
  - 异步写入数据库
  - 成本统计

**预计工时**: 3 天  
**交付物**: 可调用 OpenAI/Claude API 的统一客户端

</details>

---

#### 任务 2.2: Agent 运行时 ⭐ 优先级 P0

<details>
<summary>详细任务清单</summary>

**2.2.1 Agent 接口定义**
- [ ] 创建 `backend/internal/agent/agent.go`
  ```go
  type Agent interface {
      Execute(ctx, input) (output, error)
      ExecuteStream(ctx, input) (stream, error)
      Name() string
      Type() string
  }
  ```

**2.2.2 实现基础 Agent**
- [ ] WriterAgent - 内容创作
- [ ] ReviewerAgent - 内容审校
- [ ] FormatterAgent - 格式化

**2.2.3 Agent 上下文管理**
- [ ] 创建 `backend/internal/agent/context.go`
  - AgentContext（输入、输出、元数据）
  - 历史对话管理
  - 状态持久化

**2.2.4 Agent 注册机制**
- [ ] 创建 `backend/internal/agent/registry.go`
  - 根据 AgentType 获取 Agent 实例
  - 支持动态注册

**2.2.5 Agent API**
- [ ] POST `/api/agents/:type/execute` - 执行 Agent
- [ ] POST `/api/agents/:type/execute-stream` - 流式执行

**预计工时**: 3 天  
**交付物**: 可通过 API 调用 Agent 生成内容

</details>

---

### 🎨 Sprint 3: 工作流编排引擎 (7-10 天)

**目标**: 实现多 Agent 协作的工作流编排

#### 任务 3.1: 工作流编排引擎 ⭐ 优先级 P0

<details>
<summary>详细任务清单</summary>

**3.1.1 工作流解析器**
- [ ] 创建 `backend/internal/workflow/parser.go`
  - 解析 YAML/JSON 工作流定义
  - 验证工作流合法性
  - 构建 DAG（有向无环图）

**3.1.2 任务调度器**
- [ ] 创建 `backend/internal/workflow/scheduler.go`
  - 拓扑排序（确定执行顺序）
  - 依赖解析
  - 并行执行支持

**3.1.3 执行引擎**
- [ ] 创建 `backend/internal/workflow/executor.go`
  - 状态机实现
  - 任务执行
  - 错误处理
  - 重试机制
  - 超时控制

**3.1.4 高级特性**
- [ ] 并行执行（goroutine 池）
- [ ] 条件分支（if/else）
- [ ] 循环（for/while）
- [ ] 人工审核节点（暂停等待）

**3.1.5 工作流 API**
- [ ] POST `/api/workflows/:id/execute` - 执行工作流
- [ ] GET `/api/workflows/:id/executions` - 查询执行记录
- [ ] GET `/api/executions/:id` - 查询执行详情
- [ ] POST `/api/executions/:id/pause` - 暂停执行
- [ ] POST `/api/executions/:id/resume` - 恢复执行
- [ ] POST `/api/executions/:id/cancel` - 取消执行

**预计工时**: 6 天  
**交付物**: 可执行工作流的编排引擎

</details>

---

## 📋 开发优先级矩阵

| 任务 | 重要性 | 紧急度 | 依赖 | 预计工时 |
|------|-------|-------|------|---------|
| 完善基础设施（配置、日志、DB） | ⭐⭐⭐⭐⭐ | 🔥🔥🔥 | 无 | 2 天 |
| 补全 Service 层 | ⭐⭐⭐⭐⭐ | 🔥🔥🔥 | 基础设施 | 2 天 |
| 补全 API 路由 | ⭐⭐⭐⭐ | 🔥🔥 | Service 层 | 1.5 天 |
| AI 模型适配层 | ⭐⭐⭐⭐⭐ | 🔥🔥🔥 | 基础设施 | 3 天 |
| Agent 运行时 | ⭐⭐⭐⭐⭐ | 🔥🔥🔥 | 模型适配层 | 3 天 |
| 工作流编排引擎 | ⭐⭐⭐⭐⭐ | 🔥🔥 | Agent 运行时 | 6 天 |
| 单元测试 | ⭐⭐⭐ | 🔥 | 各模块完成 | 持续进行 |

---

## 🎯 3 周后目标（MVP）

### 功能目标
- ✅ 可通过 API 管理租户、用户、角色
- ✅ 可通过 API 管理 AI 模型配置
- ✅ 可通过 API 管理 Prompt 模板
- ✅ 可通过 API 配置 Agent
- ✅ 可通过 API 调用单个 Agent 生成内容
- ✅ 可通过 API 创建和执行工作流
- ✅ 支持流式响应（SSE）
- ✅ 支持多租户隔离
- ✅ 支持 RBAC 权限控制
- ✅ 完整的审计日志

### 性能目标
- API 响应 P95 < 500ms（不含 AI 调用）
- 支持 10+ 并发工作流
- 数据库查询 P95 < 50ms

### 质量目标
- 核心模块测