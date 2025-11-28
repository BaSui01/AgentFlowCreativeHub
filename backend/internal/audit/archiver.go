package audit

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Archiver 审计日志归档器
type Archiver struct {
	store         LogStore
	archivePath   string
	retentionDays int
	compressLevel int
	mu            sync.Mutex
}

// LogStore 日志存储接口
type LogStore interface {
	QueryLogs(ctx context.Context, filter LogFilter) ([]AuditLog, error)
	DeleteLogs(ctx context.Context, before time.Time) (int64, error)
	GetOldestLog(ctx context.Context) (*AuditLog, error)
}

// LogFilter 日志过滤条件
type LogFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	UserID    string
	Action    string
	Resource  string
	Limit     int
	Offset    int
}

// AuditLog 审计日志
type AuditLog struct {
	ID         string         `json:"id"`
	Timestamp  time.Time      `json:"timestamp"`
	UserID     string         `json:"user_id"`
	Username   string         `json:"username"`
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	ResourceID string         `json:"resource_id"`
	Details    map[string]any `json:"details,omitempty"`
	IP         string         `json:"ip"`
	UserAgent  string         `json:"user_agent"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
}

// ArchiveConfig 归档配置
type ArchiveConfig struct {
	ArchivePath   string // 归档文件存储路径
	RetentionDays int    // 数据库保留天数
	CompressLevel int    // 压缩级别 (1-9)
}

// NewArchiver 创建归档器
func NewArchiver(store LogStore, config ArchiveConfig) *Archiver {
	if config.RetentionDays <= 0 {
		config.RetentionDays = 90 // 默认保留 90 天
	}
	if config.CompressLevel <= 0 || config.CompressLevel > 9 {
		config.CompressLevel = gzip.BestCompression
	}
	if config.ArchivePath == "" {
		config.ArchivePath = "./archive/audit"
	}

	return &Archiver{
		store:         store,
		archivePath:   config.ArchivePath,
		retentionDays: config.RetentionDays,
		compressLevel: config.CompressLevel,
	}
}

// ArchiveResult 归档结果
type ArchiveResult struct {
	ArchivedFiles []string      `json:"archived_files"`
	TotalLogs     int64         `json:"total_logs"`
	DeletedLogs   int64         `json:"deleted_logs"`
	StartDate     time.Time     `json:"start_date"`
	EndDate       time.Time     `json:"end_date"`
	Duration      time.Duration `json:"duration"`
	Errors        []string      `json:"errors,omitempty"`
}

// Archive 执行归档
func (a *Archiver) Archive(ctx context.Context) (*ArchiveResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	start := time.Now()
	result := &ArchiveResult{
		ArchivedFiles: make([]string, 0),
		Errors:        make([]string, 0),
	}

	// 计算归档截止时间
	cutoffTime := time.Now().AddDate(0, 0, -a.retentionDays)

	// 获取最旧的日志
	oldest, err := a.store.GetOldestLog(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get oldest log: %w", err)
	}
	if oldest == nil {
		result.Duration = time.Since(start)
		return result, nil // 无日志需要归档
	}

	result.StartDate = oldest.Timestamp

	// 按月归档
	currentMonth := time.Date(oldest.Timestamp.Year(), oldest.Timestamp.Month(), 1, 0, 0, 0, 0, time.UTC)
	endMonth := time.Date(cutoffTime.Year(), cutoffTime.Month(), 1, 0, 0, 0, 0, time.UTC)

	for currentMonth.Before(endMonth) {
		nextMonth := currentMonth.AddDate(0, 1, 0)

		// 查询该月日志
		filter := LogFilter{
			StartTime: &currentMonth,
			EndTime:   &nextMonth,
			Limit:     100000, // 每批最多 10 万条
		}

		logs, err := a.store.QueryLogs(ctx, filter)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("query %s: %v", currentMonth.Format("2006-01"), err))
			currentMonth = nextMonth
			continue
		}

		if len(logs) > 0 {
			// 归档到文件
			filename, err := a.archiveToFile(logs, currentMonth)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("archive %s: %v", currentMonth.Format("2006-01"), err))
			} else {
				result.ArchivedFiles = append(result.ArchivedFiles, filename)
				result.TotalLogs += int64(len(logs))
			}
		}

		currentMonth = nextMonth
	}

	result.EndDate = cutoffTime

	// 删除已归档的日志
	deleted, err := a.store.DeleteLogs(ctx, cutoffTime)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("delete: %v", err))
	} else {
		result.DeletedLogs = deleted
	}

	result.Duration = time.Since(start)
	return result, nil
}

// archiveToFile 归档日志到文件
func (a *Archiver) archiveToFile(logs []AuditLog, month time.Time) (string, error) {
	// 创建目录
	yearDir := filepath.Join(a.archivePath, month.Format("2006"))
	if err := os.MkdirAll(yearDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 文件名格式: audit_2024-01.json.gz
	filename := filepath.Join(yearDir, fmt.Sprintf("audit_%s.json.gz", month.Format("2006-01")))

	// 创建文件
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// 创建 gzip 写入器
	gzWriter, err := gzip.NewWriterLevel(file, a.compressLevel)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzWriter.Close()

	// 写入 JSON 数组
	encoder := json.NewEncoder(gzWriter)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(logs); err != nil {
		return "", fmt.Errorf("failed to encode logs: %w", err)
	}

	return filename, nil
}

// ListArchives 列出归档文件
func (a *Archiver) ListArchives() ([]ArchiveInfo, error) {
	var archives []ArchiveInfo

	err := filepath.Walk(a.archivePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".gz" {
			archives = append(archives, ArchiveInfo{
				Path:      path,
				Size:      info.Size(),
				ModTime:   info.ModTime(),
				Filename:  info.Name(),
			})
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// 按时间排序
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].ModTime.After(archives[j].ModTime)
	})

	return archives, nil
}

// ArchiveInfo 归档文件信息
type ArchiveInfo struct {
	Path     string    `json:"path"`
	Filename string    `json:"filename"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
}

