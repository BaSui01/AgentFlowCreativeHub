# 📋 Reviewer & Formatter Agent RAG 集成 + 完善需求分析文档

## 🎯 目标

1. **为 Reviewer Agent 添加 RAG 支持** - 基于知识库的审校标准和最佳实践
2. **为 Formatter Agent 添加 RAG 支持** - 基于知识库的格式化规范和模板
3. **生成完整的需求分析文档** - 涵盖整个项目的功能、架构、RAG 能力

---

## 📐 技术方案

### 方案一：Reviewer Agent RAG 集成

#### 改造内容

**文件**: `backend/internal/agent/runtime/reviewer_agent.go`

**改造点**:
1. 添加 `ragHelper *RAGHelper` 字段
2. 修改构造函数 `NewReviewerAgent()` 接受 `ragHelper` 参数
3. 在 `Execute()` 方法开始时调用 `ragHelper.EnrichWithKnowledge()`
4. 在 `ExecuteStream()` 方法开始时调用 RAG 增强
5. 在 `buildMessages()` 中使用 `InjectKnowledgeIntoPrompt()` 注入知识库上下文

**代码改动**:
```go
type ReviewerAgent struct {
    config      *AgentConfig
    modelClient ai.ModelClient
    ragHelper   *RAGHelper  // ← 新增
    name        string
}

func NewReviewerAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper) *ReviewerAgent {
    return &ReviewerAgent{
        config:      config,
        modelClient: modelClient,
        ragHelper:   ragHelper,  // ← 新增
        name:        config.Name,
    }
}

func (a *ReviewerAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
    start := time.Now()
    
    // RAG 增强：从知识库检索审校标准和案例
    if a.ragHelper != nil {
        enrichedInput, err := a.ragHelper.EnrichWithKnowledge(ctx, a.config.AgentConfig, input)
        if err == nil {
            input = enrichedInput
        }
    }
    
    // ... 原有逻辑
}

func (a *ReviewerAgent) buildMessages(input *AgentInput) []ai.Message {
    // ... 系统提示词构建
    
    // RAG 增强：注入审校标准和最佳实践
    systemPrompt = InjectKnowledgeIntoPrompt(input, systemPrompt)
    
    // ... 原有逻辑
}
```

#### RAG 应用场景

**知识库类型**: 审校规范库
- 企业写作规范
- 风格指南（如《芝加哥手册》）
- 常见错误案例
- 行业术语词典
- 合规要求文档

**检索示例**:
- 用户输入: "审校这篇技术文档"
- RAG 检索: "技术文档审校标准"、"技术写作规范"
- 上下文注入: [参考资料 1] 技术文档应使用主动语态... [参考资料 2] 避免模糊表述...

---

### 方案二：Formatter Agent RAG 集成

#### 改造内容

**文件**: `backend/internal/agent/runtime/formatter_agent.go`

**改造点**:
1. 添加 `ragHelper *RAGHelper` 字段
2. 修改构造函数 `NewFormatterAgent()` 接受 `ragHelper` 参数
3. 在 `Execute()` 方法开始时调用 RAG 增强
4. 在 `ExecuteStream()` 方法开始时调用 RAG 增强
5. 在 `buildMessages()` 中注入格式化规范和模板

**代码改动**:
```go
type FormatterAgent struct {
    config      *AgentConfig
    modelClient ai.ModelClient
    ragHelper   *RAGHelper  // ← 新增
    name        string
}

func NewFormatterAgent(config *AgentConfig, modelClient ai.ModelClient, ragHelper *RAGHelper) *FormatterAgent {
    return &FormatterAgent{
        config:      config,
        modelClient: modelClient,
        ragHelper:   ragHelper,  // ← 新增
        name:        config.Name,
    }
}

func (a *FormatterAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
    start := time.Now()
    
    // RAG 增强：从知识库检索格式化规范和模板
    if a.ragHelper != nil {
        enrichedInput, err := a.ragHelper.EnrichWithKnowledge(ctx, a.config.AgentConfig, input)
        if err == nil {
            input = enrichedInput
        }
    }
    
    // ... 原有逻辑
}

func (a *FormatterAgent) buildMessages(input *AgentInput) []ai.Message {
    // ... 系统提示词构建
    
    // RAG 增强：注入格式化规范和模板示例
    systemPrompt = InjectKnowledgeIntoPrompt(input, systemPrompt)
    
    // ... 原有逻辑
}
```

#### RAG 应用场景

**知识库类型**: 格式化规范库
- Markdown 格式规范
- 企业文档模板
- 排版标准（标题层级、段落间距等）
- 代码格式化规范
- 文档结构模板

