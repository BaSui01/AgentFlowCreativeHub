package workflows

import (
	"net/http"
	"strconv"
	"time"

	response "backend/api/handlers/common"
	"backend/internal/workflow"
	"backend/internal/workflow/executor"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// WorkflowExecuteHandler 工作流执行 Handler
type WorkflowExecuteHandler struct {
	engine *executor.Engine
	db     *gorm.DB
}

// NewWorkflowExecuteHandler 创建 WorkflowExecuteHandler 实例
func NewWorkflowExecuteHandler(engine *executor.Engine, db *gorm.DB) *WorkflowExecuteHandler {
	return &WorkflowExecuteHandler{
		engine: engine,
		db:     db,
	}
}

// ExecuteWorkflowRequest 执行工作流请求
type ExecuteWorkflowRequest struct {
	Input map[string]any `json:"input" binding:"required"`
}

// WorkflowExecutionDetailResponse 执行详情响应。
type WorkflowExecutionDetailResponse struct {
	Execution workflow.WorkflowExecution `json:"execution"`
	Tasks     []workflow.WorkflowTask    `json:"tasks"`
}

// WorkflowExecutionListResponse 执行列表响应。
type WorkflowExecutionListResponse struct {
	Executions []workflow.WorkflowExecution `json:"executions"`
	Pagination response.PaginationMeta      `json:"pagination"`
}

// WorkflowExecutionResponse 工作流执行响应。
type WorkflowExecutionResponse struct {
	ExecutionID string                 `json:"execution_id"`
	WorkflowID  string                 `json:"workflow_id"`
	Status      string                 `json:"status"`
	Input       map[string]any         `json:"input,omitempty"`
	Output      map[string]any         `json:"output,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	DurationMs  int64                  `json:"duration_ms"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

// ExecuteWorkflow 执行工作流
// @Summary 提交工作流执行
// @Tags Workflows
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "工作流 ID"
// @Param request body ExecuteWorkflowRequest true "执行输入"
// @Success 202 {object} WorkflowExecutionResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/workflows/{id}/execute [post]
func (h *WorkflowExecuteHandler) ExecuteWorkflow(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	workflowID := c.Param("id")

	var req ExecuteWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	// 执行工作流
	result, err := h.engine.Execute(c.Request.Context(), workflowID, tenantID, userID, req.Input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	resp := WorkflowExecutionResponse{
		ExecutionID: result.ExecutionID,
		WorkflowID:  workflowID,
		Status:      result.Status,
		Input:       result.Input,
		Output:      result.Output,
		StartedAt:   result.StartedAt,
		CompletedAt: result.CompletedAt,
		DurationMs:  result.Duration.Milliseconds(),
	}

	c.JSON(http.StatusAccepted, resp)
}

// GetExecution 查询执行详情
// @Summary 查询执行详情
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Param id path string true "执行 ID"
// @Success 200 {object} WorkflowExecutionDetailResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/executions/{id} [get]
func (h *WorkflowExecuteHandler) GetExecution(c *gin.Context) {
	executionID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// 查询执行记录
	var execution workflow.WorkflowExecution
	if err := h.db.Where("id = ? AND tenant_id = ?", executionID, tenantID).
		First(&execution).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "执行记录不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	// 查询关联的任务
	var tasks []workflow.WorkflowTask
	h.db.Where("execution_id = ?", executionID).
		Order("created_at ASC").
		Find(&tasks)

	c.JSON(http.StatusOK, WorkflowExecutionDetailResponse{
		Execution: execution,
		Tasks:     tasks,
	})
}

// ListExecutions 查询执行列表
// @Summary 查询工作流执行历史
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Param id path string true "工作流 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param status query string false "执行状态"
// @Success 200 {object} WorkflowExecutionListResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/workflows/{id}/executions [get]
func (h *WorkflowExecuteHandler) ListExecutions(c *gin.Context) {
	workflowID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status") // 可选过滤

	// 查询执行列表
	var executions []workflow.WorkflowExecution
	var total int64

	query := h.db.Where("workflow_id = ? AND tenant_id = ?", workflowID, tenantID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Model(&workflow.WorkflowExecution{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	if err := query.Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&executions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, WorkflowExecutionListResponse{
		Executions: executions,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}
