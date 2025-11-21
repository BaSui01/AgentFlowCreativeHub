package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backend/internal/security"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ModelCredentialService 管理模型凭证的增删查
type ModelCredentialService struct {
	db *gorm.DB
}

// NewModelCredentialService 创建服务实例
func NewModelCredentialService(db *gorm.DB) *ModelCredentialService {
	return &ModelCredentialService{db: db}
}

// CreateModelCredentialRequest 创建请求
type CreateModelCredentialRequest struct {
	TenantID     string
	ModelID      string
	Provider     string
	Name         string
	APIKey       string
	BaseURL      string
	ExtraHeaders map[string]any
	CreatedBy    string
	SetAsDefault bool
}

// CreateCredential 为指定模型创建凭证
func (s *ModelCredentialService) CreateCredential(ctx context.Context, req *CreateModelCredentialRequest) (*ModelCredential, error) {
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}
	if req.TenantID == "" || req.ModelID == "" {
		return nil, fmt.Errorf("tenant_id 与 model_id 必填")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("凭证名称不能为空")
	}
	if strings.TrimSpace(req.APIKey) == "" {
		return nil, fmt.Errorf("API Key 不能为空")
	}

	var model Model
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", req.ModelID, req.TenantID).
		First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("模型不存在或已删除")
		}
		return nil, fmt.Errorf("查询模型失败: %w", err)
	}

	provider := req.Provider
	if provider == "" {
		provider = model.Provider
	}

	ciphertext, err := security.EncryptSecret(req.APIKey)
	if err != nil {
		return nil, err
	}

	cred := &ModelCredential{
		ID:           uuid.New().String(),
		TenantID:     req.TenantID,
		ModelID:      req.ModelID,
		Provider:     provider,
		Name:         req.Name,
		Ciphertext:   ciphertext,
		BaseURL:      req.BaseURL,
		ExtraHeaders: req.ExtraHeaders,
		Status:       "active",
		CreatedBy:    req.CreatedBy,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(cred).Error; err != nil {
		return nil, fmt.Errorf("创建模型凭证失败: %w", err)
	}

	if req.SetAsDefault {
		_ = s.db.WithContext(ctx).
			Model(&Model{}).
			Where("id = ?", req.ModelID).
			Update("default_credential_id", cred.ID).Error
	}

	return sanitizeCredential(cred), nil
}

// ListCredentialsRequest 查询请求
type ListCredentialsRequest struct {
	TenantID string
	ModelID  string
}

// ListCredentials 返回凭证列表（不包含密钥）
func (s *ModelCredentialService) ListCredentials(ctx context.Context, req *ListCredentialsRequest) ([]*ModelCredential, error) {
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}
	query := s.db.WithContext(ctx).
		Model(&ModelCredential{})
	if req.TenantID != "" {
		query = query.Where("tenant_id = ?", req.TenantID)
	}
	if req.ModelID != "" {
		query = query.Where("model_id = ?", req.ModelID)
	}
	var creds []*ModelCredential
	if err := query.Order("created_at DESC").Find(&creds).Error; err != nil {
		return nil, fmt.Errorf("查询模型凭证失败: %w", err)
	}
	for _, cred := range creds {
		sanitizeCredential(cred)
	}
	return creds, nil
}

// DeleteCredential 删除凭证
func (s *ModelCredentialService) DeleteCredential(ctx context.Context, tenantID, credentialID string) error {
	var cred ModelCredential
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", credentialID, tenantID).
		First(&cred).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("凭证不存在")
		}
		return fmt.Errorf("查询凭证失败: %w", err)
	}

	if err := s.db.WithContext(ctx).Delete(&cred).Error; err != nil {
		return fmt.Errorf("删除凭证失败: %w", err)
	}

	// 如果被设置为默认凭证，移除绑定
	if cred.ModelID != "" {
		_ = s.db.WithContext(ctx).
			Model(&Model{}).
			Where("id = ? AND default_credential_id = ?", cred.ModelID, cred.ID).
			Update("default_credential_id", gorm.Expr("NULL")).Error
	}

	return nil
}

// ResolveCredential 解密凭证
func (s *ModelCredentialService) ResolveCredential(ctx context.Context, tenantID, credentialID string) (string, error) {
	var cred ModelCredential
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", credentialID, tenantID).
		First(&cred).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("凭证不存在")
		}
		return "", fmt.Errorf("查询凭证失败: %w", err)
	}
	return security.DecryptSecret(cred.Ciphertext)
}

func sanitizeCredential(cred *ModelCredential) *ModelCredential {
	if cred == nil {
		return nil
	}
	cred.Ciphertext = nil
	return cred
}
