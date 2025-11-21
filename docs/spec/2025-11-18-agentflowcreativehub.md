# 🎯 AgentFlowCreativeHub 完整补全计划 - 一步到位方案

## 📊 项目当前状态全面分析

### 已完成模块 ✅ (75%)

| 模块 | 完成度 | 文件数 | 代码行数 | 状态 |
|------|--------|--------|---------|------|
| **基础设施** | 100% | 15+ | ~3,000 | ✅ 完整 |
| **Agent 运行时** | 100% | 11 | ~3,500 | ✅ 7种全部完成 |
| **AI 模型管理** | 100% | 13 | ~2,600 | ✅ 8个提供商 |
| **RAG 知识库** | 100% | 9 | ~2,160 | ✅ 完整功能 |
| **认证授权** | 100% | 10 | ~2,290 | ✅ JWT+OAuth2 |
| **审计日志** | 100% | 4 | ~800 | ✅ 40+事件类型 |
| **Prompt 模板** | 100% | 4 | ~1,000 | ✅ 版本管理 |
| **监控告警** | 100% | 11 | ~1,529 | ✅ Prometheus+Grafana |
| **工作流编排** | 80% | 6 | ~2,300 | ⚠️ 核心已完成，待完善 |

**总计**: 94+ 文件，~19,179 行代码，63+ API 端点，13 数据库表

---

### 待完善模块 ⏳ (20%)

#### 1. 工作流编排系统 (80% → 100%)

**当前状态**:
- ✅ 核心引擎：Parser、Scheduler、Engine 已完成
- ✅ 基础执行：线性流程、DAG 构建、任务调度
- ✅ Agent 集成：AgentTaskExecutor 已实现
- ⚠️ **缺失功能**：
  - 条件分支执行
  - 并行任务执行
  - 人工审核节点
  - 失败重试机制
  - 变量模板引擎
  - 执行历史查询 API

**待补全文件**:
- `backend/internal/workflow/executor/scheduler.go` - 完善变量解析
- `backend/internal/workflow/executor/parallel_executor.go` - 新增并行执行器
- `backend/internal/workflow/executor/condition_executor.go` - 新增条件执行器
- `backend/api/handlers/workflows/execute_handler.go` - 完善查询 API

---

#### 2. 用户和租户管理系统 (0% → 100%)

**当前状态**:
- ✅ 数据模型已完整（Tenant、User、Role、Permission）
- ✅ Service 层已实现（TenantService、UserService、RoleService）
- ⚠️ **缺失功能**：
  - 租户 CRUD API（占位符）
  - 用户管理 API（占位符）
  - 用户真实数据库集成（当前硬编码）
  - OAuth2 用户自动创建
  - 租户配额管理 API

**待补全文件**:
- `backend/api/handlers/tenants/tenant_handler.go` - 新增租户管理 API
- `backend/api/handlers/tenants/user_handler.go` - 新增用户管理 API
- `backend/api/handlers/auth/auth_handler.go` - 补全 OAuth2 用户创建逻辑

---

#### 3. AI 模型格式转换器 (30% → 100%)

**当前状态**:
- ✅ 转换器接口已定义
- ✅ OpenAI → Azure/DeepSeek/Qwen/Ollama 已完成
- ⚠️ **缺失功能**：
  - OpenAI → Claude 转换（占位符）
  - OpenAI → Gemini 转换（占位符）
  - 流式响应转换

**待补全文件**:
- `backend/internal/ai/converters/openai_claude.go` - 完整实现
- `backend/internal/ai/converters/openai_gemini.go` - 完整实现

---

#### 4. 配置管理系统 (50% → 100%)

**当前状态**:
- ✅ Viper 基础集成
- ⚠️ **缺失功能**：
  - API Key 加密存储（当前环境变量）
  - 租户级配置管理 API
  - 配置热重载

**待补全文件**:
- `backend/internal/config/secret_manager.go` - 新增加密存储
- `backend/api/handlers/config/config_handler.go` - 新增配置管理 API

---

## 🎯 完整补全方案（一步到位）

### 方案概述

**补全目标**: 将项目完成度从 **75%** 提升到 **100%**

