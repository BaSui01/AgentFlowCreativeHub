package workspace

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

var nonWord = regexp.MustCompile(`[^\p{Han}A-Za-z0-9]+`)

// FolderTemplate 预设目录模板
type FolderTemplate struct {
	Name     string
	Slug     string
	Category string
}

// 默认根目录
var defaultFolders = []FolderTemplate{
	{Name: "大纲", Slug: "outlines", Category: "outline"},
	{Name: "草稿", Slug: "drafts", Category: "draft"},
	{Name: "研究", Slug: "research", Category: "research"},
	{Name: "素材", Slug: "assets", Category: "asset"},
	{Name: "暂存区", Slug: "staging", Category: "staging"},
	{Name: "智能体产出", Slug: "agents", Category: "agent_output"},
	{Name: "会话记录", Slug: "sessions", Category: "session"},
}

// ArtifactType 智能体产出类型
type ArtifactType string

const (
	ArtifactTypeOutline  ArtifactType = "outline"
	ArtifactTypeDraft    ArtifactType = "draft"
	ArtifactTypeResearch ArtifactType = "research"
	ArtifactTypeAnalysis ArtifactType = "analysis"
	ArtifactTypeReport   ArtifactType = "report"
	ArtifactTypeCode     ArtifactType = "code"
	ArtifactTypeData     ArtifactType = "data"
	ArtifactTypeOther    ArtifactType = "other"
)

// ArtifactNamingRequest 产出物命名请求
type ArtifactNamingRequest struct {
	AgentName   string       // 智能体名称
	AgentID     string       // 智能体ID
	SessionID   string       // 会话ID
	TaskType    ArtifactType // 任务类型
	TitleHint   string       // 标题提示
	Content     string       // 内容（用于提取标题）
	Sequence    int          // 序号（同一会话内）
}

// ArtifactPathResult 产出物路径结果
type ArtifactPathResult struct {
	FullPath      string // 完整路径: agents/planner/outputs/outline-20251125-143052-001.md
	FolderPath    string // 目录路径: agents/planner/outputs
	FileName      string // 文件名: outline-20251125-143052-001.md
	AgentFolder   string // 智能体目录: agents/planner
	SessionFolder string // 会话目录: sessions/{session_id}
}

// AutoNamingPolicy 控制自动命名与目录归类
type AutoNamingPolicy struct {
	organizeByAgent   bool
	organizeBySession bool
	namingPattern     string
	seqCounter        map[string]int
	mu                sync.Mutex
}

// AutoNamingOption 配置选项
type AutoNamingOption func(*AutoNamingPolicy)

// WithOrganizeByAgent 启用按智能体分目录
func WithOrganizeByAgent(enabled bool) AutoNamingOption {
	return func(p *AutoNamingPolicy) {
		p.organizeByAgent = enabled
	}
}

// WithOrganizeBySession 启用按会话分目录
func WithOrganizeBySession(enabled bool) AutoNamingOption {
	return func(p *AutoNamingPolicy) {
		p.organizeBySession = enabled
	}
}

// WithNamingPattern 设置命名模式
func WithNamingPattern(pattern string) AutoNamingOption {
	return func(p *AutoNamingPolicy) {
		p.namingPattern = pattern
	}
}

// NewAutoNamingPolicy 创建策略实例
func NewAutoNamingPolicy(opts ...AutoNamingOption) *AutoNamingPolicy {
	p := &AutoNamingPolicy{
		organizeByAgent:   true,
		organizeBySession: false,
		namingPattern:     "{agent}-{type}-{timestamp}-{seq}",
		seqCounter:        make(map[string]int),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ResolveFolder 根据文件类型返回目标目录模板
func (p *AutoNamingPolicy) ResolveFolder(fileType string) FolderTemplate {
	typeLower := strings.ToLower(strings.TrimSpace(fileType))
	for _, tpl := range defaultFolders {
		if tpl.Category == typeLower {
			return tpl
		}
	}
	if typeLower == "outline" {
		return defaultFolders[0]
	}
	return defaultFolders[1] // 默认归入草稿
}

// SuggestName 根据内容生成文件名
func (p *AutoNamingPolicy) SuggestName(fileType, hint, content string) string {
	base := strings.TrimSpace(hint)
	if base == "" {
		base = extractTitle(content)
	}
	if base == "" {
		base = titleize(strings.ToLower(fileType))
	}
	base = trimNonWord(base)
	stamp := time.Now().Format("20060102-150405")
	if base == "" {
		return stamp
	}
	return base + "-" + stamp
}

func extractTitle(content string) string {
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			line = strings.TrimLeft(line, "#")
			line = strings.TrimSpace(line)
		}
		if line != "" {
			return line
		}
	}
	return ""
}

func trimNonWord(s string) string {
	s = nonWord.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len([]rune(s)) > 40 {
		s = string([]rune(s)[:40])
	}
	return s
}

func titleize(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		runes := []rune(part)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		parts[i] = string(runes)
	}
	return strings.Join(parts, "")
}

