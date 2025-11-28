package approval

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"backend/internal/logger"
	"backend/internal/notification"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// QuestionStatus 问题状态
type QuestionStatus string

const (
	QuestionStatusPending   QuestionStatus = "pending"
	QuestionStatusAnswered  QuestionStatus = "answered"
	QuestionStatusTimedOut  QuestionStatus = "timed_out"
	QuestionStatusCancelled QuestionStatus = "cancelled"
)

// UserQuestion 用户问题
type UserQuestion struct {
	ID             string         `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID       string         `json:"tenant_id" gorm:"type:uuid;not null;index"`
	SessionID      string         `json:"session_id" gorm:"size:100;not null;index"`
	WorkflowID     *string        `json:"workflow_id,omitempty" gorm:"type:uuid"`
	StepID         *string        `json:"step_id,omitempty" gorm:"size:100"`
	Question       string         `json:"question" gorm:"type:text;not null"`
	Options        []string       `json:"options" gorm:"type:jsonb;serializer:json"`
	AllowCustom    bool           `json:"allow_custom" gorm:"default:true"`
	Status         QuestionStatus `json:"status" gorm:"size:50;default:pending"`
	SelectedOption *string        `json:"selected_option,omitempty" gorm:"size:500"`
	CustomInput    *string        `json:"custom_input,omitempty" gorm:"type:text"`
	TimeoutSeconds int            `json:"timeout_seconds" gorm:"default:300"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	AnsweredAt     *time.Time     `json:"answered_at,omitempty"`
	AnsweredBy     *string        `json:"answered_by,omitempty" gorm:"size:100"`
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

func (UserQuestion) TableName() string {
	return "user_questions"
}

// UserQuestionResult 用户问题回答结果
type UserQuestionResult struct {
	Selected    string  `json:"selected"`
	CustomInput *string `json:"custom_input,omitempty"`
}

// AskUserQuestionInput 创建问题输入
type AskUserQuestionInput struct {
	TenantID       string   `json:"tenant_id"`
	SessionID      string   `json:"session_id"`
	WorkflowID     *string  `json:"workflow_id,omitempty"`
	StepID         *string  `json:"step_id,omitempty"`
	Question       string   `json:"question"`
	Options        []string `json:"options"`
	AllowCustom    bool     `json:"allow_custom"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

// UserQuestionService 用户问题服务
type UserQuestionService struct {
	db        *gorm.DB
	notifier  *notification.MultiNotifier
	waiters   map[string]chan *UserQuestionResult
	mu        sync.RWMutex
	logger    *zap.Logger
}

// NewUserQuestionService 创建用户问题服务
func NewUserQuestionService(db *gorm.DB, notifier *notification.MultiNotifier) *UserQuestionService {
	return &UserQuestionService{
		db:       db,
		notifier: notifier,
		waiters:  make(map[string]chan *UserQuestionResult),
		logger:   logger.Get(),
	}
}

// AutoMigrate 自动迁移
func (s *UserQuestionService) AutoMigrate() error {
	if s.db == nil {
		return nil
	}
	return s.db.AutoMigrate(&UserQuestion{})
}

// AskQuestion 向用户提问并等待回答
func (s *UserQuestionService) AskQuestion(ctx context.Context, input *AskUserQuestionInput) (*UserQuestionResult, error) {
	if input == nil || input.Question == "" {
		return nil, errors.New("问题不能为空")
	}
	if len(input.Options) < 2 {
		return nil, errors.New("至少需要 2 个选项")
	}

	timeout := input.TimeoutSeconds
	if timeout <= 0 {
		timeout = 300 // 默认 5 分钟
	}

	expiresAt := time.Now().Add(time.Duration(timeout) * time.Second)

	question := &UserQuestion{
		ID:             uuid.New().String(),
		TenantID:       input.TenantID,
		SessionID:      input.SessionID,
		WorkflowID:     input.WorkflowID,
		StepID:         input.StepID,
		Question:       input.Question,
		Options:        input.Options,
		AllowCustom:    input.AllowCustom,
		Status:         QuestionStatusPending,
		TimeoutSeconds: timeout,
		ExpiresAt:      &expiresAt,
	}

	// 保存到数据库
	if s.db != nil {
		if err := s.db.WithContext(ctx).Create(question).Error; err != nil {
			return nil, fmt.Errorf("保存问题失败: %w", err)
		}
	}

	// 创建等待通道
	waitChan := make(chan *UserQuestionResult, 1)
	s.mu.Lock()
	s.waiters[question.ID] = waitChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.waiters, question.ID)
		s.mu.Unlock()
		close(waitChan)
	}()

	// 发送通知
	if s.notifier != nil {
		s.notifier.Send(ctx, &notification.Notification{
			Type:     "websocket",
			TenantID: input.TenantID,
			Subject:  "user_question",
			Body:     input.Question,
			Data: map[string]any{
				"type":     "user_question",
				"question": question,
			},
		})
	}

	s.logger.Info("等待用户回答问题",
		zap.String("question_id", question.ID),
		zap.String("session_id", input.SessionID),
		zap.Int("timeout", timeout),
	)

	// 等待回答或超时
	select {
	case result := <-waitChan:
		return result, nil
	case <-time.After(time.Duration(timeout) * time.Second):
		// 更新状态为超时
		if s.db != nil {
			s.db.Model(&UserQuestion{}).Where("id = ?", question.ID).
				Update("status", QuestionStatusTimedOut)
		}
		return nil, fmt.Errorf("等待用户回答超时（%d秒）", timeout)
	case <-ctx.Done():
		// 更新状态为取消
		if s.db != nil {
			s.db.Model(&UserQuestion{}).Where("id = ?", question.ID).
				Update("status", QuestionStatusCancelled)
		}
		return nil, ctx.Err()
	}
}

// AnswerQuestion 回答问题
func (s *UserQuestionService) AnswerQuestion(ctx context.Context, questionID string, selected string, customInput *string, answeredBy string) error {
	if questionID == "" || selected == "" {
		return errors.New("问题 ID 和选项不能为空")
	}

	// 获取问题
	var question UserQuestion
	if s.db != nil {
		if err := s.db.WithContext(ctx).Where("id = ?", questionID).First(&question).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("问题不存在")
			}
			return err
		}

		if question.Status != QuestionStatusPending {
			return fmt.Errorf("问题已被回答或已超时: %s", question.Status)
		}

		// 验证选项
		validOption := false
		for _, opt := range question.Options {
			if opt == selected {
				validOption = true
				break
			}
		}
		if !validOption && !question.AllowCustom {
			return errors.New("无效的选项")
		}

		// 更新问题状态
		now := time.Now()
		updates := map[string]any{
			"status":          QuestionStatusAnswered,
			"selected_option": selected,
			"answered_at":     now,
			"answered_by":     answeredBy,
		}
		if customInput != nil {
			updates["custom_input"] = *customInput
		}

		if err := s.db.Model(&UserQuestion{}).Where("id = ?", questionID).Updates(updates).Error; err != nil {
			return fmt.Errorf("更新问题状态失败: %w", err)
		}
	}

	// 通知等待者
	result := &UserQuestionResult{
		Selected:    selected,
		CustomInput: customInput,
	}

	s.mu.RLock()
	waitChan, exists := s.waiters[questionID]
	s.mu.RUnlock()

	if exists {
		select {
		case waitChan <- result:
			s.logger.Info("用户已回答问题", zap.String("question_id", questionID), zap.String("selected", selected))
		default:
			s.logger.Warn("无法发送回答结果", zap.String("question_id", questionID))
		}
	}

	return nil
}

// GetQuestion 获取问题
func (s *UserQuestionService) GetQuestion(ctx context.Context, questionID string) (*UserQuestion, error) {
	if s.db == nil {
		return nil, errors.New("数据库未配置")
	}

	var question UserQuestion
	if err := s.db.WithContext(ctx).Where("id = ?", questionID).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &question, nil
}

// ListPendingQuestions 列出待回答的问题
func (s *UserQuestionService) ListPendingQuestions(ctx context.Context, tenantID, sessionID string) ([]UserQuestion, error) {
	if s.db == nil {
		return nil, errors.New("数据库未配置")
	}

	var questions []UserQuestion
	query := s.db.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, QuestionStatusPending)
	if sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}

	if err := query.Order("created_at DESC").Find(&questions).Error; err != nil {
		return nil, err
	}

	return questions, nil
}

// CancelQuestion 取消问题
func (s *UserQuestionService) CancelQuestion(ctx context.Context, questionID string) error {
	if s.db != nil {
		if err := s.db.Model(&UserQuestion{}).Where("id = ? AND status = ?", questionID, QuestionStatusPending).
			Update("status", QuestionStatusCancelled).Error; err != nil {
			return err
		}
	}

	// 通知等待者
	s.mu.RLock()
	waitChan, exists := s.waiters[questionID]
	s.mu.RUnlock()

	if exists {
		close(waitChan)
	}

	return nil
}
