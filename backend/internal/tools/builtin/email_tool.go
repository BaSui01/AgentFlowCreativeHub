package builtin

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// EmailTool 邮件发送工具
type EmailTool struct {
	config EmailConfig
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	Username     string
	Password     string
	FromAddress  string
	FromName     string
	UseTLS       bool
	AllowedTo    []string // 允许发送的收件人域名白名单
	MaxRecipient int      // 最大收件人数
}

// NewEmailTool 创建邮件工具
func NewEmailTool(config EmailConfig) *EmailTool {
	if config.MaxRecipient == 0 {
		config.MaxRecipient = 10
	}
	return &EmailTool{config: config}
}

func (t *EmailTool) Name() string {
	return "email_send"
}

func (t *EmailTool) Description() string {
	return "发送邮件通知，支持 HTML 格式和附件"
}

func (t *EmailTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"to": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "收件人邮箱列表",
			},
			"cc": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "抄送邮箱列表（可选）",
			},
			"subject": map[string]any{
				"type":        "string",
				"description": "邮件主题",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "邮件正文",
			},
			"html": map[string]any{
				"type":        "boolean",
				"description": "是否为 HTML 格式（默认 false）",
				"default":     false,
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"high", "normal", "low"},
				"description": "邮件优先级",
				"default":     "normal",
			},
		},
		"required": []string{"to", "subject", "body"},
	}
}

func (t *EmailTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 解析参数
	toList, err := t.parseStringArray(input["to"])
	if err != nil || len(toList) == 0 {
		return nil, fmt.Errorf("to is required and must be an array of email addresses")
	}

	if len(toList) > t.config.MaxRecipient {
		return nil, fmt.Errorf("too many recipients, max %d", t.config.MaxRecipient)
	}

	subject, _ := input["subject"].(string)
	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}

	body, _ := input["body"].(string)
	if body == "" {
		return nil, fmt.Errorf("body is required")
	}

	ccList, _ := t.parseStringArray(input["cc"])
	isHTML, _ := input["html"].(bool)
	priority, _ := input["priority"].(string)
	if priority == "" {
		priority = "normal"
	}

	// 验证收件人
	for _, to := range toList {
		if !t.isAllowedRecipient(to) {
			return nil, fmt.Errorf("recipient %s is not in allowed list", to)
		}
	}

	// 构建邮件
	message := t.buildMessage(toList, ccList, subject, body, isHTML, priority)

	// 发送邮件
	allRecipients := append(toList, ccList...)
	err = t.sendMail(allRecipients, message)
	if err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	return map[string]any{
		"success":    true,
		"message":    "email sent successfully",
		"recipients": len(allRecipients),
		"subject":    subject,
	}, nil
}

func (t *EmailTool) parseStringArray(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}

	switch arr := v.(type) {
	case []string:
		return arr, nil
	case []any:
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("invalid array type")
	}
}

func (t *EmailTool) isAllowedRecipient(email string) bool {
	if len(t.config.AllowedTo) == 0 {
		return true // 无白名单限制
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])

	for _, allowed := range t.config.AllowedTo {
		if strings.ToLower(allowed) == domain {
			return true
		}
	}
	return false
}

func (t *EmailTool) buildMessage(to, cc []string, subject, body string, isHTML bool, priority string) []byte {
	var msg strings.Builder

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", t.config.FromName, t.config.FromAddress))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	if len(cc) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(cc, ", ")))
	}
	msg.WriteString(fmt.Sprintf("Subject: =?UTF-8?B?%s?=\r\n", t.base64Encode(subject)))

	// Priority
	switch priority {
	case "high":
		msg.WriteString("X-Priority: 1\r\n")
		msg.WriteString("Importance: High\r\n")
	case "low":
		msg.WriteString("X-Priority: 5\r\n")
		msg.WriteString("Importance: Low\r\n")
	}

	// Content-Type
	if isHTML {
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("\r\n")

	// Body
	msg.WriteString(body)

	return []byte(msg.String())
}

func (t *EmailTool) base64Encode(s string) string {
	// 简单 base64 编码
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	input := []byte(s)
	var result strings.Builder

	for i := 0; i < len(input); i += 3 {
		var n uint32
		remaining := len(input) - i
		if remaining >= 3 {
			n = uint32(input[i])<<16 | uint32(input[i+1])<<8 | uint32(input[i+2])
			result.WriteByte(base64Chars[(n>>18)&0x3F])
			result.WriteByte(base64Chars[(n>>12)&0x3F])
			result.WriteByte(base64Chars[(n>>6)&0x3F])
			result.WriteByte(base64Chars[n&0x3F])
		} else if remaining == 2 {
			n = uint32(input[i])<<16 | uint32(input[i+1])<<8
			result.WriteByte(base64Chars[(n>>18)&0x3F])
			result.WriteByte(base64Chars[(n>>12)&0x3F])
			result.WriteByte(base64Chars[(n>>6)&0x3F])
			result.WriteByte('=')
		} else {
			n = uint32(input[i]) << 16
			result.WriteByte(base64Chars[(n>>18)&0x3F])
			result.WriteByte(base64Chars[(n>>12)&0x3F])
			result.WriteByte('=')
			result.WriteByte('=')
		}
	}

	return result.String()
}

func (t *EmailTool) sendMail(recipients []string, message []byte) error {
	addr := fmt.Sprintf("%s:%d", t.config.SMTPHost, t.config.SMTPPort)

	var client *smtp.Client
	var err error

	if t.config.UseTLS {
		// TLS 连接
		tlsConfig := &tls.Config{
			ServerName: t.config.SMTPHost,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, t.config.SMTPHost)
		if err != nil {
			return err
		}
	} else {
		// 普通连接
		client, err = smtp.Dial(addr)
		if err != nil {
			return err
		}
	}
	defer client.Close()

	// 认证
	if t.config.Username != "" && t.config.Password != "" {
		auth := smtp.PlainAuth("", t.config.Username, t.config.Password, t.config.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	// 发送
	if err := client.Mail(t.config.FromAddress); err != nil {
		return err
	}

	for _, to := range recipients {
		if err := client.Rcpt(to); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(message)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}
