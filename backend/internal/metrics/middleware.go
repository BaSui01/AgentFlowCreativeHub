package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// PrometheusMiddleware Prometheus 指标收集中间件
// 自动记录所有 HTTP 请求的指标（QPS、延迟、状态码等）
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过 /metrics 端点，避免自我监控
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		// 提前缓存请求体长度，避免后续读取请求体
		requestSize := c.Request.ContentLength

		// 执行请求
		c.Next()

		// 计算耗时
		duration := time.Since(start).Seconds()

		// 获取路径（使用路由模板，而非实际路径）
		path := normalizePath(c)

		// 获取状态码
		status := strconv.Itoa(c.Writer.Status())

		// 记录指标
		APIRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		APIRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)

		// 记录请求体大小
		if requestSize > 0 {
			APIRequestSize.WithLabelValues(c.Request.Method, path).Observe(float64(requestSize))
		}

		// 记录响应体大小
		if respSize := c.Writer.Size(); respSize >= 0 {
			APIResponseSize.WithLabelValues(c.Request.Method, path).Observe(float64(respSize))
		}
	}
}

// normalizePath 标准化路径（使用路由模板）
func normalizePath(c *gin.Context) string {
	// 优先使用路由模板（如 /api/agents/:id）
	path := c.FullPath()
	if path == "" {
		// 如果没有匹配的路由，使用实际路径
		path = c.Request.URL.Path
	}
	return path
}
