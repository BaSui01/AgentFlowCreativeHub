package executor

import (
	"fmt"

	workflowpkg "backend/internal/workflow"
)

// WorkflowDefinition 工作流定义
type WorkflowDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Version     string           `json:"version"`
	Steps       []StepDefinition `json:"steps"`

	// 自动化配置
	AutomationConfig *AutomationConfig `json:"automation_config,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`
}

// AutomationConfig 自动化配置
type AutomationConfig struct {
	Mode           string             `json:"mode"`            // full_auto（全自动）、semi_auto（半自动）、manual（手动）
	MaxRounds      int                `json:"max_rounds"`      // 最大执行轮次（用于自动对话）
	StopConditions []StopCondition    `json:"stop_conditions"` // 停止条件
	AgentSwitching *AgentSwitchConfig `json:"agent_switching"` // Agent 切换配置
	DefaultRetry   *RetryConfig       `json:"default_retry"`   // 默认重试配置
}

// StopCondition 停止条件
type StopCondition struct {
	Type      string  `json:"type"`      // goal_achieved、max_rounds、user_interrupt、error_threshold
	Threshold float64 `json:"threshold"` // 阈值（针对 error_threshold 等）
}

// AgentSwitchConfig Agent 切换配置
type AgentSwitchConfig struct {
	Mode  string            `json:"mode"`  // static（固定顺序）、dynamic（动态切换）
	Rules []AgentSwitchRule `json:"rules"` // 切换规则
}

// AgentSwitchRule Agent 切换规则
type AgentSwitchRule struct {
	Condition string `json:"condition"`  // 条件表达式（如 "quality_score < 80"）
	NextAgent string `json:"next_agent"` // 下一个 Agent
	Priority  int    `json:"priority"`   // 优先级（数字越小优先级越高）
}

// StepDefinition 步骤定义
type StepDefinition struct {
	ID        string  `json:"id"`         // 步骤唯一标识
	Name      string  `json:"name"`       // 步骤名称
	Type      string  `json:"type"`       // 步骤类型（agent、condition、loop、approval等）
	AgentType string  `json:"agent_type"` // Agent 类型（如果 type=agent）
	AgentID   *string `json:"agent_id"`   // Agent ID（可选，优先级高于 agent_type）

	// Agent 角色定制
	Role                 string `json:"role"`                   // Agent 角色（如 outline_reviewer、content_reviewer）
	SystemPromptOverride string `json:"system_prompt_override"` // 覆盖默认 System Prompt

	Input     map[string]any `json:"input"`      // 输入配置
	Output    string         `json:"output"`     // 输出变量名
	DependsOn []string       `json:"depends_on"` // 依赖的步骤 ID
	Condition *Condition     `json:"condition"`  // 条件（可选）
	Retry     *RetryConfig   `json:"retry"`      // 重试配置（可选）
	Timeout   int            `json:"timeout"`    // 超时时间（秒，可选）
	Parallel  bool           `json:"parallel"`   // 是否并行执行

	// 自动化控制字段
	AutoExecute      bool                `json:"auto_execute"`      // 是否自动执行（默认true）
	ApprovalRequired bool                `json:"approval_required"` // 是否需要审批（默认false）
	ApprovalConfig   *ApprovalConfig     `json:"approval_config"`   // 审批配置（可选）
	QualityCheck     *QualityCheckConfig `json:"quality_check"`     // 质量检查配置（可选）

	MapReduce *MapReduceConfig `json:"map_reduce"` // Map-Reduce 配置（可选）

	ExtraConfig map[string]any `json:"extra_config"` // 额外配置
}

// MapReduceConfig MapReduce 配置
type MapReduceConfig struct {
	Enabled        bool   `json:"enabled"`
	IterateOn      string `json:"iterate_on"`      // 迭代的列表字段名 (如 "items")
	ItemVariable   string `json:"item_variable"`   // 子任务中的变量名 (如 "item")
	MaxConcurrency int    `json:"max_concurrency"` // 最大并发数
	ReducerAgent   string `json:"reducer_agent"`   // (可选) 汇总 Agent ID
}

// ApprovalConfig 审批配置
type ApprovalConfig struct {
	Type           string              `json:"type"`            // required（强制）、optional（可选）、conditional（条件）
	AutoApproveIf  *Condition          `json:"auto_approve_if"` // 自动批准条件
	TimeoutSeconds int                 `json:"timeout_seconds"` // 审批超时时间（秒）
	NotifyChannels []string            `json:"notify_channels"` // 通知渠道（email、webhook、websocket）
	NotifyTargets  map[string][]string `json:"notify_targets"`  // 通知目标配置（email/webhook/websocket）
}

