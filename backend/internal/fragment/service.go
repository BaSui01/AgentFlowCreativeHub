package fragment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Service 片段管理服务
type Service struct {
	db *gorm.DB
}

// NewService 创建片段服务
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateFragment 创建片段
func (s *Service) CreateFragment(ctx context.Context, tenantID, userID string, req *CreateFragmentRequest) (*Fragment, error) {
	fragment := &Fragment{
		TenantID:    tenantID,
		UserID:      userID,
		Type:        req.Type,
		Title:       req.Title,
		Content:     req.Content,
		WorkspaceID: req.WorkspaceID,
		WorkID:      req.WorkID,
		ChapterID:   req.ChapterID,
		Metadata:    req.Metadata,
		Status:      FragmentStatusPending,
	}

	if req.Tags != nil {
		fragment.Tags = strings.Join(req.Tags, ",")
	}

	if req.Priority != nil {
		fragment.Priority = *req.Priority
	}

	if req.DueDate != nil {
		fragment.DueDate = req.DueDate
	}

	if err := s.db.WithContext(ctx).Create(fragment).Error; err != nil {
		return nil, fmt.Errorf("创建片段失败: %w", err)
	}

	return fragment, nil
}

// GetFragment 获取片段详情
func (s *Service) GetFragment(ctx context.Context, tenantID, fragmentID string) (*Fragment, error) {
	var fragment Fragment
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", fragmentID, tenantID).
		First(&fragment).Error; err != nil {
		return nil, err
	}
	return &fragment, nil
}

// UpdateFragment 更新片段
func (s *Service) UpdateFragment(ctx context.Context, tenantID, fragmentID string, req *UpdateFragmentRequest) (*Fragment, error) {
	fragment, err := s.GetFragment(ctx, tenantID, fragmentID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]any)

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.Tags != nil {
		updates["tags"] = strings.Join(req.Tags, ",")
	}
	if req.Status != nil {
		updates["status"] = *req.Status
		// 如果状态改为已完成，设置完成时间
		if *req.Status == FragmentStatusCompleted {
			now := time.Now()
			updates["completed_at"] = &now
		}
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.DueDate != nil {
		updates["due_date"] = *req.DueDate
	}
	if req.Metadata != nil {
		updates["metadata"] = *req.Metadata
	}

	if err := s.db.WithContext(ctx).Model(fragment).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新片段失败: %w", err)
	}

	return s.GetFragment(ctx, tenantID, fragmentID)
}

