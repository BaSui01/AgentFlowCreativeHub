package workspace

import (
	"regexp"
	"strings"
	"time"
)

var nonWord = regexp.MustCompile(`[^\p{Han}A-Za-z0-9]+`)

// FolderTemplate 预设目录模板
type FolderTemplate struct {
	Name     string
	Slug     string
	Category string
}

// 默认根目录
var defaultFolders = []FolderTemplate{
	{Name: "大纲", Slug: "outlines", Category: "outline"},
	{Name: "草稿", Slug: "drafts", Category: "draft"},
	{Name: "研究", Slug: "research", Category: "research"},
	{Name: "素材", Slug: "assets", Category: "asset"},
	{Name: "暂存区", Slug: "staging", Category: "staging"},
}

// AutoNamingPolicy 控制自动命名与目录归类
type AutoNamingPolicy struct{}

// NewAutoNamingPolicy 创建策略实例
func NewAutoNamingPolicy() *AutoNamingPolicy {
	return &AutoNamingPolicy{}
}

// ResolveFolder 根据文件类型返回目标目录模板
func (p *AutoNamingPolicy) ResolveFolder(fileType string) FolderTemplate {
	typeLower := strings.ToLower(strings.TrimSpace(fileType))
	for _, tpl := range defaultFolders {
		if tpl.Category == typeLower {
			return tpl
		}
	}
	if typeLower == "outline" {
		return defaultFolders[0]
	}
	return defaultFolders[1] // 默认归入草稿
}

// SuggestName 根据内容生成文件名
func (p *AutoNamingPolicy) SuggestName(fileType, hint, content string) string {
	base := strings.TrimSpace(hint)
	if base == "" {
		base = extractTitle(content)
	}
	if base == "" {
		base = titleize(strings.ToLower(fileType))
	}
	base = trimNonWord(base)
	stamp := time.Now().Format("20060102-150405")
	if base == "" {
		return stamp
	}
	return base + "-" + stamp
}

func extractTitle(content string) string {
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			line = strings.TrimLeft(line, "#")
			line = strings.TrimSpace(line)
		}
		if line != "" {
			return line
		}
	}
	return ""
}

func trimNonWord(s string) string {
	s = nonWord.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len([]rune(s)) > 40 {
		s = string([]rune(s)[:40])
	}
	return s
}

func titleize(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		runes := []rune(part)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		parts[i] = string(runes)
	}
	return strings.Join(parts, "")
}
