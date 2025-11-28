package builtin

import (
	"context"
	"fmt"
	"time"

	"backend/internal/codesearch"
	"backend/internal/command"
	"backend/internal/knowledge"
	"backend/internal/tools"
	"backend/internal/workflow/approval"
	"backend/internal/workflow/executor"
	"backend/internal/workspace"
)

// ACESearchTool ACE 代码搜索工具
type ACESearchTool struct {
	service *codesearch.ACECodeSearchService
}

// NewACESearchTool 创建 ACE 搜索工具
func NewACESearchTool(basePath string) *ACESearchTool {
	return &ACESearchTool{
		service: codesearch.NewACECodeSearchService(basePath),
	}
}

// Execute 执行工具
func (t *ACESearchTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	action, _ := input["action"].(string)

	switch action {
	case "find_definition":
		symbolName, _ := input["symbol_name"].(string)
		contextFile, _ := input["context_file"].(string)
		result, err := t.service.FindDefinition(ctx, symbolName, contextFile)
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": result}, nil

	case "find_references":
		symbolName, _ := input["symbol_name"].(string)
		maxResults := 100
		if v, ok := input["max_results"].(float64); ok {
			maxResults = int(v)
		}
		results, err := t.service.FindReferences(ctx, symbolName, maxResults)
		if err != nil {
			return nil, err
		}
		return map[string]any{"references": results}, nil

	case "search_symbols":
		query, _ := input["query"].(string)
		opts := &codesearch.SymbolSearchOptions{Query: query, MaxResults: 50}
		if v, ok := input["symbol_type"].(string); ok {
			opts.SymbolType = codesearch.CodeSymbolType(v)
		}
		if v, ok := input["language"].(string); ok {
			opts.Language = v
		}
		if v, ok := input["max_results"].(float64); ok {
			opts.MaxResults = int(v)
		}
		result, err := t.service.SearchSymbols(ctx, opts)
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": result}, nil

	case "file_outline":
		filePath, _ := input["file_path"].(string)
		result, err := t.service.GetFileOutline(ctx, filePath)
		if err != nil {
			return nil, err
		}
		return map[string]any{"outline": result}, nil

	case "text_search":
		pattern, _ := input["pattern"].(string)
		opts := &codesearch.TextSearchOptions{Pattern: pattern, MaxResults: 100}
		if v, ok := input["file_glob"].(string); ok {
			opts.FileGlob = v
		}
		if v, ok := input["is_regex"].(bool); ok {
			opts.IsRegex = v
		}
		if v, ok := input["max_results"].(float64); ok {
			opts.MaxResults = int(v)
		}
		results, err := t.service.TextSearch(ctx, opts)
		if err != nil {
			return nil, err
		}
		return map[string]any{"results": results}, nil

	default:
		return nil, fmt.Errorf("未知操作: %s", action)
	}
}

// Validate 验证输入
func (t *ACESearchTool) Validate(input map[string]any) error {
	action, ok := input["action"].(string)
	if !ok || action == "" {
		return fmt.Errorf("缺少 action 参数")
	}
	return nil
}

// GetDefinition 获取工具定义
func (t *ACESearchTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "ace-code-search",
		DisplayName: "ACE 代码搜索",
		Description: "提供符号搜索、定义查找、引用分析、文件大纲、文本搜索等功能",
		Category:    "search",
		Type:        "builtin",
		Status:      "active",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "操作类型: find_definition, find_references, search_symbols, file_outline, text_search",
					"enum":        []string{"find_definition", "find_references", "search_symbols", "file_outline", "text_search"},
				},
				"symbol_name": map[string]any{"type": "string", "description": "符号名称"},
				"query":       map[string]any{"type": "string", "description": "搜索查询"},
				"pattern":     map[string]any{"type": "string", "description": "文本搜索模式"},
				"file_path":   map[string]any{"type": "string", "description": "文件路径"},
				"max_results": map[string]any{"type": "number", "description": "最大结果数"},
			},
			"required": []string{"action"},
		},
	}
}

