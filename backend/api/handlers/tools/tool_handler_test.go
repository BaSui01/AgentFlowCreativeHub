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
