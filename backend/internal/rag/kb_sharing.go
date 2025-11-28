package rag

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// KBSharingService 知识库共享服务
type KBSharingService struct {
	store    KBSharingStore
	acl      *AccessControlService
	cache    map[string]*KBShare
	cacheMu  sync.RWMutex
	cacheTTL time.Duration
}

// KBSharingStore 知识库共享存储接口
type KBSharingStore interface {
	// 共享记录
	CreateShare(ctx context.Context, share *KBShare) error
	UpdateShare(ctx context.Context, share *KBShare) error
	DeleteShare(ctx context.Context, shareID string) error
	GetShare(ctx context.Context, shareID string) (*KBShare, error)
	GetShareByToken(ctx context.Context, token string) (*KBShare, error)
	ListSharesByKB(ctx context.Context, kbID string) ([]*KBShare, error)
	ListSharesByTenant(ctx context.Context, tenantID string) ([]*KBShare, error)
	ListReceivedShares(ctx context.Context, tenantID string) ([]*KBShare, error)

	// 共享接受记录
	CreateShareAcceptance(ctx context.Context, acceptance *ShareAcceptance) error
	GetShareAcceptance(ctx context.Context, shareID, tenantID string) (*ShareAcceptance, error)
	ListAcceptancesByShare(ctx context.Context, shareID string) ([]*ShareAcceptance, error)
	DeleteShareAcceptance(ctx context.Context, shareID, tenantID string) error

	// 共享访问日志
	LogShareAccess(ctx context.Context, log *ShareAccessLog) error
	GetShareAccessLogs(ctx context.Context, shareID string, limit int) ([]*ShareAccessLog, error)
}

