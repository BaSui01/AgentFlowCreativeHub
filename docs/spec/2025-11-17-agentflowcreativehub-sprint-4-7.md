# 🚀 AgentFlowCreativeHub 全面升级规划

基于对 **new-api** 和 **cherry-studio** 两个成熟项目的深入研究，以及对当前项目架构的全面审视，提出以下**4 个 Sprint 的系统性改进方案**。

---

## 📊 **当前状态评估**

### ✅ **已完成的核心能力** (Sprint 1-3)
- 基础设施层：配置管理、日志、数据库、HTTP 服务器
- 服务层：Model、Template、Agent、Workflow 四大服务
- API 层：39 个 REST 端点（30 业务 + 4 Agent 执行 + 4 Workflow + 1 健康检查）
- AI 适配层：OpenAI、Claude 客户端 + 统一接口 + 日志记录
- Agent 运行时：Writer、Reviewer、Formatter + 上下文管理 + 注册表
- 工作流引擎：YAML/JSON 解析 + DAG 构建 + 拓扑排序 + 并行调度

### 🔴 **核心短板**
1. **提供商支持严重不足**：仅支持 2 个提供商（OpenAI/Claude），对比 new-api 的 50+ 提供商
2. **格式兼容性缺失**：无法处理 Gemini、DeepSeek 等非 OpenAI 格式
3. **模型发现能力缺失**：无法自动获取各提供商的最新模型列表
4. **认证系统未实现**：仍在使用 MockTenantContext，无 JWT/OAuth2
5. **RAG 功能未实现**：有表结构和接口定义，但无实际实现
6. **工具调用未实现**：Tool 表已设计，但无执行引擎
7. **监控可观测性缺失**：无 Prometheus 指标、Tracing、错误监控

---

## 🎯 **Sprint 4: 多提供商支持 + 格式兼容层** (优先级: P0)

### **核心目标**
将当前 2 提供商系统扩展为**类 new-api 的多提供商 AI 网关**，支持主流 AI 模型统一调用。

### **1. 新增提供商适配器** (5-7 天)

#### **优先级排序**
| 提供商 | 优先级 | 理由 | 预估工期 |
|--------|-------|------|---------|
| Google Gemini | P0 | 用户需求明确 | 1.5 天 |
| Azure OpenAI | P0 | 企业常用 | 1 天 |
| DeepSeek | P0 | 国产高性价比 | 1 天 |
| Qwen (通义千问) | P1 | 国产主流 | 1 天 |
| Ollama | P1 | 本地部署需求 | 1.5 天 |
| Custom Endpoint | P1 | 灵活性 | 1 天 |

#### **目录结构**
```
backend/internal/ai/
├── client.go              # ✅ 已有统一接口
├── factory.go             # ✅ 已有工厂，需扩展
├── converters/            # 🆕 格式转换层
│   ├── converter.go       # 转换器接口
│   ├── openai_claude.go   # OpenAI ⇄ Claude
│   ├── openai_gemini.go   # OpenAI ⇄ Gemini
│   └── response_wrapper.go # 统一响应包装
├── openai/               # ✅ 已实现
├── anthropic/            # ✅ 已实现
├── google/               # 🆕 Gemini 适配器
│   ├── client.go
│   ├── converter.go      # Gemini 格式转换
│   └── models.go         # Gemini 模型列表
├── azure/                # 🆕 Azure OpenAI
│   └── client.go
├── deepseek/             # 🆕 DeepSeek
│   └── client.go
├── qwen/                 # 🆕 通义千问
│   └── client.go
├── ollama/               # 🆕 Ollama 本地
│   └── client.go
└── custom/               # 🆕 自定义端点
    └── client.go
```

#### **格式转换器设计**

**核心接口**：
```go
// converters/converter.go
type FormatConverter interface {
    ConvertRequest(from, to Format, req any) (any, error)
    ConvertResponse(from, to Format, resp any) (any, error)
}

type Format string

const (
    FormatOpenAI   Format = "openai"
    FormatClaude   Format = "claude"
    FormatGemini   Format = "gemini"
    FormatDeepSeek Format = "deepseek"
)
```

