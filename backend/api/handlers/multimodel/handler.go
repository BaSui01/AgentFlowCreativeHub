package multimodel

import (
	"net/http"

	"backend/api/handlers/common"
	"backend/internal/multimodel"

	"github.com/gin-gonic/gin"
)

// Handler 多模型处理器
type Handler struct {
	service *multimodel.Service
}

// NewHandler 创建处理器
func NewHandler(service *multimodel.Service) *Handler {
	return &Handler{service: service}
}

// Draw 执行多模型抽卡
// @Summary 多模型抽卡
// @Description 同时调用多个AI模型生成内容并对比结果
// @Tags MultiModel
// @Accept json
// @Produce json
// @Param request body multimodel.DrawRequest true "抽卡请求"
// @Success 200 {object} common.Response{data=multimodel.DrawResponse}
// @Router /api/multimodel/draw [post]
func (h *Handler) Draw(c *gin.Context) {
	var req multimodel.DrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	result, err := h.service.Draw(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "抽卡失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// GetDrawHistory 获取抽卡历史详情
// @Summary 获取抽卡历史详情
// @Description 根据ID获取抽卡历史详情
// @Tags MultiModel
// @Produce json
// @Param id path string true "抽卡ID"
// @Success 200 {object} common.Response{data=multimodel.DrawHistory}
// @Router /api/multimodel/draws/{id} [get]
func (h *Handler) GetDrawHistory(c *gin.Context) {
	drawID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	result, err := h.service.GetDrawHistory(c.Request.Context(), tenantID, drawID)
	if err != nil {
		c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "抽卡历史不存在: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// ListDrawHistory 查询抽卡历史列表
// @Summary 查询抽卡历史列表
// @Description 查询用户的抽卡历史记录
// @Tags MultiModel
// @Produce json
// @Param agent_type query string false "Agent类型"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} common.PagedResponse{data=[]multimodel.DrawHistory}
// @Router /api/multimodel/draws [get]
func (h *Handler) ListDrawHistory(c *gin.Context) {
	var req multimodel.ListDrawHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")

	histories, total, err := h.service.ListDrawHistory(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	totalPage := (int(total) + req.PageSize - 1) / req.PageSize
	c.JSON(http.StatusOK, common.ListResponse{
		Items: histories,
		Pagination: common.PaginationMeta{
			Page:      req.Page,
			PageSize:  req.PageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// Regenerate 重新生成单个模型结果
// @Summary 重新生成结果
// @Description 重新调用指定模型生成结果
// @Tags MultiModel
// @Accept json
// @Produce json
// @Param request body multimodel.RegenerateRequest true "重新生成请求"
// @Success 200 {object} common.Response{data=multimodel.DrawResult}
// @Router /api/multimodel/regenerate [post]
func (h *Handler) Regenerate(c *gin.Context) {
	var req multimodel.RegenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	result, err := h.service.Regenerate(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "重新生成失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// DeleteDrawHistory 删除抽卡历史
// @Summary 删除抽卡历史
// @Description 删除指定的抽卡历史记录
// @Tags MultiModel
// @Produce json
// @Param id path string true "抽卡ID"
// @Success 200 {object} common.Response
// @Router /api/multimodel/draws/{id} [delete]
func (h *Handler) DeleteDrawHistory(c *gin.Context) {
	drawID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	if err := h.service.DeleteDrawHistory(c.Request.Context(), tenantID, drawID); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// GetStats 获取抽卡统计
// @Summary 获取抽卡统计
// @Description 获取用户的抽卡统计信息
// @Tags MultiModel
// @Produce json
// @Success 200 {object} common.Response{data=map[string]any}
// @Router /api/multimodel/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	stats, err := h.service.GetDrawStats(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "获取统计失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: stats})
}
