package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiterConfig 限流配置
type RateLimiterConfig struct {
	RequestsPerSecond int           // 每秒请求数
	RequestsPerMinute int           // 每分钟请求数
	BurstSize         int           // 突发容量
	CleanupInterval   time.Duration // 清理间隔
}

// DefaultRateLimiterConfig 默认配置
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		RequestsPerSecond: 10,
		RequestsPerMinute: 300,
		BurstSize:         20,
		CleanupInterval:   5 * time.Minute,
	}
}

// clientState 客户端状态
type clientState struct {
	tokens     float64
	lastUpdate time.Time
	requests   int64     // 分钟内请求数
	minuteStart time.Time // 分钟计数开始时间
}

// RateLimiter 限流器
type RateLimiter struct {
	config  *RateLimiterConfig
	clients map[string]*clientState
	mu      sync.RWMutex
	stopCh  chan struct{}
}

// NewRateLimiter 创建限流器
func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}
	
	rl := &RateLimiter{
		config:  config,
		clients: make(map[string]*clientState),
		stopCh:  make(chan struct{}),
	}
	
	// 启动清理协程
	go rl.cleanup()
	
	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	state, exists := rl.clients[key]
	
	if !exists {
		rl.clients[key] = &clientState{
			tokens:      float64(rl.config.BurstSize - 1),
			lastUpdate:  now,
			requests:    1,
			minuteStart: now,
		}
		return true
	}
	
	// 令牌桶算法：计算新增令牌
	elapsed := now.Sub(state.lastUpdate).Seconds()
	state.tokens += elapsed * float64(rl.config.RequestsPerSecond)
	if state.tokens > float64(rl.config.BurstSize) {
		state.tokens = float64(rl.config.BurstSize)
	}
	state.lastUpdate = now
	
	// 检查分钟级限制
	if now.Sub(state.minuteStart) > time.Minute {
		state.requests = 0
		state.minuteStart = now
	}
	
	// 检查是否超过分钟限制
	if rl.config.RequestsPerMinute > 0 && state.requests >= int64(rl.config.RequestsPerMinute) {
		return false
	}
	
	// 检查令牌
	if state.tokens < 1 {
		return false
	}
	
	state.tokens--
	state.requests++
	return true
}

// cleanup 定期清理过期状态
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, state := range rl.clients {
				if now.Sub(state.lastUpdate) > 10*time.Minute {
					delete(rl.clients, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Stop 停止限流器
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// GetStats 获取统计
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	return map[string]interface{}{
		"active_clients": len(rl.clients),
		"config": map[string]interface{}{
			"requests_per_second": rl.config.RequestsPerSecond,
			"requests_per_minute": rl.config.RequestsPerMinute,
			"burst_size":          rl.config.BurstSize,
		},
	}
}

// ============================================================================
// Gin 中间件
// ============================================================================

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取客户端标识（优先使用用户ID，其次IP）
		key := c.GetString("user_id")
		if key == "" {
			key = c.ClientIP()
		}
		
		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "请求过于频繁，请稍后重试",
				"code":    "RATE_LIMIT_EXCEEDED",
				"retry_after": 1,
			})
			return
		}
		
		c.Next()
	}
}

// RateLimitByTenant 按租户限流中间件
func RateLimitByTenant(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = c.ClientIP()
		}
		
		key := "tenant:" + tenantID
		
		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "租户请求配额已用尽",
				"code":    "TENANT_RATE_LIMIT_EXCEEDED",
				"retry_after": 1,
			})
			return
		}
		
		c.Next()
	}
}

// RateLimitByEndpoint 按端点限流中间件（用于敏感 API）
func RateLimitByEndpoint(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			userID = c.ClientIP()
		}
		
		key := "endpoint:" + c.FullPath() + ":" + userID
		
		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "该接口请求过于频繁",
				"code":    "ENDPOINT_RATE_LIMIT_EXCEEDED",
				"retry_after": 1,
			})
			return
		}
		
		c.Next()
	}
}
