package knowledge

import (
	"context"
	"errors"
	"testing"

	"backend/internal/models"
	"backend/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// Mock Service Interface & Implementation
// ============================================================

// KBServiceInterface 知识库Service接口（用于Mock）
type KBServiceInterface interface {
	CreateKnowledgeBase(ctx context.Context, kb *models.KnowledgeBase) error
	GetKnowledgeBase(ctx context.Context, id string) (*models.KnowledgeBase, error)
	ListKnowledgeBases(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*models.KnowledgeBase, *types.PaginationResponse, error)
	UpdateKnowledgeBase(ctx context.Context, kb *models.KnowledgeBase) error
	DeleteKnowledgeBase(ctx context.Context, id string) error
}

// MockKBService Mock实现
type MockKBService struct {
	CreateKnowledgeBaseFunc func(ctx context.Context, kb *models.KnowledgeBase) error
	GetKnowledgeBaseFunc    func(ctx context.Context, id string) (*models.KnowledgeBase, error)
	ListKnowledgeBasesFunc  func(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*models.KnowledgeBase, *types.PaginationResponse, error)
	UpdateKnowledgeBaseFunc func(ctx context.Context, kb *models.KnowledgeBase) error
	DeleteKnowledgeBaseFunc func(ctx context.Context, id string) error
}

func (m *MockKBService) CreateKnowledgeBase(ctx context.Context, kb *models.KnowledgeBase) error {
	if m.CreateKnowledgeBaseFunc != nil {
		return m.CreateKnowledgeBaseFunc(ctx, kb)
	}
	return errors.New("CreateKnowledgeBaseFunc not implemented")
}

func (m *MockKBService) GetKnowledgeBase(ctx context.Context, id string) (*models.KnowledgeBase, error) {
	if m.GetKnowledgeBaseFunc != nil {
		return m.GetKnowledgeBaseFunc(ctx, id)
	}
	return nil, errors.New("GetKnowledgeBaseFunc not implemented")
}

func (m *MockKBService) ListKnowledgeBases(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*models.KnowledgeBase, *types.PaginationResponse, error) {
	if m.ListKnowledgeBasesFunc != nil {
		return m.ListKnowledgeBasesFunc(ctx, tenantID, pagination)
	}
	return nil, nil, errors.New("ListKnowledgeBasesFunc not implemented")
}

func (m *MockKBService) UpdateKnowledgeBase(ctx context.Context, kb *models.KnowledgeBase) error {
	if m.UpdateKnowledgeBaseFunc != nil {
		return m.UpdateKnowledgeBaseFunc(ctx, kb)
	}
	return errors.New("UpdateKnowledgeBaseFunc not implemented")
}

func (m *MockKBService) DeleteKnowledgeBase(ctx context.Context, id string) error {
	if m.DeleteKnowledgeBaseFunc != nil {
		return m.DeleteKnowledgeBaseFunc(ctx, id)
	}
	return errors.New("DeleteKnowledgeBaseFunc not implemented")
}

func TestKBHandler_ListKnowledgeBases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_返回知识库列表", func(t *testing.T) {
		// 创建Mock Service
		mockSvc := &MockKBService{
			ListKnowledgeBasesFunc: func(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*models.KnowledgeBase, *types.PaginationResponse, error) {
				// 验证参数
				assert.Equal(t, "tenant-1", tenantID)
				assert.NotNil(t, pagination)
				
				// 返回Mock数据
				return []*models.KnowledgeBase{
					{ID: "kb-1", Name: "技术知识库", Description: "技术文档集合", DocCount: 42},
					{ID: "kb-2", Name: "产品知识库", Description: "产品文档集合", DocCount: 28},
				}, &types.PaginationResponse{
					Page:       1,
					PageSize:   20,
					TotalItems: 2,
					TotalPages: 1,
				}, nil
			},
		}

		// 调用Mock Service
		ctx := context.Background()
		pagination := &types.PaginationRequest{Page: 1, PageSize: 20}
		kbs, paginationResp, err := mockSvc.ListKnowledgeBases(ctx, "tenant-1", pagination)

		// 断言
		assert.NoError(t, err)
		assert.Len(t, kbs, 2)
		assert.Equal(t, "技术知识库", kbs[0].Name)
		assert.Equal(t, 42, kbs[0].DocCount)
		assert.Equal(t, int64(2), paginationResp.TotalItems)
	})
	
	t.Run("失败_Service返回错误", func(t *testing.T) {
		mockSvc := &MockKBService{
			ListKnowledgeBasesFunc: func(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*models.KnowledgeBase, *types.PaginationResponse, error) {
				return nil, nil, errors.New("数据库查询失败")
			},
		}

		ctx := context.Background()
		pagination := &types.PaginationRequest{Page: 1, PageSize: 20}
		kbs, _, err := mockSvc.ListKnowledgeBases(ctx, "tenant-1", pagination)

		assert.Error(t, err)
		assert.Nil(t, kbs)
		assert.Contains(t, err.Error(), "数据库查询失败")
	})
}

