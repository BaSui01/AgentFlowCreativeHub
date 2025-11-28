# 新增后端功能 API 说明

> 创建日期：2025-11-28
> 状态：待集成到路由

本文档列出了所有新实现的后端功能及其对应的 API 接口，为前端开发提供接口规范。

---

## 📋 目录

1. [片段管理系统](#1-片段管理系统)
2. [多模型抽卡机制](#2-多模型抽卡机制)
3. [剧情推演系统](#3-剧情推演系统)
4. [世界观侧边栏](#4-世界观侧边栏)
5. [路由注册说明](#5-路由注册说明)

---

## 1. 片段管理系统

**功能说明**: 管理灵感片段、素材、待办事项、笔记等内容片段

**实现文件**:
- `backend/internal/fragment/models.go`
- `backend/internal/fragment/service.go`
- `backend/api/handlers/fragment/handler.go`

### 1.1 数据模型

**片段类型** (`FragmentType`):
- `inspiration` - 灵感片段
- `material` - 素材片段
- `todo` - 待办事项
- `note` - 笔记
- `reference` - 参考资料

**片段状态** (`FragmentStatus`):
- `pending` - 待处理
- `completed` - 已完成
- `archived` - 已归档

### 1.2 API 接口

#### POST `/api/fragments`
**创建片段**

请求体:
```json
{
  "type": "inspiration",
  "title": "新剧情灵感",
  "content": "主角在关键时刻觉醒新能力...",
  "tags": ["剧情", "能力"],
  "priority": 3,
  "work_id": "作品ID",
  "chapter_id": "章节ID"
}
```

#### GET `/api/fragments`
**查询片段列表**

查询参数:
- `type` - 片段类型
- `status` - 片段状态
- `workspace_id` - 工作空间ID
- `work_id` - 作品ID
- `chapter_id` - 章节ID
- `tags` - 标签（逗号分隔）
- `keyword` - 关键词搜索
- `page` - 页码
- `page_size` - 每页数量
- `sort_by` - 排序字段（created_at, priority, due_date）
- `sort_order` - 排序方向（asc, desc）

#### GET `/api/fragments/:id`
**获取片段详情**

#### PUT `/api/fragments/:id`
**更新片段**

请求体:
```json
{
  "title": "更新后的标题",
  "content": "更新后的内容",
  "status": "completed",
  "priority": 5
}
```

#### DELETE `/api/fragments/:id`
**删除片段**（软删除）

#### POST `/api/fragments/:id/complete`
**标记片段为已完成**（用于待办事项）

#### POST `/api/fragments/batch`
**批量操作**

请求体:
```json
{
  "ids": ["id1", "id2", "id3"],
  "operation": "complete",  // complete, archive, delete, change_status
  "status": "completed"     // 当 operation=change_status 时必填
}
```

#### GET `/api/fragments/stats`
**获取片段统计信息**

响应示例:
```json
{
  "total_count": 100,
  "by_type": {
    "inspiration": 30,
    "material": 20,
    "todo": 40,
    "note": 10
  },
  "by_status": {
    "pending": 60,
    "completed": 30,
    "archived": 10
  },
  "pending_todos": 40,
  "overdue_todos": 5,
  "completed_today": 8
}
```

---

## 2. 多模型抽卡机制

**功能说明**: 同时调用多个 AI 模型生成内容并对比结果，类似"抽卡"机制

**实现文件**:
- `backend/internal/multimodel/models.go`
- `backend/internal/multimodel/service.go`
- `backend/api/handlers/multimodel/handler.go`

### 2.1 API 接口

#### POST `/api/multimodel/draw`
**执行多模型抽卡**

请求体:
```json
{
  "model_ids": ["model1", "model2", "model3"],
  "agent_type": "plot",
  "content": "生成后续剧情分支",
  "extra_params": {
    "current_plot": "当前剧情内容",
    "num_branches": 3
  },
  "temperature": 0.8,
  "max_tokens": 2000
}
```

响应示例:
```json
{
  "draw_id": "draw-uuid",
  "results": [
    {
      "model_id": "model1",
      "model_name": "GPT-4",
      "provider": "openai",
      "content": "生成的内容...",
      "success": true,
      "latency_ms": 2500,
      "token_usage": {
        "prompt_tokens": 100,
        "completion_tokens": 500,
        "total_tokens": 600
      },
      "score": 85.5
    }
  ],
  "best_model_id": "model1",
  "total_time_ms": 3000,
  "success_rate": 100
}
```

#### GET `/api/multimodel/draws`
**查询抽卡历史列表**

查询参数:
- `agent_type` - Agent类型
- `page` - 页码
- `page_size` - 每页数量

#### GET `/api/multimodel/draws/:id`
**获取抽卡历史详情**

#### POST `/api/multimodel/regenerate`
**重新生成单个模型结果**

请求体:
```json
{
  "draw_id": "抽卡ID",
  "model_id": "要重新生成的模型ID"
}
```

#### DELETE `/api/multimodel/draws/:id`
**删除抽卡历史**

#### GET `/api/multimodel/stats`
**获取抽卡统计**

响应示例:
```json
{
  "total_draws": 50,
  "by_agent_type": [
    {"agent_type": "plot", "count": 30},
    {"agent_type": "writer", "count": 20}
  ],
  "avg_success_rate": 95.5,
  "popular_models": [
    {"model_id": "gpt-4", "count": 80},
    {"model_id": "claude-3-5-sonnet", "count": 60}
  ]
}
```

---

## 3. 剧情推演系统

**功能说明**: 基于当前剧情生成多个后续剧情分支，并支持一键应用到章节

**实现文件**:
- `backend/internal/plot/models.go`
- `backend/internal/plot/service.go`
- `backend/api/handlers/plot/handler.go`

### 3.1 数据模型

**剧情分支** (`PlotBranch`):
```json
{
  "id": 1,
  "title": "分支标题",
  "summary": "简要概述",
  "key_events": ["事件1", "事件2"],
  "emotional_tone": "情感基调（爽/虐/治愈）",
  "hook": "悬念/爽点",
  "difficulty": 3
}
```

### 3.2 API 接口

#### POST `/api/plot/recommendations`
**创建剧情推演**

请求体:
```json
{
  "title": "第十章后续剧情推演",
  "current_plot": "当前剧情内容...",
  "character_info": "角色信息...",
  "world_setting": "世界观设定...",
  "num_branches": 3,
  "model_id": "gpt-4",
  "work_id": "作品ID",
  "chapter_id": "章节ID"
}
```

响应示例:
```json
{
  "id": "plot-uuid",
  "title": "第十章后续剧情推演",
  "current_plot": "...",
  "branches": "[...]",
  "parsed_branches": [
    {
      "id": 1,
      "title": "主角突破修为",
      "summary": "主角在关键时刻突破...",
      "key_events": ["突破", "击败敌人", "获得宝物"],
      "emotional_tone": "爽",
      "hook": "强者震撼",
      "difficulty": 3
    }
  ],
  "applied": false
}
```

#### GET `/api/plot/recommendations`
**查询剧情推演列表**

查询参数:
- `workspace_id` - 工作空间ID
- `work_id` - 作品ID
- `chapter_id` - 章节ID
- `applied` - 是否已应用
- `page` - 页码
- `page_size` - 每页数量

#### GET `/api/plot/recommendations/:id`
**获取剧情推演详情**

#### PUT `/api/plot/recommendations/:id`
**更新剧情推演**

请求体:
```json
{
  "title": "更新后的标题",
  "selected_branch": 1  // 选择的分支索引
}
```

#### DELETE `/api/plot/recommendations/:id`
**删除剧情推演**

#### POST `/api/plot/apply`
**应用剧情到章节**（一键应用）

请求体:
```json
{
  "plot_id": "剧情推演ID",
  "chapter_id": "章节ID",
  "branch_index": 1,
  "append_content": true  // true=追加, false=替换
}
```

#### GET `/api/plot/stats`
**获取剧情推演统计**

响应示例:
```json
{
  "total_count": 50,
  "applied_count": 30,
  "apply_rate": 60
}
```

---

## 4. 世界观侧边栏

**功能说明**: 快速查阅当前作品的世界观设定实体（角色、地点、物品等）

**实现文件**:
- 扩展了 `backend/internal/worldbuilder/service.go`
- 扩展了 `backend/internal/worldbuilder/models.go`
- 扩展了 `backend/api/handlers/worldbuilder/handler.go`

### 4.1 API 接口

#### GET `/api/worldbuilder/sidebar/summary`
**获取作品设定摘要**（用于侧边栏显示）

查询参数:
- `workId` - 作品ID（必填）

响应示例:
```json
{
  "work_id": "work-uuid",
  "setting_id": "setting-uuid",
  "setting_name": "修仙世界设定",
  "entities": [
    {
      "id": "entity1",
      "type": "character",
      "name": "李云",
      "description": "主角，天才修士",
      "category": "主角"
    },
    {
      "id": "entity2",
      "type": "location",
      "name": "青云宗",
      "description": "主角所在门派",
      "category": "门派"
    }
  ]
}
```

#### GET `/api/worldbuilder/sidebar/search`
**搜索作品中的设定实体**

查询参数:
- `workId` - 作品ID（必填）
- `keyword` - 关键词
- `type` - 实体类型（character, location, item, skill, organization等）

响应示例:
```json
[
  {
    "id": "entity1",
    "setting_id": "setting-uuid",
    "tenant_id": "tenant-uuid",
    "name": "李云",
    "type": "character",
    "category": "主角",
    "description": "天才修士...",
    "attributes": "{...}",
    "tags": ["主角", "修士"],
    "created_at": "2025-11-28T..."
  }
]
```

---

## 5. 路由注册说明

所有新增的 API 接口需要在 `backend/api/routes.go` 中注册。

### 5.1 添加 Handler 初始化

在 `backend/api/wire.go` 的 `Handlers` 结构体中添加:

```go
type Handlers struct {
	// ... 现有字段 ...
	
	// 新增
	Fragment    *fragment.Handler
	MultiModel  *multimodel.Handler
	Plot        *plot.Handler
}
```

在 `InitHandlers` 函数中初始化:

```go
func (c *AppContainer) InitHandlers() *Handlers {
	return &Handlers{
		// ... 现有初始化 ...
		
		// 新增
		Fragment:   fragment.NewHandler(c.FragmentService),
		MultiModel: multimodel.NewHandler(c.MultiModelService),
		Plot:       plot.NewHandler(c.PlotService),
	}
}
```

### 5.2 注册路由

在 `backend/api/routes.go` 中的 `registerAPIRoutes` 函数添加:

```go
// 片段管理
registerFragmentRoutes(apiGroup, h)

// 多模型抽卡
registerMultiModelRoutes(apiGroup, h)

// 剧情推演
registerPlotRoutes(apiGroup, h)
```

创建对应的路由注册函数:

```go
// 片段管理路由
func registerFragmentRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	if h.Fragment == nil {
		return
	}
	
	fragmentGroup := apiGroup.Group("/fragments")
	{
		fragmentGroup.POST("", h.Fragment.CreateFragment)
		fragmentGroup.GET("", h.Fragment.ListFragments)
		fragmentGroup.GET("/:id", h.Fragment.GetFragment)
		fragmentGroup.PUT("/:id", h.Fragment.UpdateFragment)
		fragmentGroup.DELETE("/:id", h.Fragment.DeleteFragment)
		fragmentGroup.POST("/:id/complete", h.Fragment.CompleteFragment)
		fragmentGroup.POST("/batch", h.Fragment.BatchOperation)
		fragmentGroup.GET("/stats", h.Fragment.GetStats)
	}
}

// 多模型抽卡路由
func registerMultiModelRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	if h.MultiModel == nil {
		return
	}
	
	multiModelGroup := apiGroup.Group("/multimodel")
	{
		multiModelGroup.POST("/draw", h.MultiModel.Draw)
		multiModelGroup.GET("/draws", h.MultiModel.ListDrawHistory)
		multiModelGroup.GET("/draws/:id", h.MultiModel.GetDrawHistory)
		multiModelGroup.DELETE("/draws/:id", h.MultiModel.DeleteDrawHistory)
		multiModelGroup.POST("/regenerate", h.MultiModel.Regenerate)
		multiModelGroup.GET("/stats", h.MultiModel.GetStats)
	}
}

// 剧情推演路由
func registerPlotRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	if h.Plot == nil {
		return
	}
	
	plotGroup := apiGroup.Group("/plot")
	{
		plotGroup.POST("/recommendations", h.Plot.CreatePlotRecommendation)
		plotGroup.GET("/recommendations", h.Plot.ListPlotRecommendations)
		plotGroup.GET("/recommendations/:id", h.Plot.GetPlotRecommendation)
		plotGroup.PUT("/recommendations/:id", h.Plot.UpdatePlotRecommendation)
		plotGroup.DELETE("/recommendations/:id", h.Plot.DeletePlotRecommendation)
		plotGroup.POST("/apply", h.Plot.ApplyPlotToChapter)
		plotGroup.GET("/stats", h.Plot.GetStats)
	}
}
```

在现有的 `registerWorldBuilderRoutes` 函数中添加侧边栏路由:

```go
func registerWorldBuilderRoutes(apiGroup *gin.RouterGroup, h *Handlers) {
	// ... 现有路由 ...
	
	// 侧边栏快速查阅
	sidebarGroup := apiGroup.Group("/worldbuilder/sidebar")
	{
		sidebarGroup.GET("/summary", h.WorldBuilder.GetWorkSettingsSummary)
		sidebarGroup.GET("/search", h.WorldBuilder.SearchEntitiesInWork)
	}
}
```

### 5.3 添加服务依赖

在 `backend/api/wire.go` 的 `AppContainer` 中添加:

```go
type AppContainer struct {
	// ... 现有字段 ...
	
	// 新增服务
	FragmentService    *fragment.Service
	MultiModelService  *multimodel.Service
	PlotService        *plot.Service
}
```

在 `InitContainer` 函数中初始化服务:

```go
// 片段管理服务
FragmentService: fragment.NewService(db),

// 多模型服务
MultiModelService: multimodel.NewService(db, agentRegistry, modelService),

// 剧情推演服务
PlotService: plot.NewService(db, agentRegistry, workspaceService),
```

### 5.4 数据库迁移

需要创建数据库迁移脚本来添加新表：

```sql
-- backend/migrations/0013_fragment_multimodel_plot.sql

-- 片段表
CREATE TABLE IF NOT EXISTS fragments (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    workspace_id VARCHAR(36),
    work_id VARCHAR(36),
    chapter_id VARCHAR(36),
    type VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,
    priority INT DEFAULT 0,
    due_date TIMESTAMP,
    completed_at TIMESTAMP,
    metadata JSONB,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_fragments_tenant ON fragments(tenant_id);
CREATE INDEX idx_fragments_user ON fragments(user_id);
CREATE INDEX idx_fragments_type ON fragments(type);
CREATE INDEX idx_fragments_status ON fragments(status);

-- 抽卡历史表
CREATE TABLE IF NOT EXISTS draw_histories (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    agent_type VARCHAR(50) NOT NULL,
    model_ids TEXT NOT NULL,
    input_prompt TEXT NOT NULL,
    results JSONB NOT NULL,
    best_model_id VARCHAR(36),
    total_time_ms BIGINT,
    success_rate FLOAT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_draw_histories_tenant ON draw_histories(tenant_id);
CREATE INDEX idx_draw_histories_user ON draw_histories(user_id);

-- 剧情推演表
CREATE TABLE IF NOT EXISTS plot_recommendations (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    workspace_id VARCHAR(36),
    work_id VARCHAR(36),
    chapter_id VARCHAR(36),
    title VARCHAR(200) NOT NULL,
    current_plot TEXT NOT NULL,
    character_info TEXT,
    world_setting TEXT,
    branches JSONB NOT NULL,
    selected_branch INT,
    applied BOOLEAN DEFAULT false,
    applied_at TIMESTAMP,
    model_id VARCHAR(36) NOT NULL,
    agent_id VARCHAR(36),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_plot_recommendations_tenant ON plot_recommendations(tenant_id);
CREATE INDEX idx_plot_recommendations_user ON plot_recommendations(user_id);
CREATE INDEX idx_plot_recommendations_work ON plot_recommendations(work_id);
CREATE INDEX idx_plot_recommendations_applied ON plot_recommendations(applied);
```

---

## 6. 前端集成建议

### 6.1 片段管理

**使用场景**:
- 创作时随时记录灵感
- 收集素材和参考资料
- 管理待办事项

**UI 建议**:
- 侧边栏显示片段列表
- 支持拖拽排序
- 待办事项支持勾选完成
- 支持标签筛选和搜索

### 6.2 多模型抽卡

**使用场景**:
- 剧情推演时对比不同模型的生成结果
- 重要内容生成时多次尝试选择最佳结果

**UI 建议**:
- 卡片式展示多个结果
- 显示每个模型的评分和耗时
- 支持单独重新生成某个模型
- 保存抽卡历史供后续查看

### 6.3 剧情推演

**使用场景**:
- 写作时生成后续剧情选项
- 快速选择并应用到章节

**UI 建议**:
- 展示多个剧情分支供选择
- 显示每个分支的情感基调和难度
- 一键应用按钮
- 保存推演历史

### 6.4 世界观侧边栏

**使用场景**:
- 写作时快速查阅角色设定
- 搜索特定设定实体

**UI 建议**:
- 侧边栏常驻显示
- 按类型分组显示（角色/地点/物品等）
- 支持快速搜索
- 点击实体显示详情

---

## 7. 测试建议

### 7.1 单元测试

为每个服务创建测试文件：
- `backend/internal/fragment/service_test.go`
- `backend/internal/multimodel/service_test.go`
- `backend/internal/plot/service_test.go`

### 7.2 集成测试

测试完整的 API 调用流程：
1. 创建片段 -> 查询 -> 更新 -> 删除
2. 多模型抽卡 -> 查看历史 -> 重新生成
3. 剧情推演 -> 应用到章节 -> 验证内容更新
4. 查询世界观摘要 -> 搜索实体

### 7.3 性能测试

关注点：
- 多模型抽卡的并发性能
- 剧情推演的响应时间
- 片段列表查询的分页性能

---

## 8. 后续优化建议

1. **片段管理**
   - 添加片段分类功能
   - 支持片段导出为 Markdown
   - 添加片段提醒功能

2. **多模型抽卡**
   - 支持更多评分维度
   - 添加成本对比
   - 支持批量抽卡

3. **剧情推演**
   - 剧情树可视化
   - 支持多轮推演
   - 添加剧情评分机制

4. **世界观侧边栏**
   - 关系图可视化
   - 智能推荐相关设定
   - 支持快速编辑

---

## 📝 更新日志

### 2025-11-28
- ✅ 实现片段管理系统（8个API）
- ✅ 实现多模型抽卡机制（6个API）
- ✅ 实现剧情推演系统（7个API）
- ✅ 扩展世界观侧边栏（2个API）
- 📝 编写完整的 API 文档

---

## 🎯 总结

本次新增功能共计实现了 **23 个 API 接口**，覆盖了小说创作流程中的关键场景：

1. **片段管理** - 8个接口，支持灵感收集和待办管理
2. **多模型抽卡** - 6个接口，支持AI结果对比选择
3. **剧情推演** - 7个接口，支持剧情分支生成和应用
4. **世界观侧边栏** - 2个接口，支持快速查阅设定

所有功能均已实现完整的后端逻辑，等待：
1. 集成到 API 路由
2. 创建数据库迁移脚本
3. 前端界面开发

**建议优先级**:
1. P0: 剧情推演系统（核心创作功能）
2. P1: 片段管理系统（辅助创作）
3. P1: 世界观侧边栏（提升体验）
4. P2: 多模型抽卡（高级功能）
