package models

import (
	"context"
	"encoding/json"
	"time"

	"backend/pkg/types"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog 审计日志
type AuditLog struct {
	ID           string                 `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID     string                 `gorm:"type:uuid;not null;index:idx_audit_tenant" json:"tenant_id"`
	UserID       string                 `gorm:"type:uuid;index:idx_audit_user" json:"user_id"`
	EventType    string                 `gorm:"type:varchar(100);not null;index:idx_audit_event_type" json:"event_type"`
	EventCategory string                `gorm:"type:varchar(50);not null;index:idx_audit_category" json:"event_category"`
	EventLevel   string                 `gorm:"type:varchar(20);not null" json:"event_level"`
	Description  string                 `gorm:"type:text" json:"description"`
	IPAddress    string                 `gorm:"type:varchar(100)" json:"ip_address"`
	UserAgent    string                 `gorm:"type:text" json:"user_agent"`
	RequestPath  string                 `gorm:"type:text" json:"request_path"`
	RequestMethod string                `gorm:"type:varchar(10)" json:"request_method"`
	StatusCode   int                    `gorm:"type:int" json:"status_code"`
	Metadata     map[string]interface{} `gorm:"type:jsonb" json:"metadata"`
	CreatedAt    time.Time              `gorm:"not null;index:idx_audit_created_at" json:"created_at"`
}

// BeforeCreate GORM 钩子：创建前设置 ID
func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	return nil
}

// TableName 指定表名
func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditLogService 审计日志服务
type AuditLogService struct {
	db *gorm.DB
}

// NewAuditLogService 创建审计日志服务
func NewAuditLogService(db *gorm.DB) *AuditLogService {
	return &AuditLogService{db: db}
}

// CreateLog 创建审计日志
func (s *AuditLogService) CreateLog(ctx context.Context, log *AuditLog) error {
	// 移除对audit包的依赖
	// EventCategory和EventLevel由调用方设置
	// 如果未设置,使用默认值
	if log.EventCategory == "" {
		log.EventCategory = "general"
	}
	if log.EventLevel == "" {
		log.EventLevel = "info"
	}

	return s.db.WithContext(ctx).Create(log).Error
}

// CreateLogBatch 批量创建审计日志
func (s *AuditLogService) CreateLogBatch(ctx context.Context, logs []*AuditLog) error {
	if len(logs) == 0 {
		return nil
	}

	// 设置默认值
	for _, log := range logs {
		if log.EventCategory == "" {
			log.EventCategory = "general"
		}
		if log.EventLevel == "" {
			log.EventLevel = "info"
		}
	}

	return s.db.WithContext(ctx).CreateInBatches(logs, 100).Error
}

// AuditLogQuery 审计日志查询条件
type AuditLogQuery struct {
	TenantID      string              `json:"tenant_id"`
	UserID        string              `json:"user_id"`
	EventTypes    []string            `json:"event_types"`
	EventCategory string              `json:"event_category"`
	EventLevel    string              `json:"event_level"`
	StartTime     *time.Time          `json:"start_time"`
	EndTime       *time.Time          `json:"end_time"`
	IPAddress     string              `json:"ip_address"`
	RequestPath   string              `json:"request_path"`
	Pagination    *types.PaginationRequest  `json:"pagination"`
}

// QueryLogs 查询审计日志
func (s *AuditLogService) QueryLogs(ctx context.Context, query *AuditLogQuery) ([]*AuditLog, *types.PaginationResponse, error) {
	var logs []*AuditLog
	var total int64

	db := s.db.WithContext(ctx).Model(&AuditLog{})

	// 应用过滤条件
	if query.TenantID != "" {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if query.UserID != "" {
		db = db.Where("user_id = ?", query.UserID)
	}
	if len(query.EventTypes) > 0 {
		db = db.Where("event_type IN ?", query.EventTypes)
	}
	if query.EventCategory != "" {
		db = db.Where("event_category = ?", query.EventCategory)
	}
	if query.EventLevel != "" {
		db = db.Where("event_level = ?", query.EventLevel)
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", query.EndTime)
	}
	if query.IPAddress != "" {
		db = db.Where("ip_address = ?", query.IPAddress)
	}
	if query.RequestPath != "" {
		db = db.Where("request_path LIKE ?", "%"+query.RequestPath+"%")
	}

	// 统计总数
	if err := db.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// 应用分页
	pagination := query.Pagination
	if pagination == nil {
		pagination = &types.PaginationRequest{Page: 1, PageSize: 20}
	}

	offset := (pagination.Page - 1) * pagination.PageSize
	db = db.Offset(offset).Limit(pagination.PageSize)

	// 排序：按创建时间倒序
	db = db.Order("created_at DESC")

	// 查询
	if err := db.Find(&logs).Error; err != nil {
		return nil, nil, err
	}

	// 构造分页响应
	paginationResp := &types.PaginationResponse{
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalItems: total,
		TotalPages: (int(total) + pagination.PageSize - 1) / pagination.PageSize,
	}

	return logs, paginationResp, nil
}

// GetLogByID 通过 ID 获取审计日志
func (s *AuditLogService) GetLogByID(ctx context.Context, id string) (*AuditLog, error) {
	var log AuditLog
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&log).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

// GetUserActivitySummary 获取用户活动摘要
func (s *AuditLogService) GetUserActivitySummary(ctx context.Context, userID string, days int) (map[string]int, error) {
	var results []struct {
		EventCategory string `json:"event_category"`
		Count         int    `json:"count"`
	}

	startTime := time.Now().AddDate(0, 0, -days)

	err := s.db.WithContext(ctx).
		Model(&AuditLog{}).
		Select("event_category, COUNT(*) as count").
		Where("user_id = ? AND created_at >= ?", userID, startTime).
		Group("event_category").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	summary := make(map[string]int)
	for _, r := range results {
		summary[r.EventCategory] = r.Count
	}

	return summary, nil
}

// CleanupOldLogs 清理旧日志（保留指定天数）
func (s *AuditLogService) CleanupOldLogs(ctx context.Context, retentionDays int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	result := s.db.WithContext(ctx).
		Where("created_at < ?", cutoffTime).
		Delete(&AuditLog{})

	return result.RowsAffected, result.Error
}

// MarshalMetadata 将 Metadata 序列化为 JSON
func (a *AuditLog) MarshalMetadata() ([]byte, error) {
	if a.Metadata == nil {
		return nil, nil
	}
	return json.Marshal(a.Metadata)
}

// UnmarshalMetadata 反序列化 Metadata
func (a *AuditLog) UnmarshalMetadata(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &a.Metadata)
}
