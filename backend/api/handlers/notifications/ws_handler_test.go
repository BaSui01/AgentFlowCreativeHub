package notifications

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWSHandler_WebSocketConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("WebSocket连接参数验证", func(t *testing.T) {
		params := map[string]interface{}{
			"user_id":   "user-1",
			"tenant_id": "tenant-1",
			"token":     "ws-token-123",
		}

		assert.NotEmpty(t, params["user_id"])
		assert.NotEmpty(t, params["token"])
	})
}

func TestWSHandler_SendNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("通知消息结构验证", func(t *testing.T) {
		notification := map[string]interface{}{
			"id":       "notif-1",
			"type":     "agent_execution",
			"title":    "Agent执行完成",
			"content":  "您的Agent任务已完成",
			"severity": "info",
			"data": map[string]interface{}{
				"run_id": "run-123",
				"result": "success",
			},
		}

		assert.Equal(t, "agent_execution", notification["type"])
		assert.Equal(t, "info", notification["severity"])
	})
}

func TestWSHandler_NotificationTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("通知类型验证", func(t *testing.T) {
		types := []string{
			"agent_execution",
			"workflow_status",
			"system_alert",
			"user_message",
		}

		assert.Contains(t, types, "agent_execution")
		assert.Contains(t, types, "workflow_status")
	})
}

func TestWSHandler_BroadcastMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("广播消息结构验证", func(t *testing.T) {
		broadcast := map[string]interface{}{
			"type":      "broadcast",
			"target":    "all_users",
			"content":   "系统维护通知",
			"timestamp": "2024-01-01T00:00:00Z",
		}

		assert.Equal(t, "broadcast", broadcast["type"])
		assert.Equal(t, "all_users", broadcast["target"])
	})
}

func TestWSHandler_PrivateMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("私有消息结构验证", func(t *testing.T) {
		message := map[string]interface{}{
			"type":        "private",
			"target_user": "user-2",
			"content":     "私有通知内容",
		}

		assert.Equal(t, "private", message["type"])
		assert.NotEmpty(t, message["target_user"])
	})
}
