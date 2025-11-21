package converters

import (
	"backend/pkg/aiinterface"
	"fmt"

	
)

// OpenAIToGeminiConverter OpenAI -> Gemini 转换器
type OpenAIToGeminiConverter struct{}

func NewOpenAIToGeminiConverter() *OpenAIToGeminiConverter {
	return &OpenAIToGeminiConverter{}
}

func (c *OpenAIToGeminiConverter) SourceFormat() Format {
	return FormatOpenAI
}

func (c *OpenAIToGeminiConverter) TargetFormat() Format {
	return FormatGemini
}

// GeminiPart Gemini 消息部分
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiContent Gemini 内容
type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiGenerationConfig Gemini 生成配置
type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// GeminiRequest Gemini 请求格式
type GeminiRequest struct {
	Contents         []GeminiContent         `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

func (c *OpenAIToGeminiConverter) ConvertRequest(req *aiinterface.ChatCompletionRequest) (any, error) {
	geminiReq := &GeminiRequest{
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
		},
	}

	// 转换消息
	for _, msg := range req.Messages {
		role := c.convertRole(msg.Role)
		content := GeminiContent{
			Role: role,
			Parts: []GeminiPart{
				{Text: msg.Content},
			},
		}
		geminiReq.Contents = append(geminiReq.Contents, content)
	}

	return geminiReq, nil
}

func (c *OpenAIToGeminiConverter) convertRole(openaiRole string) string {
	switch openaiRole {
	case "system":
		// Gemini 不支持 system 角色，将其合并到第一个 user 消息
		return "user"
	case "assistant":
		return "model"
	case "user":
		return "user"
	default:
		return "user"
	}
}

func (c *OpenAIToGeminiConverter) ConvertResponse(resp any) (*aiinterface.ChatCompletionResponse, error) {
	// 需要根据 Gemini SDK 的实际响应结构实现
	return nil, fmt.Errorf("暂未实现 Gemini 响应转换")
}

func (c *OpenAIToGeminiConverter) ConvertStreamChunk(chunk any) (*aiinterface.StreamChunk, error) {
	return nil, fmt.Errorf("暂未实现 Gemini 流式响应转换")
}

// GeminiToOpenAIConverter Gemini -> OpenAI 转换器
type GeminiToOpenAIConverter struct{}

func NewGeminiToOpenAIConverter() *GeminiToOpenAIConverter {
	return &GeminiToOpenAIConverter{}
}

func (c *GeminiToOpenAIConverter) SourceFormat() Format {
	return FormatGemini
}

func (c *GeminiToOpenAIConverter) TargetFormat() Format {
	return FormatOpenAI
}

func (c *GeminiToOpenAIConverter) ConvertRequest(req *aiinterface.ChatCompletionRequest) (any, error) {
	return req, nil
}

func (c *GeminiToOpenAIConverter) ConvertResponse(resp any) (*aiinterface.ChatCompletionResponse, error) {
	return nil, fmt.Errorf("暂未实现")
}

func (c *GeminiToOpenAIConverter) ConvertStreamChunk(chunk any) (*aiinterface.StreamChunk, error) {
	return nil, fmt.Errorf("暂未实现")
}

