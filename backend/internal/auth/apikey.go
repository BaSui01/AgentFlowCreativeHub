package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrAPIKeyNotFound   = errors.New("API Key 不存在")
	ErrAPIKeyExpired    = errors.New("API Key 已过期")
	ErrAPIKeyRevoked    = errors.New("API Key 已撤销")
	ErrAPIKeyInvalid    = errors.New("无效的 API Key")
	ErrPermissionDenied = errors.New("权限不足")
)

// APIKey API 密钥模型
type APIKey struct {
	ID          string     `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string     `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID      string     `json:"userId" gorm:"type:uuid;index"`
	
	// 密钥信息
	Name        string     `json:"name" gorm:"size:100;not null"`
	KeyPrefix   string     `json:"keyPrefix" gorm:"size:10;not null"` // 显示用前缀
	KeyHash     string     `json:"-" gorm:"size:64;not null;uniqueIndex"` // SHA256 哈希
	
	// 权限范围
	Scopes      string     `json:"scopes" gorm:"size:500"` // 逗号分隔的权限列表
	
	// 限制
	RateLimitPerMinute int `json:"rateLimitPerMinute" gorm:"default:60"`
	AllowedIPs         string `json:"allowedIps" gorm:"size:500"` // 逗号分隔的 IP 白名单
	
	// 有效期
	ExpiresAt   *time.Time `json:"expiresAt"`
	
	// 状态
	IsActive    bool       `json:"isActive" gorm:"default:true"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	UsageCount  int64      `json:"usageCount" gorm:"default:0"`
	
	// 时间戳
	CreatedAt   time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	RevokedAt   *time.Time `json:"revokedAt"`
	RevokedBy   string     `json:"revokedBy" gorm:"type:uuid"`
}

func (APIKey) TableName() string {
	return "api_keys"
}

// APIKeyService API Key 服务
type APIKeyService struct {
	db *gorm.DB
}

// NewAPIKeyService 创建服务
func NewAPIKeyService(db *gorm.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

// AutoMigrate 自动迁移
func (s *APIKeyService) AutoMigrate() error {
	return s.db.AutoMigrate(&APIKey{})
}

// CreateAPIKeyRequest 创建 API Key 请求
type CreateAPIKeyRequest struct {
	TenantID           string     `json:"tenantId"`
	UserID             string     `json:"userId"`
	Name               string     `json:"name" binding:"required"`
	Scopes             []string   `json:"scopes"`
	RateLimitPerMinute int        `json:"rateLimitPerMinute"`
	AllowedIPs         []string   `json:"allowedIps"`
	ExpiresIn          int        `json:"expiresIn"` // 有效期（天），0 表示永久
}

// CreateAPIKeyResponse 创建 API Key 响应
type CreateAPIKeyResponse struct {
	ID        string `json:"id"`
	Key       string `json:"key"` // 仅在创建时返回完整密钥
	Name      string `json:"name"`
	KeyPrefix string `json:"keyPrefix"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// CreateAPIKey 创建 API Key
func (s *APIKeyService) CreateAPIKey(ctx context.Context, req *CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	// 生成随机密钥
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, err
	}
	
	// 格式：sk_live_<random>
	rawKey := "sk_live_" + hex.EncodeToString(keyBytes)
	keyPrefix := rawKey[:12] + "..."
	
	// 计算哈希
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	
	// 设置过期时间
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresIn)
		expiresAt = &t
	}
	
	// 设置默认限流
	rateLimit := req.RateLimitPerMinute
	if rateLimit <= 0 {
		rateLimit = 60
	}
	
	apiKey := &APIKey{
		ID:                 uuid.New().String(),
		TenantID:           req.TenantID,
		UserID:             req.UserID,
		Name:               req.Name,
		KeyPrefix:          keyPrefix,
		KeyHash:            keyHash,
		Scopes:             strings.Join(req.Scopes, ","),
		RateLimitPerMinute: rateLimit,
		AllowedIPs:         strings.Join(req.AllowedIPs, ","),
		ExpiresAt:          expiresAt,
		IsActive:           true,
	}
	
	if err := s.db.WithContext(ctx).Create(apiKey).Error; err != nil {
		return nil, err
	}
	
	return &CreateAPIKeyResponse{
		ID:        apiKey.ID,
		Key:       rawKey, // 仅此一次返回完整密钥
		Name:      apiKey.Name,
		KeyPrefix: keyPrefix,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateAPIKey 验证 API Key
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, rawKey string) (*APIKey, error) {
	// 计算哈希
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	
	var apiKey APIKey
	err := s.db.WithContext(ctx).Where("key_hash = ?", keyHash).First(&apiKey).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	
	// 检查状态
	if !apiKey.IsActive {
		return nil, ErrAPIKeyRevoked
	}
	
	if apiKey.RevokedAt != nil {
		return nil, ErrAPIKeyRevoked
	}
	
	// 检查过期
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, ErrAPIKeyExpired
	}
	
	// 更新使用统计
	now := time.Now()
	s.db.WithContext(ctx).Model(&apiKey).Updates(map[string]interface{}{
		"last_used_at": now,
		"usage_count":  gorm.Expr("usage_count + 1"),
	})
	
	return &apiKey, nil
}

