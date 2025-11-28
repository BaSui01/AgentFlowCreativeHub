package plot

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"backend/internal/agent/runtime"
	"backend/internal/workspace"
	
	"gorm.io/gorm"
)

// Service 剧情推演服务
type Service struct {
	db                *gorm.DB
	agentRegistry     *runtime.Registry
	workspaceService  *workspace.Service
}

// NewService 创建剧情推演服务
func NewService(db *gorm.DB, agentRegistry *runtime.Registry, workspaceService *workspace.Service) *Service {
	return &Service{
		db:               db,
		agentRegistry:    agentRegistry,
		workspaceService: workspaceService,
	}
}

// CreatePlotRecommendation 创建剧情推演
func (s *Service) CreatePlotRecommendation(ctx context.Context, tenantID, userID string, req *CreatePlotRequest) (*PlotRecommendationResponse, error) {
	// 调用 PlotAgent 生成剧情分支
	agent, err := s.agentRegistry.GetAgentByType(ctx, tenantID, "plot")
	if err != nil {
		return nil, fmt.Errorf("获取 PlotAgent 失败: %w", err)
	}

	// 构建输入参数
	extraParams := map[string]any{
		"current_plot":  req.CurrentPlot,
		"num_branches":  req.NumBranches,
	}
	if req.CharacterInfo != nil {
		extraParams["characters"] = *req.CharacterInfo
	}
	if req.WorldSetting != nil {
		extraParams["world_setting"] = *req.WorldSetting
	}

	input := &runtime.AgentInput{
		Content: fmt.Sprintf("基于以下内容生成 %d 个剧情分支", req.NumBranches),
		Context: &runtime.AgentContext{
			TenantID: tenantID,
			UserID:   userID,
		},
		ExtraParams: extraParams,
	}

	// 执行 Agent
	result, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("生成剧情分支失败: %w", err)
	}

	// 解析 Agent 返回的 JSON
	var agentOutput struct {
		Branches       []PlotBranch `json:"branches"`
		Recommendation string       `json:"recommendation"`
	}
	if err := json.Unmarshal([]byte(result.Output), &agentOutput); err != nil {
		// 如果解析失败，尝试将整个输出作为单个分支
		agentOutput.Branches = []PlotBranch{
			{
				ID:      1,
				Title:   "生成的剧情",
				Summary: result.Output,
			},
		}
	}

	branchesJSON, _ := json.Marshal(agentOutput.Branches)

	// 保存到数据库
	plot := &PlotRecommendation{
		TenantID:      tenantID,
		UserID:        userID,
		Title:         req.Title,
		CurrentPlot:   req.CurrentPlot,
		ModelID:       req.ModelID,
		Branches:      string(branchesJSON),
		Applied:       false,
	}

	if req.CharacterInfo != nil {
		plot.CharacterInfo = *req.CharacterInfo
	}
	if req.WorldSetting != nil {
		plot.WorldSetting = *req.WorldSetting
	}
	if req.WorkspaceID != nil {
		plot.WorkspaceID = *req.WorkspaceID
	}
	if req.WorkID != nil {
		plot.WorkID = *req.WorkID
	}
	if req.ChapterID != nil {
		plot.ChapterID = *req.ChapterID
	}

	if err := s.db.WithContext(ctx).Create(plot).Error; err != nil {
		return nil, fmt.Errorf("保存剧情推演失败: %w", err)
	}

	return &PlotRecommendationResponse{
		PlotRecommendation: plot,
		ParsedBranches:     agentOutput.Branches,
	}, nil
}

// GetPlotRecommendation 获取剧情推演详情
func (s *Service) GetPlotRecommendation(ctx context.Context, tenantID, plotID string) (*PlotRecommendationResponse, error) {
	var plot PlotRecommendation
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", plotID, tenantID).
		First(&plot).Error; err != nil {
		return nil, err
	}

	// 解析分支
	var branches []PlotBranch
	if err := json.Unmarshal([]byte(plot.Branches), &branches); err != nil {
		return nil, fmt.Errorf("解析剧情分支失败: %w", err)
	}

	return &PlotRecommendationResponse{
		PlotRecommendation: &plot,
		ParsedBranches:     branches,
	}, nil
}

// UpdatePlotRecommendation 更新剧情推演
func (s *Service) UpdatePlotRecommendation(ctx context.Context, tenantID, plotID string, req *UpdatePlotRequest) (*PlotRecommendationResponse, error) {
	plot, err := s.GetPlotRecommendation(ctx, tenantID, plotID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]any)
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.SelectedBranch != nil {
		updates["selected_branch"] = *req.SelectedBranch
	}

	if err := s.db.WithContext(ctx).Model(plot.PlotRecommendation).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新剧情推演失败: %w", err)
	}

	return s.GetPlotRecommendation(ctx, tenantID, plotID)
}

