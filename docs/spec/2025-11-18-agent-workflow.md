# 🔧 工作流模板引擎 + Agent-Workflow集成 + 上下文隔离与共享完整方案

## 一、核心问题分析

### 1.1 当前架构问题

**上下文管理混乱**:
- ❌ `ExecutionContext` (工作流) 和 `AgentContext` (Agent) 分离,无映射
- ❌ Agent无法访问工作流的共享数据 (`ExecutionContext.Data`)
- ❌ 缺少Agent本地上下文隔离机制
- ❌ 没有明确的上下文作用域 (isolated/shared)

**模板引擎缺失**:
- ❌ `resolveVariables` 仅有简化实现 (TODO标记)
- ❌ 无法处理复杂变量引用 `{{step1.output.title}}`
- ❌ 不支持模板函数 `{{upper .content}}`
- ❌ 不支持条件渲染 `{{if .success}}`

**Agent-Workflow集成不完整**:
- ❌ 缺少 `AgentTaskExecutor` 实现
- ❌ Task和AgentInput转换不清晰
- ❌ Agent错误处理和重试未集成

---

## 二、上下文架构设计 🎯

### 2.1 三层上下文架构

```
┌─────────────────────────────────────────────────────────────┐
│             GlobalContext (全局,整个工作流)                    │
│  ┌────────────────────────────────────────────────────┐     │
│  │ TenantID       : "tenant-123"   (不可变)            │     │
│  │ UserID         : "user-456"     (不可变)            │     │
│  │ WorkflowID     : "wf-789"       (不可变)            │     │
│  │ ExecutionID    : "exec-abc"     (不可变)            │     │
│  │ TraceID        : "trace-xyz"    (不可变)            │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│            SharedContext (共享,所有Agent可读写)                │
│  ┌────────────────────────────────────────────────────┐     │
│  │ Data: map[string]any                               │     │
│  │   - "step1.output"    : "生成的文章内容"             │     │
│  │   - "step2.output"    : "审核结果:通过"              │     │
│  │   - "shared.topic"    : "AI技术"                    │     │
│  │   - "shared.metadata" : {...}                       │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│         LocalContext (隔离,仅当前Agent可见)                   │
│  ┌────────────────────────────────────────────────────┐     │
│  │ StepID         : "step-writer-1"                    │     │
│  │ AgentType      : "writer"                           │     │
│  │ InputData      : 从上游Agent接收的数据                │     │
│  │ LocalVars      : Agent临时变量                        │     │
│  │ History        : 对话历史(仅限本Agent)                 │     │
│  │ SessionID      : "session-123" (可选)               │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 上下文隔离策略

**原则**: **默认隔离,显式共享**

| 数据类型 | 作用域 | 可见性 | 生命周期 |
|---------|--------|--------|---------|
| **全局信息** | Global | 所有Agent只读 | 整个工作流 |
| **共享数据** | Shared | 所有Agent读写 | 整个工作流 |
| **本地数据** | Local | 仅当前Agent | 单个Agent执行 |
| **对话历史** | Session | 同SessionID的Agent | 会话生命周期 |

**隔离机制**: Copy-on-Write
- Agent读取Shared数据时,获取只读引用
- Agent写入Shared数据时,显式调用 `SetShared(key, value)`
- Agent本地变量自动隔离,不污染SharedContext

### 2.3 数据传递模式

#### 模式1: 显式变量引用 (推荐)
```yaml
steps:
  - id: writer
    agent_type: writer
    output: article  # 输出保存到 shared.article

  - id: reviewer
    agent_type: reviewer
    input:
      content: "{{writer.output}}"  # 引用上游输出
```

#### 模式2: 共享命名空间
```yaml
steps:
  - id: writer
    agent_type: writer
    shared:
      topic: "AI技术"  # 写入 shared.topic
  
  - id: reviewer
    agent_type: reviewer
    # 自动读取 shared.topic
