package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// JWTService JWT 令牌服务
type JWTService struct {
	secretKey     []byte
	issuer        string
	accessExpiry  time.Duration // 访问令牌过期时间（默认 2 小时）
	refreshExpiry time.Duration // 刷新令牌过期时间（默认 7 天）
	redisClient   redis.UniversalClient // Redis 客户端，用于黑名单
}

// NewJWTService 创建 JWT 服务
func NewJWTService(secretKey, issuer string, redisClient redis.UniversalClient) *JWTService {
	return &JWTService{
		secretKey:     []byte(secretKey),
		issuer:        issuer,
		accessExpiry:  2 * time.Hour,
		refreshExpiry: 7 * 24 * time.Hour,
		redisClient:   redisClient,
	}
}

// TokenClaims JWT 声明
type TokenClaims struct {
	UserID    string   `json:"uid"`
	TenantID  string   `json:"tid"`
	Roles     []string `json:"roles"`
	TokenType string   `json:"token_type"` // access 或 refresh
	jwt.RegisteredClaims
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"` // 秒
}

// GenerateTokenPair 生成访问令牌和刷新令牌对
func (s *JWTService) GenerateTokenPair(userID, tenantID string, roles []string) (*TokenPair, error) {
	// 生成访问令牌
	accessToken, err := s.generateToken(userID, tenantID, roles, "access", s.accessExpiry)
	if err != nil {
		return nil, fmt.Errorf("生成访问令牌失败: %w", err)
	}

	// 生成刷新令牌
	refreshToken, err := s.generateToken(userID, tenantID, roles, "refresh", s.refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("生成刷新令牌失败: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessExpiry.Seconds()),
	}, nil
}

// generateToken 生成 JWT 令牌
func (s *JWTService) generateToken(userID, tenantID string, roles []string, tokenType string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := &TokenClaims{
		UserID:    userID,
		TenantID:  tenantID,
		Roles:     roles,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", fmt.Errorf("签名令牌失败: %w", err)
	}

	return tokenString, nil
}

// ValidateToken 验证并解析 JWT 令牌
func (s *JWTService) ValidateToken(ctx context.Context, tokenString string) (*TokenClaims, error) {
	// 1. 检查黑名单
	if s.IsTokenBlacklisted(ctx, tokenString) {
		return nil, fmt.Errorf("令牌已失效")
	}

	// 2. 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (any, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("无效的签名算法: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("解析令牌失败: %w", err)
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("无效的令牌")
}

// RefreshAccessToken 使用刷新令牌生成新的访问令牌
func (s *JWTService) RefreshAccessToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// 验证刷新令牌
	claims, err := s.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("刷新令牌验证失败: %w", err)
	}

	// 确保是刷新令牌
	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("令牌类型错误: 期望 refresh，实际 %s", claims.TokenType)
	}

	// 生成新的令牌对
	return s.GenerateTokenPair(claims.UserID, claims.TenantID, claims.Roles)
}

// InvalidateToken 使令牌失效（加入黑名单）
func (s *JWTService) InvalidateToken(ctx context.Context, tokenString string) error {
	if s.redisClient == nil {
		return nil // 如果没有 Redis，则无法使用黑名单功能，直接返回
	}

	// 解析令牌以获取过期时间
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &TokenClaims{})
	if err != nil {
		return fmt.Errorf("解析令牌失败: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return fmt.Errorf("无效的令牌声明")
	}

	// 计算剩余有效期
	expirationTime := claims.ExpiresAt.Time
	ttl := time.Until(expirationTime)

	if ttl <= 0 {
		return nil // 令牌已过期，无需加入黑名单
	}

	// 将令牌加入 Redis 黑名单
	key := fmt.Sprintf("blacklist:token:%s", tokenString)
	if err := s.redisClient.Set(ctx, key, "revoked", ttl).Err(); err != nil {
		return fmt.Errorf("加入黑名单失败: %w", err)
	}

	return nil
}

// IsTokenBlacklisted 检查令牌是否在黑名单中
func (s *JWTService) IsTokenBlacklisted(ctx context.Context, tokenString string) bool {
	if s.redisClient == nil {
		return false
	}

	key := fmt.Sprintf("blacklist:token:%s", tokenString)
	exists, err := s.redisClient.Exists(ctx, key).Result()
	if err != nil {
		// 如果 Redis 错误，为了安全起见，可以记录日志并返回 false（或者 true，取决于安全策略）
		// 这里选择 fail-open (返回 false)，避免 Redis 故障导致所有请求失败
		return false
	}

	return exists > 0
}

// ExtractTokenFromBearer 从 Bearer 令牌中提取纯令牌字符串
func ExtractTokenFromBearer(bearerToken string) string {
	const prefix = "Bearer "
	if len(bearerToken) > len(prefix) && bearerToken[:len(prefix)] == prefix {
		return bearerToken[len(prefix):]
	}
	return bearerToken
}
