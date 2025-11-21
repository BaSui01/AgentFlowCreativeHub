package models

import (
	"backend/pkg/types"
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// KnowledgeBase 知识库
type KnowledgeBase struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID    string    `gorm:"type:uuid;not null;index:idx_kb_tenant" json:"tenant_id"`
	Name        string    `gorm:"type:varchar(200);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Type        string    `gorm:"type:varchar(50);not null" json:"type"` // document, url, api, database
	Status      string    `gorm:"type:varchar(50);not null;default:'active'" json:"status"` // active, inactive
	Config      types.JSONMap   `gorm:"type:jsonb" json:"config"` // 配置信息（如分块大小、重叠等）
	Metadata    types.JSONMap   `gorm:"type:jsonb" json:"metadata"` // 元数据
	DocCount    int       `gorm:"type:int;default:0" json:"doc_count"` // 文档数量
	ChunkCount  int       `gorm:"type:int;default:0" json:"chunk_count"` // 分块数量
	CreatedBy   string    `gorm:"type:uuid" json:"created_by"`
	UpdatedBy   string    `gorm:"type:uuid" json:"updated_by"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null" json:"updated_at"`
}

// BeforeCreate GORM 钩子：创建前设置 ID
func (kb *KnowledgeBase) BeforeCreate(tx *gorm.DB) error {
	if kb.ID == "" {
		kb.ID = uuid.New().String()
	}
	if kb.CreatedAt.IsZero() {
		kb.CreatedAt = time.Now()
	}
	if kb.UpdatedAt.IsZero() {
		kb.UpdatedAt = time.Now()
	}
	return nil
}

// BeforeUpdate GORM 钩子：更新前设置时间
func (kb *KnowledgeBase) BeforeUpdate(tx *gorm.DB) error {
	kb.UpdatedAt = time.Now()
	return nil
}

// TableName 指定表名
func (KnowledgeBase) TableName() string {
	return "knowledge_bases"
}

// KnowledgeBaseService 知识库服务
type KnowledgeBaseService struct {
	db *gorm.DB
}

// NewKnowledgeBaseService 创建知识库服务
func NewKnowledgeBaseService(db *gorm.DB) *KnowledgeBaseService {
	return &KnowledgeBaseService{db: db}
}

// CreateKnowledgeBase 创建知识库
func (s *KnowledgeBaseService) CreateKnowledgeBase(ctx context.Context, kb *KnowledgeBase) error {
	return s.db.WithContext(ctx).Create(kb).Error
}

// GetKnowledgeBase 获取知识库
func (s *KnowledgeBaseService) GetKnowledgeBase(ctx context.Context, id string) (*KnowledgeBase, error) {
	var kb KnowledgeBase
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&kb).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &kb, nil
}

// ListKnowledgeBases 列出知识库
func (s *KnowledgeBaseService) ListKnowledgeBases(ctx context.Context, tenantID string, pagination *types.PaginationRequest) ([]*KnowledgeBase, *types.PaginationResponse, error) {
	var kbs []*KnowledgeBase
	var total int64

	query := s.db.WithContext(ctx).Model(&KnowledgeBase{}).Where("tenant_id = ?", tenantID)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// 应用分页
	if pagination == nil {
		pagination = &types.PaginationRequest{Page: 1, PageSize: 20}
	}
	offset := (pagination.Page - 1) * pagination.PageSize
	query = query.Offset(offset).Limit(pagination.PageSize)

	// 排序
	query = query.Order("created_at DESC")

	// 查询
	if err := query.Find(&kbs).Error; err != nil {
		return nil, nil, err
	}

	paginationResp := &types.PaginationResponse{
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalItems: total,
		TotalPages: (int(total) + pagination.PageSize - 1) / pagination.PageSize,
	}

	return kbs, paginationResp, nil
}

// UpdateKnowledgeBase 更新知识库
func (s *KnowledgeBaseService) UpdateKnowledgeBase(ctx context.Context, kb *KnowledgeBase) error {
	return s.db.WithContext(ctx).Save(kb).Error
}

// DeleteKnowledgeBase 删除知识库
func (s *KnowledgeBaseService) DeleteKnowledgeBase(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&KnowledgeBase{}).Error
}

// UpdateStats 更新知识库统计信息
func (s *KnowledgeBaseService) UpdateStats(ctx context.Context, kbID string) error {
	var docCount, chunkCount int64

	// 统计文档数量
	s.db.WithContext(ctx).Model(&Document{}).Where("knowledge_base_id = ?", kbID).Count(&docCount)

	// 统计分块数量
	s.db.WithContext(ctx).Model(&DocumentChunk{}).Where("knowledge_base_id = ?", kbID).Count(&chunkCount)

	return s.db.WithContext(ctx).Model(&KnowledgeBase{}).Where("id = ?", kbID).Updates(map[string]any{
		"doc_count":   docCount,
		"chunk_count": chunkCount,
		"updated_at":  time.Now(),
	}).Error
}

