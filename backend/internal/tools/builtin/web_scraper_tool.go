package builtin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WebScraperTool 网页抓取工具
type WebScraperTool struct {
	client *http.Client
}

// NewWebScraperTool 创建网页抓取工具
func NewWebScraperTool() *WebScraperTool {
	return &WebScraperTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (t *WebScraperTool) Name() string {
	return "web_scraper"
}

func (t *WebScraperTool) Description() string {
	return "抓取网页内容，提取文本、链接、图片等信息"
}

func (t *WebScraperTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "要抓取的网页 URL",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS 选择器（可选，用于提取特定内容）",
			},
			"extract": map[string]any{
				"type":        "string",
				"enum":        []string{"text", "links", "images", "all"},
				"description": "提取类型：text（文本）、links（链接）、images（图片）、all（全部）",
				"default":     "text",
			},
			"max_length": map[string]any{
				"type":        "integer",
				"description": "最大返回字符数（默认 10000）",
				"default":     10000,
			},
		},
		"required": []string{"url"},
	}
}

func (t *WebScraperTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	urlStr, ok := input["url"].(string)
	if !ok || urlStr == "" {
		return nil, fmt.Errorf("url is required")
	}

	// 验证 URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("only http/https urls are supported")
	}

	extractType := "text"
	if et, ok := input["extract"].(string); ok {
		extractType = et
	}

	maxLength := 10000
	if ml, ok := input["max_length"].(float64); ok {
		maxLength = int(ml)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: %d %s", resp.StatusCode, resp.Status)
	}

	// 读取内容（限制大小）
	limitedReader := io.LimitReader(resp.Body, 5*1024*1024) // 5MB limit
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	html := string(body)
	result := map[string]any{
		"url":          urlStr,
		"status_code":  resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
	}

	switch extractType {
	case "text":
		text := t.extractText(html)
		if len(text) > maxLength {
			text = text[:maxLength] + "..."
		}
		result["text"] = text
		result["length"] = len(text)

	case "links":
		links := t.extractLinks(html, parsedURL)
		result["links"] = links
		result["count"] = len(links)

	case "images":
		images := t.extractImages(html, parsedURL)
		result["images"] = images
		result["count"] = len(images)

	case "all":
		text := t.extractText(html)
		if len(text) > maxLength {
			text = text[:maxLength] + "..."
		}
		result["text"] = text
		result["links"] = t.extractLinks(html, parsedURL)
		result["images"] = t.extractImages(html, parsedURL)
		result["title"] = t.extractTitle(html)
		result["meta"] = t.extractMeta(html)
	}

	return result, nil
}

// extractText 提取纯文本内容
func (t *WebScraperTool) extractText(html string) string {
	// 移除 script 和 style 标签
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// 移除 HTML 注释
	commentRe := regexp.MustCompile(`<!--[\s\S]*?-->`)
	html = commentRe.ReplaceAllString(html, "")

	// 替换块级元素为换行
	blockRe := regexp.MustCompile(`(?i)</(p|div|br|h[1-6]|li|tr)>`)
	html = blockRe.ReplaceAllString(html, "\n")

	// 移除所有 HTML 标签
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text := tagRe.ReplaceAllString(html, "")

	// 解码 HTML 实体
	text = t.decodeHTMLEntities(text)

	// 清理空白
	spaceRe := regexp.MustCompile(`[ \t]+`)
	text = spaceRe.ReplaceAllString(text, " ")

	newlineRe := regexp.MustCompile(`\n{3,}`)
	text = newlineRe.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}

// extractLinks 提取链接
func (t *WebScraperTool) extractLinks(html string, baseURL *url.URL) []map[string]string {
	linkRe := regexp.MustCompile(`(?i)<a[^>]+href=["']([^"']+)["'][^>]*>([^<]*)</a>`)
	matches := linkRe.FindAllStringSubmatch(html, -1)

	links := make([]map[string]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		href := strings.TrimSpace(match[1])
		text := strings.TrimSpace(t.decodeHTMLEntities(match[2]))

		// 跳过空链接和锚点
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			continue
		}

		// 解析相对 URL
		absURL, err := baseURL.Parse(href)
		if err != nil {
			continue
		}
		absHref := absURL.String()

		// 去重
		if seen[absHref] {
			continue
		}
		seen[absHref] = true

		links = append(links, map[string]string{
			"url":  absHref,
			"text": text,
		})
	}

	return links
}

// extractImages 提取图片
func (t *WebScraperTool) extractImages(html string, baseURL *url.URL) []map[string]string {
	imgRe := regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["'][^>]*>`)
	altRe := regexp.MustCompile(`(?i)alt=["']([^"']*?)["']`)
	matches := imgRe.FindAllStringSubmatch(html, -1)

	images := make([]map[string]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		src := strings.TrimSpace(match[1])
		if src == "" || strings.HasPrefix(src, "data:") {
			continue
		}

		// 解析相对 URL
		absURL, err := baseURL.Parse(src)
		if err != nil {
			continue
		}
		absSrc := absURL.String()

		if seen[absSrc] {
			continue
		}
		seen[absSrc] = true

		alt := ""
		if altMatch := altRe.FindStringSubmatch(match[0]); len(altMatch) > 1 {
			alt = t.decodeHTMLEntities(altMatch[1])
		}

		images = append(images, map[string]string{
			"url": absSrc,
			"alt": alt,
		})
	}

	return images
}

// extractTitle 提取标题
func (t *WebScraperTool) extractTitle(html string) string {
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)
	if match := titleRe.FindStringSubmatch(html); len(match) > 1 {
		return strings.TrimSpace(t.decodeHTMLEntities(match[1]))
	}
	return ""
}

// extractMeta 提取 meta 信息
func (t *WebScraperTool) extractMeta(html string) map[string]string {
	meta := make(map[string]string)

	// description
	descRe := regexp.MustCompile(`(?i)<meta[^>]+name=["']description["'][^>]+content=["']([^"']*?)["']`)
	if match := descRe.FindStringSubmatch(html); len(match) > 1 {
		meta["description"] = t.decodeHTMLEntities(match[1])
	}

	// keywords
	kwRe := regexp.MustCompile(`(?i)<meta[^>]+name=["']keywords["'][^>]+content=["']([^"']*?)["']`)
	if match := kwRe.FindStringSubmatch(html); len(match) > 1 {
		meta["keywords"] = t.decodeHTMLEntities(match[1])
	}

	// og:title
	ogTitleRe := regexp.MustCompile(`(?i)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']*?)["']`)
	if match := ogTitleRe.FindStringSubmatch(html); len(match) > 1 {
		meta["og_title"] = t.decodeHTMLEntities(match[1])
	}

	// og:description
	ogDescRe := regexp.MustCompile(`(?i)<meta[^>]+property=["']og:description["'][^>]+content=["']([^"']*?)["']`)
	if match := ogDescRe.FindStringSubmatch(html); len(match) > 1 {
		meta["og_description"] = t.decodeHTMLEntities(match[1])
	}

	return meta
}

// decodeHTMLEntities 解码 HTML 实体
func (t *WebScraperTool) decodeHTMLEntities(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
		"&mdash;", "—",
		"&ndash;", "–",
		"&hellip;", "…",
		"&copy;", "©",
		"&reg;", "®",
		"&trade;", "™",
	)
	return replacer.Replace(s)
}
