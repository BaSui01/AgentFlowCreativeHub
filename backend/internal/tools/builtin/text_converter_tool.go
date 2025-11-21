package builtin

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"backend/internal/tools"
)

// TextConverterTool 文本格式转换工具
type TextConverterTool struct{}

// NewTextConverterTool 创建文本格式转换工具
func NewTextConverterTool() *TextConverterTool {
	return &TextConverterTool{}
}

// Execute 执行格式转换
func (t *TextConverterTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 提取参数
	text, ok := input["text"].(string)
	if !ok || text == "" {
		return nil, fmt.Errorf("text 参数类型错误或为空")
	}
	
	fromFormat, ok := input["from_format"].(string)
	if !ok || fromFormat == "" {
		return nil, fmt.Errorf("from_format 参数类型错误或为空")
	}
	
	toFormat, ok := input["to_format"].(string)
	if !ok || toFormat == "" {
		return nil, fmt.Errorf("to_format 参数类型错误或为空")
	}
	
	// 提取选项
	preserveLinks := true
	preserveImages := true
	if options, ok := input["options"].(map[string]any); ok {
		if val, ok := options["preserve_links"].(bool); ok {
			preserveLinks = val
		}
		if val, ok := options["preserve_images"].(bool); ok {
			preserveImages = val
		}
	}
	
	// 执行转换
	var converted string
	var err error
	
	switch fromFormat + "_to_" + toFormat {
	case "markdown_to_html":
		converted = t.markdownToHTML(text, preserveLinks, preserveImages)
	case "html_to_markdown":
		converted = t.htmlToMarkdown(text, preserveLinks, preserveImages)
	case "markdown_to_plain":
		converted = t.markdownToPlain(text)
	case "html_to_plain":
		converted = t.htmlToPlain(text)
	case "plain_to_markdown":
		converted = t.plainToMarkdown(text)
	case "plain_to_html":
		converted = t.plainToHTML(text)
	default:
		return nil, fmt.Errorf("不支持的转换: %s → %s", fromFormat, toFormat)
	}
	
	if err != nil {
		return nil, fmt.Errorf("转换失败: %w", err)
	}
	
	return map[string]any{
		"converted_text": converted,
		"from_format":    fromFormat,
		"to_format":      toFormat,
		"success":        true,
	}, nil
}

// markdownToHTML 将 Markdown 转换为 HTML（简化版）
func (t *TextConverterTool) markdownToHTML(text string, preserveLinks, preserveImages bool) string {
	html := text
	
	// 标题转换
	html = regexp.MustCompile(`(?m)^### (.*?)$`).ReplaceAllString(html, "<h3>$1</h3>")
	html = regexp.MustCompile(`(?m)^## (.*?)$`).ReplaceAllString(html, "<h2>$1</h2>")
	html = regexp.MustCompile(`(?m)^# (.*?)$`).ReplaceAllString(html, "<h1>$1</h1>")
	
	// 粗体和斜体
	html = regexp.MustCompile(`\*\*\*(.*?)\*\*\*`).ReplaceAllString(html, "<strong><em>$1</em></strong>")
	html = regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(html, "<strong>$1</strong>")
	html = regexp.MustCompile(`\*(.*?)\*`).ReplaceAllString(html, "<em>$1</em>")
	
	// 代码
	html = regexp.MustCompile("```([\\s\\S]*?)```").ReplaceAllString(html, "<pre><code>$1</code></pre>")
	html = regexp.MustCompile("`(.*?)`").ReplaceAllString(html, "<code>$1</code>")
	
	// 链接
	if preserveLinks {
		html = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`).ReplaceAllString(html, `<a href="$2">$1</a>`)
	} else {
		html = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`).ReplaceAllString(html, "$1")
	}
	
	// 图片
	if preserveImages {
		html = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`).ReplaceAllString(html, `<img src="$2" alt="$1" />`)
	} else {
		html = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`).ReplaceAllString(html, "")
	}
	
	// 段落
	lines := strings.Split(html, "\n")
	var result []string
	inParagraph := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inParagraph {
				result = append(result, "</p>")
				inParagraph = false
			}
			continue
		}
		
		// 跳过已经是 HTML 标签的行
		if strings.HasPrefix(trimmed, "<") {
			if inParagraph {
				result = append(result, "</p>")
				inParagraph = false
			}
			result = append(result, line)
		} else {
			if !inParagraph {
				result = append(result, "<p>")
				inParagraph = true
			}
			result = append(result, line)
		}
	}
	
	if inParagraph {
		result = append(result, "</p>")
	}
	
	return strings.Join(result, "\n")
}

