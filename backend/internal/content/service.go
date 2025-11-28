package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrWorkNotFound     = errors.New("作品不存在")
	ErrCategoryNotFound = errors.New("分类不存在")
	ErrReportNotFound   = errors.New("举报记录不存在")
	ErrAlreadyReported  = errors.New("已举报过该内容")
	ErrCannotPublish    = errors.New("当前状态无法发布")
)

// Service 内容管理服务
type Service struct {
	db *gorm.DB
}

// NewService 创建服务
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// ============================================================================
// 公开作品管理
// ============================================================================

// PublishWork 发布作品
func (s *Service) PublishWork(ctx context.Context, req *PublishWorkRequest) (*PublishedWork, error) {
	tagsJSON, _ := json.Marshal(req.Tags)
	wordCount := countWords(req.Content)

	work := &PublishedWork{
		ID:           uuid.New().String(),
		TenantID:     req.TenantID,
		UserID:       req.UserID,
		WorkspaceID:  req.WorkspaceID,
		FileID:       req.FileID,
		Title:        req.Title,
		Summary:      req.Summary,
		Content:      req.Content,
		CoverImage:   req.CoverImage,
		WordCount:    wordCount,
		CategoryID:   req.CategoryID,
		Tags:         string(tagsJSON),
		Status:       PublishStatusPending, // 待审核
		AllowComment: req.AllowComment,
	}

	if err := s.db.WithContext(ctx).Create(work).Error; err != nil {
		return nil, err
	}

	// 更新分类作品数
	if req.CategoryID != "" {
		s.db.Model(&ContentCategory{}).Where("id = ?", req.CategoryID).
			Update("work_count", gorm.Expr("work_count + 1"))
	}

	// 更新标签使用次数
	for _, tag := range req.Tags {
		s.db.Model(&ContentTag{}).Where("tenant_id = ? AND name = ?", req.TenantID, tag).
			Update("use_count", gorm.Expr("use_count + 1"))
	}

	return work, nil
}

// GetWork 获取作品详情
func (s *Service) GetWork(ctx context.Context, workID string) (*PublishedWork, error) {
	var work PublishedWork
	if err := s.db.WithContext(ctx).Where("id = ?", workID).First(&work).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkNotFound
		}
		return nil, err
	}
	return &work, nil
}

// UpdateWork 更新作品
func (s *Service) UpdateWork(ctx context.Context, workID string, req *UpdateWorkRequest) error {
	updates := make(map[string]interface{})

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Summary != nil {
		updates["summary"] = *req.Summary
	}
	if req.Content != nil {
		updates["content"] = *req.Content
		updates["word_count"] = countWords(*req.Content)
	}
	if req.CoverImage != nil {
		updates["cover_image"] = *req.CoverImage
	}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.Tags != nil {
		tagsJSON, _ := json.Marshal(req.Tags)
		updates["tags"] = string(tagsJSON)
	}
	if req.AllowComment != nil {
		updates["allow_comment"] = *req.AllowComment
	}

	if len(updates) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Model(&PublishedWork{}).Where("id = ?", workID).Updates(updates).Error
}

// ListWorks 获取作品列表
func (s *Service) ListWorks(ctx context.Context, query *ListWorksQuery) ([]PublishedWork, int64, error) {
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 20
	}
	if query.Page <= 0 {
		query.Page = 1
	}

	var works []PublishedWork
	var total int64

	q := s.db.WithContext(ctx).Model(&PublishedWork{}).Where("tenant_id = ?", query.TenantID)

	if query.UserID != "" {
		q = q.Where("user_id = ?", query.UserID)
	}
	if query.CategoryID != "" {
		q = q.Where("category_id = ?", query.CategoryID)
	}
	if query.Status != "" {
		q = q.Where("status = ?", query.Status)
	} else {
		// 默认只显示已发布
		q = q.Where("status = ?", PublishStatusPublished)
	}
	if query.Keyword != "" {
		q = q.Where("title ILIKE ? OR summary ILIKE ?", "%"+query.Keyword+"%", "%"+query.Keyword+"%")
	}
	if query.Tag != "" {
		q = q.Where("tags @> ?", fmt.Sprintf(`["%s"]`, query.Tag))
	}

	q.Count(&total)

	// 排序
	switch query.SortBy {
	case "popular":
		q = q.Order("view_count DESC, like_count DESC")
	case "recommend":
		q = q.Order("is_recommend DESC, is_featured DESC, sort_weight DESC, published_at DESC")
	default: // latest
		q = q.Order("published_at DESC, created_at DESC")
	}

	q.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&works)

	return works, total, nil
}

