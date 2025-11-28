package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"sync"
	"time"

	"backend/internal/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// EmailService 邮件服务
type EmailService struct {
	db        *gorm.DB
	config    *EmailServiceConfig
	templates map[string]*template.Template
	queue     chan *EmailTask
	mu        sync.RWMutex
	stopCh    chan struct{}
}

// EmailServiceConfig 邮件服务配置
type EmailServiceConfig struct {
	SMTPHost     string
	SMTPPort     int
	Username     string
	Password     string
	FromAddress  string
	FromName     string
	UseTLS       bool
	MaxRetries   int
	QueueSize    int
	Workers      int
}

// EmailTask 邮件任务
type EmailTask struct {
	ID          string
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        string
	HTMLBody    string
	TemplateID  string
	TemplateData map[string]any
	Priority    int // 0-低, 1-中, 2-高
	RetryCount  int
	CreatedAt   time.Time
	Callback    func(err error)
}

// EmailTemplate 邮件模板
type EmailTemplate struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(100)"`
	TenantID    string    `json:"tenant_id" gorm:"type:varchar(100);index"`
	Name        string    `json:"name" gorm:"type:varchar(255);not null"`
	Subject     string    `json:"subject" gorm:"type:varchar(500)"`
	HTMLContent string    `json:"html_content" gorm:"type:text"`
	TextContent string    `json:"text_content" gorm:"type:text"`
	Variables   []string  `json:"variables" gorm:"type:jsonb;serializer:json"`
	Category    string    `json:"category" gorm:"type:varchar(50);index"`
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (EmailTemplate) TableName() string {
	return "email_templates"
}