// CodebaseSearchTool 代码库语义搜索工具
type CodebaseSearchTool struct {
	service *codesearch.CodebaseSearchService
}

// NewCodebaseSearchTool 创建代码库搜索工具
func NewCodebaseSearchTool(basePath string) *CodebaseSearchTool {
	return &CodebaseSearchTool{
		service: codesearch.NewCodebaseSearchService(basePath, nil),
	}
}

// Execute 执行
func (t *CodebaseSearchTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	query, _ := input["query"].(string)
	topN := 10
	if v, ok := input["top_n"].(float64); ok {
		topN = int(v)
	}

	results, err := t.service.Search(ctx, query, topN)
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results, "total": len(results)}, nil
}

// Validate 验证
func (t *CodebaseSearchTool) Validate(input map[string]any) error {
	if _, ok := input["query"].(string); !ok {
		return fmt.Errorf("缺少 query 参数")
	}
	return nil
}

// GetDefinition 获取定义
func (t *CodebaseSearchTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "codebase-search",
		DisplayName: "代码库语义搜索",
		Description: "使用 embedding 进行语义代码搜索，根据含义而非关键词查找相似代码",
		Category:    "search",
		Type:        "builtin",
		Status:      "active",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "搜索查询"},
				"top_n": map[string]any{"type": "number", "description": "返回结果数量", "default": 10},
			},
			"required": []string{"query"},
		},
	}
}

// TodoTool 任务管理工具
type TodoTool struct {
	service *executor.TodoService
}

// NewTodoTool 创建任务工具
func NewTodoTool(service *executor.TodoService) *TodoTool {
	return &TodoTool{service: service}
}

// Execute 执行
func (t *TodoTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	action, _ := input["action"].(string)
	tenantID, _ := input["tenant_id"].(string)
	sessionID, _ := input["session_id"].(string)

	switch action {
	case "create":
		items, _ := input["items"].([]any)
		todoItems := make([]executor.CreateTodoInput, 0)
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				content, _ := m["content"].(string)
				var parentID *string
				if pid, ok := m["parent_id"].(string); ok {
					parentID = &pid
				}
				todoItems = append(todoItems, executor.CreateTodoInput{Content: content, ParentID: parentID})
			}
		}
		result, err := t.service.CreateTodoList(ctx, tenantID, sessionID, todoItems)
		if err != nil {
			return nil, err
		}
		return map[string]any{"todo_list": result}, nil

	case "get":
		result, err := t.service.GetTodoList(ctx, tenantID, sessionID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"todo_list": result}, nil

	case "update":
		todoID, _ := input["todo_id"].(string)
		var status *executor.TodoStatus
		var content *string
		if s, ok := input["status"].(string); ok {
			st := executor.TodoStatus(s)
			status = &st
		}
		if c, ok := input["content"].(string); ok {
			content = &c
		}
		result, err := t.service.UpdateTodoItem(ctx, tenantID, sessionID, todoID, status, content)
		if err != nil {
			return nil, err
		}
		return map[string]any{"todo_list": result}, nil

	case "add":
		content, _ := input["content"].(string)
		var parentID *string
		if pid, ok := input["parent_id"].(string); ok {
			parentID = &pid
		}
		result, err := t.service.AddTodoItem(ctx, tenantID, sessionID, executor.CreateTodoInput{Content: content, ParentID: parentID})
		if err != nil {
			return nil, err
		}
		return map[string]any{"todo_list": result}, nil

	case "delete":
		todoID, _ := input["todo_id"].(string)
		result, err := t.service.DeleteTodoItem(ctx, tenantID, sessionID, todoID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"todo_list": result}, nil

	default:
		return nil, fmt.Errorf("未知操作: %s", action)
	}
}

// Validate 验证
func (t *TodoTool) Validate(input map[string]any) error {
	if _, ok := input["action"].(string); !ok {
		return fmt.Errorf("缺少 action 参数")
	}
	return nil
}