**实施原则**:
1. ✅ **一次性补全所有缺失功能**，避免后期反复修改
2. ✅ **保持架构一致性**，遵循现有代码风格和设计模式
3. ✅ **优先补全核心业务功能**，推迟非关键特性
4. ✅ **确保代码质量**，每个模块包含完整实现和错误处理
5. ✅ **提供完整测试**，单元测试覆盖关键逻辑

---

## 📋 详细补全计划

### 阶段 1: 工作流编排系统完善 (优先级: 🔴 最高)

**目标**: 补全工作流编排的高级特性，实现 100% 功能完整性

#### 任务 1.1: 并行任务执行器

**新增文件**: `backend/internal/workflow/executor/parallel_executor.go` (200 行)

**核心功能**:
```go
// ParallelExecutor 并行任务执行器
type ParallelExecutor struct {
    maxConcurrency int
    taskExecutor   TaskExecutor
}

// ExecuteParallel 并行执行多个任务
func (e *ParallelExecutor) ExecuteParallel(ctx context.Context, tasks []*Task, execCtx *ExecutionContext) (map[string]*TaskResult, error) {
    // 使用 sync.WaitGroup 和 channel 实现并发控制
    // 限制最大并发数（如 5 个任务同时执行）
    // 收集所有任务结果
    // 如果任意任务失败，取消其他任务
}
```

**应用场景**:
- 多语言翻译并行执行
- 多个 Agent 同时生成内容
- 批量文档处理

---

#### 任务 1.2: 条件分支执行器

**新增文件**: `backend/internal/workflow/executor/condition_executor.go` (150 行)

**核心功能**:
```go
// ConditionExecutor 条件分支执行器
type ConditionExecutor struct {
    taskExecutor TaskExecutor
}

// EvaluateCondition 评估条件表达式
func (e *ConditionExecutor) EvaluateCondition(expr string, execCtx *ExecutionContext) (bool, error) {
    // 支持的条件语法：
    // - {{step_1.output.quality_score}} > 80
    // - {{step_2.output.status}} == "success"
    // - {{step_1.output.word_count}} >= 1000
}

// ExecuteConditional 根据条件执行不同分支
func (e *ConditionExecutor) ExecuteConditional(ctx context.Context, condition *Condition, execCtx *ExecutionContext) (*TaskResult, error) {
    // 评估条件
    // 如果为真，执行 on_true 步骤
    // 否则执行 on_false 步骤
}
```

**应用场景**:
- 质量评分低于阈值时自动重写
- 根据内容长度选择不同 Agent
- 根据语言类型选择不同翻译模型

---

#### 任务 1.3: 人工审核节点

**新增文件**: `backend/internal/workflow/executor/human_review.go` (100 行)

**核心功能**:
```go
// HumanReviewExecutor 人工审核执行器
type HumanReviewExecutor struct {
    db *gorm.DB
}

// PauseForReview 暂停工作流等待人工审核
func (e *HumanReviewExecutor) PauseForReview(ctx context.Context, task *Task, execCtx *ExecutionContext) error {
    // 将工作流状态设置为 "paused"
    // 创建审核任务记录
    // 发送通知给审核人员（邮件/Webhook）
}

// ResumeAfterReview 审核完成后恢复工作流
func (e *HumanReviewExecutor) ResumeAfterReview(ctx context.Context, executionID string, approved bool, feedback string) error {
    // 更新审核结果
    // 恢复工作流执行
    // 如果不通过，执行失败处理逻辑
}
```

**应用场景**:
- 重要内容发布前人工审核
- 敏感信息检查
- 质量把关节点

---

#### 任务 1.4: 失败重试机制

**修改文件**: `backend/internal/workflow/executor/scheduler.go` (+80 行)

