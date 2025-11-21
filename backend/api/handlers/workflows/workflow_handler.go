package workflows

import (
	"net/http"
	"strconv"

	response "backend/api/handlers/common"
	"backend/internal/workflow"

	"github.com/gin-gonic/gin"
)

// WorkflowHandler 工作流管理 Handler
type WorkflowHandler struct {
	service *workflow.WorkflowService
}

// NewWorkflowHandler 创建 WorkflowHandler 实例
func NewWorkflowHandler(service *workflow.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{service: service}
}

// ListWorkflows 查询工作流列表
// @Summary 查询工作流列表
// @Description 根据可见性与创建者筛选当前租户的工作流
// @Tags Workflows
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param visibility query string false "可见性范围"
// @Param created_by query string false "创建者 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} workflow.ListWorkflowsResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/workflows [get]
func (h *WorkflowHandler) ListWorkflows(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	req := &workflow.ListWorkflowsRequest{
		TenantID:   tenantID,
		Visibility: c.Query("visibility"),
		CreatedBy:  c.Query("created_by"),
	}

	if page := c.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			req.Page = p
		}
	}
	if pageSize := c.Query("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil {
			req.PageSize = ps
		}
	}

	resp, err := h.service.ListWorkflows(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetWorkflow 查询单个工作流
// @Summary 查询工作流详情
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Param id path string true "工作流 ID"
// @Success 200 {object} workflow.Workflow
// @Failure 404 {object} response.ErrorResponse
// @Router /api/workflows/{id} [get]
func (h *WorkflowHandler) GetWorkflow(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	workflowID := c.Param("id")

	wf, err := h.service.GetWorkflow(c.Request.Context(), tenantID, workflowID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, wf)
}

// CreateWorkflow 创建工作流
// @Summary 创建工作流
// @Tags Workflows
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body workflow.CreateWorkflowRequest true "工作流创建参数"
// @Success 201 {object} workflow.Workflow
// @Failure 400 {object} response.ErrorResponse
// @Router /api/workflows [post]
func (h *WorkflowHandler) CreateWorkflow(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var req workflow.CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	req.TenantID = tenantID
	req.CreatedBy = userID

	wf, err := h.service.CreateWorkflow(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, wf)
}

// UpdateWorkflow 更新工作流
// @Summary 更新工作流
// @Tags Workflows
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "工作流 ID"
// @Param request body workflow.UpdateWorkflowRequest true "更新参数"
// @Success 200 {object} workflow.Workflow
// @Failure 400 {object} response.ErrorResponse
// @Router /api/workflows/{id} [put]
func (h *WorkflowHandler) UpdateWorkflow(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	workflowID := c.Param("id")

	var req workflow.UpdateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	wf, err := h.service.UpdateWorkflow(c.Request.Context(), tenantID, workflowID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, wf)
}

// DeleteWorkflow 删除工作流
// @Summary 删除工作流
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Param id path string true "工作流 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /api/workflows/{id} [delete]
func (h *WorkflowHandler) DeleteWorkflow(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	workflowID := c.Param("id")
	operatorID := c.GetString("user_id")

	if err := h.service.DeleteWorkflow(c.Request.Context(), tenantID, workflowID, operatorID); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "工作流删除成功"})
}

// ValidateWorkflow 验证工作流定义
// @Summary 验证工作流定义
// @Tags Workflows
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body workflowValidateRequest true "定义内容"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /api/workflows/validate [post]
func (h *WorkflowHandler) ValidateWorkflow(c *gin.Context) {
	var req workflowValidateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	definition, err := mapToWorkflowDefinition(req.Definition)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	if err := h.service.ValidateWorkflow(c.Request.Context(), definition); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "工作流定义有效"})
}

// GetWorkflowStats 获取工作流统计
// @Summary 获取工作流统计
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Param id path string true "工作流 ID"
// @Success 200 {object} map[string]any
// @Failure 500 {object} response.ErrorResponse
// @Router /api/workflows/{id}/stats [get]
func (h *WorkflowHandler) GetWorkflowStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	workflowID := c.Param("id")

	stats, err := h.service.GetWorkflowStats(c.Request.Context(), tenantID, workflowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetExampleWorkflow 获取示例工作流
// @Summary 获取示例工作流定义
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/workflows/example [get]
func (h *WorkflowHandler) GetExampleWorkflow(c *gin.Context) {
	example := workflow.ExampleWorkflowDefinition()

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"example": example}})
}

// workflowValidateRequest 工作流校验请求体。
type workflowValidateRequest struct {
	Definition map[string]any `json:"definition" binding:"required"`
}
