package rag

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// AccessControlService 知识库访问控制服务
type AccessControlService struct {
	store   AccessControlStore
	cache   map[string]*KBPermission
	cacheMu sync.RWMutex
	cacheTTL time.Duration
}

// AccessControlStore 访问控制存储接口
type AccessControlStore interface {
	GetKBPermissions(ctx context.Context, kbID string) ([]*KBPermission, error)
	SetKBPermission(ctx context.Context, perm *KBPermission) error
	DeleteKBPermission(ctx context.Context, kbID, principalID string) error
	GetUserPermissions(ctx context.Context, userID string) ([]*KBPermission, error)
}

// KBPermission 知识库权限
type KBPermission struct {
	ID            string         `json:"id"`
	KnowledgeBase string         `json:"knowledge_base_id"`
	PrincipalID   string         `json:"principal_id"`   // 用户ID 或 角色ID
	PrincipalType PrincipalType  `json:"principal_type"` // user / role / team
	Permission    PermissionLevel `json:"permission"`
	Scope         PermissionScope `json:"scope,omitempty"` // 细粒度范围
	GrantedBy     string         `json:"granted_by"`
	GrantedAt     time.Time      `json:"granted_at"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
}

// PrincipalType 主体类型
type PrincipalType string

const (
	PrincipalUser PrincipalType = "user"
	PrincipalRole PrincipalType = "role"
	PrincipalTeam PrincipalType = "team"
)

// PermissionLevel 权限级别
type PermissionLevel string

const (
	PermissionNone   PermissionLevel = "none"
	PermissionRead   PermissionLevel = "read"   // 只能查询
	PermissionWrite  PermissionLevel = "write"  // 可以添加文档
	PermissionManage PermissionLevel = "manage" // 可以管理权限
	PermissionOwner  PermissionLevel = "owner"  // 所有权限
)

// PermissionScope 权限范围（细粒度）
type PermissionScope struct {
	Folders    []string `json:"folders,omitempty"`    // 限制访问的文件夹
	Tags       []string `json:"tags,omitempty"`       // 限制访问的标签
	DocTypes   []string `json:"doc_types,omitempty"`  // 限制访问的文档类型
	MaxResults int      `json:"max_results,omitempty"` // 单次查询最大结果数
}

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrKBNotFound       = errors.New("knowledge base not found")
)

// NewAccessControlService 创建访问控制服务
func NewAccessControlService(store AccessControlStore, cacheTTL time.Duration) *AccessControlService {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &AccessControlService{
		store:    store,
		cache:    make(map[string]*KBPermission),
		cacheTTL: cacheTTL,
	}
}

// CheckPermission 检查权限
func (s *AccessControlService) CheckPermission(ctx context.Context, userID, kbID string, required PermissionLevel) error {
	perm, err := s.GetEffectivePermission(ctx, userID, kbID)
	if err != nil {
		return err
	}

	if !s.hasPermission(perm.Permission, required) {
		return fmt.Errorf("%w: need %s, have %s", ErrPermissionDenied, required, perm.Permission)
	}

	return nil
}

// GetEffectivePermission 获取有效权限（考虑角色继承）
func (s *AccessControlService) GetEffectivePermission(ctx context.Context, userID, kbID string) (*KBPermission, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%s", userID, kbID)
	s.cacheMu.RLock()
	if perm, ok := s.cache[cacheKey]; ok {
		s.cacheMu.RUnlock()
		return perm, nil
	}
	s.cacheMu.RUnlock()

	// 获取所有权限
	perms, err := s.store.GetKBPermissions(ctx, kbID)
	if err != nil {
		return nil, err
	}

	// 找到最高权限
	var effectivePerm *KBPermission
	for _, perm := range perms {
		// 检查是否过期
		if perm.ExpiresAt != nil && perm.ExpiresAt.Before(time.Now()) {
			continue
		}

		// 用户直接权限
		if perm.PrincipalType == PrincipalUser && perm.PrincipalID == userID {
			if effectivePerm == nil || s.comparePermission(perm.Permission, effectivePerm.Permission) > 0 {
				effectivePerm = perm
			}
		}

		// TODO: 检查用户角色权限
		// TODO: 检查团队权限
	}

	if effectivePerm == nil {
		effectivePerm = &KBPermission{
			KnowledgeBase: kbID,
			PrincipalID:   userID,
			Permission:    PermissionNone,
		}
	}

	// 缓存结果
	s.cacheMu.Lock()
	s.cache[cacheKey] = effectivePerm
	s.cacheMu.Unlock()

	// 设置缓存过期
	go func() {
		time.Sleep(s.cacheTTL)
		s.cacheMu.Lock()
		delete(s.cache, cacheKey)
		s.cacheMu.Unlock()
	}()

	return effectivePerm, nil
}

// GrantPermission 授予权限
func (s *AccessControlService) GrantPermission(ctx context.Context, granterID string, perm *KBPermission) error {
	// 检查授权者权限
	granterPerm, err := s.GetEffectivePermission(ctx, granterID, perm.KnowledgeBase)
	if err != nil {
		return err
	}

	// 只有 manage 或 owner 可以授权
	if !s.hasPermission(granterPerm.Permission, PermissionManage) {
		return fmt.Errorf("%w: need manage permission to grant", ErrPermissionDenied)
	}

	// 不能授予比自己高的权限
	if s.comparePermission(perm.Permission, granterPerm.Permission) > 0 {
		return fmt.Errorf("%w: cannot grant higher permission than own", ErrPermissionDenied)
	}

	perm.GrantedBy = granterID
	perm.GrantedAt = time.Now()
	if perm.ID == "" {
		perm.ID = fmt.Sprintf("perm_%d", time.Now().UnixNano())
	}

	err = s.store.SetKBPermission(ctx, perm)
	if err != nil {
		return err
	}

	// 清除缓存
	s.invalidateCache(perm.PrincipalID, perm.KnowledgeBase)

	return nil
}

// RevokePermission 撤销权限
func (s *AccessControlService) RevokePermission(ctx context.Context, revokerID, kbID, principalID string) error {
	// 检查撤销者权限
	revokerPerm, err := s.GetEffectivePermission(ctx, revokerID, kbID)
	if err != nil {
		return err
	}

	if !s.hasPermission(revokerPerm.Permission, PermissionManage) {
		return fmt.Errorf("%w: need manage permission to revoke", ErrPermissionDenied)
	}

	err = s.store.DeleteKBPermission(ctx, kbID, principalID)
	if err != nil {
		return err
	}

	s.invalidateCache(principalID, kbID)

	return nil
}

// ListKBPermissions 列出知识库的所有权限
func (s *AccessControlService) ListKBPermissions(ctx context.Context, userID, kbID string) ([]*KBPermission, error) {
	// 检查查看权限
	err := s.CheckPermission(ctx, userID, kbID, PermissionManage)
	if err != nil {
		return nil, err
	}

	return s.store.GetKBPermissions(ctx, kbID)
}

// ListUserAccessibleKBs 列出用户可访问的知识库
func (s *AccessControlService) ListUserAccessibleKBs(ctx context.Context, userID string) ([]*KBPermission, error) {
	return s.store.GetUserPermissions(ctx, userID)
}

// hasPermission 检查是否有指定权限
func (s *AccessControlService) hasPermission(have, need PermissionLevel) bool {
	return s.comparePermission(have, need) >= 0
}

// comparePermission 比较权限级别
func (s *AccessControlService) comparePermission(a, b PermissionLevel) int {
	levels := map[PermissionLevel]int{
		PermissionNone:   0,
		PermissionRead:   1,
		PermissionWrite:  2,
		PermissionManage: 3,
		PermissionOwner:  4,
	}
	return levels[a] - levels[b]
}

func (s *AccessControlService) invalidateCache(userID, kbID string) {
	cacheKey := fmt.Sprintf("%s:%s", userID, kbID)
	s.cacheMu.Lock()
	delete(s.cache, cacheKey)
	s.cacheMu.Unlock()
}

// FilterByScope 根据权限范围过滤搜索结果
func FilterByScope(results []ACSearchResult, scope *PermissionScope) []ACSearchResult {
	if scope == nil {
		return results
	}

	filtered := make([]ACSearchResult, 0, len(results))
	for _, r := range results {
		// 检查文件夹限制
		if len(scope.Folders) > 0 && !containsAny(r.Metadata["folder"], scope.Folders) {
			continue
		}

		// 检查标签限制
		if len(scope.Tags) > 0 && !hasAnyTag(r.Metadata["tags"], scope.Tags) {
			continue
		}

		// 检查文档类型限制
		if len(scope.DocTypes) > 0 && !containsAny(r.Metadata["doc_type"], scope.DocTypes) {
			continue
		}

		filtered = append(filtered, r)

		// 检查最大结果数
		if scope.MaxResults > 0 && len(filtered) >= scope.MaxResults {
			break
		}
	}

	return filtered
}

// ACSearchResult 搜索结果（简化版，用于访问控制）
type ACSearchResult struct {
	ID       string
	Content  string
	Score    float64
	Metadata map[string]any
}

func containsAny(value any, allowed []string) bool {
	if value == nil {
		return false
	}
	v, ok := value.(string)
	if !ok {
		return false
	}
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func hasAnyTag(value any, allowed []string) bool {
	if value == nil {
		return false
	}
	tags, ok := value.([]string)
	if !ok {
		return false
	}
	for _, t := range tags {
		for _, a := range allowed {
			if t == a {
				return true
			}
		}
	}
	return false
}