// ReviewWork 审核作品
func (s *Service) ReviewWork(ctx context.Context, req *ReviewWorkRequest) error {
	now := time.Now()
	updates := map[string]interface{}{
		"reviewed_at": now,
		"reviewed_by": req.ReviewerID,
	}

	if req.Action == "approve" {
		updates["status"] = PublishStatusPublished
		updates["published_at"] = now
	} else if req.Action == "reject" {
		updates["status"] = PublishStatusRejected
		updates["reject_reason"] = req.RejectReason
	} else {
		return fmt.Errorf("无效的操作: %s", req.Action)
	}

	result := s.db.WithContext(ctx).Model(&PublishedWork{}).
		Where("id = ? AND status = ?", req.WorkID, PublishStatusPending).
		Updates(updates)

	if result.RowsAffected == 0 {
		return ErrCannotPublish
	}

	return result.Error
}

// SetRecommend 设置推荐
func (s *Service) SetRecommend(ctx context.Context, req *RecommendWorkRequest) error {
	return s.db.WithContext(ctx).Model(&PublishedWork{}).
		Where("id = ?", req.WorkID).
		Updates(map[string]interface{}{
			"is_recommend": req.IsRecommend,
			"is_featured":  req.IsFeatured,
			"sort_weight":  req.SortWeight,
		}).Error
}

// OfflineWork 下架作品
func (s *Service) OfflineWork(ctx context.Context, workID, reason string) error {
	return s.db.WithContext(ctx).Model(&PublishedWork{}).
		Where("id = ? AND status = ?", workID, PublishStatusPublished).
		Updates(map[string]interface{}{
			"status":        PublishStatusOffline,
			"reject_reason": reason,
		}).Error
}

// DeleteWork 删除作品
func (s *Service) DeleteWork(ctx context.Context, workID string) error {
	return s.db.WithContext(ctx).Delete(&PublishedWork{}, "id = ?", workID).Error
}

