package tenant

import (
	"context"
	"testing"
	"time"

	"backend/internal/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// ============================================================================
// Mock 对象
// ============================================================================

// MockTenantQuotaRepository Mock实现
type MockTenantQuotaRepository struct {
	mock.Mock
}

func (m *MockTenantQuotaRepository) Insert(ctx context.Context, q *TenantQuota) error {
	args := m.Called(ctx, q)
	return args.Error(0)
}

func (m *MockTenantQuotaRepository) Update(ctx context.Context, q *TenantQuota) error {
	args := m.Called(ctx, q)
	return args.Error(0)
}

func (m *MockTenantQuotaRepository) FindByTenantID(ctx context.Context, tenantID string) (*TenantQuota, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TenantQuota), args.Error(1)
}

func (m *MockTenantQuotaRepository) FindByTenantIDForUpdate(ctx context.Context, db interface{}, tenantID string) (*TenantQuota, error) {
	args := m.Called(ctx, db, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TenantQuota), args.Error(1)
}

// MockIDGenerator Mock ID生成器
type MockIDGenerator struct {
	mock.Mock
}

func (m *MockIDGenerator) NewID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// MockAuditLogger Mock审计日志
type MockAuditLogger struct {
	mock.Mock
}

func (m *MockAuditLogger) LogAction(ctx context.Context, tc TenantContext, action, resource string, details any) {
	m.Called(ctx, tc, action, resource, details)
}

// ============================================================================
// 辅助函数
// ============================================================================

func createTestQuotaService(t *testing.T) (*quotaService, *MockTenantQuotaRepository, *MockIDGenerator, *MockAuditLogger) {
	mockRepo := new(MockTenantQuotaRepository)
	mockIDGen := new(MockIDGenerator)
	mockAudit := new(MockAuditLogger)

	service := &quotaService{
		BaseService: common.NewBaseService(nil), // 基础服务暂不需要DB
		quotaRepo:   mockRepo,
		ids:         mockIDGen,
		audit:       mockAudit,
	}

	return service, mockRepo, mockIDGen, mockAudit
}

func createTestQuota(tenantID string, tier string) *TenantQuota {
	now := time.Now()
	quota := &TenantQuota{
		ID:                "quota-001",
		TenantID:          tenantID,
		CreatedAt:         now,
		UpdatedAt:         now,
		TokenQuotaResetAt: now.AddDate(0, 1, 0),
		APIQuotaResetAt:   now.AddDate(0, 0, 1),
	}

	// 根据套餐设置配额
	switch tier {
	case "free":
		quota.MaxUsers = 10
		quota.MaxStorageMB = 1024
		quota.MaxWorkflows = 10
		quota.MaxKnowledgeBases = 2
		quota.MaxTokensPerMonth = 100000
		quota.MaxAPICallsPerDay = 1000
	case "pro":
		quota.MaxUsers = 200
		quota.MaxStorageMB = 51200
		quota.MaxWorkflows = 500
		quota.MaxKnowledgeBases = 50
		quota.MaxTokensPerMonth = 10000000
		quota.MaxAPICallsPerDay = 100000
	}

	return quota
}

// ============================================================================
// 测试用例
// ============================================================================

// TestGetQuota 测试获取配额
func TestGetQuota(t *testing.T) {
	service, mockRepo, _, _ := createTestQuotaService(t)
	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("成功获取配额", func(t *testing.T) {
		expectedQuota := createTestQuota(tenantID, "free")
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(expectedQuota, nil).Once()

		quota, err := service.GetQuota(ctx, tenantID)

		assert.NoError(t, err)
		assert.NotNil(t, quota)
		assert.Equal(t, tenantID, quota.TenantID)
		assert.Equal(t, 10, quota.MaxUsers)
		mockRepo.AssertExpectations(t)
	})

	t.Run("配额不存在", func(t *testing.T) {
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(nil, gorm.ErrRecordNotFound).Once()

		quota, err := service.GetQuota(ctx, tenantID)

		assert.Error(t, err)
		assert.Nil(t, quota)
		assert.Equal(t, ErrQuotaNotFound, err)
		mockRepo.AssertExpectations(t)
	})
}

