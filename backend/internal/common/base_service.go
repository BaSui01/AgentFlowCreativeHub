package common

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// BaseService 服务基类，封装通用的数据库操作方法
// 所有业务Service可以嵌入此基类来复用通用功能
type BaseService struct {
	DB *gorm.DB
}

// NewBaseService 创建BaseService实例
func NewBaseService(db *gorm.DB) *BaseService {
	return &BaseService{DB: db}
}

// ============================================================================
// 租户过滤
// ============================================================================

// ApplyTenantFilter 应用租户过滤条件
// tenantID: 租户ID
// 返回应用了租户过滤的查询对象
func (s *BaseService) ApplyTenantFilter(query *gorm.DB, tenantID string) *gorm.DB {
	if tenantID != "" {
		return query.Where("tenant_id = ?", tenantID)
	}
	return query
}

// ============================================================================
// 软删除过滤
// ============================================================================

// ApplySoftDelete 应用软删除过滤条件（排除已删除的记录）
func (s *BaseService) ApplySoftDelete(query *gorm.DB) *gorm.DB {
	return query.Where("deleted_at IS NULL")
}

// ApplyIncludeDeleted 包含已删除的记录（不过滤软删除）
func (s *BaseService) ApplyIncludeDeleted(query *gorm.DB) *gorm.DB {
	return query.Unscoped()
}

// ============================================================================
// 分页
// ============================================================================

// ApplyPagination 应用分页条件
// page: 页码（从1开始）
// pageSize: 每页数量
func (s *BaseService) ApplyPagination(query *gorm.DB, page, pageSize int) *gorm.DB {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	return query.Offset(offset).Limit(pageSize)
}

// ApplyPaginationRequest 应用分页请求参数
func (s *BaseService) ApplyPaginationRequest(query *gorm.DB, req PaginationRequest) *gorm.DB {
	return s.ApplyPagination(query, req.Page, req.GetPageSize())
}

// ============================================================================
// 排序
// ============================================================================

// ApplySorting 应用排序条件
// sortBy: 排序字段
// sortOrder: 排序方向 (asc/desc)
// allowedFields: 允许的排序字段列表（安全检查）
func (s *BaseService) ApplySorting(query *gorm.DB, sortBy, sortOrder string, allowedFields []string) *gorm.DB {
	// 默认排序
	if sortBy == "" {
		return query.Order("created_at DESC")
	}

	// 字段白名单检查
	if len(allowedFields) > 0 {
		allowed := false
		for _, field := range allowedFields {
			if field == sortBy {
				allowed = true
				break
			}
		}
		if !allowed {
			return query.Order("created_at DESC")
		}
	}

	// 排序方向检查
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	return query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))
}

// ============================================================================
// 关键词搜索
// ============================================================================

// ApplyKeywordSearch 应用关键词模糊搜索
// keyword: 搜索关键词
// fields: 搜索字段列表
// 示例: ApplyKeywordSearch(query, "test", []string{"name", "description"})
func (s *BaseService) ApplyKeywordSearch(query *gorm.DB, keyword string, fields []string) *gorm.DB {
	if keyword == "" || len(fields) == 0 {
		return query
	}

	// 构建OR条件
	var conditions []string
	var args []interface{}
	for _, field := range fields {
		conditions = append(conditions, fmt.Sprintf("%s LIKE ?", field))
		args = append(args, "%"+keyword+"%")
	}

	whereClause := "(" + conditions[0]
	for i := 1; i < len(conditions); i++ {
		whereClause += " OR " + conditions[i]
	}
	whereClause += ")"

	return query.Where(whereClause, args...)
}

// ============================================================================
// 状态过滤
// ============================================================================

// ApplyStatusFilter 应用状态过滤
func (s *BaseService) ApplyStatusFilter(query *gorm.DB, status string) *gorm.DB {
	if status != "" {
		return query.Where("status = ?", status)
	}
	return query
}

// ============================================================================
// 日期范围过滤
// ============================================================================

// ApplyDateRangeFilter 应用日期范围过滤
// fieldName: 日期字段名称（如 created_at）
// dateRange: 日期范围
func (s *BaseService) ApplyDateRangeFilter(query *gorm.DB, fieldName string, dateRange *DateRange) *gorm.DB {
	if dateRange == nil {
		return query
	}

	if !dateRange.Start.IsZero() {
		query = query.Where(fmt.Sprintf("%s >= ?", fieldName), dateRange.Start)
	}

	if !dateRange.End.IsZero() {
		query = query.Where(fmt.Sprintf("%s <= ?", fieldName), dateRange.End)
	}

	return query
}

// ============================================================================
// 批量查询
// ============================================================================

