package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"backend/internal/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TodoStatus 任务状态
type TodoStatus string

const (
	TodoStatusPending   TodoStatus = "pending"
	TodoStatusCompleted TodoStatus = "completed"
)

// TodoItem 任务项
type TodoItem struct {
	ID        string     `json:"id"`
	Content   string     `json:"content"`
	Status    TodoStatus `json:"status"`
	ParentID  *string    `json:"parent_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TodoList 任务列表
type TodoList struct {
	SessionID string     `json:"session_id"`
	Todos     []TodoItem `json:"todos"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TodoRecord 数据库记录
type TodoRecord struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenant_id" gorm:"type:uuid;not null;index"`
	SessionID string    `json:"session_id" gorm:"size:100;not null;index"`
	Data      []byte    `json:"data" gorm:"type:jsonb"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TodoRecord) TableName() string {
	return "todo_records"
}

// TodoService 任务管理服务
type TodoService struct {
	db     *gorm.DB
	cache  map[string]*TodoList // sessionID -> TodoList
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewTodoService 创建任务服务
func NewTodoService(db *gorm.DB) *TodoService {
	return &TodoService{
		db:     db,
		cache:  make(map[string]*TodoList),
		logger: logger.Get(),
	}
}

// AutoMigrate 自动迁移
func (s *TodoService) AutoMigrate() error {
	if s.db == nil {
		return nil
	}
	return s.db.AutoMigrate(&TodoRecord{})
}

// CreateTodoInput 创建任务输入
type CreateTodoInput struct {
	Content  string  `json:"content"`
	ParentID *string `json:"parent_id,omitempty"`
}

// CreateTodoList 创建或替换任务列表
func (s *TodoService) CreateTodoList(ctx context.Context, tenantID, sessionID string, items []CreateTodoInput) (*TodoList, error) {
	if sessionID == "" {
		return nil, errors.New("sessionID 不能为空")
	}

	now := time.Now()
	todos := make([]TodoItem, 0, len(items))

	for _, item := range items {
		if item.Content == "" {
			continue
		}
		todos = append(todos, TodoItem{
			ID:        fmt.Sprintf("todo-%d_%s", time.Now().UnixNano(), uuid.New().String()[:8]),
			Content:   item.Content,
			Status:    TodoStatusPending,
			ParentID:  item.ParentID,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	todoList := &TodoList{
		SessionID: sessionID,
		Todos:     todos,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 保存到缓存
	s.mu.Lock()
	s.cache[sessionID] = todoList
	s.mu.Unlock()

	// 持久化到数据库
	if s.db != nil {
		data, _ := json.Marshal(todoList)
		record := &TodoRecord{
			ID:        uuid.New().String(),
			TenantID:  tenantID,
			SessionID: sessionID,
			Data:      data,
		}

		// 使用 upsert
		s.db.WithContext(ctx).Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).Delete(&TodoRecord{})
		if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
			s.logger.Error("保存 TODO 失败", zap.Error(err))
		}
	}

	s.logger.Info("创建 TODO 列表", zap.String("session_id", sessionID), zap.Int("count", len(todos)))
	return todoList, nil
}

// GetTodoList 获取任务列表
func (s *TodoService) GetTodoList(ctx context.Context, tenantID, sessionID string) (*TodoList, error) {
	if sessionID == "" {
		return nil, errors.New("sessionID 不能为空")
	}

	// 先查缓存
	s.mu.RLock()
	if todoList, ok := s.cache[sessionID]; ok {
		s.mu.RUnlock()
		return todoList, nil
	}
	s.mu.RUnlock()

	// 从数据库加载
	if s.db != nil {
		var record TodoRecord
		err := s.db.WithContext(ctx).Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).First(&record).Error
		if err == nil {
			var todoList TodoList
			if json.Unmarshal(record.Data, &todoList) == nil {
				s.mu.Lock()
				s.cache[sessionID] = &todoList
				s.mu.Unlock()
				return &todoList, nil
			}
		}
	}

	return nil, nil
}

// UpdateTodoItem 更新任务项
func (s *TodoService) UpdateTodoItem(ctx context.Context, tenantID, sessionID, todoID string, status *TodoStatus, content *string) (*TodoList, error) {
	todoList, err := s.GetTodoList(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	if todoList == nil {
		return nil, errors.New("TODO 列表不存在")
	}

	found := false
	for i := range todoList.Todos {
		if todoList.Todos[i].ID == todoID {
			if status != nil {
				todoList.Todos[i].Status = *status
			}
			if content != nil {
				todoList.Todos[i].Content = *content
			}
			todoList.Todos[i].UpdatedAt = time.Now()
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("TODO 项不存在: %s", todoID)
	}

	todoList.UpdatedAt = time.Now()

	// 更新缓存
	s.mu.Lock()
	s.cache[sessionID] = todoList
	s.mu.Unlock()

	// 持久化
	if s.db != nil {
		data, _ := json.Marshal(todoList)
		s.db.WithContext(ctx).Model(&TodoRecord{}).
			Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).
			Update("data", data)
	}

	s.logger.Debug("更新 TODO 项", zap.String("todo_id", todoID))
	return todoList, nil
}

// AddTodoItem 添加任务项
func (s *TodoService) AddTodoItem(ctx context.Context, tenantID, sessionID string, input CreateTodoInput) (*TodoList, error) {
	todoList, err := s.GetTodoList(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if todoList == nil {
		todoList = &TodoList{
			SessionID: sessionID,
			Todos:     make([]TodoItem, 0),
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	newItem := TodoItem{
		ID:        fmt.Sprintf("todo-%d_%s", now.UnixNano(), uuid.New().String()[:8]),
		Content:   input.Content,
		Status:    TodoStatusPending,
		ParentID:  input.ParentID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	todoList.Todos = append(todoList.Todos, newItem)
	todoList.UpdatedAt = now

	// 更新缓存
	s.mu.Lock()
	s.cache[sessionID] = todoList
	s.mu.Unlock()

	// 持久化
	if s.db != nil {
		data, _ := json.Marshal(todoList)
		s.db.WithContext(ctx).Model(&TodoRecord{}).
			Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).
			Update("data", data)
	}

	s.logger.Debug("添加 TODO 项", zap.String("session_id", sessionID))
	return todoList, nil
}

// DeleteTodoItem 删除任务项
func (s *TodoService) DeleteTodoItem(ctx context.Context, tenantID, sessionID, todoID string) (*TodoList, error) {
	todoList, err := s.GetTodoList(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	if todoList == nil {
		return nil, errors.New("TODO 列表不存在")
	}

	// 删除任务及其子任务
	filtered := make([]TodoItem, 0)
	for _, item := range todoList.Todos {
		if item.ID != todoID && (item.ParentID == nil || *item.ParentID != todoID) {
			filtered = append(filtered, item)
		}
	}

	todoList.Todos = filtered
	todoList.UpdatedAt = time.Now()

	// 更新缓存
	s.mu.Lock()
	s.cache[sessionID] = todoList
	s.mu.Unlock()

	// 持久化
	if s.db != nil {
		data, _ := json.Marshal(todoList)
		s.db.WithContext(ctx).Model(&TodoRecord{}).
			Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).
			Update("data", data)
	}

	s.logger.Debug("删除 TODO 项", zap.String("todo_id", todoID))
	return todoList, nil
}

// DeleteTodoList 删除整个任务列表
func (s *TodoService) DeleteTodoList(ctx context.Context, tenantID, sessionID string) error {
	s.mu.Lock()
	delete(s.cache, sessionID)
	s.mu.Unlock()

	if s.db != nil {
		s.db.WithContext(ctx).Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).Delete(&TodoRecord{})
	}

	s.logger.Debug("删除 TODO 列表", zap.String("session_id", sessionID))
	return nil
}

// GetProgress 获取进度
func (s *TodoService) GetProgress(ctx context.Context, tenantID, sessionID string) (completed, total int, err error) {
	todoList, err := s.GetTodoList(ctx, tenantID, sessionID)
	if err != nil || todoList == nil {
		return 0, 0, err
	}

	total = len(todoList.Todos)
	for _, item := range todoList.Todos {
		if item.Status == TodoStatusCompleted {
			completed++
		}
	}

	return completed, total, nil
}
