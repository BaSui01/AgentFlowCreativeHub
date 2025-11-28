package multimodel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"backend/internal/agent/runtime"
	"backend/internal/models"
	
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service 多模型服务
type Service struct {
	db            *gorm.DB
	agentRegistry *runtime.Registry
	modelService  *models.ModelService
}

// NewService 创建多模型服务
func NewService(db *gorm.DB, agentRegistry *runtime.Registry, modelService *models.ModelService) *Service {
	return &Service{
		db:            db,
		agentRegistry: agentRegistry,
		modelService:  modelService,
	}
}

// Draw 执行多模型抽卡
func (s *Service) Draw(ctx context.Context, tenantID, userID string, req *DrawRequest) (*DrawResponse, error) {
	startTime := time.Now()
	drawID := uuid.New().String()

	// 并发调用多个模型
	results := make([]DrawResult, len(req.ModelIDs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, modelID := range req.ModelIDs {
		wg.Add(1)
		go func(index int, mID string) {
			defer wg.Done()

			result := s.callModel(ctx, tenantID, userID, mID, req)
			
			mu.Lock()
			results[index] = result
			mu.Unlock()
		}(i, modelID)
	}

	wg.Wait()

	// 计算成功率
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	successRate := float64(successCount) / float64(len(results)) * 100

	// 选择最佳模型（基于成功状态和评分）
	bestModelID := ""
	bestScore := float64(0)
	for _, r := range results {
		if r.Success && r.Score > bestScore {
			bestScore = r.Score
			bestModelID = r.ModelID
		}
	}

	totalTime := time.Since(startTime).Milliseconds()

	response := &DrawResponse{
		DrawID:      drawID,
		Results:     results,
		BestModelID: bestModelID,
		TotalTime:   totalTime,
		SuccessRate: successRate,
	}

	// 保存历史记录
	go s.saveHistory(context.Background(), tenantID, userID, req, response)

	return response, nil
}

// callModel 调用单个模型
func (s *Service) callModel(ctx context.Context, tenantID, userID, modelID string, req *DrawRequest) DrawResult {
	startTime := time.Now()

	result := DrawResult{
		ModelID: modelID,
	}

	// 获取模型信息
	model, err := s.modelService.GetModel(ctx, tenantID, modelID)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("获取模型失败: %v", err)
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result
	}

	result.ModelName = model.Name
	result.Provider = model.Provider

	// 获取对应类型的 Agent
	agent, err := s.agentRegistry.GetAgentByType(ctx, tenantID, req.AgentType)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("获取Agent失败: %v", err)
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result
	}

	// 构建输入
	input := &runtime.AgentInput{
		Content: req.Content,
		Context: &runtime.AgentContext{
			TenantID: tenantID,
			UserID:   userID,
		},
		ExtraParams: req.ExtraParams,
	}

	// 执行 Agent
	agentResult, err := agent.Execute(ctx, input)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("执行失败: %v", err)
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result
	}

	result.Success = true
	result.Content = agentResult.Output
	result.LatencyMs = time.Since(startTime).Milliseconds()

	if agentResult.Usage != nil {
		result.TokenUsage = &TokenUsage{
			PromptTokens:     agentResult.Usage.PromptTokens,
			CompletionTokens: agentResult.Usage.CompletionTokens,
			TotalTokens:      agentResult.Usage.TotalTokens,
		}
	}

	// 简单评分逻辑（可根据需求扩展）
	result.Score = s.scoreResult(&result)

	return result
}

// scoreResult 对结果进行评分
func (s *Service) scoreResult(result *DrawResult) float64 {
	if !result.Success {
		return 0
	}

	score := float64(70) // 基础分

	// 内容长度评分（假设200-2000字为最佳）
	contentLen := len([]rune(result.Content))
	if contentLen >= 200 && contentLen <= 2000 {
		score += 15
	} else if contentLen > 2000 {
		score += 10
	} else {
		score += 5
	}

	// 延迟评分（越快越好）
	if result.LatencyMs < 3000 {
		score += 15
	} else if result.LatencyMs < 5000 {
		score += 10
	} else {
		score += 5
	}

	return score
}

// saveHistory 保存抽卡历史
func (s *Service) saveHistory(ctx context.Context, tenantID, userID string, req *DrawRequest, resp *DrawResponse) error {
	modelIDsJSON, _ := json.Marshal(req.ModelIDs)
	resultsJSON, _ := json.Marshal(resp.Results)

	history := &DrawHistory{
		ID:          resp.DrawID,
		TenantID:    tenantID,
		UserID:      userID,
		AgentType:   req.AgentType,
		ModelIDs:    string(modelIDsJSON),
		InputPrompt: req.Content,
		Results:     string(resultsJSON),
		BestModelID: resp.BestModelID,
		TotalTimeMs: resp.TotalTime,
		SuccessRate: resp.SuccessRate,
	}

	return s.db.WithContext(ctx).Create(history).Error
}

