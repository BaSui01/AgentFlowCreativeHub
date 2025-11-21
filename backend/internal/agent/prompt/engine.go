package prompt

import (
	"context"
	"sync"
)

// Engine Prompt 模板引擎
type Engine struct {
	loader Loader
	cache  map[string]*Template
	mu     sync.RWMutex
}

// NewEngine 创建模板引擎
func NewEngine(loader Loader) *Engine {
	return &Engine{
		loader: loader,
		cache:  make(map[string]*Template),
	}
}

// Render 加载并渲染模板
func (e *Engine) Render(ctx context.Context, templateID string, vars map[string]any) (string, error) {
	// 1. 尝试从缓存获取
	e.mu.RLock()
	tmpl, ok := e.cache[templateID]
	e.mu.RUnlock()

	if !ok {
		// 2. 从 Loader 加载
		var err error
		tmpl, err = e.loader.Load(ctx, templateID)
		if err != nil {
			return "", err
		}

		// 3. 更新缓存
		e.mu.Lock()
		e.cache[templateID] = tmpl
		e.mu.Unlock()
	}

	// 4. 渲染
	return tmpl.Render(vars)
}

// ClearCache 清理缓存
func (e *Engine) ClearCache(templateID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if templateID == "" {
		e.cache = make(map[string]*Template)
	} else {
		delete(e.cache, templateID)
	}
}
