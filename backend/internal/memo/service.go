package memo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// 错误定义
var (
	ErrMemoNotFound = errors.New("memo not found")
)

// MemoService 备忘录服务
type MemoService struct {
	store MemoStore
}

// CreateMemoRequest 创建备忘录请求
type CreateMemoRequest struct {
	Title    string         `json:"title" binding:"required"`
	Content  string         `json:"content"`
	Category string         `json:"category,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Priority MemoPriority   `json:"priority,omitempty"`
	DueDate  *time.Time     `json:"due_date,omitempty"`
	Reminder *time.Time     `json:"reminder,omitempty"`
	Color    string         `json:"color,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// UpdateMemoRequest 更新备忘录请求
type UpdateMemoRequest struct {
	Title    *string        `json:"title,omitempty"`
	Content  *string        `json:"content,omitempty"`
	Category *string        `json:"category,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Priority *MemoPriority  `json:"priority,omitempty"`
	DueDate  *time.Time     `json:"due_date,omitempty"`
	Reminder *time.Time     `json:"reminder,omitempty"`
	Color    *string        `json:"color,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query    string       `json:"query" binding:"required"`
	Category string       `json:"category,omitempty"`
	Tags     []string     `json:"tags,omitempty"`
	Status   MemoStatus   `json:"status,omitempty"`
	Priority MemoPriority `json:"priority,omitempty"`
	Limit    int          `json:"limit,omitempty"`
	Offset   int          `json:"offset,omitempty"`
}

// MemoStore 备忘录存储接口
type MemoStore interface {
	Create(ctx context.Context, memo *Memo) error
	Update(ctx context.Context, memo *Memo) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*Memo, error)
	List(ctx context.Context, filter MemoFilter) ([]*Memo, error)
	ListCategories(ctx context.Context, userID string) ([]string, error)
	ListTags(ctx context.Context, userID string) ([]string, error)
}

// Memo 备忘录
type Memo struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	Category    string         `json:"category,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Priority    MemoPriority   `json:"priority"`
	Status      MemoStatus     `json:"status"`
	Color       string         `json:"color,omitempty"`
	IsPinned    bool           `json:"is_pinned"`
	IsArchived  bool           `json:"is_archived"`
	DueDate     *time.Time     `json:"due_date,omitempty"`
	Reminder    *time.Time     `json:"reminder,omitempty"`
	Attachments []Attachment   `json:"attachments,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// MemoPriority 优先级
type MemoPriority string

const (
	PriorityLow    MemoPriority = "low"
	PriorityNormal MemoPriority = "normal"
	PriorityHigh   MemoPriority = "high"
	PriorityUrgent MemoPriority = "urgent"
)

// MemoStatus 状态
type MemoStatus string

const (
	StatusActive    MemoStatus = "active"
	StatusCompleted MemoStatus = "completed"
	StatusArchived  MemoStatus = "archived"
)

// Attachment 附件
type Attachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// MemoFilter 过滤条件
type MemoFilter struct {
	UserID     string
	Category   string
	Tags       []string
	Status     MemoStatus
	Priority   MemoPriority
	IsPinned   *bool
	IsArchived *bool
	Search     string
	StartDate  *time.Time
	EndDate    *time.Time
	SortBy     string
	SortOrder  string
	Limit      int
	Offset     int
}

// NewMemoService 创建备忘录服务
func NewMemoService(store MemoStore) *MemoService {
	return &MemoService{store: store}
}

// Create 创建备忘录
func (s *MemoService) Create(ctx context.Context, memo *Memo) error {
	if memo.ID == "" {
		memo.ID = fmt.Sprintf("memo_%d", time.Now().UnixNano())
	}
	if memo.Priority == "" {
		memo.Priority = PriorityNormal
	}
	if memo.Status == "" {
		memo.Status = StatusActive
	}
	memo.CreatedAt = time.Now()
	memo.UpdatedAt = time.Now()

	return s.store.Create(ctx, memo)
}

// Update 更新备忘录
func (s *MemoService) Update(ctx context.Context, memo *Memo) error {
	memo.UpdatedAt = time.Now()
	return s.store.Update(ctx, memo)
}

// Delete 删除备忘录
func (s *MemoService) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

// Get 获取备忘录
func (s *MemoService) Get(ctx context.Context, id string) (*Memo, error) {
	return s.store.Get(ctx, id)
}

