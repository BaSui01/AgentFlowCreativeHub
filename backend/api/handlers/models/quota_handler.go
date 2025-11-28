package models

import (
	"net/http"

	"backend/internal/common"
	"backend/internal/models"

	"github.com/gin-gonic/gin"
)

// QuotaHandler 模型配额管理 Handler
type QuotaHandler struct {
	service *models.ModelQuotaService
}

// NewQuotaHandler 创建 QuotaHandler 实例
func NewQuotaHandler(service *models.ModelQuotaService) *QuotaHandler {
	return &QuotaHandler{service: service}
}

// ListQuotas 查询配额列表
// @Summary 查询配额列表
// @Description 获取当前租户的模型配额列表
// @Tags Models
// @Security BearerAuth
// @Produce json
// @Param model_id query string false "模型ID过滤"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]string
// @Router /api/models/quotas [get]
func (h *QuotaHandler) ListQuotas(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	modelID := c.Query("model_id")

	quotas, err := h.service.ListQuotas(c.Request.Context(), tenantID, modelID)
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccess(c, gin.H{"quotas": quotas, "total": len(quotas)})
}

// GetQuota 获取配额详情
// @Summary 获取配额详情
// @Description 根据ID获取配额详细信息
// @Tags Models
// @Security BearerAuth
// @Produce json
// @Param id path string true "配额ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]string
// @Router /api/models/quotas/{id} [get]
func (h *QuotaHandler) GetQuota(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	quotaID := c.Param("id")

	quota, err := h.service.GetQuotaByID(c.Request.Context(), tenantID, quotaID)
	if err != nil {
		common.ResponseNotFound(c, "配额不存在")
		return
	}

	common.ResponseSuccess(c, quota)
}

// CreateQuota 创建配额
// @Summary 创建模型配额
// @Description 为指定模型创建使用配额限制
// @Tags Models
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateModelQuotaRequest true "配额信息"
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Router /api/models/quotas [post]
func (h *QuotaHandler) CreateQuota(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req CreateModelQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ResponseBadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	quota, err := h.service.CreateQuota(c.Request.Context(), tenantID, req.ModelID, &models.QuotaLimits{
		MaxTokensPerDay:   req.MaxTokensPerDay,
		MaxTokensPerMonth: req.MaxTokensPerMonth,
		MaxCostPerDay:     req.MaxCostPerDay,
		MaxCostPerMonth:   req.MaxCostPerMonth,
	})
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, common.SuccessResponse(quota))
}

// UpdateQuota 更新配额
// @Summary 更新模型配额
// @Description 更新指定配额的限制设置
// @Tags Models
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "配额ID"
// @Param request body UpdateModelQuotaRequest true "更新内容"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/models/quotas/{id} [put]
func (h *QuotaHandler) UpdateQuota(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	quotaID := c.Param("id")

	var req UpdateModelQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ResponseBadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	limits := &models.QuotaLimits{
		MaxTokensPerDay:   -1,
		MaxTokensPerMonth: -1,
		MaxCallsPerDay:    -1,
		MaxCallsPerMonth:  -1,
		MaxCostPerDay:     -1,
		MaxCostPerMonth:   -1,
	}
	if req.MaxTokensPerDay != nil {
		limits.MaxTokensPerDay = *req.MaxTokensPerDay
	}
	if req.MaxTokensPerMonth != nil {
		limits.MaxTokensPerMonth = *req.MaxTokensPerMonth
	}
	if req.MaxCostPerDay != nil {
		limits.MaxCostPerDay = *req.MaxCostPerDay
	}
	if req.MaxCostPerMonth != nil {
		limits.MaxCostPerMonth = *req.MaxCostPerMonth
	}

	quota, err := h.service.UpdateQuota(c.Request.Context(), tenantID, quotaID, limits)
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "配额更新成功", quota)
}

// DeleteQuota 删除配额
// @Summary 删除模型配额
// @Description 删除指定的模型配额设置
// @Tags Models
// @Security BearerAuth
// @Produce json
// @Param id path string true "配额ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/models/quotas/{id} [delete]
func (h *QuotaHandler) DeleteQuota(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	quotaID := c.Param("id")

	if err := h.service.DeleteQuota(c.Request.Context(), tenantID, quotaID); err != nil {
		common.ResponseNotFound(c, "配额不存在")
		return
	}

	common.ResponseSuccessMessage(c, "配额删除成功", nil)
}

// GetQuotaUsage 获取配额使用情况
// @Summary 获取配额使用情况
// @Description 获取指定配额的当前使用量统计
// @Tags Models
// @Security BearerAuth
// @Produce json
// @Param id path string true "配额ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]string
// @Router /api/models/quotas/{id}/usage [get]
func (h *QuotaHandler) GetQuotaUsage(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	quotaID := c.Param("id")

	quota, err := h.service.GetQuotaByID(c.Request.Context(), tenantID, quotaID)
	if err != nil {
		common.ResponseNotFound(c, "配额不存在")
		return
	}

	common.ResponseSuccess(c, quota)
}
