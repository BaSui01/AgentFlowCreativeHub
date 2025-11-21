package common

import "gorm.io/gorm"

// NotDeleted 过滤已软删除的记录（默认查询行为）
// 使用方法：db.Scopes(common.NotDeleted()).Find(&users)
func NotDeleted() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("deleted_at IS NULL")
	}
}

// WithDeleted 包含已软删除的记录（查询所有记录）
// 使用方法：db.Scopes(common.WithDeleted()).Find(&users)
func WithDeleted() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Unscoped()
	}
}

// OnlyDeleted 仅查询已软删除的记录
// 使用方法：db.Scopes(common.OnlyDeleted()).Find(&users)
func OnlyDeleted() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("deleted_at IS NOT NULL")
	}
}

// ByTenant 按租户ID过滤（多租户查询通用Scope）
// 使用方法：db.Scopes(common.ByTenant(tenantID)).Find(&users)
func ByTenant(tenantID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("tenant_id = ?", tenantID)
	}
}

// ActiveOnly 仅查询活跃状态的记录
// 使用方法：db.Scopes(common.ActiveOnly()).Find(&users)
func ActiveOnly() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", "active")
	}
}