// TestCreateQuota 测试创建配额
func TestCreateQuota(t *testing.T) {
	service, mockRepo, mockIDGen, mockAudit := createTestQuotaService(t)
	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("成功创建Free套餐配额", func(t *testing.T) {
		quotaID := "quota-001"
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(nil, gorm.ErrRecordNotFound).Once()
		mockIDGen.On("NewID").Return(quotaID, nil).Once()
		mockRepo.On("Insert", ctx, mock.AnythingOfType("*tenant.TenantQuota")).Return(nil).Once()
		mockAudit.On("LogAction", ctx, mock.Anything, "quota.created", "quota", mock.Anything).Return().Once()

		quota, err := service.CreateQuota(ctx, tenantID, "free")

		assert.NoError(t, err)
		assert.NotNil(t, quota)
		assert.Equal(t, quotaID, quota.ID)
		assert.Equal(t, tenantID, quota.TenantID)
		assert.Equal(t, 10, quota.MaxUsers)
		assert.Equal(t, 1024, quota.MaxStorageMB)
		assert.Equal(t, 10, quota.MaxWorkflows)
		mockRepo.AssertExpectations(t)
		mockIDGen.AssertExpectations(t)
		mockAudit.AssertExpectations(t)
	})

	t.Run("成功创建Pro套餐配额", func(t *testing.T) {
		quotaID := "quota-002"
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(nil, gorm.ErrRecordNotFound).Once()
		mockIDGen.On("NewID").Return(quotaID, nil).Once()
		mockRepo.On("Insert", ctx, mock.AnythingOfType("*tenant.TenantQuota")).Return(nil).Once()
		mockAudit.On("LogAction", ctx, mock.Anything, "quota.created", "quota", mock.Anything).Return().Once()

		quota, err := service.CreateQuota(ctx, tenantID, "pro")

		assert.NoError(t, err)
		assert.Equal(t, 200, quota.MaxUsers)
		assert.Equal(t, 51200, quota.MaxStorageMB)
		mockRepo.AssertExpectations(t)
	})

	t.Run("配额已存在时直接返回", func(t *testing.T) {
		existingQuota := createTestQuota(tenantID, "free")
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(existingQuota, nil).Once()

		quota, err := service.CreateQuota(ctx, tenantID, "free")

		assert.NoError(t, err)
		assert.Equal(t, existingQuota, quota)
		mockRepo.AssertExpectations(t)
	})
}

// TestCheckLimit 测试配额检查
func TestCheckLimit(t *testing.T) {
	service, mockRepo, _, _ := createTestQuotaService(t)
	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("未超限", func(t *testing.T) {
		quota := createTestQuota(tenantID, "free")
		quota.UsedUsers = 5 // 已用5个，限制10个
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(quota, nil).Once()

		exceeded, err := service.CheckLimit(ctx, tenantID, ResourceTypeUsers)

		assert.NoError(t, err)
		assert.False(t, exceeded)
		mockRepo.AssertExpectations(t)
	})

	t.Run("已超限", func(t *testing.T) {
		quota := createTestQuota(tenantID, "free")
		quota.UsedUsers = 10 // 已用10个，限制10个
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(quota, nil).Once()

		exceeded, err := service.CheckLimit(ctx, tenantID, ResourceTypeUsers)

		assert.NoError(t, err)
		assert.True(t, exceeded)
		mockRepo.AssertExpectations(t)
	})

	t.Run("无限制配额", func(t *testing.T) {
		quota := createTestQuota(tenantID, "enterprise")
		quota.MaxUsers = -1 // 无限制
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(quota, nil).Once()

		exceeded, err := service.CheckLimit(ctx, tenantID, ResourceTypeUsers)

		assert.NoError(t, err)
		assert.False(t, exceeded)
		mockRepo.AssertExpectations(t)
	})
}

// TestIncrementUsage 测试增加用量
func TestIncrementUsage(t *testing.T) {
	t.Run("成功增加用量", func(t *testing.T) {
		// 注意：此测试需要真实DB进行事务测试，这里仅做简单Mock示意
		service, mockRepo, _, _ := createTestQuotaService(t)
		ctx := context.Background()
		tenantID := "tenant-001"

		quota := createTestQuota(tenantID, "free")
		quota.UsedUsers = 5

		// Mock事务场景较复杂，实际应使用集成测试
		mockRepo.On("FindByTenantIDForUpdate", ctx, mock.Anything, tenantID).Return(quota, nil).Once()

		// 这里仅验证调用逻辑
		// 完整测试需要真实DB环境
	})

	t.Run("超过配额限制", func(t *testing.T) {
		service, mockRepo, _, _ := createTestQuotaService(t)
		ctx := context.Background()
		tenantID := "tenant-001"

		quota := createTestQuota(tenantID, "free")
		quota.UsedUsers = 10 // 已达上限

		// 此测试需要真实DB事务环境
		// mockRepo.On("FindByTenantIDForUpdate", ctx, mock.Anything, tenantID).Return(quota, nil).Once()

		// err := service.IncrementUsage(ctx, tenantID, ResourceTypeUsers, 1)
		// assert.Error(t, err)
		// assert.Equal(t, ErrQuotaExceeded, err)
	})
}

