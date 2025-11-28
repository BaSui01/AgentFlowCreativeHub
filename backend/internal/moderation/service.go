package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backend/internal/agent/runtime"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service 内容审核服务
type Service struct {
	db            *gorm.DB
	agentRegistry *runtime.Registry
	wordCache     map[string][]SensitiveWord // 敏感词缓存
}

// NewService 创建审核服务
func NewService(db *gorm.DB, agentRegistry *runtime.Registry) *Service {
	return &Service{
		db:            db,
		agentRegistry: agentRegistry,
		wordCache:     make(map[string][]SensitiveWord),
	}
}

// AutoMigrate 自动迁移表结构
func (s *Service) AutoMigrate() error {
	return s.db.AutoMigrate(
		&ModerationTask{},
		&ModerationRecord{},
		&SensitiveWord{},
		&ModerationRule{},
	)
}

// ============================================================================
// 任务管理
// ============================================================================

// SubmitContent 提交内容审核
func (s *Service) SubmitContent(ctx context.Context, tenantID, userID, userName string, req *SubmitContentRequest) (*ModerationTask, error) {
	task := &ModerationTask{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		ContentType:   req.ContentType,
		ContentID:     req.ContentID,
		Title:         req.Title,
		Content:       req.Content,
		ContentMeta:   req.ContentMeta,
		SubmitterID:   userID,
		SubmitterName: userName,
		Status:        TaskStatusPending,
		Priority:      req.Priority,
		CurrentLevel:  1,
		MaxLevel:      1,
	}

	// 获取审核规则确定最大审核级别
	rule, _ := s.GetRuleByContentType(ctx, tenantID, req.ContentType)
	if rule != nil {
		task.MaxLevel = rule.RequireLevels
		if rule.AIEnabled {
			// 执行 AI 预审
			s.performAIReview(ctx, task)
		}
	}

	// 敏感词过滤
	filterResult := s.FilterContent(ctx, tenantID, req.Content)
	if filterResult.HasSensitive {
		// 根据敏感词级别调整优先级
		for _, match := range filterResult.Matches {
			if match.Level == "high" {
				task.Priority = 2
				task.AIRiskLevel = "high"
				break
			} else if match.Level == "medium" && task.Priority < 1 {
				task.Priority = 1
				task.AIRiskLevel = "medium"
			}
		}
	}

	if err := s.db.WithContext(ctx).Create(task).Error; err != nil {
		return nil, fmt.Errorf("创建审核任务失败: %w", err)
	}

	return task, nil
}

// performAIReview 执行 AI 预审
func (s *Service) performAIReview(ctx context.Context, task *ModerationTask) {
	if s.agentRegistry == nil {
		return
	}

	// 获取 reviewer agent
	agent, err := s.agentRegistry.GetAgentByType(ctx, task.TenantID, "reviewer")
	if err != nil {
		return
	}

	// 构建 AI 审核输入
	input := &runtime.AgentInput{
		Content: task.Content,
		Context: &runtime.AgentContext{
			TenantID: task.TenantID,
		},
		ExtraParams: map[string]any{
			"criteria": "请评估以下内容的合规性，检查是否存在：1.政治敏感 2.色情低俗 3.暴力血腥 4.广告营销 5.违法违规。输出JSON格式：{\"risk_level\":\"safe/low/medium/high\",\"risk_score\":0-100,\"findings\":[{\"category\":\"...\",\"description\":\"...\",\"severity\":\"...\"}]}",
		},
	}

	result, err := agent.Execute(ctx, input)
	if err != nil {
		return
	}

	task.AIReviewed = true

	// 解析 AI 响应
	var aiResult struct {
		RiskLevel string      `json:"risk_level"`
		RiskScore float64     `json:"risk_score"`
		Findings  []AIFinding `json:"findings"`
	}

	// 尝试从响应中提取 JSON
	output := result.Output
	if idx := strings.Index(output, "{"); idx >= 0 {
		if endIdx := strings.LastIndex(output, "}"); endIdx > idx {
			output = output[idx : endIdx+1]
		}
	}

	if err := json.Unmarshal([]byte(output), &aiResult); err == nil {
		task.AIRiskLevel = aiResult.RiskLevel
		task.AIRiskScore = aiResult.RiskScore
		task.AIFindings = aiResult.Findings
		if len(aiResult.Findings) > 0 {
			findingsJSON, _ := json.Marshal(aiResult.Findings)
			task.AIFindingsJSON = string(findingsJSON)
		}
	}
}