```

#### 模式3: 会话历史 (多轮对话)
```yaml
steps:
  - id: chat1
    agent_type: writer
    session_id: "conv-123"  # 共享会话

  - id: chat2
    agent_type: reviewer
    session_id: "conv-123"  # 继续对话
```

---

## 三、工作流模板引擎实现 📝

### 3.1 设计目标

- ✅ 支持变量引用: `{{step1.output}}`, `{{shared.topic}}`
- ✅ 支持嵌套访问: `{{step1.output.title}}`
- ✅ 支持模板函数: `{{upper .content}}`, `{{trim .text}}`
- ✅ 支持条件渲染: `{{if .success}}成功{{else}}失败{{end}}`
- ✅ 支持循环: `{{range .items}}...{{end}}`
- ✅ 线程安全: 支持并发渲染

### 3.2 实现方案

**技术选型**: Go标准库 `text/template`

**核心组件**:
```go
// TemplateEngine 模板引擎
type TemplateEngine struct {
    funcMap template.FuncMap
    cache   map[string]*template.Template
    mu      sync.RWMutex
}

// 内置函数
var DefaultFuncMap = template.FuncMap{
    "upper":    strings.ToUpper,
    "lower":    strings.ToLower,
    "trim":     strings.TrimSpace,
    "json":     toJSON,
    "default":  defaultValue,
    "join":     strings.Join,
    "split":    strings.Split,
}
```

### 3.3 模板渲染流程

```
Input配置 (含模板)
        ↓
提取模板变量 ({{...}})
        ↓
准备数据上下文 (GlobalContext + SharedContext)
        ↓
渲染模板 (text/template)
        ↓
解析为JSON/字符串
        ↓
传递给Agent
```

### 3.4 代码实现

```go
// backend/internal/workflow/executor/template.go

package executor

import (
    "bytes"
    "encoding/json"
    "fmt"
    "strings"
    "sync"
    "text/template"
)

// TemplateEngine 工作流模板引擎
type TemplateEngine struct {
    funcMap template.FuncMap
    cache   map[string]*template.Template
    mu      sync.RWMutex
}

// NewTemplateEngine 创建模板引擎
func NewTemplateEngine() *TemplateEngine {
    return &TemplateEngine{
        funcMap: DefaultFuncMap(),
        cache:   make(map[string]*template.Template),
    }
}

// DefaultFuncMap 默认函数映射
func DefaultFuncMap() template.FuncMap {
    return template.FuncMap{
        // 字符串函数
        "upper":   strings.ToUpper,
        "lower":   strings.ToLower,
        "trim":    strings.TrimSpace,
        "title":   strings.Title,
        
        // JSON函数
        "json":    toJSON,
        
        // 默认值
        "default": defaultValue,
        
        // 数组/切片函数
        "join":    strings.Join,
        "first":   first,
        "last":    last,
    }
}

// Render 渲染模板
// tmplStr: 模板字符串 (如 "写一篇关于{{.topic}}的文章")
// data: 数据上下文 (ExecutionContext.Data)
func (e *TemplateEngine) Render(tmplStr string, data map[string]any) (string, error) {
    if tmplStr == "" {
        return "", nil
    }

    // 检查是否包含模板语法
    if !strings.Contains(tmplStr, "{{") {
        return tmplStr, nil // 普通字符串,直接返回
    }

    // 解析模板
    tmpl, err := e.parseTemplate(tmplStr)
    if err != nil {
        return "", fmt.Errorf("解析模板失败: %w", err)
    }

    // 执行渲染
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", fmt.Errorf("渲染模板失败: %w", err)
    }

    return buf.String(), nil
}

// RenderMap 渲染Map中的所有模板字段
func (e *TemplateEngine) RenderMap(inputMap map[string]any, data map[string]any) (map[string]any, error) {
    result := make(map[string]any)
    
    for key, value := range inputMap {
        switch v := value.(type) {
        case string:
            // 渲染字符串模板
            rendered, err := e.Render(v, data)
            if err != nil {
                return nil, fmt.Errorf("渲染字段 %s 失败: %w", key, err)
            }
            result[key] = rendered
            
        case map[string]any:
            // 递归渲染嵌套Map
            rendered, err := e.RenderMap(v, data)
            if err != nil {
                return nil, err
            }
            result[key] = rendered
            
        default:
            // 非字符串直接复制
            result[key] = value
        }
    }
    
    return result, nil
}

