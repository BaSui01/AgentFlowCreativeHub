下面是围绕 **Memory、RAG 模式、Model/RAG/Tool 接口标准化** 三条线的迭代规格说明（spec），仅是设计与步骤规划，不做任何实际修改。

---

## 一、Memory 线：会话记忆与上下文控制优化

### 1. 目标
- 在现有 `ContextManager` 基础上，引入“可插拔 Memory 策略”，解决：
  - 对话历史无限增长导致 token 爆炸；
  - 不同场景需要不同记忆风格（完整缓冲/窗口/摘要）；
  - 后续可扩展“向量记忆”等高级能力。

### 2. 现状梳理（只读）
- `internal/agent/runtime/context_manager.go`：
  - 负责按 `SessionID` 持久化/读取对话历史；
  - `EnrichInput` 用于在执行前把历史写入 `AgentInput.History`；
  - `SaveInteraction` 在执行后记录 user/assistant 对话；
  - 当前基本是“简单 Buffer Memory”，没有裁剪或摘要。

### 3. 设计方案

#### 3.1 引入 Memory 抽象接口
- 新接口（示意）：
```go
// Memory 会话记忆抽象
type Memory interface {
    // LoadHistory 根据会话ID加载历史消息（可能已做裁剪/摘要）
    LoadHistory(ctx context.Context, sessionID string, limit int) ([]runtime.Message, error)

    // SaveInteraction 保存一轮对话
    SaveInteraction(ctx context.Context, sessionID string, userMsg, assistantMsg string) error
}
```
- 在 `runtime` 层新增 `MemoryStrategy` 枚举和工厂：
```go
type MemoryStrategy string

const (
    MemoryFullBuffer  MemoryStrategy = "full_buffer"
    MemoryWindow      MemoryStrategy = "window"
    MemorySummary     MemoryStrategy = "summary"
)

type MemoryFactory interface {
    GetMemory(strategy MemoryStrategy) Memory
}
```

#### 3.2 基础策略实现
1. **FullBufferMemory**（与当前行为等价）
   - 简单把所有历史取回并返回；
   - 用于兼容现有逻辑，默认策略。
2. **WindowMemory**
   - `LoadHistory` 只返回最近 N 条（例如 10 条）消息；
   - `SaveInteraction` 仍然保存全量，裁剪发生在读取时；
   - 可通过配置或 AgentConfig.ExtraConfig 控制窗口大小。
3. **SummaryMemory（摘要记忆）**
   - 引入额外表/存储记录“摘要消息”；
   - 策略：
     - 每 N 轮对话后触发一次总结：调用指定模型生成 Conversation Summary，保存为 system 消息；
     - 下次 LoadHistory 时：先返回摘要，再拼接最近几条详细消息；
   - 需要一个 `SummaryService` 或复用现有模型调用层（ModelProvider）。

#### 3.3 将 Memory 接入 Registry / ContextManager
- 在 `Registry` 中：
  - 持有一个 `MemoryFactory`；
  - `Execute` / `ExecuteStream` 里原来直接调用 `ContextManager.EnrichInput` 的地方改为：
    - 根据 AgentConfig 或全局配置选择 MemoryStrategy；
    - 调用 `memory.LoadHistory` 填充 `AgentInput.History`；
  - 执行成功后调用 `memory.SaveInteraction`。
- `AgentConfig.ExtraConfig` 可增加可选字段：
  - `memory_strategy: "full_buffer" | "window" | "summary"`
  - `memory_window_size: 10`

#### 3.4 验收要点
- 兼容：默认不配置时行为与当前一致；
- 可对某个 Agent 或某个工作流配置 Memory 策略并验证：
  - Token 使用可控（窗口/摘要）；
  - summary 生成不会影响主要业务响应。

---

## 二、RAG 模式线：从“简单 stuff”到可配置多模式

### 1. 目标
- 在现有 `RAGHelper` + `rag.RAGService` 基础上：
  - 抽象出 `Retriever` 接口；
  - 支持多种 RAG 模式（stuff / map-reduce / refine），至少先落地 2 种；
  - 将模式选择下沉到 AgentConfig/ExtraConfig，可按 Agent/Step 不同配置。