**转换示例**：
```go
// OpenAI → Gemini 请求转换
func (c *OpenAIToGeminiConverter) ConvertRequest(req *ai.ChatCompletionRequest) (*gemini.GenerateContentRequest, error) {
    // 1. 消息格式转换
    contents := make([]*gemini.Content, 0)
    for _, msg := range req.Messages {
        contents = append(contents, &gemini.Content{
            Role:  convertRole(msg.Role),
            Parts: []gemini.Part{{Text: msg.Content}},
        })
    }
    
    // 2. 参数映射
    return &gemini.GenerateContentRequest{
        Contents:         contents,
        GenerationConfig: &gemini.GenerationConfig{
            Temperature:     req.Temperature,
            MaxOutputTokens: req.MaxTokens,
            TopP:            req.TopP,
        },
    }, nil
}

// Gemini → OpenAI 响应转换
func (c *GeminiToOpenAIConverter) ConvertResponse(resp *gemini.GenerateContentResponse) (*ai.ChatCompletionResponse, error) {
    return &ai.ChatCompletionResponse{
        ID:      generateID(),
        Model:   resp.ModelVersion,
        Content: resp.Candidates[0].Content.Parts[0].Text,
        Usage: ai.Usage{
            PromptTokens:     resp.UsageMetadata.PromptTokenCount,
            CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
            TotalTokens:      resp.UsageMetadata.TotalTokenCount,
        },
    }, nil
}
```

### **2. 增强 Model 数据模型** (1 天)

#### **数据库迁移**
```sql
-- 0005_enhance_models.sql
ALTER TABLE models ADD COLUMN category VARCHAR(50) DEFAULT 'chat';
ALTER TABLE models ADD COLUMN features JSONB DEFAULT '{}';
ALTER TABLE models ADD COLUMN base_url VARCHAR(500);
ALTER TABLE models ADD COLUMN api_version VARCHAR(50);
ALTER TABLE models ADD COLUMN api_format VARCHAR(50) DEFAULT 'openai';
ALTER TABLE models ADD COLUMN is_builtin BOOLEAN DEFAULT false;
ALTER TABLE models ADD COLUMN is_active BOOLEAN DEFAULT true;
ALTER TABLE models ADD COLUMN last_synced_at TIMESTAMPTZ;

COMMENT ON COLUMN models.category IS 'chat, image, audio, video, embedding, rerank';
COMMENT ON COLUMN models.features IS '{"vision": true, "function_calling": true, "streaming": true, "cache": true}';
COMMENT ON COLUMN models.api_format IS 'openai, claude, gemini, deepseek, custom';

CREATE INDEX idx_models_provider_category ON models (provider, category) WHERE deleted_at IS NULL;
CREATE INDEX idx_models_is_active ON models (is_active) WHERE deleted_at IS NULL;
```

#### **Model 结构扩展**
```go
// internal/models/models.go
type Model struct {
    // ... 现有字段 ...
    
    // 🆕 新增字段
    Category       string         `json:"category"`       // chat, image, embedding, rerank
    Features       ModelFeatures  `json:"features"`       // 能力特性
    BaseURL        string         `json:"baseUrl"`        // 自定义端点
    APIVersion     string         `json:"apiVersion"`     // Azure API 版本
    APIFormat      string         `json:"apiFormat"`      // openai, claude, gemini
    IsBuiltin      bool           `json:"isBuiltin"`      // 是否内置
    IsActive       bool           `json:"isActive"`       // 是否启用
    LastSyncedAt   *time.Time     `json:"lastSyncedAt"`   // 最后同步时间
}

type ModelFeatures struct {
    Vision          bool `json:"vision"`
    FunctionCalling bool `json:"functionCalling"`
    Streaming       bool `json:"streaming"`
    Cache           bool `json:"cache"`
    JsonMode        bool `json:"jsonMode"`
}
```