// parseTemplate 解析模板(带缓存)
func (e *TemplateEngine) parseTemplate(tmplStr string) (*template.Template, error) {
    // 生成缓存键(使用模板字符串的哈希)
    cacheKey := tmplStr
    
    // 检查缓存
    e.mu.RLock()
    if tmpl, ok := e.cache[cacheKey]; ok {
        e.mu.RUnlock()
        return tmpl, nil
    }
    e.mu.RUnlock()
    
    // 解析模板
    tmpl, err := template.New("workflow").Funcs(e.funcMap).Parse(tmplStr)
    if err != nil {
        return nil, err
    }
    
    // 存入缓存
    e.mu.Lock()
    e.cache[cacheKey] = tmpl
    e.mu.Unlock()
    
    return tmpl, nil
}

// 辅助函数

func toJSON(v any) string {
    data, _ := json.Marshal(v)
    return string(data)
}

func defaultValue(defaultVal, val any) any {
    if val == nil || val == "" {
        return defaultVal
    }
    return val
}

func first(arr []any) any {
    if len(arr) == 0 {
        return nil
    }
    return arr[0]
}

func last(arr []any) any {
    if len(arr) == 0 {
        return nil
    }
    return arr[len(arr)-1]
}
```

### 3.5 集成到Scheduler

```go
// 修改 scheduler.go 中的 executeTask 方法

func (s *Scheduler) executeTask(
    ctx context.Context,
    nodeID string,
    execCtx *ExecutionContext,
    prevResults map[string]*TaskResult,
) (*TaskResult, error) {
    node := s.dag.Nodes[nodeID]
    
    // 渲染输入模板 (使用TemplateEngine)
    renderedInput, err := s.templateEngine.RenderMap(node.Step.Input, execCtx.Data)
    if err != nil {
        return &TaskResult{
            ID:     nodeID,
            Status: "failed",
            Error:  fmt.Errorf("模板渲染失败: %w", err),
        }, err
    }
    
    // 创建任务
    task := &Task{
        ID:      nodeID,
        Step:    node.Step,
        Input:   renderedInput,
        Context: execCtx,
    }
    
    // 执行任务
    return s.executor.ExecuteTask(ctx, task)
}
```

---

## 四、Agent-Workflow集成实现 🔗

### 4.1 AgentTaskExecutor设计

**职责**:
1. 将 `Task` 转换为 `AgentInput`
2. 从Registry获取Agent实例
3. 调用Agent执行
4. 将 `AgentResult` 转换为 `TaskResult`
5. 处理错误和重试

### 4.2 上下文映射机制

```
ExecutionContext          →         AgentContext
├── WorkflowID                     ├── WorkflowID
├── ExecutionID                    ├── TraceID
├── TenantID                       ├── TenantID
├── UserID                         ├── UserID
└── Data (SharedContext)           └── Data (只读)

            +
            
Task.Input (LocalContext)    →    AgentInput
├── Step.Input                    ├── Content
├── Variables                     ├── Variables
└── RenderedTemplates            └── ExtraParams
```

### 4.3 代码实现

```go
// backend/internal/workflow/executor/agent_executor.go

package executor

import (
    "context"
    "fmt"
    "time"
    
    "backend/internal/agent/runtime"
)

// AgentTaskExecutor Agent任务执行器
// 负责将工作流任务委托给Agent执行
type AgentTaskExecutor struct {
    agentRegistry *runtime.Registry
}

// NewAgentTaskExecutor 创建Agent任务执行器
func NewAgentTaskExecutor(registry *runtime.Registry) *AgentTaskExecutor {
    return &AgentTaskExecutor{
        agentRegistry: registry,
    }
}