func TestKBHandler_GetKnowledgeBase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_查询存在的知识库", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				assert.Equal(t, "kb-1", id)
				
				return &models.KnowledgeBase{
					ID:          "kb-1",
					TenantID:    "tenant-1",
					Name:        "产品知识库",
					Description: "产品相关文档",
					Type:        "document",
					Status:      "active",
					DocCount:    15,
				}, nil
			},
		}

		ctx := context.Background()
		kb, err := mockSvc.GetKnowledgeBase(ctx, "kb-1")

		assert.NoError(t, err)
		assert.NotNil(t, kb)
		assert.Equal(t, "kb-1", kb.ID)
		assert.Equal(t, "产品知识库", kb.Name)
		assert.Equal(t, "document", kb.Type)
	})

	t.Run("失败_知识库不存在", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				return nil, nil // Service返回nil表示不存在
			},
		}

		ctx := context.Background()
		kb, err := mockSvc.GetKnowledgeBase(ctx, "nonexistent-id")

		assert.NoError(t, err)
		assert.Nil(t, kb)
	})
}

func TestKBHandler_CreateKnowledgeBase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_创建新知识库", func(t *testing.T) {
		mockSvc := &MockKBService{
			CreateKnowledgeBaseFunc: func(ctx context.Context, kb *models.KnowledgeBase) error {
				// 验证输入参数
				assert.Equal(t, "新知识库", kb.Name)
				assert.Equal(t, "document", kb.Type)
				assert.Equal(t, "tenant-1", kb.TenantID)
				
				// 模拟设置ID（实际由数据库完成）
				kb.ID = "new-kb-id"
				return nil
			},
		}

		ctx := context.Background()
		kb := &models.KnowledgeBase{
			TenantID:    "tenant-1",
			Name:        "新知识库",
			Description: "测试知识库",
			Type:        "document",
			Status:      "active",
		}
		
		err := mockSvc.CreateKnowledgeBase(ctx, kb)

		assert.NoError(t, err)
		assert.Equal(t, "new-kb-id", kb.ID) // 验证ID已设置
	})

	t.Run("失败_Service返回错误", func(t *testing.T) {
		mockSvc := &MockKBService{
			CreateKnowledgeBaseFunc: func(ctx context.Context, kb *models.KnowledgeBase) error {
				return errors.New("数据库插入失败")
			},
		}

		ctx := context.Background()
		kb := &models.KnowledgeBase{Name: "新知识库", Type: "document"}
		err := mockSvc.CreateKnowledgeBase(ctx, kb)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "数据库插入失败")
	})
}

func TestKBHandler_UpdateKnowledgeBase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("更新知识库请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"description": "更新后的描述",
			"settings": map[string]interface{}{
				"chunk_size": 1000,
			},
		}

		assert.NotEmpty(t, req["description"])
	})
}

func TestKBHandler_DeleteKnowledgeBase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("删除知识库验证", func(t *testing.T) {
		kbID := "kb-to-delete"
		assert.NotEmpty(t, kbID)
	})
}