// QualityCheckConfig 质量检查配置
type QualityCheckConfig struct {
	Enabled      bool    `json:"enabled"`       // 是否启用
	MinScore     float64 `json:"min_score"`     // 最小质量分数（0-100）
	RetryOnFail  bool    `json:"retry_on_fail"` // 质量不达标时是否重试
	RewriteAgent string  `json:"rewrite_agent"` // 质量不达标时使用的重写 Agent
}

// Condition 条件
type Condition struct {
	Expression string `json:"expression"` // 条件表达式
	OnTrue     string `json:"on_true"`    // 条件为真时执行的步骤
	OnFalse    string `json:"on_false"`   // 条件为假时执行的步骤
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int    `json:"max_retries"` // 最大重试次数
	Backoff    string `json:"backoff"`     // 退避策略（fixed、exponential）
	Delay      int    `json:"delay"`       // 延迟时间（秒）
}

// DAG 有向无环图
type DAG struct {
	Nodes map[string]*Node    // 节点映射（ID -> Node）
	Edges map[string][]string // 边映射（ID -> 依赖的节点 ID 列表）
}

// Node DAG 节点
type Node struct {
	ID           string
	Step         *StepDefinition
	Level        int      // 层级（用于拓扑排序）
	Dependencies []string // 依赖的节点 ID
	Dependents   []string // 依赖此节点的节点 ID
}

// Parser 工作流解析器
type Parser struct{}

// NewParser 创建解析器
func NewParser() *Parser {
	return &Parser{}
}

// Parse 解析工作流定义
func (p *Parser) Parse(def workflowpkg.WorkflowDefinition) (*WorkflowDefinition, error) {
	// 1. 构建节点映射
	steps := make([]StepDefinition, 0, len(def.Nodes))

	// 2. 构建依赖关系映射 (Target -> []Source)
	dependencies := make(map[string][]string)
	for _, edge := range def.Edges {
		dependencies[edge.Target] = append(dependencies[edge.Target], edge.Source)
	}

	// 3. 转换 Node 到 StepDefinition
	for _, node := range def.Nodes {
		// 跳过 Start 和 End 节点，或者将其转换为特殊步骤？
		// 目前 Executor 可能主要关注 Agent 节点
		// 但为了完整性，Start 节点可能包含初始 Input，End 节点包含最终 Output

		// 暂时简单处理：将 Agent, Tool, Approval, Router 转换为 Step
		// Start/End 节点暂时忽略，或者作为 Passthrough？
		// 在新的 Graph 模型中，数据流是通过 Inputs 映射的，不像旧模型是隐式传递。

		// 为了兼容旧的 Executor 逻辑，我们需要适配

		step := StepDefinition{
			ID:    node.ID,
			Name:  node.Data.Label,
			Type:  string(node.Type),
			Input: make(map[string]any),
		}

		// 转换输入
		for k, v := range node.Data.Inputs {
			step.Input[k] = v
		}

		// 转换 Agent 配置
		if node.Type == workflowpkg.NodeTypeAgent {
			step.AgentID = &node.Data.AgentID
			// 从 AgentConfig 中提取 AgentType (如果存在)
			if at, ok := node.Data.AgentConfig["agentType"].(string); ok {
				step.AgentType = at
			} else if at, ok := node.Data.AgentConfig["role"].(string); ok {
				step.AgentType = at // 兼容旧习惯
			}

			// 合并 AgentConfig 到 ExtraConfig
			if len(node.Data.AgentConfig) > 0 {
				if step.ExtraConfig == nil {
					step.ExtraConfig = make(map[string]any)
				}
				for k, v := range node.Data.AgentConfig {
					step.ExtraConfig[k] = v
				}
			}

			step.Output = "output" // 默认输出变量名
		}

		// 设置依赖
		if deps, ok := dependencies[node.ID]; ok {
			step.DependsOn = deps
		}

		// 处理 Approval
		if node.Type == workflowpkg.NodeTypeApproval {
			step.ApprovalRequired = true
			step.ApprovalConfig = &ApprovalConfig{
				Type:           "required",
				TimeoutSeconds: node.Data.Timeout,
				NotifyTargets: map[string][]string{
					"user": node.Data.Approvers,
				},
			}
		}

		steps = append(steps, step)
	}

	// 构建内部 WorkflowDefinition
	workflow := &WorkflowDefinition{
		Steps: steps,
	}

	return workflow, nil
}