// ExecuteTask 执行任务
func (e *AgentTaskExecutor) ExecuteTask(ctx context.Context, task *Task) (*TaskResult, error) {
    start := time.Now()
    
    // 1. 获取Agent实例
    agent, err := e.getAgent(ctx, task)
    if err != nil {
        return &TaskResult{
            ID:     task.ID,
            Status: "failed",
            Error:  fmt.Errorf("获取Agent失败: %w", err),
        }, err
    }
    
    // 2. 构建AgentInput
    agentInput := e.buildAgentInput(task)
    
    // 3. 执行Agent
    result, err := agent.Execute(ctx, agentInput)
    
    latency := time.Since(start)
    
    // 4. 转换结果
    if err != nil {
        return &TaskResult{
            ID:     task.ID,
            Status: "failed",
            Error:  err,
            Metadata: map[string]any{
                "latency_ms": latency.Milliseconds(),
                "agent_type": agent.Type(),
            },
        }, err
    }
    
    // 5. 构建TaskResult
    return &TaskResult{
        ID:     task.ID,
        Output: result.Output,
        Status: "success",
        Metadata: map[string]any{
            "latency_ms": latency.Milliseconds(),
            "agent_type": agent.Type(),
            "usage":      result.Usage,
            "cost":       result.Cost,
        },
    }, nil
}

// getAgent 获取Agent实例
func (e *AgentTaskExecutor) getAgent(ctx context.Context, task *Task) (runtime.Agent, error) {
    // 优先使用AgentID
    if task.Step.AgentID != nil {
        return e.agentRegistry.GetAgent(ctx, task.Context.TenantID, *task.Step.AgentID)
    }
    
    // 否则使用AgentType
    if task.Step.AgentType != "" {
        return e.agentRegistry.GetAgentByType(ctx, task.Context.TenantID, task.Step.AgentType)
    }
    
    return nil, fmt.Errorf("缺少agent_id或agent_type")
}

// buildAgentInput 构建Agent输入
func (e *AgentTaskExecutor) buildAgentInput(task *Task) *runtime.AgentInput {
    // 提取content字段(主要输入)
    content := ""
    if contentVal, ok := task.Input["content"]; ok {
        content, _ = contentVal.(string)
    }
    
    // 构建AgentContext (映射ExecutionContext)
    agentCtx := &runtime.AgentContext{
        TenantID:   task.Context.TenantID,
        UserID:     task.Context.UserID,
        WorkflowID: &task.Context.WorkflowID,
        TraceID:    &task.Context.ExecutionID,
        StepID:     &task.Step.ID,
        Data:       e.buildSharedData(task.Context), // 只读快照
    }
    
    // 提取SessionID (如果有)
    if sessionID, ok := task.Input["session_id"].(string); ok {
        agentCtx.SessionID = &sessionID
    }
    
    return &runtime.AgentInput{
        Content:     content,
        Variables:   task.Input,
        Context:     agentCtx,
        ExtraParams: task.Step.ExtraConfig,
    }
}

// buildSharedData 构建共享数据快照(只读)
// 避免Agent直接修改工作流上下文
func (e *AgentTaskExecutor) buildSharedData(execCtx *ExecutionContext) map[string]any {
    // 获取只读快照
    execCtx.mu.RLock()
    defer execCtx.mu.RUnlock()
    
    // 深拷贝(简化实现,仅拷贝一层)
    snapshot := make(map[string]any, len(execCtx.Data))
    for k, v := range execCtx.Data {
        snapshot[k] = v
    }
    
    return snapshot
}
```

### 4.4 重试和错误处理

```go
// ExecuteTaskWithRetry 支持重试的任务执行
func (e *AgentTaskExecutor) ExecuteTaskWithRetry(ctx context.Context, task *Task) (*TaskResult, error) {
    retryConfig := task.Step.Retry
    if retryConfig == nil {
        // 无重试配置,直接执行
        return e.ExecuteTask(ctx, task)
    }
    
    maxRetries := retryConfig.MaxRetries
    if maxRetries <= 0 {
        maxRetries = 3 // 默认重试3次
    }
    
    var lastErr error
    for attempt := 0; attempt <= maxRetries; attempt++ {
        if attempt > 0 {
            // 计算退避延迟
            delay := e.calculateBackoff(retryConfig, attempt)
            time.Sleep(delay)
        }
        
        result, err := e.ExecuteTask(ctx, task)
        if err == nil {
            return result, nil
        }
        
        lastErr = err
    }
    
    return &TaskResult{
        ID:     task.ID,
        Status: "failed",
        Error:  fmt.Errorf("重试%d次后仍失败: %w", maxRetries, lastErr),
    }, lastErr
}

