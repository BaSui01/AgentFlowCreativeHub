package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	response "backend/api/handlers/common"
	"backend/internal/models"
	"backend/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// HTTP Integration Tests - 测试完整HTTP请求响应流程
// ============================================================

// TestKBHandler_List_HTTP 测试列表查询HTTP接口
func TestKBHandler_List_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功返回200和正确的JSON结构", func(t *testing.T) {
		// 创建Mock Service
		mockSvc := &MockKBService{
			ListKnowledgeBasesFunc: func(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*models.KnowledgeBase, *types.PaginationResponse, error) {
				// 验证请求参数
				assert.Equal(t, "test-tenant", tenantID)
				assert.Equal(t, 1, pagination.Page)
				assert.Equal(t, 20, pagination.PageSize)
				
				return []*models.KnowledgeBase{
					{ID: "kb-1", Name: "技术文档库", Description: "技术文档集合", Type: "document", DocCount: 42},
					{ID: "kb-2", Name: "产品手册库", Description: "产品手册集合", Type: "document", DocCount: 28},
				}, &types.PaginationResponse{
					Page:       1,
					PageSize:   20,
					TotalItems: 2,
					TotalPages: 1,
				}, nil
			},
		}

		// 创建测试Handler
		handler := &testKBHandler{mockService: mockSvc}
		
		// 设置路由
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.GET("/api/knowledge-bases", handler.List)
		
		// 创建HTTP请求
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/knowledge-bases?page=1&page_size=20", nil)
		
		// 执行请求
		router.ServeHTTP(w, req)
		
		// 验证HTTP响应
		assert.Equal(t, http.StatusOK, w.Code)
		
		// 解析响应JSON
		var resp response.ListResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		
		// 验证响应内容
		assert.Len(t, resp.Items, 2)
		assert.NotNil(t, resp.Pagination)
		assert.Equal(t, 1, resp.Pagination.Page)
	})

	t.Run("HTTP_Service错误返回500", func(t *testing.T) {
		mockSvc := &MockKBService{
			ListKnowledgeBasesFunc: func(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*models.KnowledgeBase, *types.PaginationResponse, error) {
				return nil, nil, errors.New("数据库连接失败")
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.GET("/api/knowledge-bases", handler.List)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/knowledge-bases", nil)
		router.ServeHTTP(w, req)
		
		// 验证错误响应
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		
		var errorResp response.ErrorResponse
		json.Unmarshal(w.Body.Bytes(), &errorResp)
		assert.False(t, errorResp.Success)
		assert.Contains(t, errorResp.Message, "数据库连接失败")
	})

	t.Run("HTTP_分页参数验证", func(t *testing.T) {
		mockSvc := &MockKBService{
			ListKnowledgeBasesFunc: func(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*models.KnowledgeBase, *types.PaginationResponse, error) {
				// 验证分页参数被正确解析
				assert.Equal(t, 3, pagination.Page)
				assert.Equal(t, 50, pagination.PageSize)
				
				return []*models.KnowledgeBase{}, &types.PaginationResponse{
					Page:       3,
					PageSize:   50,
					TotalItems: 150,
					TotalPages: 3,
				}, nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.GET("/api/knowledge-bases", handler.List)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/knowledge-bases?page=3&page_size=50", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestKBHandler_Get_HTTP 测试单个查询HTTP接口
func TestKBHandler_Get_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功返回知识库详情", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				assert.Equal(t, "kb-123", id)
				
				return &models.KnowledgeBase{
					ID:          "kb-123",
					TenantID:    "test-tenant",
					Name:        "我的知识库",
					Description: "测试知识库",
					Type:        "document",
					Status:      "active",
					DocCount:    100,
				}, nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.GET("/api/knowledge-bases/:id", handler.Get)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/knowledge-bases/kb-123", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var resp models.KnowledgeBase
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "kb-123", resp.ID)
		assert.Equal(t, "我的知识库", resp.Name)
		assert.Equal(t, 100, resp.DocCount)
	})

	t.Run("HTTP_知识库不存在返回404", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				return nil, nil // 返回nil表示不存在
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.GET("/api/knowledge-bases/:id", handler.Get)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/knowledge-bases/nonexistent", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("HTTP_跨租户访问返回403", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				// 返回不同租户的知识库
				return &models.KnowledgeBase{
					ID:       "kb-123",
					TenantID: "other-tenant", // 不同的租户
					Name:     "其他租户的知识库",
				}, nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.GET("/api/knowledge-bases/:id", handler.Get)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/knowledge-bases/kb-123", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// TestKBHandler_Create_HTTP 测试创建HTTP接口
func TestKBHandler_Create_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功创建返回201", func(t *testing.T) {
		mockSvc := &MockKBService{
			CreateKnowledgeBaseFunc: func(ctx context.Context, kb *models.KnowledgeBase) error {
				// 验证请求参数
				assert.Equal(t, "test-tenant", kb.TenantID)
				assert.Equal(t, "新知识库", kb.Name)
				assert.Equal(t, "document", kb.Type)
				assert.Equal(t, "test-user", kb.CreatedBy)
				
				// 模拟数据库生成ID
				kb.ID = "new-kb-id"
				return nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.POST("/api/knowledge-bases", handler.Create)
		
		// 创建请求Body
		requestBody := map[string]interface{}{
			"name":        "新知识库",
			"description": "测试创建",
			"type":        "document",
			"config":      map[string]interface{}{"chunk_size": 500},
		}
		bodyBytes, _ := json.Marshal(requestBody)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/knowledge-bases", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusCreated, w.Code)
		
		var resp models.KnowledgeBase
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "new-kb-id", resp.ID)
		assert.Equal(t, "新知识库", resp.Name)
	})

	t.Run("HTTP_参数验证失败返回400", func(t *testing.T) {
		mockSvc := &MockKBService{
			CreateKnowledgeBaseFunc: func(ctx context.Context, kb *models.KnowledgeBase) error {
				return nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.POST("/api/knowledge-bases", handler.Create)
		
		// 缺少必填字段name
		requestBody := map[string]interface{}{
			"type": "document",
		}
		bodyBytes, _ := json.Marshal(requestBody)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/knowledge-bases", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("HTTP_Service错误返回500", func(t *testing.T) {
		mockSvc := &MockKBService{
			CreateKnowledgeBaseFunc: func(ctx context.Context, kb *models.KnowledgeBase) error {
				return errors.New("数据库写入失败")
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.POST("/api/knowledge-bases", handler.Create)
		
		requestBody := map[string]interface{}{
			"name": "新知识库",
			"type": "document",
		}
		bodyBytes, _ := json.Marshal(requestBody)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/knowledge-bases", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestKBHandler_Update_HTTP 测试更新HTTP接口
func TestKBHandler_Update_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功更新返回200", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				return &models.KnowledgeBase{
					ID:       "kb-123",
					TenantID: "test-tenant",
					Name:     "原知识库名称",
				}, nil
			},
			UpdateKnowledgeBaseFunc: func(ctx context.Context, kb *models.KnowledgeBase) error {
				assert.Equal(t, "kb-123", kb.ID)
				assert.Equal(t, "更新后的名称", kb.Name)
				assert.Equal(t, "test-user", kb.UpdatedBy)
				return nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.PUT("/api/knowledge-bases/:id", handler.Update)
		
		requestBody := map[string]interface{}{
			"name":        "更新后的名称",
			"description": "更新后的描述",
		}
		bodyBytes, _ := json.Marshal(requestBody)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/knowledge-bases/kb-123", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("HTTP_知识库不存在返回404", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				return nil, nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.PUT("/api/knowledge-bases/:id", handler.Update)
		
		requestBody := map[string]interface{}{"name": "更新名称"}
		bodyBytes, _ := json.Marshal(requestBody)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/knowledge-bases/nonexistent", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestKBHandler_Delete_HTTP 测试删除HTTP接口
func TestKBHandler_Delete_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功删除返回200", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				return &models.KnowledgeBase{
					ID:       "kb-123",
					TenantID: "test-tenant",
					Name:     "待删除的知识库",
				}, nil
			},
			DeleteKnowledgeBaseFunc: func(ctx context.Context, id string) error {
				assert.Equal(t, "kb-123", id)
				return nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.DELETE("/api/knowledge-bases/:id", handler.Delete)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/knowledge-bases/kb-123", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp["success"].(bool))
	})

	t.Run("HTTP_知识库不存在返回404", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				return nil, nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.DELETE("/api/knowledge-bases/:id", handler.Delete)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/knowledge-bases/nonexistent", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("HTTP_跨租户删除返回403", func(t *testing.T) {
		mockSvc := &MockKBService{
			GetKnowledgeBaseFunc: func(ctx context.Context, id string) (*models.KnowledgeBase, error) {
				return &models.KnowledgeBase{
					ID:       "kb-123",
					TenantID: "other-tenant", // 不同租户
					Name:     "其他租户的知识库",
				}, nil
			},
		}

		handler := &testKBHandler{mockService: mockSvc}
		router := gin.New()
		router.Use(mockAuthMiddleware("test-tenant", "test-user"))
		router.DELETE("/api/knowledge-bases/:id", handler.Delete)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/knowledge-bases/kb-123", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// ============================================================
// 测试辅助工具
// ============================================================

// testKBHandler 测试专用Handler（包装Mock Service）
type testKBHandler struct {
	mockService KBServiceInterface
}

// List 实现Handler方法（调用Mock Service）
func (h *testKBHandler) List(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	
	// 解析分页参数
	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	
	pagination := &types.PaginationRequest{
		Page:     page,
		PageSize: pageSize,
	}
	
	// 调用Mock Service
	kbs, paginationResp, err := h.mockService.ListKnowledgeBases(c.Request.Context(), tenantID, pagination)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询知识库失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, response.ListResponse{
		Items:      kbs,
		Pagination: toPaginationMeta(paginationResp),
	})
}

// Get 实现Handler方法
func (h *testKBHandler) Get(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id := c.Param("id")
	
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}
	
	kb, err := h.mockService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询知识库失败: " + err.Error()})
		return
	}
	
	if kb == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "知识库不存在"})
		return
	}
	
	// 检查权限
	if kb.TenantID != tenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}
	
	c.JSON(http.StatusOK, kb)
}