// GetTask 获取任务详情
func (s *Service) GetTask(ctx context.Context, id string) (*ModerationTask, error) {
	var task ModerationTask
	if err := s.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	
	// 解析 AI findings
	if task.AIFindingsJSON != "" {
		json.Unmarshal([]byte(task.AIFindingsJSON), &task.AIFindings)
	}
	
	return &task, nil
}

// ListTasks 获取任务列表
func (s *Service) ListTasks(ctx context.Context, query *TaskQuery) ([]ModerationTask, int64, error) {
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 20
	}
	if query.Page <= 0 {
		query.Page = 1
	}

	db := s.db.WithContext(ctx).Model(&ModerationTask{}).
		Where("tenant_id = ?", query.TenantID)

	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.ContentType != "" {
		db = db.Where("content_type = ?", query.ContentType)
	}
	if query.AssignedTo != "" {
		db = db.Where("assigned_to = ?", query.AssignedTo)
	}
	if query.SubmitterID != "" {
		db = db.Where("submitter_id = ?", query.SubmitterID)
	}
	if query.AIRiskLevel != "" {
		db = db.Where("ai_risk_level = ?", query.AIRiskLevel)
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", query.EndTime)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tasks []ModerationTask
	offset := (query.Page - 1) * query.PageSize
	if err := db.Order("priority DESC, created_at ASC").
		Limit(query.PageSize).
		Offset(offset).
		Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// GetPendingQueue 获取待审核队列
func (s *Service) GetPendingQueue(ctx context.Context, tenantID string, reviewerID string, limit int) ([]ModerationTask, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var tasks []ModerationTask
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, TaskStatusPending).
		Where("assigned_to IS NULL OR assigned_to = ?", reviewerID).
		Order("priority DESC, created_at ASC").
		Limit(limit).
		Find(&tasks).Error

	return tasks, err
}

// AssignTask 分配任务
func (s *Service) AssignTask(ctx context.Context, taskID, reviewerID string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&ModerationTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"assigned_to": reviewerID,
			"assigned_at": now,
			"status":      TaskStatusReviewing,
		}).Error
}

// ============================================================================
// 审核操作
// ============================================================================

// ReviewTask 审核任务
func (s *Service) ReviewTask(ctx context.Context, tenantID, reviewerID, reviewerName string, req *ReviewRequest) (*ModerationRecord, error) {
	// 获取任务
	task, err := s.GetTask(ctx, req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	// 创建审核记录
	record := &ModerationRecord{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		TaskID:       req.TaskID,
		ReviewerID:   reviewerID,
		ReviewerName: reviewerName,
		Level:        task.CurrentLevel,
		Action:       req.Action,
		Comment:      req.Comment,
		Reason:       req.Reason,
		Tags:         req.Tags,
		Punishment:   req.Punishment,
	}

	if len(req.Tags) > 0 {
		tagsJSON, _ := json.Marshal(req.Tags)
		record.TagsJSON = string(tagsJSON)
	}

	// 根据动作更新任务状态
	updates := make(map[string]interface{})
	switch req.Action {
	case ActionApprove:
		if task.CurrentLevel >= task.MaxLevel {
			updates["status"] = TaskStatusApproved
			record.Decision = "approve"
		} else {
			updates["current_level"] = task.CurrentLevel + 1
			updates["status"] = TaskStatusPending
			updates["assigned_to"] = nil
			record.Decision = "approve_level"
		}
	case ActionReject:
		updates["status"] = TaskStatusRejected
		record.Decision = "reject"
		record.ViolationType = req.Reason
	case ActionRevision:
		updates["status"] = TaskStatusRevision
		record.Decision = "revision"
	case ActionEscalate:
		updates["status"] = TaskStatusEscalated
		updates["current_level"] = task.CurrentLevel + 1
		record.Decision = "escalate"
	case ActionReassign:
		updates["status"] = TaskStatusPending
		updates["assigned_to"] = nil
		record.Decision = "reassign"
	}

	// 事务处理
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(&ModerationTask{}).Where("id = ?", req.TaskID).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("审核操作失败: %w", err)
	}

	return record, nil
}

