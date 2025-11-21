package infra

import (
	"context"
	"fmt"
	"time"

	"backend/internal/config"
	"backend/internal/logger"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var globalRedis *redis.Client

// InitRedis 初始化 Redis 连接
func InitRedis(cfg *config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	logger.Info("Redis 连接成功",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.Int("db", cfg.DB),
	)

	globalRedis = rdb
	return rdb, nil
}

// GetRedis 获取全局 Redis 客户端
func GetRedis() *redis.Client {
	if globalRedis == nil {
		panic("Redis 未初始化，请先调用 InitRedis()")
	}
	return globalRedis
}

// CloseRedis 关闭 Redis 连接
func CloseRedis() error {
	if globalRedis != nil {
		return globalRedis.Close()
	}
	return nil
}

// HealthCheckRedis Redis 健康检查
func HealthCheckRedis() error {
	if globalRedis == nil {
		return fmt.Errorf("Redis 未初始化")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return globalRedis.Ping(ctx).Err()
}