// KBShare 知识库共享记录
type KBShare struct {
	ID              string          `json:"id" gorm:"primaryKey;type:varchar(64)"`
	KnowledgeBaseID string          `json:"knowledge_base_id" gorm:"type:varchar(64);not null;index"`
	OwnerTenantID   string          `json:"owner_tenant_id" gorm:"type:varchar(64);not null;index"`
	
	// 共享配置
	ShareType       ShareType       `json:"share_type" gorm:"type:varchar(20);not null"`
	Permission      SharePermission `json:"permission" gorm:"type:varchar(20);not null"`
	
	// 共享链接（公开链接模式）
	ShareToken      string          `json:"share_token,omitempty" gorm:"type:varchar(128);uniqueIndex"`
	TokenHash       string          `json:"-" gorm:"type:varchar(64);index"`
	
	// 目标租户（定向共享模式）
	TargetTenantID  string          `json:"target_tenant_id,omitempty" gorm:"type:varchar(64);index"`
	TargetTenantIDs []string        `json:"target_tenant_ids,omitempty" gorm:"type:jsonb;serializer:json"`
	
	// 范围限制
	Scope           *ShareScope     `json:"scope,omitempty" gorm:"type:jsonb;serializer:json"`
	
	// 使用限制
	MaxAccesses     int             `json:"max_accesses,omitempty" gorm:"default:0"`
	CurrentAccesses int             `json:"current_accesses" gorm:"default:0"`
	MaxQueries      int             `json:"max_queries,omitempty" gorm:"default:0"`
	CurrentQueries  int             `json:"current_queries" gorm:"default:0"`
	
	// 有效期
	ExpiresAt       *time.Time      `json:"expires_at,omitempty" gorm:"index"`
	
	// 元信息
	Name            string          `json:"name" gorm:"type:varchar(255)"`
	Description     string          `json:"description,omitempty" gorm:"type:text"`
	CreatedBy       string          `json:"created_by" gorm:"type:varchar(64)"`
	
	// 状态
	Status          ShareStatus     `json:"status" gorm:"type:varchar(20);not null;default:active"`
	
	// 时间戳
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (KBShare) TableName() string {
	return "kb_shares"
}

// ShareType 共享类型
type ShareType string

const (
	ShareTypeLink     ShareType = "link"     // 公开链接
	ShareTypeTenant   ShareType = "tenant"   // 定向租户
	ShareTypePublic   ShareType = "public"   // 完全公开
)

// SharePermission 共享权限
type SharePermission string

const (
	SharePermissionRead      SharePermission = "read"      // 只读（仅查询）
	SharePermissionWrite     SharePermission = "write"     // 可写（添加文档）
	SharePermissionCollaborate SharePermission = "collaborate" // 协作（编辑文档）
)

// ShareStatus 共享状态
type ShareStatus string

const (
	ShareStatusActive   ShareStatus = "active"
	ShareStatusPaused   ShareStatus = "paused"
	ShareStatusExpired  ShareStatus = "expired"
	ShareStatusRevoked  ShareStatus = "revoked"
)

// ShareScope 共享范围
type ShareScope struct {
	Folders     []string `json:"folders,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	DocTypes    []string `json:"doc_types,omitempty"`
	DocIDs      []string `json:"doc_ids,omitempty"`
	ExcludeIDs  []string `json:"exclude_ids,omitempty"`
	MaxResults  int      `json:"max_results,omitempty"`
}

// ShareAcceptance 共享接受记录
type ShareAcceptance struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	ShareID     string    `json:"share_id" gorm:"type:varchar(64);not null;index"`
	TenantID    string    `json:"tenant_id" gorm:"type:varchar(64);not null;index"`
	AcceptedBy  string    `json:"accepted_by" gorm:"type:varchar(64)"`
	Alias       string    `json:"alias,omitempty" gorm:"type:varchar(255)"`
	AcceptedAt  time.Time `json:"accepted_at" gorm:"autoCreateTime"`
}

func (ShareAcceptance) TableName() string {
	return "kb_share_acceptances"
}

// ShareAccessLog 共享访问日志
type ShareAccessLog struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	ShareID     string    `json:"share_id" gorm:"type:varchar(64);not null;index"`
	TenantID    string    `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID      string    `json:"user_id,omitempty" gorm:"type:varchar(64)"`
	Action      string    `json:"action" gorm:"type:varchar(50);not null"`
	IPAddress   string    `json:"ip_address,omitempty" gorm:"type:varchar(45)"`
	UserAgent   string    `json:"user_agent,omitempty" gorm:"type:varchar(500)"`
	QueryText   string    `json:"query_text,omitempty" gorm:"type:text"`
	ResultCount int       `json:"result_count,omitempty"`
	LatencyMs   int       `json:"latency_ms,omitempty"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

func (ShareAccessLog) TableName() string {
	return "kb_share_access_logs"
}

var (
	ErrShareNotFound     = errors.New("share not found")
	ErrShareExpired      = errors.New("share has expired")
	ErrShareRevoked      = errors.New("share has been revoked")
	ErrShareLimitReached = errors.New("share access limit reached")
	ErrShareDenied       = errors.New("share access denied")
	ErrInvalidToken      = errors.New("invalid share token")
	ErrAlreadyAccepted   = errors.New("share already accepted")
)

// NewKBSharingService 创建知识库共享服务
func NewKBSharingService(store KBSharingStore, acl *AccessControlService, cacheTTL time.Duration) *KBSharingService {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &KBSharingService{
		store:    store,
		acl:      acl,
		cache:    make(map[string]*KBShare),
		cacheTTL: cacheTTL,
	}
}

// ============================================================================
// 创建共享
// ============================================================================

// CreateShareRequest 创建共享请求
type CreateShareRequest struct {
	KnowledgeBaseID string          `json:"knowledge_base_id"`
	OwnerTenantID   string          `json:"owner_tenant_id"`
	ShareType       ShareType       `json:"share_type"`
	Permission      SharePermission `json:"permission"`
	TargetTenantID  string          `json:"target_tenant_id,omitempty"`
	TargetTenantIDs []string        `json:"target_tenant_ids,omitempty"`
	Scope           *ShareScope     `json:"scope,omitempty"`
	MaxAccesses     int             `json:"max_accesses,omitempty"`
	MaxQueries      int             `json:"max_queries,omitempty"`
	ExpiresIn       time.Duration   `json:"expires_in,omitempty"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	CreatedBy       string          `json:"created_by"`
}

// CreateShare 创建知识库共享
func (s *KBSharingService) CreateShare(ctx context.Context, req *CreateShareRequest) (*KBShare, error) {
	// 验证权限：只有 owner 或 manage 权限可以创建共享
	if s.acl != nil {
		if err := s.acl.CheckPermission(ctx, req.CreatedBy, req.KnowledgeBaseID, PermissionManage); err != nil {
			return nil, fmt.Errorf("no permission to share: %w", err)
		}
	}

	share := &KBShare{
		ID:              generateID("share"),
		KnowledgeBaseID: req.KnowledgeBaseID,
		OwnerTenantID:   req.OwnerTenantID,
		ShareType:       req.ShareType,
		Permission:      req.Permission,
		TargetTenantID:  req.TargetTenantID,
		TargetTenantIDs: req.TargetTenantIDs,
		Scope:           req.Scope,
		MaxAccesses:     req.MaxAccesses,
		MaxQueries:      req.MaxQueries,
		Name:            req.Name,
		Description:     req.Description,
		CreatedBy:       req.CreatedBy,
		Status:          ShareStatusActive,
	}

	// 设置过期时间
	if req.ExpiresIn > 0 {
		expiresAt := time.Now().Add(req.ExpiresIn)
		share.ExpiresAt = &expiresAt
	}

	// 生成共享链接 Token（仅链接模式）
	if req.ShareType == ShareTypeLink || req.ShareType == ShareTypePublic {
		token, tokenHash := generateShareToken()
		share.ShareToken = token
		share.TokenHash = tokenHash
	}

	if err := s.store.CreateShare(ctx, share); err != nil {
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	return share, nil
}

// ============================================================================
// 共享链接验证
// ============================================================================

// ValidateShareToken 验证共享链接 Token
func (s *KBSharingService) ValidateShareToken(ctx context.Context, token string) (*KBShare, error) {
	// 检查缓存
	s.cacheMu.RLock()
	if share, ok := s.cache[token]; ok {
		s.cacheMu.RUnlock()
		if err := s.validateShare(share); err != nil {
			return nil, err
		}
		return share, nil
	}
	s.cacheMu.RUnlock()

	// 从数据库查询
	share, err := s.store.GetShareByToken(ctx, token)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if share == nil {
		return nil, ErrShareNotFound
	}

	// 验证共享状态
	if err := s.validateShare(share); err != nil {
		return nil, err
	}

	// 缓存
	s.cacheShare(token, share)

	return share, nil
}

// validateShare 验证共享状态
func (s *KBSharingService) validateShare(share *KBShare) error {
	// 检查状态
	switch share.Status {
	case ShareStatusRevoked:
		return ErrShareRevoked
	case ShareStatusExpired:
		return ErrShareExpired
	case ShareStatusPaused:
		return fmt.Errorf("share is paused")
	}

	// 检查过期时间
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now()) {
		return ErrShareExpired
	}

	// 检查访问次数限制
	if share.MaxAccesses > 0 && share.CurrentAccesses >= share.MaxAccesses {
		return ErrShareLimitReached
	}

	return nil
}

// ============================================================================
// 接受共享
// ============================================================================

// AcceptShare 接受共享（将共享的知识库添加到自己的租户）
func (s *KBSharingService) AcceptShare(ctx context.Context, shareID, tenantID, userID, alias string) (*ShareAcceptance, error) {
	share, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if share == nil {
		return nil, ErrShareNotFound
	}

	// 验证共享状态
	if err := s.validateShare(share); err != nil {
		return nil, err
	}

	// 检查是否已接受
	existing, _ := s.store.GetShareAcceptance(ctx, shareID, tenantID)
	if existing != nil {
		return nil, ErrAlreadyAccepted
	}

	// 检查是否有权接受（定向共享模式）
	if share.ShareType == ShareTypeTenant {
		if share.TargetTenantID != "" && share.TargetTenantID != tenantID {
			return nil, ErrShareDenied
		}
		if len(share.TargetTenantIDs) > 0 && !containsString(share.TargetTenantIDs, tenantID) {
			return nil, ErrShareDenied
		}
	}

	acceptance := &ShareAcceptance{
		ID:         generateID("accept"),
		ShareID:    shareID,
		TenantID:   tenantID,
		AcceptedBy: userID,
		Alias:      alias,
	}

	if err := s.store.CreateShareAcceptance(ctx, acceptance); err != nil {
		return nil, fmt.Errorf("failed to accept share: %w", err)
	}

	// 增加访问计数
	share.CurrentAccesses++
	_ = s.store.UpdateShare(ctx, share)

	return acceptance, nil
}

// AcceptShareByToken 通过 Token 接受共享
func (s *KBSharingService) AcceptShareByToken(ctx context.Context, token, tenantID, userID, alias string) (*ShareAcceptance, error) {
	share, err := s.ValidateShareToken(ctx, token)
	if err != nil {
		return nil, err
	}

	return s.AcceptShare(ctx, share.ID, tenantID, userID, alias)
}

// ============================================================================
// 查询共享的知识库
// ============================================================================

// QuerySharedKB 查询共享的知识库
func (s *KBSharingService) QuerySharedKB(ctx context.Context, shareID, tenantID, userID, query string) (*SharedQueryResult, error) {
	share, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if share == nil {
		return nil, ErrShareNotFound
	}

	// 验证共享状态
	if err := s.validateShare(share); err != nil {
		return nil, err
	}

	// 检查查询次数限制
	if share.MaxQueries > 0 && share.CurrentQueries >= share.MaxQueries {
		return nil, fmt.Errorf("query limit reached")
	}

	// 检查访问权限（定向共享需要先接受）
	if share.ShareType == ShareTypeTenant {
		acceptance, _ := s.store.GetShareAcceptance(ctx, shareID, tenantID)
		if acceptance == nil {
			return nil, ErrShareDenied
		}
	}

	start := time.Now()

	// 记录访问日志
	defer func() {
		log := &ShareAccessLog{
			ID:        generateID("log"),
			ShareID:   shareID,
			TenantID:  tenantID,
			UserID:    userID,
			Action:    "query",
			QueryText: query,
			LatencyMs: int(time.Since(start).Milliseconds()),
		}
		_ = s.store.LogShareAccess(ctx, log)
	}()

	// 增加查询计数
	share.CurrentQueries++
	_ = s.store.UpdateShare(ctx, share)

	// 返回查询结果（实际查询逻辑由调用方实现）
	return &SharedQueryResult{
		ShareID:         shareID,
		KnowledgeBaseID: share.KnowledgeBaseID,
		Permission:      share.Permission,
		Scope:           share.Scope,
	}, nil
}

// SharedQueryResult 共享查询结果
type SharedQueryResult struct {
	ShareID         string          `json:"share_id"`
	KnowledgeBaseID string          `json:"knowledge_base_id"`
	Permission      SharePermission `json:"permission"`
	Scope           *ShareScope     `json:"scope,omitempty"`
}

// ============================================================================
// 共享管理
// ============================================================================

// GetShare 获取共享详情
func (s *KBSharingService) GetShare(ctx context.Context, shareID string) (*KBShare, error) {
	return s.store.GetShare(ctx, shareID)
}

// ListKBShares 列出知识库的所有共享
func (s *KBSharingService) ListKBShares(ctx context.Context, kbID, userID string) ([]*KBShare, error) {
	// 验证权限
	if s.acl != nil {
		if err := s.acl.CheckPermission(ctx, userID, kbID, PermissionManage); err != nil {
			return nil, err
		}
	}

	return s.store.ListSharesByKB(ctx, kbID)
}

// ListTenantShares 列出租户创建的所有共享
func (s *KBSharingService) ListTenantShares(ctx context.Context, tenantID string) ([]*KBShare, error) {
	return s.store.ListSharesByTenant(ctx, tenantID)
}

// ListReceivedShares 列出租户接受的所有共享
func (s *KBSharingService) ListReceivedShares(ctx context.Context, tenantID string) ([]*KBShare, error) {
	return s.store.ListReceivedShares(ctx, tenantID)
}

// UpdateShare 更新共享
func (s *KBSharingService) UpdateShare(ctx context.Context, shareID, userID string, updates map[string]any) error {
	share, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		return err
	}
	if share == nil {
		return ErrShareNotFound
	}

	// 验证权限
	if s.acl != nil {
		if err := s.acl.CheckPermission(ctx, userID, share.KnowledgeBaseID, PermissionManage); err != nil {
			return err
		}
	}

	// 更新字段
	if name, ok := updates["name"].(string); ok {
		share.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		share.Description = desc
	}
	if perm, ok := updates["permission"].(string); ok {
		share.Permission = SharePermission(perm)
	}
	if status, ok := updates["status"].(string); ok {
		share.Status = ShareStatus(status)
	}
	if maxAccesses, ok := updates["max_accesses"].(int); ok {
		share.MaxAccesses = maxAccesses
	}
	if maxQueries, ok := updates["max_queries"].(int); ok {
		share.MaxQueries = maxQueries
	}

	if err := s.store.UpdateShare(ctx, share); err != nil {
		return err
	}

	// 清除缓存
	s.invalidateCache(share.ShareToken)

	return nil
}

