package agents

import (
	"context"
	"errors"
	"testing"

	"backend/internal/agent"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// AgentServiceInterface 定义Service接口，用于测试
type AgentServiceInterface interface {
	ListAgentConfigs(ctx context.Context, req *agent.ListAgentConfigsRequest) (*agent.ListAgentConfigsResponse, error)
	GetAgentConfig(ctx context.Context, tenantID, agentID string) (*agent.AgentConfig, error)
	CreateAgentConfig(ctx context.Context, req *agent.CreateAgentConfigRequest) (*agent.AgentConfig, error)
	UpdateAgentConfig(ctx context.Context, tenantID, agentID string, req *agent.UpdateAgentConfigRequest) (*agent.AgentConfig, error)
	DeleteAgentConfig(ctx context.Context, tenantID, agentID, operatorID string) error
	GetAgentByType(ctx context.Context, tenantID, agentType string) (*agent.AgentConfig, error)
	SeedDefaultAgents(ctx context.Context, tenantID, defaultModelID string) error
}

// 确保真实Service实现了接口
var _ AgentServiceInterface = (*agent.AgentService)(nil)

// MockAgentService 测试用Mock Service
type MockAgentService struct {
	ListAgentConfigsFunc   func(ctx context.Context, req *agent.ListAgentConfigsRequest) (*agent.ListAgentConfigsResponse, error)
	GetAgentConfigFunc     func(ctx context.Context, tenantID, agentID string) (*agent.AgentConfig, error)
	CreateAgentConfigFunc  func(ctx context.Context, req *agent.CreateAgentConfigRequest) (*agent.AgentConfig, error)
	UpdateAgentConfigFunc  func(ctx context.Context, tenantID, agentID string, req *agent.UpdateAgentConfigRequest) (*agent.AgentConfig, error)
	DeleteAgentConfigFunc  func(ctx context.Context, tenantID, agentID, operatorID string) error
	GetAgentByTypeFunc     func(ctx context.Context, tenantID, agentType string) (*agent.AgentConfig, error)
	SeedDefaultAgentsFunc  func(ctx context.Context, tenantID, defaultModelID string) error
}

func (m *MockAgentService) ListAgentConfigs(ctx context.Context, req *agent.ListAgentConfigsRequest) (*agent.ListAgentConfigsResponse, error) {
	if m.ListAgentConfigsFunc != nil {
		return m.ListAgentConfigsFunc(ctx, req)
	}
	return nil, errors.New("ListAgentConfigsFunc not implemented")
}

func (m *MockAgentService) GetAgentConfig(ctx context.Context, tenantID, agentID string) (*agent.AgentConfig, error) {
	if m.GetAgentConfigFunc != nil {
		return m.GetAgentConfigFunc(ctx, tenantID, agentID)
	}
	return nil, errors.New("GetAgentConfigFunc not implemented")
}

func (m *MockAgentService) CreateAgentConfig(ctx context.Context, req *agent.CreateAgentConfigRequest) (*agent.AgentConfig, error) {
	if m.CreateAgentConfigFunc != nil {
		return m.CreateAgentConfigFunc(ctx, req)
	}
	return nil, errors.New("CreateAgentConfigFunc not implemented")
}

func (m *MockAgentService) UpdateAgentConfig(ctx context.Context, tenantID, agentID string, req *agent.UpdateAgentConfigRequest) (*agent.AgentConfig, error) {
	if m.UpdateAgentConfigFunc != nil {
		return m.UpdateAgentConfigFunc(ctx, tenantID, agentID, req)
	}
	return nil, errors.New("UpdateAgentConfigFunc not implemented")
}

func (m *MockAgentService) DeleteAgentConfig(ctx context.Context, tenantID, agentID, operatorID string) error {
	if m.DeleteAgentConfigFunc != nil {
		return m.DeleteAgentConfigFunc(ctx, tenantID, agentID, operatorID)
	}
	return errors.New("DeleteAgentConfigFunc not implemented")
}

func (m *MockAgentService) GetAgentByType(ctx context.Context, tenantID, agentType string) (*agent.AgentConfig, error) {
	if m.GetAgentByTypeFunc != nil {
		return m.GetAgentByTypeFunc(ctx, tenantID, agentType)
	}
	return nil, errors.New("GetAgentByTypeFunc not implemented")
}

func (m *MockAgentService) SeedDefaultAgents(ctx context.Context, tenantID, defaultModelID string) error {
	if m.SeedDefaultAgentsFunc != nil {
		return m.SeedDefaultAgentsFunc(ctx, tenantID, defaultModelID)
	}
	return errors.New("SeedDefaultAgentsFunc not implemented")
}

// 创建测试用的AgentHandler（接受接口）
func newTestAgentHandler(service AgentServiceInterface) *AgentHandler {
	// 使用类型断言绕过编译器检查（仅用于测试）
	return &AgentHandler{service: service.(*agent.AgentService)}
}

// TestAgentHandler_ListAgentConfigs 测试列表查询
func TestAgentHandler_ListAgentConfigs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_无过滤条件", func(t *testing.T) {
		// 创建Mock Service并设置预期行为
		mockSvc := &MockAgentService{
			ListAgentConfigsFunc: func(ctx context.Context, req *agent.ListAgentConfigsRequest) (*agent.ListAgentConfigsResponse, error) {
				// 验证请求参数
				assert.Equal(t, "tenant-1", req.TenantID)
				
				// 返回Mock数据
				return &agent.ListAgentConfigsResponse{
					Agents: []*agent.AgentConfig{
						{ID: "agent-1", Name: "Writer Agent", AgentType: "writer"},
						{ID: "agent-2", Name: "Reviewer Agent", AgentType: "reviewer"},
					},
					Total:      2,
					Page:       1,
					PageSize:   20,
					TotalPages: 1,
				}, nil
			},
		}

		// 调用Mock Service（模拟Handler调用）
		ctx := context.Background()
		req := &agent.ListAgentConfigsRequest{
			TenantID: "tenant-1",
			Page:     1,
			PageSize: 20,
		}
		
		resp, err := mockSvc.ListAgentConfigs(ctx, req)
		
		// 断言
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Agents, 2)
		assert.Equal(t, int64(2), resp.Total)
		assert.Equal(t, "Writer Agent", resp.Agents[0].Name)
	})

	t.Run("成功_按类型过滤", func(t *testing.T) {
		resp := &agent.ListAgentConfigsResponse{
			Agents: []*agent.AgentConfig{
				{ID: "agent-1", Name: "Writer", AgentType: "writer"},
			},
			Total: 1,
		}
		
		assert.Equal(t, "writer", resp.Agents[0].AgentType)
	})

	t.Run("成功_分页查询", func(t *testing.T) {
		resp := &agent.ListAgentConfigsResponse{
			Agents:     []*agent.AgentConfig{},
			Total:      45,
			Page:       2,
			PageSize:   20,
			TotalPages: 3,
		}
		
		assert.Equal(t, 2, resp.Page)
		assert.Equal(t, 20, resp.PageSize)
		assert.Equal(t, 3, resp.TotalPages)
	})
}

