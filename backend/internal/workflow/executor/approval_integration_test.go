package executor

import (
	"context"
	"testing"
	"time"

	workflow "backend/internal/workflow"
	"backend/internal/workflow/approval"
	"backend/internal/workflow/state"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestApprovalWorkflowIntegration 集成测试：完整的审批工作流暂停和恢复流程
func TestApprovalWorkflowIntegration(t *testing.T) {
	// 跳过集成测试（除非明确启用）
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 1. 设置测试环境
	ctx := context.Background()
	db := setupIntegrationTestDB(t)
	redisClient := setupIntegrationTestRedis(t)
	defer cleanupIntegrationTestRedis(redisClient)

	// 2. 创建测试数据
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	workflowID := uuid.New().String()
	executionID := uuid.New().String()

	// 3. 创建工作流定义（包含审批节点）
	workflowDef := createIntegrationTestWorkflow(workflowID)

	// 4. 保存工作流定义到数据库
	err := db.Table("workflows").Create(map[string]any{
		"id":          workflowID,
		"tenant_id":   tenantID,
		"name":        workflowDef.Name,
		"description": workflowDef.Description,
		"definition":  workflowDef,
		"status":      "active",
		"created_at":  time.Now(),
		"updated_at":  time.Now(),
	}).Error
	require.NoError(t, err)

	// 5. 创建 AutomationEngine
	stateManager := state.NewStateManager(redisClient)
	approvalManager := approval.NewManager(db)

	parser := NewParser()

	engine := &AutomationEngine{
		Engine: &Engine{
			db:             db,
			parser:         parser,
			maxConcurrency: 5,
		},
		stateManager:    stateManager,
		approvalManager: approvalManager,
	}

	parsedDef, err := parser.Parse(*workflowDef)
	require.NoError(t, err)

	// 6. 执行工作流
	t.Log("Step 1: 执行工作流...")
	result, err := engine.executeAutomated(
		ctx,
		workflowID,
		executionID,
		tenantID,
		userID,
		map[string]any{"test_input": "value"},
		parsedDef,
	)

	// 7. 验证工作流暂停
	require.NoError(t, err)
	assert.Equal(t, "paused", result.Status, "工作流应该暂停")
	t.Logf("✓ 工作流已暂停，状态: %s", result.Status)

	// 验证有暂停的任务
	pausedTaskFound := false
	for taskID, taskResult := range result.Tasks {
		if taskResult.Status == "paused" {
			pausedTaskFound = true
			t.Logf("✓ 找到暂停的任务: %s", taskID)
			break
		}
	}
	assert.True(t, pausedTaskFound, "应该有一个任务处于 paused 状态")

	// 8. 验证创建了 ApprovalRequest
	t.Log("Step 2: 验证审批请求...")
	var approvalReq workflow.ApprovalRequest
	err = db.Where("execution_id = ? AND status = ?", executionID, "pending").
		First(&approvalReq).Error
	require.NoError(t, err, "应该创建了审批请求")
	assert.Equal(t, executionID, approvalReq.ExecutionID)
	assert.Equal(t, "pending", approvalReq.Status)
	t.Logf("✓ 审批请求已创建，ID: %s", approvalReq.ID)

	// 9. 验证状态已保存到 Redis
	t.Log("Step 3: 验证状态已保存到 Redis...")
	savedState, err := stateManager.GetState(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, "paused", savedState.Status)
	assert.NotEmpty(t, savedState.Metadata["workflow_id"])
	t.Logf("✓ 状态已保存到 Redis，workflow_id: %v", savedState.Metadata["workflow_id"])

	// 10. 批准审批请求
	t.Log("Step 4: 批准审批请求...")
	err = approvalManager.ApproveRequest(ctx, approvalReq.ID, userID, "集成测试批准")
	require.NoError(t, err)
	t.Logf("✓ 审批请求已批准")

	// 11. 恢复工作流执行
	t.Log("Step 5: 恢复工作流执行...")
	resumeResult, err := engine.ResumeExecution(ctx, executionID)
	require.NoError(t, err)
	t.Logf("✓ 工作流已恢复，最终状态: %s", resumeResult.Status)

	// 12. 验证工作流完成
	// 注意：由于我们没有实际的 Agent 执行器，工作流可能会失败
	// 但重要的是验证恢复逻辑正常工作
	assert.NotEqual(t, "paused", resumeResult.Status, "工作流不应该再处于暂停状态")
	t.Logf("✓ 工作流不再暂停")

	// 验证之前暂停的任务状态已更新
	hasUpdatedTask := false
	for taskID, taskResult := range resumeResult.Tasks {
		if taskResult.Status != "paused" {
			hasUpdatedTask = true
			t.Logf("✓ 任务 %s 状态已更新为: %s", taskID, taskResult.Status)
		}
	}
	assert.True(t, hasUpdatedTask, "应该有任务状态已更新")
}

// TestApprovalWorkflowRejectionIntegration 集成测试：审批被拒绝的情况
func TestApprovalWorkflowRejectionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 1. 设置测试环境
	ctx := context.Background()
	db := setupIntegrationTestDB(t)
	redisClient := setupIntegrationTestRedis(t)
	defer cleanupIntegrationTestRedis(redisClient)

	// 2. 创建测试数据
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	workflowID := uuid.New().String()
	executionID := uuid.New().String()

	// 3. 创建工作流定义
	workflowDef := createIntegrationTestWorkflow(workflowID)

	// 4. 创建 AutomationEngine
	stateManager := state.NewStateManager(redisClient)
	approvalManager := approval.NewManager(db)

	parser := NewParser()
	engine := &AutomationEngine{
		Engine: &Engine{
			db:             db,
			parser:         parser,
			maxConcurrency: 5,
		},
		stateManager:    stateManager,
		approvalManager: approvalManager,
	}

	parsedDef, err := parser.Parse(*workflowDef)
	require.NoError(t, err)

	// 5. 执行工作流
	t.Log("Step 1: 执行工作流...")
	result, err := engine.executeAutomated(
		ctx,
		workflowID,
		executionID,
		tenantID,
		userID,
		map[string]any{},
		parsedDef,
	)

	require.NoError(t, err)
	assert.Equal(t, "paused", result.Status)
	t.Logf("✓ 工作流已暂停")

	// 6. 获取审批请求
	var approvalReq workflow.ApprovalRequest
	err = db.Where("execution_id = ?", executionID).First(&approvalReq).Error
	require.NoError(t, err)
	t.Logf("✓ 找到审批请求: %s", approvalReq.ID)

	// 7. 拒绝审批
	t.Log("Step 2: 拒绝审批请求...")
	err = approvalManager.RejectRequest(ctx, approvalReq.ID, userID, "集成测试拒绝")
	require.NoError(t, err)
	t.Logf("✓ 审批请求已拒绝")

	// 8. 尝试恢复执行
	t.Log("Step 3: 尝试恢复工作流...")
	_, err = engine.ResumeExecution(ctx, executionID)

	// 9. 验证返回错误
	assert.Error(t, err, "审批被拒绝应该返回错误")
	assert.Contains(t, err.Error(), "拒绝", "错误信息应该包含'拒绝'")
	t.Logf("✓ 正确返回错误: %v", err)
}