// GetDefinition 获取定义
func (t *TodoTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "todo-manager",
		DisplayName: "任务管理",
		Description: "会话级任务管理，支持创建、查询、更新、添加、删除任务",
		Category:    "workflow",
		Type:        "builtin",
		Status:      "active",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":     map[string]any{"type": "string", "enum": []string{"create", "get", "update", "add", "delete"}},
				"session_id": map[string]any{"type": "string"},
				"tenant_id":  map[string]any{"type": "string"},
				"items":      map[string]any{"type": "array", "description": "任务列表（create 时使用）"},
				"todo_id":    map[string]any{"type": "string"},
				"status":     map[string]any{"type": "string", "enum": []string{"pending", "completed"}},
				"content":    map[string]any{"type": "string"},
			},
			"required": []string{"action"},
		},
	}
}

// UserQuestionTool 用户问题工具
type UserQuestionTool struct {
	service *approval.UserQuestionService
}

// NewUserQuestionTool 创建用户问题工具
func NewUserQuestionTool(service *approval.UserQuestionService) *UserQuestionTool {
	return &UserQuestionTool{service: service}
}

// Execute 执行
func (t *UserQuestionTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	question, _ := input["question"].(string)
	optionsRaw, _ := input["options"].([]any)
	options := make([]string, 0, len(optionsRaw))
	for _, opt := range optionsRaw {
		if s, ok := opt.(string); ok {
			options = append(options, s)
		}
	}

	tenantID, _ := input["tenant_id"].(string)
	sessionID, _ := input["session_id"].(string)
	timeout := 300
	if v, ok := input["timeout"].(float64); ok {
		timeout = int(v)
	}

	result, err := t.service.AskQuestion(ctx, &approval.AskUserQuestionInput{
		TenantID:       tenantID,
		SessionID:      sessionID,
		Question:       question,
		Options:        options,
		AllowCustom:    true,
		TimeoutSeconds: timeout,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"selected": result.Selected, "custom_input": result.CustomInput}, nil
}

// Validate 验证
func (t *UserQuestionTool) Validate(input map[string]any) error {
	if _, ok := input["question"].(string); !ok {
		return fmt.Errorf("缺少 question 参数")
	}
	if opts, ok := input["options"].([]any); !ok || len(opts) < 2 {
		return fmt.Errorf("至少需要 2 个选项")
	}
	return nil
}

// GetDefinition 获取定义
func (t *UserQuestionTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "ask-user-question",
		DisplayName: "询问用户",
		Description: "向用户提出多选问题以澄清需求，工作流暂停直到用户选择",
		Category:    "workflow",
		Type:        "builtin",
		Status:      "active",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{"type": "string", "description": "要问用户的问题"},
				"options":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 2},
				"timeout":  map[string]any{"type": "number", "description": "超时秒数", "default": 300},
			},
			"required": []string{"question", "options"},
		},
	}
}

// FilesystemTool 文件系统工具
type FilesystemTool struct {
	service *workspace.FilesystemService
}

// NewFilesystemTool 创建文件系统工具
func NewFilesystemTool(basePath string) *FilesystemTool {
	return &FilesystemTool{service: workspace.NewFilesystemService(basePath)}
}

