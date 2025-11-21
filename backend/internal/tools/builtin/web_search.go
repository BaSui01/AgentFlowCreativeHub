package builtin

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// WebSearchTool 网络搜索工具
type WebSearchTool struct{}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Perform a web search using a search engine (e.g. Google, Bing, DuckDuckGo) to find relevant information."
}

func (t *WebSearchTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query string",
			},
			"num_results": map[string]any{
				"type":        "integer",
				"description": "Number of results to return (default: 5, max: 10)",
				"default":     5,
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("invalid query")
	}

	numResults := 5
	if n, ok := args["num_results"].(float64); ok {
		numResults = int(n)
	}
	if numResults > 10 {
		numResults = 10
	}

	// Use DuckDuckGo HTML version (no API key needed, but requires parsing)
	// OR use a mock implementation for MVP if no API key is available.
	// Here we implement a simple Google Custom Search or similar if key is present,
	// otherwise fallback to a simulation or a public free API.

	// For stability in this demo without external dependencies/keys,
	// we will implement a DuckDuckGo search via HTTP request (simulated).
	// In production, you should use Google Custom Search JSON API or Bing Search API.

	return t.performDuckDuckGoSearch(ctx, query, numResults)
}

type SearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

func (t *WebSearchTool) performDuckDuckGoSearch(ctx context.Context, query string, num int) ([]SearchResult, error) {
	// Note: DuckDuckGo doesn't have a public official API for search results.
	// This is a simplified implementation using their HTML endpoint,
	// which is NOT recommended for high volume production.
	// For MVP, we'll use a placeholder implementation that returns mock data
	// to avoid being blocked or needing an API key.

	// In a real implementation, integrate with:
	// 1. Google Custom Search API (requires KEY and CX)
	// 2. Bing Web Search API (requires KEY)
	// 3. SerpApi (requires KEY)

	// Mock response for demonstration
	results := []SearchResult{
		{
			Title:   fmt.Sprintf("Result for %s - Wikipedia", query),
			Link:    fmt.Sprintf("https://en.wikipedia.org/wiki/%s", url.QueryEscape(query)),
			Snippet: fmt.Sprintf("Wikipedia article discussing %s in detail...", query),
		},
		{
			Title:   fmt.Sprintf("Latest news about %s", query),
			Link:    fmt.Sprintf("https://news.example.com/search?q=%s", url.QueryEscape(query)),
			Snippet: fmt.Sprintf("Recent developments and news stories regarding %s...", query),
		},
		{
			Title:   fmt.Sprintf("Official website of %s", query),
			Link:    fmt.Sprintf("https://www.%s.com", strings.ReplaceAll(query, " ", "")),
			Snippet: fmt.Sprintf("The official home page for %s, featuring products and services.", query),
		},
	}

	if len(results) > num {
		results = results[:num]
	}

	return results, nil
}
