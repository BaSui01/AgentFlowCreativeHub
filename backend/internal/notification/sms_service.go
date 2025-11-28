package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"backend/internal/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SMSProvider 短信服务商类型
type SMSProvider string

const (
	SMSProviderAliyun  SMSProvider = "aliyun"  // 阿里云短信
	SMSProviderTencent SMSProvider = "tencent" // 腾讯云短信
)

// SMSService 短信服务
type SMSService struct {
	db        *gorm.DB
	config    *SMSServiceConfig
	templates map[string]*SMSTemplate
	queue     chan *SMSTask
	mu        sync.RWMutex
	stopCh    chan struct{}
	client    *http.Client
}

// SMSServiceConfig 短信服务配置
type SMSServiceConfig struct {
	Provider SMSProvider

	// 阿里云配置
	AliyunAccessKeyID     string
	AliyunAccessKeySecret string
	AliyunSignName        string
	AliyunEndpoint        string // 默认 dysmsapi.aliyuncs.com

	// 腾讯云配置
	TencentSecretID    string
	TencentSecretKey   string
	TencentSmsSdkAppId string
	TencentSignName    string
	TencentRegion      string // 默认 ap-guangzhou

	// 通用配置
	MaxRetries int
	QueueSize  int
	Workers    int
	Timeout    time.Duration
}

// SMSTask 短信任务
type SMSTask struct {
	ID           string
	PhoneNumbers []string       // 手机号列表
	TemplateID   string         // 模板ID
	TemplateData map[string]any // 模板参数
	SignName     string         // 签名（可覆盖默认签名）
	Priority     int            // 0-低, 1-中, 2-高
	RetryCount   int
	CreatedAt    time.Time
	Callback     func(err error)
}

// SMSTemplate 短信模板
type SMSTemplate struct {
	ID              string    `json:"id" gorm:"primaryKey;type:varchar(100)"`
	TenantID        string    `json:"tenant_id" gorm:"type:varchar(100);index"`
	Name            string    `json:"name" gorm:"type:varchar(255);not null"`
	ProviderCode    string    `json:"provider_code" gorm:"type:varchar(100)"` // 云服务商模板Code
	Content         string    `json:"content" gorm:"type:text"`               // 模板内容（仅展示用）
	Variables       []string  `json:"variables" gorm:"type:jsonb;serializer:json"`
	Category        string    `json:"category" gorm:"type:varchar(50);index"` // 验证码/通知/营销
	Provider        string    `json:"provider" gorm:"type:varchar(20)"`       // aliyun/tencent
	IsActive        bool      `json:"is_active" gorm:"default:true"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (SMSTemplate) TableName() string {
	return "sms_templates"
}

// SMSLog 短信发送日志
type SMSLog struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	TenantID     string     `json:"tenant_id" gorm:"type:varchar(100);index"`
	TemplateID   string     `json:"template_id" gorm:"type:varchar(100);index"`
	PhoneNumbers []string   `json:"phone_numbers" gorm:"type:jsonb;serializer:json"`
	TemplateData string     `json:"template_data" gorm:"type:text"` // JSON格式
	SignName     string     `json:"sign_name" gorm:"type:varchar(100)"`
	Provider     string     `json:"provider" gorm:"type:varchar(20)"`
	BizID        string     `json:"biz_id" gorm:"type:varchar(100)"` // 云服务商返回的业务ID
	Status       string     `json:"status" gorm:"type:varchar(20);index"`
	ErrorCode    string     `json:"error_code" gorm:"type:varchar(50)"`
	ErrorMessage string     `json:"error_message" gorm:"type:text"`
	RetryCount   int        `json:"retry_count"`
	SentAt       *time.Time `json:"sent_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"index"`
}

func (SMSLog) TableName() string {
	return "sms_logs"
}

