package knowledge

import (
	response "backend/api/handlers/common"
	"backend/pkg/types"
)

func toPaginationMeta(p *types.PaginationResponse) response.PaginationMeta {
	if p == nil {
		return response.PaginationMeta{}
	}
	return response.PaginationMeta{
		Page:      p.Page,
		PageSize:  p.PageSize,
		Total:     p.Total,
		TotalPage: p.TotalPages,
	}
}
