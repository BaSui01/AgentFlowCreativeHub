package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DebugSession 调试会话
type DebugSession struct {
	ID           string                 `json:"id"`
	WorkflowID   string                 `json:"workflowId"`
	ExecutionID  string                 `json:"executionId"`
	TenantID     string                 `json:"tenantId"`
	UserID       string                 `json:"userId"`
	Status       DebugStatus            `json:"status"`
	CurrentStep  string                 `json:"currentStep"`
	Breakpoints  map[string]bool        `json:"breakpoints"`  // stepID -> enabled
	Variables    map[string]interface{} `json:"variables"`    // 当前变量
	StepHistory  []DebugStepRecord      `json:"stepHistory"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

// DebugStatus 调试状态
type DebugStatus string

const (
	DebugStatusPaused    DebugStatus = "paused"    // 暂停在断点
	DebugStatusRunning   DebugStatus = "running"   // 运行中
	DebugStatusStepping  DebugStatus = "stepping"  // 单步执行中
	DebugStatusCompleted DebugStatus = "completed" // 完成
	DebugStatusError     DebugStatus = "error"     // 错误
)

// DebugStepRecord 步骤执行记录
type DebugStepRecord struct {
	StepID     string                 `json:"stepId"`
	StepName   string                 `json:"stepName"`
	StepType   string                 `json:"stepType"`
	Input      map[string]interface{} `json:"input"`
	Output     map[string]interface{} `json:"output,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
	StartTime  time.Time              `json:"startTime"`
	EndTime    time.Time              `json:"endTime"`
	Status     string                 `json:"status"` // pending, running, completed, failed, skipped
}

// DebugCommand 调试命令
type DebugCommand string

const (
	CmdContinue  DebugCommand = "continue"   // 继续执行
	CmdStepOver  DebugCommand = "step_over"  // 单步跳过
	CmdStepInto  DebugCommand = "step_into"  // 单步进入
	CmdStepOut   DebugCommand = "step_out"   // 单步跳出
	CmdPause     DebugCommand = "pause"      // 暂停
	CmdStop      DebugCommand = "stop"       // 停止
	CmdRestart   DebugCommand = "restart"    // 重新开始
	CmdEvaluate  DebugCommand = "evaluate"   // 求值表达式
)

// WorkflowDebugger 工作流调试器
type WorkflowDebugger struct {
	db       *gorm.DB
	sessions sync.Map // sessionID -> *DebugSession
	commands sync.Map // sessionID -> chan DebugCommand
	results  sync.Map // sessionID -> chan DebugStepRecord
}

// NewWorkflowDebugger 创建工作流调试器
func NewWorkflowDebugger(db *gorm.DB) *WorkflowDebugger {
	return &WorkflowDebugger{db: db}
}