### **3. 模型自动发现功能** (2-3 天)

#### **核心接口**
```go
// internal/models/discovery.go
type ModelDiscoveryService struct {
    db            *gorm.DB
    clientFactory *ai.ClientFactory
}

// SyncModelsFromProvider 从提供商同步模型列表
func (s *ModelDiscoveryService) SyncModelsFromProvider(ctx context.Context, tenantID, provider string) (int, error) {
    // 1. 获取提供商客户端
    // 2. 调用 /models 或 /model/list 端点
    // 3. 解析响应并转换为统一格式
    // 4. 批量插入/更新数据库
    // 5. 返回同步数量
}

// AutoDiscoverModels 自动发现所有提供商的模型
func (s *ModelDiscoveryService) AutoDiscoverModels(ctx context.Context, tenantID string) (map[string]int, error) {
    providers := []string{"openai", "anthropic", "google", "azure", "deepseek", "qwen"}
    results := make(map[string]int)
    
    for _, provider := range providers {
        count, err := s.SyncModelsFromProvider(ctx, tenantID, provider)
        if err != nil {
            log.Warnf("发现 %s 模型失败: %v", provider, err)
            continue
        }
        results[provider] = count
    }
    
    return results, nil
}
```

#### **定时同步任务**
```go
// internal/models/sync_scheduler.go
func (s *ModelDiscoveryService) StartSyncScheduler(ctx context.Context) {
    ticker := time.NewTicker(24 * time.Hour) // 每天同步一次
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // 获取所有租户
            tenants := s.getAllTenants(ctx)
            
            for _, tenant := range tenants {
                results, err := s.AutoDiscoverModels(ctx, tenant.ID)
                if err != nil {
                    log.Errorf("租户 %s 模型同步失败: %v", tenant.ID, err)
                    continue
                }
                log.Infof("租户 %s 模型同步成功: %+v", tenant.ID, results)
            }
            
        case <-ctx.Done():
            return
        }
    }
}
```

### **4. API 新增端点** (1 天)

```go
// internal/api/handlers/models_discovery.go
// POST /api/models/discover/:provider
func (h *ModelsHandler) DiscoverModels(c *gin.Context) {
    provider := c.Param("provider")
    tenantID := getTenantID(c)
    
    count, err := h.discoveryService.SyncModelsFromProvider(c, tenantID, provider)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{
        "provider": provider,
        "count":    count,
        "message":  fmt.Sprintf("成功同步 %d 个模型", count),
    })
}

// POST /api/models/discover-all
func (h *ModelsHandler) DiscoverAllModels(c *gin.Context) {
    tenantID := getTenantID(c)
    
    results, err := h.discoveryService.AutoDiscoverModels(c, tenantID)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{
        "results": results,
        "total":   sumValues(results),
    })
}
```

---

## 🔐 **Sprint 5: 认证授权 + 审计系统** (优先级: P0)

### **核心目标**
替换 MockTenantContext，实现完整的 JWT + OAuth2 认证授权系统，补全审计日志。

### **1. JWT 认证中间件** (1-2 天)

#### **目录结构**
```
backend/internal/auth/
├── jwt.go                # JWT 生成/验证
├── oauth2.go             # OAuth2 客户端
├── rbac.go               # ✅ 已有 RBAC
├── middleware.go         # 认证中间件
└── models.go             # Session、Token 模型
```

#### **JWT 实现**
```go
// internal/auth/jwt.go
type JWTService struct {
    secretKey     []byte
    issuer        string
    accessExpiry  time.Duration // 2 小时
    refreshExpiry time.Duration // 7 天
}

func (s *JWTService) GenerateTokenPair(userID, tenantID string, roles []string) (*TokenPair, error) {
    accessToken, err := s.generateToken(userID, tenantID, roles, s.accessExpiry)
    if err != nil {
        return nil, err
    }
    
    refreshToken, err := s.generateToken(userID, tenantID, roles, s.refreshExpiry)
    if err != nil {
        return nil, err
    }
    
    return &TokenPair{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    int(s.accessExpiry.Seconds()),
    }, nil
}

func (s *JWTService) ValidateToken(tokenString string) (*TokenClaims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (any, error) {
        return s.secretKey, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
        return claims, nil
    }
    
    return nil, fmt.Errorf("无效的 Token")
}

type TokenClaims struct {
    UserID   string   `json:"uid"`
    TenantID string   `json:"tid"`
    Roles    []string `json:"roles"`
    jwt.RegisteredClaims
}
```

