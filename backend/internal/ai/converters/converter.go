package converters

import (
	"backend/pkg/aiinterface"
	"fmt"

	
)

// Format 定义 API 格式类型
type Format string

const (
	FormatOpenAI   Format = "openai"
	FormatClaude   Format = "claude"
	FormatGemini   Format = "gemini"
	FormatDeepSeek Format = "deepseek"
	FormatAzure    Format = "azure"
)

// FormatConverter 格式转换器接口
type FormatConverter interface {
	// ConvertRequest 转换请求格式
	ConvertRequest(req *aiinterface.ChatCompletionRequest) (any, error)

	// ConvertResponse 转换响应格式
	ConvertResponse(resp any) (*aiinterface.ChatCompletionResponse, error)

	// ConvertStreamChunk 转换流式响应块
	ConvertStreamChunk(chunk any) (*aiinterface.StreamChunk, error)

	// SourceFormat 源格式
	SourceFormat() Format

	// TargetFormat 目标格式
	TargetFormat() Format
}

// ConverterRegistry 转换器注册表
type ConverterRegistry struct {
	converters map[string]FormatConverter
}

// NewConverterRegistry 创建转换器注册表
func NewConverterRegistry() *ConverterRegistry {
	registry := &ConverterRegistry{
		converters: make(map[string]FormatConverter),
	}

	// 注册内置转换器
	registry.Register(NewOpenAIToClaudeConverter())
	registry.Register(NewClaudeToOpenAIConverter())
	registry.Register(NewOpenAIToGeminiConverter())
	registry.Register(NewGeminiToOpenAIConverter())

	return registry
}

// Register 注册转换器
func (r *ConverterRegistry) Register(converter FormatConverter) {
	key := fmt.Sprintf("%s->%s", converter.SourceFormat(), converter.TargetFormat())
	r.converters[key] = converter
}

// GetConverter 获取转换器
func (r *ConverterRegistry) GetConverter(from, to Format) (FormatConverter, error) {
	key := fmt.Sprintf("%s->%s", from, to)
	converter, ok := r.converters[key]
	if !ok {
		return nil, fmt.Errorf("不支持的格式转换: %s -> %s", from, to)
	}
	return converter, nil
}

// Convert 执行格式转换
func (r *ConverterRegistry) Convert(from, to Format, req *aiinterface.ChatCompletionRequest) (any, error) {
	// 如果格式相同，直接返回
	if from == to {
		return req, nil
	}

	converter, err := r.GetConverter(from, to)
	if err != nil {
		return nil, err
	}

	return converter.ConvertRequest(req)
}

// 全局转换器注册表
var defaultRegistry = NewConverterRegistry()

// GetDefaultRegistry 获取默认转换器注册表
func GetDefaultRegistry() *ConverterRegistry {
	return defaultRegistry
}

