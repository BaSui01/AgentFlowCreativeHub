package models

import (
	"context"
	"time"
)

// ============================================================================
// ModelService 接口定义
// ============================================================================

// ModelServiceInterface AI模型管理服务接口
type ModelServiceInterface interface {
	// ListModels 查询模型列表
	ListModels(ctx context.Context, req *ListModelsRequest) (*ListModelsResponse, error)

	// GetModel 查询单个模型
	GetModel(ctx context.Context, tenantID, modelID string) (*Model, error)

	// CreateModel 创建模型配置
	CreateModel(ctx context.Context, req *CreateModelRequest) (*Model, error)

	// UpdateModel 更新模型配置
	UpdateModel(ctx context.Context, tenantID, modelID string, req *UpdateModelRequest) (*Model, error)

	// DeleteModel 软删除模型
	DeleteModel(ctx context.Context, tenantID, modelID, operatorID string) error

	// SeedDefaultModels 初始化预置模型
	SeedDefaultModels(ctx context.Context, tenantID string) error

	// GetModelCallStats 获取模型调用统计
	GetModelCallStats(ctx context.Context, tenantID, modelID string, startTime, endTime time.Time) (map[string]any, error)
}

// ============================================================================
// ModelCredentialService 接口定义
// ============================================================================

// ModelCredentialServiceInterface 模型凭证管理服务接口
type ModelCredentialServiceInterface interface {
	// CreateCredential 创建模型凭证
	CreateCredential(ctx context.Context, req *CreateModelCredentialRequest) (*ModelCredential, error)

	// ListCredentials 查询租户的所有模型凭证
	ListCredentials(ctx context.Context, req *ListCredentialsRequest) ([]*ModelCredential, error)

	// DeleteCredential 删除凭证
	DeleteCredential(ctx context.Context, tenantID, credentialID string) error

	// ResolveCredential 解密凭证
	ResolveCredential(ctx context.Context, tenantID, credentialID string) (string, error)
}

// ============================================================================
// ModelDiscoveryService 接口定义
// ============================================================================

// ModelDiscoveryServiceInterface 模型发现服务接口
type ModelDiscoveryServiceInterface interface {
	// SyncModelsFromProvider 同步提供商的最新模型信息
	SyncModelsFromProvider(ctx context.Context, tenantID, provider string) (int, error)

	// AutoDiscoverModels 自动发现所有支持的提供商的模型
	AutoDiscoverModels(ctx context.Context, tenantID string) (map[string]int, error)

	// StartSyncScheduler 启动定时同步调度器
	StartSyncScheduler(ctx context.Context)
}

// ============================================================================
// SessionService 接口定义
// ============================================================================

// SessionServiceInterface 会话管理服务接口
type SessionServiceInterface interface {
	// CreateSession 创建会话（直接使用 Session 对象）
	CreateSession(ctx context.Context, session *Session) error

	// GetSession 查询会话
	GetSession(ctx context.Context, tenantID, sessionID string) (*Session, error)

	// GetSessionByRefreshToken 通过刷新令牌获取会话
	GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error)

	// ListSessions 查询会话列表
	ListSessions(ctx context.Context, tenantID, userID string, page, pageSize int) ([]*Session, int64, error)

	// UpdateSession 更新会话
	UpdateSession(ctx context.Context, tenantID, sessionID string, req *UpdateSessionRequest) (*Session, error)

	// DeleteSession 删除会话
	DeleteSession(ctx context.Context, tenantID, sessionID string) error

	// RotateRefreshToken 更新刷新令牌
	RotateRefreshToken(ctx context.Context, sessionID, refreshToken string, expiresAt time.Time) error

	// RevokeSessionByRefreshToken 通过刷新令牌撤销会话
	RevokeSessionByRefreshToken(ctx context.Context, refreshToken string) error

	// AddMessage 添加消息到会话
	AddMessage(ctx context.Context, sessionID string, message *Message) error

	// GetMessages 获取会话消息列表
	GetMessages(ctx context.Context, sessionID string, limit int) ([]*Message, error)
}

// ============================================================================
// AuditLogService 接口定义
// ============================================================================

// AuditLogServiceInterface 审计日志服务接口
type AuditLogServiceInterface interface {
	// CreateLog 创建审计日志
	CreateLog(ctx context.Context, log *AuditLog) error

	// ListLogs 查询审计日志列表
	ListLogs(ctx context.Context, req *ListAuditLogsRequest) ([]*AuditLog, int64, error)

	// GetLog 查询单条审计日志
	GetLog(ctx context.Context, tenantID, logID string) (*AuditLog, error)

	// GetUserActions 获取用户操作历史
	GetUserActions(ctx context.Context, tenantID, userID string, limit int) ([]*AuditLog, error)

	// GetResourceHistory 获取资源变更历史
	GetResourceHistory(ctx context.Context, tenantID, resource, resourceID string, limit int) ([]*AuditLog, error)
}

// ============================================================================
// KnowledgeBaseService 接口定义
// ============================================================================

// KnowledgeBaseServiceInterface 知识库管理服务接口
type KnowledgeBaseServiceInterface interface {
	// CreateKnowledgeBase 创建知识库
	CreateKnowledgeBase(ctx context.Context, req *CreateKnowledgeBaseRequest) (*KnowledgeBase, error)

	// GetKnowledgeBase 查询知识库
	GetKnowledgeBase(ctx context.Context, tenantID, kbID string) (*KnowledgeBase, error)

	// ListKnowledgeBases 查询知识库列表
	ListKnowledgeBases(ctx context.Context, tenantID string, page, pageSize int) ([]*KnowledgeBase, int64, error)

	// UpdateKnowledgeBase 更新知识库
	UpdateKnowledgeBase(ctx context.Context, tenantID, kbID string, req *UpdateKnowledgeBaseRequest) (*KnowledgeBase, error)

	// DeleteKnowledgeBase 删除知识库
	DeleteKnowledgeBase(ctx context.Context, tenantID, kbID, operatorID string) error

	// UpdateKBStats 更新知识库统计信息
	UpdateKBStats(ctx context.Context, kbID string, documentCount int64) error
}

// ============================================================================
// DocumentService 接口定义
// ============================================================================

// DocumentServiceInterface 文档管理服务接口
type DocumentServiceInterface interface {
	// CreateDocument 创建文档
	CreateDocument(ctx context.Context, req *CreateDocumentRequest) (*Document, error)

	// GetDocument 查询文档
	GetDocument(ctx context.Context, tenantID, docID string) (*Document, error)

	// ListDocuments 查询文档列表
	ListDocuments(ctx context.Context, tenantID, kbID string, page, pageSize int) ([]*Document, int64, error)

	// UpdateDocument 更新文档
	UpdateDocument(ctx context.Context, tenantID, docID string, req *UpdateDocumentRequest) (*Document, error)

	// DeleteDocument 删除文档
	DeleteDocument(ctx context.Context, tenantID, docID, operatorID string) error

	// GetDocumentChunks 获取文档分块列表
	GetDocumentChunks(ctx context.Context, docID string) ([]*DocumentChunk, error)

	// CreateChunk 创建文档分块
	CreateChunk(ctx context.Context, chunk *DocumentChunk) error

	// UpdateChunkEmbedding 更新分块的向量嵌入
	UpdateChunkEmbedding(ctx context.Context, chunkID string, embedding []float32) error

	// SearchChunks 向量检索文档分块
	SearchChunks(ctx context.Context, tenantID, kbID string, queryEmbedding []float32, topK int) ([]*DocumentChunk, error)
}