#### **认证中间件**
```go
// internal/auth/middleware.go
func JWTAuthMiddleware(jwtService *JWTService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 从 Header 提取 Token
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, gin.H{"error": "缺少认证信息"})
            c.Abort()
            return
        }
        
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        
        // 2. 验证 Token
        claims, err := jwtService.ValidateToken(tokenString)
        if err != nil {
            c.JSON(401, gin.H{"error": "认证失败"})
            c.Abort()
            return
        }
        
        // 3. 注入上下文
        c.Set("userID", claims.UserID)
        c.Set("tenantID", claims.TenantID)
        c.Set("roles", claims.Roles)
        
        c.Next()
    }
}

func RequirePermission(permission string) gin.HandlerFunc {
    return func(c *gin.Context) {
        roles, _ := c.Get("roles")
        tenantID, _ := c.Get("tenantID")
        
        // RBAC 权限检查
        if !hasPermission(tenantID, roles.([]string), permission) {
            c.JSON(403, gin.H{"error": "权限不足"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

### **2. OAuth2 集成** (1 天)

支持第三方登录：Google、GitHub、Microsoft、企业 SSO (OIDC)。

```go
// internal/auth/oauth2.go
type OAuth2Provider struct {
    Name         string
    ClientID     string
    ClientSecret string
    RedirectURL  string
    Scopes       []string
    Config       *oauth2.Config
}

func (p *OAuth2Provider) GetAuthURL(state string) string {
    return p.Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *OAuth2Provider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
    return p.Config.Exchange(ctx, code)
}

func (p *OAuth2Provider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
    // 调用提供商的 /userinfo 端点
    // 解析用户信息
}
```

### **3. 审计日志增强** (1 天)

#### **审计事件类型**
```go
// internal/audit/events.go
const (
    EventUserLogin           = "user.login"
    EventUserLogout          = "user.logout"
    EventModelCreate         = "model.create"
    EventModelUpdate         = "model.update"
    EventModelDelete         = "model.delete"
    EventWorkflowCreate      = "workflow.create"
    EventWorkflowExecute     = "workflow.execute"
    EventAgentExecute        = "agent.execute"
    EventPermissionChange    = "permission.change"
    EventConfigChange        = "config.change"
)
```

#### **审计中间件**
```go
// internal/audit/middleware.go
func AuditMiddleware(auditService *AuditService) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        // 捕获请求体（用于审计）
        var requestBody []byte
        if c.Request.Body != nil {
            requestBody, _ = io.ReadAll(c.Request.Body)
            c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
        }
        
        c.Next()
        
        // 记录审计日志
        auditService.Log(c, &AuditEntry{
            EventType:    detectEventType(c.Request.Method, c.Request.URL.Path),
            UserID:       getUserID(c),
            TenantID:     getTenantID(c),
            IPAddress:    c.ClientIP(),
            UserAgent:    c.Request.UserAgent(),
            Method:       c.Request.Method,
            Path:         c.Request.URL.Path,
            StatusCode:   c.Writer.Status(),
            RequestBody:  string(requestBody),
            ResponseTime: time.Since(start).Milliseconds(),
        })
    }
}
```

---

## 🔍 **Sprint 6: RAG 功能实现** (优先级: P1)

### **核心目标**
基于已有的 RAG 表结构和接口定义，实现完整的 RAG 全链路功能。

### **1. 文档导入与解析** (2-3 天)

#### **支持格式**
- PDF (使用 `github.com/ledongthuc/pdf` 或 `github.com/pdfcpu/pdfcpu`)
- Word (使用 `github.com/nguyenthenguyen/docx`)
- Markdown (使用 `github.com/yuin/goldmark`)
- TXT / HTML

#### **实现**
```go
// internal/rag/document_processor.go
type DocumentProcessor struct {
    db              *gorm.DB
    embeddingClient EmbeddingProvider
    vectorStore     VectorStore
}

