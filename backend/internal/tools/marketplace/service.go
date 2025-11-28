package marketplace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPackageNotFound    = errors.New("工具包不存在")
	ErrVersionNotFound    = errors.New("版本不存在")
	ErrPackageExists      = errors.New("工具包名称已存在")
	ErrVersionExists      = errors.New("版本已存在")
	ErrPermissionDenied   = errors.New("权限不足")
	ErrInvalidVersion     = errors.New("无效的版本号")
	ErrAlreadyInstalled   = errors.New("工具已安装")
	ErrNotInstalled       = errors.New("工具未安装")
)

// Service 工具市场服务
type Service struct {
	db *gorm.DB
}

// NewService 创建工具市场服务
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// AutoMigrate 自动迁移数据库表
func (s *Service) AutoMigrate() error {
	return s.db.AutoMigrate(
		&ToolPackage{},
		&ToolVersion{},
		&ToolRating{},
		&ToolInstall{},
	)
}

// --- 发布管理 ---

// Publish 发布工具包
func (s *Service) Publish(ctx context.Context, tenantID, userID, userName string, req *PublishRequest) (*ToolPackage, *ToolVersion, error) {
	// 检查名称是否已存在
	var existing ToolPackage
	if err := s.db.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		return nil, nil, ErrPackageExists
	}

	now := time.Now()
	visibility := req.Visibility
	if visibility == "" {
		visibility = "public"
	}

	// 创建工具包
	pkg := &ToolPackage{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		AuthorID:    userID,
		AuthorName:  userName,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Category:    req.Category,
		Tags:        req.Tags,
		Icon:        req.Icon,
		Homepage:    req.Homepage,
		Repository:  req.Repository,
		Status:      "approved", // 默认自动通过，可改为 pending 需审核
		Visibility:  visibility,
		PublishedAt: &now,
	}

	// 创建版本
	version := &ToolVersion{
		ID:          uuid.New().String(),
		PackageID:   pkg.ID,
		TenantID:    tenantID,
		Version:     req.Version,
		Changelog:   req.Changelog,
		Definition:  req.Definition,
		Status:      "active",
		IsLatest:    true,
		PublishedAt: now,
	}

	// 事务创建
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(pkg).Error; err != nil {
			return err
		}
		return tx.Create(version).Error
	})

	if err != nil {
		return nil, nil, err
	}

	return pkg, version, nil
}

// UpdatePackage 更新工具包信息
func (s *Service) UpdatePackage(ctx context.Context, tenantID, userID, packageID string, req *UpdatePackageRequest) (*ToolPackage, error) {
	var pkg ToolPackage
	if err := s.db.Where("id = ?", packageID).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	// 检查权限
	if pkg.AuthorID != userID && tenantID != "" && pkg.TenantID != tenantID {
		return nil, ErrPermissionDenied
	}

	updates := map[string]interface{}{}
	if req.DisplayName != "" {
		updates["display_name"] = req.DisplayName
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Tags != nil {
		updates["tags"] = req.Tags
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	if req.Homepage != "" {
		updates["homepage"] = req.Homepage
	}
	if req.Repository != "" {
		updates["repository"] = req.Repository
	}
	if req.Visibility != "" {
		updates["visibility"] = req.Visibility
	}

	if err := s.db.Model(&pkg).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.db.First(&pkg, "id = ?", packageID)
	return &pkg, nil
}

// DeletePackage 删除工具包（软删除）
func (s *Service) DeletePackage(ctx context.Context, tenantID, userID, packageID string) error {
	var pkg ToolPackage
	if err := s.db.Where("id = ?", packageID).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPackageNotFound
		}
		return err
	}

	// 检查权限
	if pkg.AuthorID != userID {
		return ErrPermissionDenied
	}

	now := time.Now()
	return s.db.Model(&pkg).Update("deleted_at", &now).Error
}

// --- 版本管理 ---

