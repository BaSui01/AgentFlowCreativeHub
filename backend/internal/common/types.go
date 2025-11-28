package common

import "time"

// ============================================================================
// 通用请求类型
// ============================================================================

// PaginationRequest 分页请求参数
type PaginationRequest struct {
	Page     int `json:"page" form:"page" binding:"omitempty,min=1"`           // 页码，从1开始
	PageSize int `json:"page_size" form:"page_size" binding:"omitempty,min=1"` // 每页数量
}

// DefaultPagination 返回默认分页参数
func DefaultPagination() PaginationRequest {
	return PaginationRequest{
		Page:     1,
		PageSize: 20,
	}
}

// GetOffset 计算数据库查询的偏移量
func (p PaginationRequest) GetOffset() int {
	if p.Page < 1 {
		p.Page = 1
	}
	return (p.Page - 1) * p.GetPageSize()
}

// GetPageSize 获取每页数量，提供默认值
func (p PaginationRequest) GetPageSize() int {
	if p.PageSize < 1 {
		return 20
	}
	if p.PageSize > 100 {
		return 100
	}
	return p.PageSize
}

// FilterRequest 通用过滤请求
type FilterRequest struct {
	Keyword       string            `json:"keyword" form:"keyword"`                 // 关键词搜索
	Status        string            `json:"status" form:"status"`                   // 状态筛选
	DateRange     *DateRange        `json:"date_range"`                             // 日期范围
	CustomFilters map[string]any    `json:"filters"`                                // 自定义过滤条件
	SortBy        string            `json:"sort_by" form:"sort_by"`                 // 排序字段
	SortOrder     string            `json:"sort_order" form:"sort_order"`           // 排序方向: asc, desc
}

// DateRange 日期范围
type DateRange struct {
	Start time.Time `json:"start"` // 开始时间
	End   time.Time `json:"end"`   // 结束时间
}

// ListRequest 通用列表请求（组合分页和过滤）
type ListRequest struct {
	PaginationRequest
	FilterRequest
}

// IDRequest 通过ID查询的请求
type IDRequest struct {
	ID string `json:"id" uri:"id" binding:"required"` // 资源ID
}

// IDsRequest 批量ID请求
type IDsRequest struct {
	IDs []string `json:"ids" binding:"required,min=1"` // 资源ID列表
}

// ============================================================================
// 通用响应类型
// ============================================================================

// APIResponse 统一API响应格式
type APIResponse struct {
	Success bool   `json:"success"`           // 是否成功
	Data    any    `json:"data,omitempty"`    // 响应数据
	Message string `json:"message,omitempty"` // 提示信息
	Code    int    `json:"code"`              // 业务状态码
}

// SuccessResponse 成功响应
func SuccessResponse(data any) APIResponse {
	return APIResponse{
		Success: true,
		Data:    data,
		Code:    0,
	}
}

// SuccessMessageResponse 成功响应（带消息）
func SuccessMessageResponse(message string, data any) APIResponse {
	return APIResponse{
		Success: true,
		Data:    data,
		Message: message,
		Code:    0,
	}
}

// ErrorResponse 错误响应
func ErrorResponse(code int, message string) APIResponse {
	return APIResponse{
		Success: false,
		Message: message,
		Code:    code,
	}
}

// PaginationMeta 分页元信息
type PaginationMeta struct {
	Page       int   `json:"page"`        // 当前页码
	PageSize   int   `json:"page_size"`   // 每页数量
	Total      int64 `json:"total"`       // 总记录数
	TotalPages int   `json:"total_pages"` // 总页数
}

// CalculateTotalPages 计算总页数
func (m *PaginationMeta) CalculateTotalPages() {
	if m.PageSize > 0 {
		m.TotalPages = int((m.Total + int64(m.PageSize) - 1) / int64(m.PageSize))
	}
}

// NewPaginationMeta 创建分页元信息
func NewPaginationMeta(page, pageSize int, total int64) PaginationMeta {
	meta := PaginationMeta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
	meta.CalculateTotalPages()
	return meta
}

// ListResponse 列表响应（包含分页信息）
type ListResponse struct {
	Items      any            `json:"items"`      // 数据列表
	Pagination PaginationMeta `json:"pagination"` // 分页信息
}

// NewListResponse 创建列表响应
func NewListResponse(items any, page, pageSize int, total int64) ListResponse {
	return ListResponse{
		Items:      items,
		Pagination: NewPaginationMeta(page, pageSize, total),
	}
}

// ============================================================================
// 业务状态码定义
// ============================================================================