// Execute 执行
func (t *FilesystemTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	action, _ := input["action"].(string)

	switch action {
	case "read":
		filePath, _ := input["file_path"].(string)
		startLine, endLine := 0, 0
		if v, ok := input["start_line"].(float64); ok {
			startLine = int(v)
		}
		if v, ok := input["end_line"].(float64); ok {
			endLine = int(v)
		}
		result, err := t.service.ReadFile(ctx, filePath, startLine, endLine)
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": result}, nil

	case "create":
		filePath, _ := input["file_path"].(string)
		content, _ := input["content"].(string)
		createDirs := true
		if v, ok := input["create_dirs"].(bool); ok {
			createDirs = v
		}
		if err := t.service.CreateFile(ctx, filePath, content, createDirs); err != nil {
			return nil, err
		}
		return map[string]any{"success": true, "message": "文件创建成功"}, nil

	case "edit_search":
		filePath, _ := input["file_path"].(string)
		searchContent, _ := input["search_content"].(string)
		replaceContent, _ := input["replace_content"].(string)
		occurrence := 1
		if v, ok := input["occurrence"].(float64); ok {
			occurrence = int(v)
		}
		result, err := t.service.EditFileBySearch(ctx, filePath, searchContent, replaceContent, occurrence)
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": result}, nil

	case "edit_line":
		filePath, _ := input["file_path"].(string)
		startLine, _ := input["start_line"].(float64)
		endLine, _ := input["end_line"].(float64)
		newContent, _ := input["new_content"].(string)
		result, err := t.service.EditFileByLine(ctx, filePath, int(startLine), int(endLine), newContent)
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": result}, nil

	default:
		return nil, fmt.Errorf("未知操作: %s", action)
	}
}

// Validate 验证
func (t *FilesystemTool) Validate(input map[string]any) error {
	if _, ok := input["action"].(string); !ok {
		return fmt.Errorf("缺少 action 参数")
	}
	return nil
}

// GetDefinition 获取定义
func (t *FilesystemTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "filesystem",
		DisplayName: "文件系统操作",
		Description: "提供文件读取、创建、编辑功能，支持智能搜索替换和行号编辑",
		Category:    "filesystem",
		Type:        "builtin",
		Status:      "active",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":          map[string]any{"type": "string", "enum": []string{"read", "create", "edit_search", "edit_line"}},
				"file_path":       map[string]any{"type": "string"},
				"content":         map[string]any{"type": "string"},
				"search_content":  map[string]any{"type": "string"},
				"replace_content": map[string]any{"type": "string"},
				"start_line":      map[string]any{"type": "number"},
				"end_line":        map[string]any{"type": "number"},
				"new_content":     map[string]any{"type": "string"},
			},
			"required": []string{"action"},
		},
	}
}

// NotebookTool 代码备忘录工具
type NotebookTool struct {
	service *knowledge.NotebookService
}

// NewNotebookTool 创建备忘录工具
func NewNotebookTool(service *knowledge.NotebookService) *NotebookTool {
	return &NotebookTool{service: service}
}

// Execute 执行
func (t *NotebookTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	action, _ := input["action"].(string)
	tenantID, _ := input["tenant_id"].(string)

	switch action {
	case "add":
		filePath, _ := input["file_path"].(string)
		note, _ := input["note"].(string)
		result, err := t.service.AddNotebook(ctx, tenantID, filePath, note, nil)
		if err != nil {
			return nil, err
		}
		return map[string]any{"entry": result}, nil

	case "query":
		pattern, _ := input["pattern"].(string)
		topN := 10
		if v, ok := input["top_n"].(float64); ok {
			topN = int(v)
		}
		results, err := t.service.QueryNotebook(ctx, tenantID, pattern, topN)
		if err != nil {
			return nil, err
		}
		return map[string]any{"entries": results}, nil

	case "update":
		notebookID, _ := input["notebook_id"].(string)
		note, _ := input["note"].(string)
		result, err := t.service.UpdateNotebook(ctx, tenantID, notebookID, note)
		if err != nil {
			return nil, err
		}
		return map[string]any{"entry": result}, nil

	case "delete":
		notebookID, _ := input["notebook_id"].(string)
		if err := t.service.DeleteNotebook(ctx, tenantID, notebookID); err != nil {
			return nil, err
		}
		return map[string]any{"success": true}, nil

	default:
		return nil, fmt.Errorf("未知操作: %s", action)
	}
}

// Validate 验证
func (t *NotebookTool) Validate(input map[string]any) error {
	if _, ok := input["action"].(string); !ok {
		return fmt.Errorf("缺少 action 参数")
	}
	return nil
}