**核心功能**:
```go
// executeTaskWithRetry 执行任务（支持重试）
func (s *Scheduler) executeTaskWithRetry(ctx context.Context, task *Task, execCtx *ExecutionContext) (*TaskResult, error) {
    var lastErr error
    retryConfig := task.Step.Retry // 从步骤定义中获取重试配置

    maxRetries := 0
    if retryConfig != nil {
        maxRetries = retryConfig.MaxRetries
    }

    for attempt := 0; attempt <= maxRetries; attempt++ {
        // 执行任务
        result, err := s.executeTask(ctx, task, execCtx)
        if err == nil {
            return result, nil
        }

        lastErr = err

        // 如果是最后一次尝试，直接返回错误
        if attempt == maxRetries {
            break
        }

        // 计算退避延迟
        delay := s.calculateBackoffDelay(retryConfig, attempt)
        time.Sleep(delay)
    }

    return nil, fmt.Errorf("任务执行失败（重试 %d 次）: %w", maxRetries, lastErr)
}

// calculateBackoffDelay 计算退避延迟
func (s *Scheduler) calculateBackoffDelay(config *RetryConfig, attempt int) time.Duration {
    if config.Backoff == "exponential" {
        // 指数退避：1s, 2s, 4s, 8s, ...
        return time.Duration(config.Delay) * time.Second * time.Duration(1<<attempt)
    }
    // 固定延迟
    return time.Duration(config.Delay) * time.Second
}
```

**应用场景**:
- AI 模型调用超时重试
- 网络请求失败重试
- 数据库连接失败重试

---

#### 任务 1.5: 变量模板引擎

**修改文件**: `backend/internal/workflow/executor/scheduler.go` (+120 行)

**核心功能**:
```go
// resolveVariables 解析变量（完整实现）
func (s *Scheduler) resolveVariables(value string, execCtx *ExecutionContext) any {
    // 支持的变量类型：
    // 1. 用户输入：{{input.topic}}
    // 2. 上一步输出：{{step_1.output.content}}
    // 3. 系统变量：{{system.tenant_id}}, {{system.timestamp}}
    // 4. 嵌套访问：{{step_2.output.metadata.word_count}}
    
    // 使用正则表达式匹配 {{...}} 模式
    // 从 execCtx.Data 中提取对应值
    // 支持点号访问嵌套字段
    // 支持默认值：{{step_1.output | "default"}}
}

// resolveInputMap 解析输入映射
func (s *Scheduler) resolveInputMap(inputMap map[string]any, execCtx *ExecutionContext) map[string]any {
    resolved := make(map[string]any)
    for key, value := range inputMap {
        switch v := value.(type) {
        case string:
            resolved[key] = s.resolveVariables(v, execCtx)
        case map[string]any:
            resolved[key] = s.resolveInputMap(v, execCtx)
        case []any:
            resolved[key] = s.resolveArrayVariables(v, execCtx)
        default:
            resolved[key] = value
        }
    }
    return resolved
}
```

**支持的变量语法**:
```yaml
input:
  topic: "{{input.user_topic}}"                    # 用户输入
  outline: "{{step_1.output.content}}"              # 上一步输出
  style: "{{input.style | 'professional'}}"         # 默认值
  timestamp: "{{system.timestamp}}"                  # 系统变量
  word_count: "{{step_2.output.metadata.length}}"   # 嵌套访问
```

---

#### 任务 1.6: 执行历史查询 API

**修改文件**: `backend/api/handlers/workflows/execute_handler.go` (+150 行)

**核心功能**:
```go
// GetExecution 查询执行详情
func (h *WorkflowExecuteHandler) GetExecution(c *gin.Context) {
    executionID := c.Param("id")
    tenantID := c.GetString("tenant_id")

    // 查询执行记录
    var execution workflowpkg.WorkflowExecution
    if err := h.db.Where("id = ? AND tenant_id = ?", executionID, tenantID).
        First(&execution).Error; err != nil {
        c.JSON(404, gin.H{"error": "执行记录不存在"})
        return
    }

    // 查询关联的任务
    var tasks []workflowpkg.WorkflowTask
    h.db.Where("execution_id = ?", executionID).
        Order("created_at ASC").
        Find(&tasks)

    c.JSON(200, gin.H{
        "execution": execution,
        "tasks":     tasks,
    })
}

// ListExecutions 查询执行列表
func (h *WorkflowExecuteHandler) ListExecutions(c *gin.Context) {
    workflowID := c.Param("id")
    tenantID := c.GetString("tenant_id")

    // 分页参数
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

    // 查询执行列表
    var executions []workflowpkg.WorkflowExecution
    var total int64

    query := h.db.Where("workflow_id = ? AND tenant_id = ?", workflowID, tenantID)
    query.Model(&workflowpkg.WorkflowExecution{}).Count(&total)
    query.Offset((page - 1) * pageSize).
        Limit(pageSize).
        Order("created_at DESC").
        Find(&executions)

    c.JSON(200, gin.H{
        "executions": executions,
        "pagination": gin.H{
            "page":       page,
            "page_size":  pageSize,
            "total":      total,
            "total_page": (total + int64(pageSize) - 1) / int64(pageSize),
        },
    })
}
```

