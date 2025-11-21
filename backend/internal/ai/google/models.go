package google

// GeminiPart Gemini 消息部分
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiContent Gemini 内容
type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiCandidate Gemini 候选结果
type GeminiCandidate struct {
	Content GeminiContent `json:"content"`
}

// GeminiUsageMetadata Gemini 使用统计
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GeminiResponse Gemini API 响应
type GeminiResponse struct {
	ID            string              `json:"id"`
	Candidates    []GeminiCandidate   `json:"candidates"`
	UsageMetadata GeminiUsageMetadata `json:"usageMetadata"`
}

// GeminiStreamChunk Gemini 流式响应块
type GeminiStreamChunk struct {
	ID         string            `json:"id"`
	Candidates []GeminiCandidate `json:"candidates"`
}

// 预置 Gemini 模型列表
var DefaultGeminiModels = []GeminiModelInfo{
	{
		ID:              "gemini-2.0-flash-exp",
		Name:            "Gemini 2.0 Flash (Experimental)",
		MaxTokens:       8192,
		ContextWindow:   1000000,
		SupportVision:   true,
		SupportStreaming: true,
	},
	{
		ID:              "gemini-1.5-pro",
		Name:            "Gemini 1.5 Pro",
		MaxTokens:       8192,
		ContextWindow:   2000000,
		SupportVision:   true,
		SupportStreaming: true,
	},
	{
		ID:              "gemini-1.5-flash",
		Name:            "Gemini 1.5 Flash",
		MaxTokens:       8192,
		ContextWindow:   1000000,
		SupportVision:   true,
		SupportStreaming: true,
	},
	{
		ID:              "gemini-1.0-pro",
		Name:            "Gemini 1.0 Pro",
		MaxTokens:       2048,
		ContextWindow:   32768,
		SupportVision:   false,
		SupportStreaming: true,
	},
	{
		ID:              "gemini-embedding-001",
		Name:            "Gemini Embedding",
		MaxTokens:       0,
		ContextWindow:   2048,
		SupportVision:   false,
		SupportStreaming: false,
	},
}

// GeminiModelInfo Gemini 模型信息
type GeminiModelInfo struct {
	ID               string
	Name             string
	MaxTokens        int
	ContextWindow    int
	SupportVision    bool
	SupportStreaming bool
}