// RevokeShare 撤销共享
func (s *KBSharingService) RevokeShare(ctx context.Context, shareID, userID string) error {
	share, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		return err
	}
	if share == nil {
		return ErrShareNotFound
	}

	// 验证权限
	if s.acl != nil {
		if err := s.acl.CheckPermission(ctx, userID, share.KnowledgeBaseID, PermissionManage); err != nil {
			return err
		}
	}

	share.Status = ShareStatusRevoked
	if err := s.store.UpdateShare(ctx, share); err != nil {
		return err
	}

	// 清除缓存
	s.invalidateCache(share.ShareToken)

	return nil
}

// DeleteShare 删除共享
func (s *KBSharingService) DeleteShare(ctx context.Context, shareID, userID string) error {
	share, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		return err
	}
	if share == nil {
		return ErrShareNotFound
	}

	// 验证权限
	if s.acl != nil {
		if err := s.acl.CheckPermission(ctx, userID, share.KnowledgeBaseID, PermissionOwner); err != nil {
			return err
		}
	}

	// 清除缓存
	s.invalidateCache(share.ShareToken)

	return s.store.DeleteShare(ctx, shareID)
}

// LeaveShare 离开共享（取消接受）
func (s *KBSharingService) LeaveShare(ctx context.Context, shareID, tenantID string) error {
	return s.store.DeleteShareAcceptance(ctx, shareID, tenantID)
}

