package workflow

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// WorkflowDefinition 工作流定义结构体
type WorkflowDefinition struct {
	Nodes    []Node   `json:"nodes,omitempty"`
	Edges    []Edge   `json:"edges,omitempty"`
	Viewport Viewport `json:"viewport,omitempty"` // 前端视图状态

	// 新版自动化工作流定义字段
	Name             string           `json:"name,omitempty"`
	Description      string           `json:"description,omitempty"`
	Version          string           `json:"version,omitempty"`
	AutomationConfig map[string]any   `json:"automation_config,omitempty"`
	Steps            []map[string]any `json:"steps,omitempty"`
	Metadata         map[string]any   `json:"metadata,omitempty"`
}

// Value 实现 driver.Valuer 接口，用于 GORM 存储 JSONB
func (w WorkflowDefinition) Value() (driver.Value, error) {
	return json.Marshal(w)
}

// Scan 实现 sql.Scanner 接口，用于 GORM 读取 JSONB
func (w *WorkflowDefinition) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, &w)
}

// Node 工作流节点
type Node struct {
	ID       string   `json:"id"`
	Type     NodeType `json:"type"` // start, end, agent, tool, router, approval
	Position Position `json:"position"`
	Data     NodeData `json:"data"`
	// UI 相关属性
	Width    int  `json:"width,omitempty"`
	Height   int  `json:"height,omitempty"`
	Selected bool `json:"selected,omitempty"`
}

// NodeType 节点类型枚举
type NodeType string

const (
	NodeTypeStart    NodeType = "start"
	NodeTypeEnd      NodeType = "end"
	NodeTypeAgent    NodeType = "agent"
	NodeTypeTool     NodeType = "tool"
	NodeTypeRouter   NodeType = "router"   // 路由/条件分支
	NodeTypeApproval NodeType = "approval" // 人工审批
)

// Position 节点坐标
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeData 节点配置数据
type NodeData struct {
	Label string `json:"label"`

	// 通用配置
	Description string `json:"description,omitempty"`

	// Agent 节点配置
	AgentID        string         `json:"agentId,omitempty"`
	AgentConfig    map[string]any `json:"agentConfig,omitempty"` // 覆盖 Agent 默认配置
	PromptTemplate string         `json:"promptTemplate,omitempty"`

	// Tool 节点配置
	ToolID     string         `json:"toolId,omitempty"`
	ToolParams map[string]any `json:"toolParams,omitempty"`

	// Router 节点配置
	Conditions []Condition `json:"conditions,omitempty"`

	// Approval 节点配置
	Approvers []string `json:"approvers,omitempty"` // 审批人 UserID 列表
	Timeout   int      `json:"timeout,omitempty"`   // 超时时间(秒)

	// 输入变量映射 (Input Mapping)
	// key: 当前节点所需的变量名
	// value: 来源变量表达式，如 "{{step_1.output.summary}}"
	Inputs map[string]string `json:"inputs,omitempty"`
}

// Condition 分支条件
type Condition struct {
	ID         string `json:"id"`
	Expression string `json:"expression"` // 表达式，如 "score > 80"
	TargetNode string `json:"targetNode"` // 满足条件时流向的节点 ID (逻辑上，实际上由 Edge 决定)
	Label      string `json:"label"`      // 分支名称
}

// Edge 连接线
type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle,omitempty"` // 连接源锚点 ID
	TargetHandle string `json:"targetHandle,omitempty"` // 连接目标锚点 ID
	Type         string `json:"type,omitempty"`         // default, smoothstep
	Label        string `json:"label,omitempty"`
	Animated     bool   `json:"animated,omitempty"`

	// 逻辑属性
	ConditionID string `json:"conditionId,omitempty"` // 关联的条件分支 ID
}

// Viewport 前端视口状态
type Viewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

// Workflow 工作流定义
type Workflow struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string `json:"tenantId" gorm:"type:uuid;not null;index"`
	OwnerUserID string `json:"ownerUserId" gorm:"type:uuid;index"`

	// 工作流信息
	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`

	// 定义（结构化）
	Definition WorkflowDefinition `json:"definition" gorm:"type:jsonb;not null;serializer:json"`

	// 版本
	Version string `json:"version" gorm:"size:50;not null"`

	// 可见性
	Visibility string `json:"visibility" gorm:"size:50;not null;default:personal"` // personal, tenant, public

	// 统计（通过触发器自动维护）
	TotalExecutions   int `json:"totalExecutions" gorm:"column:execution_count;default:0"`
	SuccessExecutions int `json:"successExecutions" gorm:"column:success_count;default:0"`
	FailedExecutions  int `json:"failedExecutions" gorm:"column:failure_count;default:0"`

	// 创建人
	CreatedBy string `json:"createdBy" gorm:"size:100"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
	DeletedBy string     `json:"deletedBy,omitempty" gorm:"size:100"`
}

// WorkflowExecution 工作流执行记录
type WorkflowExecution struct {
	ID         string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID   string `json:"tenantId" gorm:"type:uuid;not null;index"`
	WorkflowID string `json:"workflowId" gorm:"type:uuid;not null;index"`
	UserID     string `json:"userId" gorm:"type:uuid;index"`

	// 状态
	Status string `json:"status" gorm:"size:50;not null;default:pending"` // pending, running, completed, failed, paused

	// 输入输出
	Input        map[string]any `json:"input" gorm:"type:jsonb;serializer:json"`
	Output       map[string]any `json:"output" gorm:"type:jsonb;serializer:json"`
	ErrorMessage string         `json:"errorMessage" gorm:"type:text"`

	// 时间
	StartedAt   *time.Time `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt"`

	// 追踪
	TraceID string `json:"traceId" gorm:"size:100;index"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime;index"`
}

// WorkflowTask 工作流任务
type WorkflowTask struct {
	ID          string `json:"id" gorm:"primaryKey;type:uuid"`
	ExecutionID string `json:"executionId" gorm:"type:uuid;not null;index"`

	// 任务信息
	StepID    string `json:"stepId" gorm:"size:100;not null"`
	AgentType string `json:"agentType" gorm:"size:100;not null"`

	// 状态
	Status string `json:"status" gorm:"size:50;not null;default:pending"`

	// 输入输出
	Input        map[string]any `json:"input" gorm:"type:jsonb;serializer:json"`
	Output       map[string]any `json:"output" gorm:"type:jsonb;serializer:json"`
	ErrorMessage string         `json:"errorMessage" gorm:"type:text"`

	// 时间
	StartedAt   *time.Time `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt"`

	// 重试
	RetryCount int `json:"retryCount" gorm:"default:0"`

	// 时间戳
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
}
