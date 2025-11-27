package command

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"backend/internal/logger"

	"go.uber.org/zap"
)

// BashService 安全命令执行服务
type BashService struct {
	workingDir      string
	maxOutputLength int
	defaultTimeout  time.Duration
	dangerousPatterns []*regexp.Regexp
	logger          *zap.Logger
}

// CommandResult 命令执行结果
type CommandResult struct {
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	ExitCode   int       `json:"exit_code"`
	Command    string    `json:"command"`
	ExecutedAt time.Time `json:"executed_at"`
	Duration   int64     `json:"duration_ms"`
}

// NewBashService 创建命令执行服务
func NewBashService(workingDir string) *BashService {
	dangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)rm\s+(-rf?|--recursive)\s+[/~]`),     // rm -rf / 或 ~
		regexp.MustCompile(`(?i)rm\s+(-rf?|--recursive)\s+\*`),       // rm -rf *
		regexp.MustCompile(`(?i)>\s*/dev/sd[a-z]`),                    // 写入磁盘设备
		regexp.MustCompile(`(?i)dd\s+.*of=/dev/`),                     // dd 写入设备
		regexp.MustCompile(`(?i)mkfs\.`),                              // 格式化文件系统
		regexp.MustCompile(`(?i):\s*\(\s*\)\s*\{`),                   // fork 炸弹
		regexp.MustCompile(`(?i)chmod\s+(-R\s+)?777\s+/`),            // chmod 777 /
		regexp.MustCompile(`(?i)chown\s+.*\s+/`),                      // chown /
		regexp.MustCompile(`(?i)shutdown|reboot|halt|poweroff`),       // 关机命令
		regexp.MustCompile(`(?i)curl\s+.*\|\s*(ba)?sh`),              // curl | bash
		regexp.MustCompile(`(?i)wget\s+.*\|\s*(ba)?sh`),              // wget | bash
		regexp.MustCompile(`(?i)eval\s+.*\$\(`),                       // eval $(...)
	}

	return &BashService{
		workingDir:      workingDir,
		maxOutputLength: 50000,
		defaultTimeout:  30 * time.Second,
		dangerousPatterns: dangerousPatterns,
		logger:          logger.Get(),
	}
}

// IsDangerous 检查命令是否危险
func (s *BashService) IsDangerous(command string) bool {
	for _, pattern := range s.dangerousPatterns {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

// Execute 执行命令
func (s *BashService) Execute(ctx context.Context, command string, timeout time.Duration) (*CommandResult, error) {
	if command == "" {
		return nil, errors.New("命令不能为空")
	}

	// 安全检查
	if s.IsDangerous(command) {
		return nil, errors.New("检测到危险命令，已阻止执行")
	}

	// 设置超时
	if timeout <= 0 {
		timeout = s.defaultTimeout
	}
	if timeout > 5*time.Minute {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()
	executedAt := startTime

	// 根据操作系统选择 shell
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	cmd.Dir = s.workingDir

	// 设置环境变量
	cmd.Env = append(cmd.Environ(),
		"LANG=en_US.UTF-8",
		"LC_ALL=en_US.UTF-8",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	duration := time.Since(startTime).Milliseconds()

	result := &CommandResult{
		Stdout:     s.truncateOutput(stdout.String()),
		Stderr:     s.truncateOutput(stderr.String()),
		ExitCode:   0,
		Command:    command,
		ExecutedAt: executedAt,
		Duration:   duration,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, errors.New("命令执行超时")
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Stderr = err.Error()
		}
	}

	s.logger.Info("执行命令",
		zap.String("command", s.truncateLog(command, 100)),
		zap.Int("exit_code", result.ExitCode),
		zap.Int64("duration_ms", duration),
	)

	return result, nil
}

// ExecuteWithInput 执行带输入的命令
func (s *BashService) ExecuteWithInput(ctx context.Context, command string, input string, timeout time.Duration) (*CommandResult, error) {
	if command == "" {
		return nil, errors.New("命令不能为空")
	}

	if s.IsDangerous(command) {
		return nil, errors.New("检测到危险命令，已阻止执行")
	}

	if timeout <= 0 {
		timeout = s.defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	cmd.Dir = s.workingDir
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	result := &CommandResult{
		Stdout:     s.truncateOutput(stdout.String()),
		Stderr:     s.truncateOutput(stderr.String()),
		ExitCode:   0,
		Command:    command,
		ExecutedAt: startTime,
		Duration:   duration,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, errors.New("命令执行超时")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Stderr = err.Error()
		}
	}

	return result, nil
}

// GetWorkingDirectory 获取工作目录
func (s *BashService) GetWorkingDirectory() string {
	return s.workingDir
}

// SetWorkingDirectory 设置工作目录
func (s *BashService) SetWorkingDirectory(dir string) {
	s.workingDir = dir
}

// SetMaxOutputLength 设置最大输出长度
func (s *BashService) SetMaxOutputLength(length int) {
	if length > 0 {
		s.maxOutputLength = length
	}
}

// SetDefaultTimeout 设置默认超时时间
func (s *BashService) SetDefaultTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.defaultTimeout = timeout
	}
}

// AddDangerousPattern 添加危险模式
func (s *BashService) AddDangerousPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	s.dangerousPatterns = append(s.dangerousPatterns, re)
	return nil
}

// truncateOutput 截断输出
func (s *BashService) truncateOutput(output string) string {
	if len(output) <= s.maxOutputLength {
		return output
	}
	return output[:s.maxOutputLength] + "\n\n[输出已截断...]"
}

// truncateLog 截断日志
func (s *BashService) truncateLog(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// ValidateCommand 验证命令（不执行）
func (s *BashService) ValidateCommand(command string) error {
	if command == "" {
		return errors.New("命令不能为空")
	}
	if s.IsDangerous(command) {
		return errors.New("检测到危险命令")
	}
	return nil
}

// GetShellInfo 获取 shell 信息
func (s *BashService) GetShellInfo() map[string]string {
	info := map[string]string{
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"working_dir": s.workingDir,
	}

	if runtime.GOOS == "windows" {
		info["shell"] = "cmd"
	} else {
		info["shell"] = "sh"
	}

	return info
}
