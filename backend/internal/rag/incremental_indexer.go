package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IncrementalIndexer 增量索引器
type IncrementalIndexer struct {
	db                *gorm.DB
	vectorStore       VectorStore
	embeddingProvider EmbeddingProvider
	chunker           *Chunker
}

// NewIncrementalIndexer 创建增量索引器
func NewIncrementalIndexer(
	db *gorm.DB,
	vectorStore VectorStore,
	embeddingProvider EmbeddingProvider,
	chunker *Chunker,
) *IncrementalIndexer {
	return &IncrementalIndexer{
		db:                db,
		vectorStore:       vectorStore,
		embeddingProvider: embeddingProvider,
		chunker:           chunker,
	}
}

// DocumentChange 文档变更类型
type DocumentChange struct {
	DocumentID      string
	KnowledgeBaseID string
	TenantID        string
	ChangeType      string // content_update, metadata_update, delete
	OldContentHash  string
	NewContentHash  string
	NewContent      string
	UpdatedAt       time.Time
}

// ChunkChange 分块变更
type ChunkChange struct {
	ChunkID    string
	ChangeType string // add, update, delete
	OldHash    string
	NewHash    string
	Content    string
}

// UpdateDocument 增量更新文档
func (i *IncrementalIndexer) UpdateDocument(ctx context.Context, change DocumentChange) error {
	switch change.ChangeType {
	case "content_update":
		return i.handleContentUpdate(ctx, change)
	case "metadata_update":
		return i.handleMetadataUpdate(ctx, change)
	case "delete":
		return i.handleDelete(ctx, change)
	default:
		return fmt.Errorf("unknown change type: %s", change.ChangeType)
	}
}

// handleContentUpdate 处理内容更新
func (i *IncrementalIndexer) handleContentUpdate(ctx context.Context, change DocumentChange) error {
	// 1. 获取现有分块
	existingChunks, err := i.getExistingChunks(ctx, change.DocumentID)
	if err != nil {
		return fmt.Errorf("get existing chunks: %w", err)
	}

	// 2. 生成新分块
	newChunks, err := i.chunker.ChunkDocument(change.NewContent)
	if err != nil {
		return fmt.Errorf("chunk document: %w", err)
	}

	// 3. 比较差异
	chunkChanges := i.diffChunks(existingChunks, newChunks)

	// 4. 处理删除的分块
	var chunksToDelete []string
	for _, cc := range chunkChanges {
		if cc.ChangeType == "delete" {
			chunksToDelete = append(chunksToDelete, cc.ChunkID)
		}
	}
	if len(chunksToDelete) > 0 {
		if err := i.vectorStore.DeleteVectors(ctx, chunksToDelete); err != nil {
			return fmt.Errorf("delete vectors: %w", err)
		}
	}

	// 5. 处理新增和更新的分块
	var textsToEmbed []string
	var chunkInfos []*ChunkChange
	for _, cc := range chunkChanges {
		if cc.ChangeType == "add" || cc.ChangeType == "update" {
			textsToEmbed = append(textsToEmbed, cc.Content)
			chunkInfos = append(chunkInfos, &cc)
		}
	}

	if len(textsToEmbed) > 0 {
		// 批量向量化
		embeddings, err := i.embeddingProvider.EmbedBatch(ctx, textsToEmbed)
		if err != nil {
			return fmt.Errorf("embed batch: %w", err)
		}

		// 准备向量数据
		vectors := make([]*Vector, len(embeddings))
		for idx, embedding := range embeddings {
			cc := chunkInfos[idx]
			chunkID := cc.ChunkID
			if cc.ChangeType == "add" {
				chunkID = uuid.New().String()
			}

			vectors[idx] = &Vector{
				ChunkID:           chunkID,
				KnowledgeBaseID:   change.KnowledgeBaseID,
				DocumentID:        change.DocumentID,
				TenantID:          change.TenantID,
				Content:           cc.Content,
				ContentHash:       cc.NewHash,
				Embedding:         embedding,
				EmbeddingModel:    i.embeddingProvider.GetModel(),
				EmbeddingProvider: i.embeddingProvider.GetProviderName(),
			}
		}

		// 更新的分块需要先删除再添加
		var updateIDs []string
		for _, cc := range chunkInfos {
			if cc.ChangeType == "update" {
				updateIDs = append(updateIDs, cc.ChunkID)
			}
		}
		if len(updateIDs) > 0 {
			if err := i.vectorStore.DeleteVectors(ctx, updateIDs); err != nil {
				return fmt.Errorf("delete updated vectors: %w", err)
			}
		}

		// 添加新向量
		if err := i.vectorStore.AddVectors(ctx, vectors); err != nil {
			return fmt.Errorf("add vectors: %w", err)
		}
	}

	// 6. 更新文档状态
	return i.updateDocumentStatus(ctx, change.DocumentID, change.NewContentHash, len(newChunks))
}

// handleMetadataUpdate 处理元数据更新
func (i *IncrementalIndexer) handleMetadataUpdate(ctx context.Context, change DocumentChange) error {
	// 元数据更新不需要重新向量化
	// 只需要更新数据库记录
	return i.db.WithContext(ctx).
		Model(&KnowledgeDocument{}).
		Where("id = ?", change.DocumentID).
		Update("updated_at", time.Now()).Error
}

