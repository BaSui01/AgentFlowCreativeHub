package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"backend/internal/tools"
)

// SearchTool 搜索引擎工具（基于 DuckDuckGo Instant Answer API）
type SearchTool struct {
	client *http.Client
}

// NewSearchTool 创建搜索工具
func NewSearchTool() *SearchTool {
	return &SearchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute 执行搜索
func (t *SearchTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 提取查询参数
	query, ok := input["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query 参数类型错误或为空")
	}
	
	maxResults := 5
	if max, ok := input["max_results"].(float64); ok {
		maxResults = int(max)
		if maxResults < 1 {
			maxResults = 1
		}
		if maxResults > 20 {
			maxResults = 20
		}
	}
	
	// 构建请求 URL（使用 DuckDuckGo Instant Answer API）
	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", 
		url.QueryEscape(query))
	
	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	
	req.Header.Set("User-Agent", "AgentFlowCreativeHub/1.0")
	
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("搜索请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("搜索 API 返回错误状态: %d", resp.StatusCode)
	}
	
	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	
	// 提取相关结果
	results := make([]map[string]any, 0, maxResults)
	
	// 优先使用 AbstractText（摘要文本）
	if abstractText, ok := result["AbstractText"].(string); ok && abstractText != "" {
		abstractURL, _ := result["AbstractURL"].(string)
		results = append(results, map[string]any{
			"title":   result["Heading"],
			"snippet": abstractText,
			"url":     abstractURL,
			"source":  "instant_answer",
		})
	}
	
	// 然后使用 RelatedTopics（相关话题）
	if relatedTopics, ok := result["RelatedTopics"].([]any); ok {
		for i, topic := range relatedTopics {
			if len(results) >= maxResults {
				break
			}
			
			topicMap, ok := topic.(map[string]any)
			if !ok {
				continue
			}
			
			// 检查是否有 Topics 子字段（分类）
			if topics, ok := topicMap["Topics"].([]any); ok {
				for _, subTopic := range topics {
					if len(results) >= maxResults {
						break
					}
					
					subTopicMap, ok := subTopic.(map[string]any)
					if !ok {
						continue
					}
					
					text, _ := subTopicMap["Text"].(string)
					firstURL, _ := subTopicMap["FirstURL"].(string)
					
					if text != "" {
						results = append(results, map[string]any{
							"title":   text,
							"snippet": text,
							"url":     firstURL,
							"source":  "related_topics",
						})
					}
				}
			} else {
				// 直接使用 Topic
				text, _ := topicMap["Text"].(string)
				firstURL, _ := topicMap["FirstURL"].(string)
				
				if text != "" {
					results = append(results, map[string]any{
						"title":   text,
						"snippet": text,
						"url":     firstURL,
						"source":  "related_topics",
					})
				}
			}
			
			_ = i // 避免未使用警告
		}
	}
	
	return map[string]any{
		"query":   query,
		"results": results,
		"count":   len(results),
	}, nil
}

// Validate 验证输入
func (t *SearchTool) Validate(input map[string]any) error {
	if _, ok := input["query"]; !ok {
		return fmt.Errorf("缺少必需参数: query")
	}
	
	query, ok := input["query"].(string)
	if !ok {
		return fmt.Errorf("query 参数类型错误，必须是字符串")
	}
	
	if query == "" {
		return fmt.Errorf("query 参数不能为空")
	}
	
	return nil
}

// GetDefinition 获取工具定义
func (t *SearchTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "web_search",
		DisplayName: "网络搜索",
		Description: "使用 DuckDuckGo 搜索引擎搜索网络信息，获取相关的搜索结果",
		Category:    "search",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "搜索关键词或问题",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "最大返回结果数（默认5，最大20）",
					"default":     5,
					"minimum":     1,
					"maximum":     20,
				},
			},
			"required": []string{"query"},
		},
		Timeout:     30,
		Status:      "active",
		RequireAuth: false,
	}
}
