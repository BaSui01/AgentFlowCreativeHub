package builtin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CodeInterpreterTool 代码解释器工具
type CodeInterpreterTool struct {
	config *CodeInterpreterConfig
}

// CodeInterpreterConfig 代码解释器配置
type CodeInterpreterConfig struct {
	WorkDir        string        // 工作目录
	Timeout        time.Duration // 执行超时
	MaxOutputSize  int           // 最大输出大小 (字节)
	AllowedLangs   []string      // 允许的语言
	EnableNetwork  bool          // 是否允许网络访问
	EnableFileIO   bool          // 是否允许文件读写
	MemoryLimit    int64         // 内存限制 (字节)
	CPULimit       float64       // CPU 限制 (核心数)
}

// DefaultCodeInterpreterConfig 默认配置
func DefaultCodeInterpreterConfig() *CodeInterpreterConfig {
	return &CodeInterpreterConfig{
		WorkDir:       os.TempDir(),
		Timeout:       30 * time.Second,
		MaxOutputSize: 1024 * 1024, // 1MB
		AllowedLangs:  []string{"python", "javascript", "bash"},
		EnableNetwork: false,
		EnableFileIO:  true,
		MemoryLimit:   256 * 1024 * 1024, // 256MB
		CPULimit:      1.0,
	}
}

// NewCodeInterpreterTool 创建代码解释器工具
func NewCodeInterpreterTool(config *CodeInterpreterConfig) *CodeInterpreterTool {
	if config == nil {
		config = DefaultCodeInterpreterConfig()
	}
	return &CodeInterpreterTool{config: config}
}

// Execute 执行代码
func (t *CodeInterpreterTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	language, _ := input["language"].(string)
	code, _ := input["code"].(string)

	if language == "" {
		return nil, fmt.Errorf("language is required")
	}
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}

	// 检查语言是否允许
	if !t.isLanguageAllowed(language) {
		return nil, fmt.Errorf("language %s is not allowed", language)
	}

	// 安全检查
	if err := t.securityCheck(language, code); err != nil {
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	// 创建带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, t.config.Timeout)
	defer cancel()

	// 执行代码
	result, err := t.executeCode(execCtx, language, code)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
			"output":  "",
		}, nil
	}

	// 截断输出
	if len(result.Output) > t.config.MaxOutputSize {
		result.Output = result.Output[:t.config.MaxOutputSize] + "\n... (output truncated)"
	}

	return map[string]any{
		"success":  result.ExitCode == 0,
		"output":   result.Output,
		"error":    result.Error,
		"exitCode": result.ExitCode,
		"duration": result.Duration.String(),
	}, nil
}

// Validate 验证输入
func (t *CodeInterpreterTool) Validate(input map[string]any) error {
	if _, ok := input["language"]; !ok {
		return fmt.Errorf("language is required")
	}
	if _, ok := input["code"]; !ok {
		return fmt.Errorf("code is required")
	}
	return nil
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Output   string
	Error    string
	ExitCode int
	Duration time.Duration
}

// isLanguageAllowed 检查语言是否允许
func (t *CodeInterpreterTool) isLanguageAllowed(lang string) bool {
	lang = strings.ToLower(lang)
	for _, allowed := range t.config.AllowedLangs {
		if strings.ToLower(allowed) == lang {
			return true
		}
	}
	return false
}

// securityCheck 安全检查
func (t *CodeInterpreterTool) securityCheck(language, code string) error {
	// 危险模式列表
	dangerousPatterns := []struct {
		pattern string
		desc    string
	}{
		// 通用危险模式
		{`rm\s+-rf`, "dangerous rm command"},
		{`>\s*/dev/`, "writing to device files"},
		{`/etc/passwd`, "accessing password file"},
		{`/etc/shadow`, "accessing shadow file"},
		{`chmod\s+777`, "dangerous chmod"},
		{`curl.*\|.*sh`, "remote code execution"},
		{`wget.*\|.*sh`, "remote code execution"},
		{`eval\s*\(`, "eval is dangerous"},
		{`exec\s*\(`, "exec is dangerous"},
		{`__import__`, "dynamic import is dangerous"},
		{`subprocess`, "subprocess module is restricted"},
		{`os\.system`, "os.system is restricted"},
		{`os\.popen`, "os.popen is restricted"},
		{`os\.exec`, "os.exec is restricted"},
		{`socket\.`, "socket operations are restricted"},
		{`requests\.`, "network requests are restricted"},
		{`urllib`, "network requests are restricted"},
		{`http\.client`, "network requests are restricted"},
	}

	// 如果禁用网络，添加网络相关检查
	if !t.config.EnableNetwork {
		dangerousPatterns = append(dangerousPatterns, []struct {
			pattern string
			desc    string
		}{
			{`fetch\s*\(`, "fetch is not allowed"},
			{`XMLHttpRequest`, "XMLHttpRequest is not allowed"},
			{`net\.`, "network operations are not allowed"},
		}...)
	}

	// 如果禁用文件IO，添加文件相关检查
	if !t.config.EnableFileIO {
		dangerousPatterns = append(dangerousPatterns, []struct {
			pattern string
			desc    string
		}{
			{`open\s*\(`, "file operations are not allowed"},
			{`fs\.`, "file system operations are not allowed"},
			{`io\.`, "io operations are not allowed"},
		}...)
	}

	for _, dp := range dangerousPatterns {
		matched, _ := regexp.MatchString(dp.pattern, code)
		if matched {
			return fmt.Errorf("%s", dp.desc)
		}
	}

	return nil
}

