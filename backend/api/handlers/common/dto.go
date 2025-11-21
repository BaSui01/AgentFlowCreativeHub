package common

// APIResponse 通用响应结构，用于封装成功或失败结果。
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginationMeta 分页元信息。
type PaginationMeta struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int   `json:"total_page"`
}

// ListResponse 列表响应结构，包含数据与分页信息。
type ListResponse struct {
	Items      interface{}    `json:"items"`
	Pagination PaginationMeta `json:"pagination"`
}

// ErrorResponse 统一错误返回结构。
type ErrorResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}