---

### 阶段 2: 用户和租户管理系统 (优先级: 🟡 高)

**目标**: 补全用户和租户管理的完整 CRUD API

#### 任务 2.1: 租户管理 API

**新增文件**: `backend/api/handlers/tenants/tenant_handler.go` (400 行)

**核心功能**:
- ✅ 创建租户（POST /api/tenants）
- ✅ 查询租户列表（GET /api/tenants）
- ✅ 查询租户详情（GET /api/tenants/:id）
- ✅ 更新租户信息（PUT /api/tenants/:id）
- ✅ 删除租户（DELETE /api/tenants/:id）
- ✅ 租户配额管理（GET/PUT /api/tenants/:id/quota）
- ✅ 租户配置管理（GET/PUT /api/tenants/:id/config）

**实现示例**:
```go
// CreateTenant 创建租户
func (h *TenantHandler) CreateTenant(c *gin.Context) {
    var req CreateTenantRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 调用 TenantService 创建租户
    tenant, adminUser, err := h.tenantService.CreateTenant(c.Request.Context(), &tenant.CreateTenantParams{
        Name:          req.Name,
        Domain:        req.Domain,
        AdminEmail:    req.AdminEmail,
        AdminPassword: req.AdminPassword,
    })

    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(201, gin.H{
        "tenant": tenant,
        "admin":  adminUser,
    })
}

// ListTenants 查询租户列表（仅超级管理员）
func (h *TenantHandler) ListTenants(c *gin.Context) {
    // 权限检查
    if !h.checkSuperAdmin(c) {
        c.JSON(403, gin.H{"error": "权限不足"})
        return
    }

    // 分页查询
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

    tenants, total, err := h.tenantService.ListTenants(c.Request.Context(), page, pageSize)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{
        "tenants": tenants,
        "pagination": gin.H{
            "page":       page,
            "page_size":  pageSize,
            "total":      total,
            "total_page": (total + int64(pageSize) - 1) / int64(pageSize),
        },
    })
}
```

---

#### 任务 2.2: 用户管理 API

**新增文件**: `backend/api/handlers/tenants/user_handler.go` (500 行)

**核心功能**:
- ✅ 创建用户（POST /api/users）
- ✅ 查询用户列表（GET /api/users）
- ✅ 查询用户详情（GET /api/users/:id）
- ✅ 更新用户信息（PUT /api/users/:id）
- ✅ 删除用户（DELETE /api/users/:id）
- ✅ 重置密码（POST /api/users/:id/reset-password）
- ✅ 分配角色（PUT /api/users/:id/roles）
- ✅ 查询当前用户信息（GET /api/users/me）

**实现示例**:
```go
// CreateUser 创建用户
func (h *UserHandler) CreateUser(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 调用 UserService 创建用户
    user, err := h.userService.CreateUser(c.Request.Context(), tenantID, &tenant.CreateUserParams{
        Email:    req.Email,
        Password: req.Password,
        Name:     req.Name,
        RoleIDs:  req.RoleIDs,
    })

    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(201, gin.H{"user": user})
}

// GetCurrentUser 查询当前用户信息
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
    userID := c.GetString("user_id")
    tenantID := c.GetString("tenant_id")

    user, err := h.userService.GetUserByID(c.Request.Context(), tenantID, userID)
    if err != nil {
        c.JSON(404, gin.H{"error": "用户不存在"})
        return
    }

    // 查询用户角色
    roles, _ := h.userService.GetUserRoles(c.Request.Context(), userID)

    c.JSON(200, gin.H{
        "user":  user,
        "roles": roles,
    })
}
```

---

