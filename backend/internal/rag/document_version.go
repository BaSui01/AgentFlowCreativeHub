package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// DocumentVersionService 文档版本管理服务
type DocumentVersionService struct {
	store    DocumentVersionStore
	maxVer   int
	mu       sync.RWMutex
}

// DocumentVersionStore 文档版本存储接口
type DocumentVersionStore interface {
	SaveVersion(ctx context.Context, version *DocumentVersion) error
	GetVersion(ctx context.Context, docID string, version int) (*DocumentVersion, error)
	ListVersions(ctx context.Context, docID string) ([]*DocumentVersion, error)
	GetLatestVersion(ctx context.Context, docID string) (*DocumentVersion, error)
	DeleteVersion(ctx context.Context, docID string, version int) error
	DeleteAllVersions(ctx context.Context, docID string) error
}

// DocumentVersion 文档版本
type DocumentVersion struct {
	ID          string         `json:"id"`
	DocumentID  string         `json:"document_id"`
	Version     int            `json:"version"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	ContentHash string         `json:"content_hash"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	ChunkCount  int            `json:"chunk_count"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	Comment     string         `json:"comment,omitempty"`
	Size        int64          `json:"size"`
}

// NewDocumentVersionService 创建文档版本服务
func NewDocumentVersionService(store DocumentVersionStore, maxVersions int) *DocumentVersionService {
	if maxVersions <= 0 {
		maxVersions = 20
	}
	return &DocumentVersionService{
		store:  store,
		maxVer: maxVersions,
	}
}

// CreateVersion 创建新版本
func (s *DocumentVersionService) CreateVersion(ctx context.Context, doc *Document, userID, comment string) (*DocumentVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取最新版本号
	latest, _ := s.store.GetLatestVersion(ctx, doc.ID)
	nextVersion := 1
	if latest != nil {
		nextVersion = latest.Version + 1
	}

	// 计算内容哈希
	hash := sha256.Sum256([]byte(doc.Content))
	contentHash := hex.EncodeToString(hash[:])

	// 检查是否有变更
	if latest != nil && latest.ContentHash == contentHash {
		return nil, fmt.Errorf("no changes detected")
	}

	version := &DocumentVersion{
		ID:          fmt.Sprintf("%s_v%d", doc.ID, nextVersion),
		DocumentID:  doc.ID,
		Version:     nextVersion,
		Title:       doc.Title,
		Content:     doc.Content,
		ContentHash: contentHash,
		Metadata:    doc.Metadata,
		ChunkCount:  doc.ChunkCount,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		Comment:     comment,
		Size:        int64(len(doc.Content)),
	}

	if err := s.store.SaveVersion(ctx, version); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	// 清理旧版本
	go s.cleanOldVersions(context.Background(), doc.ID)

	return version, nil
}

// Document 文档结构（简化版）
type Document struct {
	ID         string
	Title      string
	Content    string
	Metadata   map[string]any
	ChunkCount int
}

// GetVersion 获取指定版本
func (s *DocumentVersionService) GetVersion(ctx context.Context, docID string, version int) (*DocumentVersion, error) {
	return s.store.GetVersion(ctx, docID, version)
}

// GetLatestVersion 获取最新版本
func (s *DocumentVersionService) GetLatestVersion(ctx context.Context, docID string) (*DocumentVersion, error) {
	return s.store.GetLatestVersion(ctx, docID)
}

// ListVersions 列出所有版本
func (s *DocumentVersionService) ListVersions(ctx context.Context, docID string) ([]*DocumentVersion, error) {
	return s.store.ListVersions(ctx, docID)
}

// RestoreVersion 恢复到指定版本
func (s *DocumentVersionService) RestoreVersion(ctx context.Context, docID string, version int, userID string) (*DocumentVersion, error) {
	// 获取目标版本
	target, err := s.store.GetVersion(ctx, docID, version)
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}

	// 创建新版本（内容来自目标版本）
	doc := &Document{
		ID:         docID,
		Title:      target.Title,
		Content:    target.Content,
		Metadata:   target.Metadata,
		ChunkCount: target.ChunkCount,
	}

	comment := fmt.Sprintf("Restored from version %d", version)
	return s.CreateVersion(ctx, doc, userID, comment)
}

// CompareVersions 比较两个版本
func (s *DocumentVersionService) CompareVersions(ctx context.Context, docID string, v1, v2 int) (*VersionDiff, error) {
	ver1, err := s.store.GetVersion(ctx, docID, v1)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", v1, err)
	}

	ver2, err := s.store.GetVersion(ctx, docID, v2)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", v2, err)
	}

	diff := &VersionDiff{
		DocumentID: docID,
		FromVer:    v1,
		ToVer:      v2,
		Changes:    make([]DiffChange, 0),
	}

	// 标题变更
	if ver1.Title != ver2.Title {
		diff.Changes = append(diff.Changes, DiffChange{
			Field:    "title",
			OldValue: ver1.Title,
			NewValue: ver2.Title,
		})
	}

	// 内容变更（简单 diff）
	if ver1.Content != ver2.Content {
		diff.ContentChanged = true
		diff.OldSize = ver1.Size
		diff.NewSize = ver2.Size
		diff.SizeDiff = ver2.Size - ver1.Size

		// 简单的行级 diff
		diff.LineDiff = s.simpleLineDiff(ver1.Content, ver2.Content)
	}

	// 元数据变更
	diff.MetaChanges = s.diffMetadata(ver1.Metadata, ver2.Metadata)

	return diff, nil
}

// VersionDiff 版本差异
type VersionDiff struct {
	DocumentID     string                 `json:"document_id"`
	FromVer        int                    `json:"from_version"`
	ToVer          int                    `json:"to_version"`
	Changes        []DiffChange           `json:"changes"`
	ContentChanged bool                   `json:"content_changed"`
	OldSize        int64                  `json:"old_size"`
	NewSize        int64                  `json:"new_size"`
	SizeDiff       int64                  `json:"size_diff"`
	LineDiff       *LineDiffResult        `json:"line_diff,omitempty"`
	MetaChanges    map[string]DiffChange  `json:"meta_changes,omitempty"`
}

// DiffChange 差异项
type DiffChange struct {
	Field    string `json:"field"`
	OldValue any    `json:"old_value,omitempty"`
	NewValue any    `json:"new_value,omitempty"`
}

// LineDiffResult 行级差异
type LineDiffResult struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
}

func (s *DocumentVersionService) simpleLineDiff(old, new string) *LineDiffResult {
	oldLines := splitLines(old)
	newLines := splitLines(new)

	oldSet := make(map[string]bool)
	for _, l := range oldLines {
		oldSet[l] = true
	}

	newSet := make(map[string]bool)
	for _, l := range newLines {
		newSet[l] = true
	}

	var added, removed int
	for l := range newSet {
		if !oldSet[l] {
			added++
		}
	}
	for l := range oldSet {
		if !newSet[l] {
			removed++
		}
	}

	return &LineDiffResult{
		Added:   added,
		Removed: removed,
		Changed: added + removed,
	}
}

func splitLines(s string) []string {
	var lines []string
	var line string
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, line)
			line = ""
		} else {
			line += string(c)
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (s *DocumentVersionService) diffMetadata(old, new map[string]any) map[string]DiffChange {
	changes := make(map[string]DiffChange)

	// 检查删除和修改
	for k, oldVal := range old {
		if newVal, ok := new[k]; ok {
			if fmt.Sprintf("%v", oldVal) != fmt.Sprintf("%v", newVal) {
				changes[k] = DiffChange{
					Field:    k,
					OldValue: oldVal,
					NewValue: newVal,
				}
			}
		} else {
			changes[k] = DiffChange{
				Field:    k,
				OldValue: oldVal,
			}
		}
	}

	// 检查新增
	for k, newVal := range new {
		if _, ok := old[k]; !ok {
			changes[k] = DiffChange{
				Field:    k,
				NewValue: newVal,
			}
		}
	}

	return changes
}

func (s *DocumentVersionService) cleanOldVersions(ctx context.Context, docID string) {
	versions, err := s.store.ListVersions(ctx, docID)
	if err != nil || len(versions) <= s.maxVer {
		return
	}

	// 删除最旧的版本
	for i := 0; i < len(versions)-s.maxVer; i++ {
		_ = s.store.DeleteVersion(ctx, docID, versions[i].Version)
	}
}

// DeleteDocumentVersions 删除文档的所有版本
func (s *DocumentVersionService) DeleteDocumentVersions(ctx context.Context, docID string) error {
	return s.store.DeleteAllVersions(ctx, docID)
}

// GetVersionHistory 获取版本历史摘要
func (s *DocumentVersionService) GetVersionHistory(ctx context.Context, docID string) ([]VersionSummary, error) {
	versions, err := s.store.ListVersions(ctx, docID)
	if err != nil {
		return nil, err
	}

	summaries := make([]VersionSummary, len(versions))
	for i, v := range versions {
		summaries[i] = VersionSummary{
			Version:   v.Version,
			CreatedBy: v.CreatedBy,
			CreatedAt: v.CreatedAt,
			Comment:   v.Comment,
			Size:      v.Size,
		}
	}

	return summaries, nil
}

// VersionSummary 版本摘要
type VersionSummary struct {
	Version   int       `json:"version"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Comment   string    `json:"comment,omitempty"`
	Size      int64     `json:"size"`
}