// GetTaskRecords 获取任务的审核记录
func (s *Service) GetTaskRecords(ctx context.Context, taskID string) ([]ModerationRecord, error) {
	var records []ModerationRecord
	err := s.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("reviewed_at ASC").
		Find(&records).Error
	
	// 解析 tags
	for i := range records {
		if records[i].TagsJSON != "" {
			json.Unmarshal([]byte(records[i].TagsJSON), &records[i].Tags)
		}
	}
	
	return records, err
}

// ============================================================================
// 敏感词管理
// ============================================================================

// AddSensitiveWord 添加敏感词
func (s *Service) AddSensitiveWord(ctx context.Context, tenantID, createdBy string, word, category, level, action, replace string) (*SensitiveWord, error) {
	sw := &SensitiveWord{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Word:      word,
		Category:  category,
		Level:     level,
		Action:    action,
		Replace:   replace,
		IsActive:  true,
		CreatedBy: createdBy,
	}

	if sw.Level == "" {
		sw.Level = "medium"
	}
	if sw.Action == "" {
		sw.Action = "flag"
	}

	if err := s.db.WithContext(ctx).Create(sw).Error; err != nil {
		return nil, err
	}

	// 清除缓存
	delete(s.wordCache, tenantID)

	return sw, nil
}

// BatchAddWords 批量添加敏感词
func (s *Service) BatchAddWords(ctx context.Context, tenantID, createdBy string, req *BatchWordRequest) (int, error) {
	words := make([]*SensitiveWord, len(req.Words))
	for i, w := range req.Words {
		words[i] = &SensitiveWord{
			ID:        uuid.New().String(),
			TenantID:  tenantID,
			Word:      w,
			Category:  req.Category,
			Level:     req.Level,
			Action:    req.Action,
			IsActive:  true,
			CreatedBy: createdBy,
		}
		if words[i].Level == "" {
			words[i].Level = "medium"
		}
		if words[i].Action == "" {
			words[i].Action = "flag"
		}
	}

	if err := s.db.WithContext(ctx).CreateInBatches(words, 100).Error; err != nil {
		return 0, err
	}

	delete(s.wordCache, tenantID)
	return len(words), nil
}

// ListSensitiveWords 获取敏感词列表
func (s *Service) ListSensitiveWords(ctx context.Context, tenantID, category string, page, pageSize int) ([]SensitiveWord, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}

	db := s.db.WithContext(ctx).Model(&SensitiveWord{}).
		Where("tenant_id = ?", tenantID)

	if category != "" {
		db = db.Where("category = ?", category)
	}

	var total int64
	db.Count(&total)

	var words []SensitiveWord
	offset := (page - 1) * pageSize
	err := db.Order("category, word").
		Limit(pageSize).
		Offset(offset).
		Find(&words).Error

	return words, total, err
}

// DeleteSensitiveWord 删除敏感词
func (s *Service) DeleteSensitiveWord(ctx context.Context, id, tenantID string) error {
	err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&SensitiveWord{}).Error
	if err == nil {
		delete(s.wordCache, tenantID)
	}
	return err
}

// FilterContent 过滤内容
func (s *Service) FilterContent(ctx context.Context, tenantID, content string) *FilterResult {
	result := &FilterResult{
		Original:     content,
		Filtered:     content,
		HasSensitive: false,
		Matches:      []MatchedWord{},
	}

	// 获取敏感词列表
	words, err := s.getSensitiveWords(ctx, tenantID)
	if err != nil || len(words) == 0 {
		return result
	}

	// 检测敏感词
	for _, w := range words {
		if !w.IsActive {
			continue
		}

		pos := strings.Index(strings.ToLower(content), strings.ToLower(w.Word))
		if pos >= 0 {
			result.HasSensitive = true
			result.Matches = append(result.Matches, MatchedWord{
				Word:     w.Word,
				Category: w.Category,
				Level:    w.Level,
				Position: pos,
				Action:   w.Action,
			})

			// 根据动作处理
			if w.Action == "replace" && w.Replace != "" {
				result.Filtered = strings.ReplaceAll(result.Filtered, w.Word, w.Replace)
			} else if w.Action == "block" {
				result.Filtered = strings.ReplaceAll(result.Filtered, w.Word, strings.Repeat("*", len([]rune(w.Word))))
			}
		}
	}

	return result
}

func (s *Service) getSensitiveWords(ctx context.Context, tenantID string) ([]SensitiveWord, error) {
	// 检查缓存
	if cached, ok := s.wordCache[tenantID]; ok {
		return cached, nil
	}

	var words []SensitiveWord
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Find(&words).Error

	if err == nil {
		s.wordCache[tenantID] = words
	}

	return words, err
}

