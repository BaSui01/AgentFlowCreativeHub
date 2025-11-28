package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageStatus 消息状态
type MessageStatus string

const (
	MessageStatusUnread MessageStatus = "unread"
	MessageStatusRead   MessageStatus = "read"
)

// MessagePriority 消息优先级
type MessagePriority string

const (
	MessagePriorityLow    MessagePriority = "low"
	MessagePriorityNormal MessagePriority = "normal"
	MessagePriorityHigh   MessagePriority = "high"
	MessagePriorityUrgent MessagePriority = "urgent"
)

// UserMessage 用户消息
type UserMessage struct {
	ID          string              `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID    string              `gorm:"type:varchar(36);not null;index" json:"tenant_id"`
	UserID      string              `gorm:"type:varchar(36);not null;index:idx_user_status" json:"user_id"`
	
	// 消息内容
	Title       string              `gorm:"type:varchar(200);not null" json:"title"`
	Content     string              `gorm:"type:text" json:"content"`
	Category    NotificationCategory `gorm:"type:varchar(50);index" json:"category"`
	Priority    MessagePriority     `gorm:"type:varchar(20);default:'normal'" json:"priority"`
	
	// 状态
	Status      MessageStatus       `gorm:"type:varchar(20);default:'unread';index:idx_user_status" json:"status"`
	ReadAt      *time.Time          `json:"read_at,omitempty"`
	
	// 关联信息
	RelatedType string              `gorm:"type:varchar(50)" json:"related_type,omitempty"`
	RelatedID   string              `gorm:"type:varchar(255)" json:"related_id,omitempty"`
	ActionURL   string              `gorm:"type:varchar(500)" json:"action_url,omitempty"`
	
	// 发送者
	SenderID    string              `gorm:"type:varchar(36)" json:"sender_id,omitempty"`
	SenderName  string              `gorm:"type:varchar(100)" json:"sender_name,omitempty"`
	
	// 元数据
	Metadata    string              `gorm:"type:jsonb" json:"metadata,omitempty"`
	
	// 过期时间（可选）
	ExpiresAt   *time.Time          `gorm:"index" json:"expires_at,omitempty"`
	
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	DeletedAt   gorm.DeletedAt      `gorm:"index" json:"-"`
}

// TableName 指定表名
func (UserMessage) TableName() string {
	return "user_messages"
}

// BeforeCreate GORM Hook
func (m *UserMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.Status == "" {
		m.Status = MessageStatusUnread
	}
	return nil
}

// MessageService 消息服务
type MessageService struct {
	db *gorm.DB
}

// NewMessageService 创建消息服务
func NewMessageService(db *gorm.DB) *MessageService {
	return &MessageService{db: db}
}

// AutoMigrate 自动迁移表
func (s *MessageService) AutoMigrate() error {
	return s.db.AutoMigrate(&UserMessage{})
}

// CreateMessageRequest 创建消息请求
type CreateMessageRequest struct {
	UserID      string              `json:"user_id" binding:"required"`
	Title       string              `json:"title" binding:"required,max=200"`
	Content     string              `json:"content,omitempty"`
	Category    NotificationCategory `json:"category,omitempty"`
	Priority    MessagePriority     `json:"priority,omitempty"`
	RelatedType string              `json:"related_type,omitempty"`
	RelatedID   string              `json:"related_id,omitempty"`
	ActionURL   string              `json:"action_url,omitempty"`
	SenderID    string              `json:"sender_id,omitempty"`
	SenderName  string              `json:"sender_name,omitempty"`
	Metadata    string              `json:"metadata,omitempty"`
	ExpiresAt   *time.Time          `json:"expires_at,omitempty"`
}

// CreateMessage 创建消息
func (s *MessageService) CreateMessage(ctx context.Context, tenantID string, req *CreateMessageRequest) (*UserMessage, error) {
	msg := &UserMessage{
		TenantID:    tenantID,
		UserID:      req.UserID,
		Title:       req.Title,
		Content:     req.Content,
		Category:    req.Category,
		Priority:    req.Priority,
		RelatedType: req.RelatedType,
		RelatedID:   req.RelatedID,
		ActionURL:   req.ActionURL,
		SenderID:    req.SenderID,
		SenderName:  req.SenderName,
		Metadata:    req.Metadata,
		ExpiresAt:   req.ExpiresAt,
		Status:      MessageStatusUnread,
	}

	if err := s.db.WithContext(ctx).Create(msg).Error; err != nil {
		return nil, fmt.Errorf("创建消息失败: %w", err)
	}

	return msg, nil
}

// GetMessage 获取消息详情
func (s *MessageService) GetMessage(ctx context.Context, tenantID, messageID string) (*UserMessage, error) {
	var msg UserMessage
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", messageID, tenantID).
		First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

// MarkAsRead 标记为已读
func (s *MessageService) MarkAsRead(ctx context.Context, tenantID, userID, messageID string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&UserMessage{}).
		Where("id = ? AND tenant_id = ? AND user_id = ? AND status = ?", messageID, tenantID, userID, MessageStatusUnread).
		Updates(map[string]any{
			"status":  MessageStatusRead,
			"read_at": &now,
		})

	if result.Error != nil {
		return fmt.Errorf("标记已读失败: %w", result.Error)
	}

	return nil
}

// MarkAsUnread 标记为未读
func (s *MessageService) MarkAsUnread(ctx context.Context, tenantID, userID, messageID string) error {
	result := s.db.WithContext(ctx).
		Model(&UserMessage{}).
		Where("id = ? AND tenant_id = ? AND user_id = ?", messageID, tenantID, userID).
		Updates(map[string]any{
			"status":  MessageStatusUnread,
			"read_at": nil,
		})

	if result.Error != nil {
		return fmt.Errorf("标记未读失败: %w", result.Error)
	}

	return nil
}

// MarkAllAsRead 标记所有消息为已读
func (s *MessageService) MarkAllAsRead(ctx context.Context, tenantID, userID string, category *NotificationCategory) error {
	now := time.Now()
	query := s.db.WithContext(ctx).
		Model(&UserMessage{}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, MessageStatusUnread)

	if category != nil {
		query = query.Where("category = ?", *category)
	}

	result := query.Updates(map[string]any{
		"status":  MessageStatusRead,
		"read_at": &now,
	})

	if result.Error != nil {
		return fmt.Errorf("批量标记已读失败: %w", result.Error)
	}

	return nil
}

// BatchMarkAsRead 批量标记为已读
func (s *MessageService) BatchMarkAsRead(ctx context.Context, tenantID, userID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&UserMessage{}).
		Where("id IN ? AND tenant_id = ? AND user_id = ? AND status = ?", messageIDs, tenantID, userID, MessageStatusUnread).
		Updates(map[string]any{
			"status":  MessageStatusRead,
			"read_at": &now,
		})

	if result.Error != nil {
		return fmt.Errorf("批量标记已读失败: %w", result.Error)
	}

	return nil
}

// ListMessagesRequest 查询消息请求
type ListMessagesRequest struct {
	Status      *MessageStatus       `form:"status"`
	Category    *NotificationCategory `form:"category"`
	Priority    *MessagePriority     `form:"priority"`
	RelatedType *string              `form:"related_type"`
	Page        int                  `form:"page" binding:"min=1"`
	PageSize    int                  `form:"page_size" binding:"min=1,max=100"`
}

// ListMessages 查询消息列表
func (s *MessageService) ListMessages(ctx context.Context, tenantID, userID string, req *ListMessagesRequest) ([]*UserMessage, int64, error) {
	query := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Where("(expires_at IS NULL OR expires_at > ?)", time.Now())

	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.Category != nil {
		query = query.Where("category = ?", *req.Category)
	}
	if req.Priority != nil {
		query = query.Where("priority = ?", *req.Priority)
	}
	if req.RelatedType != nil {
		query = query.Where("related_type = ?", *req.RelatedType)
	}

	// 计算总数
	var total int64
	if err := query.Model(&UserMessage{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize).Order("created_at DESC")

	var messages []*UserMessage
	if err := query.Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

// GetUnreadCount 获取未读消息数量
func (s *MessageService) GetUnreadCount(ctx context.Context, tenantID, userID string) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&UserMessage{}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, MessageStatusUnread).
		Where("(expires_at IS NULL OR expires_at > ?)", time.Now()).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetUnreadCountByCategory 按分类获取未读消息数量
func (s *MessageService) GetUnreadCountByCategory(ctx context.Context, tenantID, userID string) (map[NotificationCategory]int64, error) {
	type Result struct {
		Category NotificationCategory
		Count    int64
	}

	var results []Result
	if err := s.db.WithContext(ctx).
		Model(&UserMessage{}).
		Select("category, COUNT(*) as count").
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, MessageStatusUnread).
		Where("(expires_at IS NULL OR expires_at > ?)", time.Now()).
		Group("category").
		Scan(&results).Error; err != nil {
		return nil, err
	}

	countMap := make(map[NotificationCategory]int64)
	for _, r := range results {
		countMap[r.Category] = r.Count
	}

	return countMap, nil
}

// DeleteMessage 删除消息（软删除）
func (s *MessageService) DeleteMessage(ctx context.Context, tenantID, userID, messageID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", messageID, tenantID, userID).
		Delete(&UserMessage{})

	if result.Error != nil {
		return fmt.Errorf("删除消息失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// BatchDeleteMessages 批量删除消息
func (s *MessageService) BatchDeleteMessages(ctx context.Context, tenantID, userID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	result := s.db.WithContext(ctx).
		Where("id IN ? AND tenant_id = ? AND user_id = ?", messageIDs, tenantID, userID).
		Delete(&UserMessage{})

	if result.Error != nil {
		return fmt.Errorf("批量删除消息失败: %w", result.Error)
	}

	return nil
}

// CleanExpiredMessages 清理过期消息
func (s *MessageService) CleanExpiredMessages(ctx context.Context, tenantID string) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("tenant_id = ? AND expires_at IS NOT NULL AND expires_at < ?", tenantID, time.Now()).
		Delete(&UserMessage{})

	if result.Error != nil {
		return 0, fmt.Errorf("清理过期消息失败: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// MessageStats 消息统计
type MessageStats struct {
	TotalCount   int64 `json:"total_count"`
	UnreadCount  int64 `json:"unread_count"`
	ReadCount    int64 `json:"read_count"`
	TodayCount   int64 `json:"today_count"`
	ByCategory   map[NotificationCategory]int64 `json:"by_category"`
	ByPriority   map[MessagePriority]int64 `json:"by_priority"`
}

// GetStats 获取消息统计
func (s *MessageService) GetStats(ctx context.Context, tenantID, userID string) (*MessageStats, error) {
	stats := &MessageStats{
		ByCategory: make(map[NotificationCategory]int64),
		ByPriority: make(map[MessagePriority]int64),
	}

	baseQuery := s.db.WithContext(ctx).Model(&UserMessage{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID)

	// 总数
	if err := baseQuery.Count(&stats.TotalCount).Error; err != nil {
		return nil, err
	}

	// 未读数
	if err := s.db.WithContext(ctx).Model(&UserMessage{}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, MessageStatusUnread).
		Count(&stats.UnreadCount).Error; err != nil {
		return nil, err
	}

	stats.ReadCount = stats.TotalCount - stats.UnreadCount

	// 今日新消息
	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.WithContext(ctx).Model(&UserMessage{}).
		Where("tenant_id = ? AND user_id = ? AND created_at >= ?", tenantID, userID, today).
		Count(&stats.TodayCount).Error; err != nil {
		return nil, err
	}

	// 按分类统计
	var categoryStats []struct {
		Category NotificationCategory
		Count    int64
	}
	if err := s.db.WithContext(ctx).Model(&UserMessage{}).
		Select("category, COUNT(*) as count").
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Group("category").
		Scan(&categoryStats).Error; err != nil {
		return nil, err
	}
	for _, cs := range categoryStats {
		stats.ByCategory[cs.Category] = cs.Count
	}

	// 按优先级统计
	var priorityStats []struct {
		Priority MessagePriority
		Count    int64
	}
	if err := s.db.WithContext(ctx).Model(&UserMessage{}).
		Select("priority, COUNT(*) as count").
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Group("priority").
		Scan(&priorityStats).Error; err != nil {
		return nil, err
	}
	for _, ps := range priorityStats {
		stats.ByPriority[ps.Priority] = ps.Count
	}

	return stats, nil
}
