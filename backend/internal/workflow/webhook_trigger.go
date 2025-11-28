package workflow

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebhookTrigger Webhook 触发器配置
type WebhookTrigger struct {
	ID         string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID   string `json:"tenantId" gorm:"type:uuid;not null;index"`
	WorkflowID string `json:"workflowId" gorm:"type:uuid;not null;index"`

	// 触发器配置
	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`

	// Webhook 端点
	EndpointPath string `json:"endpointPath" gorm:"size:255;not null;uniqueIndex"` // 唯一路径
	Secret       string `json:"-" gorm:"size:64;not null"`                         // HMAC 签名密钥

	// 输入映射
	InputMapping map[string]string `json:"inputMapping" gorm:"type:jsonb;serializer:json"` // Webhook 参数到工作流输入的映射

	// 限制
	RateLimitPerMinute int      `json:"rateLimitPerMinute" gorm:"default:10"`
	AllowedIPs         []string `json:"allowedIps" gorm:"type:jsonb;serializer:json"` // IP 白名单

	// 状态
	IsActive     bool       `json:"isActive" gorm:"default:true"`
	LastTriggered *time.Time `json:"lastTriggered"`
	TriggerCount int64      `json:"triggerCount" gorm:"default:0"`

	// 时间戳
	CreatedAt time.Time  `json:"createdAt" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null;autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy string     `json:"createdBy" gorm:"type:uuid"`
}

func (WebhookTrigger) TableName() string {
	return "webhook_triggers"
}

// WebhookTriggerLog Webhook 触发日志
type WebhookTriggerLog struct {
	ID          string         `json:"id" gorm:"primaryKey;type:uuid"`
	TriggerID   string         `json:"triggerId" gorm:"type:uuid;not null;index"`
	TenantID    string         `json:"tenantId" gorm:"type:uuid;not null;index"`
	ExecutionID string         `json:"executionId" gorm:"type:uuid;index"` // 关联的工作流执行 ID

	// 请求信息
	RequestIP      string         `json:"requestIp" gorm:"size:50"`
	RequestHeaders map[string]any `json:"requestHeaders" gorm:"type:jsonb;serializer:json"`
	RequestBody    string         `json:"requestBody" gorm:"type:text"`

	// 结果
	Status       string `json:"status" gorm:"size:50;not null"` // success, failed, rejected
	ErrorMessage string `json:"errorMessage" gorm:"type:text"`

	CreatedAt time.Time `json:"createdAt" gorm:"not null;autoCreateTime;index"`
}

func (WebhookTriggerLog) TableName() string {
	return "webhook_trigger_logs"
}

// WebhookTriggerService Webhook 触发器服务
type WebhookTriggerService struct {
	db *gorm.DB
}

// NewWebhookTriggerService 创建服务
func NewWebhookTriggerService(db *gorm.DB) *WebhookTriggerService {
	return &WebhookTriggerService{db: db}
}

// AutoMigrate 自动迁移
func (s *WebhookTriggerService) AutoMigrate() error {
	return s.db.AutoMigrate(&WebhookTrigger{}, &WebhookTriggerLog{})
}

// CreateTriggerRequest 创建触发器请求
type CreateTriggerRequest struct {
	TenantID           string
	WorkflowID         string
	Name               string
	Description        string
	InputMapping       map[string]string
	RateLimitPerMinute int
	AllowedIPs         []string
	CreatedBy          string
}

// CreateTriggerResponse 创建触发器响应
type CreateTriggerResponse struct {
	ID           string `json:"id"`
	EndpointPath string `json:"endpointPath"`
	Secret       string `json:"secret"` // 仅创建时返回
	WebhookURL   string `json:"webhookUrl"`
}