// ImportDocument 导入文档
func (p *DocumentProcessor) ImportDocument(ctx context.Context, req *ImportRequest) (*KnowledgeDocument, error) {
    // 1. 文件上传（保存到对象存储/本地）
    fileURL, err := p.uploadFile(req.File)
    if err != nil {
        return nil, err
    }
    
    // 2. 创建文档记录
    doc := &KnowledgeDocument{
        ID:              uuid.New().String(),
        KnowledgeBaseID: req.KnowledgeBaseID,
        SourceType:      detectFileType(req.File.Filename),
        SourceURI:       fileURL,
        Status:          "pending_index",
        CreatedAt:       time.Now().UTC(),
    }
    
    if err := p.db.Create(doc).Error; err != nil {
        return nil, err
    }
    
    // 3. 异步处理（文本提取 + 分片 + 向量化）
    go p.processDocumentAsync(ctx, doc.ID)
    
    return doc, nil
}

// processDocumentAsync 异步处理文档
func (p *DocumentProcessor) processDocumentAsync(ctx context.Context, docID string) {
    // 1. 提取文本
    text, err := p.extractText(ctx, docID)
    if err != nil {
        p.updateDocumentStatus(ctx, docID, "failed", err.Error())
        return
    }
    
    // 2. 文本分片
    chunks := p.chunkText(text, 512, 50) // 512 tokens, 50 overlap
    
    // 3. 批量向量化（每批 100 个）
    embeddings, err := p.batchEmbedChunks(ctx, chunks, 100)
    if err != nil {
        p.updateDocumentStatus(ctx, docID, "failed", err.Error())
        return
    }
    
    // 4. 存储向量
    if err := p.vectorStore.IndexChunks(ctx, embeddings); err != nil {
        p.updateDocumentStatus(ctx, docID, "failed", err.Error())
        return
    }
    
    // 5. 更新状态
    p.updateDocumentStatus(ctx, docID, "indexed", "")
}
```

### **2. 向量检索实现** (2 天)

#### **Postgres + pgvector 实现**
```go
// internal/rag/pgvector_store.go
type PgVectorStore struct {
    db *gorm.DB
}

// Search 向量检索
func (s *PgVectorStore) Search(ctx context.Context, knowledgeBaseIDs []string, query VectorQuery) ([]ScoredChunk, error) {
    var results []ScoredChunk
    
    sql := `
        SELECT 
            kc.id,
            kc.document_id,
            kc.content,
            kc.metadata,
            1 - (kc.embedding <=> $1) AS score
        FROM knowledge_chunks kc
        JOIN knowledge_documents kd ON kc.document_id = kd.id
        WHERE kd.knowledge_base_id = ANY($2)
          AND kd.status = 'indexed'
          AND 1 - (kc.embedding <=> $1) >= $3
        ORDER BY kc.embedding <=> $1
        LIMIT $4
    `
    
    err := s.db.Raw(sql, 
        pgvector.NewVector(query.QueryVector),
        pq.Array(knowledgeBaseIDs),
        query.ScoreThreshold,
        query.TopK,
    ).Scan(&results).Error
    
    return results, err
}
```

### **3. RAG 增强 Agent** (1 天)

```go
// internal/agent/runtime/researcher_agent.go
type ResearcherAgent struct {
    config        *AgentConfig
    modelClient   ai.ModelClient
    vectorStore   rag.VectorStore
    embeddingClient rag.EmbeddingProvider
}