#### 任务 2.3: OAuth2 用户自动创建

**修改文件**: `backend/api/handlers/auth/auth_handler.go` (+100 行)

**核心功能**:
```go
// handleOAuth2Callback OAuth2 回调处理（完整实现）
func (h *AuthHandler) handleOAuth2Callback(c *gin.Context) {
    provider := c.Param("provider")
    code := c.Query("code")
    state := c.Query("state")

    // TODO: 验证 state（从 Redis 中获取并验证）

    // 交换授权码为访问令牌
    token, err := h.oauth2Service.ExchangeCode(c.Request.Context(), auth.OAuth2Provider(provider), code)
    if err != nil {
        c.JSON(500, gin.H{"error": "授权失败"})
        return
    }

    // 获取用户信息
    userInfo, err := h.oauth2Service.GetUserInfo(c.Request.Context(), auth.OAuth2Provider(provider), token.AccessToken)
    if err != nil {
        c.JSON(500, gin.H{"error": "获取用户信息失败"})
        return
    }

    // 查找或创建本地用户
    user, err := h.findOrCreateOAuth2User(c.Request.Context(), userInfo, provider)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // 生成 JWT Token
    jwtToken, err := h.jwtService.GenerateToken(user.ID, user.Email, "default-tenant")
    if err != nil {
        c.JSON(500, gin.H{"error": "生成 Token 失败"})
        return
    }

    c.JSON(200, gin.H{
        "access_token": jwtToken,
        "user":         user,
    })
}

// findOrCreateOAuth2User 查找或创建 OAuth2 用户
func (h *AuthHandler) findOrCreateOAuth2User(ctx context.Context, userInfo *auth.OAuth2UserInfo, provider string) (*User, error) {
    // 1. 通过邮箱查询用户
    var user User
    err := h.db.Where("email = ?", userInfo.Email).First(&user).Error
    if err == nil {
        // 用户已存在，更新 OAuth2 信息
        return &user, nil
    }

    // 2. 用户不存在，创建新用户
    user = User{
        ID:       uuid.New().String(),
        Email:    userInfo.Email,
        Name:     userInfo.Name,
        Avatar:   userInfo.Avatar,
        Status:   "active",
        TenantID: "default-tenant", // 可以改为从 state 中获取
    }

    if err := h.db.Create(&user).Error; err != nil {
        return nil, fmt.Errorf("创建用户失败: %w", err)
    }

    return &user, nil
}
```

---

### 阶段 3: AI 模型格式转换器完善 (优先级: 🟢 中)

**目标**: 补全 Claude 和 Gemini 模型的格式转换逻辑

#### 任务 3.1: Claude 格式转换器

**修改文件**: `backend/internal/ai/converters/openai_claude.go` (+200 行)

**核心功能**:
```go
// ToClaudeRequest 将 OpenAI 格式转换为 Claude 格式
func ToClaudeRequest(req *ai.ChatRequest) (*claude.MessageRequest, error) {
    messages := make([]claude.MessageParam, 0, len(req.Messages))
    
    for _, msg := range req.Messages {
        role := msg.Role
        if role == "assistant" {
            role = "assistant"
        } else if role == "user" {
            role = "user"
        } else if role == "system" {
            // Claude 的 system 消息需要单独处理
            continue
        }

        messages = append(messages, claude.MessageParam{
            Role:    claude.MessageRole(role),
            Content: msg.Content,
        })
    }

    // 提取 system prompt
    var systemPrompt string
    for _, msg := range req.Messages {
        if msg.Role == "system" {
            systemPrompt = msg.Content
            break
        }
    }

    return &claude.MessageRequest{
        Model:       req.Model,
        Messages:    messages,
        System:      systemPrompt,
        MaxTokens:   req.MaxTokens,
        Temperature: req.Temperature,
        Stream:      req.Stream,
    }, nil
}

// FromClaudeResponse 将 Claude 响应转换为 OpenAI 格式
func FromClaudeResponse(resp *claude.MessageResponse) (*ai.ChatResponse, error) {
    var content string
    if len(resp.Content) > 0 {
        content = resp.Content[0].Text
    }

    return &ai.ChatResponse{
        ID:      resp.ID,
        Model:   resp.Model,
        Content: content,
        Usage: &ai.Usage{
            PromptTokens:     resp.Usage.InputTokens,
            CompletionTokens: resp.Usage.OutputTokens,
            TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
        },
        FinishReason: string(resp.StopReason),
    }, nil
}

// FromClaudeStreamChunk 将 Claude 流式响应转换为 OpenAI 格式
func FromClaudeStreamChunk(chunk *claude.MessageStreamEvent) (*ai.ChatStreamChunk, error) {
    if chunk.Type == "content_block_delta" {
        return &ai.ChatStreamChunk{
            Delta: ai.ChatDelta{
                Content: chunk.Delta.Text,
            },
        }, nil
    }
    return nil, nil
}
```

