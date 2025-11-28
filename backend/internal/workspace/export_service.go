package workspace

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportService 产出物导出服务
type ExportService struct {
	fileStore    FileStore
	tempDir      string
	maxExportSize int64 // 最大导出大小（字节）
}

// FileStore 文件存储接口
type FileStore interface {
	GetFile(ctx context.Context, fileID string) (*File, io.ReadCloser, error)
	ListFiles(ctx context.Context, workspaceID string, filter FileFilter) ([]*File, error)
}

// File 文件信息
type File struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	MimeType    string    `json:"mime_type"`
	WorkspaceID string    `json:"workspace_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FileFilter 文件过滤
type FileFilter struct {
	Path      string
	MimeTypes []string
	After     *time.Time
	Before    *time.Time
}

// ExportConfig 导出配置
type ExportConfig struct {
	TempDir       string
	MaxExportSize int64 // 默认 1GB
}

// NewExportService 创建导出服务
func NewExportService(store FileStore, config ExportConfig) *ExportService {
	if config.TempDir == "" {
		config.TempDir = os.TempDir()
	}
	if config.MaxExportSize <= 0 {
		config.MaxExportSize = 1 << 30 // 1GB
	}
	return &ExportService{
		fileStore:     store,
		tempDir:       config.TempDir,
		maxExportSize: config.MaxExportSize,
	}
}

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatZIP      ExportFormat = "zip"
	FormatJSON     ExportFormat = "json"
	FormatMarkdown ExportFormat = "markdown"
)

// ExportRequest 导出请求
type ExportRequest struct {
	WorkspaceID string       `json:"workspace_id"`
	FileIDs     []string     `json:"file_ids,omitempty"`    // 指定文件，为空则导出全部
	Path        string       `json:"path,omitempty"`        // 指定路径
	Format      ExportFormat `json:"format"`
	IncludeMeta bool         `json:"include_meta"`          // 是否包含元信息
}

// ExportResult 导出结果
type ExportResult struct {
	FilePath    string    `json:"file_path"`
	FileName    string    `json:"file_name"`
	Size        int64     `json:"size"`
	FileCount   int       `json:"file_count"`
	Format      string    `json:"format"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Export 导出产出物
func (s *ExportService) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	// 获取要导出的文件列表
	files, err := s.getFilesToExport(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files to export")
	}

	// 检查总大小
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}
	if totalSize > s.maxExportSize {
		return nil, fmt.Errorf("export size exceeds limit: %d > %d", totalSize, s.maxExportSize)
	}

	// 根据格式导出
	switch req.Format {
	case FormatZIP:
		return s.exportAsZip(ctx, req.WorkspaceID, files, req.IncludeMeta)
	case FormatJSON:
		return s.exportAsJSON(ctx, req.WorkspaceID, files)
	case FormatMarkdown:
		return s.exportAsMarkdown(ctx, req.WorkspaceID, files)
	default:
		return nil, fmt.Errorf("unsupported format: %s", req.Format)
	}
}

func (s *ExportService) getFilesToExport(ctx context.Context, req *ExportRequest) ([]*File, error) {
	if len(req.FileIDs) > 0 {
		// 指定文件列表
		files := make([]*File, 0, len(req.FileIDs))
		for _, id := range req.FileIDs {
			file, _, err := s.fileStore.GetFile(ctx, id)
			if err != nil {
				continue
			}
			files = append(files, file)
		}
		return files, nil
	}

	// 按条件查询
	filter := FileFilter{
		Path: req.Path,
	}
	return s.fileStore.ListFiles(ctx, req.WorkspaceID, filter)
}

func (s *ExportService) exportAsZip(ctx context.Context, workspaceID string, files []*File, includeMeta bool) (*ExportResult, error) {
	// 创建临时文件
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("export_%s_%s.zip", workspaceID, timestamp)
	filePath := filepath.Join(s.tempDir, fileName)

	zipFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 添加文件
	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		_, reader, err := s.fileStore.GetFile(ctx, file.ID)
		if err != nil {
			continue
		}

		// 创建 zip 条目
		path := file.Path
		if path == "" {
			path = file.Name
		}
		
		header := &zip.FileHeader{
			Name:     path,
			Method:   zip.Deflate,
			Modified: file.UpdatedAt,
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			reader.Close()
			continue
		}

		_, err = io.Copy(writer, reader)
		reader.Close()
		if err != nil {
			continue
		}
	}

	// 添加元信息
	if includeMeta {
		meta := map[string]any{
			"workspace_id": workspaceID,
			"export_time":  time.Now(),
			"file_count":   len(files),
			"files":        files,
		}
		metaJSON, _ := json.MarshalIndent(meta, "", "  ")
		
		metaWriter, err := zipWriter.Create("_metadata.json")
		if err == nil {
			metaWriter.Write(metaJSON)
		}
	}

	zipWriter.Close()

	// 获取文件大小
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	return &ExportResult{
		FilePath:  filePath,
		FileName:  fileName,
		Size:      stat.Size(),
		FileCount: len(files),
		Format:    string(FormatZIP),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 小时后过期
	}, nil
}

