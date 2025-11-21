## 目标
评估 `docs/` 是否覆盖并反映当前 Go 代码实现，并给出让《架构设计文档》细化到每个 Go 文件的改进路线。

## 既有实现参照
1. `backend/internal/workflow/service.go`：展示 WorkflowService 的 CRUD/校验逻辑，与文档中的“工作流编排”章节对应但缺少细节。
2. `backend/api/handlers/workflows/workflow_handler.go`：HTTP Handler 层调用 WorkflowService，当前架构文档仅笼统描述“API 路由层”。
3. `backend/internal/agent/runtime/agent.go`：定义 Agent 接口、输入输出与执行选项，文档只提到“Agent 模块”未覆盖该级别说明。

## 执行步骤
1. **文档基线采样**  
   - 枚举 `docs/` 下所有文件、版本信息、更新时间，生成对照表。  
   - 粗读核心文档（架构、技术栈、需求、数据库、计划）标记声明的模块、接口、依赖、部署信息。

2. **代码架构盘点**  
   - 使用 glob/解析列出所有 Go 文件，按 `backend/internal/*`、`backend/api/*`、`backend/pkg/*`、`cmd/*` 等分组。  
   - 提取每组的职责摘要与关键结构体/接口，确认与文档章节对应关系（例如 workflow、agent、rag、tenant、auth、tools、metrics 等）。

3. **文档 vs 实现差异分析**  
   - 对比架构文档中的模块列表与实际代码目录，标出缺失（如文档写了 `internal/user` 但代码不存在）或新增未记录模块（如 `internal/metrics`、`internal/agent/runtime` 的细化）。  
   - 检查文档的接口/数据流是否与代码示例一致（例如 WorkflowService 请求字段、Agent 输入输出）。

4. **架构文档细化方案**  
   - 设计“模块 → Go 文件”映射清单，说明每个文件的责任、依赖、关键导出元素。  
   - 拟定章节模板（模块简介、对应目录、关键文件表、调用关系、外部依赖）以便将架构文档扩展到文件粒度。  
   - 标记需要更新的其他文档（如技术栈或数据库文档）并说明原因/优先级。

5. **输出内容**  
   - 文档覆盖现状表（含是否最新、主要内容）。  
   - 差异与风险清单（陈旧/缺失/冲突）。  
   - 架构文档文件级细化建议（示例表格 + 后续工作序列）。

请确认以上方案，确认后我将按步骤执行并形成完整分析报告。