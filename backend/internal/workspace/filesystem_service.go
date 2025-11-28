package workspace

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"backend/internal/logger"

	"go.uber.org/zap"
)

// FilesystemService 文件系统服务
type FilesystemService struct {
	basePath     string
	maxFileSize  int64
	allowedExts  map[string]bool
	logger       *zap.Logger
}

// FileReadResult 文件读取结果
type FileReadResult struct {
	Content    string `json:"content"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	TotalLines int    `json:"total_lines"`
	IsImage    bool   `json:"is_image,omitempty"`
	MimeType   string `json:"mime_type,omitempty"`
}

// FileEditResult 文件编辑结果
type FileEditResult struct {
	Success     bool     `json:"success"`
	Message     string   `json:"message"`
	BeforeLines []string `json:"before_lines,omitempty"`
	AfterLines  []string `json:"after_lines,omitempty"`
	FilePath    string   `json:"file_path"`
}

// NewFilesystemService 创建文件系统服务
func NewFilesystemService(basePath string) *FilesystemService {
	return &FilesystemService{
		basePath:    basePath,
		maxFileSize: 10 * 1024 * 1024, // 10MB
		allowedExts: map[string]bool{
			".txt": true, ".md": true, ".json": true, ".yaml": true, ".yml": true,
			".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
			".py": true, ".java": true, ".rs": true, ".cpp": true, ".c": true,
			".h": true, ".hpp": true, ".cs": true, ".rb": true, ".php": true,
			".html": true, ".css": true, ".scss": true, ".less": true,
			".sql": true, ".sh": true, ".bash": true, ".zsh": true,
			".xml": true, ".toml": true, ".ini": true, ".env": true,
		},
		logger: logger.Get(),
	}
}

// imageMimeTypes 图片 MIME 类型映射
var imageMimeTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp",
	".svg": "image/svg+xml", ".ico": "image/x-icon",
}

// isImageFile 检查是否是图片文件
func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := imageMimeTypes[ext]
	return ok
}

// ReadFile 读取文件内容
func (s *FilesystemService) ReadFile(ctx context.Context, filePath string, startLine, endLine int) (*FileReadResult, error) {
	fullPath := s.resolvePath(filePath)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("文件不存在: %s", filePath)
		}
		return nil, err
	}

	if info.IsDir() {
		return s.listDirectory(fullPath, filePath)
	}

	// 处理图片文件
	if isImageFile(fullPath) {
		return s.readImageFile(fullPath, filePath)
	}

	// 检查文件大小
	if info.Size() > s.maxFileSize {
		return nil, fmt.Errorf("文件过大: %d bytes (最大 %d bytes)", info.Size(), s.maxFileSize)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine > totalLines {
		endLine = totalLines
	}
	if startLine > totalLines {
		startLine = totalLines
	}
	if endLine < startLine {
		endLine = startLine
	}

	selectedLines := lines[startLine-1 : endLine]
	numberedLines := make([]string, len(selectedLines))
	for i, line := range selectedLines {
		lineNum := startLine + i
		numberedLines[i] = fmt.Sprintf("%d→%s", lineNum, line)
	}

	return &FileReadResult{
		Content:    strings.Join(numberedLines, "\n"),
		StartLine:  startLine,
		EndLine:    endLine,
		TotalLines: totalLines,
	}, nil
}

// readImageFile 读取图片文件
func (s *FilesystemService) readImageFile(fullPath, filePath string) (*FileReadResult, error) {
	ext := strings.ToLower(filepath.Ext(fullPath))
	mimeType := imageMimeTypes[ext]

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	base64Data := base64.StdEncoding.EncodeToString(data)

	return &FileReadResult{
		Content:  base64Data,
		IsImage:  true,
		MimeType: mimeType,
	}, nil
}

// listDirectory 列出目录内容
func (s *FilesystemService) listDirectory(fullPath, relativePath string) (*FileReadResult, error) {
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("📁 Directory: %s", relativePath))
	lines = append(lines, "")

	for _, entry := range entries {
		prefix := "📄"
		if entry.IsDir() {
			prefix = "📁"
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, entry.Name()))
	}

	return &FileReadResult{
		Content:    strings.Join(lines, "\n"),
		TotalLines: len(lines),
		StartLine:  1,
		EndLine:    len(lines),
	}, nil
}

// CreateFile 创建新文件
func (s *FilesystemService) CreateFile(ctx context.Context, filePath, content string, createDirs bool) error {
	fullPath := s.resolvePath(filePath)

	// 检查文件是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("文件已存在: %s", filePath)
	}

	// 创建父目录
	if createDirs {
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	s.logger.Info("创建文件", zap.String("path", filePath))
	return nil
}

// EditFileBySearch 通过搜索替换编辑文件
func (s *FilesystemService) EditFileBySearch(ctx context.Context, filePath, searchContent, replaceContent string, occurrence int) (*FileEditResult, error) {
	fullPath := s.resolvePath(filePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	originalContent := string(content)
	
	// 规范化换行符
	normalizedSearch := strings.ReplaceAll(searchContent, "\r\n", "\n")
	normalizedContent := strings.ReplaceAll(originalContent, "\r\n", "\n")

	// 查找匹配
	matches := findAllMatches(normalizedContent, normalizedSearch)
	if len(matches) == 0 {
		// 尝试模糊匹配
		matches = findFuzzyMatches(normalizedContent, normalizedSearch, 0.6)
		if len(matches) == 0 {
			return &FileEditResult{
				Success:  false,
				Message:  fmt.Sprintf("未找到匹配内容。搜索内容长度: %d 字符", len(searchContent)),
				FilePath: filePath,
			}, nil
		}
	}

	// 选择要替换的匹配
	if occurrence <= 0 {
		occurrence = 1
	}
	if occurrence > len(matches) {
		return &FileEditResult{
			Success:  false,
			Message:  fmt.Sprintf("只找到 %d 个匹配，但请求替换第 %d 个", len(matches), occurrence),
			FilePath: filePath,
		}, nil
	}

	match := matches[occurrence-1]
	
	// 执行替换
	newContent := normalizedContent[:match.start] + replaceContent + normalizedContent[match.end:]

	// 写入文件
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 计算上下文行
	beforeLines := getContextLines(normalizedContent, match.start, 3)
	afterLines := getContextLines(newContent, match.start, 3)

	s.logger.Info("编辑文件", zap.String("path", filePath), zap.Int("matches", len(matches)))

	return &FileEditResult{
		Success:     true,
		Message:     fmt.Sprintf("成功替换（共找到 %d 个匹配，替换第 %d 个）", len(matches), occurrence),
		BeforeLines: beforeLines,
		AfterLines:  afterLines,
		FilePath:    filePath,
	}, nil
}

// EditFileByLine 按行号编辑文件
func (s *FilesystemService) EditFileByLine(ctx context.Context, filePath string, startLine, endLine int, newContent string) (*FileEditResult, error) {
	fullPath := s.resolvePath(filePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	if startLine < 1 || endLine < startLine || startLine > totalLines {
		return &FileEditResult{
			Success:  false,
			Message:  fmt.Sprintf("无效的行号范围: %d-%d（文件共 %d 行）", startLine, endLine, totalLines),
			FilePath: filePath,
		}, nil
	}

	if endLine > totalLines {
		endLine = totalLines
	}

	// 构建新内容
	newLines := strings.Split(newContent, "\n")
	result := make([]string, 0, len(lines)-endLine+startLine+len(newLines))
	result = append(result, lines[:startLine-1]...)
	result = append(result, newLines...)
	result = append(result, lines[endLine:]...)

	// 写入文件
	if err := os.WriteFile(fullPath, []byte(strings.Join(result, "\n")), 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 获取上下文
	beforeLines := lines[max(0, startLine-4):min(totalLines, endLine+3)]
	afterStart := max(0, startLine-4)
	afterEnd := min(len(result), startLine+len(newLines)+2)
	afterLines := result[afterStart:afterEnd]

	s.logger.Info("编辑文件", zap.String("path", filePath), zap.Int("start", startLine), zap.Int("end", endLine))

	return &FileEditResult{
		Success:     true,
		Message:     fmt.Sprintf("成功替换行 %d-%d", startLine, endLine),
		BeforeLines: beforeLines,
		AfterLines:  afterLines,
		FilePath:    filePath,
	}, nil
}

// DeleteFile 删除文件
func (s *FilesystemService) DeleteFile(ctx context.Context, filePath string) error {
	fullPath := s.resolvePath(filePath)

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}

	s.logger.Info("删除文件", zap.String("path", filePath))
	return nil
}

// FileExists 检查文件是否存在
func (s *FilesystemService) FileExists(ctx context.Context, filePath string) bool {
	fullPath := s.resolvePath(filePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// GetFileInfo 获取文件信息
func (s *FilesystemService) GetFileInfo(ctx context.Context, filePath string) (map[string]any, error) {
	fullPath := s.resolvePath(filePath)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"name":         info.Name(),
		"size":         info.Size(),
		"is_dir":       info.IsDir(),
		"modified":     info.ModTime().Format(time.RFC3339),
		"is_image":     isImageFile(fullPath),
	}, nil
}

// resolvePath 解析路径
func (s *FilesystemService) resolvePath(filePath string) string {
	if filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(s.basePath, filePath)
}

// SetBasePath 设置基础路径
func (s *FilesystemService) SetBasePath(basePath string) {
	s.basePath = basePath
}

// 辅助类型和函数

type matchRange struct {
	start int
	end   int
	score float64
}

func findAllMatches(content, search string) []matchRange {
	matches := make([]matchRange, 0)
	start := 0
	for {
		idx := strings.Index(content[start:], search)
		if idx == -1 {
			break
		}
		absStart := start + idx
		matches = append(matches, matchRange{
			start: absStart,
			end:   absStart + len(search),
			score: 1.0,
		})
		start = absStart + 1
	}
	return matches
}

func findFuzzyMatches(content, search string, threshold float64) []matchRange {
	matches := make([]matchRange, 0)
	searchLines := strings.Split(search, "\n")
	contentLines := strings.Split(content, "\n")

	for i := 0; i <= len(contentLines)-len(searchLines); i++ {
		candidateLines := contentLines[i : i+len(searchLines)]
		candidate := strings.Join(candidateLines, "\n")
		
		score := calculateSimilarity(search, candidate)
		if score >= threshold {
			start := 0
			for j := 0; j < i; j++ {
				start += len(contentLines[j]) + 1
			}
			end := start + len(candidate)
			matches = append(matches, matchRange{start: start, end: end, score: score})
		}
	}

	return matches
}

func calculateSimilarity(a, b string) float64 {
	// 简化的相似度计算
	a = strings.TrimSpace(strings.ReplaceAll(a, "\t", " "))
	b = strings.TrimSpace(strings.ReplaceAll(b, "\t", " "))
	
	// 去除多余空格
	spaceRegex := regexp.MustCompile(`\s+`)
	a = spaceRegex.ReplaceAllString(a, " ")
	b = spaceRegex.ReplaceAllString(b, " ")

	if a == b {
		return 1.0
	}

	// 计算公共子串长度
	shorter, longer := a, b
	if len(a) > len(b) {
		shorter, longer = b, a
	}

	if len(shorter) == 0 {
		return 0
	}

	matchCount := 0
	for _, char := range shorter {
		if strings.ContainsRune(longer, char) {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(longer))
}

func getContextLines(content string, position int, contextSize int) []string {
	lines := strings.Split(content, "\n")
	
	// 找到位置对应的行
	charCount := 0
	lineIdx := 0
	for i, line := range lines {
		if charCount+len(line)+1 > position {
			lineIdx = i
			break
		}
		charCount += len(line) + 1
	}

	start := max(0, lineIdx-contextSize)
	end := min(len(lines), lineIdx+contextSize+1)

	return lines[start:end]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ReadMultipleFiles 批量读取文件
func (s *FilesystemService) ReadMultipleFiles(ctx context.Context, files []string, startLine, endLine int) ([]FileReadResult, error) {
	results := make([]FileReadResult, 0, len(files))

	for _, filePath := range files {
		result, err := s.ReadFile(ctx, filePath, startLine, endLine)
		if err != nil {
			results = append(results, FileReadResult{
				Content: fmt.Sprintf("❌ %s: %s", filePath, err.Error()),
			})
			continue
		}
		result.Content = fmt.Sprintf("📄 %s (lines %d-%d/%d)\n%s",
			filePath, result.StartLine, result.EndLine, result.TotalLines, result.Content)
		results = append(results, *result)
	}

	return results, nil
}
