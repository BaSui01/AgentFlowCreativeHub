package tools

import (
	"strings"
	"testing"
)

func TestDynamicHTTPToolBuildPayload(t *testing.T) {
	tool := NewDynamicHTTPTool(&ToolDefinition{
		HTTPConfig: &HTTPToolConfig{
			Method: "GET",
			URL:    "https://api.example.com/weather",
			Headers: map[string]string{
				"X-Default": "base",
			},
			Auth: &AuthConfig{Type: "bearer", Token: "base-token"},
		},
	})
	payload, err := tool.buildPayload(map[string]any{
		"method":  "post",
		"query":   map[string]any{"city": "shenzhen"},
		"headers": map[string]any{"X-Default": "override"},
		"auth":    map[string]any{"token": "override-token"},
	})
	if err != nil {
		t.Fatalf("buildPayload failed: %v", err)
	}
	if payload["method"].(string) != "POST" {
		t.Fatalf("expected method POST, got %v", payload["method"])
	}
	url := payload["url"].(string)
	if !strings.Contains(url, "city=shenzhen") {
		t.Fatalf("expected query string in url, got %s", url)
	}
	headers := payload["headers"].(map[string]string)
	if headers["X-Default"] != "override" {
		t.Fatalf("expected header override, got %v", headers["X-Default"])
	}
	auth := payload["auth"].(map[string]any)
	if auth["token"].(string) != "override-token" {
		t.Fatalf("expected auth override, got %v", auth["token"])
	}
}

func TestDynamicHTTPToolMissingConfig(t *testing.T) {
	tool := NewDynamicHTTPTool(&ToolDefinition{})
	if _, err := tool.buildPayload(nil); err == nil {
		t.Fatalf("expected error when config missing")
	}
}
