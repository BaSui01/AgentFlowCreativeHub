package audit

import (
	"time"

	"backend/internal/auth"
	"backend/internal/models"

	"github.com/gin-gonic/gin"
)

// AuditMiddleware 审计日志中间件
func AuditMiddleware(auditService *models.AuditLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录开始时间
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 创建审计日志
		log := &models.AuditLog{
			IPAddress:     getClientIP(c),
			UserAgent:     c.Request.UserAgent(),
			RequestPath:   c.Request.URL.Path,
			RequestMethod: c.Request.Method,
			StatusCode:    c.Writer.Status(),
			CreatedAt:     time.Now(),
		}

		// 获取用户上下文
		if userCtx, exists := auth.GetUserContext(c); exists {
			log.UserID = userCtx.UserID
			log.TenantID = userCtx.TenantID
		}

		// 根据路径和方法推断事件类型
		eventType := inferEventType(c.Request.Method, c.Request.URL.Path, c.Writer.Status())
		log.EventType = string(eventType)

		// 添加元数据
		metadata := make(map[string]interface{})
		metadata["duration"] = time.Since(startTime).Milliseconds()

		// 如果有错误信息，添加到元数据
		if len(c.Errors) > 0 {
			metadata["errors"] = c.Errors.Errors()
		}

		// 从上下文获取额外元数据（可由 handler 设置）
		if meta, exists := c.Get("audit_metadata"); exists {
			if metaMap, ok := meta.(map[string]interface{}); ok {
				for k, v := range metaMap {
					metadata[k] = v
				}
			}
		}

		log.Metadata = metadata

		// 异步写入审计日志（不阻塞请求）
		go func() {
			_ = auditService.CreateLog(c.Request.Context(), log)
		}()
	}
}

// inferEventType 根据请求路径和方法推断事件类型
func inferEventType(method, path string, statusCode int) EventType {
	// 认证相关
	if contains(path, "/auth/login") {
		if statusCode >= 200 && statusCode < 300 {
			return EventUserLogin
		}
		return EventUserLoginFailed
	}
	if contains(path, "/auth/logout") {
		return EventUserLogout
	}
	if contains(path, "/auth/register") {
		return EventUserRegister
	}
	if contains(path, "/auth/refresh") {
		return EventTokenRefresh
	}

	// 模型相关
	if contains(path, "/models") {
		switch method {
		case "POST":
			return EventModelCreate
		case "PUT", "PATCH":
			return EventModelUpdate
		case "DELETE":
			return EventModelDelete
		case "GET":
			return EventModelView
		}
	}

	// 模板相关
	if contains(path, "/templates") {
		switch method {
		case "POST":
			if contains(path, "/render") {
				return EventTemplateRender
			}
			return EventTemplateCreate
		case "PUT", "PATCH":
			return EventTemplateUpdate
		case "DELETE":
			return EventTemplateDelete
		}
	}

	// Agent 相关
	if contains(path, "/agents") {
		switch method {
		case "POST":
			if contains(path, "/execute") {
				return EventAgentExecute
			}
			return EventAgentCreate
		case "PUT", "PATCH":
			return EventAgentUpdate
		case "DELETE":
			return EventAgentDelete
		}
	}

	// Workflow 相关
	if contains(path, "/workflows") {
		switch method {
		case "POST":
			if contains(path, "/execute") {
				return EventWorkflowExecute
			}
			return EventWorkflowCreate
		case "PUT", "PATCH":
			return EventWorkflowUpdate
		case "DELETE":
			return EventWorkflowDelete
		}
	}

	// 数据相关
	if contains(path, "/export") {
		return EventDataExport
	}
	if contains(path, "/import") {
		return EventDataImport
	}

	// 默认：数据查询
	if method == "GET" {
		return EventDataQuery
	}

	// 其他操作
	return EventType("api.request")
}

// SetAuditMetadata 在上下文中设置审计元数据（供 handler 使用）
func SetAuditMetadata(c *gin.Context, key string, value interface{}) {
	var metadata map[string]interface{}

	if meta, exists := c.Get("audit_metadata"); exists {
		if metaMap, ok := meta.(map[string]interface{}); ok {
			metadata = metaMap
		} else {
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	metadata[key] = value
	c.Set("audit_metadata", metadata)
}

// SetAuditResourceInfo 设置资源信息到审计元数据
func SetAuditResourceInfo(c *gin.Context, resourceType, resourceID string) {
	SetAuditMetadata(c, "resource_type", resourceType)
	SetAuditMetadata(c, "resource_id", resourceID)
}

// SetAuditChanges 设置变更内容到审计元数据
func SetAuditChanges(c *gin.Context, changes map[string]interface{}) {
	SetAuditMetadata(c, "changes", changes)
}

// getClientIP 获取客户端 IP 地址
func getClientIP(c *gin.Context) string {
	// 优先从 X-Forwarded-For 获取
	clientIP := c.GetHeader("X-Forwarded-For")
	if clientIP != "" {
		return clientIP
	}

	// 其次从 X-Real-IP 获取
	clientIP = c.GetHeader("X-Real-IP")
	if clientIP != "" {
		return clientIP
	}

	// 最后使用 RemoteAddr
	return c.ClientIP()
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