// CreateTrigger 创建 Webhook 触发器
func (s *WebhookTriggerService) CreateTrigger(ctx context.Context, req *CreateTriggerRequest) (*CreateTriggerResponse, error) {
	// 验证工作流存在
	var wf Workflow
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", req.WorkflowID, req.TenantID).First(&wf).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("工作流不存在")
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	// 生成唯一端点路径
	pathBytes := make([]byte, 16)
	rand.Read(pathBytes)
	endpointPath := hex.EncodeToString(pathBytes)

	// 生成签名密钥
	secretBytes := make([]byte, 32)
	rand.Read(secretBytes)
	secret := hex.EncodeToString(secretBytes)

	rateLimit := req.RateLimitPerMinute
	if rateLimit <= 0 {
		rateLimit = 10
	}

	trigger := &WebhookTrigger{
		ID:                 uuid.New().String(),
		TenantID:           req.TenantID,
		WorkflowID:         req.WorkflowID,
		Name:               req.Name,
		Description:        req.Description,
		EndpointPath:       endpointPath,
		Secret:             secret,
		InputMapping:       req.InputMapping,
		RateLimitPerMinute: rateLimit,
		AllowedIPs:         req.AllowedIPs,
		IsActive:           true,
		CreatedBy:          req.CreatedBy,
	}

	if err := s.db.WithContext(ctx).Create(trigger).Error; err != nil {
		return nil, fmt.Errorf("创建触发器失败: %w", err)
	}

	return &CreateTriggerResponse{
		ID:           trigger.ID,
		EndpointPath: endpointPath,
		Secret:       secret,
		WebhookURL:   fmt.Sprintf("/api/webhooks/%s", endpointPath),
	}, nil
}

// GetTriggerByPath 根据路径获取触发器
func (s *WebhookTriggerService) GetTriggerByPath(ctx context.Context, path string) (*WebhookTrigger, error) {
	var trigger WebhookTrigger
	if err := s.db.WithContext(ctx).
		Where("endpoint_path = ? AND is_active = true AND deleted_at IS NULL", path).
		First(&trigger).Error; err != nil {
		return nil, err
	}
	return &trigger, nil
}

// ValidateSignature 验证 HMAC 签名
func (s *WebhookTriggerService) ValidateSignature(trigger *WebhookTrigger, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(trigger.Secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// RecordTrigger 记录触发
func (s *WebhookTriggerService) RecordTrigger(ctx context.Context, triggerID string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&WebhookTrigger{}).
		Where("id = ?", triggerID).
		Updates(map[string]interface{}{
			"last_triggered": now,
			"trigger_count":  gorm.Expr("trigger_count + 1"),
		}).Error
}

// CreateLog 创建触发日志
func (s *WebhookTriggerService) CreateLog(ctx context.Context, log *WebhookTriggerLog) error {
	log.ID = uuid.New().String()
	return s.db.WithContext(ctx).Create(log).Error
}

// ListTriggers 列出触发器
func (s *WebhookTriggerService) ListTriggers(ctx context.Context, tenantID string, workflowID string) ([]WebhookTrigger, error) {
	var triggers []WebhookTrigger
	query := s.db.WithContext(ctx).Where("tenant_id = ? AND deleted_at IS NULL", tenantID)
	if workflowID != "" {
		query = query.Where("workflow_id = ?", workflowID)
	}
	err := query.Order("created_at DESC").Find(&triggers).Error
	return triggers, err
}

// UpdateTrigger 更新触发器
func (s *WebhookTriggerService) UpdateTrigger(ctx context.Context, triggerID, tenantID string, updates map[string]interface{}) error {
	result := s.db.WithContext(ctx).Model(&WebhookTrigger{}).
		Where("id = ? AND tenant_id = ?", triggerID, tenantID).
		Updates(updates)
	if result.RowsAffected == 0 {
		return fmt.Errorf("触发器不存在")
	}
	return result.Error
}

// DeleteTrigger 删除触发器
func (s *WebhookTriggerService) DeleteTrigger(ctx context.Context, triggerID, tenantID string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).Model(&WebhookTrigger{}).
		Where("id = ? AND tenant_id = ?", triggerID, tenantID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"is_active":  false,
		})
	if result.RowsAffected == 0 {
		return fmt.Errorf("触发器不存在")
	}
	return result.Error
}

// RegenerateSecret 重新生成密钥
func (s *WebhookTriggerService) RegenerateSecret(ctx context.Context, triggerID, tenantID string) (string, error) {
	secretBytes := make([]byte, 32)
	rand.Read(secretBytes)
	newSecret := hex.EncodeToString(secretBytes)

	result := s.db.WithContext(ctx).Model(&WebhookTrigger{}).
		Where("id = ? AND tenant_id = ?", triggerID, tenantID).
		Update("secret", newSecret)

	if result.RowsAffected == 0 {
		return "", fmt.Errorf("触发器不存在")
	}

	return newSecret, result.Error
}

// ListLogs 列出触发日志
func (s *WebhookTriggerService) ListLogs(ctx context.Context, triggerID string, limit int) ([]WebhookTriggerLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var logs []WebhookTriggerLog
	err := s.db.WithContext(ctx).
		Where("trigger_id = ?", triggerID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
