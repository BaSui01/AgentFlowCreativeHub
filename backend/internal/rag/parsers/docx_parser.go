package parsers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// DocxParser Word 文档解析器（.docx）
// .docx 文件本质上是 ZIP 压缩包，包含 XML 格式的文档内容
type DocxParser struct{}

// NewDocxParser 创建 DOCX 解析器
func NewDocxParser() *DocxParser {
	return &DocxParser{}
}

// Parse 解析 DOCX 文档
func (p *DocxParser) Parse(reader io.Reader) (string, error) {
	// 读取所有内容到内存（zip 需要 ReaderAt）
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取文档失败: %w", err)
	}

	// 打开 ZIP 归档
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("打开 DOCX 失败: %w", err)
	}

	// 查找 word/document.xml
	var documentXML []byte
	for _, file := range zipReader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", fmt.Errorf("打开 document.xml 失败: %w", err)
			}
			documentXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", fmt.Errorf("读取 document.xml 失败: %w", err)
			}
			break
		}
	}

	if documentXML == nil {
		return "", fmt.Errorf("无效的 DOCX 文件：找不到 document.xml")
	}

	// 解析 XML 提取文本
	text, err := p.extractTextFromXML(documentXML)
	if err != nil {
		return "", fmt.Errorf("解析文档内容失败: %w", err)
	}

	return text, nil
}

// SupportedExtensions 支持的扩展名
func (p *DocxParser) SupportedExtensions() []string {
	return []string{".docx"}
}

// CanParse 检查是否支持该扩展名
func (p *DocxParser) CanParse(ext string) bool {
	for _, e := range p.SupportedExtensions() {
		if e == ext {
			return true
		}
	}
	return false
}

// extractTextFromXML 从 Word XML 中提取纯文本
func (p *DocxParser) extractTextFromXML(xmlData []byte) (string, error) {
	var result strings.Builder

	// 定义 XML 结构
	type Text struct {
		Content string `xml:",chardata"`
	}

	type Run struct {
		Text []Text `xml:"t"`
	}

	type Paragraph struct {
		Runs []Run `xml:"r"`
	}

	type Body struct {
		Paragraphs []Paragraph `xml:"p"`
	}

	type Document struct {
		XMLName xml.Name `xml:"document"`
		Body    Body     `xml:"body"`
	}

	var doc Document
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		// 如果结构化解析失败，使用正则提取
		return p.extractTextByRegex(xmlData), nil
	}

	// 遍历段落和文本
	for i, para := range doc.Body.Paragraphs {
		var paraText strings.Builder
		for _, run := range para.Runs {
			for _, t := range run.Text {
				paraText.WriteString(t.Content)
			}
		}

		text := strings.TrimSpace(paraText.String())
		if text != "" {
			if i > 0 {
				result.WriteString("\n")
			}
			result.WriteString(text)
		}
	}

	return result.String(), nil
}

// extractTextByRegex 使用正则表达式提取文本（备用方法）
func (p *DocxParser) extractTextByRegex(xmlData []byte) string {
	content := string(xmlData)

	// 提取 <w:t> 标签中的文本
	textRegex := regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
	matches := textRegex.FindAllStringSubmatch(content, -1)

	var texts []string
	for _, match := range matches {
		if len(match) > 1 && match[1] != "" {
			texts = append(texts, match[1])
		}
	}

	// 尝试按段落分组
	paraRegex := regexp.MustCompile(`<w:p[^>]*>(.*?)</w:p>`)
	paraMatches := paraRegex.FindAllStringSubmatch(content, -1)

	if len(paraMatches) > 0 {
		var result strings.Builder
		textInParaRegex := regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)

		for i, para := range paraMatches {
			if len(para) < 2 {
				continue
			}

			textMatches := textInParaRegex.FindAllStringSubmatch(para[1], -1)
			var paraText strings.Builder

			for _, tm := range textMatches {
				if len(tm) > 1 {
					paraText.WriteString(tm[1])
				}
			}

			text := strings.TrimSpace(paraText.String())
			if text != "" {
				if i > 0 && result.Len() > 0 {
					result.WriteString("\n")
				}
				result.WriteString(text)
			}
		}

		return result.String()
	}

	return strings.Join(texts, " ")
}

// DocParser 旧版 .doc 文件解析器（二进制格式）
// 注意：.doc 是 OLE 复合文档格式，完整解析需要专门的库
// 这里提供基础的文本提取功能
type DocParser struct{}

// NewDocParser 创建 DOC 解析器
func NewDocParser() *DocParser {
	return &DocParser{}
}

// Parse 解析 DOC 文档
func (p *DocParser) Parse(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取文档失败: %w", err)
	}

	// .doc 文件是 OLE 复合文档
	// 简单提取：查找可打印的 ASCII/UTF-8 文本序列
	text := p.extractPrintableText(data)

	if text == "" {
		return "", fmt.Errorf("无法从 .doc 文件提取文本，建议转换为 .docx 格式")
	}

	return text, nil
}

// SupportedExtensions 支持的扩展名
func (p *DocParser) SupportedExtensions() []string {
	return []string{".doc"}
}

// CanParse 检查是否支持该扩展名
func (p *DocParser) CanParse(ext string) bool {
	for _, e := range p.SupportedExtensions() {
		if e == ext {
			return true
		}
	}
	return false
}

// extractPrintableText 提取可打印文本
func (p *DocParser) extractPrintableText(data []byte) string {
	var result strings.Builder
	var currentWord strings.Builder

	for _, b := range data {
		// 检查是否为可打印字符或常见空白
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			currentWord.WriteByte(b)
		} else if currentWord.Len() > 0 {
			// 遇到非打印字符，保存当前词
			word := currentWord.String()
			if len(word) >= 2 { // 过滤过短的噪声
				result.WriteString(word)
				result.WriteString(" ")
			}
			currentWord.Reset()
		}
	}

	// 处理最后一个词
	if currentWord.Len() >= 2 {
		result.WriteString(currentWord.String())
	}

	// 清理结果
	text := result.String()

	// 移除多余空白
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}
