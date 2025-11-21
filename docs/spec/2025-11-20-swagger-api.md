## 问题概述
- 现象：`frontend/src/shared/api/services` 只有 `AuthService` 与 `SystemService`，多数业务 API 没有生成对应的 TS 客户端；Swagger 产物 `backend/api/docs/swagger.{json,yaml}` 仅包含健康检查与登录相关定义。
- 根因：大部分 Handler（workflow、tenant、tool、agent、knowledge 等）缺少 `swaggo` 注解，请求/响应结构也未以可复用 DTO 暴露，导致 `swag init` 只拾取极少数端点。
- 影响：前端无法通过统一的 OpenAPI 客户端访问主要业务接口，类型缺失且需要手写请求，容易引发运行期 404/500 及契约漂移。

## 实施计划
1. **统一响应/请求 DTO（新建 `backend/api/handlers/common/dto.go`）**
   - 定义 `APIResponse[T any]`、`ErrorResponse`、`PaginationMeta` 等结构，并在 handler 中复用。
   - 为常见请求（如分页、工作流校验、执行请求）补全显式 struct，移除匿名 struct，便于 `swag` 生成 schema。
2. **补齐核心 Handler 的 Swagger 注解**
   - 文件范围：`backend/api/handlers/workflows/*.go`、`handlers/tenant/*.go`、`handlers/agents/*.go`、`handlers/tools/tool_handler.go`、`handlers/knowledge/*.go`、`handlers/audit/*.go` 等。
   - 每个路由补充 `@Summary/@Description/@Tags/@Security/@Param/@Success/@Failure`，引用步骤 1 中的 DTO。
   - 对需要路径/查询参数的接口（如分页、过滤）使用 `@Param page query int false` 等语句，确保生成器能识别。
3. **完善执行相关接口的 Schema**
   - 将 `WorkflowExecuteHandler` 的请求/响应改写为显式 struct（含任务列表、分页信息）。
   - 对工具执行、Agent 执行接口同样提供 request/response struct，使流式和异步接口在 swagger 中可见。
4. **增强 Swagger 生成链路**
   - 在 `backend/Makefile` 新增 `swagger` 目标：`swag fmt && swag init -g cmd/server/main.go -o api/docs`。
   - 在 README 里已存在的 `tools/tools.go` 基础上，补充 `go:generate swag fmt` 与 `go:generate swag init` 指令（例如写入 `cmd/server/doc.go`）以便一键生成。
5. **同步前端 OpenAPI 客户端**
   - 运行 `npm --prefix frontend run generate:api` 生成新的 `models/`、`services/`，补全 workflows、tenants、tools 等服务。
   - 视情况更新 `frontend/src/shared/api/core/OpenAPI.ts`（如需调整 BASE/headers）。
6. **验证步骤**
   - 后端：`cd backend && go test ./...` 确认注解改动未破坏业务逻辑。
   - Swagger：`cd backend && make swagger`，对 `api/docs` 做 `git diff`，确认新增端点。
   - 前端：`npm --prefix frontend run generate:api && npm --prefix frontend run lint && npm --prefix frontend run build`，确保生成代码通过静态检查与构建。

## 交付物
- 新增的 DTO 文件与完善后的 handler 注释。
- 更新后的 `api/docs/swagger.{json,yaml}`。
- 重新生成的前端 OpenAPI 客户端（services/models）。
- `Makefile`/`go:generate` 脚本与验证所需命令记录。
