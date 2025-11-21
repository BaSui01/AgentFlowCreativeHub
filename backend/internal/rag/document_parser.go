package rag

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// DocumentParser 文档解析器接口
type DocumentParser interface {
	Parse(content []byte) (*ParsedDocument, error)
	SupportedTypes() []string
}

// ParsedDocument 解析后的文档
type ParsedDocument struct {
	Content     string            // 文本内容
	Title       string            // 标题
	Metadata    map[string]string // 元数据
	CharCount   int               // 字符数
	WordCount   int               // 词数
}

// ParserRegistry 解析器注册表
type ParserRegistry struct {
	parsers map[string]DocumentParser
}

// NewParserRegistry 创建解析器注册表
func NewParserRegistry() *ParserRegistry {
	registry := &ParserRegistry{
		parsers: make(map[string]DocumentParser),
	}

	// 注册默认解析器
	registry.Register("text/plain", NewTextParser())
	registry.Register("text/markdown", NewMarkdownParser())
	registry.Register("text/html", NewHTMLParser())

	return registry
}

// Register 注册解析器
func (r *ParserRegistry) Register(contentType string, parser DocumentParser) {
	r.parsers[contentType] = parser
}

// GetParser 获取解析器
func (r *ParserRegistry) GetParser(contentType string) (DocumentParser, error) {
	parser, exists := r.parsers[contentType]
	if !exists {
		return nil, fmt.Errorf("不支持的内容类型: %s", contentType)
	}
	return parser, nil
}

// Parse 解析文档
func (r *ParserRegistry) Parse(contentType string, content []byte) (*ParsedDocument, error) {
	parser, err := r.GetParser(contentType)
	if err != nil {
		return nil, err
	}
	return parser.Parse(content)
}

// TextParser 纯文本解析器
type TextParser struct{}

// NewTextParser 创建纯文本解析器
func NewTextParser() *TextParser {
	return &TextParser{}
}

// Parse 解析纯文本
func (p *TextParser) Parse(content []byte) (*ParsedDocument, error) {
	text := string(content)

	// 确保 UTF-8 编码
	if !utf8.Valid(content) {
		return nil, fmt.Errorf("无效的 UTF-8 编码")
	}

	// 提取标题（第一行非空行）
	title := extractTitle(text)

	// 统计字数
	charCount := utf8.RuneCountInString(text)
	wordCount := countWords(text)

	return &ParsedDocument{
		Content:   text,
		Title:     title,
		Metadata:  make(map[string]string),
		CharCount: charCount,
		WordCount: wordCount,
	}, nil
}

// SupportedTypes 支持的类型
func (p *TextParser) SupportedTypes() []string {
	return []string{"text/plain", "text/txt"}
}

// MarkdownParser Markdown 解析器
type MarkdownParser struct{}

// NewMarkdownParser 创建 Markdown 解析器
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{}
}

// Parse 解析 Markdown
func (p *MarkdownParser) Parse(content []byte) (*ParsedDocument, error) {
	text := string(content)

	// 提取标题（寻找第一个 # 标题）
	title := extractMarkdownTitle(text)

	// 简化 Markdown 语法（移除部分标记）
	cleaned := cleanMarkdown(text)

	charCount := utf8.RuneCountInString(cleaned)
	wordCount := countWords(cleaned)

	return &ParsedDocument{
		Content:   cleaned,
		Title:     title,
		Metadata:  map[string]string{"format": "markdown"},
		CharCount: charCount,
		WordCount: wordCount,
	}, nil
}

// SupportedTypes 支持的类型
func (p *MarkdownParser) SupportedTypes() []string {
	return []string{"text/markdown", "text/x-markdown"}
}

// HTMLParser HTML 解析器（简化版）
type HTMLParser struct{}

// NewHTMLParser 创建 HTML 解析器
func NewHTMLParser() *HTMLParser {
	return &HTMLParser{}
}