// htmlToMarkdown 将 HTML 转换为 Markdown（简化版）
func (t *TextConverterTool) htmlToMarkdown(text string, preserveLinks, preserveImages bool) string {
	md := text
	
	// 标题转换
	md = regexp.MustCompile(`<h1>(.*?)</h1>`).ReplaceAllString(md, "# $1")
	md = regexp.MustCompile(`<h2>(.*?)</h2>`).ReplaceAllString(md, "## $1")
	md = regexp.MustCompile(`<h3>(.*?)</h3>`).ReplaceAllString(md, "### $1")
	
	// 粗体和斜体
	md = regexp.MustCompile(`<strong><em>(.*?)</em></strong>`).ReplaceAllString(md, "***$1***")
	md = regexp.MustCompile(`<strong>(.*?)</strong>`).ReplaceAllString(md, "**$1**")
	md = regexp.MustCompile(`<em>(.*?)</em>`).ReplaceAllString(md, "*$1*")
	md = regexp.MustCompile(`<b>(.*?)</b>`).ReplaceAllString(md, "**$1**")
	md = regexp.MustCompile(`<i>(.*?)</i>`).ReplaceAllString(md, "*$1*")
	
	// 代码
	md = regexp.MustCompile(`<pre><code>([\s\S]*?)</code></pre>`).ReplaceAllString(md, "```\n$1\n```")
	md = regexp.MustCompile(`<code>(.*?)</code>`).ReplaceAllString(md, "`$1`")
	
	// 链接
	if preserveLinks {
		md = regexp.MustCompile(`<a href="(.*?)">(.*?)</a>`).ReplaceAllString(md, "[$2]($1)")
	} else {
		md = regexp.MustCompile(`<a href="(.*?)">(.*?)</a>`).ReplaceAllString(md, "$2")
	}
	
	// 图片
	if preserveImages {
		md = regexp.MustCompile(`<img src="(.*?)" alt="(.*?)" ?/?>`).ReplaceAllString(md, "![$2]($1)")
	} else {
		md = regexp.MustCompile(`<img src="(.*?)" alt="(.*?)" ?/?>`).ReplaceAllString(md, "")
	}
	
	// 段落
	md = regexp.MustCompile(`<p>(.*?)</p>`).ReplaceAllString(md, "$1\n\n")
	md = regexp.MustCompile(`<br ?/?>`).ReplaceAllString(md, "\n")
	
	// 清理多余的换行
	md = regexp.MustCompile(`\n{3,}`).ReplaceAllString(md, "\n\n")
	
	return strings.TrimSpace(md)
}

// markdownToPlain 将 Markdown 转换为纯文本
func (t *TextConverterTool) markdownToPlain(text string) string {
	plain := text
	
	// 移除标题标记
	plain = regexp.MustCompile(`(?m)^#{1,6}\s+(.*?)$`).ReplaceAllString(plain, "$1")
	
	// 移除粗体和斜体
	plain = regexp.MustCompile(`\*\*\*(.*?)\*\*\*`).ReplaceAllString(plain, "$1")
	plain = regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(plain, "$1")
	plain = regexp.MustCompile(`\*(.*?)\*`).ReplaceAllString(plain, "$1")
	
	// 移除代码块
	plain = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(plain, "")
	plain = regexp.MustCompile("`(.*?)`").ReplaceAllString(plain, "$1")
	
	// 移除链接和图片
	plain = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`).ReplaceAllString(plain, "")
	plain = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`).ReplaceAllString(plain, "$1")
	
	// 清理多余空行
	plain = regexp.MustCompile(`\n{3,}`).ReplaceAllString(plain, "\n\n")
	
	return strings.TrimSpace(plain)
}

