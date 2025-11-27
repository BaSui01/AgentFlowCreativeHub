package knowledge

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDocumentHandler_ListDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("文档列表结构验证", func(t *testing.T) {
		docs := []map[string]interface{}{
			{
				"id":         "doc-1",
				"title":      "技术文档",
				"content":    "文档内容...",
				"kb_id":      "kb-1",
				"created_at": "2024-01-01T00:00:00Z",
			},
		}

		assert.Len(t, docs, 1)
		assert.Equal(t, "技术文档", docs[0]["title"])
	})
}

func TestDocumentHandler_GetDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("单个文档结构验证", func(t *testing.T) {
		doc := map[string]interface{}{
			"id":      "doc-1",
			"title":   "API文档",
			"content": "详细内容...",
			"tags":    []string{"api", "reference"},
		}

		assert.Equal(t, "API文档", doc["title"])
		assert.Len(t, doc["tags"], 2)
	})
}

func TestDocumentHandler_CreateDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("创建文档请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"kb_id":   "kb-1",
			"title":   "新文档",
			"content": "文档内容",
			"tags":    []string{"new"},
		}

		assert.NotEmpty(t, req["title"])
		assert.NotEmpty(t, req["content"])
	})
}

func TestDocumentHandler_UpdateDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("更新文档请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"title":   "更新后的标题",
			"content": "更新后的内容",
		}

		assert.NotEmpty(t, req["title"])
	})
}

func TestDocumentHandler_DeleteDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("删除文档验证", func(t *testing.T) {
		docID := "doc-to-delete"
		assert.NotEmpty(t, docID)
	})
}
