package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"backend/internal/workflow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL = "http://localhost:8080/api/v1"
	// 预置的租户和用户信息（需要在初始化脚本中插入或在此处创建）
	testTenantID = "00000000-0000-0000-0000-000000000001"
	testUserID   = "00000000-0000-0000-0000-000000000002"
)

// TestE2EWorkflowExecution 端到端工作流执行测试
// 前提：必须先启动 docker-compose 并运行后端服务
func TestE2EWorkflowExecution(t *testing.T) {
	if testing.Short() || os.Getenv("RUN_E2E_TESTS") != "1" {
		t.Skip("Skipping E2E test; set RUN_E2E_TESTS=1 to enable")
	}

	// 1. 创建工作流
	wfDef := workflow.WorkflowDefinition{
		Nodes: []workflow.Node{
			{
				ID:   "start",
				Type: workflow.NodeTypeStart,
				Data: workflow.NodeData{Label: "Start"},
			},
			{
				ID:   "search_step",
				Type: workflow.NodeTypeAgent, // 或 Tool 节点
				Data: workflow.NodeData{
					Label: "Web Search",
					AgentConfig: map[string]any{
						"agentType": "researcher",
					},
					Inputs: map[string]string{
						"query": "AgentFlow creative hub features",
					},
				},
			},
			{
				ID:   "end",
				Type: workflow.NodeTypeEnd,
				Data: workflow.NodeData{Label: "End"},
			},
		},
		Edges: []workflow.Edge{
			{ID: "e1", Source: "start", Target: "search_step"},
			{ID: "e2", Source: "search_step", Target: "end"},
		},
	}

	createReq := map[string]any{
		"name":        "E2E Test Workflow",
		"description": "Created by integration test",
		"definition":  wfDef,
		"tenantId":    testTenantID,
		"createdBy":   testUserID,
	}

	createBody, _ := json.Marshal(createReq)
	resp, err := http.Post(baseURL+"/workflows", "application/json", bytes.NewReader(createBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var wfResp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&wfResp)
	workflowID := wfResp.ID
	t.Logf("Created workflow: %s", workflowID)

	// 2. 执行工作流
	execReq := map[string]any{
		"input": map[string]any{
			"start": map[string]any{"query": "golang"},
		},
	}
	execBody, _ := json.Marshal(execReq)
	resp, err = http.Post(baseURL+"/workflows/"+workflowID+"/execute", "application/json", bytes.NewReader(execBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var execResp struct {
		ExecutionID string `json:"executionId"`
	}
	json.NewDecoder(resp.Body).Decode(&execResp)
	executionID := execResp.ExecutionID
	t.Logf("Started execution: %s", executionID)

	// 3. 轮询状态
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		time.Sleep(1 * time.Second)

		resp, err = http.Get(baseURL + "/executions/" + executionID)
		require.NoError(t, err)

		var statusResp struct {
			Status string         `json:"status"`
			Output map[string]any `json:"output"`
		}
		json.NewDecoder(resp.Body).Decode(&statusResp)

		t.Logf("Execution status: %s", statusResp.Status)

		if statusResp.Status == "completed" {
			assert.NotEmpty(t, statusResp.Output)
			break
		}
		if statusResp.Status == "failed" {
			t.Fatalf("Execution failed")
		}
	}
}