// SearchWorks 搜索作品（支持全文搜索和高级筛选）
func (s *Service) SearchWorks(ctx context.Context, req *SearchWorksRequest) ([]PublishedWork, int64, error) {
	// 设置默认分页参数
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	var works []PublishedWork
	var total int64

	// 构建查询
	q := s.db.WithContext(ctx).Model(&PublishedWork{}).Where("tenant_id = ?", req.TenantID)

	// 关键词搜索（标题、摘要、内容）
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		q = q.Where("title ILIKE ? OR summary ILIKE ? OR content ILIKE ?", keyword, keyword, keyword)
	}

	// 用户ID筛选
	if req.UserID != "" {
		q = q.Where("user_id = ?", req.UserID)
	}

	// 分类ID筛选（支持多个分类，OR关系）
	if len(req.CategoryIDs) > 0 {
		q = q.Where("category_id IN ?", req.CategoryIDs)
	}

	// 标签筛选（支持多个标签，AND关系）
	if len(req.Tags) > 0 {
		for _, tag := range req.Tags {
			q = q.Where("tags @> ?", fmt.Sprintf(`["%s"]`, tag))
		}
	}

	// 状态筛选
	if req.Status != "" {
		q = q.Where("status = ?", req.Status)
	} else {
		// 默认只显示已发布的作品
		q = q.Where("status = ?", PublishStatusPublished)
	}

	// 推荐筛选
	if req.IsRecommend != nil {
		q = q.Where("is_recommend = ?", *req.IsRecommend)
	}

	// 精选筛选
	if req.IsFeatured != nil {
		q = q.Where("is_featured = ?", *req.IsFeatured)
	}

	// 统计筛选
	if req.MinViewCount > 0 {
		q = q.Where("view_count >= ?", req.MinViewCount)
	}
	if req.MinLikeCount > 0 {
		q = q.Where("like_count >= ?", req.MinLikeCount)
	}
	if req.MinWordCount > 0 {
		q = q.Where("word_count >= ?", req.MinWordCount)
	}
	if req.MaxWordCount > 0 {
		q = q.Where("word_count <= ?", req.MaxWordCount)
	}

	// 日期范围筛选
	if req.PublishedAfter != nil {
		q = q.Where("published_at >= ?", *req.PublishedAfter)
	}
	if req.PublishedBefore != nil {
		q = q.Where("published_at <= ?", *req.PublishedBefore)
	}

	// 统计总数
	q.Count(&total)

	// 排序
	sortOrder := "DESC"
	if !req.SortDesc {
		sortOrder = "ASC"
	}

	switch req.SortBy {
	case "views":
		q = q.Order(fmt.Sprintf("view_count %s, published_at DESC", sortOrder))
	case "likes":
		q = q.Order(fmt.Sprintf("like_count %s, published_at DESC", sortOrder))
	case "popular":
		q = q.Order(fmt.Sprintf("view_count %s, like_count %s, published_at DESC", sortOrder, sortOrder))
	case "recommend":
		q = q.Order(fmt.Sprintf("is_recommend DESC, is_featured DESC, sort_weight DESC, published_at %s", sortOrder))
	default: // latest
		q = q.Order(fmt.Sprintf("published_at %s, created_at %s", sortOrder, sortOrder))
	}

	// 分页查询
	q.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Find(&works)

	return works, total, nil
}

// IncrementView 增加浏览量
func (s *Service) IncrementView(ctx context.Context, workID string) error {
	return s.db.WithContext(ctx).Model(&PublishedWork{}).
		Where("id = ?", workID).
		Update("view_count", gorm.Expr("view_count + 1")).Error
}

// IncrementLike 增加点赞量
func (s *Service) IncrementLike(ctx context.Context, workID string, delta int) error {
	return s.db.WithContext(ctx).Model(&PublishedWork{}).
		Where("id = ?", workID).
		Update("like_count", gorm.Expr("like_count + ?", delta)).Error
}

// ============================================================================
// 分类管理
// ============================================================================

// CreateCategory 创建分类
func (s *Service) CreateCategory(ctx context.Context, req *CreateCategoryRequest) (*ContentCategory, error) {
	category := &ContentCategory{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		ParentID:    req.ParentID,
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Icon:        req.Icon,
		CoverImage:  req.CoverImage,
		IsActive:    true,
	}

	if category.Code == "" {
		category.Code = uuid.New().String()[:8]
	}

	if err := s.db.WithContext(ctx).Create(category).Error; err != nil {
		return nil, err
	}

	return category, nil
}

// GetCategory 获取分类
func (s *Service) GetCategory(ctx context.Context, categoryID string) (*ContentCategory, error) {
	var category ContentCategory
	if err := s.db.WithContext(ctx).Where("id = ?", categoryID).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, err
	}
	return &category, nil
}

// ListCategories 获取分类列表
func (s *Service) ListCategories(ctx context.Context, tenantID, parentID string) ([]ContentCategory, error) {
	var categories []ContentCategory
	q := s.db.WithContext(ctx).Where("tenant_id = ? AND is_active = ?", tenantID, true)
	
	if parentID != "" {
		q = q.Where("parent_id = ?", parentID)
	} else {
		q = q.Where("parent_id IS NULL OR parent_id = ''")
	}
	
	if err := q.Order("sort_order, name").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// UpdateCategory 更新分类
func (s *Service) UpdateCategory(ctx context.Context, categoryID string, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&ContentCategory{}).Where("id = ?", categoryID).Updates(updates).Error
}