// PublishVersion 发布新版本
func (s *Service) PublishVersion(ctx context.Context, tenantID, userID, packageID string, req *PublishVersionRequest) (*ToolVersion, error) {
	var pkg ToolPackage
	if err := s.db.Where("id = ?", packageID).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	// 检查权限
	if pkg.AuthorID != userID {
		return nil, ErrPermissionDenied
	}

	// 检查版本是否已存在
	var existing ToolVersion
	if err := s.db.Where("package_id = ? AND version = ?", packageID, req.Version).First(&existing).Error; err == nil {
		return nil, ErrVersionExists
	}

	now := time.Now()
	version := &ToolVersion{
		ID:          uuid.New().String(),
		PackageID:   packageID,
		TenantID:    tenantID,
		Version:     req.Version,
		Changelog:   req.Changelog,
		MinVersion:  req.MinVersion,
		Definition:  req.Definition,
		Status:      "active",
		IsLatest:    true,
		PublishedAt: now,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 将之前的最新版本标记为非最新
		if err := tx.Model(&ToolVersion{}).
			Where("package_id = ? AND is_latest = ?", packageID, true).
			Update("is_latest", false).Error; err != nil {
			return err
		}
		return tx.Create(version).Error
	})

	if err != nil {
		return nil, err
	}

	return version, nil
}

// GetVersions 获取版本列表
func (s *Service) GetVersions(ctx context.Context, packageID string) ([]ToolVersion, error) {
	var versions []ToolVersion
	err := s.db.Where("package_id = ?", packageID).
		Order("created_at DESC").
		Find(&versions).Error
	return versions, err
}

// GetVersion 获取指定版本
func (s *Service) GetVersion(ctx context.Context, packageID, version string) (*ToolVersion, error) {
	var v ToolVersion
	err := s.db.Where("package_id = ? AND version = ?", packageID, version).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrVersionNotFound
	}
	return &v, err
}

// DeprecateVersion 废弃版本
func (s *Service) DeprecateVersion(ctx context.Context, tenantID, userID, packageID, version string) error {
	var pkg ToolPackage
	if err := s.db.Where("id = ?", packageID).First(&pkg).Error; err != nil {
		return ErrPackageNotFound
	}

	if pkg.AuthorID != userID {
		return ErrPermissionDenied
	}

	return s.db.Model(&ToolVersion{}).
		Where("package_id = ? AND version = ?", packageID, version).
		Update("status", "deprecated").Error
}

// --- 搜索 ---

