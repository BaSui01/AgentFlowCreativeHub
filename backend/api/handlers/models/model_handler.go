package models

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"backend/internal/common"
	"backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createCredentialRequest struct {
	Name         string         `json:"name" binding:"required"`
	Provider     string         `json:"provider"`
	APIKey       string         `json:"apiKey" binding:"required"`
	BaseURL      string         `json:"baseUrl"`
	ExtraHeaders map[string]any `json:"extraHeaders"`
	SetAsDefault bool           `json:"setAsDefault"`
}

// ModelHandler AI 模型管理 Handler
type ModelHandler struct {
	service            *models.ModelService
	discoveryService   *models.ModelDiscoveryService
	credentialService  *models.ModelCredentialService
}

// NewModelHandler 创建 ModelHandler 实例
func NewModelHandler(service *models.ModelService, discoveryService *models.ModelDiscoveryService, credentialService *models.ModelCredentialService) *ModelHandler {
	return &ModelHandler{
		service:           service,
		discoveryService:  discoveryService,
		credentialService: credentialService,
	}
}

// ListModels 查询模型列表
// GET /api/models?provider=openai&type=chat&status=active&page=1&page_size=20
func (h *ModelHandler) ListModels(c *gin.Context) {
	// 获取租户 ID（从中间件注入的上下文中获取）
	tenantID := c.GetString("tenant_id")

	// 解析查询参数
	req := &models.ListModelsRequest{
		TenantID: tenantID,
		Provider: c.Query("provider"),
		Type:     c.Query("type"),
		Status:   c.Query("status"),
	}

	// 解析分页参数
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

	// 调用 Service
	resp, err := h.service.ListModels(c.Request.Context(), req)
	if err != nil {
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	// 返回统一格式（兼容旧的响应结构）
	c.JSON(http.StatusOK, common.SuccessResponse(resp))
}

// GetModel 查询单个模型
// GET /api/models/:id
func (h *ModelHandler) GetModel(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	modelID := c.Param("id")

	model, err := h.service.GetModel(c.Request.Context(), tenantID, modelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ResponseNotFound(c, "模型不存在")
			return
		}
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccess(c, model)
}

// CreateModel 创建模型
// POST /api/models
func (h *ModelHandler) CreateModel(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req models.CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ResponseBadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	req.TenantID = tenantID

	model, err := h.service.CreateModel(c.Request.Context(), &req)
	if err != nil {
		// 判断是否为业务错误
		if bizErr, ok := err.(*common.BusinessError); ok {
			common.ResponseBusinessError(c, bizErr)
			return
		}
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "模型创建成功", model)
}

// UpdateModel 更新模型
// PUT /api/models/:id
func (h *ModelHandler) UpdateModel(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	modelID := c.Param("id")

	var req models.UpdateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ResponseBadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	model, err := h.service.UpdateModel(c.Request.Context(), tenantID, modelID, &req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ResponseNotFound(c, "模型不存在")
			return
		}
		// 判断是否为业务错误
		if bizErr, ok := err.(*common.BusinessError); ok {
			common.ResponseBusinessError(c, bizErr)
			return
		}
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "模型更新成功", model)
}

// DeleteModel 删除模型
// DELETE /api/models/:id
func (h *ModelHandler) DeleteModel(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	modelID := c.Param("id")
	operatorID := c.GetString("user_id") // 从上下文获取操作者 ID

	if err := h.service.DeleteModel(c.Request.Context(), tenantID, modelID, operatorID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ResponseNotFound(c, "模型不存在")
			return
		}
		common.ResponseError(c, common.CodeInternalError, err.Error())
		return
	}

	common.ResponseSuccessMessage(c, "模型删除成功", nil)
}

// GetModelStats 获取模型统计
// GET /api/models/:id/stats?start_time=2024-01-01&end_time=2024-12-31
func (h *ModelHandler) GetModelStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	modelID := c.Param("id")

	// 解析时间参数，默认最近30天
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)
	endTime := now

	if st := c.Query("start_time"); st != "" {
		if t, err := time.Parse("2006-01-02", st); err == nil {
			startTime = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		if t, err := time.Parse("2006-01-02", et); err == nil {
			endTime = t
		}
	}

	stats, err := h.service.GetModelCallStats(c.Request.Context(), tenantID, modelID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ListModelCredentials 列出模型绑定的凭证
func (h *ModelHandler) ListModelCredentials(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	modelID := c.Param("id")

	req := &models.ListCredentialsRequest{
		TenantID: tenantID,
		ModelID:  modelID,
	}
	creds, err := h.credentialService.ListCredentials(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, creds)
}

// CreateModelCredential 创建模型凭证
func (h *ModelHandler) CreateModelCredential(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	modelID := c.Param("id")
	_ = c.GetString("user_id") // TODO: 用于审计日志

	var body createCredentialRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	cred, err := h.credentialService.CreateCredential(c.Request.Context(), &models.CreateModelCredentialRequest{
		TenantID:     tenantID,
		ModelID:      modelID,
		Provider:     body.Provider,
		Name:         body.Name,
		APIKey:       body.APIKey,
		BaseURL:      body.BaseURL,
		ExtraHeaders: body.ExtraHeaders,
		SetAsDefault: body.SetAsDefault,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cred)
}

// DeleteModelCredential 删除模型凭证
func (h *ModelHandler) DeleteModelCredential(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	credentialID := c.Param("credentialId")

	if err := h.credentialService.DeleteCredential(c.Request.Context(), tenantID, credentialID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// SeedDefaultModels 初始化预置模型
// POST /api/models/seed
func (h *ModelHandler) SeedDefaultModels(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	if err := h.service.SeedDefaultModels(c.Request.Context(), tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "预置模型初始化成功",
	})
}

// DiscoverModels 从指定提供商同步模型
// POST /api/models/discover/:provider
func (h *ModelHandler) DiscoverModels(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	provider := c.Param("provider")

	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "提供商参数不能为空",
		})
		return
	}

	count, err := h.discoveryService.SyncModelsFromProvider(c.Request.Context(), tenantID, provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"provider": provider,
		"count":    count,
		"message":  "成功发现 " + strconv.Itoa(count) + " 个模型",
	})
}

// DiscoverAllModels 从所有提供商同步模型
// POST /api/models/discover-all
func (h *ModelHandler) DiscoverAllModels(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	results, err := h.discoveryService.AutoDiscoverModels(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 计算总数
	total := 0
	for _, count := range results {
		if count > 0 {
			total += count
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   total,
		"message": "成功同步 " + strconv.Itoa(total) + " 个模型",
	})
}
