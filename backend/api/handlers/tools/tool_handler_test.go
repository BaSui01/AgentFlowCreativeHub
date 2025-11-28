package tools

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/tools"

	"github.com/gin-gonic/gin"
)

func setupHandler() *ToolHandler {
	gin.SetMode(gin.TestMode)
	registry := tools.NewToolRegistry()
	executor := tools.NewToolExecutor(registry, nil)
	return NewToolHandler(registry, executor, nil)
}

func TestRegisterToolHTTPAPI(t *testing.T) {
	handler := setupHandler()
	body := map[string]any{
		"name":        "weather_api",
		"displayName": "天气查询",
		"description": "查询天气",
		"category":    "api",
		"type":        "http_api",
		"httpConfig": map[string]any{
			"method": "GET",
			"url":    "https://example.com/weather",
		},
	}
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/tools/register", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("tenant_id", "tenant-1")
	handler.RegisterTool(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
	def, exists := handler.registry.GetDefinition("weather_api")
	if !exists {
		t.Fatalf("tool not registered in registry")
	}
	if def.TenantID != "tenant-1" {
		t.Fatalf("expected tenant ID tenant-1, got %s", def.TenantID)
	}
}

func TestListToolsFiltersByTenant(t *testing.T) {
	handler := setupHandler()
	// 注册一个租户工具
	def := &tools.ToolDefinition{
		ID:          "tool-1",
		TenantID:    "tenant-1",
		Name:        "tenant_tool",
		DisplayName: "Tenant Tool",
		Description: "",
		Category:    "api",
		Type:        "http_api",
		HTTPConfig: &tools.HTTPToolConfig{
			Method: "GET",
			URL:    "https://example.com",
		},
	}
	if err := handler.registry.Register("tenant_tool", tools.NewDynamicHTTPTool(def), def); err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}
	// 列表请求来自其他租户
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	c.Request = req
	c.Set("tenant_id", "tenant-2")
	handler.ListTools(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	var resp struct {
		Tools []any `json:"tools"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response decode failed: %v", err)
	}
	if len(resp.Tools) != 0 {
		t.Fatalf("expected no tools for other tenant, got %d", len(resp.Tools))
	}
}

func TestUnregisterToolRequiresOwnership(t *testing.T) {
	handler := setupHandler()
	definition := &tools.ToolDefinition{
		ID:          "tool-2",
		TenantID:    "tenant-1",
		Name:        "private_tool",
		DisplayName: "Private",
		Description: "",
		Category:    "api",
		Type:        "http_api",
		HTTPConfig: &tools.HTTPToolConfig{
			Method: "GET",
			URL:    "https://example.com",
		},
	}
	if err := handler.registry.Register("private_tool", tools.NewDynamicHTTPTool(definition), definition); err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodDelete, "/api/tools/private_tool", nil)
	c.Request = req
	c.Set("tenant_id", "tenant-2")
	c.Params = gin.Params{{Key: "name", Value: "private_tool"}}
	handler.UnregisterTool(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}
	if _, exists := handler.registry.GetDefinition("private_tool"); !exists {
		t.Fatalf("tool should still exist after forbidden request")
	}
}


// ============================================================
// HTTP Integration Tests - 测试GetTool和ExecuteTool
// ============================================================

// TestGetTool_HTTP 测试获取工具详情HTTP接口
func TestGetTool_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功返回工具详情", func(t *testing.T) {
		handler := setupHandler()
		
		// 注册一个工具
		def := &tools.ToolDefinition{
			ID:          "test-tool-1",
			TenantID:    "",  // 系统工具
			Name:        "test_tool",
			DisplayName: "Test Tool",
			Description: "A test tool",
			Category:    "api",
			Type:        "http_api",
			HTTPConfig: &tools.HTTPToolConfig{
				Method: "GET",
				URL:    "https://example.com/test",
			},
		}
		handler.registry.Register("test_tool", tools.NewDynamicHTTPTool(def), def)

		router := gin.New()
		router.GET("/api/tools/:name", func(c *gin.Context) {
			c.Set("tenant_id", "tenant-1")
			handler.GetTool(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tools/test_tool", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
		}

		var respDef tools.ToolDefinition
		if err := json.Unmarshal(w.Body.Bytes(), &respDef); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if respDef.Name != "test_tool" {
			t.Fatalf("expected tool name test_tool, got %s", respDef.Name)
		}
	})

	t.Run("HTTP_工具不存在返回404", func(t *testing.T) {
		handler := setupHandler()

		router := gin.New()
		router.GET("/api/tools/:name", func(c *gin.Context) {
			c.Set("tenant_id", "tenant-1")
			handler.GetTool(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tools/nonexistent_tool", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("HTTP_跨租户访问私有工具返回404", func(t *testing.T) {
		handler := setupHandler()
		
		// 注册租户私有工具
		def := &tools.ToolDefinition{
			ID:          "private-tool-1",
			TenantID:    "tenant-1",  // 租户1的私有工具
			Name:        "private_tool",
			DisplayName: "Private Tool",
			Description: "A private tool",
			Category:    "api",
			Type:        "http_api",
			HTTPConfig: &tools.HTTPToolConfig{
				Method: "GET",
				URL:    "https://example.com/private",
			},
		}
		handler.registry.Register("private_tool", tools.NewDynamicHTTPTool(def), def)

		router := gin.New()
		router.GET("/api/tools/:name", func(c *gin.Context) {
			c.Set("tenant_id", "tenant-2")  // 租户2尝试访问
			handler.GetTool(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tools/private_tool", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404 for cross-tenant access, got %d", w.Code)
		}
	})
}

// TestExecuteTool_HTTP 测试执行工具HTTP接口
func TestExecuteTool_HTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HTTP_成功执行工具", func(t *testing.T) {
		handler := setupHandler()
		
		// 注册一个简单的测试工具
		def := &tools.ToolDefinition{
			ID:          "echo-tool",
			TenantID:    "",
			Name:        "echo_tool",
			DisplayName: "Echo Tool",
			Description: "Returns input",
			Category:    "utility",
			Type:        "http_api",
			HTTPConfig: &tools.HTTPToolConfig{
				Method: "POST",
				URL:    "https://httpbin.org/post",
			},
		}
		handler.registry.Register("echo_tool", tools.NewDynamicHTTPTool(def), def)

		router := gin.New()
		router.POST("/api/tools/:name/execute", func(c *gin.Context) {
			c.Set("tenant_id", "tenant-1")
			c.Set("user_id", "user-1")
			handler.ExecuteTool(c)
		})

		requestBody := map[string]any{
			"input": map[string]any{
				"message": "test message",
			},
		}
		bodyBytes, _ := json.Marshal(requestBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tools/echo_tool/execute", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["tool_name"] != "echo_tool" {
			t.Fatalf("expected tool_name echo_tool, got %v", resp["tool_name"])
		}
	})

	t.Run("HTTP_工具不存在返回404", func(t *testing.T) {
		handler := setupHandler()

		router := gin.New()
		router.POST("/api/tools/:name/execute", func(c *gin.Context) {
			c.Set("tenant_id", "tenant-1")
			c.Set("user_id", "user-1")
			handler.ExecuteTool(c)
		})

		requestBody := map[string]any{
			"input": map[string]any{
				"test": "data",
			},
		}
		bodyBytes, _ := json.Marshal(requestBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tools/nonexistent_tool/execute", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("HTTP_参数错误返回400", func(t *testing.T) {
		handler := setupHandler()
		
		// 注册工具
		def := &tools.ToolDefinition{
			ID:          "test-tool-2",
			TenantID:    "",
			Name:        "test_tool_2",
			DisplayName: "Test Tool 2",
			Description: "Test tool",
			Category:    "api",
			Type:        "http_api",
			HTTPConfig: &tools.HTTPToolConfig{
				Method: "GET",
				URL:    "https://example.com",
			},
		}
		handler.registry.Register("test_tool_2", tools.NewDynamicHTTPTool(def), def)

		router := gin.New()
		router.POST("/api/tools/:name/execute", func(c *gin.Context) {
			c.Set("tenant_id", "tenant-1")
			c.Set("user_id", "user-1")
			handler.ExecuteTool(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tools/test_tool_2/execute", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})
}
