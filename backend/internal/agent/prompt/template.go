package prompt

import (
	"context"
	"fmt"
	"strings"
	"text/template"
)

// Template Prompt 模板
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Content     string            `json:"content"` // Go text/template 格式
	Variables   []string          `json:"variables"`
	Version     string            `json:"version"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Render 渲染模板
func (t *Template) Render(vars map[string]any) (string, error) {
	tmpl, err := template.New(t.Name).Parse(t.Content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// Loader 模板加载器接口
type Loader interface {
	// Load 加载指定 ID 的模板
	Load(ctx context.Context, id string) (*Template, error)
	// List 列出所有可用模板
	List(ctx context.Context) ([]*Template, error)
}

// InMemoryLoader 内存模板加载器（用于开发/测试）
type InMemoryLoader struct {
	templates map[string]*Template
}

func NewInMemoryLoader() *InMemoryLoader {
	return &InMemoryLoader{
		templates: make(map[string]*Template),
	}
}

func (l *InMemoryLoader) Register(tmpl *Template) {
	l.templates[tmpl.ID] = tmpl
}

func (l *InMemoryLoader) Load(ctx context.Context, id string) (*Template, error) {
	t, ok := l.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return t, nil
}

func (l *InMemoryLoader) List(ctx context.Context) ([]*Template, error) {
	list := make([]*Template, 0, len(l.templates))
	for _, t := range l.templates {
		list = append(list, t)
	}
	return list, nil
}

