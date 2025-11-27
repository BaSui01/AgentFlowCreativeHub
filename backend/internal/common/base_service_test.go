package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestModel 测试用的模型
type TestModel struct {
	ID        uint      `gorm:"primaryKey"`
	TenantID  string    `gorm:"size:255;index"`
	Name      string    `gorm:"size:255"`
	Status    string    `gorm:"size:50"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `gorm:"index"`
	CreatedBy string     `gorm:"size:255"`
	UpdatedBy string     `gorm:"size:255"`
	DeletedBy string     `gorm:"size:255"`
}

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect database: %v", err)
	}

	// 自动迁移
	err = db.AutoMigrate(&TestModel{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

// seedTestData 插入测试数据
func seedTestData(t *testing.T, db *gorm.DB) {
	models := []TestModel{
		{TenantID: "tenant1", Name: "Test 1", Status: "active", CreatedAt: time.Now()},
		{TenantID: "tenant1", Name: "Test 2", Status: "inactive", CreatedAt: time.Now()},
		{TenantID: "tenant2", Name: "Test 3", Status: "active", CreatedAt: time.Now()},
		{TenantID: "tenant2", Name: "Test 4", Status: "pending", CreatedAt: time.Now()},
		{TenantID: "tenant1", Name: "Deleted Test", Status: "active", CreatedAt: time.Now(), DeletedAt: ptrTime(time.Now())},
	}

	for _, model := range models {
		if err := db.Create(&model).Error; err != nil {
			t.Fatalf("Failed to seed data: %v", err)
		}
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

// TestApplyTenantFilter 测试租户过滤
func TestApplyTenantFilter(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)

	tests := []struct {
		name        string
		tenantID    string
		expectCount int64
	}{
		{"Filter tenant1", "tenant1", 3}, // 包含已删除的
		{"Filter tenant2", "tenant2", 2},
		{"No filter", "", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := db.Model(&TestModel{}).Unscoped()
			query = service.ApplyTenantFilter(query, tt.tenantID)

			var count int64
			err := query.Count(&count).Error
			assert.NoError(t, err)
			assert.Equal(t, tt.expectCount, count)
		})
	}
}

// TestApplySoftDelete 测试软删除过滤
func TestApplySoftDelete(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)

	// 不应用软删除过滤
	var countAll int64
	db.Model(&TestModel{}).Unscoped().Count(&countAll)
	assert.Equal(t, int64(5), countAll)

	// 应用软删除过滤
	query := db.Model(&TestModel{})
	query = service.ApplySoftDelete(query)

	var countFiltered int64
	err := query.Count(&countFiltered).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(4), countFiltered) // 排除1个已删除的
}

// TestPagination 测试分页
func TestPagination(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)

	tests := []struct {
		name        string
		page        int
		pageSize    int
		expectCount int
	}{
		{"Page 1, size 2", 1, 2, 2},
		{"Page 2, size 2", 2, 2, 2},
		{"Page 3, size 2", 3, 2, 0}, // 超出范围
		{"Page 1, size 10", 1, 10, 4}, // 只有4条未删除记录
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := db.Model(&TestModel{}).Where("deleted_at IS NULL")
			query = service.ApplyPagination(query, tt.page, tt.pageSize)

			var models []TestModel
			err := query.Find(&models).Error
			assert.NoError(t, err)
			assert.Equal(t, tt.expectCount, len(models))
		})
	}
}

// TestApplySorting 测试排序
func TestApplySorting(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)

	tests := []struct {
		name          string
		sortBy        string
		sortOrder     string
		allowedFields []string
		expectFirst   string
	}{
		{"Sort by name ASC", "name", "asc", []string{"name", "status"}, "Deleted Test"},
		{"Sort by name DESC", "name", "desc", []string{"name", "status"}, "Test 4"},
		{"Sort by status ASC", "status", "asc", []string{"name", "status"}, "Test 1"},
		{"Default sort", "", "", nil, ""}, // 默认按created_at DESC排序
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := db.Model(&TestModel{}).Unscoped()
			query = service.ApplySorting(query, tt.sortBy, tt.sortOrder, tt.allowedFields)

			var models []TestModel
			err := query.Find(&models).Error
			assert.NoError(t, err)

			if tt.expectFirst != "" && len(models) > 0 {
				assert.Equal(t, tt.expectFirst, models[0].Name)
			}
		})
	}
}

// TestApplyFilters 测试综合过滤
func TestApplyFilters(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)

	t.Run("Combined filters", func(t *testing.T) {
		// 组合使用租户过滤、软删除过滤、分页、排序
		query := db.Model(&TestModel{})
		query = service.ApplyTenantFilter(query, "tenant1")
		query = service.ApplySoftDelete(query)
		query = service.ApplyPagination(query, 1, 10)
		query = service.ApplySorting(query, "name", "asc", []string{"name", "status"})

		var models []TestModel
		err := query.Find(&models).Error
		assert.NoError(t, err)
		assert.Equal(t, 2, len(models)) // tenant1 未删除的2条
		assert.Equal(t, "Test 1", models[0].Name) // 按name ASC排序
	})
}

// TestCreate 测试创建记录
func TestCreate(t *testing.T) {
	db := setupTestDB(t)
	service := NewBaseService(db)
	ctx := context.Background()

	model := &TestModel{
		TenantID: "tenant1",
		Name:     "New Test",
		Status:   "active",
	}

	err := service.Create(ctx, model)
	assert.NoError(t, err)
	assert.NotZero(t, model.ID)
	assert.NotZero(t, model.CreatedAt)
}

// TestUpdate 测试更新记录
func TestUpdate(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)
	ctx := context.Background()

	// 获取第一条记录
	var model TestModel
	db.First(&model)

	// 更新
	model.Name = "Updated Name"
	err := service.Update(ctx, &model)
	assert.NoError(t, err)

	// 验证更新
	var updated TestModel
	db.First(&updated, model.ID)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.NotZero(t, updated.UpdatedAt)
}

// TestSoftDelete 测试软删除
func TestSoftDelete(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)
	ctx := context.Background()

	// 获取第一条未删除记录
	var model TestModel
	db.Where("deleted_at IS NULL").First(&model)

	// 软删除
	err := service.SoftDelete(ctx, &model, "admin")
	assert.NoError(t, err)

	// 验证软删除
	var deleted TestModel
	err = db.Unscoped().First(&deleted, model.ID).Error
	assert.NoError(t, err)
	assert.NotNil(t, deleted.DeletedAt)
	assert.Equal(t, "admin", deleted.DeletedBy)
}

// TestDelete 测试硬删除
func TestDelete(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)
	ctx := context.Background()

	// 获取第一条记录
	var model TestModel
	db.First(&model)
	id := model.ID

	// 硬删除
	err := service.Delete(ctx, &model)
	assert.NoError(t, err)

	// 验证已删除
	var deleted TestModel
	err = db.Unscoped().First(&deleted, id).Error
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

// TestFindByID 测试根据ID查询
func TestFindByID(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)
	ctx := context.Background()

	// 获取第一条记录的ID
	var firstModel TestModel
	db.First(&firstModel)

	// 根据ID查询
	var model TestModel
	err := service.FindByID(ctx, &model, string(rune(firstModel.ID)))
	assert.NoError(t, err)
	assert.Equal(t, firstModel.Name, model.Name)
}

// TestExists 测试记录存在性检查
func TestExists(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)
	ctx := context.Background()

	tests := []struct {
		name      string
		condition string
		args      []interface{}
		expect    bool
	}{
		{"Exists - tenant1", "tenant_id = ?", []interface{}{"tenant1"}, true},
		{"Exists - active status", "status = ?", []interface{}{"active"}, true},
		{"Not exists - unknown tenant", "tenant_id = ?", []interface{}{"tenant999"}, false},
		{"Not exists - unknown status", "status = ?", []interface{}{"archived"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := service.Exists(ctx, &TestModel{}, tt.condition, tt.args...)
			assert.NoError(t, err)
			assert.Equal(t, tt.expect, exists)
		})
	}
}

// TestTransaction 测试事务
func TestTransaction(t *testing.T) {
	db := setupTestDB(t)
	service := NewBaseService(db)
	ctx := context.Background()

	t.Run("Successful transaction", func(t *testing.T) {
		err := service.Transaction(ctx, func(tx *gorm.DB) error {
			model1 := &TestModel{TenantID: "tenant1", Name: "TX Test 1", Status: "active"}
			model2 := &TestModel{TenantID: "tenant1", Name: "TX Test 2", Status: "active"}

			if err := tx.Create(model1).Error; err != nil {
				return err
			}
			if err := tx.Create(model2).Error; err != nil {
				return err
			}

			return nil
		})

		assert.NoError(t, err)

		// 验证记录已创建
		var count int64
		db.Model(&TestModel{}).Where("name LIKE ?", "TX Test%").Count(&count)
		assert.Equal(t, int64(2), count)
	})

	t.Run("Failed transaction (rollback)", func(t *testing.T) {
		var countBefore int64
		db.Model(&TestModel{}).Count(&countBefore)

		err := service.Transaction(ctx, func(tx *gorm.DB) error {
			model := &TestModel{TenantID: "tenant1", Name: "Rollback Test", Status: "active"}
			if err := tx.Create(model).Error; err != nil {
				return err
			}

			// 模拟错误，触发回滚
			return gorm.ErrInvalidTransaction
		})

		assert.Error(t, err)

		// 验证记录未创建（已回滚）
		var countAfter int64
		db.Model(&TestModel{}).Count(&countAfter)
		assert.Equal(t, countBefore, countAfter)
	})
}

// TestCount 测试计数
func TestCount(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	service := NewBaseService(db)
	ctx := context.Background()

	tests := []struct {
		name      string
		condition string
		args      []interface{}
		expect    int64
	}{
		{"Count all (exclude deleted)", "", nil, 4},
		{"Count tenant1", "tenant_id = ?", []interface{}{"tenant1"}, 2},
		{"Count active status", "status = ?", []interface{}{"active"}, 2},
		{"Count tenant2 + pending", "tenant_id = ? AND status = ?", []interface{}{"tenant2", "pending"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := service.Count(ctx, &TestModel{}, tt.condition, tt.args...)
			assert.NoError(t, err)
			assert.Equal(t, tt.expect, count)
		})
	}
}

// TestBatchCreate 测试批量创建
func TestBatchCreate(t *testing.T) {
	db := setupTestDB(t)
	service := NewBaseService(db)
	ctx := context.Background()

	models := []TestModel{
		{TenantID: "tenant1", Name: "Batch 1", Status: "active"},
		{TenantID: "tenant1", Name: "Batch 2", Status: "active"},
		{TenantID: "tenant1", Name: "Batch 3", Status: "active"},
	}

	err := service.BatchCreate(ctx, &models, 100)
	assert.NoError(t, err)

	// 验证记录已创建
	var count int64
	db.Model(&TestModel{}).Where("name LIKE ?", "Batch%").Count(&count)
	assert.Equal(t, int64(3), count)
}