// Search 搜索工具包
func (s *Service) Search(ctx context.Context, req *SearchRequest) (*PackageListResponse, error) {
	query := s.db.Model(&ToolPackage{}).Where("deleted_at IS NULL")

	// 默认只显示已通过审核的公开工具
	if req.Status == "" {
		query = query.Where("status = ?", "approved")
	} else {
		query = query.Where("status = ?", req.Status)
	}

	if req.Visibility == "" {
		query = query.Where("visibility = ?", "public")
	} else {
		query = query.Where("visibility = ?", req.Visibility)
	}

	// 关键词搜索
	if req.Query != "" {
		keyword := "%" + strings.ToLower(req.Query) + "%"
		query = query.Where(
			"LOWER(name) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(description) LIKE ?",
			keyword, keyword, keyword,
		)
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

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 排序
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "downloads"
	}
	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	orderField := map[string]string{
		"downloads": "downloads",
		"stars":     "stars",
		"rating":    "rating_avg",
		"created":   "created_at",
		"updated":   "updated_at",
	}[sortBy]
	if orderField == "" {
		orderField = "downloads"
	}
	query = query.Order(fmt.Sprintf("%s %s", orderField, sortOrder))

	// 分页
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var packages []ToolPackage
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&packages).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &PackageListResponse{
		Packages:   packages,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetPackage 获取工具包详情
func (s *Service) GetPackage(ctx context.Context, tenantID, userID, packageID string) (*PackageDetailResponse, error) {
	var pkg ToolPackage
	if err := s.db.Where("id = ? AND deleted_at IS NULL", packageID).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	// 获取最新版本
	var latestVersion ToolVersion
	s.db.Where("package_id = ? AND is_latest = ?", packageID, true).First(&latestVersion)

	// 获取所有版本
	var versions []ToolVersion
	s.db.Where("package_id = ?", packageID).Order("created_at DESC").Find(&versions)

	// 获取评分列表
	var ratings []ToolRating
	s.db.Where("package_id = ?", packageID).Order("created_at DESC").Limit(10).Find(&ratings)

	// 检查用户是否已安装
	var install ToolInstall
	isInstalled := s.db.Where("package_id = ? AND user_id = ? AND status = ?", packageID, userID, "installed").First(&install).Error == nil

	// 获取用户评分
	var userRating *ToolRating
	var rating ToolRating
	if err := s.db.Where("package_id = ? AND user_id = ?", packageID, userID).First(&rating).Error; err == nil {
		userRating = &rating
	}

	return &PackageDetailResponse{
		Package:       &pkg,
		LatestVersion: &latestVersion,
		Versions:      versions,
		Ratings:       ratings,
		IsInstalled:   isInstalled,
		UserRating:    userRating,
	}, nil
}

// GetPackageByName 根据名称获取工具包
func (s *Service) GetPackageByName(ctx context.Context, name string) (*ToolPackage, error) {
	var pkg ToolPackage
	err := s.db.Where("name = ? AND deleted_at IS NULL", name).First(&pkg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPackageNotFound
	}
	return &pkg, err
}

// --- 评分 ---

// Rate 评分
func (s *Service) Rate(ctx context.Context, tenantID, userID, packageID string, req *RatingRequest) (*ToolRating, error) {
	var pkg ToolPackage
	if err := s.db.Where("id = ?", packageID).First(&pkg).Error; err != nil {
		return nil, ErrPackageNotFound
	}

	// 查找或创建评分记录
	var rating ToolRating
	err := s.db.Where("package_id = ? AND user_id = ?", packageID, userID).First(&rating).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)

	if isNew {
		rating = ToolRating{
			ID:        uuid.New().String(),
			PackageID: packageID,
			TenantID:  tenantID,
			UserID:    userID,
		}
	}

	rating.Rating = req.Rating
	rating.Review = req.Review
	rating.Version = req.Version

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if isNew {
			if err := tx.Create(&rating).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Save(&rating).Error; err != nil {
				return err
			}
		}

		// 更新工具包的平均评分
		var avgRating float64
		var count int64
		tx.Model(&ToolRating{}).Where("package_id = ?", packageID).Count(&count)
		tx.Model(&ToolRating{}).Where("package_id = ?", packageID).Select("AVG(rating)").Scan(&avgRating)

		return tx.Model(&pkg).Updates(map[string]interface{}{
			"rating_avg":   avgRating,
			"rating_count": count,
		}).Error
	})

	if err != nil {
		return nil, err
	}

	return &rating, nil
}

// GetRatings 获取评分列表
func (s *Service) GetRatings(ctx context.Context, packageID string, page, pageSize int) ([]ToolRating, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	s.db.Model(&ToolRating{}).Where("package_id = ?", packageID).Count(&total)

	var ratings []ToolRating
	err := s.db.Where("package_id = ?", packageID).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&ratings).Error

	return ratings, total, err
}

// --- 安装/卸载 ---

// Install 安装工具
func (s *Service) Install(ctx context.Context, tenantID, userID, packageID, version string) (*ToolInstall, error) {
	var pkg ToolPackage
	if err := s.db.Where("id = ?", packageID).First(&pkg).Error; err != nil {
		return nil, ErrPackageNotFound
	}

	// 检查是否已安装
	var existing ToolInstall
	if err := s.db.Where("package_id = ? AND user_id = ? AND status = ?", packageID, userID, "installed").First(&existing).Error; err == nil {
		return nil, ErrAlreadyInstalled
	}

	// 获取版本
	var v ToolVersion
	if version == "" {
		if err := s.db.Where("package_id = ? AND is_latest = ?", packageID, true).First(&v).Error; err != nil {
			return nil, ErrVersionNotFound
		}
	} else {
		if err := s.db.Where("package_id = ? AND version = ?", packageID, version).First(&v).Error; err != nil {
			return nil, ErrVersionNotFound
		}
	}

	install := &ToolInstall{
		ID:        uuid.New().String(),
		PackageID: packageID,
		VersionID: v.ID,
		TenantID:  tenantID,
		UserID:    userID,
		Version:   v.Version,
		Status:    "installed",
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(install).Error; err != nil {
			return err
		}

		// 更新下载计数
		tx.Model(&pkg).Update("downloads", gorm.Expr("downloads + 1"))
		tx.Model(&v).Update("downloads", gorm.Expr("downloads + 1"))

		return nil
	})

	if err != nil {
		return nil, err
	}

	return install, nil
}

