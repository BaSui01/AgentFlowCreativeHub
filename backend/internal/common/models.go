package common

import "time"

// SoftDeleteModel 软删除基础模型
// 提供统一的软删除字段和方法，可嵌入到需要软删除功能的模型中
type SoftDeleteModel struct {
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
	DeletedBy string     `json:"deletedBy,omitempty" gorm:"size:100"`
}

// IsDeleted 检查记录是否已被软删除
func (m *SoftDeleteModel) IsDeleted() bool {
	return m.DeletedAt != nil
}

// SoftDelete 执行软删除操作
// operatorID: 执行删除操作的用户ID
func (m *SoftDeleteModel) SoftDelete(operatorID string) {
	now := time.Now()
	m.DeletedAt = &now
	m.DeletedBy = operatorID
}

// Restore 恢复已删除的记录
func (m *SoftDeleteModel) Restore() {
	m.DeletedAt = nil
	m.DeletedBy = ""
}

// TimestampModel 时间戳基础模型
// 提供统一的创建时间和更新时间字段
type TimestampModel struct {
	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"not null;autoUpdateTime"`
}

// AuditableModel 可审计模型
// 结合时间戳和软删除功能
type AuditableModel struct {
	TimestampModel
	SoftDeleteModel
}