func (s *ExportService) exportAsJSON(ctx context.Context, workspaceID string, files []*File) (*ExportResult, error) {
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("export_%s_%s.json", workspaceID, timestamp)
	filePath := filepath.Join(s.tempDir, fileName)

	// 构建导出数据
	exportData := map[string]any{
		"workspace_id": workspaceID,
		"export_time":  time.Now(),
		"files":        make([]map[string]any, 0, len(files)),
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		_, reader, err := s.fileStore.GetFile(ctx, file.ID)
		if err != nil {
			continue
		}

		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			continue
		}

		fileData := map[string]any{
			"id":         file.ID,
			"name":       file.Name,
			"path":       file.Path,
			"mime_type":  file.MimeType,
			"size":       file.Size,
			"created_at": file.CreatedAt,
			"updated_at": file.UpdatedAt,
		}

		// 文本文件直接嵌入内容
		if s.isTextFile(file.MimeType) {
			fileData["content"] = string(content)
		} else {
			fileData["content_base64"] = content // 二进制文件 base64 编码
		}

		exportData["files"] = append(exportData["files"].([]map[string]any), fileData)
	}

	// 写入文件
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return nil, err
	}

	return &ExportResult{
		FilePath:  filePath,
		FileName:  fileName,
		Size:      int64(len(jsonData)),
		FileCount: len(files),
		Format:    string(FormatJSON),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil
}

func (s *ExportService) exportAsMarkdown(ctx context.Context, workspaceID string, files []*File) (*ExportResult, error) {
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("export_%s_%s.md", workspaceID, timestamp)
	filePath := filepath.Join(s.tempDir, fileName)

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# Workspace Export: %s\n\n", workspaceID))
	md.WriteString(fmt.Sprintf("Export Time: %s\n\n", time.Now().Format(time.RFC3339)))
	md.WriteString(fmt.Sprintf("Total Files: %d\n\n", len(files)))
	md.WriteString("---\n\n")

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		md.WriteString(fmt.Sprintf("## %s\n\n", file.Name))
		md.WriteString(fmt.Sprintf("- **Path:** `%s`\n", file.Path))
		md.WriteString(fmt.Sprintf("- **Size:** %d bytes\n", file.Size))
		md.WriteString(fmt.Sprintf("- **Type:** %s\n", file.MimeType))
		md.WriteString(fmt.Sprintf("- **Updated:** %s\n\n", file.UpdatedAt.Format(time.RFC3339)))

		// 只展示文本文件内容
		if s.isTextFile(file.MimeType) {
			_, reader, err := s.fileStore.GetFile(ctx, file.ID)
			if err == nil {
				content, _ := io.ReadAll(reader)
				reader.Close()

				// 限制内容长度
				contentStr := string(content)
				if len(contentStr) > 10000 {
					contentStr = contentStr[:10000] + "\n\n... (truncated)"
				}

				// 代码块
				lang := s.getLanguageFromMime(file.MimeType)
				md.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", lang, contentStr))
			}
		} else {
			md.WriteString("*Binary file, content not displayed*\n\n")
		}

		md.WriteString("---\n\n")
	}

	// 写入文件
	if err := os.WriteFile(filePath, []byte(md.String()), 0644); err != nil {
		return nil, err
	}

	stat, _ := os.Stat(filePath)

	return &ExportResult{
		FilePath:  filePath,
		FileName:  fileName,
		Size:      stat.Size(),
		FileCount: len(files),
		Format:    string(FormatMarkdown),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil
}

func (s *ExportService) isTextFile(mimeType string) bool {
	textTypes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-yaml",
	}
	for _, t := range textTypes {
		if strings.HasPrefix(mimeType, t) {
			return true
		}
	}
	return false
}

func (s *ExportService) getLanguageFromMime(mimeType string) string {
	mimeToLang := map[string]string{
		"text/plain":              "",
		"text/markdown":           "markdown",
		"text/html":               "html",
		"text/css":                "css",
		"text/javascript":         "javascript",
		"application/json":        "json",
		"application/xml":         "xml",
		"application/x-yaml":      "yaml",
		"text/x-python":           "python",
		"text/x-go":               "go",
		"text/x-java":             "java",
	}
	if lang, ok := mimeToLang[mimeType]; ok {
		return lang
	}
	return ""
}

// CleanExpiredExports 清理过期的导出文件
func (s *ExportService) CleanExpiredExports() (int, error) {
	files, err := os.ReadDir(s.tempDir)
	if err != nil {
		return 0, err
	}

	count := 0
	cutoff := time.Now().Add(-24 * time.Hour)

	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "export_") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(s.tempDir, file.Name())
			if err := os.Remove(path); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// GetExportFile 获取导出文件（用于下载）
func (s *ExportService) GetExportFile(filePath string) (*os.File, error) {
	// 安全检查：确保文件在临时目录内
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	tempAbs, _ := filepath.Abs(s.tempDir)
	if !strings.HasPrefix(absPath, tempAbs) {
		return nil, fmt.Errorf("invalid file path")
	}

	return os.Open(filePath)
}