// ============================================================================
// 统计
// ============================================================================

// ShareStats 共享统计
type ShareStats struct {
	TotalShares     int `json:"total_shares"`
	ActiveShares    int `json:"active_shares"`
	TotalAccesses   int `json:"total_accesses"`
	TotalQueries    int `json:"total_queries"`
	AcceptedTenants int `json:"accepted_tenants"`
}

// GetShareStats 获取共享统计
func (s *KBSharingService) GetShareStats(ctx context.Context, shareID string) (*ShareStats, error) {
	share, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if share == nil {
		return nil, ErrShareNotFound
	}

	acceptances, _ := s.store.ListAcceptancesByShare(ctx, shareID)

	return &ShareStats{
		TotalAccesses:   share.CurrentAccesses,
		TotalQueries:    share.CurrentQueries,
		AcceptedTenants: len(acceptances),
	}, nil
}

// GetAccessLogs 获取访问日志
func (s *KBSharingService) GetAccessLogs(ctx context.Context, shareID string, limit int) ([]*ShareAccessLog, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.store.GetShareAccessLogs(ctx, shareID, limit)
}

// ============================================================================
// 共享链接生成
// ============================================================================

// GenerateShareLink 生成共享链接
func (s *KBSharingService) GenerateShareLink(baseURL string, share *KBShare) string {
	if share.ShareToken == "" {
		return ""
	}
	return fmt.Sprintf("%s/shared/kb/%s", baseURL, share.ShareToken)
}

