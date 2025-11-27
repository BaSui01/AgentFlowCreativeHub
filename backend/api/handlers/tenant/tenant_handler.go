package tenant

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	response "backend/api/handlers/common"

	"github.com/gin-gonic/gin"

	"backend/internal/auth"
	tenantSvc "backend/internal/tenant"
)

// TenantHandler 聚合多租户管理相关的业务 Handler（租户、用户、角色、配置）。
type TenantHandler struct {
	tenantService  tenantSvc.TenantService
	userService    tenantSvc.UserService
	roleService    tenantSvc.RoleService
	configService  tenantSvc.TenantConfigService
	passwordHasher tenantSvc.PasswordHasher
}

func NewTenantHandler(
	tenantService tenantSvc.TenantService,
	userService tenantSvc.UserService,
	roleService tenantSvc.RoleService,
	configService tenantSvc.TenantConfigService,
	passwordHasher tenantSvc.PasswordHasher,
) *TenantHandler {
	return &TenantHandler{
		tenantService:  tenantService,
		userService:    userService,
		roleService:    roleService,
		configService:  configService,
		passwordHasher: passwordHasher,
	}
}

// --- Tenant APIs ---

// createTenantRequest 创建租户请求体。
type createTenantRequest struct {
	Name          string `json:"name" binding:"required"`
	Slug          string `json:"slug" binding:"required"`
	AdminEmail    string `json:"adminEmail" binding:"required"`
	AdminUsername string `json:"adminUsername" binding:"required"`
	AdminPassword string `json:"adminPassword" binding:"required"`
}

// updateTenantRequest 更新租户请求体。
type updateTenantRequest struct {
	Name   *string `json:"name"`
	Slug   *string `json:"slug"`
	Status *string `json:"status"`
}