const (
	// 成功状态码
	CodeSuccess = 0

	// 通用错误码 (1000-1999)
	CodeInvalidRequest   = 1000 // 请求参数错误
	CodeUnauthorized     = 1001 // 未授权
	CodeForbidden        = 1002 // 禁止访问
	CodeNotFound         = 1003 // 资源不存在
	CodeConflict         = 1004 // 资源冲突
	CodeInternalError    = 1005 // 内部错误
	CodeServiceUnavailable = 1006 // 服务不可用

	// 租户相关错误码 (2000-2099)
	CodeTenantNotFound      = 2000 // 租户不存在
	CodeTenantDisabled      = 2001 // 租户已禁用
	CodeTenantQuotaExceeded = 2002 // 租户配额超限
	CodeUserNotFound        = 2010 // 用户不存在
	CodeUserDisabled        = 2011 // 用户已禁用
	CodeInvalidCredentials  = 2012 // 凭证无效
	CodeRoleNotFound        = 2020 // 角色不存在

	// 模型相关错误码 (3000-3099)
	CodeModelNotFound        = 3000 // 模型不存在
	CodeModelCallFailed      = 3001 // 模型调用失败
	CodeInvalidModelConfig   = 3002 // 模型配置无效
	CodeCredentialNotFound   = 3010 // 凭证不存在
	CodeCredentialInvalid    = 3011 // 凭证无效

	// Agent相关错误码 (4000-4099)
	CodeAgentNotFound      = 4000 // Agent不存在
	CodeAgentExecutionFailed = 4001 // Agent执行失败
	CodeInvalidAgentConfig = 4002 // Agent配置无效

	// 工作流相关错误码 (5000-5099)
	CodeWorkflowNotFound      = 5000 // 工作流不存在
	CodeWorkflowValidationFailed = 5001 // 工作流验证失败
	CodeWorkflowExecutionFailed = 5002 // 工作流执行失败

	// 知识库相关错误码 (6000-6099)
	CodeKnowledgeBaseNotFound = 6000 // 知识库不存在
	CodeDocumentNotFound      = 6001 // 文档不存在
	CodeVectorSearchFailed    = 6002 // 向量检索失败
)

// ErrorMessages 错误码对应的默认消息
var ErrorMessages = map[int]string{
	CodeSuccess:            "操作成功",
	CodeInvalidRequest:     "请求参数错误",
	CodeUnauthorized:       "未授权，请先登录",
	CodeForbidden:          "无权限访问",
	CodeNotFound:           "资源不存在",
	CodeConflict:           "资源冲突",
	CodeInternalError:      "系统内部错误",
	CodeServiceUnavailable: "服务暂不可用",

	CodeTenantNotFound:      "租户不存在",
	CodeTenantDisabled:      "租户已禁用",
	CodeTenantQuotaExceeded: "租户配额已超限",
	CodeUserNotFound:        "用户不存在",
	CodeUserDisabled:        "用户已禁用",
	CodeInvalidCredentials:  "用户名或密码错误",
	CodeRoleNotFound:        "角色不存在",

	CodeModelNotFound:      "模型不存在",
	CodeModelCallFailed:    "模型调用失败",
	CodeInvalidModelConfig: "模型配置无效",
	CodeCredentialNotFound: "凭证不存在",
	CodeCredentialInvalid:  "凭证无效",

	CodeAgentNotFound:        "Agent不存在",
	CodeAgentExecutionFailed: "Agent执行失败",
	CodeInvalidAgentConfig:   "Agent配置无效",

	CodeWorkflowNotFound:          "工作流不存在",
	CodeWorkflowValidationFailed:  "工作流验证失败",
	CodeWorkflowExecutionFailed:   "工作流执行失败",

	CodeKnowledgeBaseNotFound: "知识库不存在",
	CodeDocumentNotFound:      "文档不存在",
	CodeVectorSearchFailed:    "向量检索失败",
}

// GetErrorMessage 获取错误码对应的消息
func GetErrorMessage(code int) string {
	if msg, ok := ErrorMessages[code]; ok {
		return msg
	}
	return "未知错误"
}

// ============================================================================
// 通用业务错误类型
// ============================================================================

// BusinessError 业务错误
type BusinessError struct {
	Code    int    // 错误码
	Message string // 错误信息
}

// Error 实现error接口
func (e *BusinessError) Error() string {
	return e.Message
}

// NewBusinessError 创建业务错误
func NewBusinessError(code int, message string) *BusinessError {
	if message == "" {
		message = GetErrorMessage(code)
	}
	return &BusinessError{
		Code:    code,
		Message: message,
	}
}

// NewBusinessErrorWithCode 根据错误码创建业务错误
func NewBusinessErrorWithCode(code int) *BusinessError {
	return NewBusinessError(code, GetErrorMessage(code))
}

// ============================================================================
// 资源统计信息
// ============================================================================

// ResourceStats 资源统计信息
type ResourceStats struct {
	TotalCount   int64     `json:"total_count"`   // 总数
	ActiveCount  int64     `json:"active_count"`  // 活跃数
	CreatedToday int64     `json:"created_today"` // 今日新增
	UpdatedAt    time.Time `json:"updated_at"`    // 统计更新时间
}

// UsageStats 用量统计
type UsageStats struct {
	ResourceType string    `json:"resource_type"` // 资源类型
	Used         int64     `json:"used"`          // 已使用
	Limit        int64     `json:"limit"`         // 限制
	Percentage   float64   `json:"percentage"`    // 使用率
	UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
}

// CalculatePercentage 计算使用率
func (s *UsageStats) CalculatePercentage() {
	if s.Limit > 0 {
		s.Percentage = float64(s.Used) / float64(s.Limit) * 100
	}
}
