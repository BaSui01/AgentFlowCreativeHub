package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// WebhookService Webhook 通知服务
type WebhookService struct {
	client    *http.Client
	endpoints map[string]*WebhookEndpoint
	mu        sync.RWMutex
	queue     chan *WebhookEvent
	maxRetry  int
}

// WebhookEndpoint Webhook 端点配置
type WebhookEndpoint struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Secret    string            `json:"secret,omitempty"` // HMAC 签名密钥
	Headers   map[string]string `json:"headers,omitempty"`
	Events    []string          `json:"events"`    // 订阅的事件类型
	Active    bool              `json:"active"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// WebhookEvent Webhook 事件
type WebhookEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Timestamp   time.Time      `json:"timestamp"`
	Payload     map[string]any `json:"payload"`
	EndpointID  string         `json:"-"`
	Signature   string         `json:"signature,omitempty"`
	RetryCount  int            `json:"-"`
	NextRetryAt time.Time      `json:"-"`
}

// WebhookDelivery 投递记录
type WebhookDelivery struct {
	ID           string        `json:"id"`
	EndpointID   string        `json:"endpoint_id"`
	EventID      string        `json:"event_id"`
	EventType    string        `json:"event_type"`
	URL          string        `json:"url"`
	RequestBody  string        `json:"request_body"`
	ResponseCode int           `json:"response_code"`
	ResponseBody string        `json:"response_body"`
	Duration     time.Duration `json:"duration"`
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}

// NewWebhookService 创建 Webhook 服务
func NewWebhookService(maxRetry int) *WebhookService {
	if maxRetry <= 0 {
		maxRetry = 3
	}

	ws := &WebhookService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoints: make(map[string]*WebhookEndpoint),
		queue:     make(chan *WebhookEvent, 1000),
		maxRetry:  maxRetry,
	}

	// 启动后台处理器
	go ws.processQueue()

	return ws
}

// RegisterEndpoint 注册 Webhook 端点
func (s *WebhookService) RegisterEndpoint(endpoint *WebhookEndpoint) error {
	if endpoint.URL == "" {
		return fmt.Errorf("url is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if endpoint.ID == "" {
		endpoint.ID = fmt.Sprintf("wh_%d", time.Now().UnixNano())
	}
	endpoint.CreatedAt = time.Now()
	endpoint.UpdatedAt = time.Now()
	endpoint.Active = true

	s.endpoints[endpoint.ID] = endpoint
	return nil
}

// UpdateEndpoint 更新端点
func (s *WebhookService) UpdateEndpoint(id string, update *WebhookEndpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.endpoints[id]
	if !ok {
		return fmt.Errorf("endpoint not found: %s", id)
	}

	if update.URL != "" {
		existing.URL = update.URL
	}
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Secret != "" {
		existing.Secret = update.Secret
	}
	if update.Headers != nil {
		existing.Headers = update.Headers
	}
	if update.Events != nil {
		existing.Events = update.Events
	}
	existing.UpdatedAt = time.Now()

	return nil
}

// DeleteEndpoint 删除端点
func (s *WebhookService) DeleteEndpoint(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.endpoints[id]; !ok {
		return fmt.Errorf("endpoint not found: %s", id)
	}

	delete(s.endpoints, id)
	return nil
}

// SetEndpointActive 设置端点活动状态
func (s *WebhookService) SetEndpointActive(id string, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ep, ok := s.endpoints[id]
	if !ok {
		return fmt.Errorf("endpoint not found: %s", id)
	}

	ep.Active = active
	ep.UpdatedAt = time.Now()
	return nil
}

// ListEndpoints 列出所有端点
func (s *WebhookService) ListEndpoints() []*WebhookEndpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WebhookEndpoint, 0, len(s.endpoints))
	for _, ep := range s.endpoints {
		result = append(result, ep)
	}
	return result
}

// Emit 发送事件
func (s *WebhookService) Emit(ctx context.Context, eventType string, payload map[string]any) error {
	s.mu.RLock()
	endpoints := make([]*WebhookEndpoint, 0)
	for _, ep := range s.endpoints {
		if ep.Active && s.shouldDeliver(ep, eventType) {
			endpoints = append(endpoints, ep)
		}
	}
	s.mu.RUnlock()

	if len(endpoints) == 0 {
		return nil
	}

	event := &WebhookEvent{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// 向每个端点发送
	for _, ep := range endpoints {
		evt := *event
		evt.EndpointID = ep.ID
		s.queue <- &evt
	}

	return nil
}

// EmitSync 同步发送事件（等待响应）
func (s *WebhookService) EmitSync(ctx context.Context, eventType string, payload map[string]any) ([]*WebhookDelivery, error) {
	s.mu.RLock()
	endpoints := make([]*WebhookEndpoint, 0)
	for _, ep := range s.endpoints {
		if ep.Active && s.shouldDeliver(ep, eventType) {
			endpoints = append(endpoints, ep)
		}
	}
	s.mu.RUnlock()

	if len(endpoints) == 0 {
		return nil, nil
	}

	event := &WebhookEvent{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	deliveries := make([]*WebhookDelivery, 0, len(endpoints))
	for _, ep := range endpoints {
		evt := *event
		evt.EndpointID = ep.ID
		delivery := s.deliver(ctx, ep, &evt)
		deliveries = append(deliveries, delivery)
	}

	return deliveries, nil
}

func (s *WebhookService) shouldDeliver(ep *WebhookEndpoint, eventType string) bool {
	if len(ep.Events) == 0 {
		return true // 空列表表示订阅所有事件
	}

	for _, e := range ep.Events {
		if e == "*" || e == eventType {
			return true
		}
	}
	return false
}

func (s *WebhookService) processQueue() {
	for event := range s.queue {
		s.mu.RLock()
		ep, ok := s.endpoints[event.EndpointID]
		s.mu.RUnlock()

		if !ok || !ep.Active {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		delivery := s.deliver(ctx, ep, event)
		cancel()

		// 失败重试
		if !delivery.Success && event.RetryCount < s.maxRetry {
			event.RetryCount++
			event.NextRetryAt = time.Now().Add(s.retryDelay(event.RetryCount))

			go func(evt *WebhookEvent) {
				time.Sleep(time.Until(evt.NextRetryAt))
				s.queue <- evt
			}(event)
		}
	}
}

func (s *WebhookService) retryDelay(attempt int) time.Duration {
	// 指数退避：1s, 2s, 4s, 8s...
	return time.Duration(1<<(attempt-1)) * time.Second
}

func (s *WebhookService) deliver(ctx context.Context, ep *WebhookEndpoint, event *WebhookEvent) *WebhookDelivery {
	delivery := &WebhookDelivery{
		ID:         fmt.Sprintf("dlv_%d", time.Now().UnixNano()),
		EndpointID: ep.ID,
		EventID:    event.ID,
		EventType:  event.Type,
		URL:        ep.URL,
		CreatedAt:  time.Now(),
	}

	// 构建请求体
	body, err := json.Marshal(event)
	if err != nil {
		delivery.Error = err.Error()
		return delivery
	}
	delivery.RequestBody = string(body)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", ep.URL, bytes.NewReader(body))
	if err != nil {
		delivery.Error = err.Error()
		return delivery
	}

	// 设置头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AgentFlow-Webhook/1.0")
	req.Header.Set("X-Webhook-ID", event.ID)
	req.Header.Set("X-Webhook-Event", event.Type)
	req.Header.Set("X-Webhook-Timestamp", event.Timestamp.Format(time.RFC3339))

	// 自定义头
	for k, v := range ep.Headers {
		req.Header.Set(k, v)
	}

	// HMAC 签名
	if ep.Secret != "" {
		signature := s.sign(body, ep.Secret)
		req.Header.Set("X-Webhook-Signature", signature)
		event.Signature = signature
	}

	// 发送请求
	start := time.Now()
	resp, err := s.client.Do(req)
	delivery.Duration = time.Since(start)

	if err != nil {
		delivery.Error = err.Error()
		return delivery
	}
	defer resp.Body.Close()

	delivery.ResponseCode = resp.StatusCode

	// 读取响应
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024)) // 最大 10KB
	delivery.ResponseBody = string(respBody)

	// 判断成功
	delivery.Success = resp.StatusCode >= 200 && resp.StatusCode < 300

	return delivery
}

func (s *WebhookService) sign(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// 预定义事件类型
const (
	EventAgentExecuted     = "agent.executed"
	EventAgentFailed       = "agent.failed"
	EventWorkflowStarted   = "workflow.started"
	EventWorkflowCompleted = "workflow.completed"
	EventWorkflowFailed    = "workflow.failed"
	EventDocumentIndexed   = "document.indexed"
	EventUserCreated       = "user.created"
	EventUserLoggedIn      = "user.logged_in"
)
