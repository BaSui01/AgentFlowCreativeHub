package api

import (
	"strings"
	"time"

	"backend/internal/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger 请求日志中间件
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		logger.Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowedOrigins := getEnvList("CORS_ALLOW_ORIGINS")
		origin := c.GetHeader("Origin")

		switch {
		case len(allowedOrigins) == 0:
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		case origin != "" && stringInSlice(origin, allowedOrigins):
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		allowedHeaders := defaultIfEmpty(
			getEnvList("CORS_ALLOW_HEADERS"),
			[]string{
				"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization",
				"Accept", "Origin", "Cache-Control", "X-Requested-With",
			},
		)
		c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))

		allowedMethods := defaultIfEmpty(
			getEnvList("CORS_ALLOW_METHODS"),
			[]string{"POST", "OPTIONS", "GET", "PUT", "DELETE", "PATCH"},
		)
		c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
		c.Writer.Header().Set("Access-Control-Max-Age", "600")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