**检索示例**:
- 用户输入: "格式化这篇文档为 Markdown"
- RAG 检索: "Markdown 格式规范"、"文档模板示例"
- 上下文注入: [参考资料 1] 一级标题使用 #... [参考资料 2] 代码块使用 ```...

---

### 方案三：Registry 已支持（无需修改）

**当前 Registry 实现**:
```go
switch config.AgentType {
case "writer":
    return NewWriterAgent(agentConfig, modelClient, r.ragHelper), nil
case "reviewer":
    return NewReviewerAgent(agentConfig, modelClient, r.ragHelper), nil  // ✅ 已支持
case "formatter":
    return NewFormatterAgent(agentConfig, modelClient, r.ragHelper), nil  // ✅ 已支持
}
```

**说明**: Registry 在创建 Agent 时已经传递了 `r.ragHelper`，无需额外修改。

---

## 📝 完善需求分析文档

### 文档结构

创建 **完整的需求规格说明书**: `docs/需求规格说明书-完整版.md`

#### 目录大纲

```markdown
# AgentFlowCreativeHub 需求规格说明书（完整版）

## 1. 项目概述
   1.1 项目背景
   1.2 项目目标
   1.3 项目范围
   1.4 术语定义

## 2. 整体架构
   2.1 系统架构图
   2.2 技术栈
   2.3 模块划分
   2.4 部署架构

## 3. 核心功能模块

### 3.1 多 Agent 协作系统
   - Agent 类型定义（Writer/Reviewer/Formatter/Planner/Translator/Analyzer/Researcher）
   - Agent 能力矩阵
   - Agent 编排模式

### 3.2 AI 模型管理
   - 多提供商支持（OpenAI/Azure/Google/AWS/Anthropic/DeepSeek/Qwen/Ollama）
   - 模型配置管理
   - 模型发现与路由

### 3.3 RAG 知识库系统 ⭐
   - 知识库管理（CRUD）
   - 文档处理流程（上传→解析→分块→向量化）
   - 语义检索（pgvector + 余弦相似度）
   - Agent RAG 集成（Writer/Reviewer/Formatter）

### 3.4 Prompt 模板管理
   - 模板 CRUD
   - 变量注入
   - 版本管理

### 3.5 工作流编排
   - 工作流定义（YAML/JSON）
   - 任务调度
   - 状态管理

### 3.6 认证授权系统
   - JWT + Session
   - OAuth2 集成
   - 多租户隔离
   - RBAC 权限控制

### 3.7 审计日志系统
   - 40+ 审计事件类型
   - 日志查询与过滤
   - 审计报告

## 4. 数据模型
   4.1 核心实体关系图
   4.2 数据库表设计
   4.3 pgvector 向量存储设计

## 5. API 设计
   5.1 API 端点列表（63+ 端点）
   5.2 认证方式
   5.3 请求/响应格式
   5.4 错误码定义

## 6. RAG 知识库详细设计 ⭐

### 6.1 功能概述
   - 知识库管理
   - 文档处理
   - 语义检索
   - Agent 集成

### 6.2 技术架构
   - pgvector 向量数据库
   - 文档解析器（TXT/Markdown/HTML）
   - 递归分块算法
   - HNSW 向量索引

### 6.3 API 端点（13 个）
   - 知识库 CRUD（5 个）
   - 文档管理（6 个）
   - 语义检索（2 个）

### 6.4 Agent RAG 集成
   - Writer Agent RAG 支持 ✅
   - Reviewer Agent RAG 支持 ⏳
   - Formatter Agent RAG 支持 ⏳

### 6.5 使用场景
   - 技术文档问答
   - 企业知识库
   - 合规审查
   - 代码助手

### 6.6 配置参数
   - knowledge_base_id: 知识库 ID
   - rag_enabled: 启用开关
   - rag_top_k: 检索数量（默认 3）
   - rag_min_score: 最小相似度（默认 0.7）

## 7. 非功能性需求
   7.1 性能要求
   7.2 可扩展性
   7.3 安全性
   7.4 可观测性

## 8. 部署与运维
   8.1 环境要求
   8.2 部署步骤
   8.3 监控指标
   8.4 故障处理

## 9. 测试策略
   9.1 单元测试
   9.2 集成测试
   9.3 性能测试
   9.4 RAG 测试用例

## 10. 里程碑与路线图
   - Sprint 1-3: 基础设施 ✅
   - Sprint 4: 多提供商支持 ✅
   - Sprint 5: 认证授权 ✅
   - Sprint 6: RAG 知识库 ✅
   - Sprint 7: Agent RAG 全面集成 ⏳
   - Sprint 8+: 工具调用、监控、前端

## 11. 附录
   - A. 数据库 Schema
   - B. API 完整列表
   - C. 配置示例
   - D. RAG 检索示例
```