// GenerateArtifactPath 生成智能体产出物的完整路径
func (p *AutoNamingPolicy) GenerateArtifactPath(req *ArtifactNamingRequest) *ArtifactPathResult {
	result := &ArtifactPathResult{}

	// 生成时间戳
	timestamp := time.Now().Format("20060102-150405")

	// 获取序号
	seq := req.Sequence
	if seq <= 0 {
		seq = p.getNextSequence(req.SessionID, req.AgentID)
	}

	// 生成文件名
	fileName := p.generateArtifactFileName(req, timestamp, seq)
	result.FileName = fileName

	// 构建目录路径
	var pathParts []string

	if p.organizeByAgent && req.AgentName != "" {
		agentSlug := slugifyAgent(req.AgentName)
		result.AgentFolder = fmt.Sprintf("agents/%s", agentSlug)
		pathParts = append(pathParts, "agents", agentSlug, "outputs")
	}

	if p.organizeBySession && req.SessionID != "" {
		sessionSlug := slugifySession(req.SessionID)
		result.SessionFolder = fmt.Sprintf("sessions/%s", sessionSlug)
		if len(pathParts) == 0 {
			pathParts = append(pathParts, "sessions", sessionSlug)
		}
	}

	// 如果没有启用任何组织方式，使用默认目录
	if len(pathParts) == 0 {
		tpl := p.ResolveFolder(string(req.TaskType))
		pathParts = append(pathParts, tpl.Slug)
	}

	result.FolderPath = strings.Join(pathParts, "/")
	result.FullPath = fmt.Sprintf("%s/%s", result.FolderPath, fileName)

	return result
}

// generateArtifactFileName 生成产出物文件名
func (p *AutoNamingPolicy) generateArtifactFileName(req *ArtifactNamingRequest, timestamp string, seq int) string {
	// 提取或生成基础名称
	baseName := strings.TrimSpace(req.TitleHint)
	if baseName == "" {
		baseName = extractTitle(req.Content)
	}
	baseName = trimNonWord(baseName)

	// 获取任务类型
	taskType := string(req.TaskType)
	if taskType == "" {
		taskType = "output"
	}

	// 获取智能体名称
	agentName := slugifyAgent(req.AgentName)
	if agentName == "" {
		agentName = "agent"
	}

	// 根据命名模式生成文件名
	name := p.namingPattern
	name = strings.ReplaceAll(name, "{agent}", agentName)
	name = strings.ReplaceAll(name, "{type}", taskType)
	name = strings.ReplaceAll(name, "{timestamp}", timestamp)
	name = strings.ReplaceAll(name, "{seq}", fmt.Sprintf("%03d", seq))
	name = strings.ReplaceAll(name, "{title}", baseName)

	// 添加扩展名
	ext := inferExtension(req.TaskType, req.Content)
	return name + ext
}

// getNextSequence 获取下一个序号
func (p *AutoNamingPolicy) getNextSequence(sessionID, agentID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := fmt.Sprintf("%s:%s", sessionID, agentID)
	p.seqCounter[key]++
	return p.seqCounter[key]
}

// ResetSequence 重置序号计数器
func (p *AutoNamingPolicy) ResetSequence(sessionID, agentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := fmt.Sprintf("%s:%s", sessionID, agentID)
	delete(p.seqCounter, key)
}

// ResolveAgentFolder 解析智能体专属目录路径
func (p *AutoNamingPolicy) ResolveAgentFolder(agentName string) string {
	if agentName == "" {
		return "agents/unknown"
	}
	return fmt.Sprintf("agents/%s", slugifyAgent(agentName))
}

// ResolveSessionFolder 解析会话专属目录路径
func (p *AutoNamingPolicy) ResolveSessionFolder(sessionID string) string {
	if sessionID == "" {
		return "sessions/unknown"
	}
	return fmt.Sprintf("sessions/%s", slugifySession(sessionID))
}

// slugifyAgent 将智能体名称转换为 slug
func slugifyAgent(name string) string {
	if name == "" {
		return ""
	}
	// 常见智能体名称映射
	agentMap := map[string]string{
		"planner":    "planner",
		"researcher": "researcher",
		"analyzer":   "analyzer",
		"writer":     "writer",
		"reviewer":   "reviewer",
		"translator": "translator",
		"formatter":  "formatter",
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	if slug, ok := agentMap[lower]; ok {
		return slug
	}
	return trimNonWord(lower)
}

// slugifySession 将会话ID转换为 slug
func slugifySession(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	// 保留UUID格式或截断长ID
	id := strings.TrimSpace(sessionID)
	if len(id) > 36 {
		id = id[:36]
	}
	return strings.ToLower(id)
}

// inferExtension 根据类型和内容推断文件扩展名
func inferExtension(artifactType ArtifactType, content string) string {
	switch artifactType {
	case ArtifactTypeCode:
		// 尝试从内容检测代码类型
		if strings.Contains(content, "package ") && strings.Contains(content, "func ") {
			return ".go"
		}
		if strings.Contains(content, "def ") || strings.Contains(content, "import ") {
			return ".py"
		}
		if strings.Contains(content, "function ") || strings.Contains(content, "const ") {
			return ".js"
		}
		return ".txt"
	case ArtifactTypeData:
		if strings.HasPrefix(strings.TrimSpace(content), "{") || strings.HasPrefix(strings.TrimSpace(content), "[") {
			return ".json"
		}
		return ".csv"
	default:
		return ".md"
	}
}

// GetDefaultAgentFolders 获取智能体默认子目录结构
func GetDefaultAgentFolders() []FolderTemplate {
	return []FolderTemplate{
		{Name: "产出", Slug: "outputs", Category: "output"},
		{Name: "草稿", Slug: "drafts", Category: "draft"},
		{Name: "日志", Slug: "logs", Category: "log"},
	}
}

// GetDefaultSessionFolders 获取会话默认子目录结构
func GetDefaultSessionFolders() []FolderTemplate {
	return []FolderTemplate{
		{Name: "上下文", Slug: "context", Category: "context"},
		{Name: "产出物", Slug: "artifacts", Category: "artifact"},
		{Name: "历史", Slug: "history", Category: "history"},
	}
}
