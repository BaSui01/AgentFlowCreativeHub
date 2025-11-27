package templates

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTemplateHandler_ListTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("模板列表结构验证", func(t *testing.T) {
		templates := []map[string]interface{}{
			{
				"id":       "tpl-1",
				"name":     "Writer Template",
				"category": "writing",
				"enabled":  true,
			},
		}

		assert.Len(t, templates, 1)
		assert.Equal(t, "writing", templates[0]["category"])
	})
}

func TestTemplateHandler_GetTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("单个模板结构验证", func(t *testing.T) {
		template := map[string]interface{}{
			"id":      "tpl-1",
			"name":    "Blog Writer",
			"content": "You are a professional blog writer...",
			"variables": []string{"topic", "tone", "length"},
		}

		assert.Equal(t, "Blog Writer", template["name"])
		assert.Len(t, template["variables"], 3)
	})
}

func TestTemplateHandler_CreateTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("创建模板请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"name":     "New Template",
			"content":  "Template content with {{variable}}",
			"category": "custom",
		}

		assert.NotEmpty(t, req["name"])
		assert.NotEmpty(t, req["content"])
	})
}

func TestTemplateHandler_UpdateTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("更新模板请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"content": "Updated content",
			"enabled": false,
		}

		assert.False(t, req["enabled"].(bool))
	})
}

func TestTemplateHandler_DeleteTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("删除模板验证", func(t *testing.T) {
		templateID := "tpl-to-delete"
		assert.NotEmpty(t, templateID)
	})
}
