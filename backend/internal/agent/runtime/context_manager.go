package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pkoukk/tiktoken-go"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrSessionNotFound 会话不存在错误
	ErrSessionNotFound = errors.New("session not found")
)

// SessionStore 会话存储接口
type SessionStore interface {
	// Get 获取会话
	Get(ctx context.Context, sessionID string) (*Session, error)
	// Save 保存会话
	Save(ctx context.Context, session *Session) error
	// Delete 删除会话
	Delete(ctx context.Context, sessionID string) error
}

// InMemorySessionStore 内存会话存储
type InMemorySessionStore struct {
	sessions map[string]*Session
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*Session),
	}
}

func (s *InMemorySessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (s *InMemorySessionStore) Save(ctx context.Context, session *Session) error {
	s.sessions[session.ID] = session
	return nil
}

func (s *InMemorySessionStore) Delete(ctx context.Context, sessionID string) error {
	delete(s.sessions, sessionID)
	return nil
}

// RedisSessionStore Redis 会话存储
type RedisSessionStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisSessionStore(client *redis.Client, ttl time.Duration) *RedisSessionStore {
	return &RedisSessionStore{
		client: client,
		ttl:    ttl,
	}
}

func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *RedisSessionStore) Save(ctx context.Context, session *Session) error {
	key := fmt.Sprintf("session:%s", session.ID)
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, key, data, s.ttl).Err()
}

func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return s.client.Del(ctx, key).Err()
}

// ContextManager Agent 上下文管理器
type ContextManager struct {
	store SessionStore
}

// NewContextManager 创建上下文管理器
func NewContextManager(store SessionStore) *ContextManager {
	if store == nil {
		store = NewInMemorySessionStore()
	}
	return &ContextManager{
		store: store,
	}
}

