package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"backend/internal/tools"
	"backend/pkg/httputil"
)

// HTTPAPITool HTTP API 工具 - 通用的 REST API 调用工具
type HTTPAPITool struct {
	client *httputil.Client
}

// NewHTTPAPITool 创建 HTTP API 工具
func NewHTTPAPITool() *HTTPAPITool {
	return &HTTPAPITool{
		client: httputil.NewClient(
			httputil.WithTimeout(30 * time.Second),
		),
	}
}

// Execute 执行 HTTP API 调用
func (t *HTTPAPITool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 提取必需参数
	method, ok := input["method"].(string)
	if !ok || method == "" {
		return nil, fmt.Errorf("method 参数类型错误或为空")
	}
	method = strings.ToUpper(method)
	
	url, ok := input["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url 参数类型错误或为空")
	}
	
	// 提取可选参数
	var bodyData []byte
	if body, ok := input["body"]; ok && body != nil {
		// 支持 map 或 string
		switch v := body.(type) {
		case string:
			bodyData = []byte(v)
		case map[string]any:
			var err error
			bodyData, err = json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("序列化请求体失败: %w", err)
			}
		default:
			return nil, fmt.Errorf("body 参数类型错误，必须是字符串或对象")
		}
	}
	
	// 创建请求
	var req *http.Request
	var err error
	
	if len(bodyData) > 0 {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyData))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	
	// 设置请求头
	if headers, ok := input["headers"].(map[string]any); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Set(key, strValue)
			}
		}
	}
	
	// 如果有 body 且没有设置 Content-Type，默认设置为 application/json
	if len(bodyData) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	
	// 设置认证
	if auth, ok := input["auth"].(map[string]any); ok {
		if err := t.setAuth(req, auth); err != nil {
			return nil, fmt.Errorf("设置认证失败: %w", err)
		}
	}
	
	// 设置 User-Agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "AgentFlowCreativeHub/1.0")
	}
	
	// 发送请求
	resp, err := t.client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	
	// 尝试解析为 JSON
	var respData any
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.Unmarshal(respBody, &respData); err != nil {
			// 解析失败，返回原始字符串
			respData = string(respBody)
		}
	} else {
		respData = string(respBody)
	}
	
	// 构建结果
	return map[string]any{
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"headers":     t.headerToMap(resp.Header),
		"body":        respData,
		"success":     resp.StatusCode >= 200 && resp.StatusCode < 300,
	}, nil
}

// setAuth 设置认证
func (t *HTTPAPITool) setAuth(req *http.Request, auth map[string]any) error {
	authType, ok := auth["type"].(string)
	if !ok {
		return fmt.Errorf("auth.type 参数缺失或类型错误")
	}
	
	switch authType {
	case "bearer":
		token, ok := auth["token"].(string)
		if !ok || token == "" {
			return fmt.Errorf("bearer 认证需要 token 参数")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		
	case "api_key":
		apiKey, ok := auth["api_key"].(string)
		if !ok || apiKey == "" {
			return fmt.Errorf("api_key 认证需要 api_key 参数")
		}
		header, ok := auth["header"].(string)
		if !ok || header == "" {
			header = "X-API-Key" // 默认头部
		}
		req.Header.Set(header, apiKey)
		
	case "basic":
		username, ok := auth["username"].(string)
		if !ok {
			return fmt.Errorf("basic 认证需要 username 参数")
		}
		password, ok := auth["password"].(string)
		if !ok {
			return fmt.Errorf("basic 认证需要 password 参数")
		}
		req.SetBasicAuth(username, password)
		
	default:
		return fmt.Errorf("不支持的认证类型: %s", authType)
	}
	
	return nil
}

// headerToMap 将 HTTP Header 转换为 map
func (t *HTTPAPITool) headerToMap(header http.Header) map[string]string {
	result := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) > 0 {
			result[key] = values[0] // 只取第一个值
		}
	}
	return result
}

// Validate 验证输入
func (t *HTTPAPITool) Validate(input map[string]any) error {
	// 验证 method
	method, ok := input["method"].(string)
	if !ok {
		return fmt.Errorf("缺少必需参数: method")
	}
	
	method = strings.ToUpper(method)
	allowedMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}
	
	if !allowedMethods[method] {
		return fmt.Errorf("不支持的 HTTP 方法: %s", method)
	}
	
	// 验证 URL
	url, ok := input["url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("缺少必需参数: url")
	}
	
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("URL 必须以 http:// 或 https:// 开头")
	}
	
	// 验证 auth（如果提供）
	if auth, ok := input["auth"].(map[string]any); ok {
		authType, ok := auth["type"].(string)
		if !ok {
			return fmt.Errorf("auth.type 参数缺失")
		}
		
		switch authType {
		case "bearer":
			if _, ok := auth["token"]; !ok {
				return fmt.Errorf("bearer 认证需要 token 参数")
			}
		case "api_key":
			if _, ok := auth["api_key"]; !ok {
				return fmt.Errorf("api_key 认证需要 api_key 参数")
			}
		case "basic":
			if _, ok := auth["username"]; !ok {
				return fmt.Errorf("basic 认证需要 username 参数")
			}
			if _, ok := auth["password"]; !ok {
				return fmt.Errorf("basic 认证需要 password 参数")
			}
		default:
			return fmt.Errorf("不支持的认证类型: %s", authType)
		}
	}
	
	return nil
}

// GetDefinition 获取工具定义
func (t *HTTPAPITool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "http_api",
		DisplayName: "HTTP API 调用",
		Description: "通用的 HTTP/REST API 调用工具，支持 GET、POST、PUT、DELETE 等方法，支持多种认证方式",
		Category:    "api",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP 方法",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
				},
				"url": map[string]any{
					"type":        "string",
					"description": "API 端点 URL（必须以 http:// 或 https:// 开头）",
				},
				"headers": map[string]any{
					"type":        "object",
					"description": "HTTP 请求头（可选）",
				},
				"body": map[string]any{
					"type":        "object",
					"description": "请求体（对象或字符串，可选）",
				},
				"auth": map[string]any{
					"type":        "object",
					"description": "认证配置（可选）",
					"properties": map[string]any{
						"type": map[string]any{
							"type": "string",
							"enum": []string{"bearer", "api_key", "basic"},
							"description": "认证类型",
						},
						"token": map[string]any{
							"type":        "string",
							"description": "Bearer Token（type=bearer 时使用）",
						},
						"api_key": map[string]any{
							"type":        "string",
							"description": "API Key（type=api_key 时使用）",
						},
						"header": map[string]any{
							"type":        "string",
							"description": "API Key 头部名称（默认 X-API-Key）",
						},
						"username": map[string]any{
							"type":        "string",
							"description": "用户名（type=basic 时使用）",
						},
						"password": map[string]any{
							"type":        "string",
							"description": "密码（type=basic 时使用）",
						},
					},
				},
			},
			"required": []string{"method", "url"},
		},
		Timeout:     30,
		Status:      "active",
		RequireAuth: true, // 需要授权才能调用外部 API
	}
}