// createUserRequest 创建用户请求体。
type createUserRequest struct {
	Email    string `json:"email" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// createRoleRequest 创建角色请求体。
type createRoleRequest struct {
	Name          string   `json:"name" binding:"required"`
	Description   string   `json:"description"`
	PermissionIDs []string `json:"permissionIds"`
}

// updateRoleRequest 更新角色请求体。
type updateRoleRequest struct {
	ID            string   `json:"id" binding:"required"`
	Name          string   `json:"name" binding:"required"`
	Description   string   `json:"description"`
	PermissionIDs []string `json:"permissionIds"`
}

type updateUserRolesRequest struct {
	RoleIDs []string `json:"roleIds"`
}

// updateConfigRequest 更新配置请求体。
type updateConfigRequest struct {
	DisplayName      *string                     `json:"displayName"`
	Description      *string                     `json:"description"`
	LogoURL          *string                     `json:"logoUrl"`
	Language         *string                     `json:"language"`
	TimeZone         *string                     `json:"timeZone"`
	FeatureFlags     *map[string]bool            `json:"featureFlags"`
	ApprovalSettings *tenantSvc.ApprovalSettings `json:"approvalSettings"`
}

// CreateTenant 供系统管理员创建租户
// @Summary 创建租户
// @Tags Tenants
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body createTenantRequest true "租户信息"
// @Success 201 {object} tenantSvc.Tenant
// @Failure 400 {object} response.ErrorResponse
// @Router /api/tenants [post]
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var body createTenantRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "invalid JSON body"})
		return
	}
	hash, err := h.passwordHasher.Hash(strings.TrimSpace(body.AdminPassword))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "failed to hash admin password"})
		return
	}
	params := tenantSvc.CreateTenantParams{
		Name:              strings.TrimSpace(body.Name),
		Slug:              strings.TrimSpace(body.Slug),
		AdminEmail:        strings.TrimSpace(body.AdminEmail),
		AdminUsername:     strings.TrimSpace(body.AdminUsername),
		AdminPasswordHash: hash,
	}

	ctx := h.contextWithOverrides(c, "", true)
	tenant, err := h.tenantService.CreateTenant(ctx, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tenant)
}

// ListTenants 列出所有租户（系统管理员）
// @Summary 租户列表
// @Tags Tenants
// @Security BearerAuth
// @Produce json
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} response.ListResponse
// @Router /api/tenants [get]
func (h *TenantHandler) ListTenants(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	ctx := h.contextWithOverrides(c, "", true)
	tenants, total, err := h.tenantService.ListTenants(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.ListResponse{
		Items: tenants,
		Pagination: response.PaginationMeta{
			Page:      page,
			PageSize:  pageSize,
			Total:     total,
			TotalPage: int((total + int64(pageSize) - 1) / int64(pageSize)),
		},
	})
}

// GetTenant 获取单个租户详情
// @Summary 获取租户详情
// @Tags Tenants
// @Security BearerAuth
// @Produce json
// @Param id path string true "租户 ID"
// @Success 200 {object} tenantSvc.Tenant
// @Failure 404 {object} response.ErrorResponse
// @Router /api/tenants/{id} [get]
func (h *TenantHandler) GetTenant(c *gin.Context) {
	id := c.Param("id")
	ctx := h.contextWithOverrides(c, "", false)
	tenant, err := h.tenantService.GetTenant(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

// UpdateTenant 更新租户信息
// @Summary 更新租户
// @Tags Tenants
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "租户 ID"
// @Param request body updateTenantRequest true "更新参数"
// @Success 200 {object} tenantSvc.Tenant
// @Failure 400 {object} response.ErrorResponse
// @Router /api/tenants/{id} [put]
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	id := c.Param("id")
	var body updateTenantRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "invalid JSON body"})
		return
	}
	params := tenantSvc.UpdateTenantParams{
		Name:   body.Name,
		Slug:   body.Slug,
		Status: body.Status,
	}

	ctx := h.contextWithOverrides(c, "", true)
	tenant, err := h.tenantService.UpdateTenant(ctx, id, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

// DeleteTenant 删除租户（软删除）
// @Summary 删除租户
// @Tags Tenants
// @Security BearerAuth
// @Produce json
// @Param id path string true "租户 ID"
// @Success 204 "删除成功"
// @Failure 400 {object} response.ErrorResponse
// @Router /api/tenants/{id} [delete]
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	id := c.Param("id")
	ctx := h.contextWithOverrides(c, "", true)
	if err := h.tenantService.DeleteTenant(ctx, id); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// --- User APIs ---

// CreateUser 在指定租户下创建用户
// @Summary 创建租户用户
// @Tags Tenants
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "租户 ID"
// @Param request body createUserRequest true "用户信息"
// @Success 201 {object} tenantSvc.User
// @Failure 400 {object} response.ErrorResponse
// @Router /api/tenants/{id}/users [post]
// @Router /api/tenant/users [post] // deprecated
func (h *TenantHandler) CreateUser(c *gin.Context) {
	var body createUserRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "invalid JSON body"})
		return
	}
	params := tenantSvc.CreateUserParams{
		Email:    strings.TrimSpace(body.Email),
		Username: strings.TrimSpace(body.Username),
		Password: body.Password,
	}

	tenantID := c.Param("id")
	if tenantID == "" {
		tenantID = c.GetString("tenant_id")
	}
	ctx := h.contextWithOverrides(c, tenantID, true)

	u, err := h.userService.CreateUser(ctx, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

// ListUsers 列出当前租户下用户
// @Summary 列出租户用户
// @Tags Tenants
// @Security BearerAuth
// @Produce json
// @Param id path string true "租户 ID"
// @Success 200 {object} response.ListResponse
// @Router /api/tenants/{id}/users [get]
// @Router /api/tenant/users [get] // deprecated
func (h *TenantHandler) ListUsers(c *gin.Context) {
	tenantID := c.Param("id")
	if tenantID == "" {
		tenantID = c.GetString("tenant_id")
	}
	ctx := h.contextWithOverrides(c, tenantID, false)

	users, err := h.userService.ListUsers(ctx)
	if err != nil {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.ListResponse{
		Items: users,
		Pagination: response.PaginationMeta{
			Page:      1,
			PageSize:  len(users),
			Total:     int64(len(users)),
			TotalPage: 1,
		},
	})
}

// --- Role APIs ---

// CreateRole 创建角色
// @Summary 创建角色
// @Tags Tenants
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "租户 ID"
// @Param request body createRoleRequest true "角色信息"
// @Success 201 {object} tenantSvc.Role
// @Failure 400 {object} response.ErrorResponse
// @Router /api/tenants/{id}/roles [post]
// @Router /api/tenant/roles [post] // deprecated
func (h *TenantHandler) CreateRole(c *gin.Context) {
	var body createRoleRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "invalid JSON body"})
		return
	}
	params := tenantSvc.CreateRoleParams{
		Name:          strings.TrimSpace(body.Name),
		Description:   strings.TrimSpace(body.Description),
		PermissionIDs: body.PermissionIDs,
	}
	tenantID := c.Param("id")
	if tenantID == "" {
		tenantID = c.GetString("tenant_id")
	}
	ctx := h.contextWithOverrides(c, tenantID, false)

	role, err := h.roleService.CreateRole(ctx, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, role)
}

// UpdateRole 更新角色
// @Summary 更新角色
// @Tags Tenants
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "租户 ID"
// @Param request body updateRoleRequest true "角色信息"
// @Success 200 {object} tenantSvc.Role
// @Failure 400 {object} response.ErrorResponse
// @Router /api/tenants/{id}/roles [put]
// @Router /api/tenant/roles [put] // deprecated
func (h *TenantHandler) UpdateRole(c *gin.Context) {
	var body updateRoleRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "invalid JSON body"})
		return
	}
	params := tenantSvc.UpdateRoleParams{
		Name:          strings.TrimSpace(body.Name),
		Description:   strings.TrimSpace(body.Description),
		PermissionIDs: body.PermissionIDs,
	}
	tenantID := c.Param("id")
	if tenantID == "" {
		tenantID = c.GetString("tenant_id")
	}
	ctx := h.contextWithOverrides(c, tenantID, false)
	role, err := h.roleService.UpdateRole(ctx, body.ID, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, role)
}

// ListRoles 获取租户角色及权限
func (h *TenantHandler) ListRoles(c *gin.Context) {
	tenantID := c.Param("id")
	ctx := h.contextWithOverrides(c, tenantID, false)
	roles, err := h.roleService.ListRoles(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	items := make([]map[string]any, 0, len(roles))
	for _, r := range roles {
		items = append(items, map[string]any{
			"id":            r.Role.ID,
			"name":          r.Role.Name,
			"code":          r.Role.Code,
			"description":   r.Role.Description,
			"isSystem":      r.Role.IsSystem,
			"isDefault":     r.Role.IsDefault,
			"priority":      r.Role.Priority,
			"permissionIds": r.PermissionIDs,
			"createdAt":     r.Role.CreatedAt,
			"updatedAt":     r.Role.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, response.ListResponse{
		Items: items,
		Pagination: response.PaginationMeta{
			Page:      1,
			PageSize:  len(items),
			Total:     int64(len(items)),
			TotalPage: 1,
		},
	})
}

// DeleteRole 删除角色
func (h *TenantHandler) DeleteRole(c *gin.Context) {
	roleID := c.Param("roleId")
	if roleID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "missing roleId"})
		return
	}
	ctx := h.contextWithOverrides(c, c.Param("id"), false)
	if err := h.roleService.DeleteRole(ctx, roleID); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ReplaceUserRoles 批量替换用户角色
func (h *TenantHandler) ReplaceUserRoles(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "missing userId"})
		return
	}
	var body updateUserRolesRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "invalid JSON body"})
		return
	}
	ctx := h.contextWithOverrides(c, c.Param("id"), false)
	if err := h.roleService.ReplaceUserRoles(ctx, userID, body.RoleIDs); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ListUserRoles 列出指定用户的角色 ID
func (h *TenantHandler) ListUserRoles(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "missing userId"})
		return
	}
	ctx := h.contextWithOverrides(c, c.Param("id"), false)
	roleIDs, err := h.roleService.ListUserRoles(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.ListResponse{
		Items: roleIDs,
		Pagination: response.PaginationMeta{
			Page:      1,
			PageSize:  len(roleIDs),
			Total:     int64(len(roleIDs)),
			TotalPage: 1,
		},
	})
}

// ListPermissions 列出可用权限
// @Summary 列出权限
// @Tags Tenants
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.ListResponse
// @Router /api/tenant/permissions [get]
func (h *TenantHandler) ListPermissions(c *gin.Context) {
	ctx := h.contextWithOverrides(c, "", false)
	perms, err := h.roleService.ListPermissions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.ListResponse{
		Items: perms,
		Pagination: response.PaginationMeta{
			Page:      1,
			PageSize:  len(perms),
			Total:     int64(len(perms)),
			TotalPage: 1,
		},
	})
}

// GetPermissionCatalog 返回带多语言的权限字典
func (h *TenantHandler) GetPermissionCatalog(c *gin.Context) {
	catalog := tenantSvc.GetPermissionCatalog()
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, catalog)
}

// --- Config APIs ---

// GetConfig 读取当前租户配置
// @Summary 获取租户配置
// @Tags Tenants
// @Security BearerAuth
// @Produce json
// @Success 200 {object} tenantSvc.TenantConfig
// @Failure 403 {object} response.ErrorResponse
// @Router /api/tenant/config [get]
func (h *TenantHandler) GetConfig(c *gin.Context) {
	ctx := h.contextWithOverrides(c, "", false)
	cfg, err := h.configService.GetConfig(ctx)
	if err != nil {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// UpdateConfig 更新当前租户配置
// @Summary 更新租户配置
// @Tags Tenants
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body updateConfigRequest true "配置内容"
// @Success 200 {object} tenantSvc.TenantConfig
// @Failure 400 {object} response.ErrorResponse
// @Router /api/tenant/config [put]
func (h *TenantHandler) UpdateConfig(c *gin.Context) {
	var body updateConfigRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "invalid JSON body"})
		return
	}
	params := tenantSvc.UpdateTenantConfigParams{
		DisplayName:      body.DisplayName,
		Description:      body.Description,
		LogoURL:          body.LogoURL,
		Language:         body.Language,
		TimeZone:         body.TimeZone,
		FeatureFlags:     body.FeatureFlags,
		ApprovalSettings: body.ApprovalSettings,
	}

	ctx := h.contextWithOverrides(c, "", false)
	cfg, err := h.configService.UpdateConfig(ctx, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (h *TenantHandler) contextWithOverrides(c *gin.Context, tenantID string, forceAdmin bool) context.Context {
	tc := h.buildTenantContext(c)
	if tenantID != "" {
		tc.TenantID = tenantID
	}
	if forceAdmin {
		tc.IsSystemAdmin = true
	}
	return tenantSvc.WithTenantContext(c.Request.Context(), tc)
}

func (h *TenantHandler) buildTenantContext(c *gin.Context) tenantSvc.TenantContext {
	tc := tenantSvc.TenantContext{
		TenantID:      strings.TrimSpace(c.GetString("tenant_id")),
		UserID:        strings.TrimSpace(c.GetString("user_id")),
		IsSystemAdmin: false,
	}

	if userCtx, ok := auth.GetUserContext(c); ok {
		if userCtx.TenantID != "" {
			tc.TenantID = userCtx.TenantID
		}
		if userCtx.UserID != "" {
			tc.UserID = userCtx.UserID
		}
		tc.Roles = userCtx.Roles
		tc.IsSystemAdmin = hasSystemAdminRole(userCtx.Roles)
	}

	if tc.TenantID == "" {
		tc.TenantID = strings.TrimSpace(c.Param("id"))
	}
	if tc.TenantID == "" {
		tc.TenantID = strings.TrimSpace(c.GetHeader("X-Tenant-ID"))
	}
	if tc.UserID == "" {
		tc.UserID = strings.TrimSpace(c.GetHeader("X-User-ID"))
	}

	if len(tc.Roles) == 0 && !tc.IsSystemAdmin {
		// 仍处于开发阶段时允许缺省权限，避免历史调用全部失败
		tc.IsSystemAdmin = true
	}

	return tc
}

func hasSystemAdminRole(roles []string) bool {
	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "super_admin", "system_admin":
			return true
		}
	}
	return false
}
