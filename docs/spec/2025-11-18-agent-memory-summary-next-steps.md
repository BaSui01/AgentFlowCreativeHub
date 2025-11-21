# 下一步计划（Spec）

## 1. 当前状态小结
- **Memory**：已支持 `history_limit` 窗口记忆；新增 Summary Memory：
  - `memory_mode="summary"` + 会话足够长时，后台异步调用模型生成会话摘要并存入 `memory_summary`；
  - EnrichInput 会在历史消息前注入一条系统摘要消息 + 最近 N 条对话。
- **RAG**：`RAGHelper` 已支持 `rag_mode`（none/stuff/map_reduce），Map-Reduce 模式使用模型对检索片段先局部总结再汇总。
- **Workflow**：工作流引擎统一使用 `AgentTaskExecutor`，并发执行与调度已适配新的接口。
- **验证**：Agent/RAG/Workflow 相关包和 Agent handlers 均可独立编译通过；全量 `go test ./...` 仍因既有 auth/audit 等模块问题失败（与本次改动无直接相关）。

---

## 2. 总体目标
在现有基础上，进一步把“记忆 + RAG + Provider 抽象”做成更可控、更易用、更可扩展的能力，重点：
1. **Memory 策略完善**：补齐摘要记忆的配置面板和行为边界，避免“不可预期的截断/摘要”；
2. **RAG 策略与接口清理**：将 RAG 模式和检索参数对齐到统一接口，便于不同 Agent/步骤按需配置；
3. **Provider 抽象落地**：统一 Model/RAG/Tool Provider，打通后续“切换模型/向量库/工具执行后端”的路径；
4. **验证链路稳定**：保持 Agent/RAG/Workflow 核心路径可独立构建和自测，暂不强行修 auth 等历史问题。

---

## 3. 近期迭代计划

### 3.1 Memory 线：Summary Memory 打磨

**目标**：让“摘要记忆”成为一等公民能力，具备清晰的配置和稳定行为，不影响未开启时的兼容性。

#### 3.1.1 配置与协议收敛
- 在 `agent.AgentConfig.ExtraConfig` 中约定 Memory 相关 key：
  - `memory_mode`: `"full" | "window" | "summary"`（当前仅显式使用 `summary`，默认视为 window）；
  - `memory_history_limit`: 默认 10，可覆盖 Registry `defaultHistoryLimit`；
  - `memory_summary_enabled`: bool，是否允许自动摘要（默认跟随 `memory_mode=summary`）；
  - `memory_summary_trigger_messages`: 触发摘要所需最小消息条数（默认 30）；
  - `memory_summary_max_tokens`: 摘要长度上限（默认 512）。
- 规则：
  - **优先级**：`AgentConfig.ExtraConfig` < 请求级 `AgentInput.ExtraParams`；
  - 未配置时保持现有“纯窗口记忆”行为。

#### 3.1.2 行为细化
- 在 `Registry.Execute/ExecuteStream` 中：
  - 聚合“实际生效”的 Memory 配置（从 AgentConfig 与 ExtraParams 合并）并传递给 `EnrichInput` 和 `maybeSummarizeSession`；
  - 明确三种模式：
    - **full**：`history_limit<=0` 且不做摘要（回退到当前“全量历史”语义）；
    - **window**（默认）：只使用 `history_limit` 窗口，不做摘要；
    - **summary**：启用摘要逻辑 + 在 EnrichInput 中注入摘要 + 窗口。
- 在 `ContextManager.EnrichInput`：
  - 从已聚合的配置中读取 `memory_mode`（不再直接读取 ExtraParams，以避免耦合），并按 Spec 行为构建 `History`：
    - `summary`：`[摘要(system)] + tail(history_limit)`；
    - 其他：`tail(history_limit)`。
- 在 `maybeSummarizeSession`：
  - 改为接收“已解析好的 MemoryOptions”结构（避免多次解析 ExtraParams）；
  - 精简默认参数，确保不会在短会话上过早触发摘要；
  - 确认“每次摘要覆盖的历史条数”逻辑在并发下也安全（依赖 `ContextManager` 的锁）。

#### 3.1.3 总结模型与成本控制
- 增加摘要模型策略：
  - 默认使用 Agent 的主模型；
  - 若 `summary_model_id` 配置存在，则优先使用独立摘要模型（常选便宜/快速模型）。
- 成本控制：
  - 将摘要调用的 `MaxTokens` 和触发频率暴露为配置，避免默认参数带来不可控成本；
  - 可以在摘要请求的 Metadata 中标记 `"purpose": "memory_summary"`（如需要后续统计，可扩展 ModelCallLogger）。

#### 3.1.4 可选：Summary 恢复与调试
- 在 `Context.Data` 中保留 `memory_summary`，方便上层调试或在 UI 中展示“会话摘要”；
- 后续可考虑添加简单 API（例如 debug endpoint 或”查看当前会话摘要“工具），但这可以放到之后的 UI/运维阶段。

