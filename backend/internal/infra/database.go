package infra

import (
	"fmt"
	"time"

	"backend/internal/config"
	"backend/internal/logger"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

var globalDB *gorm.DB

// InitDatabase 初始化数据库连接
func InitDatabase(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	// GORM 日志配置
	var logLevel gormLogger.LogLevel
	switch cfg.SSLMode {
	case "disable":
		logLevel = gormLogger.Info
	default:
		logLevel = gormLogger.Warn
	}

	// 自定义 GORM 日志适配器
	gormLog := &GormZapLogger{
		ZapLogger:                 logger.Get(),
		LogLevel:                  logLevel,
		SlowThreshold:             200 * time.Millisecond,
		IgnoreRecordNotFoundError: true,
	}

	// 打开数据库连接
	db, err := gorm.Open(postgres.Open(cfg.GetDSN()), &gorm.Config{
		Logger: gormLog,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 获取底层 *sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 SQL DB 失败: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	logger.Info("数据库连接成功",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.DBName),
	)

	globalDB = db
	return db, nil
}

// GetDB 获取全局数据库实例
func GetDB() *gorm.DB {
	if globalDB == nil {
		panic("数据库未初始化，请先调用 InitDatabase()")
	}
	return globalDB
}

// AutoMigrate 执行自动迁移
func AutoMigrate(db *gorm.DB, models ...interface{}) error {
	logger.Info("开始执行数据库自动迁移")
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	logger.Info("数据库迁移完成")
	return nil
}

// CloseDatabase 关闭数据库连接
func CloseDatabase() error {
	if globalDB != nil {
		sqlDB, err := globalDB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// HealthCheck 数据库健康检查
func HealthCheck() error {
	if globalDB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	sqlDB, err := globalDB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}