// handleDelete 处理删除
func (i *IncrementalIndexer) handleDelete(ctx context.Context, change DocumentChange) error {
	// 删除所有向量
	return i.vectorStore.DeleteByDocument(ctx, change.KnowledgeBaseID, change.DocumentID)
}

// getExistingChunks 获取现有分块
func (i *IncrementalIndexer) getExistingChunks(ctx context.Context, documentID string) ([]ChunkInfo, error) {
	var chunks []ChunkInfo
	err := i.db.WithContext(ctx).
		Table("knowledge_vectors").
		Select("chunk_id, content, content_hash, chunk_index").
		Where("document_id = ?", documentID).
		Order("chunk_index").
		Scan(&chunks).Error
	return chunks, err
}

// ChunkInfo 分块信息
type ChunkInfo struct {
	ChunkID     string `gorm:"column:chunk_id"`
	Content     string `gorm:"column:content"`
	ContentHash string `gorm:"column:content_hash"`
	ChunkIndex  int    `gorm:"column:chunk_index"`
}

// diffChunks 比较分块差异
func (i *IncrementalIndexer) diffChunks(existing []ChunkInfo, newChunks []*ChunkResult) []ChunkChange {
	var changes []ChunkChange

	// 建立现有分块的哈希映射
	existingByHash := make(map[string]ChunkInfo)
	existingByIndex := make(map[int]ChunkInfo)
	for _, chunk := range existing {
		existingByHash[chunk.ContentHash] = chunk
		existingByIndex[chunk.ChunkIndex] = chunk
	}

	// 建立新分块的哈希映射
	newByHash := make(map[string]*ChunkResult)
	for _, chunk := range newChunks {
		newByHash[chunk.ContentHash] = chunk
	}

	// 检测删除的分块 (存在于旧的但不存在于新的)
	for _, old := range existing {
		if _, exists := newByHash[old.ContentHash]; !exists {
			changes = append(changes, ChunkChange{
				ChunkID:    old.ChunkID,
				ChangeType: "delete",
				OldHash:    old.ContentHash,
			})
		}
	}

	// 检测新增和更新的分块
	for _, newChunk := range newChunks {
		if _, exists := existingByHash[newChunk.ContentHash]; exists {
			// 内容相同，无需更新
			continue
		}

		// 检查是否是同位置的更新
		if old, exists := existingByIndex[newChunk.ChunkIndex]; exists {
			// 同位置但内容不同 -> 更新
			changes = append(changes, ChunkChange{
				ChunkID:    old.ChunkID,
				ChangeType: "update",
				OldHash:    old.ContentHash,
				NewHash:    newChunk.ContentHash,
				Content:    newChunk.Content,
			})
		} else {
			// 新增的分块
			changes = append(changes, ChunkChange{
				ChangeType: "add",
				NewHash:    newChunk.ContentHash,
				Content:    newChunk.Content,
			})
		}
	}

	return changes
}

// updateDocumentStatus 更新文档状态
func (i *IncrementalIndexer) updateDocumentStatus(ctx context.Context, documentID, contentHash string, chunkCount int) error {
	return i.db.WithContext(ctx).
		Model(&KnowledgeDocument{}).
		Where("id = ?", documentID).
		Updates(map[string]interface{}{
			"file_hash":  contentHash,
			"updated_at": time.Now(),
		}).Error
}

// ComputeContentHash 计算内容哈希
func ComputeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// SyncKnowledgeBase 同步整个知识库 (检测变更并增量更新)
func (i *IncrementalIndexer) SyncKnowledgeBase(ctx context.Context, kbID, tenantID string) (*SyncResult, error) {
	result := &SyncResult{
		StartTime: time.Now(),
	}

	// 获取所有文档
	var docs []KnowledgeDocument
	err := i.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND tenant_id = ? AND deleted_at IS NULL", kbID, tenantID).
		Find(&docs).Error
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	result.TotalDocuments = len(docs)

	// 检查每个文档
	for _, doc := range docs {
		// 计算当前内容哈希
		currentHash := ComputeContentHash(doc.Content)

		// 比较哈希 (使用 FileHash 字段存储内容哈希)
		if doc.FileHash != currentHash {
			// 内容已变更，需要重新索引
			change := DocumentChange{
				DocumentID:      doc.ID,
				KnowledgeBaseID: kbID,
				TenantID:        tenantID,
				ChangeType:      "content_update",
				OldContentHash:  doc.FileHash,
				NewContentHash:  currentHash,
				NewContent:      doc.Content,
				UpdatedAt:       time.Now(),
			}

			if err := i.UpdateDocument(ctx, change); err != nil {
				result.FailedDocuments++
				result.Errors = append(result.Errors, fmt.Sprintf("doc %s: %v", doc.ID, err))
			} else {
				result.UpdatedDocuments++
			}
		} else {
			result.UnchangedDocuments++
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// SyncResult 同步结果
type SyncResult struct {
	TotalDocuments     int           `json:"totalDocuments"`
	UpdatedDocuments   int           `json:"updatedDocuments"`
	UnchangedDocuments int           `json:"unchangedDocuments"`
	FailedDocuments    int           `json:"failedDocuments"`
	Errors             []string      `json:"errors,omitempty"`
	StartTime          time.Time     `json:"startTime"`
	EndTime            time.Time     `json:"endTime"`
	Duration           time.Duration `json:"duration"`
}