// List 列出备忘录
func (s *MemoService) List(ctx context.Context, filter MemoFilter) ([]*Memo, error) {
	return s.store.List(ctx, filter)
}

// Archive 归档备忘录
func (s *MemoService) Archive(ctx context.Context, id string) error {
	memo, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	memo.IsArchived = true
	memo.Status = StatusArchived
	return s.Update(ctx, memo)
}

// Unarchive 取消归档
func (s *MemoService) Unarchive(ctx context.Context, id string) error {
	memo, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	memo.IsArchived = false
	memo.Status = StatusActive
	return s.Update(ctx, memo)
}

// Pin 置顶备忘录
func (s *MemoService) Pin(ctx context.Context, id string) error {
	memo, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	memo.IsPinned = true
	return s.Update(ctx, memo)
}

// Unpin 取消置顶
func (s *MemoService) Unpin(ctx context.Context, id string) error {
	memo, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	memo.IsPinned = false
	return s.Update(ctx, memo)
}

// Complete 标记完成
func (s *MemoService) Complete(ctx context.Context, id string) error {
	memo, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	memo.Status = StatusCompleted
	return s.Update(ctx, memo)
}

// SetCategory 设置分类
func (s *MemoService) SetCategory(ctx context.Context, id, category string) error {
	memo, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	memo.Category = category
	return s.Update(ctx, memo)
}

// AddTag 添加标签
func (s *MemoService) AddTag(ctx context.Context, id, tag string) error {
	memo, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	// 检查是否已存在
	for _, t := range memo.Tags {
		if t == tag {
			return nil
		}
	}

	memo.Tags = append(memo.Tags, tag)
	return s.Update(ctx, memo)
}

// RemoveTag 移除标签
func (s *MemoService) RemoveTag(ctx context.Context, id, tag string) error {
	memo, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	newTags := make([]string, 0, len(memo.Tags))
	for _, t := range memo.Tags {
		if t != tag {
			newTags = append(newTags, t)
		}
	}
	memo.Tags = newTags

	return s.Update(ctx, memo)
}

// ListCategories 列出所有分类
func (s *MemoService) ListCategories(ctx context.Context, userID string) ([]string, error) {
	return s.store.ListCategories(ctx, userID)
}

// ListTags 列出所有标签
func (s *MemoService) ListTags(ctx context.Context, userID string) ([]string, error) {
	return s.store.ListTags(ctx, userID)
}

// ========== 导出功能 ==========

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportMarkdown ExportFormat = "markdown"
	ExportJSON     ExportFormat = "json"
	ExportHTML     ExportFormat = "html"
)

// ExportOptions 导出选项
type ExportOptions struct {
	Format       ExportFormat
	IncludeArch  bool   // 包含归档
	Category     string // 按分类筛选
	Tags         []string
}

// Export 导出备忘录
func (s *MemoService) Export(ctx context.Context, userID string, opts ExportOptions) ([]byte, error) {
	filter := MemoFilter{
		UserID:   userID,
		Category: opts.Category,
		Tags:     opts.Tags,
	}

	if !opts.IncludeArch {
		f := false
		filter.IsArchived = &f
	}

	memos, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	switch opts.Format {
	case ExportMarkdown:
		return s.exportMarkdown(memos)
	case ExportJSON:
		return s.exportJSON(memos)
	case ExportHTML:
		return s.exportHTML(memos)
	default:
		return s.exportMarkdown(memos)
	}
}

func (s *MemoService) exportMarkdown(memos []*Memo) ([]byte, error) {
	var md strings.Builder

	md.WriteString("# Memos Export\n\n")
	md.WriteString(fmt.Sprintf("Exported at: %s\n\n", time.Now().Format(time.RFC3339)))
	md.WriteString(fmt.Sprintf("Total: %d memos\n\n", len(memos)))
	md.WriteString("---\n\n")

	// 按分类分组
	byCategory := make(map[string][]*Memo)
	for _, m := range memos {
		cat := m.Category
		if cat == "" {
			cat = "Uncategorized"
		}
		byCategory[cat] = append(byCategory[cat], m)
	}

	// 排序分类
	categories := make([]string, 0, len(byCategory))
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		md.WriteString(fmt.Sprintf("## %s\n\n", cat))

		for _, m := range byCategory[cat] {
			// 标题
			title := m.Title
			if title == "" {
				title = "Untitled"
			}
			if m.IsPinned {
				title = "📌 " + title
			}
			md.WriteString(fmt.Sprintf("### %s\n\n", title))

			// 元信息
			md.WriteString(fmt.Sprintf("- **Created:** %s\n", m.CreatedAt.Format("2006-01-02 15:04")))
			md.WriteString(fmt.Sprintf("- **Priority:** %s\n", m.Priority))
			md.WriteString(fmt.Sprintf("- **Status:** %s\n", m.Status))
			if len(m.Tags) > 0 {
				md.WriteString(fmt.Sprintf("- **Tags:** %s\n", strings.Join(m.Tags, ", ")))
			}
			if m.DueDate != nil {
				md.WriteString(fmt.Sprintf("- **Due:** %s\n", m.DueDate.Format("2006-01-02")))
			}
			md.WriteString("\n")

			// 内容
			md.WriteString(m.Content)
			md.WriteString("\n\n---\n\n")
		}
	}

	return []byte(md.String()), nil
}