// RefreshShareToken 刷新共享 Token
func (s *KBSharingService) RefreshShareToken(ctx context.Context, shareID, userID string) (string, error) {
	share, err := s.store.GetShare(ctx, shareID)
	if err != nil {
		return "", err
	}
	if share == nil {
		return "", ErrShareNotFound
	}

	// 验证权限
	if s.acl != nil {
		if err := s.acl.CheckPermission(ctx, userID, share.KnowledgeBaseID, PermissionManage); err != nil {
			return "", err
		}
	}

	// 清除旧缓存
	s.invalidateCache(share.ShareToken)

	// 生成新 Token
	token, tokenHash := generateShareToken()
	share.ShareToken = token
	share.TokenHash = tokenHash

	if err := s.store.UpdateShare(ctx, share); err != nil {
		return "", err
	}

	return token, nil
}

// ============================================================================
// 辅助函数
// ============================================================================

func (s *KBSharingService) cacheShare(token string, share *KBShare) {
	s.cacheMu.Lock()
	s.cache[token] = share
	s.cacheMu.Unlock()

	// 设置缓存过期
	go func() {
		time.Sleep(s.cacheTTL)
		s.cacheMu.Lock()
		delete(s.cache, token)
		s.cacheMu.Unlock()
	}()
}

func (s *KBSharingService) invalidateCache(token string) {
	if token == "" {
		return
	}
	s.cacheMu.Lock()
	delete(s.cache, token)
	s.cacheMu.Unlock()
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func generateShareToken() (token, hash string) {
	// 生成 32 字节随机数
	b := make([]byte, 32)
	rand.Read(b)
	token = base64.URLEncoding.EncodeToString(b)

	// 计算哈希用于索引
	h := sha256.Sum256([]byte(token))
	hash = hex.EncodeToString(h[:])

	return token, hash
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