// calculateBackoff 计算退避延迟
func (e *AgentTaskExecutor) calculateBackoff(retry *RetryConfig, attempt int) time.Duration {
    baseDelay := time.Duration(retry.Delay) * time.Second
    
    switch retry.Backoff {
    case "exponential":
        // 指数退避: delay * 2^attempt
        return baseDelay * time.Duration(1<<uint(attempt))
    default:
        // 固定延迟
        return baseDelay
    }
}
```

---

## 五、完整上下文管理器 🎛️

### 5.1 增强ExecutionContext

```go
// backend/internal/workflow/executor/context.go

package executor

import (
    "sync"
)

// ExecutionContext 工作流执行上下文(增强版)
type ExecutionContext struct {
    // === 全局信息 (不可变) ===
    WorkflowID  string
    ExecutionID string
    TenantID    string
    UserID      string
    TraceID     string
    
    // === 共享数据 (可读写) ===
    Data map[string]any // 步骤间共享数据
    
    // === 元数据 ===
    Metadata map[string]any
    
    // === 并发控制 ===
    mu sync.RWMutex
}

// NewExecutionContext 创建执行上下文
func NewExecutionContext(workflowID, executionID, tenantID, userID string) *ExecutionContext {
    return &ExecutionContext{
        WorkflowID:  workflowID,
        ExecutionID: executionID,
        TenantID:    tenantID,
        UserID:      userID,
        TraceID:     executionID, // 默认使用ExecutionID作为TraceID
        Data:        make(map[string]any),
        Metadata:    make(map[string]any),
    }
}

// SetShared 设置共享数据(显式共享)
func (ec *ExecutionContext) SetShared(key string, value any) {
    ec.mu.Lock()
    defer ec.mu.Unlock()
    ec.Data[key] = value
}

// GetShared 获取共享数据(只读)
func (ec *ExecutionContext) GetShared(key string) (any, bool) {
    ec.mu.RLock()
    defer ec.mu.RUnlock()
    val, ok := ec.Data[key]
    return val, ok
}

// SetStepOutput 设置步骤输出(便捷方法)
func (ec *ExecutionContext) SetStepOutput(stepID string, output any) {
    key := stepID + ".output"
    ec.SetShared(key, output)
}

// GetStepOutput 获取步骤输出
func (ec *ExecutionContext) GetStepOutput(stepID string) (any, bool) {
    key := stepID + ".output"
    return ec.GetShared(key)
}

// GetAllData 获取所有共享数据的只读快照
func (ec *ExecutionContext) GetAllData() map[string]any {
    ec.mu.RLock()
    defer ec.mu.RUnlock()
    
    // 浅拷贝
    snapshot := make(map[string]any, len(ec.Data))
    for k, v := range ec.Data {
        snapshot[k] = v
    }
    return snapshot
}

// ToAgentContext 转换为AgentContext
func (ec *ExecutionContext) ToAgentContext(stepID string) *runtime.AgentContext {
    return &runtime.AgentContext{
        TenantID:   ec.TenantID,
        UserID:     ec.UserID,
        WorkflowID: &ec.WorkflowID,
        TraceID:    &ec.TraceID,
        StepID:     &stepID,
        Data:       ec.GetAllData(), // 只读快照
    }
}
```

---

## 六、使用示例 📖

### 6.1 工作流定义 (YAML)

```yaml
name: content_creation_workflow
description: 内容创作工作流
version: "1.0"