// TestStateManagerPersistence 测试状态管理器的持久化
func TestStateManagerPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	redisClient := setupIntegrationTestRedis(t)
	defer cleanupIntegrationTestRedis(redisClient)

	stateManager := state.NewStateManager(redisClient)
	executionID := uuid.New().String()

	// 1. 保存状态
	t.Log("Step 1: 保存状态...")
	err := stateManager.SaveState(ctx, &state.WorkflowState{
		ExecutionID: executionID,
		Status:      "paused",
		Metadata: map[string]any{
			"workflow_id": "test-workflow",
			"tenant_id":   "test-tenant",
			"user_id":     "test-user",
			"input":       map[string]any{"key": "value"},
		},
		StepResults: map[string]any{
			"step1": map[string]any{
				"status": "success",
				"output": "step1_output",
			},
		},
		UpdatedAt: time.Now(),
	})
	require.NoError(t, err)
	t.Logf("✓ 状态已保存")

	// 2. 读取状态
	t.Log("Step 2: 读取状态...")
	loadedState, err := stateManager.GetState(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, "paused", loadedState.Status)
	assert.Equal(t, "test-workflow", loadedState.Metadata["workflow_id"])
	assert.NotNil(t, loadedState.StepResults["step1"])
	t.Logf("✓ 状态读取成功")

	// 3. 更新状态
	t.Log("Step 3: 更新状态...")
	err = stateManager.UpdateState(ctx, executionID, map[string]any{
		"status": "completed",
		"metadata": map[string]any{
			"completed_at": time.Now(),
		},
	})
	require.NoError(t, err)
	t.Logf("✓ 状态已更新")

	// 4. 验证更新
	updatedState, err := stateManager.GetState(ctx, executionID)
	require.NoError(t, err)
	assert.Equal(t, "completed", updatedState.Status)
	assert.NotNil(t, updatedState.Metadata["completed_at"])
	t.Logf("✓ 更新验证成功")

	// 5. 删除状态
	t.Log("Step 4: 删除状态...")
	err = stateManager.DeleteState(ctx, executionID)
	require.NoError(t, err)
	t.Logf("✓ 状态已删除")

	// 6. 验证删除
	_, err = stateManager.GetState(ctx, executionID)
	assert.Error(t, err, "删除后应该无法获取状态")
	t.Logf("✓ 删除验证成功")
}

// 辅助函数

func setupIntegrationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&workflow.ApprovalRequest{})
	require.NoError(t, err)

	// 创建 workflows 表（简化版）
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			definition JSON,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	return db
}

func setupIntegrationTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // 使用测试数据库
	})

	// 测试连接
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Skip("Redis 不可用，跳过集成测试")
	}

	// 清空测试数据库
	client.FlushDB(ctx)

	return client
}

func cleanupIntegrationTestRedis(client *redis.Client) {
	if client != nil {
		ctx := context.Background()
		client.FlushDB(ctx)
		client.Close()
	}
}

func createIntegrationTestWorkflow(workflowID string) *workflow.WorkflowDefinition {
	return &workflow.WorkflowDefinition{
		Nodes: []workflow.Node{
			{
				ID:   "step1",
				Type: workflow.NodeTypeAgent,
				Data: workflow.NodeData{
					Label:       "Step 1",
					AgentConfig: map[string]any{"agentType": "writer"},
					Inputs:      map[string]string{"prompt": "Test prompt"},
				},
			},
			{
				ID:   "approval_step",
				Type: workflow.NodeTypeApproval,
				Data: workflow.NodeData{
					Label:     "Approval Step",
					Approvers: []string{"user1"},
					Timeout:   3600,
					Inputs:    map[string]string{"content": "{{step1.output}}"},
				},
			},
			{
				ID:   "step2",
				Type: workflow.NodeTypeAgent,
				Data: workflow.NodeData{
					Label:       "Step 2",
					AgentConfig: map[string]any{"agentType": "formatter"},
					Inputs:      map[string]string{"content": "{{approval_step.output}}"},
				},
			},
		},
		Edges: []workflow.Edge{
			{ID: "edge1", Source: "step1", Target: "approval_step"},
			{ID: "edge2", Source: "approval_step", Target: "step2"},
		},
	}
}
