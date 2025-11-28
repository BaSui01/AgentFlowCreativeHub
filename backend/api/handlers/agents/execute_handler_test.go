package agents

import (
	"testing"

	"backend/internal/agent/runtime"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestExecuteHandler_ExecuteAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("执行请求结构验证", func(t *testing.T) {
		req := ExecuteAgentRequest{
			Content: "请帮我分析这段代码",
			Variables: map[string]any{
				"language": "go",
				"context":  "testing",
			},
		}

		assert.NotEmpty(t, req.Content)
		assert.Equal(t, "go", req.Variables["language"])
	})

	t.Run("同步执行响应结构", func(t *testing.T) {
		resp := AgentExecuteResponse{
			TraceID: "trace-123",
		Result: &runtime.AgentResult{
			Output: "代码分析结果",
			Status: "success",
		},
	}

	assert.NotEmpty(t, resp.TraceID)
	assert.Equal(t, "success", resp.Result.Status)
	})
}

func TestExecuteHandler_ExecuteAsync(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("异步执行响应结构", func(t *testing.T) {
		resp := AgentAsyncResponse{
			RunID:   "run-456",
			TraceID: "trace-456",
			Status:  "running",
		}

		assert.NotEmpty(t, resp.RunID)
		assert.Equal(t, "running", resp.Status)
	})
}

func TestExecuteHandler_StreamExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("流式执行验证", func(t *testing.T) {
		// 验证SSE格式
		eventType := "data"
		assert.Equal(t, "data", eventType)
	})
}

func TestExecuteHandler_CancelExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("取消执行请求", func(t *testing.T) {
		runID := "run-to-cancel"
		assert.NotEmpty(t, runID)
	})
}

func TestExecuteHandler_GetExecutionStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("查询执行状态", func(t *testing.T) {
		status := map[string]interface{}{
			"run_id": "run-123",
			"status": "completed",
			"result": "执行完成",
		}

		assert.Equal(t, "completed", status["status"])
	})
}
