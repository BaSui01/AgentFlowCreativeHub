package models

import (
	"backend/pkg/types"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

// Document 文档
type Document struct {
	ID              string    `gorm:"type:uuid;primaryKey" json:"id"`
	KnowledgeBaseID string    `gorm:"type:uuid;not null;index:idx_doc_kb" json:"knowledge_base_id"`
	TenantID        string    `gorm:"type:uuid;not null;index:idx_doc_tenant" json:"tenant_id"`
	Title           string    `gorm:"type:varchar(500);not null" json:"title"`
	Content         string    `gorm:"type:text;not null" json:"content"`
	ContentType     string    `gorm:"type:varchar(50);not null" json:"content_type"` // text/plain, text/markdown, application/pdf, etc.
	Source          string    `gorm:"type:varchar(500)" json:"source"` // 来源（URL、文件路径等）
	SourceType      string    `gorm:"type:varchar(50)" json:"source_type"` // file, url, manual
	FileSize        int64     `gorm:"type:bigint" json:"file_size"` // 文件大小（字节）
	CharCount       int       `gorm:"type:int" json:"char_count"` // 字符数
	WordCount       int       `gorm:"type:int" json:"word_count"` // 词数
	ChunkCount      int       `gorm:"type:int;default:0" json:"chunk_count"` // 分块数量
	Status          string    `gorm:"type:varchar(50);not null;default:'pending'" json:"status"` // pending, processing, completed, failed
	ErrorMessage    string    `gorm:"type:text" json:"error_message"` // 错误信息
	Metadata        types.JSONMap   `gorm:"type:jsonb" json:"metadata"` // 元数据（作者、创建时间等）
	CreatedBy       string    `gorm:"type:uuid" json:"created_by"`
	CreatedAt       time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null" json:"updated_at"`
}

// BeforeCreate GORM 钩子：创建前设置 ID
func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = time.Now()
	}
	return nil
}

// BeforeUpdate GORM 钩子：更新前设置时间
func (d *Document) BeforeUpdate(tx *gorm.DB) error {
	d.UpdatedAt = time.Now()
	return nil
}

// TableName 指定表名
func (Document) TableName() string {
	return "documents"
}

