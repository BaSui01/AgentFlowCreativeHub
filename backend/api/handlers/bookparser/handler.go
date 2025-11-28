package bookparser

import (
	"net/http"
	"strconv"

	response "backend/api/handlers/common"
	"backend/internal/auth"
	"backend/internal/bookparser"

	"github.com/gin-gonic/gin"
)

// Handler 拆书系统 API 处理器
type Handler struct {
	service *bookparser.Service
}

// NewHandler 创建处理器
func NewHandler(service *bookparser.Service) *Handler {
	return &Handler{service: service}
}

// CreateTask 创建拆书任务
// @Summary 创建拆书任务
// @Tags BookParser
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body bookparser.CreateTaskRequest true "任务请求"
// @Success 200 {object} response.APIResponse{data=bookparser.BookParserTask}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/bookparser/tasks [post]
func (h *Handler) CreateTask(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	var req bookparser.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	req.TenantID = userCtx.TenantID
	req.UserID = userCtx.UserID

	task, err := h.service.CreateTask(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "创建任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: task})
}

// GetTask 获取任务详情
// @Summary 获取拆书任务详情
// @Tags BookParser
// @Security BearerAuth
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} response.APIResponse{data=bookparser.BookParserTask}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/bookparser/tasks/{id} [get]
func (h *Handler) GetTask(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	taskID := c.Param("id")
	task, err := h.service.GetTask(c.Request.Context(), userCtx.TenantID, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: task})
}

// ListTasks 列出任务
// @Summary 列出拆书任务
// @Tags BookParser
// @Security BearerAuth
// @Produce json
// @Param status query string false "任务状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.ListResponse{items=[]bookparser.BookParserTask}
// @Failure 401 {object} response.ErrorResponse
// @Router /api/bookparser/tasks [get]
func (h *Handler) ListTasks(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	req := &bookparser.TaskListRequest{
		TenantID: userCtx.TenantID,
		UserID:   userCtx.UserID,
		Status:   bookparser.TaskStatus(c.Query("status")),
		Page:     page,
		PageSize: pageSize,
	}

	tasks, total, err := h.service.ListTasks(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询失败: " + err.Error()})
		return
	}

	totalPage := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPage++
	}

	c.JSON(http.StatusOK, response.ListResponse{
		Items: tasks,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: totalPage,
		},
	})
}

// GetTaskProgress 获取任务进度
// @Summary 获取拆书任务进度
// @Tags BookParser
// @Security BearerAuth
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} response.APIResponse{data=bookparser.TaskProgress}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/bookparser/tasks/{id}/progress [get]
func (h *Handler) GetTaskProgress(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	taskID := c.Param("id")
	progress, err := h.service.GetTaskProgress(c.Request.Context(), userCtx.TenantID, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: progress})
}

// GetTaskResults 获取任务分析结果
// @Summary 获取拆书分析结果
// @Tags BookParser
// @Security BearerAuth
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} response.APIResponse{data=bookparser.AnalysisResult}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/bookparser/tasks/{id}/results [get]
func (h *Handler) GetTaskResults(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	taskID := c.Param("id")
	results, err := h.service.GetTaskResults(c.Request.Context(), userCtx.TenantID, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "获取结果失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: results})
}

// CancelTask 取消任务
// @Summary 取消拆书任务
// @Tags BookParser
// @Security BearerAuth
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/bookparser/tasks/{id}/cancel [post]
func (h *Handler) CancelTask(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	taskID := c.Param("id")
	if err := h.service.CancelTask(c.Request.Context(), userCtx.TenantID, taskID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "取消失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "任务已取消"})
}

// SearchKnowledge 搜索知识库
// @Summary 搜索拆书知识库
// @Tags BookParser
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body bookparser.SearchKnowledgeRequest true "搜索请求"
// @Success 200 {object} response.APIResponse{data=[]bookparser.BookKnowledge}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/bookparser/knowledge/search [post]
func (h *Handler) SearchKnowledge(c *gin.Context) {
	userCtx, exists := auth.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Success: false, Message: "未认证"})
		return
	}

	var req bookparser.SearchKnowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	req.TenantID = userCtx.TenantID

	knowledge, err := h.service.SearchKnowledge(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: knowledge})
}

// GetDimensions 获取支持的分析维度
// @Summary 获取支持的分析维度列表
// @Tags BookParser
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/bookparser/dimensions [get]
func (h *Handler) GetDimensions(c *gin.Context) {
	dimensions := []map[string]string{
		{"id": "style", "name": "文风叙事", "description": "分析叙事方式、文风特点、用词习惯"},
		{"id": "plot", "name": "情节设计", "description": "分析核心冲突、悬念设计、故事节奏"},
		{"id": "character", "name": "人物塑造", "description": "分析角色塑造技巧与性格刻画"},
		{"id": "emotion", "name": "读者情绪", "description": "分析共鸣点、爽点布局、嗨点设计"},
		{"id": "meme", "name": "热梗搞笑", "description": "提取流行梗、搞笑点、网络文化元素"},
		{"id": "outline", "name": "章节大纲", "description": "提取章节结构与情节发展脉络"},
		{"id": "all", "name": "全部维度", "description": "分析所有维度"},
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: dimensions})
}