// DeleteCategory 删除分类
func (s *Service) DeleteCategory(ctx context.Context, categoryID string) error {
	return s.db.WithContext(ctx).Model(&ContentCategory{}).Where("id = ?", categoryID).Update("is_active", false).Error
}

// ============================================================================
// 标签管理
// ============================================================================

// GetOrCreateTag 获取或创建标签
func (s *Service) GetOrCreateTag(ctx context.Context, tenantID, name string) (*ContentTag, error) {
	var tag ContentTag
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND name = ?", tenantID, name).First(&tag).Error
	if err == nil {
		return &tag, nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		tag = ContentTag{
			ID:       uuid.New().String(),
			TenantID: tenantID,
			Name:     name,
		}
		if err := s.db.WithContext(ctx).Create(&tag).Error; err != nil {
			return nil, err
		}
		return &tag, nil
	}

	return nil, err
}

// ListTags 获取标签列表
func (s *Service) ListTags(ctx context.Context, tenantID string, hotOnly bool, limit int) ([]ContentTag, error) {
	var tags []ContentTag
	q := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	
	if hotOnly {
		q = q.Where("is_hot = ?", true)
	}
	
	if limit <= 0 {
		limit = 50
	}
	
	if err := q.Order("use_count DESC").Limit(limit).Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// SetHotTag 设置热门标签
func (s *Service) SetHotTag(ctx context.Context, tagID string, isHot bool) error {
	return s.db.WithContext(ctx).Model(&ContentTag{}).Where("id = ?", tagID).Update("is_hot", isHot).Error
}

// ============================================================================
// 举报管理
// ============================================================================

// CreateReport 创建举报
func (s *Service) CreateReport(ctx context.Context, req *CreateReportRequest) (*ContentReport, error) {
	// 检查是否已举报
	var count int64
	s.db.WithContext(ctx).Model(&ContentReport{}).
		Where("work_id = ? AND reporter_id = ? AND status IN ?", req.WorkID, req.ReporterID, 
			[]ReportStatus{ReportStatusPending, ReportStatusReviewing}).
		Count(&count)
	if count > 0 {
		return nil, ErrAlreadyReported
	}

	report := &ContentReport{
		ID:         uuid.New().String(),
		TenantID:   req.TenantID,
		WorkID:     req.WorkID,
		ReporterID: req.ReporterID,
		ReportType: req.ReportType,
		Reason:     req.Reason,
		Evidence:   req.Evidence,
		Status:     ReportStatusPending,
	}

	if err := s.db.WithContext(ctx).Create(report).Error; err != nil {
		return nil, err
	}

	return report, nil
}

// GetReport 获取举报详情
func (s *Service) GetReport(ctx context.Context, reportID string) (*ContentReport, error) {
	var report ContentReport
	if err := s.db.WithContext(ctx).Where("id = ?", reportID).First(&report).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReportNotFound
		}
		return nil, err
	}
	return &report, nil
}

// ListReports 获取举报列表
func (s *Service) ListReports(ctx context.Context, tenantID string, status ReportStatus, page, pageSize int) ([]ContentReport, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var reports []ContentReport
	var total int64

	q := s.db.WithContext(ctx).Model(&ContentReport{}).Where("tenant_id = ?", tenantID)
	if status != "" {
		q = q.Where("status = ?", status)
	}

	q.Count(&total)
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&reports)

	return reports, total, nil
}