// Uninstall 卸载工具
func (s *Service) Uninstall(ctx context.Context, tenantID, userID, packageID string) error {
	var install ToolInstall
	if err := s.db.Where("package_id = ? AND user_id = ? AND status = ?", packageID, userID, "installed").First(&install).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotInstalled
		}
		return err
	}

	now := time.Now()
	return s.db.Model(&install).Updates(map[string]interface{}{
		"status":         "uninstalled",
		"uninstalled_at": &now,
	}).Error
}

// ListInstalled 获取已安装的工具
func (s *Service) ListInstalled(ctx context.Context, tenantID, userID string) ([]ToolPackage, error) {
	var packages []ToolPackage
	err := s.db.
		Joins("JOIN tool_installs ON tool_installs.package_id = tool_packages.id").
		Where("tool_installs.user_id = ? AND tool_installs.status = ?", userID, "installed").
		Find(&packages).Error
	return packages, err
}

// --- 统计 ---

// GetStats 获取市场统计
func (s *Service) GetStats(ctx context.Context) (*MarketplaceStats, error) {
	stats := &MarketplaceStats{
		CategoryStats: make(map[string]int64),
	}

	// 总工具数
	s.db.Model(&ToolPackage{}).Where("status = ? AND deleted_at IS NULL", "approved").Count(&stats.TotalPackages)

	// 总下载数
	s.db.Model(&ToolPackage{}).Where("deleted_at IS NULL").Select("COALESCE(SUM(downloads), 0)").Scan(&stats.TotalDownloads)

	// 总安装数
	s.db.Model(&ToolInstall{}).Where("status = ?", "installed").Count(&stats.TotalInstalls)

	// 分类统计
	type catCount struct {
		Category string
		Count    int64
	}
	var catCounts []catCount
	s.db.Model(&ToolPackage{}).
		Where("status = ? AND deleted_at IS NULL", "approved").
		Select("category, COUNT(*) as count").
		Group("category").
		Scan(&catCounts)
	for _, c := range catCounts {
		stats.CategoryStats[c.Category] = c.Count
	}

	// 热门工具
	s.db.Where("status = ? AND deleted_at IS NULL", "approved").
		Order("downloads DESC").
		Limit(10).
		Find(&stats.TopPackages)

	// 最新工具
	s.db.Where("status = ? AND deleted_at IS NULL", "approved").
		Order("created_at DESC").
		Limit(10).
		Find(&stats.RecentPackages)

	return stats, nil
}

// --- 管理员功能 ---

// ApprovePackage 审核通过
func (s *Service) ApprovePackage(ctx context.Context, packageID string) error {
	now := time.Now()
	return s.db.Model(&ToolPackage{}).Where("id = ?", packageID).Updates(map[string]interface{}{
		"status":       "approved",
		"published_at": &now,
	}).Error
}

// RejectPackage 审核拒绝
func (s *Service) RejectPackage(ctx context.Context, packageID string) error {
	return s.db.Model(&ToolPackage{}).Where("id = ?", packageID).Update("status", "rejected").Error
}

// DeprecatePackage 废弃工具包
func (s *Service) DeprecatePackage(ctx context.Context, packageID string) error {
	return s.db.Model(&ToolPackage{}).Where("id = ?", packageID).Update("status", "deprecated").Error
}

// ListPendingPackages 获取待审核的工具包
func (s *Service) ListPendingPackages(ctx context.Context, page, pageSize int) (*PackageListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	s.db.Model(&ToolPackage{}).Where("status = ?", "pending").Count(&total)

	var packages []ToolPackage
	err := s.db.Where("status = ?", "pending").
		Order("created_at ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&packages).Error

	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &PackageListResponse{
		Packages:   packages,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}
