package infra

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	gormLogger "gorm.io/gorm/logger"
)

// GormZapLogger GORM 日志适配器（输出到 Zap）
type GormZapLogger struct {
	ZapLogger                 *zap.Logger
	LogLevel                  gormLogger.LogLevel
	SlowThreshold             time.Duration
	IgnoreRecordNotFoundError bool
}

// LogMode 设置日志级别
func (l *GormZapLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 日志
func (l *GormZapLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Info {
		l.ZapLogger.Sugar().Infof(msg, data...)
	}
}

// Warn 日志
func (l *GormZapLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Warn {
		l.ZapLogger.Sugar().Warnf(msg, data...)
	}
}

// Error 日志
func (l *GormZapLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormLogger.Error {
		l.ZapLogger.Sugar().Errorf(msg, data...)
	}
}

// Trace SQL 执行日志
func (l *GormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormLogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.String("sql", sql),
		zap.Int64("rows", rows),
	}

	// 错误日志
	if err != nil && (!errors.Is(err, gormLogger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError) {
		fields = append(fields, zap.Error(err))
		l.ZapLogger.Error("SQL 执行错误", fields...)
		return
	}

	// 慢查询日志
	if l.SlowThreshold > 0 && elapsed > l.SlowThreshold {
		l.ZapLogger.Warn("SQL 慢查询", fields...)
		return
	}

	// 普通日志
	if l.LogLevel >= gormLogger.Info {
		l.ZapLogger.Debug("SQL 执行", fields...)
	}
}