func (a *ResearcherAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
    // 1. 向量化查询
    queryEmbedding, err := a.embeddingClient.EmbedTexts(ctx, "text-embedding-3-small", []string{input.Content})
    if err != nil {
        return nil, err
    }
    
    // 2. 检索相关知识
    chunks, err := a.vectorStore.Search(ctx, input.KnowledgeBaseIDs, rag.VectorQuery{
        QueryVector:    queryEmbedding[0],
        TopK:           10,
        ScoreThreshold: 0.7,
    })
    if err != nil {
        return nil, err
    }
    
    // 3. 构建增强 Prompt
    context := buildContextFromChunks(chunks)
    prompt := fmt.Sprintf(`基于以下知识回答问题：

知识上下文：
%s

问题：%s

请基于上述知识给出专业回答，并标注引用来源。`, context, input.Content)
    
    // 4. 调用 AI 模型
    resp, err := a.modelClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
        Messages: []ai.Message{{Role: "user", Content: prompt}},
    })
    
    return &AgentResult{
        Output:   resp.Content,
        Metadata: map[string]any{"retrieved_chunks": len(chunks)},
    }, nil
}
```

---

## 🛠️ **Sprint 7: 工具调用 + 监控可观测性** (优先级: P1)

### **1. Tool 执行引擎** (2-3 天)

#### **工具类型**
- **HTTP 工具**：调用外部 API（搜索引擎、天气、翻译等）
- **数据库工具**：查询数据库
- **Python 工具**：执行 Python 脚本（沙箱环境）
- **自定义工具**：用户自定义函数

#### **实现**
```go
// internal/tool/executor.go
type ToolExecutor struct {
    db          *gorm.DB
    httpClient  *http.Client
    pythonPool  *PythonWorkerPool // 可选
}

func (e *ToolExecutor) Execute(ctx context.Context, toolID string, input map[string]any) (any, error) {
    // 1. 加载工具配置
    tool, err := e.loadTool(ctx, toolID)
    if err != nil {
        return nil, err
    }
    
    // 2. 根据类型执行
    switch tool.ImplType {
    case "http":
        return e.executeHTTP(ctx, tool, input)
    case "database":
        return e.executeDatabase(ctx, tool, input)
    case "python":
        return e.executePython(ctx, tool, input)
    default:
        return nil, fmt.Errorf("不支持的工具类型: %s", tool.ImplType)
    }
}

// Function Calling 集成
func (a *BaseAgent) ExecuteWithTools(ctx context.Context, input *AgentInput, tools []Tool) (*AgentResult, error) {
    // 1. 构建 Function Calling Prompt
    req := &ai.ChatCompletionRequest{
        Messages: []ai.Message{{Role: "user", Content: input.Content}},
        Tools:    convertTools(tools),
    }
    
    // 2. 首次调用
    resp, err := a.modelClient.ChatCompletion(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // 3. 如果有工具调用
    if resp.ToolCalls != nil {
        for _, toolCall := range resp.ToolCalls {
            // 执行工具
            result, err := a.toolExecutor.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
            if err != nil {
                return nil, err
            }
            
            // 添加工具结果到对话
            req.Messages = append(req.Messages, ai.Message{
                Role:    "tool",
                Content: fmt.Sprintf("%v", result),
                ToolCallID: toolCall.ID,
            })
        }
        
        // 4. 再次调用获取最终答案
        resp, err = a.modelClient.ChatCompletion(ctx, req)
    }
    
    return &AgentResult{Output: resp.Content}, nil
}
```

### **2. Prometheus 监控** (1-2 天)

#### **核心指标**
```go
// internal/infra/metrics.go
var (
    // HTTP 请求指标
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )
    
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )
    
    // AI 模型调用指标
    aiModelCallsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_model_calls_total",
            Help: "Total number of AI model calls",
        },
        []string{"provider", "model", "status"},
    )
    
    aiModelCallDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "ai_model_call_duration_seconds",
            Help:    "AI model call duration in seconds",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
        },
        []string{"provider", "model"},
    )
    
    aiModelTokensUsed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_model_tokens_used_total",
            Help: "Total tokens used by AI models",
        },
        []string{"provider", "model", "type"}, // type: prompt/completion
    )
    
    // 工作流指标
    workflowExecutionsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "workflow_executions_total",
            Help: "Total number of workflow executions",
        },
        []string{"workflow_id", "status"},
    )
    
    workflowExecutionDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "workflow_execution_duration_seconds",
            Help:    "Workflow execution duration in seconds",
            Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
        },
        []string{"workflow_id"},
    )
)
```

#### **Prometheus 端点**
```go
// cmd/server/main.go
import "github.com/prometheus/client_golang/prometheus/promhttp"

