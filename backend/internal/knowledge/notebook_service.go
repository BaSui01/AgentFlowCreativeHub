package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"backend/internal/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// NotebookEntry 代码备忘录条目
type NotebookEntry struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenant_id" gorm:"type:uuid;not null;index"`
	FilePath  string    `json:"file_path" gorm:"size:500;not null;index"`
	Note      string    `json:"note" gorm:"type:text;not null"`
	Tags      []string  `json:"tags,omitempty" gorm:"type:jsonb;serializer:json"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (NotebookEntry) TableName() string {
	return "notebook_entries"
}

// NotebookService 代码备忘录服务
// 用于记录脆弱代码、注意事项等
type NotebookService struct {
	db     *gorm.DB
	cache  map[string][]NotebookEntry // filePath -> entries
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewNotebookService 创建备忘录服务
func NewNotebookService(db *gorm.DB) *NotebookService {
	return &NotebookService{
		db:     db,
		cache:  make(map[string][]NotebookEntry),
		logger: logger.Get(),
	}
}

// AutoMigrate 自动迁移
func (s *NotebookService) AutoMigrate() error {
	if s.db == nil {
		return nil
	}
	return s.db.AutoMigrate(&NotebookEntry{})
}

// AddNotebook 添加备忘录
func (s *NotebookService) AddNotebook(ctx context.Context, tenantID, filePath, note string, tags []string) (*NotebookEntry, error) {
	if filePath == "" || note == "" {
		return nil, errors.New("文件路径和备忘内容不能为空")
	}

	entry := &NotebookEntry{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		FilePath:  filePath,
		Note:      note,
		Tags:      tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 保存到数据库
	if s.db != nil {
		if err := s.db.WithContext(ctx).Create(entry).Error; err != nil {
			return nil, err
		}
	}

	// 更新缓存
	s.mu.Lock()
	s.cache[filePath] = append(s.cache[filePath], *entry)
	s.mu.Unlock()

	s.logger.Info("添加代码备忘录", zap.String("file_path", filePath))
	return entry, nil
}

// QueryNotebook 查询备忘录
func (s *NotebookService) QueryNotebook(ctx context.Context, tenantID, filePathPattern string, topN int) ([]NotebookEntry, error) {
	if topN <= 0 {
		topN = 10
	}
	if topN > 50 {
		topN = 50
	}

	var entries []NotebookEntry

	if s.db != nil {
		query := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
		if filePathPattern != "" {
			query = query.Where("file_path LIKE ?", "%"+filePathPattern+"%")
		}
		if err := query.Order("created_at DESC").Limit(topN).Find(&entries).Error; err != nil {
			return nil, err
		}
		return entries, nil
	}

	// 从缓存查询
	s.mu.RLock()
	defer s.mu.RUnlock()

	for filePath, fileEntries := range s.cache {
		if filePathPattern == "" || strings.Contains(filePath, filePathPattern) {
			entries = append(entries, fileEntries...)
		}
	}

	// 按时间排序
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].CreatedAt.After(entries[i].CreatedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	if len(entries) > topN {
		entries = entries[:topN]
	}

	return entries, nil
}

// UpdateNotebook 更新备忘录
func (s *NotebookService) UpdateNotebook(ctx context.Context, tenantID, notebookID, newNote string) (*NotebookEntry, error) {
	if notebookID == "" || newNote == "" {
		return nil, errors.New("备忘录 ID 和内容不能为空")
	}

	if s.db != nil {
		var entry NotebookEntry
		if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", notebookID, tenantID).First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("备忘录不存在")
			}
			return nil, err
		}

		entry.Note = newNote
		entry.UpdatedAt = time.Now()

		if err := s.db.WithContext(ctx).Save(&entry).Error; err != nil {
			return nil, err
		}

		// 更新缓存
		s.mu.Lock()
		if entries, ok := s.cache[entry.FilePath]; ok {
			for i := range entries {
				if entries[i].ID == notebookID {
					entries[i] = entry
					break
				}
			}
		}
		s.mu.Unlock()

		return &entry, nil
	}

	return nil, errors.New("数据库未配置")
}

// DeleteNotebook 删除备忘录
func (s *NotebookService) DeleteNotebook(ctx context.Context, tenantID, notebookID string) error {
	if notebookID == "" {
		return errors.New("备忘录 ID 不能为空")
	}

	if s.db != nil {
		// 先查询获取 filePath
		var entry NotebookEntry
		if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", notebookID, tenantID).First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("备忘录不存在")
			}
			return err
		}

		if err := s.db.WithContext(ctx).Delete(&entry).Error; err != nil {
			return err
		}

		// 更新缓存
		s.mu.Lock()
		if entries, ok := s.cache[entry.FilePath]; ok {
			filtered := make([]NotebookEntry, 0)
			for _, e := range entries {
				if e.ID != notebookID {
					filtered = append(filtered, e)
				}
			}
			s.cache[entry.FilePath] = filtered
		}
		s.mu.Unlock()

		s.logger.Info("删除代码备忘录", zap.String("id", notebookID))
		return nil
	}

	return errors.New("数据库未配置")
}

// GetNotebooksByFile 获取指定文件的所有备忘录
func (s *NotebookService) GetNotebooksByFile(ctx context.Context, tenantID, filePath string) ([]NotebookEntry, error) {
	if filePath == "" {
		return nil, errors.New("文件路径不能为空")
	}

	if s.db != nil {
		var entries []NotebookEntry
		if err := s.db.WithContext(ctx).
			Where("tenant_id = ? AND file_path = ?", tenantID, filePath).
			Order("created_at ASC").
			Find(&entries).Error; err != nil {
			return nil, err
		}
		return entries, nil
	}

	// 从缓存获取
	s.mu.RLock()
	defer s.mu.RUnlock()
	if entries, ok := s.cache[filePath]; ok {
		return entries, nil
	}

	return []NotebookEntry{}, nil
}

// GetNotebookCount 获取备忘录数量
func (s *NotebookService) GetNotebookCount(ctx context.Context, tenantID string) (int64, error) {
	if s.db != nil {
		var count int64
		if err := s.db.WithContext(ctx).Model(&NotebookEntry{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
			return 0, err
		}
		return count, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, entries := range s.cache {
		count += int64(len(entries))
	}
	return count, nil
}

// ExportNotebooks 导出备忘录
func (s *NotebookService) ExportNotebooks(ctx context.Context, tenantID string) ([]byte, error) {
	entries, err := s.QueryNotebook(ctx, tenantID, "", 1000)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(entries, "", "  ")
}

// ImportNotebooks 导入备忘录
func (s *NotebookService) ImportNotebooks(ctx context.Context, tenantID string, data []byte) (int, error) {
	var entries []NotebookEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, err
	}

	imported := 0
	for _, entry := range entries {
		entry.ID = uuid.New().String()
		entry.TenantID = tenantID
		entry.CreatedAt = time.Now()
		entry.UpdatedAt = time.Now()

		if s.db != nil {
			if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
				continue
			}
		}

		s.mu.Lock()
		s.cache[entry.FilePath] = append(s.cache[entry.FilePath], entry)
		s.mu.Unlock()

		imported++
	}

	s.logger.Info("导入代码备忘录", zap.Int("count", imported))
	return imported, nil
}
