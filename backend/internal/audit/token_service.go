package audit

import (
	"gorm.io/gorm"
)

// GormTokenAuditService 基于 GORM 的 Token 审计服务
type GormTokenAuditService struct {
	db *gorm.DB
}

// NewTokenAuditService 创建 Token 审计服务
func NewTokenAuditService(db *gorm.DB) *GormTokenAuditService {
	return &GormTokenAuditService{db: db}
}

// RecordTokenUsage 记录 Token 使用情况
func (s *GormTokenAuditService) RecordTokenUsage(usage *TokenUsage) error {
	if usage.ID == "" {
		// Generate UUID if missing, or let DB handle it? 
		// GORM usually handles UUID generation if configured, but explicit is safer for uuid strings.
		// For now let's assume the caller or DB hook handles ID generation, 
		// or we can use uuid.New().String() if we import google/uuid.
		// To keep imports minimal here, relying on DB default or caller is better.
	}
	return s.db.Create(usage).Error
}
