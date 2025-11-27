package codesearch

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"backend/internal/logger"

	"go.uber.org/zap"
)

// ACECodeSearchService ACE 代码搜索服务
type ACECodeSearchService struct {
	basePath       string
	indexCache     map[string][]CodeSymbol
	lastIndexTime  time.Time
	cacheDuration  time.Duration
	customExcludes []string
	mu             sync.RWMutex
	logger         *zap.Logger
}

// NewACECodeSearchService 创建 ACE 代码搜索服务
func NewACECodeSearchService(basePath string) *ACECodeSearchService {
	return &ACECodeSearchService{
		basePath:      basePath,
		indexCache:    make(map[string][]CodeSymbol),
		cacheDuration: time.Minute,
		customExcludes: []string{
			"node_modules", ".git", "dist", "build", "__pycache__",
			"target", ".next", ".nuxt", "coverage", "vendor", ".idea",
		},
		logger: logger.Get(),
	}
}

var languageExtensions = map[string]string{
	".go": "go", ".ts": "typescript", ".tsx": "typescript",
	".js": "javascript", ".jsx": "javascript", ".py": "python",
	".java": "java", ".rs": "rust", ".cs": "csharp",
	".cpp": "cpp", ".c": "c", ".h": "c", ".hpp": "cpp",
}

func detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	return languageExtensions[ext]
}

func (s *ACECodeSearchService) shouldExcludeDirectory(name string) bool {
	for _, exclude := range s.customExcludes {
		if name == exclude {
			return true
		}
	}
	return false
}

// BuildIndex 构建代码符号索引
func (s *ACECodeSearchService) BuildIndex(ctx context.Context, forceRefresh bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if !forceRefresh && len(s.indexCache) > 0 && now.Sub(s.lastIndexTime) < s.cacheDuration {
		return nil
	}

	if forceRefresh {
		s.indexCache = make(map[string][]CodeSymbol)
	}

	filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info.IsDir() {
			if s.shouldExcludeDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		lang := detectLanguage(path)
		if lang == "" {
			return nil
		}
		symbols, _ := s.parseFileSymbols(path, lang)
		if len(symbols) > 0 {
			s.indexCache[path] = symbols
		}
		return nil
	})

	s.lastIndexTime = now
	s.logger.Info("代码索引构建完成", zap.Int("files", len(s.indexCache)))
	return nil
}

func (s *ACECodeSearchService) parseFileSymbols(filePath, language string) ([]CodeSymbol, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(s.basePath, filePath)
	lines := strings.Split(string(content), "\n")

	switch language {
	case "go":
		return s.parseGoSymbols(lines, relPath, language), nil
	case "typescript", "javascript":
		return s.parseJSSymbols(lines, relPath, language), nil
	case "python":
		return s.parsePythonSymbols(lines, relPath, language), nil
	default:
		return s.parseGenericSymbols(lines, relPath, language), nil
	}
}

func (s *ACECodeSearchService) parseGoSymbols(lines []string, filePath, language string) []CodeSymbol {
	symbols := make([]CodeSymbol, 0)
	funcRegex := regexp.MustCompile(`^func\s+(\w+)\s*\(`)
	methodRegex := regexp.MustCompile(`^func\s+\([^)]+\)\s+(\w+)\s*\(`)
	typeRegex := regexp.MustCompile(`^type\s+(\w+)\s+(struct|interface)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if matches := funcRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolFunction, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		} else if matches := methodRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolMethod, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		} else if matches := typeRegex.FindStringSubmatch(trimmed); len(matches) > 2 {
			symbolType := SymbolStruct
			if matches[2] == "interface" {
				symbolType = SymbolInterface
			}
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: symbolType, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		}
	}
	return symbols
}

func (s *ACECodeSearchService) parseJSSymbols(lines []string, filePath, language string) []CodeSymbol {
	symbols := make([]CodeSymbol, 0)
	funcRegex := regexp.MustCompile(`(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	classRegex := regexp.MustCompile(`(?:export\s+)?class\s+(\w+)`)
	interfaceRegex := regexp.MustCompile(`(?:export\s+)?interface\s+(\w+)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if matches := funcRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolFunction, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		} else if matches := classRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolClass, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		} else if matches := interfaceRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolInterface, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		}
	}
	return symbols
}

func (s *ACECodeSearchService) parsePythonSymbols(lines []string, filePath, language string) []CodeSymbol {
	symbols := make([]CodeSymbol, 0)
	funcRegex := regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(`)
	classRegex := regexp.MustCompile(`^class\s+(\w+)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if matches := funcRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolFunction, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		} else if matches := classRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolClass, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		}
	}
	return symbols
}

func (s *ACECodeSearchService) parseGenericSymbols(lines []string, filePath, language string) []CodeSymbol {
	symbols := make([]CodeSymbol, 0)
	funcRegex := regexp.MustCompile(`(?:func|function|def)\s+(\w+)`)
	classRegex := regexp.MustCompile(`class\s+(\w+)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if matches := funcRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolFunction, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		} else if matches := classRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			symbols = append(symbols, CodeSymbol{Name: matches[1], Type: SymbolClass, FilePath: filePath, Line: i + 1, Language: language, Context: trimmed})
		}
	}
	return symbols
}