// HandleReport 处理举报
func (s *Service) HandleReport(ctx context.Context, req *HandleReportRequest) error {
	now := time.Now()
	
	// 更新举报状态
	err := s.db.WithContext(ctx).Model(&ContentReport{}).
		Where("id = ? AND status IN ?", req.ReportID, []ReportStatus{ReportStatusPending, ReportStatusReviewing}).
		Updates(map[string]interface{}{
			"status":        ReportStatusResolved,
			"handler_id":    req.HandlerID,
			"handle_note":   req.HandleNote,
			"handle_result": req.Action,
			"handled_at":    now,
		}).Error
	if err != nil {
		return err
	}

	// 根据处理结果对作品进行操作
	report, _ := s.GetReport(ctx, req.ReportID)
	if report == nil {
		return nil
	}

	switch req.Action {
	case "offline":
		// 下架作品
		s.OfflineWork(ctx, report.WorkID, "因违规被举报下架")
	case "ban":
		// 下架并标记
		s.OfflineWork(ctx, report.WorkID, "严重违规，永久下架")
	}

	return nil
}

// ============================================================================
// 统计
// ============================================================================

// GetContentStats 获取内容统计
func (s *Service) GetContentStats(ctx context.Context, tenantID string) (*ContentStats, error) {
	stats := &ContentStats{TenantID: tenantID}

	// 作品统计
	s.db.WithContext(ctx).Model(&PublishedWork{}).Where("tenant_id = ?", tenantID).Count(&stats.TotalWorks)
	s.db.WithContext(ctx).Model(&PublishedWork{}).Where("tenant_id = ? AND status = ?", tenantID, PublishStatusPublished).Count(&stats.PublishedWorks)
	s.db.WithContext(ctx).Model(&PublishedWork{}).Where("tenant_id = ? AND status = ?", tenantID, PublishStatusPending).Count(&stats.PendingWorks)

	// 互动统计
	var viewSum, likeSum, commentSum struct{ Total int64 }
	s.db.WithContext(ctx).Model(&PublishedWork{}).Select("COALESCE(SUM(view_count),0) as total").Where("tenant_id = ?", tenantID).Scan(&viewSum)
	s.db.WithContext(ctx).Model(&PublishedWork{}).Select("COALESCE(SUM(like_count),0) as total").Where("tenant_id = ?", tenantID).Scan(&likeSum)
	s.db.WithContext(ctx).Model(&PublishedWork{}).Select("COALESCE(SUM(comment_count),0) as total").Where("tenant_id = ?", tenantID).Scan(&commentSum)
	stats.TotalViews = viewSum.Total
	stats.TotalLikes = likeSum.Total
	stats.TotalComments = commentSum.Total

	// 举报统计
	s.db.WithContext(ctx).Model(&ContentReport{}).Where("tenant_id = ?", tenantID).Count(&stats.TotalReports)
	s.db.WithContext(ctx).Model(&ContentReport{}).Where("tenant_id = ? AND status = ?", tenantID, ReportStatusPending).Count(&stats.PendingReports)

	return stats, nil
}

// GetRecommendWorks 获取推荐作品
func (s *Service) GetRecommendWorks(ctx context.Context, tenantID string, limit int) ([]PublishedWork, error) {
	if limit <= 0 {
		limit = 10
	}

	var works []PublishedWork
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ? AND (is_recommend = ? OR is_featured = ?)", 
			tenantID, PublishStatusPublished, true, true).
		Order("is_featured DESC, sort_weight DESC, view_count DESC").
		Limit(limit).
		Find(&works).Error

	return works, err
}

// AutoMigrate 自动迁移表结构
func (s *Service) AutoMigrate() error {
	return s.db.AutoMigrate(
		&PublishedWork{},
		&ContentCategory{},
		&ContentTag{},
		&ContentReport{},
	)
}

// ============================================================================
// 辅助函数
// ============================================================================

func countWords(text string) int {
	chineseCount := 0
	englishWords := 0
	inWord := false

	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
			inWord = false
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			if !inWord {
				englishWords++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	return chineseCount + englishWords
}
