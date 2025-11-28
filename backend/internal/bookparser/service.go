package bookparser

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"backend/internal/agent/runtime"
	"backend/internal/rag"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service 拆书服务
type Service struct {
	db            *gorm.DB
	agentRegistry *runtime.Registry
	ragService    *rag.RAGService
	httpClient    *http.Client
}

// NewService 创建拆书服务
func NewService(db *gorm.DB, agentRegistry *runtime.Registry, ragService *rag.RAGService) *Service {
	return &Service{
		db:            db,
		agentRegistry: agentRegistry,
		ragService:    ragService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AutoMigrate 自动迁移表结构
func (s *Service) AutoMigrate() error {
	return s.db.AutoMigrate(&BookParserTask{}, &BookParserResult{}, &BookKnowledge{})
}

// CreateTask 创建拆书任务
func (s *Service) CreateTask(ctx context.Context, req *CreateTaskRequest) (*BookParserTask, error) {
	// 获取内容
	content := req.Content
	if req.SourceType == "url" && req.SourceURL != "" {
		fetchedContent, err := s.fetchFromURL(ctx, req.SourceURL)
		if err != nil {
			return nil, fmt.Errorf("抓取内容失败: %w", err)
		}
		content = fetchedContent
	}

	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("内容不能为空")
	}

	// 计算内容哈希
	hash := md5.Sum([]byte(content))
	contentHash := hex.EncodeToString(hash[:])

	// 设置默认维度
	dimensions := req.Dimensions
	if len(dimensions) == 0 {
		dimensions = []AnalysisDimension{DimensionAll}
	}

	dimensionsJSON, _ := json.Marshal(dimensions)

	task := &BookParserTask{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		UserID:      req.UserID,
		Title:       req.Title,
		SourceType:  req.SourceType,
		SourceURL:   req.SourceURL,
		Content:     content,
		ContentHash: contentHash,
		Dimensions:  dimensionsJSON,
		ModelID:     req.ModelID,
		Status:      TaskStatusPending,
	}

	if err := s.db.WithContext(ctx).Create(task).Error; err != nil {
		return nil, err
	}

	// 异步启动分析任务
	go s.processTask(context.Background(), task.ID)

	return task, nil
}

// GetTask 获取任务详情
func (s *Service) GetTask(ctx context.Context, tenantID, taskID string) (*BookParserTask, error) {
	var task BookParserTask
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", taskID, tenantID).
		First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// ListTasks 列出任务
func (s *Service) ListTasks(ctx context.Context, req *TaskListRequest) ([]BookParserTask, int64, error) {
	var tasks []BookParserTask
	var total int64

	query := s.db.WithContext(ctx).Model(&BookParserTask{}).
		Where("tenant_id = ?", req.TenantID)

	if req.UserID != "" {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := req.Page, req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// GetTaskProgress 获取任务进度
func (s *Service) GetTaskProgress(ctx context.Context, tenantID, taskID string) (*TaskProgress, error) {
	var task BookParserTask
	if err := s.db.WithContext(ctx).
		Select("id, status, progress, total_chunks, done_chunks, tokens_used, error_msg").
		Where("id = ? AND tenant_id = ?", taskID, tenantID).
		First(&task).Error; err != nil {
		return nil, err
	}

	return &TaskProgress{
		TaskID:      task.ID,
		Status:      task.Status,
		Progress:    task.Progress,
		TotalChunks: task.TotalChunks,
		DoneChunks:  task.DoneChunks,
		TokensUsed:  task.TokensUsed,
		ErrorMsg:    task.ErrorMsg,
	}, nil
}

// GetTaskResults 获取任务分析结果
func (s *Service) GetTaskResults(ctx context.Context, tenantID, taskID string) (*AnalysisResult, error) {
	task, err := s.GetTask(ctx, tenantID, taskID)
	if err != nil {
		return nil, err
	}

	// 获取分析结果
	var results []BookParserResult
	if err := s.db.WithContext(ctx).
		Where("task_id = ? AND tenant_id = ?", taskID, tenantID).
		Find(&results).Error; err != nil {
		return nil, err
	}

	// 获取提取的知识
	var knowledge []BookKnowledge
	if err := s.db.WithContext(ctx).
		Where("task_id = ? AND tenant_id = ?", taskID, tenantID).
		Find(&knowledge).Error; err != nil {
		return nil, err
	}

	// 聚合结果
	resultMap := make(map[AnalysisDimension]any)
	var summaries []string

	for _, r := range results {
		var analysis DimensionAnalysis
		if err := json.Unmarshal(r.Analysis, &analysis); err == nil {
			resultMap[r.Dimension] = analysis
			if r.Summary != "" {
				summaries = append(summaries, r.Summary)
			}
		}
	}

	return &AnalysisResult{
		TaskID:    task.ID,
		Title:     task.Title,
		Status:    task.Status,
		Results:   resultMap,
		Knowledge: knowledge,
		Summary:   strings.Join(summaries, "\n\n"),
	}, nil
}

// CancelTask 取消任务
func (s *Service) CancelTask(ctx context.Context, tenantID, taskID string) error {
	return s.db.WithContext(ctx).
		Model(&BookParserTask{}).
		Where("id = ? AND tenant_id = ? AND status IN ?", taskID, tenantID, []TaskStatus{TaskStatusPending, TaskStatusRunning}).
		Update("status", TaskStatusCancelled).Error
}

// SearchKnowledge 搜索知识库
func (s *Service) SearchKnowledge(ctx context.Context, req *SearchKnowledgeRequest) ([]BookKnowledge, error) {
	query := s.db.WithContext(ctx).Model(&BookKnowledge{}).
		Where("tenant_id = ?", req.TenantID)

	// 关键词搜索
	if req.Query != "" {
		pattern := "%" + strings.ToLower(req.Query) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(technique) LIKE ?",
			pattern, pattern, pattern)
	}

	// 维度过滤
	if req.Dimension != "" {
		query = query.Where("dimension = ?", req.Dimension)
	}

	// 分类过滤
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}

	// 标签过滤
	if len(req.Tags) > 0 {
		for _, tag := range req.Tags {
			query = query.Where("tags @> ?", fmt.Sprintf(`["%s"]`, tag))
		}
	}

	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var results []BookKnowledge
	if err := query.Order("use_count DESC, created_at DESC").
		Limit(limit).
		Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

// processTask 处理分析任务（异步）
func (s *Service) processTask(ctx context.Context, taskID string) {
	// 获取任务
	var task BookParserTask
	if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		return
	}

	// 更新状态为运行中
	now := time.Now()
	s.db.Model(&task).Updates(map[string]any{
		"status":     TaskStatusRunning,
		"started_at": &now,
	})

	// 解析要分析的维度
	var dimensions []AnalysisDimension
	if err := json.Unmarshal(task.Dimensions, &dimensions); err != nil {
		s.failTask(&task, "解析维度配置失败: "+err.Error())
		return
	}

	// 如果是全部维度，展开为具体维度
	if len(dimensions) == 1 && dimensions[0] == DimensionAll {
		dimensions = []AnalysisDimension{
			DimensionStyle,
			DimensionPlot,
			DimensionCharacter,
			DimensionEmotion,
			DimensionMeme,
			DimensionOutline,
		}
	}

	totalChunks := len(dimensions)
	s.db.Model(&task).Update("total_chunks", totalChunks)

	totalTokens := 0
	doneChunks := 0

	// 对每个维度进行分析
	for _, dim := range dimensions {
		// 检查任务是否被取消
		var currentTask BookParserTask
		if err := s.db.Where("id = ?", taskID).First(&currentTask).Error; err != nil {
			return
		}
		if currentTask.Status == TaskStatusCancelled {
			return
		}

		// 执行分析
		result, err := s.analyzeDimension(ctx, &task, dim)
		if err != nil {
			// 记录错误但继续处理其他维度
			s.db.Create(&BookParserResult{
				ID:        uuid.New().String(),
				TaskID:    task.ID,
				TenantID:  task.TenantID,
				Dimension: dim,
				Summary:   "分析失败: " + err.Error(),
			})
		} else {
			// 保存结果
			s.db.Create(result)
			totalTokens += result.TokensUsed

			// 提取知识点
			s.extractKnowledge(ctx, &task, result)
		}

		doneChunks++
		progress := (doneChunks * 100) / totalChunks

		s.db.Model(&task).Updates(map[string]any{
			"done_chunks": doneChunks,
			"progress":    progress,
			"tokens_used": totalTokens,
		})
	}

	// 完成任务
	completedAt := time.Now()
	s.db.Model(&task).Updates(map[string]any{
		"status":       TaskStatusCompleted,
		"completed_at": &completedAt,
		"progress":     100,
	})
}

// analyzeDimension 分析单个维度
func (s *Service) analyzeDimension(ctx context.Context, task *BookParserTask, dim AnalysisDimension) (*BookParserResult, error) {
	// 获取 Analyzer Agent
	agent, err := s.agentRegistry.GetAgentByType(ctx, task.TenantID, "analyzer")
	if err != nil {
		return nil, fmt.Errorf("获取分析Agent失败: %w", err)
	}

	// 构建分析输入
	input := &runtime.AgentInput{
		Content: task.Content,
		Context: &runtime.AgentContext{
			TenantID: task.TenantID,
			UserID:   task.UserID,
		},
		ExtraParams: map[string]any{
			"dimensions":    string(dim),
			"analysis_type": string(dim),
			"format":        "json",
		},
	}

	// 根据维度设置不同的系统提示
	systemPrompt := s.getDimensionPrompt(dim)
	input.ExtraParams["system_prompt_override"] = systemPrompt

	start := time.Now()
	result, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	latency := time.Since(start).Milliseconds()

	// 解析输出
	var analysis DimensionAnalysis
	analysis.Dimension = dim

	// 尝试解析 JSON 输出
	if err := json.Unmarshal([]byte(result.Output), &analysis); err != nil {
		// 如果不是 JSON，直接作为摘要
		analysis.Summary = result.Output
	}

	analysisJSON, _ := json.Marshal(analysis)

	tokensUsed := 0
	modelUsed := ""
	if result.Usage != nil {
		tokensUsed = result.Usage.TotalTokens
	}
	if result.Metadata != nil {
		if m, ok := result.Metadata["model_id"].(string); ok {
			modelUsed = m
		}
	}

	return &BookParserResult{
		ID:         uuid.New().String(),
		TaskID:     task.ID,
		TenantID:   task.TenantID,
		Dimension:  dim,
		Analysis:   analysisJSON,
		Summary:    analysis.Summary,
		ModelUsed:  modelUsed,
		TokensUsed: tokensUsed,
		LatencyMs:  latency,
	}, nil
}

// extractKnowledge 从分析结果提取知识点
func (s *Service) extractKnowledge(ctx context.Context, task *BookParserTask, result *BookParserResult) {
	var analysis DimensionAnalysis
	if err := json.Unmarshal(result.Analysis, &analysis); err != nil {
		return
	}

	for _, finding := range analysis.Findings {
		if finding.Technique == "" && finding.Observation == "" {
			continue
		}

		knowledge := &BookKnowledge{
			ID:          uuid.New().String(),
			TenantID:    task.TenantID,
			TaskID:      task.ID,
			SourceTitle: task.Title,
			Dimension:   result.Dimension,
			Category:    finding.Aspect,
			Title:       finding.Aspect,
			Content:     finding.Observation,
			Example:     finding.Example,
			Technique:   finding.Technique,
		}

		s.db.Create(knowledge)
	}

	// 如果有可操作的建议，也存储为知识
	for i, tip := range analysis.Tips {
		knowledge := &BookKnowledge{
			ID:          uuid.New().String(),
			TenantID:    task.TenantID,
			TaskID:      task.ID,
			SourceTitle: task.Title,
			Dimension:   result.Dimension,
			Category:    "实践建议",
			Title:       fmt.Sprintf("建议 %d", i+1),
			Content:     tip,
			Technique:   tip,
		}
		s.db.Create(knowledge)
	}
}

// failTask 标记任务失败
func (s *Service) failTask(task *BookParserTask, errMsg string) {
	s.db.Model(task).Updates(map[string]any{
		"status":    TaskStatusFailed,
		"error_msg": errMsg,
	})
}

// fetchFromURL 从 URL 抓取内容
func (s *Service) fetchFromURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; BookParser/1.0)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	// 限制读取大小（10MB）
	const maxSize = 10 * 1024 * 1024
	limitedReader := &limitedReader{r: resp.Body, remaining: maxSize}

	var buf strings.Builder
	_, err = io.Copy(&buf, limitedReader)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// getDimensionPrompt 获取维度特定的系统提示
func (s *Service) getDimensionPrompt(dim AnalysisDimension) string {
	prompts := map[AnalysisDimension]string{
		DimensionStyle: `你是文风分析专家，专注分析作品的语言特色：
1. 叙事视角：第一/第三人称、全知/限知
2. 语言风格：简洁/华丽、严肃/幽默、冷峻/温情
3. 句式特点：长句/短句、复杂/简单
4. 用词习惯：口语化/书面化、专业术语使用
5. 节奏控制：快节奏/慢节奏、张弛把控

输出格式（JSON）：
{"dimension":"style","findings":[{"aspect":"方面","observation":"发现","example":"示例","technique":"技法"}],"summary":"总结","actionable_tips":["建议"]}`,

		DimensionPlot: `你是情节设计分析专家，专注分析故事结构：
1. 核心冲突：主要矛盾、冲突来源
2. 悬念设计：悬念设置、伏笔埋设
3. 故事节奏：起承转合、高潮布局
4. 情节转折：转折点设计、意外感营造
5. 伏笔回收：前后呼应、逻辑闭环

输出格式（JSON）：
{"dimension":"plot","findings":[{"aspect":"方面","observation":"发现","example":"示例","technique":"技法"}],"summary":"总结","actionable_tips":["建议"]}`,

		DimensionCharacter: `你是人物塑造分析专家，专注分析角色设计：
1. 角色塑造：性格特点、行为动机
2. 性格刻画：正面/负面特质、矛盾性格
3. 对话风格：语言特点、口头禅、说话方式
4. 人物弧光：成长轨迹、性格变化
5. 角色关系：互动模式、关系变化

输出格式（JSON）：
{"dimension":"character","findings":[{"aspect":"方面","observation":"发现","example":"示例","technique":"技法"}],"summary":"总结","actionable_tips":["建议"]}`,

		DimensionEmotion: `你是读者情绪分析专家，专注分析情感设计：
1. 共鸣点：情感触发、代入感设计
2. 爽点布局：快感来源、爽点密度
3. 嗨点设计：高潮情绪、情感宣泄
4. 虐点处理：悲剧美学、虐心技巧
5. 情感曲线：情绪起伏、张弛节奏

输出格式（JSON）：
{"dimension":"emotion","findings":[{"aspect":"方面","observation":"发现","example":"示例","technique":"技法"}],"summary":"总结","actionable_tips":["建议"]}`,

		DimensionMeme: `你是网文热梗分析专家，专注提取流行元素：
1. 流行梗：网络用语、热门梗使用
2. 搞笑点：幽默技巧、笑点设计
3. 网络文化：二次元元素、网文套路解构
4. 金句台词：经典台词、名场面
5. 创意表达：独特比喻、新颖说法

输出格式（JSON）：
{"dimension":"meme","findings":[{"aspect":"方面","observation":"发现","example":"示例","technique":"技法"}],"summary":"总结","actionable_tips":["建议"]}`,

		DimensionOutline: `你是章节大纲分析专家，专注提取结构信息：
1. 章节结构：章节划分、小节安排
2. 情节脉络：主线发展、支线穿插
3. 节点事件：关键事件、转折点
4. 承上启下：衔接技巧、过渡处理
5. 悬念钩子：章节结尾、追更动力

输出格式（JSON）：
{"dimension":"outline","findings":[{"aspect":"方面","observation":"发现","example":"示例","technique":"技法"}],"summary":"总结","actionable_tips":["建议"]}`,
	}

	if prompt, ok := prompts[dim]; ok {
		return prompt
	}
	return prompts[DimensionStyle]
}

// limitedReader 限制读取大小的 Reader
type limitedReader struct {
	r         interface{ Read([]byte) (int, error) }
	remaining int64
}

func (lr *limitedReader) Read(p []byte) (int, error) {
	if lr.remaining <= 0 {
		return 0, fmt.Errorf("内容超过大小限制")
	}
	if int64(len(p)) > lr.remaining {
		p = p[:lr.remaining]
	}
	n, err := lr.r.Read(p)
	lr.remaining -= int64(n)
	return n, err
}
