package marketplace

import (
	"net/http"
	"strconv"

	response "backend/api/handlers/common"

	"github.com/gin-gonic/gin"
)

// Handler 工具市场 HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Publish 发布工具
// @Summary 发布工具到市场
// @Tags Marketplace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body PublishRequest true "发布请求"
// @Success 201 {object} response.APIResponse
// @Router /api/marketplace/publish [post]
func (h *Handler) Publish(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	userName := c.GetString("user_name")

	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	pkg, version, err := h.svc.Publish(c.Request.Context(), tenantID, userID, userName, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrPackageExists {
			status = http.StatusConflict
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response.APIResponse{
		Success: true,
		Data: gin.H{
			"package": pkg,
			"version": version,
		},
	})
}

// UpdatePackage 更新工具包
// @Summary 更新工具包信息
// @Tags Marketplace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "工具包ID"
// @Param request body UpdatePackageRequest true "更新请求"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/packages/{id} [put]
func (h *Handler) UpdatePackage(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	packageID := c.Param("id")

	var req UpdatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	pkg, err := h.svc.UpdatePackage(c.Request.Context(), tenantID, userID, packageID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrPackageNotFound {
			status = http.StatusNotFound
		} else if err == ErrPermissionDenied {
			status = http.StatusForbidden
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: pkg})
}

// DeletePackage 删除工具包
// @Summary 删除工具包
// @Tags Marketplace
// @Security BearerAuth
// @Param id path string true "工具包ID"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/packages/{id} [delete]
func (h *Handler) DeletePackage(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	packageID := c.Param("id")

	err := h.svc.DeletePackage(c.Request.Context(), tenantID, userID, packageID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrPackageNotFound {
			status = http.StatusNotFound
		} else if err == ErrPermissionDenied {
			status = http.StatusForbidden
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "删除成功"})
}

// PublishVersion 发布新版本
// @Summary 发布工具新版本
// @Tags Marketplace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "工具包ID"
// @Param request body PublishVersionRequest true "版本请求"
// @Success 201 {object} response.APIResponse
// @Router /api/marketplace/packages/{id}/versions [post]
func (h *Handler) PublishVersion(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	packageID := c.Param("id")

	var req PublishVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	version, err := h.svc.PublishVersion(c.Request.Context(), tenantID, userID, packageID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrPackageNotFound {
			status = http.StatusNotFound
		} else if err == ErrVersionExists {
			status = http.StatusConflict
		} else if err == ErrPermissionDenied {
			status = http.StatusForbidden
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response.APIResponse{Success: true, Data: version})
}

// ListVersions 获取版本列表
// @Summary 获取工具版本列表
// @Tags Marketplace
// @Produce json
// @Param id path string true "工具包ID"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/packages/{id}/versions [get]
func (h *Handler) ListVersions(c *gin.Context) {
	packageID := c.Param("id")

	versions, err := h.svc.GetVersions(c.Request.Context(), packageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"versions": versions,
			"count":    len(versions),
		},
	})
}

// GetVersion 获取版本详情
// @Summary 获取版本详情
// @Tags Marketplace
// @Produce json
// @Param id path string true "工具包ID"
// @Param version path string true "版本号"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/packages/{id}/versions/{version} [get]
func (h *Handler) GetVersion(c *gin.Context) {
	packageID := c.Param("id")
	version := c.Param("version")

	v, err := h.svc.GetVersion(c.Request.Context(), packageID, version)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrVersionNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: v})
}

// Search 搜索工具
// @Summary 搜索工具市场
// @Tags Marketplace
// @Accept json
// @Produce json
// @Param request body SearchRequest true "搜索请求"
// @Success 200 {object} PackageListResponse
// @Router /api/marketplace/search [post]
func (h *Handler) Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	result, err := h.svc.Search(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: result})
}

// ListPackages 工具列表
// @Summary 获取工具列表
// @Tags Marketplace
// @Produce json
// @Param category query string false "分类"
// @Param sort_by query string false "排序字段"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} PackageListResponse
// @Router /api/marketplace/packages [get]
func (h *Handler) ListPackages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	req := &SearchRequest{
		Category:  c.Query("category"),
		SortBy:    c.DefaultQuery("sort_by", "downloads"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
		Page:      page,
		PageSize:  pageSize,
	}

	result, err := h.svc.Search(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: result})
}