func main() {
    // ... 其他初始化 ...
    
    // Prometheus metrics endpoint
    router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
```

### **3. 分布式 Tracing** (1 天)

使用 OpenTelemetry 实现全链路追踪。

```go
// internal/infra/tracing.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracer(serviceName string) (*trace.TracerProvider, error) {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint())
    if err != nil {
        return nil, err
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}

// 使用示例
func (a *WriterAgent) Execute(ctx context.Context, input *AgentInput) (*AgentResult, error) {
    ctx, span := otel.Tracer("agent").Start(ctx, "WriterAgent.Execute")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("agent.type", "writer"),
        attribute.String("model", a.config.ModelID),
    )
    
    // ... 执行逻辑 ...
}
```

---

## 📊 **综合对比：改进前 vs 改进后**

| 维度 | 当前状态 | Sprint 4-7 完成后 | 对标项目 |
|------|---------|-----------------|---------|
| **提供商数量** | 2 (OpenAI/Claude) | 10+ (Gemini/Azure/DeepSeek/Qwen/Ollama) | new-api: 50+ |
| **格式兼容** | 仅 OpenAI 格式 | 支持 OpenAI/Claude/Gemini 互转 | new-api: ✅ |
| **模型发现** | 手动配置 | 自动同步 + 定时更新 | cherry-studio: ✅ |
| **认证系统** | Mock | JWT + OAuth2 + SSO | 生产级 |
| **RAG 功能** | 未实现 | 完整实现（导入/检索/增强） | 企业级 |
| **工具调用** | 未实现 | HTTP/DB/Python + Function Calling | 生产级 |
| **监控** | 无 | Prometheus + Tracing + 错误监控 | 生产级 |
| **API 端点** | 39 个 | 60+ 个 | - |

---

## ⏱️ **总体工期估算**

| Sprint | 主要任务 | 工期 | 工作量 |
|--------|---------|------|--------|
| **Sprint 4** | 多提供商 + 格式转换 + 模型发现 | 9-13 天 | 中等 |
| **Sprint 5** | JWT + OAuth2 + 审计增强 | 3-4 天 | 较小 |
| **Sprint 6** | RAG 全链路实现 | 5-6 天 | 中等 |
| **Sprint 7** | 工具调用 + 监控 | 4-6 天 | 中等 |
| **总计** | - | **21-29 天** | **约 3-4 周** |

---

## 🎯 **核心价值与收益**

完成 Sprint 4-7 后，项目将获得：

1. **✅ 对标 new-api 的多提供商网关能力**
2. **✅ 对标 cherry-studio 的模型管理能力**
3. **✅ 生产级认证授权系统**
4. **✅ 完整的 RAG 知识库功能**
5. **✅ 工具调用与 Function Calling 支持**
6. **✅ 企业级可观测性（监控/追踪/告警）**

---

## 📝 **后续建议**

### **Sprint 8-10（可选扩展）**
- **前端控制台**：React + TypeScript 管理后台
- **工作流可视化编辑器**：拖拽式配置
- **成本中心**：按租户/用户/模型统计成本
- **性能优化**：连接池、缓存、异步处理
- **安全加固**：API 限流、DDoS 防护、加密存储

---

是否开始实施 Sprint 4 的改进计划？我建议按以下优先级推进：

**Phase 1 (必须)**：Sprint 4 + Sprint 5
**Phase 2 (重要)**：Sprint 6
**Phase 3 (优化)**：Sprint 7

请确认优先级或调整方案。