package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// KeyRotationConfig 密钥轮换配置
type KeyRotationConfig struct {
	RotationInterval time.Duration // 轮换间隔 (默认 7 天)
	KeyOverlap       time.Duration // 新旧密钥重叠时间 (默认 24 小时)
	KeyLength        int           // 密钥长度 (默认 32 字节)
}

// DefaultKeyRotationConfig 默认配置
func DefaultKeyRotationConfig() *KeyRotationConfig {
	return &KeyRotationConfig{
		RotationInterval: 7 * 24 * time.Hour,
		KeyOverlap:       24 * time.Hour,
		KeyLength:        32,
	}
}

// JWTKey JWT 密钥
type JWTKey struct {
	ID        string    `json:"id"`
	Secret    []byte    `json:"secret"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	IsActive  bool      `json:"isActive"`
}

// JWTKeyManager JWT 密钥管理器 (支持轮换)
type JWTKeyManager struct {
	mu          sync.RWMutex
	config      *KeyRotationConfig
	redisClient redis.UniversalClient
	currentKey  *JWTKey
	previousKey *JWTKey // 旧密钥 (用于验证过渡期的 token)
	redisPrefix string
}

// NewJWTKeyManager 创建密钥管理器
func NewJWTKeyManager(redisClient redis.UniversalClient, config *KeyRotationConfig) *JWTKeyManager {
	if config == nil {
		config = DefaultKeyRotationConfig()
	}
	return &JWTKeyManager{
		config:      config,
		redisClient: redisClient,
		redisPrefix: "jwt:key:",
	}
}

// Initialize 初始化密钥管理器
func (m *JWTKeyManager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 尝试从 Redis 加载现有密钥
	if err := m.loadKeysFromRedis(ctx); err != nil {
		// 如果没有密钥，生成新的
		if err := m.generateNewKey(ctx); err != nil {
			return fmt.Errorf("generate initial key: %w", err)
		}
	}

	return nil
}

// loadKeysFromRedis 从 Redis 加载密钥
func (m *JWTKeyManager) loadKeysFromRedis(ctx context.Context) error {
	// 加载当前密钥
	currentData, err := m.redisClient.Get(ctx, m.redisPrefix+"current").Bytes()
	if err != nil {
		return err
	}

	var current JWTKey
	if err := json.Unmarshal(currentData, &current); err != nil {
		return err
	}
	m.currentKey = &current

	// 尝试加载旧密钥
	previousData, err := m.redisClient.Get(ctx, m.redisPrefix+"previous").Bytes()
	if err == nil {
		var previous JWTKey
		if err := json.Unmarshal(previousData, &previous); err == nil {
			m.previousKey = &previous
		}
	}

	return nil
}

// generateNewKey 生成新密钥
func (m *JWTKeyManager) generateNewKey(ctx context.Context) error {
	// 生成随机密钥
	secret := make([]byte, m.config.KeyLength)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("generate random key: %w", err)
	}

	now := time.Now()
	newKey := &JWTKey{
		ID:        generateKeyID(),
		Secret:    secret,
		CreatedAt: now,
		ExpiresAt: now.Add(m.config.RotationInterval + m.config.KeyOverlap),
		IsActive:  true,
	}

	// 将当前密钥移到旧密钥
	if m.currentKey != nil {
		m.previousKey = m.currentKey
		m.previousKey.IsActive = false

		// 保存旧密钥到 Redis (带过期时间)
		data, _ := json.Marshal(m.previousKey)
		m.redisClient.Set(ctx, m.redisPrefix+"previous", data, m.config.KeyOverlap)
	}

	// 设置新密钥为当前密钥
	m.currentKey = newKey

	// 保存到 Redis
	data, _ := json.Marshal(m.currentKey)
	return m.redisClient.Set(ctx, m.redisPrefix+"current", data, 0).Err()
}

// RotateKey 手动轮换密钥
func (m *JWTKeyManager) RotateKey(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.generateNewKey(ctx)
}

// GetCurrentKey 获取当前密钥
func (m *JWTKeyManager) GetCurrentKey() *JWTKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentKey
}

// GetSigningKey 获取签名密钥
func (m *JWTKeyManager) GetSigningKey() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentKey == nil {
		return nil
	}
	return m.currentKey.Secret
}

// GetKeyFunc 获取 jwt.Keyfunc (用于验证)
func (m *JWTKeyManager) GetKeyFunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		m.mu.RLock()
		defer m.mu.RUnlock()

		// 优先使用当前密钥
		if m.currentKey != nil {
			return m.currentKey.Secret, nil
		}

		return nil, fmt.Errorf("no valid signing key")
	}
}

// ValidateWithAllKeys 使用所有有效密钥验证 token
func (m *JWTKeyManager) ValidateWithAllKeys(tokenString string) (*jwt.Token, error) {
	m.mu.RLock()
	keys := []*JWTKey{m.currentKey, m.previousKey}
	m.mu.RUnlock()

	var lastErr error
	for _, key := range keys {
		if key == nil {
			continue
		}

		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return key.Secret, nil
		})

		if err == nil && token.Valid {
			return token, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

// NeedsRotation 检查是否需要轮换
func (m *JWTKeyManager) NeedsRotation() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentKey == nil {
		return true
	}

	// 如果当前密钥创建时间超过轮换间隔，需要轮换
	return time.Since(m.currentKey.CreatedAt) > m.config.RotationInterval
}

// StartAutoRotation 启动自动轮换
func (m *JWTKeyManager) StartAutoRotation(ctx context.Context) {
	ticker := time.NewTicker(time.Hour) // 每小时检查一次
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if m.NeedsRotation() {
					if err := m.RotateKey(ctx); err != nil {
						fmt.Printf("JWT key rotation failed: %v\n", err)
					} else {
						fmt.Println("JWT key rotated successfully")
					}
				}
			}
		}
	}()
}

// GetKeyInfo 获取密钥信息 (不包含密钥本身)
func (m *JWTKeyManager) GetKeyInfo() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := make(map[string]interface{})

	if m.currentKey != nil {
		info["current"] = map[string]interface{}{
			"id":        m.currentKey.ID,
			"createdAt": m.currentKey.CreatedAt,
			"expiresAt": m.currentKey.ExpiresAt,
			"isActive":  m.currentKey.IsActive,
		}
	}

	if m.previousKey != nil {
		info["previous"] = map[string]interface{}{
			"id":        m.previousKey.ID,
			"createdAt": m.previousKey.CreatedAt,
			"expiresAt": m.previousKey.ExpiresAt,
			"isActive":  m.previousKey.IsActive,
		}
	}

	return info
}

// generateKeyID 生成密钥 ID
func generateKeyID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// JWTServiceWithRotation 支持密钥轮换的 JWT 服务
type JWTServiceWithRotation struct {
	*JWTService
	keyManager *JWTKeyManager
}

// NewJWTServiceWithRotation 创建支持密钥轮换的 JWT 服务
func NewJWTServiceWithRotation(issuer string, redisClient redis.UniversalClient, config *KeyRotationConfig) (*JWTServiceWithRotation, error) {
	keyManager := NewJWTKeyManager(redisClient, config)

	// 初始化密钥管理器
	ctx := context.Background()
	if err := keyManager.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initialize key manager: %w", err)
	}

	// 创建基础 JWT 服务
	jwtService := &JWTService{
		secretKey:     keyManager.GetSigningKey(),
		issuer:        issuer,
		accessExpiry:  2 * time.Hour,
		refreshExpiry: 7 * 24 * time.Hour,
		redisClient:   redisClient,
	}

	return &JWTServiceWithRotation{
		JWTService: jwtService,
		keyManager: keyManager,
	}, nil
}

// GenerateTokenPair 生成 token (使用当前密钥)
func (s *JWTServiceWithRotation) GenerateTokenPair(userID, tenantID string, roles []string) (*TokenPair, error) {
	// 确保使用最新的密钥
	s.secretKey = s.keyManager.GetSigningKey()
	return s.JWTService.GenerateTokenPair(userID, tenantID, roles)
}

// ValidateToken 验证 token (尝试所有有效密钥)
func (s *JWTServiceWithRotation) ValidateToken(ctx context.Context, tokenString string) (*TokenClaims, error) {
	// 检查黑名单
	if s.IsTokenBlacklisted(ctx, tokenString) {
		return nil, fmt.Errorf("令牌已失效")
	}

	// 使用所有有效密钥验证
	token, err := s.keyManager.ValidateWithAllKeys(tokenString)
	if err != nil {
		return nil, fmt.Errorf("验证令牌失败: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("解析令牌声明失败")
	}

	// 转换为 TokenClaims
	tokenClaims := &TokenClaims{
		UserID:   claims["sub"].(string),
		TenantID: claims["tenant_id"].(string),
	}

	if rolesRaw, ok := claims["roles"].([]interface{}); ok {
		for _, r := range rolesRaw {
			if roleStr, ok := r.(string); ok {
				tokenClaims.Roles = append(tokenClaims.Roles, roleStr)
			}
		}
	}

	return tokenClaims, nil
}

// RotateKey 轮换密钥
func (s *JWTServiceWithRotation) RotateKey(ctx context.Context) error {
	return s.keyManager.RotateKey(ctx)
}

// StartAutoRotation 启动自动轮换
func (s *JWTServiceWithRotation) StartAutoRotation(ctx context.Context) {
	s.keyManager.StartAutoRotation(ctx)
}

// GetKeyInfo 获取密钥信息
func (s *JWTServiceWithRotation) GetKeyInfo() map[string]interface{} {
	return s.keyManager.GetKeyInfo()
}