func (s *MemoService) exportJSON(memos []*Memo) ([]byte, error) {
	export := struct {
		ExportedAt time.Time `json:"exported_at"`
		Count      int       `json:"count"`
		Memos      []*Memo   `json:"memos"`
	}{
		ExportedAt: time.Now(),
		Count:      len(memos),
		Memos:      memos,
	}

	return json.MarshalIndent(export, "", "  ")
}

func (s *MemoService) exportHTML(memos []*Memo) ([]byte, error) {
	var html strings.Builder

	html.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	html.WriteString("<meta charset=\"UTF-8\">\n")
	html.WriteString("<title>Memos Export</title>\n")
	html.WriteString("<style>\n")
	html.WriteString("body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }\n")
	html.WriteString(".memo { border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin-bottom: 16px; }\n")
	html.WriteString(".memo.pinned { border-color: #f59e0b; }\n")
	html.WriteString(".memo-title { font-size: 18px; font-weight: 600; margin-bottom: 8px; }\n")
	html.WriteString(".memo-meta { color: #666; font-size: 12px; margin-bottom: 8px; }\n")
	html.WriteString(".memo-content { line-height: 1.6; }\n")
	html.WriteString(".tag { background: #e5e7eb; padding: 2px 8px; border-radius: 4px; font-size: 12px; margin-right: 4px; }\n")
	html.WriteString(".category { color: #3b82f6; font-weight: 500; }\n")
	html.WriteString("</style>\n")
	html.WriteString("</head>\n<body>\n")

	html.WriteString("<h1>Memos Export</h1>\n")
	html.WriteString(fmt.Sprintf("<p>Exported at: %s | Total: %d memos</p>\n", time.Now().Format("2006-01-02 15:04"), len(memos)))

	for _, m := range memos {
		class := "memo"
		if m.IsPinned {
			class += " pinned"
		}
		html.WriteString(fmt.Sprintf("<div class=\"%s\">\n", class))

		title := m.Title
		if title == "" {
			title = "Untitled"
		}
		html.WriteString(fmt.Sprintf("<div class=\"memo-title\">%s</div>\n", title))

		html.WriteString("<div class=\"memo-meta\">\n")
		if m.Category != "" {
			html.WriteString(fmt.Sprintf("<span class=\"category\">%s</span> | ", m.Category))
		}
		html.WriteString(fmt.Sprintf("%s | %s", m.CreatedAt.Format("2006-01-02"), m.Priority))
		html.WriteString("</div>\n")

		if len(m.Tags) > 0 {
			html.WriteString("<div class=\"memo-tags\">\n")
			for _, tag := range m.Tags {
				html.WriteString(fmt.Sprintf("<span class=\"tag\">%s</span>", tag))
			}
			html.WriteString("</div>\n")
		}

		html.WriteString(fmt.Sprintf("<div class=\"memo-content\">%s</div>\n", m.Content))
		html.WriteString("</div>\n")
	}

	html.WriteString("</body>\n</html>")

	return []byte(html.String()), nil
}

// Import 导入备忘录（从 JSON）
func (s *MemoService) Import(ctx context.Context, userID string, data []byte) (int, error) {
	var imported struct {
		Memos []*Memo `json:"memos"`
	}

	if err := json.Unmarshal(data, &imported); err != nil {
		return 0, fmt.Errorf("invalid json: %w", err)
	}

	count := 0
	for _, m := range imported.Memos {
		m.ID = "" // 重新生成 ID
		m.UserID = userID
		if err := s.Create(ctx, m); err == nil {
			count++
		}
	}

	return count, nil
}