### 2. 现状梳理（只读）
- `runtime.RAGHelper`：
  - `EnrichWithKnowledge`：
    - 按 AgentConfig 中 `KnowledgeBaseID/RAGTopK/RAGMinScore` 调用 `ragService.Search`；
    - 过滤分数，拼接成 `knowledge_context` 文本，写入 `AgentContext.Data`；
  - `InjectKnowledgeIntoPrompt`：把 `knowledge_context` 拼到 System Prompt 前；
  - 当前属于典型“stuff”模式，逻辑集中在一处，便于演进。

### 3. 设计方案

#### 3.1 抽象 Retriever 接口
- 在 `internal/rag` 或 `internal/rag/retriever` 中引入：
```go
type Retriever interface {
    Search(ctx context.Context, kbID string, query string, topK int, minScore float64) ([]*SearchResult, error)
}
```
- `RAGService` 实现该接口，内部可以切换 pgvector/Milvus 等实现；
- `RAGHelper` 依赖的是 `Retriever` 接口，而不是具体 `RAGService` 类型。

#### 3.2 定义 RAG 模式与策略
- 在 `runtime` 中引入：
```go
type RAGMode string

const (
    RAGModeNone      RAGMode = "none"
    RAGModeStuff     RAGMode = "stuff"
    RAGModeMapReduce RAGMode = "map_reduce"
    RAGModeRefine    RAGMode = "refine" // 可后续实现
)
```
- 在 `AgentConfig.ExtraConfig` 中允许配置：
  - `rag_mode: "stuff" | "map_reduce" | "refine"`；
  - 也可以在 Step ExtraConfig 中覆盖。

#### 3.3 落地两种模式
1. **Stuff 模式（现有实现）**
   - 继续使用现有 `buildContextText` + `InjectKnowledgeIntoPrompt`；
   - 只需抽象为一个 `RAGStrategy` 实现：
```go
type RAGStrategy interface {
    Enrich(ctx context.Context, retriever Retriever, cfg *agent.AgentConfig, input *AgentInput) (*AgentInput, error)
}
```
2. **Map-Reduce 模式（新增）**
   - 思路：
     - Step 1：对每个检索到的 chunk，调用模型生成简要总结（map）；
     - Step 2：把所有小总结再交给模型做一次总的总结（reduce）；
     - Step 3：将“总总结”作为 `knowledge_context` 注入 Prompt；
   - 实现：
     - `MapReduceRAGStrategy` 使用 `ModelClient` 多次调用模型；
     - 为避免成本爆炸：
       - 限制 chunk 数量（TopK）；
       - 对 map 阶段控制每个 chunk 的 token 上限；
       - 在 ExtraConfig 中允许配置 `rag_map_model`/`rag_reduce_model`，默认用主模型或相对便宜模型。

#### 3.4 接入点
- 在各 Agent（Writer/Reviewer/Researcher 等）的执行前调用：
  - `ragStrategy := RAGStrategyFactory.FromConfig(agentConfig)`；
  - `input = ragStrategy.Enrich(ctx, retriever, agentConfig, input)`；
- 对调用方透明，只需依赖 AgentConfig/ExtraConfig 配置 RAG 模式。

#### 3.5 验收要点
- 默认（不配置 rag_mode）行为与现在一致（stuff）；
- 为某个 ResearcherAgent 配置 map-reduce 模式后：可以看到 prompt 中使用的是“多文档总结后的 context”，输出更聚合；
- 出错降级：如果 RAG 检索或 map/reduce 任何一步失败，回退到“无 RAG”或“简单 stuff”并记录日志，而不是直接报错中断。

---

## 三、Model/RAG/Tool 接口标准化线：Provider 抽象统一

### 1. 目标
- 将当前分散的 `ai.ClientFactory`、`rag.RAGService`、`ToolRegistry/ToolExecutor` 抽象成统一的 Provider 层：
  - 便于替换底层实现（直接 SDK vs Langchaingo vs LocalAI）；
  - 便于在配置和测试中注入 mock 实现；
  - 为后续“插件化”打基础。

### 2. 现状梳理（只读）
- 模型：`internal/ai.ClientFactory` 基于 ModelID/TenantID 返回具体 `ModelClient`（go-openai 封装），但接口命名/功能尚未统一对外暴露；
- RAG：`rag.RAGService` 直接被 `RAGHelper` 使用；
- 工具：`ToolRegistry` 管理定义，`ToolExecutor` 执行工具，`ToolHelper` 组合调用过程。

