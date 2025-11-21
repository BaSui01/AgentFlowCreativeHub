package builtin

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"backend/internal/tools"
)

// TextStatisticsTool 文本统计工具
type TextStatisticsTool struct{}

// NewTextStatisticsTool 创建文本统计工具
func NewTextStatisticsTool() *TextStatisticsTool {
	return &TextStatisticsTool{}
}

// Execute 执行文本统计
func (t *TextStatisticsTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 提取文本
	text, ok := input["text"].(string)
	if !ok || text == "" {
		return nil, fmt.Errorf("text 参数类型错误或为空")
	}
	
	// 提取语言参数
	language, _ := input["language"].(string)
	if language == "" {
		language = "auto"
	}
	
	// 自动检测语言
	if language == "auto" {
		language = t.detectLanguage(text)
	}
	
	// 统计字符数
	totalChars := len([]rune(text))
	
	// 统计不含空白字符的字符数（移除所有空白字符）
	noSpaces := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1 // 删除字符
		}
		return r
	}, text)
	charsNoSpaces := len([]rune(noSpaces))
	
	// 统计中文字符
	chineseChars := t.countChineseCharacters(text)
	
	// 统计英文单词
	englishWords := t.countEnglishWords(text)
	
	// 统计段落数
	paragraphs := t.countParagraphs(text)
	
	// 统计句子数
	sentences := t.countSentences(text)
	
	// 计算平均句长
	avgSentenceLength := 0.0
	if sentences > 0 {
		if language == "zh" {
			avgSentenceLength = float64(chineseChars) / float64(sentences)
		} else {
			avgSentenceLength = float64(englishWords) / float64(sentences)
		}
	}
	
	// 估算阅读时间（分钟）
	readingTime := t.estimateReadingTime(chineseChars, englishWords, language)
	
	return map[string]any{
		"total_characters":      totalChars,
		"characters_no_spaces":  charsNoSpaces,
		"chinese_characters":    chineseChars,
		"english_words":         englishWords,
		"paragraphs":            paragraphs,
		"sentences":             sentences,
		"avg_sentence_length":   fmt.Sprintf("%.1f", avgSentenceLength),
		"reading_time_minutes":  fmt.Sprintf("%.1f", readingTime),
		"detected_language":     language,
	}, nil
}

// detectLanguage 自动检测语言
func (t *TextStatisticsTool) detectLanguage(text string) string {
	chineseCount := 0
	englishCount := 0
	
	for _, r := range text {
		if t.isChinese(r) {
			chineseCount++
		} else if unicode.IsLetter(r) {
			englishCount++
		}
	}
	
	if chineseCount > englishCount {
		return "zh"
	}
	return "en"
}

// isChinese 判断是否为中文字符（扩展范围）
func (t *TextStatisticsTool) isChinese(r rune) bool {
	// CJK 统一表意文字及扩展区
	return (r >= 0x4e00 && r <= 0x9fff) || // CJK 统一表意文字
		(r >= 0x3400 && r <= 0x4dbf) || // CJK 扩展 A
		(r >= 0x20000 && r <= 0x2a6df) // CJK 扩展 B
}

// countChineseCharacters 统计中文字符数
func (t *TextStatisticsTool) countChineseCharacters(text string) int {
	count := 0
	for _, r := range text {
		if t.isChinese(r) {
			count++
		}
	}
	return count
}

// countEnglishWords 统计英文单词数
func (t *TextStatisticsTool) countEnglishWords(text string) int {
	// 移除中文字符
	re := regexp.MustCompile(`[\x{4e00}-\x{9fa5}]`)
	englishText := re.ReplaceAllString(text, "")
	
	// 按空格和标点分割
	words := strings.Fields(englishText)
	
	// 过滤纯标点
	validWords := 0
	for _, word := range words {
		hasLetter := false
		for _, r := range word {
			if unicode.IsLetter(r) {
				hasLetter = true
				break
			}
		}
		if hasLetter {
			validWords++
		}
	}
	
	return validWords
}

// countParagraphs 统计段落数
func (t *TextStatisticsTool) countParagraphs(text string) int {
	// 按双换行符分割
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	
	// 统一换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	
	// 按空行分割
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
	
	// 过滤空段落
	count := 0
	for _, p := range paragraphs {
		if strings.TrimSpace(p) != "" {
			count++
		}
	}
	
	return count
}

// countSentences 统计句子数
func (t *TextStatisticsTool) countSentences(text string) int {
	// 中文句子结束符：。！？
	// 英文句子结束符：.!?
	re := regexp.MustCompile(`[。！？.!?]+`)
	matches := re.FindAllString(text, -1)
	return len(matches)
}

// estimateReadingTime 估算阅读时间（分钟）
func (t *TextStatisticsTool) estimateReadingTime(chineseChars, englishWords int, language string) float64 {
	// 中文阅读速度：300 字/分钟
	// 英文阅读速度：200 词/分钟
	
	chineseTime := float64(chineseChars) / 300.0
	englishTime := float64(englishWords) / 200.0
	
	totalTime := chineseTime + englishTime
	
	// 至少 0.1 分钟
	if totalTime < 0.1 {
		totalTime = 0.1
	}
	
	return totalTime
}

// Validate 验证输入
func (t *TextStatisticsTool) Validate(input map[string]any) error {
	text, ok := input["text"].(string)
	if !ok {
		return fmt.Errorf("缺少必需参数: text")
	}
	
	if text == "" {
		return fmt.Errorf("text 参数不能为空")
	}
	
	// 验证语言参数
	if language, ok := input["language"].(string); ok {
		if language != "" && language != "zh" && language != "en" && language != "auto" {
			return fmt.Errorf("language 参数必须是 zh、en 或 auto")
		}
	}
	
	return nil
}

// GetDefinition 获取工具定义
func (t *TextStatisticsTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "text_statistics",
		DisplayName: "文本统计",
		Description: "统计文本的字数、字符数、段落数、句子数、平均句长，并估算阅读时间。支持中英文混合文本。",
		Category:    "text_processing",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "待统计的文本内容",
				},
				"language": map[string]any{
					"type":        "string",
					"enum":        []string{"zh", "en", "auto"},
					"default":     "auto",
					"description": "文本语言（zh=中文, en=英文, auto=自动检测）",
				},
			},
			"required": []string{"text"},
		},
		Timeout:     5,
		Status:      "active",
		RequireAuth: false,
	}
}
