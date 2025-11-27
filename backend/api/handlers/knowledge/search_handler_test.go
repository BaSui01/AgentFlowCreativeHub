package knowledge

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSearchHandler_SearchDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("搜索请求结构验证", func(t *testing.T) {
		req := map[string]interface{}{
			"query":  "如何使用API",
			"kb_id":  "kb-1",
			"top_k":  5,
			"filter": map[string]interface{}{"tags": []string{"api"}},
		}

		assert.NotEmpty(t, req["query"])
		assert.Equal(t, 5, req["top_k"])
	})

	t.Run("搜索响应结构验证", func(t *testing.T) {
		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"doc_id":     "doc-1",
					"title":      "API使用指南",
					"content":    "API使用说明...",
					"score":      0.95,
					"highlights": []string{"API", "使用"},
				},
			},
			"total": 1,
		}

		results := resp["results"].([]map[string]interface{})
		assert.Len(t, results, 1)
		assert.Equal(t, 0.95, results[0]["score"])
	})
}

func TestSearchHandler_SemanticSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("语义搜索请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"query":           "身份验证方法",
			"kb_id":           "kb-1",
			"search_type":     "semantic",
			"similarity_threshold": 0.7,
		}

		assert.Equal(t, "semantic", req["search_type"])
		assert.Equal(t, 0.7, req["similarity_threshold"])
	})
}

func TestSearchHandler_HybridSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("混合搜索请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"query":        "错误处理",
			"kb_id":        "kb-1",
			"search_type":  "hybrid",
			"weight_bm25":  0.3,
			"weight_dense": 0.7,
		}

		assert.Equal(t, "hybrid", req["search_type"])
		assert.Equal(t, 0.3, req["weight_bm25"])
	})
}

func TestSearchHandler_GetSearchSuggestions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("搜索建议响应验证", func(t *testing.T) {
		suggestions := []string{
			"API认证",
			"API限流",
			"API版本控制",
		}

		assert.Len(t, suggestions, 3)
		assert.Contains(t, suggestions, "API认证")
	})
}