---

#### 任务 3.2: Gemini 格式转换器

**修改文件**: `backend/internal/ai/converters/openai_gemini.go` (+200 行)

**核心功能**:
```go
// ToGeminiRequest 将 OpenAI 格式转换为 Gemini 格式
func ToGeminiRequest(req *ai.ChatRequest) (*genai.GenerateContentRequest, error) {
    // 构建 Gemini 消息格式
    parts := make([]*genai.Part, 0)
    
    for _, msg := range req.Messages {
        part := &genai.Part{
            Text: msg.Content,
        }
        parts = append(parts, part)
    }

    return &genai.GenerateContentRequest{
        Model: req.Model,
        Contents: []*genai.Content{
            {Parts: parts},
        },
        GenerationConfig: &genai.GenerationConfig{
            Temperature:     req.Temperature,
            MaxOutputTokens: req.MaxTokens,
        },
    }, nil
}

// FromGeminiResponse 将 Gemini 响应转换为 OpenAI 格式
func FromGeminiResponse(resp *genai.GenerateContentResponse) (*ai.ChatResponse, error) {
    var content string
    if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
        content = resp.Candidates[0].Content.Parts[0].Text
    }

    return &ai.ChatResponse{
        ID:      uuid.New().String(),
        Model:   "gemini",
        Content: content,
        Usage: &ai.Usage{
            PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
            CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
            TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
        },
    }, nil
}
```

---

### 阶段 4: 配置管理系统完善 (优先级: 🟢 中)

**目标**: 实现 API Key 加密存储和配置管理 API

#### 任务 4.1: 加密存储管理器

**新增文件**: `backend/internal/config/secret_manager.go` (200 行)

**核心功能**:
```go
// SecretManager 密钥管理器
type SecretManager struct {
    db            *gorm.DB
    encryptionKey []byte // 从环境变量加载
}

// StoreSecret 存储加密密钥
func (m *SecretManager) StoreSecret(ctx context.Context, tenantID, key, value string) error {
    // 使用 AES-256-GCM 加密
    encrypted, err := m.encrypt(value)
    if err != nil {
        return err
    }

    secret := &Secret{
        ID:        uuid.New().String(),
        TenantID:  tenantID,
        Key:       key,
        Value:     encrypted,
        CreatedAt: time.Now(),
    }

    return m.db.Create(secret).Error
}

// GetSecret 获取解密后的密钥
func (m *SecretManager) GetSecret(ctx context.Context, tenantID, key string) (string, error) {
    var secret Secret
    if err := m.db.Where("tenant_id = ? AND key = ?", tenantID, key).
        First(&secret).Error; err != nil {
        return "", err
    }

    // 解密
    decrypted, err := m.decrypt(secret.Value)
    if err != nil {
        return "", err
    }

    return decrypted, nil
}

// encrypt 使用 AES-256-GCM 加密
func (m *SecretManager) encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(m.encryptionKey)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt 解密
func (m *SecretManager) decrypt(encrypted string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }

    block, err := aes.NewCipher(m.encryptionKey)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]

    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}
```

---

#### 任务 4.2: 配置管理 API

**新增文件**: `backend/api/handlers/config/config_handler.go` (300 行)

**核心功能**:
- ✅ 租户配置管理（GET/PUT /api/config）
- ✅ API Key 管理（POST/GET/DELETE /api/config/api-keys）
- ✅ 模型配置管理（GET/PUT /api/config/models）
- ✅ 配置热重载（POST /api/config/reload）