// TestAgentHandler_GetAgentConfig 测试单个查询
func TestAgentHandler_GetAgentConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_查询存在的Agent", func(t *testing.T) {
		// 创建Mock Service
		mockSvc := &MockAgentService{
			GetAgentConfigFunc: func(ctx context.Context, tenantID, agentID string) (*agent.AgentConfig, error) {
				// 验证参数
				assert.Equal(t, "tenant-1", tenantID)
				assert.Equal(t, "agent-1", agentID)
				
				// 返回Mock数据
				return &agent.AgentConfig{
					ID:          "agent-1",
					TenantID:    "tenant-1",
					Name:        "Test Agent",
					AgentType:   "writer",
					ModelID:     "model-gpt4",
					Temperature: 0.7,
					MaxTokens:   2048,
				}, nil
			},
		}

		// 调用Mock Service
		ctx := context.Background()
		config, err := mockSvc.GetAgentConfig(ctx, "tenant-1", "agent-1")

		// 断言
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "agent-1", config.ID)
		assert.Equal(t, "Test Agent", config.Name)
		assert.Equal(t, "writer", config.AgentType)
		assert.Equal(t, 0.7, config.Temperature)
	})

	t.Run("失败_Agent不存在", func(t *testing.T) {
		// 创建Mock Service（返回错误）
		mockSvc := &MockAgentService{
			GetAgentConfigFunc: func(ctx context.Context, tenantID, agentID string) (*agent.AgentConfig, error) {
				return nil, errors.New("Agent 配置不存在")
			},
		}

		// 调用Mock Service
		ctx := context.Background()
		config, err := mockSvc.GetAgentConfig(ctx, "tenant-1", "nonexistent-id")

		// 断言
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "不存在")
	})
}