steps:
  # Step 1: Writer Agent - 创作内容
  - id: writer
    name: 内容创作
    type: agent
    agent_type: writer
    input:
      content: "写一篇关于{{.topic}}的技术文章"
      topic: "{{.input.topic}}"  # 从工作流输入获取
      style: "专业"
    output: article  # 保存到 shared["writer.output"]
    
  # Step 2: Reviewer Agent - 审核内容
  - id: reviewer
    name: 内容审核
    type: agent
    agent_type: reviewer
    depends_on: [writer]  # 依赖writer步骤
    input:
      content: "{{writer.output}}"  # 引用上游输出
      criteria: "检查语法、逻辑、专业性"
    output: review_result
    
  # Step 3: Formatter Agent - 格式化
  - id: formatter
    name: 内容格式化
    type: agent
    agent_type: formatter
    depends_on: [writer, reviewer]
    input:
      content: "{{writer.output}}"
      format: "markdown"
      metadata:
        title: "{{.input.topic}}"
        review: "{{reviewer.output}}"
    output: formatted_article
    
  # Step 4: 条件步骤 - 仅在审核通过时发布
  - id: publisher
    name: 发布文章
    type: agent
    agent_type: publisher
    depends_on: [formatter]
    condition:
      expression: "{{if eq reviewer.output.status 'approved'}}true{{else}}false{{end}}"
    input:
      article: "{{formatter.output}}"
      channel: "blog"
```

### 6.2 执行代码

```go
// main.go

package main

import (
    "context"
    "fmt"
    
    "backend/internal/workflow/executor"
    "backend/internal/agent/runtime"
)

func main() {
    // 1. 创建组件
    agentRegistry := runtime.NewRegistry(db, clientFactory)
    agentExecutor := executor.NewAgentTaskExecutor(agentRegistry)
    templateEngine := executor.NewTemplateEngine()
    
    // 2. 解析工作流
    parser := executor.NewParser()
    definition, _ := parser.ParseYAML(workflowYAML)
    dag, _ := parser.BuildDAG(definition)
    
    // 3. 创建调度器
    scheduler := executor.NewScheduler(dag, agentExecutor, 5)
    scheduler.SetTemplateEngine(templateEngine)
    
    // 4. 创建执行上下文
    execCtx := executor.NewExecutionContext(
        "wf-123",      // WorkflowID
        "exec-456",    // ExecutionID
        "tenant-789",  // TenantID
        "user-abc",    // UserID
    )
    
    // 设置输入
    execCtx.SetShared("input", map[string]any{
        "topic": "AI大语言模型",
    })
    
    // 5. 执行工作流
    results, err := scheduler.Schedule(context.Background(), execCtx)
    if err != nil {
        panic(err)
    }
    
    // 6. 获取最终结果
    if result, ok := results["formatter"]; ok {
        fmt.Println("最终文章:", result.Output)
    }
}
```

### 6.3 Agent访问共享数据

```go
// Agent内部实现示例

