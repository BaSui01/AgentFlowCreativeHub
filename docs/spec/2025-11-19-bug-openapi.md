## 1. 背景与目标
- 现状：后端以 Go+Gin（`backend/api/setup.go`）为核心，尚未集成 Swagger；前端计划依赖 OpenAPI 类型生成（`docs/spec/frontend_architecture_analysis.md`）。
- 目标：建立可重复的后端 bug 诊断流程，并落地 OpenAPI→TS 的自动化产物，同时说明 Go/Java 的可行性对比。

## 2. 后端 bug 诊断与修复流程
1. **复现场景**：通过 `.env`+`config/*.yaml` 启动 `backend/cmd/server/main.go`，并使用 `docker-compose` 提供 Postgres/Redis，保证日志/审计中间件（`internal/logger`, `api.RequestLogger`, `audit.AuditMiddleware`）开启。
2. **收集线索**：
   - 启用 `GIN_MODE=debug` 及 `LOG_LEVEL=debug`，通过 Zap/GORM 慢查询日志定位数据库、队列（Asynq）、RAG、Workflow 三大模块。
   - 利用 `metrics.PrometheusMiddleware` + `/metrics` 观察延迟、错误峰值。
   - 若为异步任务，关联 `worker.Server`（Asynq）日志，必要时启用 `asynq stats`。
3. **定位因果链**：
   - 按路由分层（auth/tenant/models/workflows/tools 等 handler）回溯，重点关注技术债列表（`docs/spec/2025-11-17-agentflowcreativehub-1.md` 中 P0/P1 TODO）。
   - 结合 `internal` 目录下 service/repository 层，确认数据一致性、事务和多租户上下文（`internal/tenant`）。
4. **修复策略**：
   - 在 service 层添加输入校验/容错，必要时扩展 `models` 中 JSON schema 字段。
   - 通过单元测试（`go test ./...`）+ 回归脚本（curl/insomnia 集）验证；对异步任务补充 integration test（可用 testify + testcontainers）。
5. **回归与监控**：修复后，确认 `/health`、`/ready`、`/metrics` 正常，审计日志写入成功；将验证命令记录于 `docs/testing.md`。

## 3. Go 版 OpenAPI 自动生成方案
1. **依赖集成**：在 `backend/go.mod` 添加 `github.com/swaggo/swag`, `github.com/swaggo/gin-swagger`, `github.com/swaggo/files`；创建 `tools.go` 维护 go:generate。
2. **注释体系**：在 `cmd/server/main.go` 顶部补充 @title/@version/@host/@BasePath；为每个 handler 方法添加 @Summary/@Description/@Tags/@Param/@Success/@Failure；如需复用结构体，集中在 `api/handlers/*/dto.go`。
3. **生成命令**：
   - 新增 `Makefile` 目标 `swagger`: `swag fmt && swag init -g cmd/server/main.go -o api/docs`。
   - 在 `cmd/server/main.go` 中注册 `router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))`。
4. **CI 钩子**：在 `frontend`/`backend` 顶层脚本中加入 `make swagger`，并在未来 CI 中缓存 `api/docs`，保证 PR 校验包含 OpenAPI 更新。

## 4. OpenAPI → 前端 TypeScript 客户端
1. **契约产物**：后端生成的 `api/docs/swagger.json` 作为单一事实源。
2. **生成工具**：在 `frontend/package.json` 添加 `"generate:api": "openapi-typescript-codegen --input ../backend/api/docs/swagger.json --output src/shared/api"`（先安装 `openapi-typescript-codegen`）。
3. **调用方式**：
   - `src/shared/api` 暴露基础 client；
   - 各 feature（如 `features/workflow/api/`）只做轻量封装；禁止手写接口类型。
4. **同步机制**：在根目录新增脚本 `pnpm gen:api` / `make gen-api`，并在前端构建前自动执行；可通过 Git hook 或 CI 确保 swagger/TS 同步。

## 5. Java 方案可行性 vs Go
- **Go**：与现有栈一致；swaggo 可直接读取 Gin 注释；生成速度快，部署单二进制；适合当前轻量、中高并发场景。
- **Java**：可用 Spring Boot + springdoc-openapi + Maven plugin 自动导出 spec + `openapi-generator-maven-plugin` 反向生成 client。优点是生态丰富、对大型团队友好；缺点是引入全新运行时、部署链路与现有 Go 基建不兼容，需要大量迁移成本。
- **结论**：在保持 Go 主体的前提下快速落地 swagger pipeline；若未来确需 Java 模块，可通过 OpenAPI 共享契约与现有 Go 服务共存（例如独立微服务），但短期内不建议替换主后端。

## 6. 验证计划
1. `go test ./...`（确保修复未破坏现有逻辑）。
2. `make swagger && git diff --exit-code api/docs`（验证文档同步）。
3. `npm --prefix frontend run generate:api && npm --prefix frontend run lint && npm --prefix frontend run build`（确保前端客户端可用）。
4. 关键回归：`curl /api/...` 核实修复的 bug 场景。

> 如认可本规格，可进入实现阶段：先补充后端注释与 Swagger 生成，再执行 bug 修复与前端客户端同步。