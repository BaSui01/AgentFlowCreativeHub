package plot

import (
	"net/http"

	"backend/api/handlers/common"
	"backend/internal/plot"

	"github.com/gin-gonic/gin"
)

// Handler 剧情推演处理器
type Handler struct {
	service *plot.Service
}

// NewHandler 创建处理器
func NewHandler(service *plot.Service) *Handler {
	return &Handler{service: service}
}

// CreatePlotRecommendation 创建剧情推演
// @Summary 创建剧情推演
// @Description 基于当前剧情生成多个后续剧情分支
// @Tags Plot
// @Accept json
// @Produce json
// @Param request body plot.CreatePlotRequest true "推演请求"
// @Success 200 {object} common.Response{data=plot.PlotRecommendationResponse}
// @Router /api/plot/recommendations [post]
func (h *Handler) CreatePlotRecommendation(c *gin.Context) {
	var req plot.CreatePlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	result, err := h.service.CreatePlotRecommendation(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "生成剧情分支失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// GetPlotRecommendation 获取剧情推演详情
// @Summary 获取剧情推演详情
// @Description 根据ID获取剧情推演详情和分支
// @Tags Plot
// @Produce json
// @Param id path string true "推演ID"
// @Success 200 {object} common.Response{data=plot.PlotRecommendationResponse}
// @Router /api/plot/recommendations/{id} [get]
func (h *Handler) GetPlotRecommendation(c *gin.Context) {
	plotID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	result, err := h.service.GetPlotRecommendation(c.Request.Context(), tenantID, plotID)
	if err != nil {
		c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "剧情推演不存在: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// UpdatePlotRecommendation 更新剧情推演
// @Summary 更新剧情推演
// @Description 更新剧情推演信息（如选择分支）
// @Tags Plot
// @Accept json
// @Produce json
// @Param id path string true "推演ID"
// @Param request body plot.UpdatePlotRequest true "更新请求"
// @Success 200 {object} common.Response{data=plot.PlotRecommendationResponse}
// @Router /api/plot/recommendations/{id} [put]
func (h *Handler) UpdatePlotRecommendation(c *gin.Context) {
	plotID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	var req plot.UpdatePlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	result, err := h.service.UpdatePlotRecommendation(c.Request.Context(), tenantID, plotID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// DeletePlotRecommendation 删除剧情推演
// @Summary 删除剧情推演
// @Description 删除剧情推演记录
// @Tags Plot
// @Produce json
// @Param id path string true "推演ID"
// @Success 200 {object} common.Response
// @Router /api/plot/recommendations/{id} [delete]
func (h *Handler) DeletePlotRecommendation(c *gin.Context) {
	plotID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	if err := h.service.DeletePlotRecommendation(c.Request.Context(), tenantID, plotID); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// ListPlotRecommendations 查询剧情推演列表
// @Summary 查询剧情推演列表
// @Description 查询剧情推演历史记录
// @Tags Plot
// @Produce json
// @Param workspace_id query string false "工作空间ID"
// @Param work_id query string false "作品ID"
// @Param chapter_id query string false "章节ID"
// @Param applied query bool false "是否已应用"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} common.PagedResponse{data=[]plot.PlotRecommendationResponse}
// @Router /api/plot/recommendations [get]
func (h *Handler) ListPlotRecommendations(c *gin.Context) {
	var req plot.ListPlotRecommendationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")

	plots, total, err := h.service.ListPlotRecommendations(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	totalPage := (int(total) + req.PageSize - 1) / req.PageSize
	c.JSON(http.StatusOK, common.ListResponse{
		Items: plots,
		Pagination: common.PaginationMeta{
			Page:      req.Page,
			PageSize:  req.PageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// ApplyPlotToChapter 应用剧情到章节
// @Summary 应用剧情到章节
// @Description 将选定的剧情分支应用到指定章节
// @Tags Plot
// @Accept json
// @Produce json
// @Param request body plot.ApplyPlotRequest true "应用请求"
// @Success 200 {object} common.Response
// @Router /api/plot/apply [post]
func (h *Handler) ApplyPlotToChapter(c *gin.Context) {
	var req plot.ApplyPlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")

	if err := h.service.ApplyPlotToChapter(c.Request.Context(), tenantID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "应用剧情失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// GetStats 获取剧情推演统计
// @Summary 获取剧情推演统计
// @Description 获取剧情推演的统计信息
// @Tags Plot
// @Produce json
// @Success 200 {object} common.Response{data=map[string]any}
// @Router /api/plot/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	stats, err := h.service.GetPlotStats(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "获取统计失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: stats})
}