// FindByIDs 根据ID列表批量查询
func (s *BaseService) FindByIDs(ctx context.Context, model interface{}, ids []string) error {
	return s.DB.WithContext(ctx).Where("id IN ?", ids).Find(model).Error
}

// ============================================================================
// 通用CRUD操作
// ============================================================================

// Create 创建记录
func (s *BaseService) Create(ctx context.Context, model interface{}) error {
	return s.DB.WithContext(ctx).Create(model).Error
}

// Update 更新记录
func (s *BaseService) Update(ctx context.Context, model interface{}) error {
	return s.DB.WithContext(ctx).Save(model).Error
}

// Delete 删除记录（硬删除）
func (s *BaseService) Delete(ctx context.Context, model interface{}) error {
	return s.DB.WithContext(ctx).Delete(model).Error
}

// SoftDelete 软删除记录
func (s *BaseService) SoftDelete(ctx context.Context, model interface{}, operatorID string) error {
	if softDeleteModel, ok := model.(interface{ SoftDelete(string) }); ok {
		softDeleteModel.SoftDelete(operatorID)
		return s.Update(ctx, model)
	}
	return fmt.Errorf("model does not support soft delete")
}

// FindByID 根据ID查询单条记录
func (s *BaseService) FindByID(ctx context.Context, model interface{}, id string) error {
	return s.DB.WithContext(ctx).Where("id = ?", id).First(model).Error
}

// Exists 检查记录是否存在
func (s *BaseService) Exists(ctx context.Context, model interface{}, condition string, args ...interface{}) (bool, error) {
	var count int64
	err := s.DB.WithContext(ctx).Model(model).Where(condition, args...).Count(&count).Error
	return count > 0, err
}

// ============================================================================
// 事务支持
// ============================================================================

// Transaction 执行事务
func (s *BaseService) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.DB.WithContext(ctx).Transaction(fn)
}

// WithTransaction 使用指定的事务对象创建新的BaseService
func (s *BaseService) WithTransaction(tx *gorm.DB) *BaseService {
	return &BaseService{DB: tx}
}

// ============================================================================
// 计数与统计
// ============================================================================

// Count 统计记录数
func (s *BaseService) Count(ctx context.Context, model interface{}, condition string, args ...interface{}) (int64, error) {
	var count int64
	query := s.DB.WithContext(ctx).Model(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err := query.Count(&count).Error
	return count, err
}

// CountWithQuery 使用自定义查询统计
func (s *BaseService) CountWithQuery(ctx context.Context, query *gorm.DB) (int64, error) {
	var count int64
	err := query.WithContext(ctx).Count(&count).Error
	return count, err
}

// ============================================================================
// 批量操作
// ============================================================================

// BatchCreate 批量创建记录
func (s *BaseService) BatchCreate(ctx context.Context, models interface{}, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 100
	}
	return s.DB.WithContext(ctx).CreateInBatches(models, batchSize).Error
}

// BatchUpdate 批量更新记录
func (s *BaseService) BatchUpdate(ctx context.Context, model interface{}, updates map[string]interface{}, condition string, args ...interface{}) error {
	query := s.DB.WithContext(ctx).Model(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	return query.Updates(updates).Error
}

// BatchDelete 批量删除记录
func (s *BaseService) BatchDelete(ctx context.Context, model interface{}, condition string, args ...interface{}) error {
	return s.DB.WithContext(ctx).Where(condition, args...).Delete(model).Error
}

// ============================================================================
// 工具方法
// ============================================================================

// GetDB 获取数据库实例
func (s *BaseService) GetDB() *gorm.DB {
	return s.DB
}

// GetDBWithContext 获取带上下文的数据库实例
func (s *BaseService) GetDBWithContext(ctx context.Context) *gorm.DB {
	return s.DB.WithContext(ctx)
}

// BuildQuery 构建基础查询（应用通用过滤条件）
// 这是一个辅助方法，可以链式调用多个过滤条件
func (s *BaseService) BuildQuery(ctx context.Context, model interface{}, tenantID string, req FilterRequest) *gorm.DB {
	query := s.GetDBWithContext(ctx).Model(model)

	// 应用租户过滤
	query = s.ApplyTenantFilter(query, tenantID)

	// 应用软删除过滤
	query = s.ApplySoftDelete(query)

	// 应用状态过滤
	query = s.ApplyStatusFilter(query, req.Status)

	// 应用日期范围过滤
	query = s.ApplyDateRangeFilter(query, "created_at", req.DateRange)

	// 应用排序
	query = s.ApplySorting(query, req.SortBy, req.SortOrder, nil)

	return query
}