// ListAPIKeys 列出 API Keys
func (s *APIKeyService) ListAPIKeys(ctx context.Context, tenantID string) ([]APIKey, error) {
	var keys []APIKey
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// RevokeAPIKey 撤销 API Key
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, tenantID, keyID, revokerID string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).Model(&APIKey{}).
		Where("id = ? AND tenant_id = ?", keyID, tenantID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"revoked_at": now,
			"revoked_by": revokerID,
		})
	
	if result.RowsAffected == 0 {
		return ErrAPIKeyNotFound
	}
	return result.Error
}

// DeleteAPIKey 删除 API Key
func (s *APIKeyService) DeleteAPIKey(ctx context.Context, tenantID, keyID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", keyID, tenantID).
		Delete(&APIKey{})
	
	if result.RowsAffected == 0 {
		return ErrAPIKeyNotFound
	}
	return result.Error
}

// HasScope 检查 API Key 是否有指定权限
func (k *APIKey) HasScope(scope string) bool {
	if k.Scopes == "" || k.Scopes == "*" {
		return true // 空或 * 表示全部权限
	}
	
	scopes := strings.Split(k.Scopes, ",")
	for _, s := range scopes {
		if strings.TrimSpace(s) == scope {
			return true
		}
	}
	return false
}

// IsIPAllowed 检查 IP 是否在白名单中
func (k *APIKey) IsIPAllowed(ip string) bool {
	if k.AllowedIPs == "" {
		return true // 空表示不限制
	}
	
	allowedIPs := strings.Split(k.AllowedIPs, ",")
	for _, allowed := range allowedIPs {
		if strings.TrimSpace(allowed) == ip {
			return true
		}
	}
	return false
}

// ============================================================================
// Gin 中间件
// ============================================================================

// APIKeyAuthMiddleware API Key 认证中间件
func APIKeyAuthMiddleware(service *APIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 API Key
		authHeader := c.GetHeader("Authorization")
		apiKey := ""
		
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			apiKey = c.GetHeader("X-API-Key")
		}
		
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "缺少 API Key",
				"code":  "API_KEY_MISSING",
			})
			return
		}
		
		// 验证 API Key
		key, err := service.ValidateAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			status := http.StatusUnauthorized
			code := "API_KEY_INVALID"
			msg := "无效的 API Key"
			
			switch err {
			case ErrAPIKeyExpired:
				code = "API_KEY_EXPIRED"
				msg = "API Key 已过期"
			case ErrAPIKeyRevoked:
				code = "API_KEY_REVOKED"
				msg = "API Key 已被撤销"
			}
			
			c.AbortWithStatusJSON(status, gin.H{
				"error": msg,
				"code":  code,
			})
			return
		}
		
		// 检查 IP 白名单
		if !key.IsIPAllowed(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "IP 地址不在白名单中",
				"code":  "IP_NOT_ALLOWED",
			})
			return
		}
		
		// 设置上下文
		c.Set("api_key_id", key.ID)
		c.Set("tenant_id", key.TenantID)
		c.Set("user_id", key.UserID)
		c.Set("api_key_scopes", key.Scopes)
		
		c.Next()
	}
}

// RequireAPIKeyScope 要求特定权限的中间件
func RequireAPIKeyScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopes := c.GetString("api_key_scopes")
		if scopes == "" || scopes == "*" {
			c.Next()
			return
		}
		
		scopeList := strings.Split(scopes, ",")
		for _, s := range scopeList {
			if strings.TrimSpace(s) == scope {
				c.Next()
				return
			}
		}
		
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "权限不足",
			"code":  "SCOPE_REQUIRED",
			"required_scope": scope,
		})
	}
}
