package parsers

import (
	"fmt"
	"io"
	"strings"
)

// TextParser 文本文件解析器
// 支持: .txt, .md
type TextParser struct{}

// NewTextParser 创建文本解析器
func NewTextParser() *TextParser {
	return &TextParser{}
}

// Parse 解析文本文件
func (p *TextParser) Parse(reader io.Reader) (string, error) {
	// 读取全部内容
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	// 转换为字符串
	text := string(content)

	// 基本清理
	text = strings.TrimSpace(text)

	if text == "" {
		return "", fmt.Errorf("文件内容为空")
	}

	return text, nil
}

// SupportedExtensions 支持的文件扩展名
func (p *TextParser) SupportedExtensions() []string {
	return []string{".txt", ".md", ".markdown"}
}

// CanParse 检查是否可以解析指定扩展名的文件
func (p *TextParser) CanParse(extension string) bool {
	extension = strings.ToLower(extension)
	for _, ext := range p.SupportedExtensions() {
		if ext == extension {
			return true
		}
	}
	return false
}
