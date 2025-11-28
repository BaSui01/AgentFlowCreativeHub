package fragment

import (
	"net/http"

	"backend/api/handlers/common"
	"backend/internal/fragment"

	"github.com/gin-gonic/gin"
)

// Handler 片段管理处理器
type Handler struct {
	service *fragment.Service
}

// NewHandler 创建处理器
func NewHandler(service *fragment.Service) *Handler {
	return &Handler{service: service}
}

// CreateFragment 创建片段
// @Summary 创建片段
// @Description 创建灵感片段、素材、待办事项等
// @Tags Fragment
// @Accept json
// @Produce json
// @Param request body fragment.CreateFragmentRequest true "创建请求"
// @Success 200 {object} common.Response{data=fragment.Fragment}
// @Router /api/fragments [post]
func (h *Handler) CreateFragment(c *gin.Context) {
	var req fragment.CreateFragmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	result, err := h.service.CreateFragment(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "创建片段失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// GetFragment 获取片段详情
// @Summary 获取片段详情
// @Description 根据ID获取片段详情
// @Tags Fragment
// @Produce json
// @Param id path string true "片段ID"
// @Success 200 {object} common.Response{data=fragment.Fragment}
// @Router /api/fragments/{id} [get]
func (h *Handler) GetFragment(c *gin.Context) {
	fragmentID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	result, err := h.service.GetFragment(c.Request.Context(), tenantID, fragmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "片段不存在: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// UpdateFragment 更新片段
// @Summary 更新片段
// @Description 更新片段信息
// @Tags Fragment
// @Accept json
// @Produce json
// @Param id path string true "片段ID"
// @Param request body fragment.UpdateFragmentRequest true "更新请求"
// @Success 200 {object} common.Response{data=fragment.Fragment}
// @Router /api/fragments/{id} [put]
func (h *Handler) UpdateFragment(c *gin.Context) {
	fragmentID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	var req fragment.UpdateFragmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	result, err := h.service.UpdateFragment(c.Request.Context(), tenantID, fragmentID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "更新片段失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// DeleteFragment 删除片段
// @Summary 删除片段
// @Description 删除片段（软删除）
// @Tags Fragment
// @Produce json
// @Param id path string true "片段ID"
// @Success 200 {object} common.Response
// @Router /api/fragments/{id} [delete]
func (h *Handler) DeleteFragment(c *gin.Context) {
	fragmentID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	if err := h.service.DeleteFragment(c.Request.Context(), tenantID, fragmentID); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "删除片段失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// ListFragments 查询片段列表
// @Summary 查询片段列表
// @Description 根据条件查询片段列表（支持分页、过滤、排序）
// @Tags Fragment
// @Produce json
// @Param type query string false "片段类型" Enums(inspiration, material, todo, note, reference)
// @Param status query string false "片段状态" Enums(pending, completed, archived)
// @Param workspace_id query string false "工作空间ID"
// @Param work_id query string false "作品ID"
// @Param chapter_id query string false "章节ID"
// @Param tags query string false "标签（逗号分隔）"
// @Param keyword query string false "关键词搜索"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param sort_by query string false "排序字段" Enums(created_at, priority, due_date)
// @Param sort_order query string false "排序方向" Enums(asc, desc)
// @Success 200 {object} common.PagedResponse{data=[]fragment.Fragment}
// @Router /api/fragments [get]
func (h *Handler) ListFragments(c *gin.Context) {
	var req fragment.ListFragmentsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")

	fragments, total, err := h.service.ListFragments(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "查询片段列表失败: " + err.Error()})
		return
	}

	totalPage := (int(total) + req.PageSize - 1) / req.PageSize
	c.JSON(http.StatusOK, common.ListResponse{
		Items: fragments,
		Pagination: common.PaginationMeta{
			Page:      req.Page,
			PageSize:  req.PageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// CompleteFragment 完成片段
// @Summary 完成片段
// @Description 标记片段为已完成（主要用于待办事项）
// @Tags Fragment
// @Produce json
// @Param id path string true "片段ID"
// @Success 200 {object} common.Response
// @Router /api/fragments/{id}/complete [post]
func (h *Handler) CompleteFragment(c *gin.Context) {
	fragmentID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	if err := h.service.CompleteFragment(c.Request.Context(), tenantID, fragmentID); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "完成片段失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// BatchOperation 批量操作
// @Summary 批量操作
// @Description 批量完成、归档、删除或更改状态
// @Tags Fragment
// @Accept json
// @Produce json
// @Param request body fragment.BatchOperationRequest true "批量操作请求"
// @Success 200 {object} common.Response
// @Router /api/fragments/batch [post]
func (h *Handler) BatchOperation(c *gin.Context) {
	var req fragment.BatchOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")

	if err := h.service.BatchOperation(c.Request.Context(), tenantID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "批量操作失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: nil})
}

// GetStats 获取统计信息
// @Summary 获取片段统计
// @Description 获取片段统计信息（按类型、状态分组）
// @Tags Fragment
// @Produce json
// @Success 200 {object} common.Response{data=fragment.FragmentStatsResponse}
// @Router /api/fragments/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	stats, err := h.service.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "获取统计信息失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: stats})
}