func (a *ReviewerAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
    // 1. 访问共享数据 (只读)
    if input.Context != nil && input.Context.Data != nil {
        // 读取上游步骤输出
        if writerOutput, ok := input.Context.Data["writer.output"]; ok {
            fmt.Println("Writer的输出:", writerOutput)
        }
        
        // 读取全局共享变量
        if topic, ok := input.Context.Data["shared.topic"]; ok {
            fmt.Println("主题:", topic)
        }
    }
    
    // 2. 使用本地变量 (隔离)
    localVars := map[string]any{
        "temp_result": "审核中...",
    }
    
    // 3. 执行审核逻辑
    review := a.reviewContent(input.Content)
    
    // 4. 返回结果 (自动写入shared["reviewer.output"])
    return &AgentResult{
        Output: review,
        Status: "success",
    }, nil
}
```

---

## 七、隔离与共享对比表 📊

| 场景 | 实现方式 | 隔离级别 | 性能 | 适用场景 |
|------|---------|---------|------|---------|
| **Agent本地变量** | 栈上分配 | 完全隔离 | 最高 | 临时计算、中间结果 |
| **步骤输出** | `SetStepOutput(id, output)` | 命名空间隔离 | 高 | Agent间数据传递 |
| **共享变量** | `SetShared(key, value)` | 显式共享 | 中 | 全局配置、元数据 |
| **会话历史** | `ContextManager.Session` | SessionID隔离 | 中 | 多轮对话 |
| **RAG知识库** | 向量检索 | 租户隔离 | 低 | 知识增强 |

---

## 八、实施计划 📅

### 阶段1: 模板引擎 (2天)

**Day 1: 核心实现**
- [ ] 创建 `template.go`
- [ ] 实现 `TemplateEngine` 结构
- [ ] 实现 `Render` 方法
- [ ] 实现内置函数
- [ ] 单元测试

**Day 2: 集成**
- [ ] 修改 `scheduler.go` 集成模板引擎
- [ ] 实现 `RenderMap` 批量渲染
- [ ] 集成测试
- [ ] 文档编写

### 阶段2: Agent-Workflow集成 (2天)

**Day 3: 核心实现**
- [ ] 创建 `agent_executor.go`
- [ ] 实现 `AgentTaskExecutor`
- [ ] 实现上下文映射
- [ ] 实现错误处理

**Day 4: 增强功能**
- [ ] 实现重试机制
- [ ] 增强 `ExecutionContext`
- [ ] 集成到调度器
- [ ] 端到端测试

### 验收标准

**模板引擎**:
- ✅ 支持 `{{step1.output}}` 变量引用
- ✅ 支持 `{{upper .text}}` 函数调用
- ✅ 支持 `{{if .condition}}` 条件渲染
- ✅ 并发渲染无竞态

**Agent集成**:
- ✅ 工作流可调用Agent
- ✅ Agent可访问共享数据(只读)
- ✅ Agent输出正确回写
- ✅ 错误重试机制工作

**上下文隔离**:
- ✅ Agent本地变量不污染SharedContext
- ✅ Agent只能读取SharedContext
- ✅ 并发Agent执行无冲突

---

## 九、性能优化建议 ⚡

### 9.1 模板缓存
- 已解析的模板缓存复用
- 使用LRU淘汰策略
- 支持模板预热

### 9.2 上下文快照
- 使用Copy-on-Write减少拷贝
- 考虑使用immutable数据结构
- 大对象使用指针传递

### 9.3 并发控制
- Agent执行使用goroutine池
- 限制最大并发数(默认5)
- 超时自动取消

---

## 十、后续增强 🚀

### 短期 (1-2周)
- [ ] 支持复杂条件表达式 (cel-go)
- [ ] 支持循环步骤 `{{range .items}}`
- [ ] Agent输出Schema验证
- [ ] 上下文持久化(Redis)

### 中期 (1-2月)
- [ ] 可视化工作流编排器
- [ ] 工作流版本管理
- [ ] 动态工作流(运行时修改)
- [ ] 工作流A/B测试

### 长期 (3-6月)
- [ ] 分布式工作流调度
- [ ] 工作流断点续传
- [ ] 工作流回滚机制
- [ ] AI自动优化工作流

---

## 十一、总结

### 核心设计原则

1. **默认隔离,显式共享** - 避免上下文污染
2. **Copy-on-Write** - 保证并发安全
3. **分层架构** - Global/Shared/Local三层
4. **模板驱动** - 声明式配置,灵活组合
5. **接口抽象** - 易于扩展和测试

### 预期收益

- ✅ Agent上下文完全隔离,无相互干扰
- ✅ 工作流模板灵活,支持复杂场景
- ✅ Agent-Workflow无缝集成
- ✅ 并发执行安全可靠
- ✅ 代码清晰,易于维护

### 工作量评估

- **模板引擎**: 2天 (约500行代码)
- **Agent集成**: 2天 (约400行代码)
- **测试**: 包含在内
- **文档**: 本方案已涵盖

**总计**: 4天完成P0任务2和任务3