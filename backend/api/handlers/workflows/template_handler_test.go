package workflows

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"backend/internal/workflow"
	"backend/internal/workflow/template"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const templateYAML = `templates:
  quick:
    name: "快速写作"
    description: "用于测试的模板"
    category: "test"
    definition:
      name: "测试模板"
      version: "1.0"
      steps:
        - id: "step_1"
          type: "agent"
          agent_type: "writer"
          input:
            topic: "{{user_input.topic}}"
`

const capabilityYAML = `capabilities:
  writer:
    description: "写作能力"
    roles:
      - role: "writer"
        name: "Writer"
        description: "写作者"
        input_fields: ["topic"]
        output_fields: ["draft"]
        system_prompt: "你是写作者"
`

func setupTemplateHandlerTest(t *testing.T) (*TemplateHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:template_handler?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&workflow.Workflow{}))

	tplFile := writeTempConfig(t, templateYAML)
	capFile := writeTempConfig(t, capabilityYAML)

	loader := template.NewTemplateLoader()
	require.NoError(t, loader.LoadFromFile(tplFile))
	capLoader := template.NewAgentCapabilityLoader()
	require.NoError(t, capLoader.LoadFromFile(capFile))

	return NewTemplateHandler(db, loader, capLoader), db
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "tmpl-*.yaml")
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString(content)
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestTemplateHandlerQuickCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupTemplateHandlerTest(t)
	body := []byte(`{"template":"quick","name":"测试工作流"}`)
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/quick-create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("tenant_id", "tenant-quick")
	c.Set("user_id", "user-1")

	handler.QuickCreate(c)
	if resp.Code != http.StatusOK {
		t.Fatalf("期望 200, 实际 %d: %s", resp.Code, resp.Body.String())
	}

	var count int64
	require.NoError(t, db.Model(&workflow.Workflow{}).Count(&count).Error)
	if count != 1 {
		t.Fatalf("应创建 1 条工作流记录, 实际 %d", count)
	}
}

func TestTemplateHandlerValidateWorkflowDefinition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupTemplateHandlerTest(t)

	tests := []struct {
		name       string
		payload    map[string]any
		expectCode int
	}{
		{
			name:       "invalid definition",
			payload:    map[string]any{"definition": map[string]any{}},
			expectCode: http.StatusBadRequest,
		},
		{
			name: "valid definition",
			payload: map[string]any{"definition": map[string]any{
				"steps": []any{
					map[string]any{
						"id":         "step_1",
						"type":       "agent",
						"agent_type": "writer",
					},
				},
			}},
			expectCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(resp)
			payloadBytes, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/validate", bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req
			handler.ValidateWorkflowDefinition(c)
			if resp.Code != tc.expectCode {
				t.Fatalf("期望状态 %d, 实际 %d: %s", tc.expectCode, resp.Code, resp.Body.String())
			}
		})
	}
}
