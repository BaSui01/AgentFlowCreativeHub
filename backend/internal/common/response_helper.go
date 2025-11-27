package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ResponseSuccess 返回成功响应
func ResponseSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, SuccessResponse(data))
}

// ResponseSuccessMessage 返回成功响应（带消息）
func ResponseSuccessMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, SuccessMessageResponse(message, data))
}

// ResponseList 返回分页列表响应
func ResponseList(c *gin.Context, items any, total int64, req *PaginationRequest) {
	if req == nil {
		defaultReq := DefaultPagination()
		req = &defaultReq
	}

	pageSize := req.GetPageSize()
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	response := ListResponse{
		Items: items,
		Pagination: PaginationMeta{
			Page:       req.Page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	c.JSON(http.StatusOK, SuccessResponse(response))
}

// ResponseError 返回错误响应
func ResponseError(c *gin.Context, code int, message string) {
	httpStatus := http.StatusOK // 业务错误也返回200

	// 特殊业务状态码映射到HTTP状态码
	switch code {
	case CodeUnauthorized:
		httpStatus = http.StatusUnauthorized
	case CodeForbidden:
		httpStatus = http.StatusForbidden
	case CodeNotFound:
		httpStatus = http.StatusNotFound
	case CodeInvalidRequest:
		httpStatus = http.StatusBadRequest
	case CodeInternalError:
		httpStatus = http.StatusInternalServerError
	}

	c.JSON(httpStatus, ErrorResponse(code, message))
}

// ResponseBusinessError 返回业务错误响应
func ResponseBusinessError(c *gin.Context, err *BusinessError) {
	ResponseError(c, err.Code, err.Message)
}

// AbortWithError 中断并返回错误
func AbortWithError(c *gin.Context, code int, message string) {
	ResponseError(c, code, message)
	c.Abort()
}

// ResponseCreated 返回创建成功响应（201）
func ResponseCreated(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, SuccessResponse(data))
}

// ResponseNoContent 返回无内容响应（204）
func ResponseNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
// ResponseBadRequest 返回参数错误响应
func ResponseBadRequest(c *gin.Context, message string) {
	ResponseError(c, CodeInvalidRequest, message)
}

// ResponseUnauthorized 返回未认证响应
func ResponseUnauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "未认证，请先登录"
	}
	ResponseError(c, CodeUnauthorized, message)
}

// ResponseForbidden 返回无权限响应
func ResponseForbidden(c *gin.Context, message string) {
	if message == "" {
		message = "权限不足"
	}
	ResponseError(c, CodeForbidden, message)
}

// ResponseNotFound 返回资源不存在响应
func ResponseNotFound(c *gin.Context, message string) {
	if message == "" {
		message = "资源不存在"
	}
	ResponseError(c, CodeNotFound, message)
}

// ResponseServerError 返回服务器错误响应
func ResponseServerError(c *gin.Context, message string) {
	if message == "" {
		message = "服务器内部错误"
	}
	ResponseError(c, CodeInternalError, message)
}


// ResponseWithPagination 返回带分页的响应（兼容旧接口）
func ResponseWithPagination(c *gin.Context, items any, total int64, page, pageSize int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	response := ListResponse{
		Items: items,
		Pagination: PaginationMeta{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	c.JSON(http.StatusOK, SuccessResponse(response))
}