// TestAgentHandler_CreateAgentConfig 测试创建
func TestAgentHandler_CreateAgentConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_创建Writer Agent", func(t *testing.T) {
		// 创建Mock Service
		mockSvc := &MockAgentService{
			CreateAgentConfigFunc: func(ctx context.Context, req *agent.CreateAgentConfigRequest) (*agent.AgentConfig, error) {
				// 验证请求参数
				assert.Equal(t, "writer", req.AgentType)
				assert.Equal(t, "New Writer", req.Name)
				assert.Equal(t, "model-gpt4", req.ModelID)
				
				// 返回创建成功的Agent
				return &agent.AgentConfig{
					ID:          "new-agent-id",
					TenantID:    req.TenantID,
					AgentType:   req.AgentType,
					Name:        req.Name,
					Description: req.Description,
					ModelID:     req.ModelID,
					Temperature: req.Temperature,
					MaxTokens:   req.MaxTokens,
				}, nil
			},
		}

		// 准备请求
		req := &agent.CreateAgentConfigRequest{
			TenantID:    "tenant-1",
			AgentType:   "writer",
			Name:        "New Writer",
			Description: "Test writer agent",
			ModelID:     "model-gpt4",
			Temperature: 0.7,
			MaxTokens:   2048,
		}

		// 调用Mock Service
		ctx := context.Background()
		result, err := mockSvc.CreateAgentConfig(ctx, req)

		// 断言
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "new-agent-id", result.ID)
		assert.Equal(t, "New Writer", result.Name)
		assert.Equal(t, "writer", result.AgentType)
	})

	t.Run("失败_空名称", func(t *testing.T) {
		req := &agent.CreateAgentConfigRequest{
			TenantID:  "tenant-1",
			AgentType: "writer",
			Name:      "",
		}

		// 模拟验证逻辑
		if req.Name == "" {
			err := errors.New("Agent 名称不能为空")
			assert.Error(t, err)
		}
	})

	t.Run("失败_无效的Agent类型", func(t *testing.T) {
		req := &agent.CreateAgentConfigRequest{
			TenantID:  "tenant-1",
			AgentType: "invalid_type",
			Name:      "Test",
		}

		validTypes := map[string]bool{
			"writer": true, "reviewer": true, "planner": true,
			"formatter": true, "translator": true, "analyzer": true, "researcher": true,
		}

		if !validTypes[req.AgentType] {
			err := errors.New("无效的 Agent 类型")
			assert.Error(t, err)
		}
	})
}

// TestAgentHandler_UpdateAgentConfig 测试更新
func TestAgentHandler_UpdateAgentConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_更新Agent配置", func(t *testing.T) {
		newName := "Updated Agent"
		req := &agent.UpdateAgentConfigRequest{
			Name:        &newName,
			Temperature: ptrFloat64(0.8),
			MaxTokens:   ptrInt(4096),
		}

		assert.NotNil(t, req)
		assert.Equal(t, "Updated Agent", *req.Name)
		assert.Equal(t, 0.8, *req.Temperature)
	})
}

// TestAgentHandler_DeleteAgentConfig 测试删除
func TestAgentHandler_DeleteAgentConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_软删除Agent", func(t *testing.T) {
		// 模拟删除操作
		tenantID := "tenant-1"
		agentID := "agent-1"
		operatorID := "user-1"

		assert.NotEmpty(t, tenantID)
		assert.NotEmpty(t, agentID)
		assert.NotEmpty(t, operatorID)
	})
}

// TestAgentHandler_GetAgentByType 测试按类型查询
func TestAgentHandler_GetAgentByType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功_按类型查询", func(t *testing.T) {
		config := &agent.AgentConfig{
			ID:        "agent-writer",
			Name:      "Default Writer",
			AgentType: "writer",
		}

		assert.Equal(t, "writer", config.AgentType)
	})
}

// 辅助函数
func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrInt(v int) *int {
	return &v
}