// NewSMSService 创建短信服务
func NewSMSService(db *gorm.DB, config *SMSServiceConfig) *SMSService {
	if config == nil {
		config = &SMSServiceConfig{
			Provider:   SMSProviderAliyun,
			MaxRetries: 3,
			QueueSize:  1000,
			Workers:    3,
			Timeout:    10 * time.Second,
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
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.AliyunEndpoint == "" {
		config.AliyunEndpoint = "dysmsapi.aliyuncs.com"
	}
	if config.TencentRegion == "" {
		config.TencentRegion = "ap-guangzhou"
	}

	svc := &SMSService{
		db:        db,
		config:    config,
		templates: make(map[string]*SMSTemplate),
		queue:     make(chan *SMSTask, config.QueueSize),
		stopCh:    make(chan struct{}),
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}

	// 自动迁移表
	if db != nil {
		db.AutoMigrate(&SMSTemplate{}, &SMSLog{})
	}

	// 启动工作协程
	for i := 0; i < config.Workers; i++ {
		go svc.worker(i)
	}

	return svc
}

// worker 短信发送工作协程
func (s *SMSService) worker(id int) {
	for {
		select {
		case <-s.stopCh:
			return
		case task := <-s.queue:
			err := s.sendSMS(task)
			if err != nil && task.RetryCount < s.config.MaxRetries {
				task.RetryCount++
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

// SendAsync 异步发送短信
func (s *SMSService) SendAsync(task *SMSTask) error {
	if task.ID == "" {
		task.ID = fmt.Sprintf("sms_%d", time.Now().UnixNano())
	}
	task.CreatedAt = time.Now()

	select {
	case s.queue <- task:
		return nil
	default:
		return fmt.Errorf("短信队列已满")
	}
}

// SendSync 同步发送短信
func (s *SMSService) SendSync(ctx context.Context, task *SMSTask) error {
	return s.sendSMS(task)
}

// sendSMS 发送短信
func (s *SMSService) sendSMS(task *SMSTask) error {
	var err error
	var bizID string

	switch s.config.Provider {
	case SMSProviderAliyun:
		bizID, err = s.sendAliyunSMS(task)
	case SMSProviderTencent:
		bizID, err = s.sendTencentSMS(task)
	default:
		err = fmt.Errorf("不支持的短信服务商: %s", s.config.Provider)
	}

	// 记录日志
	s.logSendResult(task, bizID, err)

	return err
}

// sendAliyunSMS 阿里云短信发送
func (s *SMSService) sendAliyunSMS(task *SMSTask) (string, error) {
	signName := task.SignName
	if signName == "" {
		signName = s.config.AliyunSignName
	}

	// 获取模板ProviderCode
	templateCode := task.TemplateID
	s.mu.RLock()
	if tmpl, ok := s.templates[task.TemplateID]; ok && tmpl.ProviderCode != "" {
		templateCode = tmpl.ProviderCode
	}
	s.mu.RUnlock()

	// 构建参数
	templateParam, _ := json.Marshal(task.TemplateData)

	params := map[string]string{
		"AccessKeyId":      s.config.AliyunAccessKeyID,
		"Action":           "SendSms",
		"Format":           "JSON",
		"PhoneNumbers":     strings.Join(task.PhoneNumbers, ","),
		"SignName":         signName,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   fmt.Sprintf("%d", time.Now().UnixNano()),
		"SignatureVersion": "1.0",
		"TemplateCode":     templateCode,
		"TemplateParam":    string(templateParam),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2017-05-25",
	}

	// 签名
	signature := s.aliyunSign(params)
	params["Signature"] = signature

	// 构建请求URL
	urlStr := fmt.Sprintf("https://%s/", s.config.AliyunEndpoint)
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	resp, err := s.client.PostForm(urlStr, values)
	if err != nil {
		return "", fmt.Errorf("阿里云短信请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		BizId     string `json:"BizId"`
		RequestId string `json:"RequestId"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析阿里云响应失败: %w", err)
	}

	if result.Code != "OK" {
		return "", fmt.Errorf("阿里云短信发送失败[%s]: %s", result.Code, result.Message)
	}

	return result.BizId, nil
}

// aliyunSign 阿里云签名
func (s *SMSService) aliyunSign(params map[string]string) string {
	// 排序参数
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建待签名字符串
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", specialURLEncode(k), specialURLEncode(params[k])))
	}
	canonicalizedQueryString := strings.Join(pairs, "&")
	stringToSign := "POST&%2F&" + specialURLEncode(canonicalizedQueryString)

	// HMAC-SHA1签名
	mac := hmac.New(sha256.New, []byte(s.config.AliyunAccessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// sendTencentSMS 腾讯云短信发送
func (s *SMSService) sendTencentSMS(task *SMSTask) (string, error) {
	signName := task.SignName
	if signName == "" {
		signName = s.config.TencentSignName
	}

	// 获取模板ProviderCode
	templateID := task.TemplateID
	s.mu.RLock()
	if tmpl, ok := s.templates[task.TemplateID]; ok && tmpl.ProviderCode != "" {
		templateID = tmpl.ProviderCode
	}
	s.mu.RUnlock()

	// 构建请求体
	templateParams := make([]string, 0)
	for _, v := range task.TemplateData {
		templateParams = append(templateParams, fmt.Sprintf("%v", v))
	}

	phoneNumberSet := make([]string, len(task.PhoneNumbers))
	for i, phone := range task.PhoneNumbers {
		if !strings.HasPrefix(phone, "+86") {
			phoneNumberSet[i] = "+86" + phone
		} else {
			phoneNumberSet[i] = phone
		}
	}

	requestBody := map[string]any{
		"PhoneNumberSet":   phoneNumberSet,
		"SmsSdkAppId":      s.config.TencentSmsSdkAppId,
		"SignName":         signName,
		"TemplateId":       templateID,
		"TemplateParamSet": templateParams,
	}

	bodyBytes, _ := json.Marshal(requestBody)

	// 构建请求
	host := "sms.tencentcloudapi.com"
	service := "sms"
	action := "SendSms"
	version := "2021-01-11"
	timestamp := time.Now().Unix()

	// 构建签名
	authorization := s.tencentSign(host, service, action, version, timestamp, bodyBytes)

	req, err := http.NewRequest("POST", "https://"+host, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("创建腾讯云请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Region", s.config.TencentRegion)
	req.Header.Set("Authorization", authorization)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("腾讯云短信请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Response struct {
			SendStatusSet []struct {
				SerialNo    string `json:"SerialNo"`
				PhoneNumber string `json:"PhoneNumber"`
				Fee         int    `json:"Fee"`
				SessionContext string `json:"SessionContext"`
				Code        string `json:"Code"`
				Message     string `json:"Message"`
			} `json:"SendStatusSet"`
			RequestId string `json:"RequestId"`
			Error     struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析腾讯云响应失败: %w", err)
	}

	if result.Response.Error.Code != "" {
		return "", fmt.Errorf("腾讯云短信发送失败[%s]: %s", result.Response.Error.Code, result.Response.Error.Message)
	}

	// 检查每个号码的发送状态
	var serialNo string
	for _, status := range result.Response.SendStatusSet {
		if status.Code != "Ok" {
			logger.Warn("腾讯云短信发送部分失败",
				zap.String("phone", status.PhoneNumber),
				zap.String("code", status.Code),
				zap.String("message", status.Message))
		}
		if serialNo == "" {
			serialNo = status.SerialNo
		}
	}

	return serialNo, nil
}

// tencentSign 腾讯云签名 (TC3-HMAC-SHA256)
func (s *SMSService) tencentSign(host, service, action, version string, timestamp int64, payload []byte) string {
	algorithm := "TC3-HMAC-SHA256"
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)

	// 1. 拼接规范请求串
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json; charset=utf-8\nhost:%s\nx-tc-action:%s\n",
		host, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"

	hashedPayload := sha256Hex(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod, canonicalURI, canonicalQueryString, canonicalHeaders, signedHeaders, hashedPayload)

	// 2. 拼接待签名字符串
	hashedCanonicalRequest := sha256Hex([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm, timestamp, credentialScope, hashedCanonicalRequest)

	// 3. 计算签名
	secretDate := hmacSHA256([]byte("TC3"+s.config.TencentSecretKey), date)
	secretService := hmacSHA256(secretDate, service)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	// 4. 拼接Authorization
	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, s.config.TencentSecretID, credentialScope, signedHeaders, signature)
}

// logSendResult 记录发送结果
func (s *SMSService) logSendResult(task *SMSTask, bizID string, err error) {
	if s.db == nil {
		return
	}

	templateData, _ := json.Marshal(task.TemplateData)
	log := &SMSLog{
		TemplateID:   task.TemplateID,
		PhoneNumbers: task.PhoneNumbers,
		TemplateData: string(templateData),
		SignName:     task.SignName,
		Provider:     string(s.config.Provider),
		BizID:        bizID,
		RetryCount:   task.RetryCount,
		CreatedAt:    time.Now(),
	}

	if err != nil {
		log.Status = "failed"
		log.ErrorMessage = err.Error()
	} else {
		log.Status = "sent"
		now := time.Now()
		log.SentAt = &now
	}

	if dbErr := s.db.Create(log).Error; dbErr != nil {
		logger.Error("保存短信日志失败", zap.Error(dbErr))
	}
}

// RegisterTemplate 注册短信模板
func (s *SMSService) RegisterTemplate(tmpl *SMSTemplate) error {
	if tmpl.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}

	s.mu.Lock()
	s.templates[tmpl.ID] = tmpl
	s.mu.Unlock()

	if s.db != nil {
		return s.db.Save(tmpl).Error
	}

	return nil
}

// GetTemplate 获取模板
func (s *SMSService) GetTemplate(templateID string) (*SMSTemplate, error) {
	s.mu.RLock()
	tmpl, ok := s.templates[templateID]
	s.mu.RUnlock()

	if ok {
		return tmpl, nil
	}

	if s.db != nil {
		var dbTmpl SMSTemplate
		if err := s.db.First(&dbTmpl, "id = ?", templateID).Error; err == nil {
			s.mu.Lock()
			s.templates[templateID] = &dbTmpl
			s.mu.Unlock()
			return &dbTmpl, nil
		}
	}

	return nil, fmt.Errorf("模板不存在: %s", templateID)
}

// ListTemplates 列出模板
func (s *SMSService) ListTemplates(ctx context.Context, tenantID string, page, pageSize int) ([]SMSTemplate, int64, error) {
	var templates []SMSTemplate
	var total int64

	query := s.db.WithContext(ctx).Model(&SMSTemplate{})
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&templates).Error; err != nil {
		return nil, 0, err
	}

	return templates, total, nil
}

// GetLogs 获取短信发送日志
func (s *SMSService) GetLogs(ctx context.Context, tenantID string, page, pageSize int) ([]SMSLog, int64, error) {
	var logs []SMSLog
	var total int64

	query := s.db.WithContext(ctx).Model(&SMSLog{})
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

// GetLogsByPhone 按手机号查询日志
func (s *SMSService) GetLogsByPhone(ctx context.Context, phone string, page, pageSize int) ([]SMSLog, int64, error) {
	var logs []SMSLog
	var total int64

	query := s.db.WithContext(ctx).Model(&SMSLog{}).
		Where("phone_numbers @> ?", fmt.Sprintf(`["%s"]`, phone))

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
func (s *SMSService) GetQueueStats() map[string]int {
	return map[string]int{
		"queue_size":     len(s.queue),
		"queue_capacity": cap(s.queue),
	}
}

// Stop 停止短信服务
func (s *SMSService) Stop() {
	close(s.stopCh)
}

// 辅助函数
func specialURLEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// 预置短信模板
var BuiltinSMSTemplates = []SMSTemplate{
	{
		ID:           "verification_code",
		Name:         "验证码",
		ProviderCode: "", // 需要配置云服务商模板Code
		Content:      "您的验证码是${code}，${expire}分钟内有效。",
		Variables:    []string{"code", "expire"},
		Category:     "verification",
		IsActive:     true,
	},
	{
		ID:           "login_alert",
		Name:         "登录提醒",
		ProviderCode: "",
		Content:      "您的账号于${time}在${location}登录，如非本人操作请立即修改密码。",
		Variables:    []string{"time", "location"},
		Category:     "notification",
		IsActive:     true,
	},
	{
		ID:           "approval_notify",
		Name:         "审批通知",
		ProviderCode: "",
		Content:      "您有新的审批待处理：${workflow_name}，请及时登录系统处理。",
		Variables:    []string{"workflow_name"},
		Category:     "notification",
		IsActive:     true,
	},
	{
		ID:           "workflow_complete",
		Name:         "工作流完成",
		ProviderCode: "",
		Content:      "工作流「${workflow_name}」执行${status}，耗时${duration}。",
		Variables:    []string{"workflow_name", "status", "duration"},
		Category:     "notification",
		IsActive:     true,
	},
	{
		ID:           "quota_warning",
		Name:         "配额预警",
		ProviderCode: "",
		Content:      "您的${quota_type}使用量已达${percent}%，请及时处理。",
		Variables:    []string{"quota_type", "percent"},
		Category:     "notification",
		IsActive:     true,
	},
}