---

## 🔄 实施步骤

### 步骤 1: Reviewer Agent RAG 集成 (30 分钟)
1. 修改 `reviewer_agent.go` 添加 `ragHelper` 字段
2. 改造构造函数和执行方法
3. 在 `buildMessages()` 中注入知识库上下文

### 步骤 2: Formatter Agent RAG 集成 (30 分钟)
1. 修改 `formatter_agent.go` 添加 `ragHelper` 字段
2. 改造构造函数和执行方法
3. 在 `buildMessages()` 中注入知识库上下文

### 步骤 3: 生成完整需求规格说明书 (1 小时)
1. 创建 `docs/需求规格说明书-完整版.md`
2. 整合所有 Sprint 的功能说明
3. 重点补充 RAG 系统的详细设计
4. 添加使用场景和配置示例
5. 添加 API 端点完整列表
6. 添加数据模型和关系图

### 步骤 4: 生成 RAG 集成最终报告 (30 分钟)
1. 更新 `AGENT_RAG_INTEGRATION_REPORT.md`
2. 添加 Reviewer 和 Formatter 的集成说明
3. 更新统计数据和完成度

---

## 📊 预期成果

### 代码变更
- **修改文件**: 2 个
  - `reviewer_agent.go` (+25 行)
  - `formatter_agent.go` (+25 行)
- **新增代码**: ~50 行

### 文档交付
1. **`docs/需求规格说明书-完整版.md`** (~5000 行)
   - 涵盖整个项目的功能、架构、API
   - RAG 系统完整设计说明
   - 使用场景和配置指南

2. **更新 `AGENT_RAG_INTEGRATION_REPORT.md`**
   - 三种 Agent 类型的 RAG 支持说明
   - 完整的使用指南和场景示例

### 功能完成度
- ✅ Writer Agent RAG 支持（已完成）
- ✅ Reviewer Agent RAG 支持（本次完成）
- ✅ Formatter Agent RAG 支持（本次完成）
- ✅ **所有主要 Agent 类型支持 RAG** 🎉

---

## 🎯 验收标准

### 代码层面
1. ✅ Reviewer Agent 构造函数接受 `ragHelper` 参数
2. ✅ Reviewer Agent 执行时调用 RAG 增强
3. ✅ Formatter Agent 构造函数接受 `ragHelper` 参数
4. ✅ Formatter Agent 执行时调用 RAG 增强
5. ✅ 两种 Agent 的 `buildMessages()` 都注入知识库上下文
6. ✅ 代码风格与 Writer Agent 保持一致

### 文档层面
1. ✅ 需求规格说明书包含所有功能模块
2. ✅ RAG 系统有独立章节详细说明
3. ✅ 包含完整的 API 端点列表（63+）
4. ✅ 包含数据模型和关系图
5. ✅ 包含使用场景和配置示例
6. ✅ 包含所有 Sprint 的实施总结

### 功能层面
1. ✅ 可以为 Reviewer Agent 配置知识库
2. ✅ Reviewer Agent 执行时自动检索审校标准
3. ✅ 可以为 Formatter Agent 配置知识库
4. ✅ Formatter Agent 执行时自动检索格式规范
5. ✅ RAG 失败不影响 Agent 正常执行（降级）

---

## 💡 核心价值

### 对 Reviewer Agent
- ✅ **基于企业标准的审校** - 自动参考企业写作规范
- ✅ **专业术语检查** - 基于行业词典审校
- ✅ **合规性检查** - 基于合规文档检查内容

### 对 Formatter Agent
- ✅ **统一格式标准** - 基于企业模板格式化
- ✅ **智能排版** - 参考最佳实践优化排版
- ✅ **模板化输出** - 基于知识库模板生成标准格式

### 对整体项目
- ✅ **完整的需求文档** - 便于团队理解和维护
- ✅ **标准化文档结构** - 符合软件工程规范
- ✅ **可追溯性** - 需求到实现的完整链路

---

## 📝 时间估算

- **Reviewer Agent RAG 集成**: 30 分钟
- **Formatter Agent RAG 集成**: 30 分钟
- **需求规格说明书编写**: 1 小时
- **报告更新和验证**: 30 分钟

**总计**: 约 2.5 小时

---

准备好开始实施了吗？确认后我将：
1. 修改 Reviewer Agent 和 Formatter Agent 添加 RAG 支持
2. 生成完整的需求规格说明书（~5000 行）
3. 更新 RAG 集成报告
4. 验证所有 Agent 类型的 RAG 功能