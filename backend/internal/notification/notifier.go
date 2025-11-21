package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"text/template"
	"time"
)

// Notifier 通知器接口
type Notifier interface {
	Send(ctx context.Context, notification *Notification) error
}

// Notification 通知消息
type Notification struct {
	Type     string         // email, webhook, websocket
	TenantID string         // 租户 ID（websocket 通道需要）
	To       string         // 接收者（邮箱/URL）
	Subject  string         // 主题
	Body     string         // 内容
	Data     map[string]any // 附加数据
}

// MultiNotifier 多通道通知器
type MultiNotifier struct {
	email     *EmailNotifier
	webhook   *WebhookNotifier
	websocket *WebSocketNotifier
}

// NewMultiNotifier 创建多通道通知器
func NewMultiNotifier(emailConfig *EmailConfig, webhookConfig *WebhookConfig, hub *WebSocketHub) *MultiNotifier {
	return &MultiNotifier{
		email:     NewEmailNotifier(emailConfig),
		webhook:   NewWebhookNotifier(webhookConfig),
		websocket: NewWebSocketNotifier(hub),
	}
}

// Send 发送通知
func (m *MultiNotifier) Send(ctx context.Context, notification *Notification) error {
	var notifier Notifier

	switch notification.Type {
	case "email":
		notifier = m.email
	case "webhook":
		notifier = m.webhook
	case "websocket":
		notifier = m.websocket
	default:
		return fmt.Errorf("不支持的通知类型: %s", notification.Type)
	}

	if notifier == nil {
		return fmt.Errorf("通知器未配置: %s", notification.Type)
	}

	return notifier.Send(ctx, notification)
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	Username     string
	Password     string
	From         string
	FromName     string
	TemplatePath string
}

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	config    *EmailConfig
	templates *template.Template
}

// NewEmailNotifier 创建邮件通知器
func NewEmailNotifier(config *EmailConfig) *EmailNotifier {
	if config == nil {
		return nil
	}

	var templates *template.Template
	if config.TemplatePath != "" {
		templates, _ = template.ParseGlob(config.TemplatePath)
	}

	return &EmailNotifier{
		config:    config,
		templates: templates,
	}
}

// Send 发送邮件
func (e *EmailNotifier) Send(ctx context.Context, notification *Notification) error {
	if e.config == nil {
		return fmt.Errorf("邮件未配置")
	}

	// 构建邮件内容
	var body bytes.Buffer

	// 如果有模板，使用模板渲染
	if e.templates != nil && notification.Data != nil {
		if tmpl := e.templates.Lookup("approval_request.html"); tmpl != nil {
			if err := tmpl.Execute(&body, notification.Data); err != nil {
				return fmt.Errorf("渲染邮件模板失败: %w", err)
			}
		} else {
			body.WriteString(notification.Body)
		}
	} else {
		body.WriteString(notification.Body)
	}

	// 构建 MIME 邮件
	message := fmt.Sprintf(
		"From: %s <%s>\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n"+
			"\r\n"+
			"%s",
		e.config.FromName,
		e.config.From,
		notification.To,
		notification.Subject,
		body.String(),
	)

	// 发送邮件
	auth := smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.SMTPHost)
	addr := fmt.Sprintf("%s:%d", e.config.SMTPHost, e.config.SMTPPort)

	err := smtp.SendMail(
		addr,
		auth,
		e.config.From,
		[]string{notification.To},
		[]byte(message),
	)

	if err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}

	return nil
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	DefaultURL string
	Timeout    time.Duration
	Headers    map[string]string
}

// WebhookNotifier Webhook 通知器
type WebhookNotifier struct {
	config *WebhookConfig
	client *http.Client
}

// NewWebhookNotifier 创建 Webhook 通知器
func NewWebhookNotifier(config *WebhookConfig) *WebhookNotifier {
	if config == nil {
		config = &WebhookConfig{
			Timeout: 10 * time.Second,
		}
	}

	return &WebhookNotifier{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Send 发送 Webhook
func (w *WebhookNotifier) Send(ctx context.Context, notification *Notification) error {
	// 确定 URL
	url := notification.To
	if url == "" {
		url = w.config.DefaultURL
	}

	if url == "" {
		return fmt.Errorf("Webhook URL 未配置")
	}

	// 构建 Payload
	payload := map[string]any{
		"type":      notification.Type,
		"subject":   notification.Subject,
		"body":      notification.Body,
		"data":      notification.Data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 Webhook 负载失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("创建 Webhook 请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AgentFlowCreativeHub-Notifier/1.0")

	for key, value := range w.config.Headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送 Webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Webhook 返回错误状态: %d", resp.StatusCode)
	}

	return nil
}

// WebSocketNotifier WebSocket 通知器
type WebSocketNotifier struct {
	hub *WebSocketHub
}

// NewWebSocketNotifier 创建 WebSocket 通知器
func NewWebSocketNotifier(hub *WebSocketHub) *WebSocketNotifier {
	return &WebSocketNotifier{hub: hub}
}

// Send 发送 WebSocket 消息
func (ws *WebSocketNotifier) Send(ctx context.Context, notification *Notification) error {
	if ws == nil || ws.hub == nil {
		return fmt.Errorf("WebSocket hub 未配置")
	}
	if notification.TenantID == "" || notification.To == "" {
		return fmt.Errorf("WebSocket 通知缺少租户或用户信息")
	}
	payload := map[string]any{
		"type":      notification.Type,
		"subject":   notification.Subject,
		"body":      notification.Body,
		"data":      notification.Data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return ws.hub.SendToUser(notification.TenantID, notification.To, payload)
}