// DocumentChunk 文档分块
type DocumentChunk struct {
	ID              string           `gorm:"type:uuid;primaryKey" json:"id"`
	DocumentID      string           `gorm:"type:uuid;not null;index:idx_chunk_doc" json:"document_id"`
	KnowledgeBaseID string           `gorm:"type:uuid;not null;index:idx_chunk_kb" json:"knowledge_base_id"`
	TenantID        string           `gorm:"type:uuid;not null;index:idx_chunk_tenant" json:"tenant_id"`
	Content         string           `gorm:"type:text;not null" json:"content"`
	ChunkIndex      int              `gorm:"type:int;not null" json:"chunk_index"` // 分块序号（从 0 开始）
	StartPos        int              `gorm:"type:int" json:"start_pos"` // 在原文档中的起始位置
	EndPos          int              `gorm:"type:int" json:"end_pos"` // 在原文档中的结束位置
	TokenCount      int              `gorm:"type:int" json:"token_count"` // Token 数量
	Embedding       pgvector.Vector  `gorm:"type:vector(1536)" json:"-"` // 向量嵌入（OpenAI text-embedding-ada-002: 1536维）
	Metadata        types.JSONMap          `gorm:"type:jsonb" json:"metadata"` // 元数据
	CreatedAt       time.Time        `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time        `gorm:"not null" json:"updated_at"`
}

// BeforeCreate GORM 钩子：创建前设置 ID
func (dc *DocumentChunk) BeforeCreate(tx *gorm.DB) error {
	if dc.ID == "" {
		dc.ID = uuid.New().String()
	}
	if dc.CreatedAt.IsZero() {
		dc.CreatedAt = time.Now()
	}
	if dc.UpdatedAt.IsZero() {
		dc.UpdatedAt = time.Now()
	}
	return nil
}

// BeforeUpdate GORM 钩子：更新前设置时间
func (dc *DocumentChunk) BeforeUpdate(tx *gorm.DB) error {
	dc.UpdatedAt = time.Now()
	return nil
}

// TableName 指定表名
func (DocumentChunk) TableName() string {
	return "document_chunks"
}

// DocumentService 文档服务
type DocumentService struct {
	db *gorm.DB
}

// NewDocumentService 创建文档服务
func NewDocumentService(db *gorm.DB) *DocumentService {
	return &DocumentService{db: db}
}

// CreateDocument 创建文档
func (s *DocumentService) CreateDocument(ctx context.Context, doc *Document) error {
	return s.db.WithContext(ctx).Create(doc).Error
}

// GetDocument 获取文档
func (s *DocumentService) GetDocument(ctx context.Context, id string) (*Document, error) {
	var doc Document
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// ListDocuments 列出文档
func (s *DocumentService) ListDocuments(ctx context.Context, kbID string, pagination *types.PaginationRequest) ([]*Document, *types.PaginationResponse, error) {
	var docs []*Document
	var total int64

	query := s.db.WithContext(ctx).Model(&Document{}).Where("knowledge_base_id = ?", kbID)

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
	if err := query.Find(&docs).Error; err != nil {
		return nil, nil, err
	}

	paginationResp := &types.PaginationResponse{
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalItems: total,
		TotalPages: (int(total) + pagination.PageSize - 1) / pagination.PageSize,
	}

	return docs, paginationResp, nil
}

// UpdateDocument 更新文档
func (s *DocumentService) UpdateDocument(ctx context.Context, doc *Document) error {
	return s.db.WithContext(ctx).Save(doc).Error
}

// DeleteDocument 删除文档（同时删除所有分块）
func (s *DocumentService) DeleteDocument(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除文档分块
		if err := tx.Where("document_id = ?", id).Delete(&DocumentChunk{}).Error; err != nil {
			return err
		}

		// 删除文档
		return tx.Where("id = ?", id).Delete(&Document{}).Error
	})
}

// CreateChunk 创建文档分块
func (s *DocumentService) CreateChunk(ctx context.Context, chunk *DocumentChunk) error {
	return s.db.WithContext(ctx).Create(chunk).Error
}

// CreateChunkBatch 批量创建文档分块
func (s *DocumentService) CreateChunkBatch(ctx context.Context, chunks []*DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).CreateInBatches(chunks, 100).Error
}

// GetChunk 获取文档分块
func (s *DocumentService) GetChunk(ctx context.Context, id string) (*DocumentChunk, error) {
	var chunk DocumentChunk
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&chunk).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &chunk, nil
}

// ListChunks 列出文档分块
func (s *DocumentService) ListChunks(ctx context.Context, docID string) ([]*DocumentChunk, error) {
	var chunks []*DocumentChunk
	err := s.db.WithContext(ctx).
		Where("document_id = ?", docID).
		Order("chunk_index ASC").
		Find(&chunks).Error
	return chunks, err
}

// UpdateChunkEmbedding 更新分块的向量嵌入
func (s *DocumentService) UpdateChunkEmbedding(ctx context.Context, chunkID string, embedding []float32) error {
	return s.db.WithContext(ctx).Model(&DocumentChunk{}).
		Where("id = ?", chunkID).
		Update("embedding", pgvector.NewVector(embedding)).Error
}

// SearchSimilarChunks 语义检索相似分块
func (s *DocumentService) SearchSimilarChunks(ctx context.Context, kbID string, queryEmbedding []float32, limit int) ([]*DocumentChunk, []float64, error) {
	var chunks []*DocumentChunk
	var distances []float64

	// 使用 pgvector 的余弦相似度检索
	// 1 - (embedding <=> query_embedding) 表示相似度（越接近 1 越相似）
	type Result struct {
		Chunk    DocumentChunk
		Distance float64
	}

	var results []Result
	
	err := s.db.WithContext(ctx).
		Raw(`
			SELECT 
				*, 
				1 - (embedding <=> ?) as distance
			FROM document_chunks
			WHERE knowledge_base_id = ?
			ORDER BY embedding <=> ?
			LIMIT ?
		`, pgvector.NewVector(queryEmbedding), kbID, pgvector.NewVector(queryEmbedding), limit).
		Scan(&results).Error

	if err != nil {
		return nil, nil, err
	}

	for _, r := range results {
		chunks = append(chunks, &r.Chunk)
		distances = append(distances, r.Distance)
	}

	return chunks, distances, nil
}

