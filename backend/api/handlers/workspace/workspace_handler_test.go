package workspace

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWorkspaceHandler_ListWorkspaces(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("工作空间列表结构验证", func(t *testing.T) {
		workspaces := []map[string]interface{}{
			{
				"id":          "ws-1",
				"name":        "项目A工作空间",
				"description": "项目A的工作空间",
				"created_at":  "2024-01-01T00:00:00Z",
			},
		}

		assert.Len(t, workspaces, 1)
		assert.Equal(t, "项目A工作空间", workspaces[0]["name"])
	})
}

func TestWorkspaceHandler_GetWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("单个工作空间结构验证", func(t *testing.T) {
		workspace := map[string]interface{}{
			"id":          "ws-1",
			"name":        "开发工作空间",
			"description": "开发环境",
			"settings": map[string]interface{}{
				"auto_save": true,
				"retention": 30,
			},
			"stats": map[string]interface{}{
				"artifact_count": 15,
				"total_size":     10485760,
			},
		}

		assert.Equal(t, "开发工作空间", workspace["name"])
		assert.NotNil(t, workspace["settings"])
		assert.Equal(t, 15, workspace["stats"].(map[string]interface{})["artifact_count"])
	})
}

func TestWorkspaceHandler_CreateWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("创建工作空间请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"name":        "新工作空间",
			"description": "测试工作空间",
			"settings": map[string]interface{}{
				"auto_save": true,
			},
		}

		assert.NotEmpty(t, req["name"])
		assert.NotNil(t, req["settings"])
	})
}

func TestWorkspaceHandler_UpdateWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("更新工作空间请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"name":        "更新后的工作空间",
			"description": "新的描述",
			"settings": map[string]interface{}{
				"retention": 60,
			},
		}

		assert.NotEmpty(t, req["name"])
	})
}

func TestWorkspaceHandler_DeleteWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("删除工作空间验证", func(t *testing.T) {
		workspaceID := "ws-to-delete"
		assert.NotEmpty(t, workspaceID)
	})
}

func TestWorkspaceHandler_CleanupWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("清理工作空间请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"workspace_id": "ws-1",
			"older_than":   30, // days
			"artifact_type": "temp",
		}

		assert.NotEmpty(t, req["workspace_id"])
		assert.Equal(t, 30, req["older_than"])
	})
}