// DeletePlotRecommendation 删除剧情推演
func (s *Service) DeletePlotRecommendation(ctx context.Context, tenantID, plotID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", plotID, tenantID).
		Delete(&PlotRecommendation{})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// ListPlotRecommendations 查询剧情推演列表
func (s *Service) ListPlotRecommendations(ctx context.Context, tenantID string, req *ListPlotRecommendationsRequest) ([]*PlotRecommendationResponse, int64, error) {
	query := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID)

	if req.WorkspaceID != nil {
		query = query.Where("workspace_id = ?", *req.WorkspaceID)
	}
	if req.WorkID != nil {
		query = query.Where("work_id = ?", *req.WorkID)
	}
	if req.ChapterID != nil {
		query = query.Where("chapter_id = ?", *req.ChapterID)
	}
	if req.Applied != nil {
		query = query.Where("applied = ?", *req.Applied)
	}

	// 计算总数
	var total int64
	if err := query.Model(&PlotRecommendation{}).Count(&total).Error; err != nil {
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

	var plots []*PlotRecommendation
	if err := query.Find(&plots).Error; err != nil {
		return nil, 0, err
	}

	// 解析分支
	responses := make([]*PlotRecommendationResponse, len(plots))
	for i, plot := range plots {
		var branches []PlotBranch
		json.Unmarshal([]byte(plot.Branches), &branches)
		
		responses[i] = &PlotRecommendationResponse{
			PlotRecommendation: plot,
			ParsedBranches:     branches,
		}
	}

	return responses, total, nil
}

// ApplyPlotToChapter 将剧情应用到章节
func (s *Service) ApplyPlotToChapter(ctx context.Context, tenantID string, req *ApplyPlotRequest) error {
	// 获取剧情推演
	plot, err := s.GetPlotRecommendation(ctx, tenantID, req.PlotID)
	if err != nil {
		return fmt.Errorf("获取剧情推演失败: %w", err)
	}

	// 验证分支索引
	if req.BranchIndex < 0 || req.BranchIndex >= len(plot.ParsedBranches) {
		return fmt.Errorf("无效的分支索引: %d", req.BranchIndex)
	}

	selectedBranch := plot.ParsedBranches[req.BranchIndex]

	// 读取章节内容
	fileDetail, err := s.workspaceService.GetFileDetail(ctx, tenantID, req.ChapterID)
	if err != nil {
		return fmt.Errorf("获取章节失败: %w", err)
	}

	// 获取当前内容
	currentContent := ""
	if fileDetail.Version != nil {
		currentContent = fileDetail.Version.Content
	}

	// 构建新内容
	newContent := ""
	if req.AppendContent {
		// 追加模式
		newContent = currentContent + "\n\n## 新增剧情\n\n" + selectedBranch.Summary
		if len(selectedBranch.KeyEvents) > 0 {
			newContent += "\n\n**关键事件:**\n"
			for i, event := range selectedBranch.KeyEvents {
				newContent += fmt.Sprintf("%d. %s\n", i+1, event)
			}
		}
	} else {
		// 替换模式
		newContent = "# " + selectedBranch.Title + "\n\n" + selectedBranch.Summary
		if len(selectedBranch.KeyEvents) > 0 {
			newContent += "\n\n**关键事件:**\n"
			for i, event := range selectedBranch.KeyEvents {
				newContent += fmt.Sprintf("%d. %s\n", i+1, event)
			}
		}
	}

	// 更新章节内容
	updateReq := &workspace.UpdateFileRequest{
		TenantID: tenantID,
		NodeID:   req.ChapterID,
		Content:  newContent,
	}
	if _, err := s.workspaceService.UpdateFileContent(ctx, updateReq); err != nil {
		return fmt.Errorf("更新章节内容失败: %w", err)
	}

	// 标记剧情已应用
	now := time.Now()
	updates := map[string]any{
		"applied":         true,
		"applied_at":      &now,
		"selected_branch": req.BranchIndex,
		"chapter_id":      req.ChapterID,
	}

	if err := s.db.WithContext(ctx).Model(&PlotRecommendation{}).
		Where("id = ? AND tenant_id = ?", req.PlotID, tenantID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("更新剧情状态失败: %w", err)
	}

	return nil
}

// GetPlotStats 获取剧情推演统计
func (s *Service) GetPlotStats(ctx context.Context, tenantID string) (map[string]any, error) {
	stats := make(map[string]any)

	// 总推演次数
	var totalCount int64
	if err := s.db.WithContext(ctx).
		Model(&PlotRecommendation{}).
		Where("tenant_id = ?", tenantID).
		Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total_count"] = totalCount

	// 已应用次数
	var appliedCount int64
	if err := s.db.WithContext(ctx).
		Model(&PlotRecommendation{}).
		Where("tenant_id = ? AND applied = ?", tenantID, true).
		Count(&appliedCount).Error; err != nil {
		return nil, err
	}
	stats["applied_count"] = appliedCount

	// 应用率
	if totalCount > 0 {
		stats["apply_rate"] = float64(appliedCount) / float64(totalCount) * 100
	} else {
		stats["apply_rate"] = 0
	}

	return stats, nil
}