---

### 3.2 RAG 线：模式与参数统一

**目标**：让所有 Agent 和 Workflow 步骤通过统一方式控制 RAG 策略，而不是散落在各处的字段与魔法值。

#### 3.2.1 RAG 配置结构
- 在 `agent.AgentConfig.ExtraConfig` 中标准化以下 key：
  - `rag_mode`: `"none" | "stuff" | "map_reduce"`（当前已支持）；
  - `rag_top_k`: 覆盖 `RAGTopK`；
  - `rag_min_score`: 覆盖 `RAGMinScore`；
  - `rag_use_summary`: 是否对 `knowledge_context` 再做一次摘要（可选）。
- 在 `AgentInput.ExtraParams` 中允许覆盖上述字段，用于 workflow 步骤细粒度控制。

#### 3.2.2 RAGHelper & RAGService 对齐
- 为 `RAGHelper.EnrichWithKnowledge` 增加一个内部配置解析函数，将 AgentConfig + ExtraParams 合并为 `RAGOptions` 结构；
- 将当前 Search 调用统一改为使用 `rag.SearchRequest`（已部分完成），并确保 Score/Similarity 兼容逻辑集中在一处；
- 在 Map-Reduce 模式中：
  - 加强错误与超时控制（map 阶段单片失败可跳过，reduce 失败回退 stuff）；
  - 限制参与 map 的片段数量（已有 maxChunks=5，可暴露为配置 `rag_map_max_chunks`）。

#### 3.2.3 工作流集成
- 在 Workflow `StepDefinition.ExtraConfig` 中沿用同一组 RAG key，确保在工作流 YAML/JSON 中的配置与 Agent ExtraConfig 一致；
- 检查 `Researcher`、`Analyzer` 等 Agent 在 Workflow 中使用时，RAG 行为与单次调用一致。

---

### 3.3 Provider 线：Model/RAG/Tool Provider 抽象

**目标**：统一 Provider 抽象，为后续“切换模型栈/向量库/工具执行后端”打基础。

#### 3.3.1 ModelProvider 抽象
- 在 `internal/ai` 包新增接口：
  - `type ModelProvider interface { GetClient(ctx, tenantID, modelID string) (ModelClient, error) }`
  - 由现有 `ClientFactory` 实现；
- 在 Registry / RAGHelper / 未来的高阶组件中依赖 `ModelProvider` 接口，而非具体 `ClientFactory`，为后续替换或包装（例如熔断、路由）留余地。

#### 3.3.2 Retriever / RAGProvider 抽象
- 在 `internal/rag` 中定义：
  - `type Retriever interface { Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) }`
  - 由当前 `RAGService` 实现；
- 修改 `RAGHelper` 构造函数，从依赖 `*RAGService` 过渡为依赖 `Retriever` 接口，保持现有实现作为默认实现。

#### 3.3.3 Tool Provider 继续落地
- 基于已定义的 `tools.DefinitionProvider` / `ExecutionProvider`：
  - 让现有 `ToolRegistry` 和 `ToolExecutor` 实现这些接口；
  - 在 Workflow/Agent 侧（如后续引入“工具调用 Agent”）只依赖 Provider 接口，统一工具发现和执行入口。

---

### 3.4 验证策略

- 在保持当前“局部构建通过”的前提下，验证重点放在：
  - `./internal/agent/...`、`./internal/rag/...`、`./internal/workflow/...`、`./api/handlers/agents/...` 的持续可编译；
  - 针对 Summary Memory 和 RAG 新选项添加**最小单元测试**或表驱动测试：
    - Memory：不同 `memory_mode` 和 `history_limit`/`memory_summary_enabled` 组合下，`EnrichInput` 返回的 History 形态；
    - RAG：不同 `rag_mode`、score 阈值场景下，`EnrichWithKnowledge` 是否正确注入 context；
  - 全量 `go test ./...` 仍会因 auth/audit 等已有问题失败，但可在说明中注明与本次改动无因果关系。

---

## 4. 执行顺序建议

如果你确认本 spec，可以按以下顺序实施：
1. **Memory 配置&行为收敛**：引入 `MemoryOptions` 结构，统一 Registry → ContextManager → maybeSummarizeSession 之间的配置传递，并补上必要单测；
2. **RAGOptions 抽象与 RAGHelper 对齐**：统一 RAG 配置解析逻辑，补上 Map-Reduce 的参数化能力；
3. **Provider 抽象落地**：引入 `ModelProvider`/`Retriever` 接口，并无侵入地替换当前直接依赖；
4. **（可选）工具 Provider 接线与少量 Guard 测试**：完成工具侧 Provider 化。

确认后，我会退出 spec 模式并按上述顺序开始修改代码与补充测试。