// parseStep 解析单个步骤 (不再需要，但保留以防有引用)
func (p *Parser) parseStep(stepMap map[string]any) (*StepDefinition, error) {
	return nil, fmt.Errorf("deprecated")
}

// BuildDAG 构建 DAG
func (p *Parser) BuildDAG(workflow *WorkflowDefinition) (*DAG, error) {
	dag := &DAG{
		Nodes: make(map[string]*Node),
		Edges: make(map[string][]string),
	}

	// 创建节点
	for i := range workflow.Steps {
		step := &workflow.Steps[i]
		node := &Node{
			ID:           step.ID,
			Step:         step,
			Level:        0,
			Dependencies: step.DependsOn,
			Dependents:   make([]string, 0),
		}
		dag.Nodes[step.ID] = node
		dag.Edges[step.ID] = step.DependsOn
	}

	// 检查节点是否存在
	for id, deps := range dag.Edges {
		for _, depID := range deps {
			if _, exists := dag.Nodes[depID]; !exists {
				return nil, fmt.Errorf("步骤 %s 依赖的步骤 %s 不存在", id, depID)
			}
		}
	}

	// 构建 Dependents（反向依赖）
	for id, deps := range dag.Edges {
		for _, depID := range deps {
			dag.Nodes[depID].Dependents = append(dag.Nodes[depID].Dependents, id)
		}
	}

	// 检测循环依赖
	if err := p.detectCycle(dag); err != nil {
		return nil, err
	}

	// 计算层级（拓扑排序）
	if err := p.calculateLevels(dag); err != nil {
		return nil, err
	}

	return dag, nil
}

// detectCycle 检测循环依赖
func (p *Parser) detectCycle(dag *DAG) error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for id := range dag.Nodes {
		if !visited[id] {
			if p.hasCycle(id, dag, visited, recStack) {
				return fmt.Errorf("检测到循环依赖")
			}
		}
	}

	return nil
}

// hasCycle DFS 检测循环
func (p *Parser) hasCycle(nodeID string, dag *DAG, visited, recStack map[string]bool) bool {
	visited[nodeID] = true
	recStack[nodeID] = true

	for _, depID := range dag.Edges[nodeID] {
		if !visited[depID] {
			if p.hasCycle(depID, dag, visited, recStack) {
				return true
			}
		} else if recStack[depID] {
			return true
		}
	}

	recStack[nodeID] = false
	return false
}

// calculateLevels 计算节点层级（拓扑排序）
func (p *Parser) calculateLevels(dag *DAG) error {
	// 计算入度
	inDegree := make(map[string]int)
	for id := range dag.Nodes {
		inDegree[id] = len(dag.Edges[id])
	}

	// 找到所有入度为 0 的节点（起始节点）
	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
			dag.Nodes[id].Level = 0
		}
	}

	// BFS 计算层级
	processed := 0
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		processed++

		currentLevel := dag.Nodes[nodeID].Level

		// 处理依赖此节点的节点
		for _, dependentID := range dag.Nodes[nodeID].Dependents {
			inDegree[dependentID]--

			// 更新层级（取所有依赖中的最大层级 + 1）
			if dag.Nodes[dependentID].Level < currentLevel+1 {
				dag.Nodes[dependentID].Level = currentLevel + 1
			}

			if inDegree[dependentID] == 0 {
				queue = append(queue, dependentID)
			}
		}
	}

	// 检查是否所有节点都被处理（再次检测循环）
	if processed != len(dag.Nodes) {
		return fmt.Errorf("存在循环依赖或孤立节点")
	}

	return nil
}

// GetExecutionOrder 获取执行顺序（按层级分组）
func (p *Parser) GetExecutionOrder(dag *DAG) [][]string {
	// 找到最大层级
	maxLevel := 0
	for _, node := range dag.Nodes {
		if node.Level > maxLevel {
			maxLevel = node.Level
		}
	}

	// 按层级分组
	levels := make([][]string, maxLevel+1)
	for id, node := range dag.Nodes {
		levels[node.Level] = append(levels[node.Level], id)
	}

	return levels
}
