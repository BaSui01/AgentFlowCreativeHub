package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse 通用响应结构，用于封装成功或失败结果。
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// Error 返回错误响应
func Error(c *gin.Context, code int, message string) {
	c.JSON(code, ErrorResponse{
		Success: false,
		Message: message,
	})
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

// Response 是 APIResponse 的别名，用于 Swagger 文档兼容。
type Response = APIResponse

// PagedResponse 分页响应结构，包含数据列表与分页信息。
type PagedResponse struct {
	Success    bool        `json:"success"`
	Data       interface{} `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}
