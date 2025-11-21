package logger

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.Logger

// 上下文键
type contextKey string

const traceIDKey contextKey = "trace_id"

// Init 初始化日志系统
func Init(level, format, outputPath string) error {
	// 解析日志级别
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	// 编码配置
	var encoderConfig zapcore.EncoderConfig
	if format == "json" {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// 输出配置
	var writer zapcore.WriteSyncer
	switch outputPath {
	case "stdout":
		writer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writer = zapcore.AddSync(os.Stderr)
	default:
		file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("打开日志文件失败: %w", err)
		}
		writer = zapcore.AddSync(file)
	}

	// 构建核心
	var encoder zapcore.Encoder
	if format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(encoder, writer, zapLevel)

	// 创建 Logger
	globalLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return nil
}

// Get 获取全局 Logger
func Get() *zap.Logger {
	if globalLogger == nil {
		panic("日志系统未初始化，请先调用 Init()")
	}
	return globalLogger
}

// WithTraceID 创建带 TraceID 的上下文
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID 从上下文获取 TraceID
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// WithContext 创建带上下文信息的 Logger
func WithContext(ctx context.Context) *zap.Logger {
	logger := Get()
	if traceID := GetTraceID(ctx); traceID != "" {
		logger = logger.With(zap.String("trace_id", traceID))
	}
	return logger
}

// Debug 便捷方法
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Info 便捷方法
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Warn 便捷方法
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error 便捷方法
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal 便捷方法
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// Sync 刷新日志缓冲区
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}
