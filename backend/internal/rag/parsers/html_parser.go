package parsers

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// HTMLParser HTML 文档解析器
type HTMLParser struct{}

// NewHTMLParser 创建 HTML 解析器
func NewHTMLParser() *HTMLParser {
	return &HTMLParser{}
}

// Parse 解析 HTML 文档
func (p *HTMLParser) Parse(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	html := string(data)

	// 提取正文内容
	content := p.extractMainContent(html)

	// 清理并返回纯文本
	return p.cleanText(content), nil
}

// SupportedExtensions 支持的扩展名
func (p *HTMLParser) SupportedExtensions() []string {
	return []string{".html", ".htm"}
}

// CanParse 检查是否支持该扩展名
func (p *HTMLParser) CanParse(ext string) bool {
	for _, e := range p.SupportedExtensions() {
		if e == ext {
			return true
		}
	}
	return false
}

// extractMainContent 提取主要内容
func (p *HTMLParser) extractMainContent(html string) string {
	// 尝试提取 <main> 标签
	if content := p.extractTag(html, "main"); content != "" {
		return content
	}

	// 尝试提取 <article> 标签
	if content := p.extractTag(html, "article"); content != "" {
		return content
	}

	// 尝试提取 <body> 标签
	if content := p.extractTag(html, "body"); content != "" {
		return content
	}

	return html
}

// extractTag 提取指定标签内容
func (p *HTMLParser) extractTag(html, tag string) string {
	pattern := `(?is)<` + tag + `[^>]*>(.*?)</` + tag + `>`
	regex := regexp.MustCompile(pattern)
	if match := regex.FindStringSubmatch(html); len(match) > 1 {
		return match[1]
	}
	return ""
}

// cleanText 清理 HTML 为纯文本
func (p *HTMLParser) cleanText(html string) string {
	// 移除 script 标签
	scriptRegex := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRegex.ReplaceAllString(html, "")

	// 移除 style 标签
	styleRegex := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRegex.ReplaceAllString(html, "")

	// 移除 nav, header, footer, aside 等非主要内容
	navRegex := regexp.MustCompile(`(?is)<(nav|header|footer|aside|noscript)[^>]*>.*?</\1>`)
	html = navRegex.ReplaceAllString(html, "")

	// 移除 HTML 注释
	commentRegex := regexp.MustCompile(`<!--.*?-->`)
	html = commentRegex.ReplaceAllString(html, "")

	// 将块级元素转换为换行
	blockRegex := regexp.MustCompile(`(?i)</(p|div|section|h[1-6]|li|tr|br|hr)[^>]*>`)
	html = blockRegex.ReplaceAllString(html, "\n")

	// 移除所有 HTML 标签
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text := tagRegex.ReplaceAllString(html, " ")

	// 解码 HTML 实体
	text = p.decodeEntities(text)

	// 清理多余空白
	spaceRegex := regexp.MustCompile(`[ \t]+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	// 清理多余换行
	newlineRegex := regexp.MustCompile(`\n\s*\n+`)
	text = newlineRegex.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}

// decodeEntities 解码 HTML 实体
func (p *HTMLParser) decodeEntities(s string) string {
	entities := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   `"`,
		"&apos;":   "'",
		"&nbsp;":   " ",
		"&copy;":   "(c)",
		"&reg;":    "(R)",
		"&trade;":  "(TM)",
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&hellip;": "...",
		"&laquo;":  "<<",
		"&raquo;":  ">>",
		"&ldquo;":  "\"",
		"&rdquo;":  "\"",
		"&lsquo;":  "'",
		"&rsquo;":  "'",
	}

	for entity, char := range entities {
		s = strings.ReplaceAll(s, entity, char)
	}

	// 解码数字实体
	numRegex := regexp.MustCompile(`&#(\d+);`)
	s = numRegex.ReplaceAllStringFunc(s, func(match string) string {
		var num int
		if _, err := fmt.Sscanf(match, "&#%d;", &num); err == nil && num > 0 && num < 65536 {
			return string(rune(num))
		}
		return match
	})

	// 解码十六进制实体
	hexRegex := regexp.MustCompile(`&#x([0-9a-fA-F]+);`)
	s = hexRegex.ReplaceAllStringFunc(s, func(match string) string {
		var num int
		if _, err := fmt.Sscanf(match, "&#x%x;", &num); err == nil && num > 0 && num < 65536 {
			return string(rune(num))
		}
		return match
	})

	return s
}