// executeCode 执行代码
func (t *CodeInterpreterTool) executeCode(ctx context.Context, language, code string) (*ExecutionResult, error) {
	startTime := time.Now()

	var cmd *exec.Cmd
	var tempFile string
	var err error

	switch strings.ToLower(language) {
	case "python", "python3":
		tempFile, err = t.writeTempFile(".py", code)
		if err != nil {
			return nil, err
		}
		defer os.Remove(tempFile)
		cmd = exec.CommandContext(ctx, "python3", tempFile)

	case "javascript", "js", "node":
		tempFile, err = t.writeTempFile(".js", code)
		if err != nil {
			return nil, err
		}
		defer os.Remove(tempFile)
		cmd = exec.CommandContext(ctx, "node", tempFile)

	case "bash", "sh":
		tempFile, err = t.writeTempFile(".sh", code)
		if err != nil {
			return nil, err
		}
		defer os.Remove(tempFile)
		cmd = exec.CommandContext(ctx, "bash", tempFile)

	case "go", "golang":
		tempFile, err = t.writeTempFile(".go", code)
		if err != nil {
			return nil, err
		}
		defer os.Remove(tempFile)
		cmd = exec.CommandContext(ctx, "go", "run", tempFile)

	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// 设置工作目录
	cmd.Dir = t.config.WorkDir

	// 设置环境变量 (限制)
	cmd.Env = []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=" + t.config.WorkDir,
		"TMPDIR=" + t.config.WorkDir,
	}

	// 捕获输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行
	err = cmd.Run()
	duration := time.Since(startTime)

	result := &ExecutionResult{
		Output:   stdout.String(),
		Error:    stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Error = "execution timeout"
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	}

	return result, nil
}

// writeTempFile 写入临时文件
func (t *CodeInterpreterTool) writeTempFile(ext, content string) (string, error) {
	tmpFile, err := os.CreateTemp(t.config.WorkDir, "code_*"+ext)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("close temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// SandboxedCodeInterpreter 沙箱化代码解释器 (使用 Docker)
type SandboxedCodeInterpreter struct {
	config      *SandboxConfig
	dockerImage string
}

// SandboxConfig 沙箱配置
type SandboxConfig struct {
	DockerImage   string
	MemoryLimit   string // e.g., "256m"
	CPULimit      string // e.g., "1.0"
	Timeout       time.Duration
	NetworkMode   string // "none" for no network
	ReadOnlyRoot  bool
	MaxOutputSize int
}

// DefaultSandboxConfig 默认沙箱配置
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		DockerImage:   "python:3.11-slim",
		MemoryLimit:   "256m",
		CPULimit:      "1.0",
		Timeout:       30 * time.Second,
		NetworkMode:   "none",
		ReadOnlyRoot:  true,
		MaxOutputSize: 1024 * 1024,
	}
}

// NewSandboxedCodeInterpreter 创建沙箱化代码解释器
func NewSandboxedCodeInterpreter(config *SandboxConfig) *SandboxedCodeInterpreter {
	if config == nil {
		config = DefaultSandboxConfig()
	}
	return &SandboxedCodeInterpreter{
		config:      config,
		dockerImage: config.DockerImage,
	}
}

// Execute 在 Docker 沙箱中执行代码
func (s *SandboxedCodeInterpreter) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	language, _ := input["language"].(string)
	code, _ := input["code"].(string)

	if language == "" || code == "" {
		return nil, fmt.Errorf("language and code are required")
	}

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "sandbox_*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 写入代码文件
	var filename string
	switch strings.ToLower(language) {
	case "python", "python3":
		filename = "code.py"
	case "javascript", "js", "node":
		filename = "code.js"
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	codePath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(codePath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("write code file: %w", err)
	}

	// 构建 Docker 命令
	args := []string{
		"run",
		"--rm",
		"--memory=" + s.config.MemoryLimit,
		"--cpus=" + s.config.CPULimit,
		"--network=" + s.config.NetworkMode,
		"-v", tmpDir + ":/app:ro",
		"-w", "/app",
	}

	if s.config.ReadOnlyRoot {
		args = append(args, "--read-only")
	}

	args = append(args, s.dockerImage)

	// 添加执行命令
	switch strings.ToLower(language) {
	case "python", "python3":
		args = append(args, "python3", filename)
	case "javascript", "js", "node":
		args = append(args, "node", filename)
	}

	// 执行
	execCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)

	result := map[string]any{
		"success":  err == nil,
		"output":   stdout.String(),
		"error":    stderr.String(),
		"duration": duration.String(),
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result["exitCode"] = exitErr.ExitCode()
	} else if err != nil {
		result["exitCode"] = -1
	} else {
		result["exitCode"] = 0
	}

	return result, nil
}