// DeleteFragment 删除片段（软删除）
func (s *Service) DeleteFragment(ctx context.Context, tenantID, fragmentID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", fragmentID, tenantID).
		Delete(&Fragment{})

	if result.Error != nil {
		return fmt.Errorf("删除片段失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// ListFragments 查询片段列表
func (s *Service) ListFragments(ctx context.Context, tenantID string, req *ListFragmentsRequest) ([]*Fragment, int64, error) {
	query := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID)

	// 过滤条件
	if req.Type != nil {
		query = query.Where("type = ?", *req.Type)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.WorkspaceID != nil {
		query = query.Where("workspace_id = ?", *req.WorkspaceID)
	}
	if req.WorkID != nil {
		query = query.Where("work_id = ?", *req.WorkID)
	}
	if req.ChapterID != nil {
		query = query.Where("chapter_id = ?", *req.ChapterID)
	}
	if req.Tags != nil && *req.Tags != "" {
		tags := strings.Split(*req.Tags, ",")
		for _, tag := range tags {
			query = query.Where("tags LIKE ?", "%"+strings.TrimSpace(tag)+"%")
		}
	}
	if req.Keyword != nil && *req.Keyword != "" {
		keyword := "%" + *req.Keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", keyword, keyword)
	}

	// 计算总数
	var total int64
	if err := query.Model(&Fragment{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	sortBy := "created_at"
	if req.SortBy != "" {
		sortBy = req.SortBy
	}
	sortOrder := "desc"
	if req.SortOrder == "asc" {
		sortOrder = "asc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

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
	query = query.Offset(offset).Limit(pageSize)

	// 查询
	var fragments []*Fragment
	if err := query.Find(&fragments).Error; err != nil {
		return nil, 0, err
	}

	return fragments, total, nil
}

// CompleteFragment 完成片段（用于待办事项）
func (s *Service) CompleteFragment(ctx context.Context, tenantID, fragmentID string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&Fragment{}).
		Where("id = ? AND tenant_id = ?", fragmentID, tenantID).
		Updates(map[string]any{
			"status":       FragmentStatusCompleted,
			"completed_at": &now,
		})

	if result.Error != nil {
		return fmt.Errorf("完成片段失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// BatchOperation 批量操作
func (s *Service) BatchOperation(ctx context.Context, tenantID string, req *BatchOperationRequest) error {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Fragment{}).
		Where("id IN ? AND tenant_id = ?", req.IDs, tenantID)

	switch req.Operation {
	case "complete":
		now := time.Now()
		if err := query.Updates(map[string]any{
			"status":       FragmentStatusCompleted,
			"completed_at": &now,
		}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("批量完成失败: %w", err)
		}

	case "archive":
		if err := query.Update("status", FragmentStatusArchived).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("批量归档失败: %w", err)
		}

	case "delete":
		if err := query.Delete(&Fragment{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("批量删除失败: %w", err)
		}

	case "change_status":
		if req.Status == nil {
			tx.Rollback()
			return fmt.Errorf("状态参数不能为空")
		}
		updates := map[string]any{"status": *req.Status}
		if *req.Status == FragmentStatusCompleted {
			now := time.Now()
			updates["completed_at"] = &now
		}
		if err := query.Updates(updates).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("批量更新状态失败: %w", err)
		}

	default:
		tx.Rollback()
		return fmt.Errorf("不支持的操作: %s", req.Operation)
	}

	return tx.Commit().Error
}

// GetStats 获取统计信息
func (s *Service) GetStats(ctx context.Context, tenantID string) (*FragmentStatsResponse, error) {
	stats := &FragmentStatsResponse{
		ByType:   make(map[string]int),
		ByStatus: make(map[string]int),
	}

	// 总数
	if err := s.db.WithContext(ctx).
		Model(&Fragment{}).
		Where("tenant_id = ?", tenantID).
		Count(&stats.TotalCount).Error; err != nil {
		return nil, err
	}

	// 按类型统计
	var typeStats []struct {
		Type  string
		Count int
	}
	if err := s.db.WithContext(ctx).
		Model(&Fragment{}).
		Select("type, COUNT(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("type").
		Scan(&typeStats).Error; err != nil {
		return nil, err
	}
	for _, stat := range typeStats {
		stats.ByType[stat.Type] = stat.Count
	}

	// 按状态统计
	var statusStats []struct {
		Status string
		Count  int
	}
	if err := s.db.WithContext(ctx).
		Model(&Fragment{}).
		Select("status, COUNT(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("status").
		Scan(&statusStats).Error; err != nil {
		return nil, err
	}
	for _, stat := range statusStats {
		stats.ByStatus[stat.Status] = stat.Count
	}

	// 待办事项统计
	if err := s.db.WithContext(ctx).
		Model(&Fragment{}).
		Where("tenant_id = ? AND type = ? AND status = ?", tenantID, FragmentTypeTodo, FragmentStatusPending).
		Count(&stats.PendingTodos).Error; err != nil {
		return nil, err
	}

	// 过期待办
	now := time.Now()
	if err := s.db.WithContext(ctx).
		Model(&Fragment{}).
		Where("tenant_id = ? AND type = ? AND status = ? AND due_date < ?", 
			tenantID, FragmentTypeTodo, FragmentStatusPending, now).
		Count(&stats.OverdueTodos).Error; err != nil {
		return nil, err
	}

	// 今日完成
	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.WithContext(ctx).
		Model(&Fragment{}).
		Where("tenant_id = ? AND status = ? AND completed_at >= ?", 
			tenantID, FragmentStatusCompleted, today).
		Count(&stats.CompletedToday).Error; err != nil {
		return nil, err
	}

	return stats, nil
}