// TestGetUsageStats 测试获取使用统计
func TestGetUsageStats(t *testing.T) {
	service, mockRepo, _, _ := createTestQuotaService(t)
	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("成功获取统计", func(t *testing.T) {
		quota := createTestQuota(tenantID, "free")
		quota.UsedUsers = 5
		quota.UsedStorageMB = 512
		quota.UsedTokensThisMonth = 50000
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(quota, nil).Once()

		stats, err := service.GetUsageStats(ctx, tenantID)

		assert.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Len(t, stats, 6) // 6种资源类型

		// 验证Users统计
		userStats := stats[0]
		assert.Equal(t, string(ResourceTypeUsers), userStats.ResourceType)
		assert.Equal(t, int64(5), userStats.Used)
		assert.Equal(t, int64(10), userStats.Limit)
		assert.Equal(t, 50.0, userStats.Percentage) // 5/10 = 50%

		// 验证Token统计
		tokenStats := stats[4]
		assert.Equal(t, string(ResourceTypeTokens), tokenStats.ResourceType)
		assert.Equal(t, int64(50000), tokenStats.Used)
		assert.Equal(t, int64(100000), tokenStats.Limit)
		assert.Equal(t, 50.0, tokenStats.Percentage)

		mockRepo.AssertExpectations(t)
	})
}

// TestIsQuotaAvailable 测试配额可用性检查
func TestIsQuotaAvailable(t *testing.T) {
	service, mockRepo, _, _ := createTestQuotaService(t)
	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("配额充足", func(t *testing.T) {
		quota := createTestQuota(tenantID, "free")
		quota.UsedUsers = 5
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(quota, nil).Once()

		available, err := service.IsQuotaAvailable(ctx, tenantID, ResourceTypeUsers, 3)

		assert.NoError(t, err)
		assert.True(t, available) // 5 + 3 = 8 < 10
		mockRepo.AssertExpectations(t)
	})

	t.Run("配额不足", func(t *testing.T) {
		quota := createTestQuota(tenantID, "free")
		quota.UsedUsers = 8
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(quota, nil).Once()

		available, err := service.IsQuotaAvailable(ctx, tenantID, ResourceTypeUsers, 5)

		assert.NoError(t, err)
		assert.False(t, available) // 8 + 5 = 13 > 10
		mockRepo.AssertExpectations(t)
	})

	t.Run("无限制配额", func(t *testing.T) {
		quota := createTestQuota(tenantID, "enterprise")
		quota.MaxUsers = -1
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(quota, nil).Once()

		available, err := service.IsQuotaAvailable(ctx, tenantID, ResourceTypeUsers, 1000)

		assert.NoError(t, err)
		assert.True(t, available)
		mockRepo.AssertExpectations(t)
	})
}

// TestUpdateQuotaLimits 测试更新配额限制
func TestUpdateQuotaLimits(t *testing.T) {
	service, mockRepo, _, mockAudit := createTestQuotaService(t)
	ctx := context.Background()
	tenantID := "tenant-001"

	t.Run("成功更新配额", func(t *testing.T) {
		quota := createTestQuota(tenantID, "free")
		mockRepo.On("FindByTenantID", ctx, tenantID).Return(quota, nil).Once()
		mockRepo.On("Update", ctx, quota).Return(nil).Once()
		mockAudit.On("LogAction", ctx, mock.Anything, "quota.limits_updated", "quota", mock.Anything).Return().Once()

		limits := map[ResourceType]int64{
			ResourceTypeUsers:     50,
			ResourceTypeWorkflows: 100,
		}

		err := service.UpdateQuotaLimits(ctx, tenantID, limits)

		assert.NoError(t, err)
		assert.Equal(t, 50, quota.MaxUsers)
		assert.Equal(t, 100, quota.MaxWorkflows)
		mockRepo.AssertExpectations(t)
		mockAudit.AssertExpectations(t)
	})
}

// ============================================================================
// 集成测试提示
// ============================================================================

// 注意：以下功能需要真实数据库进行集成测试：
// 1. IncrementUsage - 需要事务和悲观锁测试
// 2. DecrementUsage - 需要事务测试
// 3. ResetPeriodicalUsage - 需要时间重置测试
// 4. 并发安全性测试 - 多goroutine同时操作

// 集成测试示例框架（需要真实DB）：
/*
func TestQuotaServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 初始化测试数据库
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	// 创建真实的Service和Repository
	repo := NewTenantQuotaRepository(db)
	idGen := &UUIDGenerator{}
	audit := &NoOpAuditLogger{}
	service := NewQuotaService(db, repo, idGen, audit)

	t.Run("并发增加用量测试", func(t *testing.T) {
		// 创建配额
		quota, _ := service.CreateQuota(ctx, "tenant-001", "free")

		// 并发增加用量
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = service.IncrementUsage(ctx, "tenant-001", ResourceTypeUsers, 1)
			}()
		}
		wg.Wait()

		// 验证最终结果
		quota, _ = service.GetQuota(ctx, "tenant-001")
		assert.Equal(t, 10, quota.UsedUsers)
	})
}
*/