// GetDefinition 获取定义
func (t *NotebookTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "notebook",
		DisplayName: "代码备忘录",
		Description: "记录脆弱代码、注意事项，防止新功能破坏现有功能",
		Category:    "knowledge",
		Type:        "builtin",
		Status:      "active",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":      map[string]any{"type": "string", "enum": []string{"add", "query", "update", "delete"}},
				"file_path":   map[string]any{"type": "string"},
				"note":        map[string]any{"type": "string"},
				"pattern":     map[string]any{"type": "string"},
				"notebook_id": map[string]any{"type": "string"},
				"top_n":       map[string]any{"type": "number", "default": 10},
			},
			"required": []string{"action"},
		},
	}
}

// BashTool 命令执行工具
type BashTool struct {
	service *command.BashService
}

// NewBashTool 创建命令执行工具
func NewBashTool(workingDir string) *BashTool {
	return &BashTool{service: command.NewBashService(workingDir)}
}

// Execute 执行
func (t *BashTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	cmd, _ := input["command"].(string)
	timeout := 30000
	if v, ok := input["timeout"].(float64); ok {
		timeout = int(v)
	}

	result, err := t.service.Execute(ctx, cmd, time.Duration(timeout)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"stdout":      result.Stdout,
		"stderr":      result.Stderr,
		"exit_code":   result.ExitCode,
		"duration_ms": result.Duration,
	}, nil
}

// Validate 验证
func (t *BashTool) Validate(input map[string]any) error {
	cmd, ok := input["command"].(string)
	if !ok || cmd == "" {
		return fmt.Errorf("缺少 command 参数")
	}
	return t.service.ValidateCommand(cmd)
}

// GetDefinition 获取定义
func (t *BashTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "terminal-execute",
		DisplayName: "终端命令执行",
		Description: "安全执行终端命令，支持超时控制和危险命令检测",
		Category:    "system",
		Type:        "builtin",
		Status:      "active",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的命令"},
				"timeout": map[string]any{"type": "number", "description": "超时毫秒数", "default": 30000},
			},
			"required": []string{"command"},
		},
	}
}

// RegisterMCPTools 注册所有 MCP 工具
func RegisterMCPTools(registry *tools.ToolRegistry, basePath string, todoService *executor.TodoService, questionService *approval.UserQuestionService, notebookService *knowledge.NotebookService) error {
	// ACE 代码搜索
	aceTool := NewACESearchTool(basePath)
	if err := registry.Register(aceTool.GetDefinition().Name, aceTool, aceTool.GetDefinition()); err != nil {
		return err
	}

	// 代码库语义搜索
	codebaseTool := NewCodebaseSearchTool(basePath)
	if err := registry.Register(codebaseTool.GetDefinition().Name, codebaseTool, codebaseTool.GetDefinition()); err != nil {
		return err
	}

	// 任务管理
	if todoService != nil {
		todoTool := NewTodoTool(todoService)
		if err := registry.Register(todoTool.GetDefinition().Name, todoTool, todoTool.GetDefinition()); err != nil {
			return err
		}
	}

	// 用户问题
	if questionService != nil {
		questionTool := NewUserQuestionTool(questionService)
		if err := registry.Register(questionTool.GetDefinition().Name, questionTool, questionTool.GetDefinition()); err != nil {
			return err
		}
	}

	// 文件系统
	fsTool := NewFilesystemTool(basePath)
	if err := registry.Register(fsTool.GetDefinition().Name, fsTool, fsTool.GetDefinition()); err != nil {
		return err
	}

	// 备忘录
	if notebookService != nil {
		notebookTool := NewNotebookTool(notebookService)
		if err := registry.Register(notebookTool.GetDefinition().Name, notebookTool, notebookTool.GetDefinition()); err != nil {
			return err
		}
	}

	// 终端命令
	bashTool := NewBashTool(basePath)
	if err := registry.Register(bashTool.GetDefinition().Name, bashTool, bashTool.GetDefinition()); err != nil {
		return err
	}

	return nil
}
