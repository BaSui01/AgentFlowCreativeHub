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

var globalRedis redis.UniversalClient

// InitRedis 初始化 Redis 连接
// 支持三种模式: standalone(单节点), sentinel(哨兵), cluster(集群)
func InitRedis(cfg *config.RedisConfig) (redis.UniversalClient, error) {
	var rdb redis.UniversalClient

	mode := cfg.Mode
	if mode == "" {
		mode = "standalone"
	}

	switch mode {
	case "standalone":
		rdb = redis.NewClient(&redis.Options{
			Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password:     cfg.Password,
			DB:           cfg.DB,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
		})
		logger.Info("Redis 单节点模式初始化",
			zap.String("host", cfg.Host),
			zap.Int("port", cfg.Port),
			zap.Int("db", cfg.DB),
		)

	case "sentinel":
		if cfg.MasterName == "" || len(cfg.SentinelAddrs) == 0 {
			return nil, fmt.Errorf("哨兵模式需要配置 master_name 和 sentinel_addrs")
		}
		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       cfg.MasterName,
			SentinelAddrs:    cfg.SentinelAddrs,
			SentinelPassword: cfg.SentinelPassword,
			Password:         cfg.Password,
			DB:               cfg.DB,
			PoolSize:         cfg.PoolSize,
			MinIdleConns:     cfg.MinIdleConns,
		})
		logger.Info("Redis 哨兵模式初始化",
			zap.String("master", cfg.MasterName),
			zap.Strings("sentinels", cfg.SentinelAddrs),
			zap.Int("db", cfg.DB),
		)

	case "cluster":
		if len(cfg.ClusterAddrs) == 0 {
			return nil, fmt.Errorf("集群模式需要配置 cluster_addrs")
		}
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        cfg.ClusterAddrs,
			Password:     cfg.Password,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
		})
		logger.Info("Redis 集群模式初始化",
			zap.Strings("addrs", cfg.ClusterAddrs),
		)

	default:
		return nil, fmt.Errorf("不支持的 Redis 模式: %s (可选: standalone, sentinel, cluster)", mode)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	logger.Info("Redis 连接成功", zap.String("mode", mode))

	globalRedis = rdb
	return rdb, nil
}

// GetRedis 获取全局 Redis 客户端
func GetRedis() redis.UniversalClient {
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