// Session 会话
type Session struct {
	ID        string         `json:"id"`
	TenantID  string         `json:"tenant_id"`
	UserID    string         `json:"user_id"`
	History   []Message      `json:"history"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// CreateSession 创建会话
func (cm *ContextManager) CreateSession(ctx context.Context, tenantID, userID, sessionID string) (*Session, error) {
	session := &Session{
		ID:        sessionID,
		TenantID:  tenantID,
		UserID:    userID,
		History:   make([]Message, 0),
		Data:      make(map[string]any),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := cm.store.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// GetSession 获取会话
func (cm *ContextManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return cm.store.Get(ctx, sessionID)
}

// GetOrCreateSession 获取或创建会话
func (cm *ContextManager) GetOrCreateSession(ctx context.Context, tenantID, userID, sessionID string) (*Session, error) {
	session, err := cm.store.Get(ctx, sessionID)
	if err == nil {
		session.UpdatedAt = time.Now()
		// 异步更新，忽略错误
		go func() {
			_ = cm.store.Save(context.Background(), session)
		}()
		return session, nil
	}

	if !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}

	return cm.CreateSession(ctx, tenantID, userID, sessionID)
}

// AddMessage 添加消息到会话历史
func (cm *ContextManager) AddMessage(ctx context.Context, sessionID, role, content string) error {
	session, err := cm.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	session.History = append(session.History, Message{
		Role:    role,
		Content: content,
	})
	session.UpdatedAt = time.Now()

	return cm.store.Save(ctx, session)
}

// GetHistory 获取会话历史
// 如果 limit > 0，返回最近的 limit 条
// 如果 maxTokens > 0，使用 TokenAwareTrimmer 截断历史（优先保留最近消息，并保证包含 SystemPrompt）
func (cm *ContextManager) GetHistory(ctx context.Context, sessionID string, limit int, maxTokens int, model string) ([]Message, error) {
	session, err := cm.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	history := session.History

	// 1. 优先基于条数截断（硬限制）
	if limit > 0 && len(history) > limit {
		history = history[len(history)-limit:]
	}

	// 2. 如果指定了 maxTokens，进行 Token 截断
	if maxTokens > 0 {
		trimmed, err := TrimHistoryByTokens(history, maxTokens, model)
		if err != nil {
			// 降级：如果 Token 计算失败，仅按条数返回
			return history, nil
		}
		history = trimmed
	}

	return history, nil
}

// SetData 设置会话数据
func (cm *ContextManager) SetData(ctx context.Context, sessionID, key string, value any) error {
	session, err := cm.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.Data == nil {
		session.Data = make(map[string]any)
	}
	session.Data[key] = value
	session.UpdatedAt = time.Now()

	return cm.store.Save(ctx, session)
}

// GetData 获取会话数据
func (cm *ContextManager) GetData(ctx context.Context, sessionID, key string) (any, error) {
	session, err := cm.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Data == nil {
		return nil, fmt.Errorf("data not found: %s", key)
	}

	val, ok := session.Data[key]
	if !ok {
		return nil, fmt.Errorf("data not found: %s", key)
	}

	return val, nil
}

// DeleteSession 删除会话
func (cm *ContextManager) DeleteSession(ctx context.Context, sessionID string) error {
	return cm.store.Delete(ctx, sessionID)
}

// EnrichInput 丰富输入（添加历史对话与摘要）
// historyLimit: 历史消息条数限制
// maxTokens: 最大 Token 数限制 (传 0 表示不限制)
// model: 模型名称 (用于 Token 计算)
func (cm *ContextManager) EnrichInput(ctx context.Context, input *AgentInput, sessionID string, historyLimit int, maxTokens int, model string) error {
	if sessionID == "" || input == nil {
		return nil
	}

	// 默认模型
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	history, err := cm.GetHistory(ctx, sessionID, historyLimit, maxTokens, model)
	if err != nil {
		// 忽略会话不存在的错误
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}

	messages := history

	// 检查是否启用摘要记忆模式
	memoryMode := ""
	if input.ExtraParams != nil {
		if v, ok := input.ExtraParams["memory_mode"]; ok {
			if s, ok2 := v.(string); ok2 {
				memoryMode = s
			}
		}
	}

	if memoryMode == "summary" {
		if val, err := cm.GetData(ctx, sessionID, "memory_summary"); err == nil {
			if summary, ok := val.(string); ok && summary != "" {
				messages = append([]Message{{
					Role:    "system",
					Content: summary,
				}}, history...)
			}
		}
	}

	input.History = messages
	return nil
}

// SaveInteraction 保存交互
func (cm *ContextManager) SaveInteraction(ctx context.Context, sessionID, userInput, agentOutput string) error {
	if sessionID == "" {
		return nil
	}

	// 为了保持事务性（尽量），这里先读再追加保存，但 Redis 无事务锁情况下仍有竞态
	// 简单起见，直接追加
	if err := cm.AddMessage(ctx, sessionID, "user", userInput); err != nil {
		return err
	}

	return cm.AddMessage(ctx, sessionID, "assistant", agentOutput)
}

// CalculateTokenCount 计算消息列表的 Token 总数
func CalculateTokenCount(messages []Message, model string) (int, error) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		tkm, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return 0, err
		}
	}

	totalTokens := 0
	for _, msg := range messages {
		// 简单估算：content tokens + role overhead
		totalTokens += len(tkm.Encode(msg.Content, nil, nil)) + 4
	}
	return totalTokens, nil
}

// TrimHistoryByTokens 基于 Token 数量截断历史消息
// 保留策略：
// 1. 总是保留最新的消息
// 2. 尝试从旧到新丢弃消息，直到满足 maxTokens
// 3. (可选优化) 总是保留 System Prompt (如有)
func TrimHistoryByTokens(history []Message, maxTokens int, model string) ([]Message, error) {
	if len(history) == 0 {
		return history, nil
	}

	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// 如果模型未识别，回退到 cl100k_base
		tkm, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, err
		}
	}

	// 计算所有消息的 Token
	type msgToken struct {
		msg    Message
		tokens int
	}
	var msgTokens []msgToken
	totalTokens := 0

	for _, msg := range history {
		// 简单估算：content tokens + role overhead
		tokens := len(tkm.Encode(msg.Content, nil, nil)) + 4 // 4 is approx overhead for role etc.
		msgTokens = append(msgTokens, msgToken{msg: msg, tokens: tokens})
		totalTokens += tokens
	}

	if totalTokens <= maxTokens {
		return history, nil
	}

	// 开始截断
	// 策略：保留 system (如果它是第一条)，然后从第二条开始丢弃，直到满足要求
	var keptMessages []Message
	currentTokens := 0

	// 1. 检查 System Prompt
	hasSystem := false
	if len(msgTokens) > 0 && msgTokens[0].msg.Role == "system" {
		hasSystem = true
		currentTokens += msgTokens[0].tokens
	}

	// 2. 从后往前添加消息，直到超限
	// 即使有 System Prompt，我们也先算最新的消息
	var reversedKept []Message
	
	// 从最后一条开始遍历
	startIndex := len(msgTokens) - 1
	endIndex := 0
	if hasSystem {
		endIndex = 1 // 跳过第一条 (System)
	}

	for i := startIndex; i >= endIndex; i-- {
		if currentTokens+msgTokens[i].tokens > maxTokens {
			break
		}
		currentTokens += msgTokens[i].tokens
		reversedKept = append(reversedKept, msgTokens[i].msg)
	}

	// 3. 组装最终列表
	if hasSystem {
		keptMessages = append(keptMessages, msgTokens[0].msg)
	}

	// 反转 reversedKept
	for i := len(reversedKept) - 1; i >= 0; i-- {
		keptMessages = append(keptMessages, reversedKept[i])
	}

	// 如果即使只保留 System 和最新一条都超限，那只能返回最新的一条（或者报错，这里选择返回空的或仅最新）
	if len(keptMessages) == 0 && len(history) > 0 {
		// 极端情况：连一条都放不下，强制返回最后一条
		return []Message{history[len(history)-1]}, nil
	}

	return keptMessages, nil
}