### 3. 设计方案

#### 3.1 ModelProvider 接口
- 在 `pkg/aiinterface` 或 `internal/ai` 中规范：
```go
type ModelProvider interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, <-chan error)
    Embed(ctx context.Context, texts []string, model string) ([][]float32, error)
}
```
- `ClientFactory` 返回的是 `ModelProvider` 而不是具体 go-openai client；
- 不同实现：
  - `OpenAIProvider`（当前 go-openai 实现）；
  - 将来可以有 `LangchaingoProvider`/`LocalAIProvider` 等，封装不同 SDK；
- `runtime.Agent` 只依赖该接口，完全解耦具体 SDK。

#### 3.2 Retriever / VectorStore Provider
- 在 `internal/rag` 中新增：
```go
type Retriever interface {
    Search(ctx context.Context, kbID string, query string, topK int, minScore float64) ([]*SearchResult, error)
}

type VectorStore interface {
    Upsert(ctx context.Context, kbID string, docs []Document) error
    Delete(ctx context.Context, kbID string, docIDs []string) error
}
```
- 当前 `RAGService` 既可以实现 Retriever，也可以聚合多个 VectorStore 实现；
- 将来如需用 Langchaingo 的 Retriever/VectorStore，只需实现这两个接口。

#### 3.3 Tool Provider
- 在 `internal/tools` 中整理接口：
```go
type ToolDefinitionProvider interface {
    GetDefinition(name string) (*ToolDefinition, bool)
    List() []*ToolDefinition
    ListByCategory(category string) []*ToolDefinition
}

type ToolExecutionProvider interface {
    Execute(ctx context.Context, req *ToolExecutionRequest) (*ToolExecutionResult, error)
}
```
- 现有 `ToolRegistry` / `ToolExecutor` 分别实现这两个接口；
- `ToolHelper` 只依赖接口类型，方便未来替换实现（例如外部插件系统）。

#### 3.4 配置与依赖注入
- 在 `backend/cmd/main.go` 或 `internal/infra` 初始化阶段：
  - 根据配置文件（`config/dev.yaml`/`prod.yaml`）选择 Provider 实现：
    - `model.provider: openai | langchaingo | localai`；
    - `rag.provider: pgvector | milvus | qdrant | langchaingo`；
  - 使用简单的工厂/容器组装 `Registry`、`RAGHelper`、`ToolHelper` 的依赖。

#### 3.5 验收要点
- 代码层：Agent/Workflow 模块对底层 SDK 完全无感，只依赖 Provider 接口；
- 实验性地新增一个“伪 Provider”（例如一个只回 echo 的模型），用于本地快速测试，证明 Provider 抽象易于替换；
- 未来如果要尝试 Langchaingo，只需要：
  - 写一个 `LangchaingoProvider` 实现 `ModelProvider`/`Retriever` 接口；
  - 在配置中切换 Provider，而不动核心业务逻辑。

---

## 四、整体推进顺序建议

1. **阶段 1：接口标准化（Provider + Memory 抽象）**  
   - 定义并落地 `ModelProvider` / `Retriever` / `ToolExecutionProvider` 接口；
   - 初步实现 `Memory` 抽象（FullBuffer + Window），不引入摘要。

2. **阶段 2：RAG 模式扩展（Map-Reduce）**  
   - 在 `RAGHelper` 中接入 `RAGStrategy`，实现 Stuff + Map-Reduce 两种模式；
   - 在 AgentConfig.ExtraConfig 中新增 `rag_mode` 等配置项。

3. **阶段 3：高级 Memory（Summary）+ 可选 Langchaingo Provider**  
   - 实现 SummaryMemory（调用模型生成摘要），并在高价值 Agent 上试点；
   - 如有需要，新增 Langchaingo-based Provider/Retriever 作为内部实验实现。

如果你确认这份 spec 的方向没问题，下一步我可以：
- 按你选定的阶段（例如先做“阶段 1：接口标准化 + WindowMemory”）进一步细化到“修改哪些文件、增加哪些接口和结构体、示例调用代码”的粒度，为后续实际编码做准备。