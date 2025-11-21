package types

// PaginationRequest 分页请求
type PaginationRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// PaginationResponse 分页响应
type PaginationResponse struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalItems int64 `json:"total_items"` // 等同于Total,为兼容性保留
	TotalPages int   `json:"total_pages"`
}