// RestoreArchive 从归档文件恢复（用于审计查询）
func (a *Archiver) RestoreArchive(path string) ([]AuditLog, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	var logs []AuditLog
	decoder := json.NewDecoder(gzReader)
	if err := decoder.Decode(&logs); err != nil {
		return nil, fmt.Errorf("failed to decode logs: %w", err)
	}

	return logs, nil
}

// SearchArchives 搜索归档日志
func (a *Archiver) SearchArchives(ctx context.Context, filter LogFilter) ([]AuditLog, error) {
	archives, err := a.ListArchives()
	if err != nil {
		return nil, err
	}

	var results []AuditLog

	for _, archive := range archives {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		logs, err := a.RestoreArchive(archive.Path)
		if err != nil {
			continue
		}

		// 过滤
		for _, log := range logs {
			if a.matchFilter(log, filter) {
				results = append(results, log)

				if filter.Limit > 0 && len(results) >= filter.Limit {
					return results, nil
				}
			}
		}
	}

	return results, nil
}

func (a *Archiver) matchFilter(log AuditLog, filter LogFilter) bool {
	if filter.StartTime != nil && log.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && log.Timestamp.After(*filter.EndTime) {
		return false
	}
	if filter.UserID != "" && log.UserID != filter.UserID {
		return false
	}
	if filter.Action != "" && log.Action != filter.Action {
		return false
	}
	if filter.Resource != "" && log.Resource != filter.Resource {
		return false
	}
	return true
}

// CleanOldArchives 清理过期归档文件
func (a *Archiver) CleanOldArchives(maxAge time.Duration) ([]string, error) {
	archives, err := a.ListArchives()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-maxAge)
	var deleted []string

	for _, archive := range archives {
		if archive.ModTime.Before(cutoff) {
			if err := os.Remove(archive.Path); err == nil {
				deleted = append(deleted, archive.Path)
			}
		}
	}

	return deleted, nil
}

// GetArchiveStats 获取归档统计
func (a *Archiver) GetArchiveStats() (*ArchiveStats, error) {
	archives, err := a.ListArchives()
	if err != nil {
		return nil, err
	}

	stats := &ArchiveStats{
		TotalFiles:      len(archives),
		RetentionDays:   a.retentionDays,
		ArchivePath:     a.archivePath,
	}

	for _, archive := range archives {
		stats.TotalSize += archive.Size
	}

	if len(archives) > 0 {
		stats.OldestArchive = &archives[len(archives)-1].ModTime
		stats.NewestArchive = &archives[0].ModTime
	}

	return stats, nil
}

// ArchiveStats 归档统计
type ArchiveStats struct {
	TotalFiles    int        `json:"total_files"`
	TotalSize     int64      `json:"total_size"`
	RetentionDays int        `json:"retention_days"`
	ArchivePath   string     `json:"archive_path"`
	OldestArchive *time.Time `json:"oldest_archive,omitempty"`
	NewestArchive *time.Time `json:"newest_archive,omitempty"`
}

// ExportToWriter 导出归档到 io.Writer（用于下载）
func (a *Archiver) ExportToWriter(path string, w io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	return err
}
