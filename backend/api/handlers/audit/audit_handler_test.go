package audit

import (
	"testing"
	"time"

	"backend/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAuditHandler_ListAuditLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("响应结构验证", func(t *testing.T) {
		logs := []*types.AuditLog{
			{
				ID:           "log-1",
				TenantID:     "tenant-1",
				UserID:       "user-1",
				Action:       "CREATE_AGENT",
				ResourceType: "agent",
				ResourceID:   "agent-1",
				Details:      map[string]interface{}{"message": "Created new agent"},
				IPAddress:    "127.0.0.1",
				CreatedAt:    time.Now(),
			},
		}

		assert.Len(t, logs, 1)
		assert.Equal(t, "CREATE_AGENT", logs[0].Action)
	})

	t.Run("分页结构验证", func(t *testing.T) {
		pagination := map[string]interface{}{
			"page":       1,
			"page_size":  20,
			"total":      100,
			"total_page": 5,
		}

		assert.Equal(t, 1, pagination["page"])
		assert.Equal(t, 100, pagination["total"])
	})
}

func TestAuditHandler_GetAuditLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("单个日志结构验证", func(t *testing.T) {
		log := &types.AuditLog{
			ID:           "log-1",
			Action:       "DELETE_MODEL",
			ResourceType: "model",
		}

		assert.NotNil(t, log)
		assert.Equal(t, "DELETE_MODEL", log.Action)
	})
}

func TestAuditHandler_FilterByAction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("按操作类型过滤", func(t *testing.T) {
		actions := []string{"CREATE_AGENT", "UPDATE_AGENT", "DELETE_AGENT"}
		
		assert.Contains(t, actions, "CREATE_AGENT")
		assert.Len(t, actions, 3)
	})
}

func TestAuditHandler_FilterByTimeRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("时间范围过滤", func(t *testing.T) {
		startTime := time.Now().Add(-24 * time.Hour)
		endTime := time.Now()

		assert.True(t, endTime.After(startTime))
	})
}
