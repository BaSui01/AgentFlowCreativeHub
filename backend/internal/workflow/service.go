package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"backend/internal/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowService 工作流管理服务
type WorkflowService struct {
	db *gorm.DB
}

// NewWorkflowService 创建 WorkflowService 实例
func NewWorkflowService(db *gorm.DB) *WorkflowService {
	return &WorkflowService{db: db}
}

// ListWorkflowsRequest 查询工作流列表请求
type ListWorkflowsRequest struct {
	TenantID   string
	Visibility string // personal, tenant, public
	CreatedBy  string
	Page       int
	PageSize   int
}

// ListWorkflowsResponse 查询工作流列表响应
type ListWorkflowsResponse struct {
	Workflows  []*Workflow `json:"workflows"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// ListWorkflows 查询工作流列表
func (s *WorkflowService) ListWorkflows(ctx context.Context, req *ListWorkflowsRequest) (*ListWorkflowsResponse, error) {
	// 构建查询
	query := s.db.WithContext(ctx).
		Model(&Workflow{}).
		Scopes(common.NotDeleted())

	// 租户过滤
	if req.TenantID != "" {
		query = query.Where("tenant_id = ?", req.TenantID)
	}

	// 可见性过滤
	if req.Visibility != "" {
		query = query.Where("visibility = ?", req.Visibility)
	}

	// 创建者过滤
	if req.CreatedBy != "" {
		query = query.Where("created_by = ?", req.CreatedBy)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("统计工作流数量失败: %w", err)
	}

	// 分页
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 查询列表
	var workflows []*Workflow
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&workflows).Error; err != nil {
		return nil, fmt.Errorf("查询工作流列表失败: %w", err)
	}

	// 计算总页数
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &ListWorkflowsResponse{
		Workflows:  workflows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetWorkflow 查询单个工作流
func (s *WorkflowService) GetWorkflow(ctx context.Context, tenantID, workflowID string) (*Workflow, error) {
	var workflow Workflow
	if err := s.db.WithContext(ctx).
		Scopes(common.NotDeleted()).
		Where("id = ? AND (tenant_id = ? OR visibility = ?)", workflowID, tenantID, "public").
		First(&workflow).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("工作流不存在")
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}
	return &workflow, nil
}

// CreateWorkflowRequest 创建工作流请求
type CreateWorkflowRequest struct {
	TenantID    string
	Name        string
	Description string
	Definition  WorkflowDefinition
	Version     string
	Visibility  string
	CreatedBy   string
}

// CreateWorkflow 创建工作流
func (s *WorkflowService) CreateWorkflow(ctx context.Context, req *CreateWorkflowRequest) (*Workflow, error) {
	// 验证必填字段
	if req.Name == "" {
		return nil, fmt.Errorf("工作流名称不能为空")
	}
	// 简单的非空验证
	if len(req.Definition.Nodes) == 0 {
		return nil, fmt.Errorf("工作流节点不能为空")
	}
	if req.Version == "" {
		req.Version = "v1.0"
	}
	if req.Visibility == "" {
		req.Visibility = "personal"
	}

	// 验证工作流定义
	if err := s.validateWorkflowDefinition(req.Definition); err != nil {
		return nil, fmt.Errorf("工作流定义无效: %w", err)
	}

	// 创建工作流
	workflow := &Workflow{
		ID:                uuid.New().String(),
		TenantID:          req.TenantID,
		Name:              req.Name,
		Description:       req.Description,
		Definition:        req.Definition,
		Version:           req.Version,
		Visibility:        req.Visibility,
		TotalExecutions:   0,
		SuccessExecutions: 0,
		FailedExecutions:  0,
		CreatedBy:         req.CreatedBy,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(workflow).Error; err != nil {
		return nil, fmt.Errorf("创建工作流失败: %w", err)
	}

	return workflow, nil
}

// UpdateWorkflowRequest 更新工作流请求
type UpdateWorkflowRequest struct {
	Name        *string
	Description *string
	Definition  *WorkflowDefinition
	Version     *string
	Visibility  *string
}

// UpdateWorkflow 更新工作流
func (s *WorkflowService) UpdateWorkflow(ctx context.Context, tenantID, workflowID string, req *UpdateWorkflowRequest) (*Workflow, error) {
	// 查询工作流
	workflow, err := s.GetWorkflow(ctx, tenantID, workflowID)
	if err != nil {
		return nil, err
	}

	// 权限检查
	if workflow.TenantID != tenantID {
		return nil, fmt.Errorf("无权限修改此工作流")
	}

	// 验证工作流定义
	if req.Definition != nil {
		if err := s.validateWorkflowDefinition(*req.Definition); err != nil {
			return nil, fmt.Errorf("工作流定义无效: %w", err)
		}
	}

	// 更新字段
	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Definition != nil {
		updates["definition"] = *req.Definition
	}
	if req.Version != nil {
		updates["version"] = *req.Version
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	updates["updated_at"] = time.Now().UTC()

	// 执行更新
	if err := s.db.WithContext(ctx).
		Model(workflow).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新工作流失败: %w", err)
	}

	// 重新查询返回最新数据
	return s.GetWorkflow(ctx, tenantID, workflowID)
}

// DeleteWorkflow 软删除工作流
func (s *WorkflowService) DeleteWorkflow(ctx context.Context, tenantID, workflowID, operatorID string) error {
	// 查询工作流
	workflow, err := s.GetWorkflow(ctx, tenantID, workflowID)
	if err != nil {
		return err
	}

	// 权限检查
	if workflow.TenantID != tenantID {
		return fmt.Errorf("无权限删除此工作流")
	}

	// 软删除
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).
		Model(workflow).
		Updates(map[string]any{
			"deleted_at": now,
			"deleted_by": operatorID,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("删除工作流失败: %w", err)
	}

	return nil
}

// ValidateWorkflow 验证工作流定义
func (s *WorkflowService) ValidateWorkflow(ctx context.Context, definition WorkflowDefinition) error {
	return s.validateWorkflowDefinition(definition)
}

// validateWorkflowDefinition 验证工作流定义（内部方法）
func (s *WorkflowService) validateWorkflowDefinition(definition WorkflowDefinition) error {
	// 1. 基础检查
	if len(definition.Nodes) == 0 {
		return fmt.Errorf("工作流至少需要一个节点")
	}

	// 2. 检查是否有 Start 节点
	hasStart := false
	nodeIDs := make(map[string]bool)

	for _, node := range definition.Nodes {
		// ID 唯一性检查
		if nodeIDs[node.ID] {
			return fmt.Errorf("节点 ID 重复: %s", node.ID)
		}
		nodeIDs[node.ID] = true

		// 类型检查
		switch node.Type {
		case NodeTypeStart:
			if hasStart {
				return fmt.Errorf("只能有一个 Start 节点")
			}
			hasStart = true
		case NodeTypeAgent:
			if node.Data.AgentID == "" && node.Data.Label == "" {
				return fmt.Errorf("Agent 节点 %s 缺少配置", node.ID)
			}
		}
	}

	if !hasStart {
		return fmt.Errorf("缺少 Start 节点")
	}
	// End 节点不是强制的，有些工作流可能只是执行任务不返回特定结果？
	// 但为了规范，通常建议有。暂时不强制。

	// 3. 检查 Edge 连接有效性
	for _, edge := range definition.Edges {
		if !nodeIDs[edge.Source] {
			return fmt.Errorf("连线 %s 的源节点 %s 不存在", edge.ID, edge.Source)
		}
		if !nodeIDs[edge.Target] {
			return fmt.Errorf("连线 %s 的目标节点 %s 不存在", edge.ID, edge.Target)
		}
	}

	return nil
}

// GetWorkflowStats 获取工作流统计信息
func (s *WorkflowService) GetWorkflowStats(ctx context.Context, tenantID, workflowID string) (map[string]any, error) {
	// 验证工作流是否存在
	workflow, err := s.GetWorkflow(ctx, tenantID, workflowID)
	if err != nil {
		return nil, err
	}

	// 计算成功率
	var successRate float64
	if workflow.TotalExecutions > 0 {
		successRate = float64(workflow.SuccessExecutions) / float64(workflow.TotalExecutions) * 100
	}

	return map[string]any{
		"total_executions":   workflow.TotalExecutions,
		"success_executions": workflow.SuccessExecutions,
		"failed_executions":  workflow.FailedExecutions,
		"success_rate":       successRate,
	}, nil
}

// ExampleWorkflowDefinition 示例工作流定义
func ExampleWorkflowDefinition() WorkflowDefinition {
	return WorkflowDefinition{
		Nodes: []Node{
			{
				ID:       "node_start",
				Type:     NodeTypeStart,
				Position: Position{X: 100, Y: 100},
				Data:     NodeData{Label: "开始"},
			},
			{
				ID:       "node_writer",
				Type:     NodeTypeAgent,
				Position: Position{X: 100, Y: 200},
				Data: NodeData{
					Label:       "内容创作",
					AgentConfig: map[string]any{"role": "writer"},
					Inputs: map[string]string{
						"topic": "{{start.topic}}",
					},
				},
			},
			{
				ID:       "node_end",
				Type:     NodeTypeEnd,
				Position: Position{X: 100, Y: 400},
				Data:     NodeData{Label: "结束"},
			},
		},
		Edges: []Edge{
			{ID: "edge_1", Source: "node_start", Target: "node_writer"},
			{ID: "edge_2", Source: "node_writer", Target: "node_end"},
		},
	}
}

// CreateWorkflowFromJSON 从 JSON 字符串创建工作流
func (s *WorkflowService) CreateWorkflowFromJSON(ctx context.Context, tenantID, name, jsonStr, createdBy string) (*Workflow, error) {
	// 解析 JSON
	var definition WorkflowDefinition
	if err := json.Unmarshal([]byte(jsonStr), &definition); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	// 创建工作流
	return s.CreateWorkflow(ctx, &CreateWorkflowRequest{
		TenantID:   tenantID,
		Name:       name,
		Definition: definition,
		Version:    "v1.0",
		Visibility: "tenant",
		CreatedBy:  createdBy,
	})
}
