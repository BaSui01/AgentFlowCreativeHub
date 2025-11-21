package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"text/template"
)

// TemplateEngine 工作流模板引擎
// 负责渲染工作流中的模板变量,支持复杂表达式和函数调用
type TemplateEngine struct {
	funcMap template.FuncMap
	cache   map[string]*template.Template
	mu      sync.RWMutex
}

// NewTemplateEngine 创建模板引擎实例
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		funcMap: DefaultFuncMap(),
		cache:   make(map[string]*template.Template),
	}
}

// DefaultFuncMap 默认函数映射
// 提供模板中可用的内置函数
func DefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		// 字符串函数
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"trim":  strings.TrimSpace,
		"title": strings.Title,

		// JSON函数
		"json": toJSON,

		// 默认值
		"default": defaultValue,

		// 数组/切片函数
		"join":  join,
		"first": first,
		"last":  last,

		// 类型检查
		"isnil": isNil,
	}
}

// Render 渲染单个模板字符串
// tmplStr: 模板字符串 (如 "写一篇关于{{.topic}}的文章")
// data: 数据上下文 (ExecutionContext.Data)
// 返回: 渲染后的字符串
func (e *TemplateEngine) Render(tmplStr string, data map[string]any) (string, error) {
	if tmplStr == "" {
		return "", nil
	}

	// 检查是否包含模板语法
	if !strings.Contains(tmplStr, "{{") {
		return tmplStr, nil // 普通字符串,直接返回
	}

	// 解析模板
	tmpl, err := e.parseTemplate(tmplStr)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	// 执行渲染
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}

	return buf.String(), nil
}

// RenderMap 渲染Map中的所有模板字段
// 递归处理嵌套Map和字符串模板
func (e *TemplateEngine) RenderMap(inputMap map[string]any, data map[string]any) (map[string]any, error) {
	if inputMap == nil {
		return nil, nil
	}

	result := make(map[string]any)

	for key, value := range inputMap {
		switch v := value.(type) {
		case string:
			// 渲染字符串模板
			rendered, err := e.Render(v, data)
			if err != nil {
				return nil, fmt.Errorf("渲染字段 %s 失败: %w", key, err)
			}
			result[key] = rendered

		case map[string]any:
			// 递归渲染嵌套Map
			rendered, err := e.RenderMap(v, data)
			if err != nil {
				return nil, err
			}
			result[key] = rendered

		case []any:
			// 渲染数组中的字符串元素
			renderedArr := make([]any, len(v))
			for i, item := range v {
				if str, ok := item.(string); ok {
					rendered, err := e.Render(str, data)
					if err != nil {
						return nil, fmt.Errorf("渲染数组元素 %s[%d] 失败: %w", key, i, err)
					}
					renderedArr[i] = rendered
				} else {
					renderedArr[i] = item
				}
			}
			result[key] = renderedArr

		default:
			// 非字符串直接复制
			result[key] = value
		}
	}

	return result, nil
}

// parseTemplate 解析模板(带缓存)
// 使用缓存避免重复解析相同的模板
func (e *TemplateEngine) parseTemplate(tmplStr string) (*template.Template, error) {
	// 生成缓存键(使用模板字符串本身)
	cacheKey := tmplStr

	// 检查缓存
	e.mu.RLock()
	if tmpl, ok := e.cache[cacheKey]; ok {
		e.mu.RUnlock()
		return tmpl, nil
	}
	e.mu.RUnlock()

	// 解析模板
	tmpl, err := template.New("workflow").Funcs(e.funcMap).Parse(tmplStr)
	if err != nil {
		return nil, err
	}

	// 存入缓存
	e.mu.Lock()
	e.cache[cacheKey] = tmpl
	e.mu.Unlock()

	return tmpl, nil
}

// ClearCache 清除模板缓存
// 用于测试或内存管理
func (e *TemplateEngine) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache = make(map[string]*template.Template)
}

// GetCacheSize 获取缓存大小
func (e *TemplateEngine) GetCacheSize() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.cache)
}

// ===== 模板函数实现 =====

// toJSON 将值转换为JSON字符串
func toJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return string(data)
}

// defaultValue 返回默认值(如果值为空)
func defaultValue(defaultVal, val any) any {
	if val == nil || val == "" {
		return defaultVal
	}
	return val
}

// join 连接字符串数组
func join(sep string, arr []string) string {
	return strings.Join(arr, sep)
}

// first 返回数组第一个元素
func first(arr []any) any {
	if len(arr) == 0 {
		return nil
	}
	return arr[0]
}

// last 返回数组最后一个元素
func last(arr []any) any {
	if len(arr) == 0 {
		return nil
	}
	return arr[len(arr)-1]
}

// isNil 检查值是否为nil
func isNil(v any) bool {
	return v == nil
}
