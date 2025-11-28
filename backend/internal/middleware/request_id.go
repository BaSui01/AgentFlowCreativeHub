package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 上下文键
type contextKey string

const (
	// RequestIDKey 请求 ID 上下文键
	RequestIDKey contextKey = "request_id"
	// TraceIDKey 追踪 ID 上下文键
	TraceIDKey contextKey = "trace_id"
	// SpanIDKey Span ID 上下文键
	SpanIDKey contextKey = "span_id"
)

// HTTP 头常量
const (
	HeaderRequestID  = "X-Request-ID"
	HeaderTraceID    = "X-Trace-ID"
	HeaderSpanID     = "X-Span-ID"
	HeaderParentSpan = "X-Parent-Span-ID"
)

// RequestIDMiddleware 请求 ID 中间件
// 为每个请求生成唯一的请求 ID，支持分布式追踪
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从请求头获取 Request ID（支持上游传递）
		requestID := c.GetHeader(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		
		// 尝试从请求头获取 Trace ID
		traceID := c.GetHeader(HeaderTraceID)
		if traceID == "" {
			traceID = requestID // 如果没有 Trace ID，使用 Request ID
		}
		
		// 生成 Span ID
		spanID := uuid.New().String()[:8]
		
		// 获取 Parent Span ID
		parentSpanID := c.GetHeader(HeaderParentSpan)
		
		// 设置到 Gin 上下文
		c.Set(string(RequestIDKey), requestID)
		c.Set(string(TraceIDKey), traceID)
		c.Set(string(SpanIDKey), spanID)
		if parentSpanID != "" {
			c.Set("parent_span_id", parentSpanID)
		}
		
		// 注入到 context.Context
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
		ctx = context.WithValue(ctx, TraceIDKey, traceID)
		ctx = context.WithValue(ctx, SpanIDKey, spanID)
		c.Request = c.Request.WithContext(ctx)
		
		// 设置响应头
		c.Header(HeaderRequestID, requestID)
		c.Header(HeaderTraceID, traceID)
		c.Header(HeaderSpanID, spanID)
		
		c.Next()
	}
}

// GetRequestID 从上下文获取请求 ID
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(RequestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// GetTraceID 从上下文获取追踪 ID
func GetTraceID(ctx context.Context) string {
	if v := ctx.Value(TraceIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// GetSpanID 从上下文获取 Span ID
func GetSpanID(ctx context.Context) string {
	if v := ctx.Value(SpanIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// GetRequestIDFromGin 从 Gin 上下文获取请求 ID
func GetRequestIDFromGin(c *gin.Context) string {
	if id, exists := c.Get(string(RequestIDKey)); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// GetTraceIDFromGin 从 Gin 上下文获取追踪 ID
func GetTraceIDFromGin(c *gin.Context) string {
	if id, exists := c.Get(string(TraceIDKey)); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// TraceContext 追踪上下文
type TraceContext struct {
	RequestID    string `json:"request_id"`
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
}

// GetTraceContext 获取完整追踪上下文
func GetTraceContext(c *gin.Context) *TraceContext {
	tc := &TraceContext{
		RequestID: GetRequestIDFromGin(c),
		TraceID:   GetTraceIDFromGin(c),
	}
	
	if spanID, exists := c.Get(string(SpanIDKey)); exists {
		tc.SpanID = spanID.(string)
	}
	
	if parentSpanID, exists := c.Get("parent_span_id"); exists {
		tc.ParentSpanID = parentSpanID.(string)
	}
	
	return tc
}

// WithTraceContext 将追踪上下文注入到新的 context
func WithTraceContext(ctx context.Context, tc *TraceContext) context.Context {
	ctx = context.WithValue(ctx, RequestIDKey, tc.RequestID)
	ctx = context.WithValue(ctx, TraceIDKey, tc.TraceID)
	ctx = context.WithValue(ctx, SpanIDKey, tc.SpanID)
	return ctx
}