// SearchSymbols 搜索符号
func (s *ACECodeSearchService) SearchSymbols(ctx context.Context, opts *SymbolSearchOptions) (*SemanticSearchResult, error) {
	startTime := time.Now()
	if err := s.BuildIndex(ctx, false); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(opts.Query)
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	type scoredSymbol struct {
		symbol CodeSymbol
		score  int
	}
	scored := make([]scoredSymbol, 0)

	for _, fileSymbols := range s.indexCache {
		for _, symbol := range fileSymbols {
			if opts.SymbolType != "" && symbol.Type != opts.SymbolType {
				continue
			}
			if opts.Language != "" && symbol.Language != opts.Language {
				continue
			}
			score := calculateMatchScore(queryLower, strings.ToLower(symbol.Name))
			if score > 0 {
				scored = append(scored, scoredSymbol{symbol: symbol, score: score})
			}
		}
	}

	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })

	symbols := make([]CodeSymbol, 0, maxResults)
	for i := 0; i < len(scored) && i < maxResults; i++ {
		symbols = append(symbols, scored[i].symbol)
	}

	return &SemanticSearchResult{
		Query: opts.Query, Symbols: symbols, TotalResults: len(symbols),
		SearchTimeMs: time.Since(startTime).Milliseconds(),
	}, nil
}

func calculateMatchScore(query, name string) int {
	if name == query {
		return 100
	}
	if strings.HasPrefix(name, query) {
		return 80
	}
	if strings.Contains(name, query) {
		return 60
	}
	score, queryIndex := 0, 0
	for i := 0; i < len(name) && queryIndex < len(query); i++ {
		if name[i] == query[queryIndex] {
			score += 20
			queryIndex++
		}
	}
	if queryIndex == len(query) {
		return score
	}
	return 0
}

// FindDefinition 查找符号定义
func (s *ACECodeSearchService) FindDefinition(ctx context.Context, symbolName, contextFile string) (*CodeSymbol, error) {
	if err := s.BuildIndex(ctx, false); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if contextFile != "" {
		fullPath := filepath.Join(s.basePath, contextFile)
		if symbols, ok := s.indexCache[fullPath]; ok {
			for _, sym := range symbols {
				if sym.Name == symbolName && isDefinitionType(sym.Type) {
					return &sym, nil
				}
			}
		}
	}

	for _, fileSymbols := range s.indexCache {
		for _, sym := range fileSymbols {
			if sym.Name == symbolName && isDefinitionType(sym.Type) {
				return &sym, nil
			}
		}
	}
	return nil, nil
}

func isDefinitionType(t CodeSymbolType) bool {
	return t == SymbolFunction || t == SymbolClass || t == SymbolMethod || t == SymbolInterface || t == SymbolStruct || t == SymbolType
}

// FindReferences 查找所有引用
func (s *ACECodeSearchService) FindReferences(ctx context.Context, symbolName string, maxResults int) ([]CodeReference, error) {
	if maxResults <= 0 {
		maxResults = 100
	}
	references := make([]CodeReference, 0)
	regex := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbolName) + `\b`)

	filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && s.shouldExcludeDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(references) >= maxResults {
			return filepath.SkipAll
		}
		if detectLanguage(path) == "" {
			return nil
		}
		content, _ := os.ReadFile(path)
		relPath, _ := filepath.Rel(s.basePath, path)
		lines := strings.Split(string(content), "\n")

		for i, line := range lines {
			if len(references) >= maxResults {
				break
			}
			if regex.MatchString(line) {
				refType := "usage"
				if strings.Contains(line, "import") {
					refType = "import"
				} else if regexp.MustCompile(`(?:func|function|def|class|type)\s+` + regexp.QuoteMeta(symbolName)).MatchString(line) {
					refType = "definition"
				}
				references = append(references, CodeReference{Symbol: symbolName, FilePath: relPath, Line: i + 1, Context: strings.TrimSpace(line), ReferenceType: refType})
			}
		}
		return nil
	})
	return references, nil
}

// GetFileOutline 获取文件大纲
func (s *ACECodeSearchService) GetFileOutline(ctx context.Context, filePath string) (*FileOutline, error) {
	fullPath := filepath.Join(s.basePath, filePath)
	lang := detectLanguage(fullPath)
	if lang == "" {
		return nil, fmt.Errorf("不支持的文件类型: %s", filePath)
	}
	symbols, err := s.parseFileSymbols(fullPath, lang)
	if err != nil {
		return nil, err
	}
	return &FileOutline{FilePath: filePath, Language: lang, Symbols: symbols}, nil
}