// GetPackage 获取工具详情
// @Summary 获取工具详情
// @Tags Marketplace
// @Security BearerAuth
// @Produce json
// @Param id path string true "工具包ID"
// @Success 200 {object} PackageDetailResponse
// @Router /api/marketplace/packages/{id} [get]
func (h *Handler) GetPackage(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	packageID := c.Param("id")

	result, err := h.svc.GetPackage(c.Request.Context(), tenantID, userID, packageID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrPackageNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: result})
}

// Rate 评分
// @Summary 评价工具
// @Tags Marketplace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "工具包ID"
// @Param request body RatingRequest true "评分请求"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/packages/{id}/rate [post]
func (h *Handler) Rate(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	packageID := c.Param("id")

	var req RatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	rating, err := h.svc.Rate(c.Request.Context(), tenantID, userID, packageID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrPackageNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: rating})
}

// ListRatings 获取评分列表
// @Summary 获取工具评分列表
// @Tags Marketplace
// @Produce json
// @Param id path string true "工具包ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/packages/{id}/ratings [get]
func (h *Handler) ListRatings(c *gin.Context) {
	packageID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	ratings, total, err := h.svc.GetRatings(c.Request.Context(), packageID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"ratings":    ratings,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"totalPages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// Install 安装工具
// @Summary 安装工具
// @Tags Marketplace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "工具包ID"
// @Param version query string false "版本号（默认最新）"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/packages/{id}/install [post]
func (h *Handler) Install(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	packageID := c.Param("id")
	version := c.Query("version")

	install, err := h.svc.Install(c.Request.Context(), tenantID, userID, packageID, version)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrPackageNotFound || err == ErrVersionNotFound {
			status = http.StatusNotFound
		} else if err == ErrAlreadyInstalled {
			status = http.StatusConflict
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: install})
}

// Uninstall 卸载工具
// @Summary 卸载工具
// @Tags Marketplace
// @Security BearerAuth
// @Param id path string true "工具包ID"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/packages/{id}/uninstall [post]
func (h *Handler) Uninstall(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	packageID := c.Param("id")

	err := h.svc.Uninstall(c.Request.Context(), tenantID, userID, packageID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrNotInstalled {
			status = http.StatusNotFound
		}
		c.JSON(status, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "卸载成功"})
}

// ListInstalled 获取已安装的工具
// @Summary 获取已安装的工具列表
// @Tags Marketplace
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/installed [get]
func (h *Handler) ListInstalled(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	packages, err := h.svc.ListInstalled(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"packages": packages,
			"count":    len(packages),
		},
	})
}

// GetStats 获取市场统计
// @Summary 获取市场统计
// @Tags Marketplace
// @Produce json
// @Success 200 {object} MarketplaceStats
// @Router /api/marketplace/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: stats})
}

// --- 管理员接口 ---

// ApprovePackage 审核通过
// @Summary 审核通过工具包
// @Tags Marketplace Admin
// @Security BearerAuth
// @Param id path string true "工具包ID"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/admin/packages/{id}/approve [post]
func (h *Handler) ApprovePackage(c *gin.Context) {
	packageID := c.Param("id")

	if err := h.svc.ApprovePackage(c.Request.Context(), packageID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "审核通过"})
}

// RejectPackage 审核拒绝
// @Summary 审核拒绝工具包
// @Tags Marketplace Admin
// @Security BearerAuth
// @Param id path string true "工具包ID"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/admin/packages/{id}/reject [post]
func (h *Handler) RejectPackage(c *gin.Context) {
	packageID := c.Param("id")

	if err := h.svc.RejectPackage(c.Request.Context(), packageID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "已拒绝"})
}

// DeprecatePackage 废弃工具包
// @Summary 废弃工具包
// @Tags Marketplace Admin
// @Security BearerAuth
// @Param id path string true "工具包ID"
// @Success 200 {object} response.APIResponse
// @Router /api/marketplace/admin/packages/{id}/deprecate [post]
func (h *Handler) DeprecatePackage(c *gin.Context) {
	packageID := c.Param("id")

	if err := h.svc.DeprecatePackage(c.Request.Context(), packageID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "已废弃"})
}

// ListPendingPackages 获取待审核的工具包
// @Summary 获取待审核的工具包列表
// @Tags Marketplace Admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} PackageListResponse
// @Router /api/marketplace/admin/pending [get]
func (h *Handler) ListPendingPackages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.ListPendingPackages(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: result})
}