// ============================================================================
// 审核规则管理
// ============================================================================

// CreateRule 创建审核规则
func (s *Service) CreateRule(ctx context.Context, rule *ModerationRule) error {
	rule.ID = uuid.New().String()
	return s.db.WithContext(ctx).Create(rule).Error
}

// GetRuleByContentType 根据内容类型获取规则
func (s *Service) GetRuleByContentType(ctx context.Context, tenantID, contentType string) (*ModerationRule, error) {
	var rule ModerationRule
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND content_type = ? AND is_active = ?", tenantID, contentType, true).
		Order("priority DESC").
		First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListRules 获取规则列表
func (s *Service) ListRules(ctx context.Context, tenantID string) ([]ModerationRule, error) {
	var rules []ModerationRule
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("content_type, priority DESC").
		Find(&rules).Error
	return rules, err
}

// UpdateRule 更新规则
func (s *Service) UpdateRule(ctx context.Context, id string, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&ModerationRule{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeleteRule 删除规则
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&ModerationRule{}, "id = ?", id).Error
}

// ============================================================================
// 统计
// ============================================================================

// GetStats 获取审核统计
func (s *Service) GetStats(ctx context.Context, tenantID string, period string) (*ModerationStats, error) {
	stats := &ModerationStats{
		TenantID:      tenantID,
		Period:        period,
		ByContentType: make(map[string]int64),
		ByRiskLevel:   make(map[string]int64),
	}

	now := time.Now()
	switch period {
	case "daily":
		stats.StartDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "weekly":
		stats.StartDate = now.AddDate(0, 0, -7)
	case "monthly":
		stats.StartDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		stats.StartDate = now.AddDate(0, 0, -30)
	}
	stats.EndDate = now

	db := s.db.WithContext(ctx).Model(&ModerationTask{}).
		Where("tenant_id = ? AND created_at >= ?", tenantID, stats.StartDate)

	// 总任务数
	db.Count(&stats.TotalTasks)

	// 各状态数量
	s.db.WithContext(ctx).Model(&ModerationTask{}).
		Where("tenant_id = ? AND created_at >= ? AND status = ?", tenantID, stats.StartDate, TaskStatusPending).
		Count(&stats.PendingTasks)

	s.db.WithContext(ctx).Model(&ModerationTask{}).
		Where("tenant_id = ? AND created_at >= ? AND status = ?", tenantID, stats.StartDate, TaskStatusApproved).
		Count(&stats.ApprovedTasks)

	s.db.WithContext(ctx).Model(&ModerationTask{}).
		Where("tenant_id = ? AND created_at >= ? AND status = ?", tenantID, stats.StartDate, TaskStatusRejected).
		Count(&stats.RejectedTasks)

	// AI 预审率
	var aiReviewed int64
	s.db.WithContext(ctx).Model(&ModerationTask{}).
		Where("tenant_id = ? AND created_at >= ? AND ai_reviewed = ?", tenantID, stats.StartDate, true).
		Count(&aiReviewed)
	if stats.TotalTasks > 0 {
		stats.AIReviewRate = float64(aiReviewed) / float64(stats.TotalTasks) * 100
	}

	// 按内容类型统计
	var contentTypeStats []struct {
		ContentType string
		Count       int64
	}
	s.db.WithContext(ctx).Model(&ModerationTask{}).
		Select("content_type, COUNT(*) as count").
		Where("tenant_id = ? AND created_at >= ?", tenantID, stats.StartDate).
		Group("content_type").
		Scan(&contentTypeStats)
	for _, ct := range contentTypeStats {
		stats.ByContentType[ct.ContentType] = ct.Count
	}

	// 按风险级别统计
	var riskStats []struct {
		AIRiskLevel string
		Count       int64
	}
	s.db.WithContext(ctx).Model(&ModerationTask{}).
		Select("ai_risk_level, COUNT(*) as count").
		Where("tenant_id = ? AND created_at >= ? AND ai_risk_level != ''", tenantID, stats.StartDate).
		Group("ai_risk_level").
		Scan(&riskStats)
	for _, rs := range riskStats {
		stats.ByRiskLevel[rs.AIRiskLevel] = rs.Count
	}

	return stats, nil
}