// TextSearch 文本搜索
func (s *ACECodeSearchService) TextSearch(ctx context.Context, opts *TextSearchOptions) ([]SearchResult, error) {
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	if s.isGitRepository() {
		if results, err := s.gitGrepSearch(ctx, opts); err == nil && len(results) > 0 {
			return results, nil
		}
	}
	if s.isCommandAvailable("rg") {
		if results, err := s.ripgrepSearch(ctx, opts); err == nil {
			return results, nil
		}
	}
	return s.goTextSearch(ctx, opts)
}

func (s *ACECodeSearchService) isGitRepository() bool {
	info, err := os.Stat(filepath.Join(s.basePath, ".git"))
	return err == nil && info.IsDir()
}

func (s *ACECodeSearchService) isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func (s *ACECodeSearchService) gitGrepSearch(ctx context.Context, opts *TextSearchOptions) ([]SearchResult, error) {
	args := []string{"grep", "--untracked", "-n"}
	if !opts.CaseSensitive {
		args = append(args, "-i")
	}
	if opts.IsRegex {
		args = append(args, "-E")
	} else {
		args = append(args, "--fixed-strings")
	}
	args = append(args, opts.Pattern)
	if opts.FileGlob != "" {
		args = append(args, "--", opts.FileGlob)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = s.basePath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []SearchResult{}, nil
		}
		return nil, err
	}
	return s.parseGrepOutput(string(output), opts.MaxResults), nil
}

func (s *ACECodeSearchService) ripgrepSearch(ctx context.Context, opts *TextSearchOptions) ([]SearchResult, error) {
	args := []string{"-n", "--no-heading"}
	if !opts.CaseSensitive {
		args = append(args, "-i")
	}
	args = append(args, opts.Pattern)
	if opts.FileGlob != "" {
		args = append(args, "--glob", opts.FileGlob)
	}

	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = s.basePath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []SearchResult{}, nil
		}
		return nil, err
	}
	return s.parseGrepOutput(string(output), opts.MaxResults), nil
}

func (s *ACECodeSearchService) parseGrepOutput(output string, maxResults int) []SearchResult {
	results := make([]SearchResult, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() && len(results) < maxResults {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		var lineNum int
		fmt.Sscanf(parts[1], "%d", &lineNum)
		results = append(results, SearchResult{FilePath: parts[0], Line: lineNum, Content: strings.TrimSpace(parts[2])})
	}
	return results
}

func (s *ACECodeSearchService) goTextSearch(ctx context.Context, opts *TextSearchOptions) ([]SearchResult, error) {
	results := make([]SearchResult, 0)
	var searchRegex *regexp.Regexp

	if opts.IsRegex {
		flags := ""
		if !opts.CaseSensitive {
			flags = "(?i)"
		}
		var err error
		searchRegex, err = regexp.Compile(flags + opts.Pattern)
		if err != nil {
			return nil, fmt.Errorf("无效的正则表达式: %s", opts.Pattern)
		}
	}

	binaryExts := map[string]bool{".jpg": true, ".png": true, ".gif": true, ".pdf": true, ".zip": true, ".exe": true}

	filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && s.shouldExcludeDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(results) >= opts.MaxResults {
			return filepath.SkipAll
		}
		if binaryExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		if opts.FileGlob != "" {
			if matched, _ := filepath.Match(opts.FileGlob, info.Name()); !matched {
				return nil
			}
		}

		content, _ := os.ReadFile(path)
		relPath, _ := filepath.Rel(s.basePath, path)
		lines := strings.Split(string(content), "\n")

		for i, line := range lines {
			if len(results) >= opts.MaxResults {
				break
			}
			var matched bool
			if searchRegex != nil {
				matched = searchRegex.MatchString(line)
			} else if opts.CaseSensitive {
				matched = strings.Contains(line, opts.Pattern)
			} else {
				matched = strings.Contains(strings.ToLower(line), strings.ToLower(opts.Pattern))
			}
			if matched {
				results = append(results, SearchResult{FilePath: relPath, Line: i + 1, Content: strings.TrimSpace(line)})
			}
		}
		return nil
	})
	return results, nil
}

// SetBasePath 设置基础路径
func (s *ACECodeSearchService) SetBasePath(basePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.basePath = basePath
	s.indexCache = make(map[string][]CodeSymbol)
	s.lastIndexTime = time.Time{}
}

// GetConcurrency 获取并发数
func GetConcurrency() int {
	n := runtime.NumCPU()
	if n > 4 {
		return n / 2
	}
	return n
}