// Parse 解析 HTML
func (p *HTMLParser) Parse(content []byte) (*ParsedDocument, error) {
	text := string(content)

	// 提取标题（<title> 或 <h1>）
	title := extractHTMLTitle(text)

	// 移除 HTML 标签（简化实现）
	cleaned := stripHTMLTags(text)

	charCount := utf8.RuneCountInString(cleaned)
	wordCount := countWords(cleaned)

	return &ParsedDocument{
		Content:   cleaned,
		Title:     title,
		Metadata:  map[string]string{"format": "html"},
		CharCount: charCount,
		WordCount: wordCount,
	}, nil
}

// SupportedTypes 支持的类型
func (p *HTMLParser) SupportedTypes() []string {
	return []string{"text/html", "application/xhtml+xml"}
}

// 辅助函数

// extractTitle 提取标题（第一行非空行）
func extractTitle(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 100 {
				return line[:100] + "..."
			}
			return line
		}
	}
	return "Untitled"
}

// extractMarkdownTitle 提取 Markdown 标题
func extractMarkdownTitle(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimPrefix(line, "# ")
			if len(title) > 100 {
				return title[:100] + "..."
			}
			return title
		}
	}
	return extractTitle(text)
}

// extractHTMLTitle 提取 HTML 标题
func extractHTMLTitle(text string) string {
	// 简化实现：查找 <title> 或 <h1>
	if start := strings.Index(text, "<title>"); start != -1 {
		start += 7
		if end := strings.Index(text[start:], "</title>"); end != -1 {
			title := text[start : start+end]
			return strings.TrimSpace(title)
		}
	}

	if start := strings.Index(text, "<h1>"); start != -1 {
		start += 4
		if end := strings.Index(text[start:], "</h1>"); end != -1 {
			title := text[start : start+end]
			return strings.TrimSpace(stripHTMLTags(title))
		}
	}

	return "Untitled"
}

// cleanMarkdown 清理 Markdown 语法
func cleanMarkdown(text string) string {
	var buf bytes.Buffer
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		// 移除 Markdown 标记但保留内容
		line = strings.TrimPrefix(line, "# ")
		line = strings.TrimPrefix(line, "## ")
		line = strings.TrimPrefix(line, "### ")
		line = strings.TrimPrefix(line, "#### ")
		line = strings.TrimPrefix(line, "##### ")
		line = strings.TrimPrefix(line, "###### ")

		// 移除列表标记
		line = strings.TrimPrefix(strings.TrimSpace(line), "- ")
		line = strings.TrimPrefix(strings.TrimSpace(line), "* ")
		line = strings.TrimPrefix(strings.TrimSpace(line), "+ ")

		// 移除代码块标记
		if !strings.HasPrefix(line, "```") {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// stripHTMLTags 移除 HTML 标签（简化实现）
func stripHTMLTags(text string) string {
	var buf bytes.Buffer
	inTag := false

	for _, r := range text {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			buf.WriteRune(' ') // 标签后添加空格
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}

	// 清理多余空白
	result := buf.String()
	result = strings.Join(strings.Fields(result), " ")
	return result
}

// countWords 统计词数（中英文混合）
func countWords(text string) int {
	// 简化实现：按空格分割 + 统计中文字符
	words := strings.Fields(text)
	count := len(words)

	// 统计中文字符（每个中文字符算一个词）
	for _, r := range text {
		if isCJK(r) {
			count++
		}
	}

	return count
}

// isCJK 判断是否为中日韩字符
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Ideographs Extension A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK Unified Ideographs Extension B
		(r >= 0x2A700 && r <= 0x2B73F) || // CJK Unified Ideographs Extension C
		(r >= 0x2B740 && r <= 0x2B81F) || // CJK Unified Ideographs Extension D
		(r >= 0x2B820 && r <= 0x2CEAF) || // CJK Unified Ideographs Extension E
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0x2F800 && r <= 0x2FA1F) // CJK Compatibility Ideographs Supplement
}

// ReadAll 读取所有内容（辅助函数）
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
