package api

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"backend/internal/config"
	"backend/internal/rag"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// ReadinessResponse 就绪检查响应
type ReadinessResponse struct {
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
	Database string `json:"database,omitempty"`
}

// HealthCheck 健康检查
// @Summary 服务健康检查
// @Description 返回基础健康状态，可供监控探针使用
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func HealthCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "AgentFlowCreativeHub",
		})
	}
}

// ReadinessCheck 就绪检查
// @Summary 服务就绪检查
// @Description 包含数据库连通性结果，用于判断可接收请求
// @Tags System
// @Produce json
// @Success 200 {object} ReadinessResponse
// @Failure 503 {object} ReadinessResponse
// @Router /ready [get]
func ReadinessCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{
				"status": "not_ready",
				"reason": "database connection error",
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{
				"status": "not_ready",
				"reason": "database ping failed",
			})
			return
		}

		c.JSON(200, gin.H{
			"status":   "ready",
			"database": "connected",
		})
	}
}

// --- 环境变量辅助函数 ---

// getEnvList 读取逗号分隔的环境变量列表
func getEnvList(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var res []string
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			res = append(res, v)
		}
	}
	return res
}

// stringInSlice 判断字符串是否存在于切片中
func stringInSlice(target string, list []string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// defaultIfEmpty 返回非空列表或默认值
func defaultIfEmpty(list []string, def []string) []string {
	if len(list) == 0 {
		return def
	}
	return list
}

// --- Redis 配置辅助函数 ---

// normalizeRedisConfig 归一化 Redis 配置
func normalizeRedisConfig(cfg config.RedisConfig) config.RedisConfig {
	resolved := cfg
	resolved.Host = strings.TrimSpace(resolved.Host)

	if resolved.Host == "" {
		if addr := strings.TrimSpace(os.Getenv("REDIS_ADDR")); addr != "" {
			host, port := parseRedisAddr(addr)
			if host != "" {
				resolved.Host = host
			}
			if resolved.Port == 0 && port > 0 {
				resolved.Port = port
			}
		}
	}

	if resolved.Host == "" {
		resolved.Host = "localhost"
	}
	if resolved.Port == 0 {
		resolved.Port = 6379
	}

	return resolved
}

// parseRedisAddr 解析 Redis 地址
func parseRedisAddr(addr string) (string, int) {
	if strings.TrimSpace(addr) == "" {
		return "", 0
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return strings.TrimSpace(addr), 0
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, 0
	}

	return host, port
}

// --- 向量存储初始化 ---

// initVectorStore 初始化向量存储
func initVectorStore(cfg *config.Config, db *gorm.DB) (rag.VectorStore, error) {
	if cfg != nil {
		vsType := strings.ToLower(strings.TrimSpace(cfg.RAG.VectorStore.Type))

		if vsType == "qdrant" {
			qcfg := cfg.RAG.VectorStore.Qdrant
			if strings.TrimSpace(qcfg.Endpoint) == "" {
				return nil, fmt.Errorf("未配置 Qdrant endpoint")
			}
			opts := rag.QdrantOptions{
				Endpoint:        qcfg.Endpoint,
				APIKey:          qcfg.APIKey,
				Collection:      qcfg.Collection,
				VectorDimension: qcfg.VectorDimension,
				Distance:        qcfg.Distance,
				TimeoutSeconds:  qcfg.TimeoutSeconds,
			}
			return rag.NewQdrantStore(opts)
		}
	}

	return rag.NewPGVectorStore(db)
}
