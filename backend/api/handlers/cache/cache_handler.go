package cache

import (
	"net/http"

	"backend/internal/cache"
	
	"github.com/gin-gonic/gin"
)

// CacheHandler 缓存统计API处理器
type CacheHandler struct {
	diskCache *cache.DiskCache
}

// NewCacheHandler 创建缓存处理器
func NewCacheHandler(diskCache *cache.DiskCache) *CacheHandler {
	return &CacheHandler{
		diskCache: diskCache,
	}
}

// GetStats 获取缓存统计信息
// @Summary 获取缓存统计
// @Description 获取LLM缓存的统计信息，包括命中率、缓存大小等
// @Tags 缓存
// @Produce json
// @Success 200 {object} map[string]interface{} "缓存统计数据"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /api/v1/cache/stats [get]
func (h *CacheHandler) GetStats(c *gin.Context) {
	stats, err := h.diskCache.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取缓存统计失败",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": stats,
	})
}

// GetHealth 获取缓存健康状态
// @Summary 缓存健康检查
// @Description 检查缓存系统是否正常运行
// @Tags 缓存
// @Produce json
// @Success 200 {object} map[string]interface{} "健康状态"
// @Router /api/v1/cache/health [get]
func (h *CacheHandler) GetHealth(c *gin.Context) {
	stats, err := h.diskCache.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"status": "unhealthy",
			"error": err.Error(),
		})
		return
	}

	// 判断健康状态
	healthy := true
	warnings := []string{}

	// 检查命中率
	if hitRate, ok := stats["hit_rate_percent"].(float64); ok {
		if hitRate < 50 && stats["total_requests"].(int64) > 100 {
			warnings = append(warnings, "缓存命中率较低")
		}
	}

	// 检查缓存大小
	if sizeGB, ok := stats["total_size_mb"].(float64); ok {
		if sizeGB > 18000 { // 接近20GB限制
			warnings = append(warnings, "缓存空间即将满")
		}
	}

	status := "healthy"
	if len(warnings) > 0 {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status": status,
		"healthy": healthy,
		"warnings": warnings,
		"stats": stats,
	})
}

// ClearCache 清空缓存（危险操作，需要管理员权限）
// @Summary 清空缓存
// @Description 清空所有LLM缓存数据
// @Tags 缓存
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} map[string]interface{} "操作结果"
// @Failure 403 {object} map[string]interface{} "权限不足"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /api/v1/cache/clear [post]
func (h *CacheHandler) ClearCache(c *gin.Context) {
	// TODO: 添加管理员权限检查
	// if !isAdmin(c) {
	//     c.JSON(403, gin.H{"error": "需要管理员权限"})
	//     return
	// }

	err := h.diskCache.Clear(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": "清空缓存失败",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "缓存已清空",
	})
}