// StartDebugSession 开始调试会话
func (d *WorkflowDebugger) StartDebugSession(ctx context.Context, workflowID, tenantID, userID string, breakpoints []string) (*DebugSession, error) {
	session := &DebugSession{
		ID:          uuid.New().String(),
		WorkflowID:  workflowID,
		TenantID:    tenantID,
		UserID:      userID,
		Status:      DebugStatusPaused,
		Breakpoints: make(map[string]bool),
		Variables:   make(map[string]interface{}),
		StepHistory: make([]DebugStepRecord, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置断点
	for _, bp := range breakpoints {
		session.Breakpoints[bp] = true
	}

	// 存储会话
	d.sessions.Store(session.ID, session)

	// 创建命令通道
	cmdChan := make(chan DebugCommand, 10)
	d.commands.Store(session.ID, cmdChan)

	// 创建结果通道
	resultChan := make(chan DebugStepRecord, 100)
	d.results.Store(session.ID, resultChan)

	return session, nil
}

// GetSession 获取调试会话
func (d *WorkflowDebugger) GetSession(sessionID string) (*DebugSession, error) {
	if val, ok := d.sessions.Load(sessionID); ok {
		return val.(*DebugSession), nil
	}
	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// SetBreakpoint 设置断点
func (d *WorkflowDebugger) SetBreakpoint(sessionID, stepID string, enabled bool) error {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Breakpoints[stepID] = enabled
	session.UpdatedAt = time.Now()
	return nil
}

// RemoveBreakpoint 移除断点
func (d *WorkflowDebugger) RemoveBreakpoint(sessionID, stepID string) error {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return err
	}

	delete(session.Breakpoints, stepID)
	session.UpdatedAt = time.Now()
	return nil
}

// SendCommand 发送调试命令
func (d *WorkflowDebugger) SendCommand(sessionID string, cmd DebugCommand) error {
	cmdChanVal, ok := d.commands.Load(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	cmdChan := cmdChanVal.(chan DebugCommand)
	select {
	case cmdChan <- cmd:
		return nil
	default:
		return fmt.Errorf("command channel full")
	}
}

// WaitForCommand 等待调试命令
func (d *WorkflowDebugger) WaitForCommand(ctx context.Context, sessionID string) (DebugCommand, error) {
	cmdChanVal, ok := d.commands.Load(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	cmdChan := cmdChanVal.(chan DebugCommand)
	select {
	case cmd := <-cmdChan:
		return cmd, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// RecordStep 记录步骤执行
func (d *WorkflowDebugger) RecordStep(sessionID string, record DebugStepRecord) error {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.StepHistory = append(session.StepHistory, record)
	session.CurrentStep = record.StepID
	session.UpdatedAt = time.Now()

	// 发送到结果通道
	if resultChanVal, ok := d.results.Load(sessionID); ok {
		resultChan := resultChanVal.(chan DebugStepRecord)
		select {
		case resultChan <- record:
		default:
			// 通道满了，跳过
		}
	}

	return nil
}

// UpdateVariables 更新变量
func (d *WorkflowDebugger) UpdateVariables(sessionID string, variables map[string]interface{}) error {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return err
	}

	for k, v := range variables {
		session.Variables[k] = v
	}
	session.UpdatedAt = time.Now()
	return nil
}

// SetVariable 设置单个变量
func (d *WorkflowDebugger) SetVariable(sessionID, name string, value interface{}) error {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Variables[name] = value
	session.UpdatedAt = time.Now()
	return nil
}

// GetVariable 获取变量值
func (d *WorkflowDebugger) GetVariable(sessionID, name string) (interface{}, error) {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.Variables[name], nil
}

// EvaluateExpression 求值表达式
func (d *WorkflowDebugger) EvaluateExpression(sessionID, expression string) (interface{}, error) {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	// 简单的变量替换 (实际可以使用表达式引擎)
	if val, ok := session.Variables[expression]; ok {
		return val, nil
	}

	return nil, fmt.Errorf("expression evaluation not supported: %s", expression)
}

// ShouldBreak 检查是否应该在此步骤暂停
func (d *WorkflowDebugger) ShouldBreak(sessionID, stepID string) bool {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return false
	}

	// 检查断点
	if enabled, ok := session.Breakpoints[stepID]; ok && enabled {
		return true
	}

	// 检查是否在单步模式
	return session.Status == DebugStatusStepping
}

// UpdateStatus 更新调试状态
func (d *WorkflowDebugger) UpdateStatus(sessionID string, status DebugStatus) error {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Status = status
	session.UpdatedAt = time.Now()
	return nil
}

// EndSession 结束调试会话
func (d *WorkflowDebugger) EndSession(sessionID string) error {
	// 关闭通道
	if cmdChanVal, ok := d.commands.Load(sessionID); ok {
		close(cmdChanVal.(chan DebugCommand))
		d.commands.Delete(sessionID)
	}

	if resultChanVal, ok := d.results.Load(sessionID); ok {
		close(resultChanVal.(chan DebugStepRecord))
		d.results.Delete(sessionID)
	}

	// 删除会话
	d.sessions.Delete(sessionID)

	return nil
}

// GetStepHistory 获取步骤历史
func (d *WorkflowDebugger) GetStepHistory(sessionID string) ([]DebugStepRecord, error) {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.StepHistory, nil
}

// SubscribeResults 订阅执行结果
func (d *WorkflowDebugger) SubscribeResults(sessionID string) (<-chan DebugStepRecord, error) {
	resultChanVal, ok := d.results.Load(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return resultChanVal.(chan DebugStepRecord), nil
}

// DebugWorkflowExecution 调试执行工作流的包装器
type DebugWorkflowExecution struct {
	debugger  *WorkflowDebugger
	sessionID string
}

// NewDebugWorkflowExecution 创建调试执行包装器
func NewDebugWorkflowExecution(debugger *WorkflowDebugger, sessionID string) *DebugWorkflowExecution {
	return &DebugWorkflowExecution{
		debugger:  debugger,
		sessionID: sessionID,
	}
}

// BeforeStep 步骤执行前的钩子
func (e *DebugWorkflowExecution) BeforeStep(ctx context.Context, stepID, stepName, stepType string, input map[string]interface{}) error {
	// 检查是否应该暂停
	if e.debugger.ShouldBreak(e.sessionID, stepID) {
		e.debugger.UpdateStatus(e.sessionID, DebugStatusPaused)

		// 记录当前状态
		record := DebugStepRecord{
			StepID:    stepID,
			StepName:  stepName,
			StepType:  stepType,
			Input:     input,
			Status:    "paused",
			StartTime: time.Now(),
		}
		e.debugger.RecordStep(e.sessionID, record)

		// 等待用户命令
		cmd, err := e.debugger.WaitForCommand(ctx, e.sessionID)
		if err != nil {
			return err
		}

		switch cmd {
		case CmdContinue:
			e.debugger.UpdateStatus(e.sessionID, DebugStatusRunning)
		case CmdStepOver:
			e.debugger.UpdateStatus(e.sessionID, DebugStatusStepping)
		case CmdStop:
			return fmt.Errorf("debug session stopped by user")
		}
	}

	return nil
}

// AfterStep 步骤执行后的钩子
func (e *DebugWorkflowExecution) AfterStep(ctx context.Context, stepID string, output map[string]interface{}, stepErr error, duration time.Duration) error {
	status := "completed"
	errMsg := ""
	if stepErr != nil {
		status = "failed"
		errMsg = stepErr.Error()
	}

	record := DebugStepRecord{
		StepID:   stepID,
		Output:   output,
		Error:    errMsg,
		Duration: duration,
		EndTime:  time.Now(),
		Status:   status,
	}

	return e.debugger.RecordStep(e.sessionID, record)
}

// DebugSessionPersistence 调试会话持久化模型
type DebugSessionPersistence struct {
	ID          string          `json:"id" gorm:"primaryKey;size:36"`
	WorkflowID  string          `json:"workflowId" gorm:"size:36;index"`
	TenantID    string          `json:"tenantId" gorm:"size:36;index"`
	UserID      string          `json:"userId" gorm:"size:36"`
	Status      string          `json:"status" gorm:"size:20"`
	SessionData json.RawMessage `json:"sessionData" gorm:"type:jsonb"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// SaveSession 保存调试会话到数据库
func (d *WorkflowDebugger) SaveSession(ctx context.Context, sessionID string) error {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return err
	}

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	persistence := &DebugSessionPersistence{
		ID:          session.ID,
		WorkflowID:  session.WorkflowID,
		TenantID:    session.TenantID,
		UserID:      session.UserID,
		Status:      string(session.Status),
		SessionData: data,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   time.Now(),
	}

	return d.db.WithContext(ctx).Save(persistence).Error
}

// LoadSession 从数据库加载调试会话
func (d *WorkflowDebugger) LoadSession(ctx context.Context, sessionID string) (*DebugSession, error) {
	var persistence DebugSessionPersistence
	if err := d.db.WithContext(ctx).Where("id = ?", sessionID).First(&persistence).Error; err != nil {
		return nil, err
	}

	var session DebugSession
	if err := json.Unmarshal(persistence.SessionData, &session); err != nil {
		return nil, err
	}

	// 恢复到内存
	d.sessions.Store(session.ID, &session)

	// 重新创建通道
	cmdChan := make(chan DebugCommand, 10)
	d.commands.Store(session.ID, cmdChan)

	resultChan := make(chan DebugStepRecord, 100)
	d.results.Store(session.ID, resultChan)

	return &session, nil
}

// ListUserSessions 列出用户的调试会话
func (d *WorkflowDebugger) ListUserSessions(ctx context.Context, tenantID, userID string) ([]*DebugSessionPersistence, error) {
	var sessions []*DebugSessionPersistence
	err := d.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("updated_at DESC").
		Limit(50).
		Find(&sessions).Error
	return sessions, err
}
