package credits

import (
	"net/http"
	"strconv"
	"time"

	response "backend/api/handlers/common"
	creditsSvc "backend/internal/credits"

	"github.com/gin-gonic/gin"
)

// Handler 积分管理处理器
type Handler struct {
	svc *creditsSvc.Service
}

// NewHandler 创建处理器
func NewHandler(svc *creditsSvc.Service) *Handler {
	return &Handler{svc: svc}
}

// GetBalance 获取当前用户余额
// @Summary 获取积分余额
// @Tags Credits
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/credits/balance [get]
func (h *Handler) GetBalance(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	account, err := h.svc.GetOrCreateAccount(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: account})
}

// GetUserBalance 获取指定用户余额（管理员）
// @Summary 获取指定用户积分余额
// @Tags Credits
// @Security BearerAuth
// @Param userId path string true "用户ID"
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/credits/users/{userId}/balance [get]
func (h *Handler) GetUserBalance(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.Param("userId")

	account, err := h.svc.GetOrCreateAccount(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: account})
}

type rechargeDTO struct {
	UserID string `json:"userId" binding:"required"`
	Amount int64  `json:"amount" binding:"required,gt=0"`
	Remark string `json:"remark"`
}

// Recharge 管理员充值
// @Summary 为用户充值积分
// @Tags Credits
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body rechargeDTO true "充值请求"
// @Success 200 {object} response.APIResponse
// @Router /api/credits/recharge [post]
func (h *Handler) Recharge(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	operatorID := c.GetString("user_id")
	operatorName := c.GetString("username")

	var dto rechargeDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	tx, err := h.svc.Recharge(c.Request.Context(), &creditsSvc.RechargeRequest{
		TenantID:     tenantID,
		UserID:       dto.UserID,
		Amount:       dto.Amount,
		Remark:       dto.Remark,
		OperatorID:   operatorID,
		OperatorName: operatorName,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: tx, Message: "充值成功"})
}

type giftDTO struct {
	UserID      string `json:"userId" binding:"required"`
	Amount      int64  `json:"amount" binding:"required,gt=0"`
	Type        string `json:"type"` // register, activity, gift
	Description string `json:"description"`
}

// Gift 赠送积分
// @Summary 赠送积分给用户
// @Tags Credits
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body giftDTO true "赠送请求"
// @Success 200 {object} response.APIResponse
// @Router /api/credits/gift [post]
func (h *Handler) Gift(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	operatorID := c.GetString("user_id")
	operatorName := c.GetString("username")

	var dto giftDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	txType := creditsSvc.TransactionTypeGift
	if dto.Type != "" {
		txType = creditsSvc.TransactionType(dto.Type)
	}

	tx, err := h.svc.Gift(c.Request.Context(), &creditsSvc.GiftRequest{
		TenantID:     tenantID,
		UserID:       dto.UserID,
		Amount:       dto.Amount,
		Type:         txType,
		Description:  dto.Description,
		OperatorID:   operatorID,
		OperatorName: operatorName,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: tx, Message: "赠送成功"})
}

// ListTransactions 查询流水
// @Summary 查询积分流水
// @Tags Credits
// @Security BearerAuth
// @Param userId query string false "用户ID"
// @Param type query string false "交易类型"
// @Param startTime query string false "开始时间"
// @Param endTime query string false "结束时间"
// @Param limit query int false "每页数量"
// @Param offset query int false "偏移量"
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/credits/transactions [get]
func (h *Handler) ListTransactions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	currentUserID := c.GetString("user_id")

	query := &creditsSvc.TransactionQuery{
		TenantID: tenantID,
	}

	// 普通用户只能查看自己的流水
	if userID := c.Query("userId"); userID != "" {
		// TODO: 检查是否是管理员
		query.UserID = userID
	} else {
		query.UserID = currentUserID
	}

	if t := c.Query("type"); t != "" {
		query.Type = creditsSvc.TransactionType(t)
	}

	if startStr := c.Query("startTime"); startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			query.StartTime = &t
		}
	}

	if endStr := c.Query("endTime"); endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			end := t.AddDate(0, 0, 1)
			query.EndTime = &end
		}
	}

	if limit, _ := strconv.Atoi(c.Query("limit")); limit > 0 {
		query.Limit = limit
	}
	if offset, _ := strconv.Atoi(c.Query("offset")); offset > 0 {
		query.Offset = offset
	}

	transactions, total, err := h.svc.ListTransactions(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"items": transactions,
			"total": total,
		},
	})
}

// GetStats 获取统计数据
// @Summary 获取积分统计
// @Tags Credits
// @Security BearerAuth
// @Param period query string false "统计周期 (daily/weekly/monthly)"
// @Param userId query string false "用户ID"
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/credits/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.Query("userId")
	if userID == "" {
		userID = c.GetString("user_id")
	}
	period := c.DefaultQuery("period", "monthly")

	stats, err := h.svc.GetStats(c.Request.Context(), tenantID, userID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: stats})
}

// ListUserSummaries 获取用户积分摘要列表
// @Summary 获取所有用户积分摘要
// @Tags Credits
// @Security BearerAuth
// @Param limit query int false "每页数量"
// @Param offset query int false "偏移量"
// @Produce json
// @Success 200 {object} response.APIResponse
// @Router /api/credits/users [get]
func (h *Handler) ListUserSummaries(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	summaries, total, err := h.svc.ListUserSummaries(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{
		Success: true,
		Data: gin.H{
			"items": summaries,
			"total": total,
		},
	})
}

type updateThresholdDTO struct {
	Threshold int64 `json:"threshold" binding:"required,gte=0"`
}

// UpdateWarnThreshold 更新预警阈值
// @Summary 更新积分预警阈值
// @Tags Credits
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body updateThresholdDTO true "阈值"
// @Success 200 {object} response.APIResponse
// @Router /api/credits/warn-threshold [put]
func (h *Handler) UpdateWarnThreshold(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var dto updateThresholdDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	if err := h.svc.UpdateWarnThreshold(c.Request.Context(), tenantID, userID, dto.Threshold); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "更新成功"})
}

// ExportTransactions 导出流水CSV
// @Summary 导出积分流水为CSV
// @Tags Credits
// @Security BearerAuth
// @Param userId query string false "用户ID"
// @Param type query string false "交易类型"
// @Param startTime query string false "开始时间"
// @Param endTime query string false "结束时间"
// @Produce text/csv
// @Success 200 {string} string "CSV内容"
// @Router /api/credits/export [get]
func (h *Handler) ExportTransactions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	query := &creditsSvc.TransactionQuery{
		TenantID: tenantID,
	}

	if userID := c.Query("userId"); userID != "" {
		query.UserID = userID
	}
	if t := c.Query("type"); t != "" {
		query.Type = creditsSvc.TransactionType(t)
	}
	if startStr := c.Query("startTime"); startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			query.StartTime = &t
		}
	}
	if endStr := c.Query("endTime"); endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			end := t.AddDate(0, 0, 1)
			query.EndTime = &end
		}
	}

	csvContent, err := h.svc.ExportTransactionsCSV(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	filename := "credits_" + time.Now().Format("20060102150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	// 添加 BOM 以支持 Excel 打开
	c.String(http.StatusOK, "\xEF\xBB\xBF"+csvContent)
}