// GetDrawHistory 获取抽卡历史详情
func (s *Service) GetDrawHistory(ctx context.Context, tenantID, drawID string) (*DrawHistory, error) {
	var history DrawHistory
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", drawID, tenantID).
		First(&history).Error; err != nil {
		return nil, err
	}
	return &history, nil
}

// ListDrawHistory 查询抽卡历史列表
func (s *Service) ListDrawHistory(ctx context.Context, tenantID string, req *ListDrawHistoryRequest) ([]*DrawHistory, int64, error) {
	query := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID)

	if req.AgentType != nil && *req.AgentType != "" {
		query = query.Where("agent_type = ?", *req.AgentType)
	}

	// 计算总数
	var total int64
	if err := query.Model(&DrawHistory{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize).Order("created_at DESC")

	var histories []*DrawHistory
	if err := query.Find(&histories).Error; err != nil {
		return nil, 0, err
	}

	return histories, total, nil
}

// Regenerate 重新生成单个模型结果
func (s *Service) Regenerate(ctx context.Context, tenantID, userID string, req *RegenerateRequest) (*DrawResult, error) {
	// 获取历史记录
	history, err := s.GetDrawHistory(ctx, tenantID, req.DrawID)
	if err != nil {
		return nil, fmt.Errorf("获取历史记录失败: %w", err)
	}

	// 解析模型ID列表
	var modelIDs []string
	if err := json.Unmarshal([]byte(history.ModelIDs), &modelIDs); err != nil {
		return nil, fmt.Errorf("解析模型ID失败: %w", err)
	}

	// 检查模型ID是否在原列表中
	found := false
	for _, mID := range modelIDs {
		if mID == req.ModelID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("模型ID不在原抽卡列表中")
	}

	// 重新调用模型
	drawReq := &DrawRequest{
		ModelIDs:  []string{req.ModelID},
		AgentType: history.AgentType,
		Content:   history.InputPrompt,
	}

	result := s.callModel(ctx, tenantID, userID, req.ModelID, drawReq)
	return &result, nil
}

// DeleteDrawHistory 删除抽卡历史
func (s *Service) DeleteDrawHistory(ctx context.Context, tenantID, drawID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", drawID, tenantID).
		Delete(&DrawHistory{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// GetDrawStats 获取抽卡统计
func (s *Service) GetDrawStats(ctx context.Context, tenantID string) (map[string]any, error) {
	stats := make(map[string]any)

	// 总抽卡次数
	var totalDraws int64
	if err := s.db.WithContext(ctx).
		Model(&DrawHistory{}).
		Where("tenant_id = ?", tenantID).
		Count(&totalDraws).Error; err != nil {
		return nil, err
	}
	stats["total_draws"] = totalDraws

	// 按Agent类型统计
	var byAgentType []struct {
		AgentType string
		Count     int64
	}
	if err := s.db.WithContext(ctx).
		Model(&DrawHistory{}).
		Select("agent_type, COUNT(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("agent_type").
		Scan(&byAgentType).Error; err != nil {
		return nil, err
	}
	stats["by_agent_type"] = byAgentType

	// 平均成功率
	var avgSuccessRate float64
	if err := s.db.WithContext(ctx).
		Model(&DrawHistory{}).
		Select("AVG(success_rate)").
		Where("tenant_id = ?", tenantID).
		Scan(&avgSuccessRate).Error; err != nil {
		return nil, err
	}
	stats["avg_success_rate"] = avgSuccessRate

	// 最常使用的模型
	type ModelStat struct {
		ModelID string
		Count   int64
	}
	var modelStats []ModelStat
	
	// 获取所有历史记录
	var histories []*DrawHistory
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Find(&histories).Error; err == nil {
		
		modelCount := make(map[string]int64)
		for _, h := range histories {
			var modelIDs []string
			if err := json.Unmarshal([]byte(h.ModelIDs), &modelIDs); err == nil {
				for _, mID := range modelIDs {
					modelCount[mID]++
				}
			}
		}
		
		for mID, count := range modelCount {
			modelStats = append(modelStats, ModelStat{ModelID: mID, Count: count})
		}
	}
	stats["popular_models"] = modelStats

	return stats, nil
}
