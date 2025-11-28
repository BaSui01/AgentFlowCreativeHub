package worldbuilder

import (
	"net/http"
	"strconv"

	response "backend/api/handlers/common"
	"backend/internal/worldbuilder"

	"github.com/gin-gonic/gin"
)

// Handler 世界观构建 API 处理器
type Handler struct {
	service *worldbuilder.Service
}

// NewHandler 创建处理器
func NewHandler(service *worldbuilder.Service) *Handler {
	return &Handler{service: service}
}

// ============================================================================
// 世界观设定 CRUD
// ============================================================================

// CreateSetting 创建世界观
// @Summary 创建世界观设定
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body worldbuilder.CreateSettingRequest true "设定信息"
// @Success 200 {object} response.APIResponse{data=worldbuilder.WorldSetting}
// @Router /api/worldbuilder/settings [post]
func (h *Handler) CreateSetting(c *gin.Context) {
	var req worldbuilder.CreateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	setting, err := h.service.CreateSetting(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, setting)
}

// ListSettings 获取设定列表
// @Summary 获取世界观设定列表
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param workId query string false "作品ID"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} response.ListResponse
// @Router /api/worldbuilder/settings [get]
func (h *Handler) ListSettings(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	workID := c.Query("workId")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	settings, total, err := h.service.ListSettings(c.Request.Context(), tenantID, workID, page, pageSize)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPage := (int(total) + pageSize - 1) / pageSize
	c.JSON(http.StatusOK, response.ListResponse{
		Items: settings,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// GetSetting 获取设定详情
// @Summary 获取世界观设定详情
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param id path string true "设定ID"
// @Success 200 {object} response.APIResponse{data=worldbuilder.WorldSetting}
// @Router /api/worldbuilder/settings/{id} [get]
func (h *Handler) GetSetting(c *gin.Context) {
	id := c.Param("id")

	setting, err := h.service.GetSetting(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "设定不存在")
		return
	}

	response.Success(c, setting)
}

// UpdateSetting 更新设定
// @Summary 更新世界观设定
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "设定ID"
// @Param request body object true "更新内容"
// @Success 200 {object} response.APIResponse
// @Router /api/worldbuilder/settings/{id} [put]
func (h *Handler) UpdateSetting(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateSetting(c.Request.Context(), id, userID, updates); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// DeleteSetting 删除设定
// @Summary 删除世界观设定
// @Tags WorldBuilder
// @Security BearerAuth
// @Param id path string true "设定ID"
// @Success 200 {object} response.APIResponse
// @Router /api/worldbuilder/settings/{id} [delete]
func (h *Handler) DeleteSetting(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteSetting(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============================================================================
// AI 生成
// ============================================================================

// GenerateSetting AI 生成设定
// @Summary AI生成世界观设定
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body worldbuilder.GenerateSettingRequest true "生成请求"
// @Success 200 {object} response.APIResponse{data=worldbuilder.WorldSetting}
// @Router /api/worldbuilder/generate [post]
func (h *Handler) GenerateSetting(c *gin.Context) {
	var req worldbuilder.GenerateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	setting, err := h.service.GenerateSetting(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, setting)
}

// ModifySetting 增量修改设定
// @Summary AI增量修改设定
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body worldbuilder.ModifySettingRequest true "修改请求"
// @Success 200 {object} response.APIResponse{data=worldbuilder.WorldSetting}
// @Router /api/worldbuilder/modify [post]
func (h *Handler) ModifySetting(c *gin.Context) {
	var req worldbuilder.ModifySettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	setting, err := h.service.ModifySetting(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, setting)
}

// GenerateCharacter AI 生成角色
// @Summary AI生成角色
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "生成请求"
// @Success 200 {object} response.APIResponse{data=worldbuilder.SettingEntity}
// @Router /api/worldbuilder/generate/character [post]
func (h *Handler) GenerateCharacter(c *gin.Context) {
	var req struct {
		SettingID   string `json:"settingId" binding:"required"`
		Instruction string `json:"instruction" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	entity, err := h.service.GenerateCharacter(c.Request.Context(), tenantID, userID, req.SettingID, req.Instruction)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, entity)
}

// GenerateRelations AI 生成关系
// @Summary AI生成关系网络
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "生成请求"
// @Success 200 {object} response.APIResponse{data=worldbuilder.RelationGraph}
// @Router /api/worldbuilder/generate/relations [post]
func (h *Handler) GenerateRelations(c *gin.Context) {
	var req struct {
		SettingID string `json:"settingId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	graph, err := h.service.GenerateRelations(c.Request.Context(), tenantID, userID, req.SettingID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, graph)
}

// ============================================================================
// 实体管理
// ============================================================================

// CreateEntity 创建实体
// @Summary 创建设定实体
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body worldbuilder.CreateEntityRequest true "实体信息"
// @Success 200 {object} response.APIResponse{data=worldbuilder.SettingEntity}
// @Router /api/worldbuilder/entities [post]
func (h *Handler) CreateEntity(c *gin.Context) {
	var req worldbuilder.CreateEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	entity, err := h.service.CreateEntity(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, entity)
}

// ListEntities 获取实体列表
// @Summary 获取设定实体列表
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param settingId query string true "设定ID"
// @Param type query string false "实体类型"
// @Param keyword query string false "关键词"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} response.ListResponse
// @Router /api/worldbuilder/entities [get]
func (h *Handler) ListEntities(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

	query := &worldbuilder.EntityQuery{
		SettingID: c.Query("settingId"),
		Type:      c.Query("type"),
		Category:  c.Query("category"),
		Keyword:   c.Query("keyword"),
		ParentID:  c.Query("parentId"),
		Page:      page,
		PageSize:  pageSize,
	}

	entities, total, err := h.service.ListEntities(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPage := (int(total) + pageSize - 1) / pageSize
	c.JSON(http.StatusOK, response.ListResponse{
		Items: entities,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// GetEntity 获取实体详情
// @Summary 获取设定实体详情
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param id path string true "实体ID"
// @Success 200 {object} response.APIResponse{data=worldbuilder.SettingEntity}
// @Router /api/worldbuilder/entities/{id} [get]
func (h *Handler) GetEntity(c *gin.Context) {
	id := c.Param("id")

	entity, err := h.service.GetEntity(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "实体不存在")
		return
	}

	response.Success(c, entity)
}

// UpdateEntity 更新实体
// @Summary 更新设定实体
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "实体ID"
// @Param request body object true "更新内容"
// @Success 200 {object} response.APIResponse
// @Router /api/worldbuilder/entities/{id} [put]
func (h *Handler) UpdateEntity(c *gin.Context) {
	id := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateEntity(c.Request.Context(), id, updates); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// DeleteEntity 删除实体
// @Summary 删除设定实体
// @Tags WorldBuilder
// @Security BearerAuth
// @Param id path string true "实体ID"
// @Success 200 {object} response.APIResponse
// @Router /api/worldbuilder/entities/{id} [delete]
func (h *Handler) DeleteEntity(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteEntity(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============================================================================
// 关系管理
// ============================================================================

// CreateRelation 创建关系
// @Summary 创建实体关系
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body worldbuilder.CreateRelationRequest true "关系信息"
// @Success 200 {object} response.APIResponse{data=worldbuilder.EntityRelation}
// @Router /api/worldbuilder/relations [post]
func (h *Handler) CreateRelation(c *gin.Context) {
	var req worldbuilder.CreateRelationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")

	relation, err := h.service.CreateRelation(c.Request.Context(), tenantID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, relation)
}

// GetRelationGraph 获取关系图
// @Summary 获取关系图数据
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param settingId query string true "设定ID"
// @Success 200 {object} response.APIResponse{data=worldbuilder.RelationGraph}
// @Router /api/worldbuilder/relations/graph [get]
func (h *Handler) GetRelationGraph(c *gin.Context) {
	settingID := c.Query("settingId")
	if settingID == "" {
		response.Error(c, http.StatusBadRequest, "settingId 必填")
		return
	}

	graph, err := h.service.GetRelationGraph(c.Request.Context(), settingID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, graph)
}

// DeleteRelation 删除关系
// @Summary 删除实体关系
// @Tags WorldBuilder
// @Security BearerAuth
// @Param id path string true "关系ID"
// @Success 200 {object} response.APIResponse
// @Router /api/worldbuilder/relations/{id} [delete]
func (h *Handler) DeleteRelation(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteRelation(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============================================================================
// 版本管理
// ============================================================================

// GetVersionHistory 获取版本历史
// @Summary 获取设定版本历史
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param settingId query string true "设定ID"
// @Success 200 {object} response.APIResponse{data=[]worldbuilder.SettingVersion}
// @Router /api/worldbuilder/versions [get]
func (h *Handler) GetVersionHistory(c *gin.Context) {
	settingID := c.Query("settingId")
	if settingID == "" {
		response.Error(c, http.StatusBadRequest, "settingId 必填")
		return
	}

	versions, err := h.service.GetVersionHistory(c.Request.Context(), settingID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, versions)
}

// GetVersion 获取指定版本
// @Summary 获取指定版本详情
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param id path string true "版本ID"
// @Success 200 {object} response.APIResponse{data=worldbuilder.SettingVersion}
// @Router /api/worldbuilder/versions/{id} [get]
func (h *Handler) GetVersion(c *gin.Context) {
	id := c.Param("id")

	version, err := h.service.GetVersion(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "版本不存在")
		return
	}

	response.Success(c, version)
}

// RevertToVersion 恢复到指定版本
// @Summary 恢复到指定版本
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body object true "恢复请求"
// @Success 200 {object} response.APIResponse{data=worldbuilder.WorldSetting}
// @Router /api/worldbuilder/versions/revert [post]
func (h *Handler) RevertToVersion(c *gin.Context) {
	var req struct {
		SettingID string `json:"settingId" binding:"required"`
		VersionID string `json:"versionId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.GetString("user_id")

	setting, err := h.service.RevertToVersion(c.Request.Context(), req.SettingID, req.VersionID, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, setting)
}

// DiffVersions 对比版本
// @Summary 对比两个版本
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param versionA query string true "版本A ID"
// @Param versionB query string true "版本B ID"
// @Success 200 {object} response.APIResponse{data=worldbuilder.VersionDiff}
// @Router /api/worldbuilder/versions/diff [get]
func (h *Handler) DiffVersions(c *gin.Context) {
	versionA := c.Query("versionA")
	versionB := c.Query("versionB")

	if versionA == "" || versionB == "" {
		response.Error(c, http.StatusBadRequest, "versionA 和 versionB 必填")
		return
	}

	diff, err := h.service.DiffVersions(c.Request.Context(), versionA, versionB)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, diff)
}

// ============================================================================
// 模板管理
// ============================================================================

// ListTemplates 获取模板列表
// @Summary 获取设定模板列表
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param genre query string false "类型"
// @Success 200 {object} response.APIResponse{data=[]worldbuilder.SettingTemplate}
// @Router /api/worldbuilder/templates [get]
func (h *Handler) ListTemplates(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	genre := c.Query("genre")

	templates, err := h.service.ListTemplates(c.Request.Context(), tenantID, genre)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, templates)
}

// CreateTemplate 创建模板
// @Summary 创建设定模板
// @Tags WorldBuilder
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body worldbuilder.SettingTemplate true "模板信息"
// @Success 200 {object} response.APIResponse
// @Router /api/worldbuilder/templates [post]
func (h *Handler) CreateTemplate(c *gin.Context) {
	var template worldbuilder.SettingTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := c.GetString("tenant_id")

	if err := h.service.CreateTemplate(c.Request.Context(), tenantID, &template); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, template)
}

// ============================================================================
// 统计
// ============================================================================

// GetStats 获取设定统计
// @Summary 获取设定统计
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param settingId query string true "设定ID"
// @Success 200 {object} response.APIResponse{data=worldbuilder.SettingStats}
// @Router /api/worldbuilder/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	settingID := c.Query("settingId")
	if settingID == "" {
		response.Error(c, http.StatusBadRequest, "settingId 必填")
		return
	}

	stats, err := h.service.GetStats(c.Request.Context(), settingID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, stats)
}


// ============================================================================
// 侧边栏快速查阅 API
// ============================================================================

// GetWorkSettingsSummary 获取作品设定摘要（用于侧边栏）
// @Summary 获取作品设定摘要
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param workId query string true "作品ID"
// @Success 200 {object} response.APIResponse{data=worldbuilder.WorkSettingsSummary}
// @Router /api/worldbuilder/sidebar/summary [get]
func (h *Handler) GetWorkSettingsSummary(c *gin.Context) {
	workID := c.Query("workId")
	if workID == "" {
		response.Error(c, http.StatusBadRequest, "workId 参数不能为空")
		return
	}

	summary, err := h.service.GetWorkSettingsSummary(c.Request.Context(), workID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, summary)
}

// SearchEntitiesInWork 搜索作品中的实体
// @Summary 搜索作品中的设定实体
// @Tags WorldBuilder
// @Security BearerAuth
// @Produce json
// @Param workId query string true "作品ID"
// @Param keyword query string false "关键词"
// @Param type query string false "实体类型"
// @Success 200 {object} response.APIResponse{data=[]worldbuilder.SettingEntity}
// @Router /api/worldbuilder/sidebar/search [get]
func (h *Handler) SearchEntitiesInWork(c *gin.Context) {
	workID := c.Query("workId")
	if workID == "" {
		response.Error(c, http.StatusBadRequest, "workId 参数不能为空")
		return
	}

	keyword := c.Query("keyword")
	typeStr := c.Query("type")

	var entityType *string
	if typeStr != "" {
		entityType = &typeStr
	}

	entities, err := h.service.SearchEntitiesInWork(c.Request.Context(), workID, keyword, entityType)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, entities)
}