// htmlToPlain 将 HTML 转换为纯文本
func (t *TextConverterTool) htmlToPlain(text string) string {
	plain := text
	
	// 移除所有 HTML 标签
	plain = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(plain, "")
	
	// 解码 HTML 实体
	plain = strings.ReplaceAll(plain, "&lt;", "<")
	plain = strings.ReplaceAll(plain, "&gt;", ">")
	plain = strings.ReplaceAll(plain, "&amp;", "&")
	plain = strings.ReplaceAll(plain, "&quot;", "\"")
	plain = strings.ReplaceAll(plain, "&#39;", "'")
	plain = strings.ReplaceAll(plain, "&nbsp;", " ")
	
	// 清理多余空行
	plain = regexp.MustCompile(`\n{3,}`).ReplaceAllString(plain, "\n\n")
	
	return strings.TrimSpace(plain)
}

// plainToMarkdown 将纯文本转换为 Markdown
func (t *TextConverterTool) plainToMarkdown(text string) string {
	// 纯文本转 Markdown 主要是保持原样，添加段落分隔
	return text
}

// plainToHTML 将纯文本转换为 HTML
func (t *TextConverterTool) plainToHTML(text string) string {
	html := text
	
	// 转义 HTML 特殊字符
	html = strings.ReplaceAll(html, "&", "&amp;")
	html = strings.ReplaceAll(html, "<", "&lt;")
	html = strings.ReplaceAll(html, ">", "&gt;")
	html = strings.ReplaceAll(html, "\"", "&quot;")
	html = strings.ReplaceAll(html, "'", "&#39;")
	
	// 段落处理
	paragraphs := strings.Split(html, "\n\n")
	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			// 处理单行换行
			p = strings.ReplaceAll(p, "\n", "<br />")
			result = append(result, "<p>"+p+"</p>")
		}
	}
	
	return strings.Join(result, "\n")
}

// Validate 验证输入
func (t *TextConverterTool) Validate(input map[string]any) error {
	if _, ok := input["text"].(string); !ok {
		return fmt.Errorf("缺少必需参数: text")
	}
	
	fromFormat, ok := input["from_format"].(string)
	if !ok {
		return fmt.Errorf("缺少必需参数: from_format")
	}
	
	toFormat, ok := input["to_format"].(string)
	if !ok {
		return fmt.Errorf("缺少必需参数: to_format")
	}
	
	validFormats := map[string]bool{"markdown": true, "html": true, "plain": true}
	if !validFormats[fromFormat] {
		return fmt.Errorf("from_format 必须是 markdown、html 或 plain")
	}
	if !validFormats[toFormat] {
		return fmt.Errorf("to_format 必须是 markdown、html 或 plain")
	}
	
	return nil
}

// GetDefinition 获取工具定义
func (t *TextConverterTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "text_converter",
		DisplayName: "文本格式转换",
		Description: "在 Markdown、HTML 和纯文本之间相互转换。支持保留链接、图片等选项。",
		Category:    "text_processing",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "待转换的文本",
				},
				"from_format": map[string]any{
					"type":        "string",
					"enum":        []string{"markdown", "html", "plain"},
					"description": "源格式",
				},
				"to_format": map[string]any{
					"type":        "string",
					"enum":        []string{"markdown", "html", "plain"},
					"description": "目标格式",
				},
				"options": map[string]any{
					"type":        "object",
					"description": "转换选项（可选）",
					"properties": map[string]any{
						"preserve_links": map[string]any{
							"type":    "boolean",
							"default": true,
							"description": "是否保留链接",
						},
						"preserve_images": map[string]any{
							"type":    "boolean",
							"default": true,
							"description": "是否保留图片",
						},
					},
				},
			},
			"required": []string{"text", "from_format", "to_format"},
		},
		Timeout:     10,
		Status:      "active",
		RequireAuth: false,
	}
}
