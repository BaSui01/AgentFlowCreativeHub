package workflows

import (
	"context"
	"net/http"

	workflow "backend/internal/workflow"
	"backend/internal/workflow/approval"
	"backend/internal/workflow/executor"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// AutomationHandler 自动化工作流处理器
type AutomationHandler struct {
	db               *gorm.DB
	redisClient      redis.UniversalClient
	automationEngine *executor.AutomationEngine
	approvalManager  *approval.Manager
}

// NewAutomationHandler 创建自动化工作流处理器

func NewAutomationHandler(
	db *gorm.DB,
	redisClient redis.UniversalClient,
	automationEngine *executor.AutomationEngine,
	approvalMgr *approval.Manager,
) *AutomationHandler {
	if approvalMgr == nil {
		approvalMgr = approval.NewManager(db)
	}
	return &AutomationHandler{
		db:               db,
		redisClient:      redisClient,
		automationEngine: automationEngine,
		approvalManager:  approvalMgr,
	}
}

// ExecuteAutomated 执行自动化工作流
// POST /api/v1/workflows/:id/execute/automated
func (h *AutomationHandler) ExecuteAutomated(c *gin.Context) {
	workflowID := c.Param("id")
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "workflow_id 不能为空",
		})
		return
	}

	// 解析请求
	var req struct {
		Input map[string]any `json:"input"`
		Mode  string         `json:"mode"` // full_auto, semi_auto, manual
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 执行自动化工作流
	result, err := h.automationEngine.ExecuteWithAutomation(
		c.Request.Context(),
		workflowID,
		tenantID,
		userID,
		req.Input,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"execution_id": result.ExecutionID,
		"status":       result.Status,
		"output":       result.Output,
		"duration_ms":  result.Duration.Milliseconds(),
		"tasks":        result.Tasks,
	})
}

// GetApprovalRequests 获取待审批请求列表
// GET /api/v1/workflows/approvals/pending
func (h *AutomationHandler) GetApprovalRequests(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	executionID := c.Query("execution_id")

	// 构建查询
	query := h.db.WithContext(c.Request.Context()).
		Model(&workflow.ApprovalRequest{}).
		Where("tenant_id = ? AND status = ?", tenantID, "pending")

	if executionID != "" {
		query = query.Where("execution_id = ?", executionID)
	}

	var approvals []*workflow.ApprovalRequest
	if err := query.Order("created_at ASC").Find(&approvals).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询审批请求失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"approvals": approvals,
		"total":     len(approvals),
	})
}

// ApproveRequest 批准审批请求
// POST /api/v1/workflows/approvals/:id/approve
func (h *AutomationHandler) ApproveRequest(c *gin.Context) {
	approvalID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		Comment string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 批准请求
	err := h.approvalManager.ApproveRequest(
		c.Request.Context(),
		approvalID,
		userID,
		req.Comment,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 查询审批请求以获取 execution_id
	var approvalReq workflow.ApprovalRequest
	err = h.db.WithContext(c.Request.Context()).
		Where("id = ?", approvalID).
		First(&approvalReq).Error

	if err == nil && approvalReq.ExecutionID != "" {
		// 异步恢复工作流执行
		go func() {
			ctx := context.Background()
			_, _ = h.automationEngine.ResumeExecution(ctx, approvalReq.ExecutionID)
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "审批请求已批准",
	})
}

// RejectRequest 拒绝审批请求
// POST /api/v1/workflows/approvals/:id/reject
func (h *AutomationHandler) RejectRequest(c *gin.Context) {
	approvalID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		Comment string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 拒绝请求
	err := h.approvalManager.RejectRequest(
		c.Request.Context(),
		approvalID,
		userID,
		req.Comment,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "审批请求已拒绝",
	})
}

// ResendNotification 手动重发审批通知
func (h *AutomationHandler) ResendNotification(c *gin.Context) {
	approvalID := c.Param("id")
	tenantID := c.GetString("tenant_id")
	if approvalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的审批 ID"})
		return
	}

	if err := h.approvalManager.ResendNotification(c.Request.Context(), tenantID, approvalID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "通知已重发"})
}

// GetExecutionState 获取工作流执行状态
// GET /api/v1/workflows/executions/:id/state
func (h *AutomationHandler) GetExecutionState(c *gin.Context) {
	executionID := c.Param("id")

	// 查询执行记录
	var execution workflow.WorkflowExecution
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ?", executionID).
		First(&execution).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "工作流执行不存在",
		})
		return
	}

	// 获取待审批请求
	approvals, _ := h.approvalManager.GetPendingApprovals(c.Request.Context(), executionID)

	// 查询任务列表
	var tasks []workflow.WorkflowTask
	_ = h.db.WithContext(c.Request.Context()).
		Where("execution_id = ?", executionID).
		Order("created_at ASC").
		Find(&tasks).Error

	c.JSON(http.StatusOK, gin.H{
		"execution":         execution,
		"pending_approvals": approvals,
		"tasks":             tasks,
	})
}

// GetAutomationLogs 获取自动化执行日志
// GET /api/v1/workflows/executions/:id/automation-logs
func (h *AutomationHandler) GetAutomationLogs(c *gin.Context) {
	executionID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	var logs []workflow.AutomationLog
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND execution_id = ?", tenantID, executionID).
		Order("created_at ASC").
		Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询自动化日志失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": len(logs),
	})
}
