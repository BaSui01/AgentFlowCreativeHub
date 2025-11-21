package workflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupWorkflowServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:workflow_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开 sqlite 失败: %v", err)
	}
	if err := db.AutoMigrate(&Workflow{}); err != nil {
		t.Fatalf("迁移 schema 失败: %v", err)
	}
	return db
}

func sampleWorkflowDefinition() WorkflowDefinition {
	return WorkflowDefinition{
		Nodes: []Node{
			{ID: "start", Type: NodeTypeStart, Data: NodeData{Label: "开始"}},
			{ID: "writer", Type: NodeTypeAgent, Data: NodeData{Label: "写作", AgentID: "agent_writer"}},
		},
		Edges: []Edge{
			{ID: "edge1", Source: "start", Target: "writer"},
		},
	}
}

func TestWorkflowServiceCreateListAndStats(t *testing.T) {
	ctx := context.Background()
	db := setupWorkflowServiceTestDB(t)
	svc := NewWorkflowService(db)

	created, err := svc.CreateWorkflow(ctx, &CreateWorkflowRequest{
		TenantID:   "tenant-A",
		Name:       "内容生成",
		Definition: sampleWorkflowDefinition(),
		CreatedBy:  "user-1",
	})
	if err != nil {
		t.Fatalf("创建工作流失败: %v", err)
	}

	resp, err := svc.ListWorkflows(ctx, &ListWorkflowsRequest{TenantID: "tenant-A", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("查询列表失败: %v", err)
	}
	if resp.Total != 1 || len(resp.Workflows) != 1 {
		t.Fatalf("列表结果不正确: %+v", resp)
	}

	updates := map[string]any{
		"execution_count": 5,
		"success_count":   3,
		"failure_count":   2,
	}
	if err := db.Model(&Workflow{}).Where("id = ?", created.ID).Updates(updates).Error; err != nil {
		t.Fatalf("更新统计失败: %v", err)
	}
	stats, err := svc.GetWorkflowStats(ctx, "tenant-A", created.ID)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}
	if stats["total_executions"].(int) != 5 {
		t.Fatalf("执行总数不匹配: %+v", stats)
	}
	if fmt.Sprintf("%.0f", stats["success_rate"].(float64)) != "60" {
		t.Fatalf("成功率应为60%%, 实际 %v", stats["success_rate"])
	}
}

func TestWorkflowServiceDeleteWorkflow(t *testing.T) {
	ctx := context.Background()
	db := setupWorkflowServiceTestDB(t)
	svc := NewWorkflowService(db)
	wf, err := svc.CreateWorkflow(ctx, &CreateWorkflowRequest{
		TenantID:   "tenant-del",
		Name:       "待删除",
		Definition: sampleWorkflowDefinition(),
		CreatedBy:  "deleter",
	})
	if err != nil {
		t.Fatalf("创建工作流失败: %v", err)
	}

	if err := svc.DeleteWorkflow(ctx, "tenant-del", wf.ID, "deleter"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	if _, err := svc.GetWorkflow(ctx, "tenant-del", wf.ID); err == nil {
		t.Fatalf("删除后的工作流不应可见")
	}
}

func TestWorkflowServiceValidateDefinitionErrors(t *testing.T) {
	ctx := context.Background()
	db := setupWorkflowServiceTestDB(t)
	svc := NewWorkflowService(db)

	invalidDef := WorkflowDefinition{
		Nodes: []Node{{ID: "agent_only", Type: NodeTypeAgent, Data: NodeData{Label: "缺少开始", AgentID: "agent"}}},
	}

	_, err := svc.CreateWorkflow(ctx, &CreateWorkflowRequest{
		TenantID:   "tenant-bad",
		Name:       "坏定义",
		Definition: invalidDef,
		CreatedBy:  "tester",
	})
	if err == nil {
		t.Fatalf("不合法的定义应当失败")
	}

	if err := svc.ValidateWorkflow(ctx, invalidDef); err == nil {
		t.Fatalf("缺少开始节点的定义应返回错误")
	}
}