// EmailLog 邮件发送日志
type EmailLog struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	TenantID     string    `json:"tenant_id" gorm:"type:varchar(100);index"`
	TemplateID   string    `json:"template_id" gorm:"type:varchar(100);index"`
	ToAddresses  []string  `json:"to_addresses" gorm:"type:jsonb;serializer:json"`
	Subject      string    `json:"subject" gorm:"type:varchar(500)"`
	Status       string    `json:"status" gorm:"type:varchar(20);index"` // pending, sent, failed
	ErrorMessage string    `json:"error_message" gorm:"type:text"`
	RetryCount   int       `json:"retry_count"`
	SentAt       *time.Time `json:"sent_at"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
}

func (EmailLog) TableName() string {
	return "email_logs"
}

// NewEmailService 创建邮件服务
func NewEmailService(db *gorm.DB, config *EmailServiceConfig) *EmailService {
	if config == nil {
		config = &EmailServiceConfig{
			MaxRetries: 3,
			QueueSize:  1000,
			Workers:    3,
		}
	}

	if config.QueueSize <= 0 {
		config.QueueSize = 1000
	}
	if config.Workers <= 0 {
		config.Workers = 3
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	svc := &EmailService{
		db:        db,
		config:    config,
		templates: make(map[string]*template.Template),
		queue:     make(chan *EmailTask, config.QueueSize),
		stopCh:    make(chan struct{}),
	}

	// 自动迁移表
	if db != nil {
		db.AutoMigrate(&EmailTemplate{}, &EmailLog{})
	}

	// 启动工作协程
	for i := 0; i < config.Workers; i++ {
		go svc.worker(i)
	}

	return svc
}

// worker 邮件发送工作协程
func (s *EmailService) worker(id int) {
	for {
		select {
		case <-s.stopCh:
			return
		case task := <-s.queue:
			err := s.sendEmail(task)
			if err != nil && task.RetryCount < s.config.MaxRetries {
				task.RetryCount++
				// 指数退避重试
				time.AfterFunc(time.Duration(task.RetryCount*task.RetryCount)*time.Second, func() {
					s.queue <- task
				})
			}
			if task.Callback != nil {
				task.Callback(err)
			}
		}
	}
}

// SendAsync 异步发送邮件
func (s *EmailService) SendAsync(task *EmailTask) error {
	if task.ID == "" {
		task.ID = fmt.Sprintf("email_%d", time.Now().UnixNano())
	}
	task.CreatedAt = time.Now()

	select {
	case s.queue <- task:
		return nil
	default:
		return fmt.Errorf("邮件队列已满")
	}
}

// SendSync 同步发送邮件
func (s *EmailService) SendSync(ctx context.Context, task *EmailTask) error {
	return s.sendEmail(task)
}

// sendEmail 发送邮件
func (s *EmailService) sendEmail(task *EmailTask) error {
	// 使用模板渲染
	body := task.HTMLBody
	if body == "" {
		body = task.Body
	}

	if task.TemplateID != "" && task.TemplateData != nil {
		rendered, err := s.RenderTemplate(task.TemplateID, task.TemplateData)
		if err != nil {
			logger.Warn("渲染邮件模板失败", zap.String("template_id", task.TemplateID), zap.Error(err))
		} else {
			body = rendered
		}
	}

	// 构建MIME消息
	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", s.config.FromName, s.config.FromAddress))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", task.To[0]))
	
	if len(task.CC) > 0 {
		for _, cc := range task.CC {
			msg.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
		}
	}
	
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", task.Subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// 发送邮件
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
	
	var err error
	if s.config.UseTLS {
		err = s.sendWithTLS(addr, task.To, msg.Bytes())
	} else {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.SMTPHost)
		err = smtp.SendMail(addr, auth, s.config.FromAddress, task.To, msg.Bytes())
	}

	// 记录日志
	if s.db != nil {
		log := &EmailLog{
			TemplateID:  task.TemplateID,
			ToAddresses: task.To,
			Subject:     task.Subject,
			RetryCount:  task.RetryCount,
			CreatedAt:   time.Now(),
		}
		if err != nil {
			log.Status = "failed"
			log.ErrorMessage = err.Error()
		} else {
			log.Status = "sent"
			now := time.Now()
			log.SentAt = &now
		}
		s.db.Create(log)
	}

	return err
}

// sendWithTLS 使用TLS发送邮件
func (s *EmailService) sendWithTLS(addr string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: s.config.SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()

	// 认证
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP认证失败: %w", err)
	}

	// 发送
	if err := client.Mail(s.config.FromAddress); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("设置收件人失败: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("关闭数据写入器失败: %w", err)
	}

	return client.Quit()
}

// RegisterTemplate 注册邮件模板
func (s *EmailService) RegisterTemplate(tmpl *EmailTemplate) error {
	if tmpl.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}

	// 解析模板
	t, err := template.New(tmpl.ID).Parse(tmpl.HTMLContent)
	if err != nil {
		return fmt.Errorf("解析模板失败: %w", err)
	}

	s.mu.Lock()
	s.templates[tmpl.ID] = t
	s.mu.Unlock()

	// 保存到数据库
	if s.db != nil {
		return s.db.Save(tmpl).Error
	}

	return nil
}

// RenderTemplate 渲染模板
func (s *EmailService) RenderTemplate(templateID string, data map[string]any) (string, error) {
	s.mu.RLock()
	tmpl, ok := s.templates[templateID]
	s.mu.RUnlock()

	if !ok {
		// 尝试从数据库加载
		if s.db != nil {
			var dbTmpl EmailTemplate
			if err := s.db.First(&dbTmpl, "id = ?", templateID).Error; err == nil {
				t, err := template.New(templateID).Parse(dbTmpl.HTMLContent)
				if err != nil {
					return "", fmt.Errorf("解析模板失败: %w", err)
				}
				s.mu.Lock()
				s.templates[templateID] = t
				s.mu.Unlock()
				tmpl = t
			}
		}
	}

	if tmpl == nil {
		return "", fmt.Errorf("模板不存在: %s", templateID)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}

	return buf.String(), nil
}

// GetLogs 获取邮件发送日志
func (s *EmailService) GetLogs(ctx context.Context, tenantID string, page, pageSize int) ([]EmailLog, int64, error) {
	var logs []EmailLog
	var total int64

	query := s.db.WithContext(ctx).Model(&EmailLog{})
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetQueueStats 获取队列统计
func (s *EmailService) GetQueueStats() map[string]int {
	return map[string]int{
		"queue_size":     len(s.queue),
		"queue_capacity": cap(s.queue),
	}
}

// Stop 停止邮件服务
func (s *EmailService) Stop() {
	close(s.stopCh)
}

// 预置邮件模板
var BuiltinTemplates = []EmailTemplate{
	{
		ID:       "approval_request",
		Name:     "审批请求通知",
		Category: "approval",
		Subject:  "您有新的审批请求待处理",
		HTMLContent: `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
  <h2>审批请求通知</h2>
  <p>您好，{{.UserName}}：</p>
  <p>您有一个新的审批请求需要处理：</p>
  <ul>
    <li><strong>工作流：</strong>{{.WorkflowName}}</li>
    <li><strong>请求类型：</strong>{{.RequestType}}</li>
    <li><strong>提交时间：</strong>{{.SubmittedAt}}</li>
  </ul>
  <p>请及时登录系统处理。</p>
  <p><a href="{{.ApprovalURL}}" style="background:#007bff;color:#fff;padding:10px 20px;text-decoration:none;border-radius:5px;">立即处理</a></p>
</body>
</html>`,
		Variables: []string{"UserName", "WorkflowName", "RequestType", "SubmittedAt", "ApprovalURL"},
	},
	{
		ID:       "workflow_completed",
		Name:     "工作流完成通知",
		Category: "workflow",
		Subject:  "工作流执行完成",
		HTMLContent: `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
  <h2>工作流执行完成</h2>
  <p>您好，{{.UserName}}：</p>
  <p>您的工作流已执行完成：</p>
  <ul>
    <li><strong>工作流：</strong>{{.WorkflowName}}</li>
    <li><strong>状态：</strong>{{.Status}}</li>
    <li><strong>耗时：</strong>{{.Duration}}</li>
  </ul>
  <p><a href="{{.ResultURL}}">查看详情</a></p>
</body>
</html>`,
		Variables: []string{"UserName", "WorkflowName", "Status", "Duration", "ResultURL"},
	},
	{
		ID:       "quota_warning",
		Name:     "配额预警通知",
		Category: "quota",
		Subject:  "配额使用预警",
		HTMLContent: `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
  <h2>配额使用预警</h2>
  <p>您好：</p>
  <p>您的 {{.QuotaType}} 使用量已达到 {{.UsagePercent}}%，请及时处理。</p>
  <ul>
    <li><strong>当前用量：</strong>{{.CurrentUsage}}</li>
    <li><strong>配额上限：</strong>{{.QuotaLimit}}</li>
  </ul>
</body>
</html>`,
		Variables: []string{"QuotaType", "UsagePercent", "CurrentUsage", "QuotaLimit"},
	},
}