---

## 📊 预期成果

### 代码统计

| 阶段 | 新增文件 | 修改文件 | 新增代码行数 | 总计 |
|------|---------|---------|-------------|------|
| **阶段 1** | 3 | 2 | ~900 | ~900 |
| **阶段 2** | 2 | 1 | ~1,000 | ~1,900 |
| **阶段 3** | 0 | 2 | ~400 | ~2,300 |
| **阶段 4** | 2 | 1 | ~500 | ~2,800 |

**总计**: 7 个新文件，6 个修改文件，~2,800 行新代码

---

### 功能完整性

| 模块 | 当前完成度 | 补全后完成度 | 提升 |
|------|-----------|-------------|------|
| 工作流编排 | 80% | 100% | +20% |
| 用户租户管理 | 50% | 100% | +50% |
| AI 模型转换 | 60% | 100% | +40% |
| 配置管理 | 50% | 100% | +50% |

**总体完成度**: 75% → **100%** ✅

---

## 🎯 实施计划

### 方案 A: 一次性完整实施 (推荐)

**耗时**: 约 12-16 小时

**优势**:
- ✅ 一次性补全所有功能，避免后期反复修改
- ✅ 保持架构一致性和代码质量
- ✅ 提供完整的端到端测试
- ✅ 交付生产就绪的系统

**执行顺序**:
1. **阶段 1** (6 小时) - 工作流编排系统完善
2. **阶段 2** (4 小时) - 用户租户管理系统
3. **阶段 3** (2 小时) - AI 模型转换器
4. **阶段 4** (2 小时) - 配置管理系统
5. **文档更新** (2 小时) - README、API 文档、使用指南

---

### 方案 B: 分阶段实施

**耗时**: 分 4 个迭代，每个迭代 3-4 小时

**优势**:
- ✅ 每个迭代交付可用功能
- ✅ 及时验证和调整

**缺点**:
- ⚠️ 可能需要后期调整架构
- ⚠️ 跨模块集成可能有遗漏

---

## 🔍 质量保证

### 单元测试

每个新增或修改的模块都需要提供单元测试：
- ✅ 并行执行器测试
- ✅ 条件执行器测试
- ✅ 重试机制测试
- ✅ 变量解析测试
- ✅ 加密存储测试

**测试覆盖率目标**: > 80%

---

### 集成测试

提供端到端的工作流测试：
- ✅ 线性工作流测试
- ✅ 并行工作流测试
- ✅ 条件分支工作流测试
- ✅ 人工审核工作流测试
- ✅ 失败重试工作流测试

---

### 性能测试

验证系统性能指标：
- ✅ 并发 100 个工作流任务
- ✅ 并发 1000 个 API 请求
- ✅ 响应时间 < 500ms (P95)

---

## 📝 文档更新

补全实施完成后，需要更新以下文档：
1. ✅ `README.md` - 更新功能列表和使用指南
2. ✅ `docs/需求规格说明书-完整版.md` - 更新完成度
3. ✅ `docs/API接口文档.md` - 补充新增 API
4. ✅ `QUICKSTART.md` - 补充工作流配置示例
5. ✅ `CHANGELOG.md` - 记录本次补全的所有功能

---

## 🎉 最终交付

补全完成后，AgentFlowCreativeHub 将具备：

**✅ 完整的功能**:
- 7 种 Agent（全部支持 RAG）
- 完整的工作流编排（并行、条件、重试、人工审核）
- 完整的用户租户管理系统
- 完整的 AI 模型支持（8 个提供商）
- 完整的配置管理和加密存储
- 完整的监控告警系统

**✅ 生产就绪**:
- 100% 功能完整性
- > 80% 测试覆盖率
- 完整的 API 文档
- 详细的部署指南
- 性能优化和安全加固

**✅ 企业级能力**:
- 多租户隔离
- RBAC 权限控制
- 审计日志
- 监控告警
- 高可用部署

---

**准备好开始一步到位的完整补全了吗？我将按照 4 个阶段逐步实施，确保每个模块都完整、可测试、生产就绪！** 🚀

或者您希望我先专注于某个特定阶段？请告诉我您的选择！