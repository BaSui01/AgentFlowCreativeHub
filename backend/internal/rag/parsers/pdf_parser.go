package parsers

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/dslipak/pdf"
)

// PDFParser PDF 文件解析器
type PDFParser struct{}

// NewPDFParser 创建 PDF 解析器
func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

// Parse 解析 PDF 文件
func (p *PDFParser) Parse(reader io.Reader) (string, error) {
	// 将 reader 内容读取到 bytes.Reader，因为 pdf.NewReader 需要 ReaderAt
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取 PDF 内容失败: %w", err)
	}
	
	readSeeker := bytes.NewReader(data)
	r, err := pdf.NewReader(readSeeker, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("打开 PDF 失败: %w", err)
	}

	var buf strings.Builder
	numPages := r.NumPage()

	for i := 1; i <= numPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		
		text, err := page.GetPlainText(nil)
		if err != nil {
			// 记录错误但继续处理其他页面
			fmt.Printf("解析 PDF 第 %d 页失败: %v\n", i, err)
			continue
		}
		
		buf.WriteString(text)
		buf.WriteString("\n") // 页面间添加换行
	}

	content := strings.TrimSpace(buf.String())
	if content == "" {
		return "", fmt.Errorf("PDF 内容为空或无法解析文本")
	}

	return content, nil
}

// SupportedExtensions 支持的文件扩展名
func (p *PDFParser) SupportedExtensions() []string {
	return []string{".pdf"}
}

// CanParse 检查是否可以解析指定扩展名的文件
func (p *PDFParser) CanParse(extension string) bool {
	extension = strings.ToLower(extension)
	for _, ext := range p.SupportedExtensions() {
		if ext == extension {
			return true
		}
	}
	return false
}
