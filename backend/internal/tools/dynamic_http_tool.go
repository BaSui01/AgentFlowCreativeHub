package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DynamicHTTPTool 根据预设配置调用外部 HTTP API，支持运行时覆盖部分参数
type DynamicHTTPTool struct {
	definition *ToolDefinition
	client     *http.Client
}

// NewDynamicHTTPTool 创建动态 HTTP 工具实例
func NewDynamicHTTPTool(def *ToolDefinition) *DynamicHTTPTool {
	return &DynamicHTTPTool{
		definition: def,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute 执行 HTTP 请求
func (t *DynamicHTTPTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	payload, err := t.buildPayload(input)
	if err != nil {
		return nil, err
	}
	req, err := t.buildRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	var respBody any
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(body, &respBody); err != nil {
			respBody = string(body)
		}
	} else {
		respBody = string(body)
	}
	return map[string]any{
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"headers":     headerToMap(resp.Header),
		"body":        respBody,
		"success":     resp.StatusCode >= 200 && resp.StatusCode < 300,
	}, nil
}

// Validate 验证输入
func (t *DynamicHTTPTool) Validate(input map[string]any) error {
	payload, err := t.buildPayload(input)
	if err != nil {
		return err
	}
	method, _ := payload["method"].(string)
	if method == "" {
		return fmt.Errorf("缺少必需参数: method")
	}
	method = strings.ToUpper(method)
	allowed := map[string]struct{}{"GET": {}, "POST": {}, "PUT": {}, "DELETE": {}, "PATCH": {}, "HEAD": {}, "OPTIONS": {}}
	if _, ok := allowed[method]; !ok {
		return fmt.Errorf("不支持的 HTTP 方法: %s", method)
	}
	urlStr, _ := payload["url"].(string)
	if urlStr == "" {
		return fmt.Errorf("缺少必需参数: url")
	}
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return fmt.Errorf("URL 必须以 http:// 或 https:// 开头")
	}
	if auth, ok := payload["auth"].(map[string]any); ok {
		if _, ok := auth["type"].(string); !ok {
			return fmt.Errorf("auth.type 参数缺失")
		}
	}
	return nil
}

func (t *DynamicHTTPTool) buildPayload(input map[string]any) (map[string]any, error) {
	if t.definition == nil || t.definition.HTTPConfig == nil {
		return nil, fmt.Errorf("HTTP 工具缺少配置")
	}
	cfg := t.definition.HTTPConfig
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if override, ok := getString(input, "method"); ok {
		method = strings.ToUpper(override)
	}
	if method == "" {
		return nil, fmt.Errorf("缺少 HTTP 方法配置")
	}
	urlStr := cfg.URL
	if override, ok := getString(input, "url"); ok {
		urlStr = override
	}
	if urlStr == "" {
		return nil, fmt.Errorf("缺少 URL 配置")
	}
	if query, ok := input["query"].(map[string]any); ok {
		parsed, err := url.Parse(urlStr)
		if err != nil {
			return nil, fmt.Errorf("解析 URL 失败: %w", err)
		}
		values := parsed.Query()
		for k, v := range query {
			if str, ok := v.(string); ok {
				values.Set(k, str)
			}
		}
		parsed.RawQuery = values.Encode()
		urlStr = parsed.String()
	}
	headers := map[string]string{}
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	if overrides, ok := input["headers"].(map[string]any); ok {
		for k, v := range overrides {
			if str, ok := v.(string); ok {
				headers[k] = str
			}
		}
	}
	payload := map[string]any{
		"method":  method,
		"url":     urlStr,
		"headers": headers,
	}
	if body, ok := input["body"]; ok {
		payload["body"] = body
	}
	if cfg.Auth != nil {
		payload["auth"] = map[string]any{
			"type":    cfg.Auth.Type,
			"token":   cfg.Auth.Token,
			"api_key": cfg.Auth.APIKey,
			"header":  cfg.Auth.Header,
		}
	}
	if override, ok := input["auth"].(map[string]any); ok {
		authPayload, _ := payload["auth"].(map[string]any)
		if authPayload == nil {
			authPayload = map[string]any{}
		}
		for k, v := range override {
			if str, ok := v.(string); ok {
				authPayload[k] = str
			}
		}
		payload["auth"] = authPayload
	}
	return payload, nil
}

func (t *DynamicHTTPTool) buildRequest(ctx context.Context, payload map[string]any) (*http.Request, error) {
	method, _ := payload["method"].(string)
	urlStr, _ := payload["url"].(string)
	var bodyReader io.Reader
	if body, ok := payload["body"]; ok && body != nil {
		switch v := body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		case map[string]any:
			data, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("序列化请求体失败: %w", err)
			}
			bodyReader = bytes.NewReader(data)
		default:
			return nil, fmt.Errorf("body 参数必须为字符串或对象")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	if headers, ok := payload["headers"].(map[string]string); ok {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth, ok := payload["auth"].(map[string]any); ok {
		if err := applyAuth(req, auth); err != nil {
			return nil, err
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "AgentFlowCreativeHub/1.0")
	}
	return req, nil
}

func applyAuth(req *http.Request, auth map[string]any) error {
	typ, _ := auth["type"].(string)
	switch typ {
	case "bearer":
		token, _ := auth["token"].(string)
		if token == "" {
			return fmt.Errorf("bearer 认证需要 token")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	case "api_key":
		key, _ := auth["api_key"].(string)
		if key == "" {
			return fmt.Errorf("api_key 认证需要 api_key")
		}
		header := auth["header"].(string)
		if header == "" {
			header = "X-API-Key"
		}
		req.Header.Set(header, key)
	case "basic":
		username, _ := auth["username"].(string)
		password, _ := auth["password"].(string)
		if username == "" || password == "" {
			return fmt.Errorf("basic 认证需要 username/password")
		}
		req.SetBasicAuth(username, password)
	case "":
		return nil
	default:
		return fmt.Errorf("不支持的认证类型: %s", typ)
	}
	return nil
}

func headerToMap(header http.Header) map[string]string {
	result := make(map[string]string, len(header))
	for k, values := range header {
		if len(values) > 0 {
			result[k] = values[0]
		}
	}
	return result
}

func getString(input map[string]any, key string) (string, bool) {
	if input == nil {
		return "", false
	}
	if val, ok := input[key]; ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}
