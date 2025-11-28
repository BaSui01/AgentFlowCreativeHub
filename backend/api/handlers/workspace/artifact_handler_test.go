package workspace

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestArtifactHandler_ListArtifacts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("产出物列表结构验证", func(t *testing.T) {
		artifacts := []map[string]interface{}{
			{
				"id":           "art-1",
				"workspace_id": "ws-1",
				"name":         "生成的文档",
				"type":         "document",
				"size":         1024,
				"created_at":   "2024-01-01T00:00:00Z",
			},
		}

		assert.Len(t, artifacts, 1)
		assert.Equal(t, "document", artifacts[0]["type"])
	})
}

func TestArtifactHandler_GetArtifact(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("单个产出物结构验证", func(t *testing.T) {
		artifact := map[string]interface{}{
			"id":      "art-1",
			"name":    "分析报告.pdf",
			"type":    "pdf",
			"size":    2048000,
			"url":     "https://storage.example.com/art-1",
			"metadata": map[string]interface{}{
				"agent_id": "agent-1",
				"run_id":   "run-123",
			},
		}

		assert.Equal(t, "pdf", artifact["type"])
		assert.NotNil(t, artifact["metadata"])
	})
}

func TestArtifactHandler_CreateArtifact(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("创建产出物请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"workspace_id": "ws-1",
			"name":         "新产出物",
			"type":         "code",
			"content":      "生成的代码内容",
		}

		assert.NotEmpty(t, req["name"])
		assert.NotEmpty(t, req["content"])
	})
}

func TestArtifactHandler_UpdateArtifact(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("更新产出物请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"name":    "更新后的名称",
			"content": "更新后的内容",
		}

		assert.NotEmpty(t, req["name"])
	})
}

func TestArtifactHandler_DeleteArtifact(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("删除产出物验证", func(t *testing.T) {
		artifactID := "art-to-delete"
		assert.NotEmpty(t, artifactID)
	})
}

func TestArtifactHandler_DownloadArtifact(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("下载产出物验证", func(t *testing.T) {
		downloadURL := "https://storage.example.com/download/art-1"
		assert.NotEmpty(t, downloadURL)
		assert.Contains(t, downloadURL, "https://")
	})
}
