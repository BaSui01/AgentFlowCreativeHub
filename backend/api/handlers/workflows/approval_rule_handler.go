package workflows

import (
	"net/http"
	"time"

	"backend/api/handlers/common"
	"backend/internal/workflow/approval"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApprovalRuleHandler 审批规则管理 Handler
type ApprovalRuleHandler struct {
	db *gorm.DB
}

// NewApprovalRuleHandler 创建 ApprovalRuleHandler 实例
func NewApprovalRuleHandler(db *gorm.DB) *ApprovalRuleHandler {
	return &ApprovalRuleHandler{db: db}
}

// ListApprovalRules 查询审批规则列表
// @Summary 查询审批规则列表
// @Description 获取当前租户的审批规则列表
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Param workflow_id query string false "工作流ID过滤"
// @Success 200 {object} common.APIResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/workflows/approval-rules [get]
func (h *ApprovalRuleHandler) ListApprovalRules(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	workflowID := c.Query("workflow_id")

	query := h.db.WithContext(c.Request.Context()).
		Model(&approval.ApprovalRule{}).
		Where("tenant_id = ?", tenantID)

	if workflowID != "" {
		query = query.Where("workflow_id = ?", workflowID)
	}

	var rules []*approval.ApprovalRule
	if err := query.Order("priority DESC, created_at DESC").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "查询审批规则失败"})
		return
	}

	common.Success(c, gin.H{"rules": rules, "total": len(rules)})
}

// GetApprovalRule 获取审批规则详情
// @Summary 获取审批规则详情
// @Description 根据ID获取审批规则详细信息
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Param id path string true "规则ID"
// @Success 200 {object} common.APIResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/workflows/approval-rules/{id} [get]
func (h *ApprovalRuleHandler) GetApprovalRule(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	ruleID := c.Param("id")

	var rule approval.ApprovalRule
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "审批规则不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	common.Success(c, rule)
}

// CreateApprovalRule 创建审批规则
// @Summary 创建审批规则
// @Description 创建新的审批规则
// @Tags Workflows
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateApprovalRuleRequest true "规则信息"
// @Success 201 {object} common.APIResponse
// @Failure 400 {object} common.ErrorResponse
// @Router /api/workflows/approval-rules [post]
func (h *ApprovalRuleHandler) CreateApprovalRule(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req CreateApprovalRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	rule := &approval.ApprovalRule{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		WorkflowID:  req.WorkflowID,
		Name:        req.Name,
		Description: req.Description,
		Priority:    req.Priority,
		IsActive:    req.IsActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := h.db.WithContext(c.Request.Context()).Create(rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "创建审批规则失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, common.APIResponse{Success: true, Message: "审批规则创建成功", Data: rule})
}

// UpdateApprovalRule 更新审批规则
// @Summary 更新审批规则
// @Description 更新指定审批规则的配置
// @Tags Workflows
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "规则ID"
// @Param request body UpdateApprovalRuleRequest true "更新内容"
// @Success 200 {object} common.APIResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/workflows/approval-rules/{id} [put]
func (h *ApprovalRuleHandler) UpdateApprovalRule(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	ruleID := c.Param("id")

	var rule approval.ApprovalRule
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "审批规则不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	var req UpdateApprovalRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{
		"updated_at": time.Now().UTC(),
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Conditions != nil {
		updates["conditions"] = req.Conditions
	}
	if req.Actions != nil {
		updates["action"] = req.Actions
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := h.db.WithContext(c.Request.Context()).Model(&rule).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "更新审批规则失败: " + err.Error()})
		return
	}

	// 重新查询
	h.db.First(&rule, "id = ?", ruleID)

	common.Success(c, gin.H{"message": "审批规则更新成功", "rule": rule})
}

// DeleteApprovalRule 删除审批规则
// @Summary 删除审批规则
// @Description 删除指定的审批规则
// @Tags Workflows
// @Security BearerAuth
// @Produce json
// @Param id path string true "规则ID"
// @Success 200 {object} common.APIResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/workflows/approval-rules/{id} [delete]
func (h *ApprovalRuleHandler) DeleteApprovalRule(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	ruleID := c.Param("id")

	result := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Delete(&approval.ApprovalRule{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "删除审批规则失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "审批规则不存在"})
		return
	}

	common.Success(c, gin.H{"message": "审批规则删除成功"})
}