// Create 实现Handler方法
func (h *testKBHandler) Create(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	
	var req struct {
		Name        string                 `json:"name" binding:"required,min=1,max=200"`
		Description string                 `json:"description"`
		Type        string                 `json:"type" binding:"required,oneof=document url api database"`
		Config      map[string]interface{} `json:"config"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	
	kb := &models.KnowledgeBase{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Status:      "active",
		Config:      req.Config,
		Metadata:    req.Metadata,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}
	
	if err := h.mockService.CreateKnowledgeBase(c.Request.Context(), kb); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "创建知识库失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, kb)
}

// Update 实现Handler方法
func (h *testKBHandler) Update(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	id := c.Param("id")
	
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}
	
	var req struct {
		Name        string                 `json:"name" binding:"omitempty,min=1,max=200"`
		Description string                 `json:"description"`
		Type        string                 `json:"type" binding:"omitempty,oneof=document url api database"`
		Config      map[string]interface{} `json:"config"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	
	// 获取现有知识库
	kb, err := h.mockService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询知识库失败: " + err.Error()})
		return
	}
	
	if kb == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "知识库不存在"})
		return
	}
	
	// 检查权限
	if kb.TenantID != tenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}
	
	// 更新字段
	if req.Name != "" {
		kb.Name = req.Name
	}
	kb.Description = req.Description
	if req.Type != "" {
		kb.Type = req.Type
	}
	if req.Config != nil {
		kb.Config = req.Config
	}
	if req.Metadata != nil {
		kb.Metadata = req.Metadata
	}
	kb.UpdatedBy = userID
	
	if err := h.mockService.UpdateKnowledgeBase(c.Request.Context(), kb); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "更新知识库失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, kb)
}

// Delete 实现Handler方法
func (h *testKBHandler) Delete(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id := c.Param("id")
	
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}
	
	// 获取知识库
	kb, err := h.mockService.GetKnowledgeBase(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询知识库失败: " + err.Error()})
		return
	}
	
	if kb == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "知识库不存在"})
		return
	}
	
	// 检查权限
	if kb.TenantID != tenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}
	
	if err := h.mockService.DeleteKnowledgeBase(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "删除知识库失败: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "知识库已删除"})
}

// mockAuthMiddleware 模拟认证中间件（设置tenant_id和user_id）
func mockAuthMiddleware(tenantID, userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("tenant_id", tenantID)
		c.Set("user_id", userID)
		c.Next()
	}
}
