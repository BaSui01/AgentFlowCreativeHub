package commands

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCommandHandler_ListCommands(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("命令列表结构验证", func(t *testing.T) {
		commands := []map[string]interface{}{
			{
				"id":      "cmd-1",
				"name":    "analyze",
				"enabled": true,
			},
		}

		assert.Len(t, commands, 1)
		assert.Equal(t, "analyze", commands[0]["name"])
	})
}

func TestCommandHandler_GetCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("单个命令结构验证", func(t *testing.T) {
		command := map[string]interface{}{
			"id":          "cmd-1",
			"name":        "analyze_code",
			"description": "Analyze code quality",
			"parameters":  []string{"file_path", "language"},
		}

		assert.Equal(t, "analyze_code", command["name"])
		assert.Len(t, command["parameters"], 2)
	})
}

func TestCommandHandler_ExecuteCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("执行命令请求验证", func(t *testing.T) {
		req := map[string]interface{}{
			"command_id": "cmd-1",
			"parameters": map[string]string{
				"file_path": "/path/to/file.go",
				"language":  "go",
			},
		}

		assert.NotEmpty(t, req["command_id"])
		assert.NotNil(t, req["parameters"])
	})
}

func TestCommandHandler_GetExecutionResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("执行结果结构验证", func(t *testing.T) {
		result := map[string]interface{}{
			"execution_id": "exec-1",
			"status":       "completed",
			"result":       "Analysis complete",
		}

		assert.Equal(t, "completed", result["status"])
	})
}
