package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIResponse(t *testing.T) {
	t.Run("成功响应", func(t *testing.T) {
		resp := APIResponse{
			Success: true,
			Message: "Operation successful",
			Data: map[string]string{
				"key": "value",
			},
		}

		assert.True(t, resp.Success)
		assert.Equal(t, "Operation successful", resp.Message)
		assert.NotNil(t, resp.Data)
	})

	t.Run("错误响应", func(t *testing.T) {
		resp := ErrorResponse{
			Success: false,
			Message: "Operation failed",
		}

		assert.False(t, resp.Success)
		assert.Equal(t, "Operation failed", resp.Message)
	})
}

func TestListResponse(t *testing.T) {
	t.Run("分页响应", func(t *testing.T) {
		resp := ListResponse{
			Items: []interface{}{
				map[string]string{"id": "1"},
				map[string]string{"id": "2"},
			},
			Pagination: PaginationMeta{
				Page:      1,
				PageSize:  20,
				Total:     2,
				TotalPage: 1,
			},
		}

		assert.Len(t, resp.Items, 2)
		assert.Equal(t, 1, resp.Pagination.Page)
		assert.Equal(t, int64(2), resp.Pagination.Total)
	})
}

func TestPaginationMeta(t *testing.T) {
	t.Run("计算总页数", func(t *testing.T) {
		meta := PaginationMeta{
			Page:      1,
			PageSize:  20,
			Total:     45,
			TotalPage: 3,
		}

		assert.Equal(t, 1, meta.Page)
		assert.Equal(t, 20, meta.PageSize)
		assert.Equal(t, int64(45), meta.Total)
		assert.Equal(t, 3, meta.TotalPage)
	})

	t.Run("空列表", func(t *testing.T) {
		meta := PaginationMeta{
			Page:      1,
			PageSize:  20,
			Total:     0,
			TotalPage: 0,
		}

		assert.Equal(t, int64(0), meta.Total)
		assert.Equal(t, 0, meta.TotalPage)
	})
}
