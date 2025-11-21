package notification

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"backend/internal/logger"
	"backend/internal/metrics"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type clientConn struct {
	conn     *websocket.Conn
	channels map[string]struct{}
	mu       sync.Mutex
}

// WebSocketHub 负责管理租户/用户的 WebSocket 连接
type WebSocketHub struct {
	mu                sync.RWMutex
	clients           map[string]map[string]map[*websocket.Conn]*clientConn
	offline           OfflineStore
	keepAliveInterval time.Duration
	logger            *zap.Logger
}

// HubOption 配置 hub
type HubOption func(*WebSocketHub)

// WithOfflineStore 指定离线存储
func WithOfflineStore(store OfflineStore) HubOption {
	return func(h *WebSocketHub) { h.offline = store }
}

// WithKeepAliveInterval 设置心跳间隔
func WithKeepAliveInterval(interval time.Duration) HubOption {
	return func(h *WebSocketHub) { h.keepAliveInterval = interval }
}

// WithHubLogger 设置日志器
func WithHubLogger(l *zap.Logger) HubOption {
	return func(h *WebSocketHub) { h.logger = l }
}

// NewWebSocketHub 创建 Hub
func NewWebSocketHub(opts ...HubOption) *WebSocketHub {
	hub := &WebSocketHub{
		clients:           make(map[string]map[string]map[*websocket.Conn]*clientConn),
		offline:           NewMemoryOfflineStore(50),
		keepAliveInterval: 30 * time.Second,
		logger:            logger.Get(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(hub)
		}
	}
	return hub
}

// Register 注册连接
func (h *WebSocketHub) Register(tenantID, userID string, conn *websocket.Conn, channels []string) {
	h.mu.Lock()
	if _, ok := h.clients[tenantID]; !ok {
		h.clients[tenantID] = make(map[string]map[*websocket.Conn]*clientConn)
	}
	if _, ok := h.clients[tenantID][userID]; !ok {
		h.clients[tenantID][userID] = make(map[*websocket.Conn]*clientConn)
	}
	client := &clientConn{
		conn:     conn,
		channels: sliceToSet(channels),
	}
	h.clients[tenantID][userID][conn] = client
	h.mu.Unlock()

	metrics.WebSocketConnectionsGauge.WithLabelValues(tenantID).Inc()
	h.replayOffline(context.Background(), tenantID, userID, client)
	h.startKeepAlive(tenantID, userID, client)
}

// Unregister 移除连接
func (h *WebSocketHub) Unregister(tenantID, userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if users, ok := h.clients[tenantID]; ok {
		if conns, ok := users[userID]; ok {
			if _, ok := conns[conn]; ok {
				delete(conns, conn)
				metrics.WebSocketConnectionsGauge.WithLabelValues(tenantID).Dec()
			}
			if len(conns) == 0 {
				delete(users, userID)
			}
		}
		if len(users) == 0 {
			delete(h.clients, tenantID)
		}
	}
}

// SendToUser 将通知发送给指定租户/用户的所有连接
func (h *WebSocketHub) SendToUser(tenantID, userID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	h.mu.RLock()
	userConns := h.clients[tenantID][userID]
	h.mu.RUnlock()
	if len(userConns) == 0 {
		return h.storeOffline(context.Background(), tenantID, userID, data)
	}

	var firstErr error
	for conn, client := range userConns {
		client.mu.Lock()
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			client.mu.Unlock()
			h.Unregister(tenantID, userID, conn)
			_ = conn.Close()
			_ = h.storeOffline(context.Background(), tenantID, userID, data)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		client.mu.Unlock()
	}
	return firstErr
}

// CloseTenant 清理租户下所有连接
func (h *WebSocketHub) CloseTenant(tenantID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if users, ok := h.clients[tenantID]; ok {
		for userID, conns := range users {
			for conn := range conns {
				_ = conn.Close()
			}
			delete(users, userID)
		}
		delete(h.clients, tenantID)
	}
}

// ConnectedCount 返回指定租户/用户的连接数（用于调试/指标）
func (h *WebSocketHub) ConnectedCount(tenantID, userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[tenantID][userID])
}

func (h *WebSocketHub) replayOffline(ctx context.Context, tenantID, userID string, client *clientConn) {
	if h.offline == nil {
		return
	}
	messages, err := h.offline.Drain(ctx, tenantID, userID)
	if err != nil && h.logger != nil {
		h.logger.Warn("离线消息重放失败", zap.String("tenantId", tenantID), zap.String("userId", userID), zap.Error(err))
		return
	}
	for _, msg := range messages {
		client.mu.Lock()
		client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil && h.logger != nil {
			h.logger.Debug("推送离线消息失败", zap.Error(err))
		}
		client.mu.Unlock()
	}
}

func (h *WebSocketHub) storeOffline(ctx context.Context, tenantID, userID string, payload []byte) error {
	if h.offline == nil {
		return nil
	}
	return h.offline.Append(ctx, tenantID, userID, payload)
}

func (h *WebSocketHub) startKeepAlive(tenantID, userID string, client *clientConn) {
	if h.keepAliveInterval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(h.keepAliveInterval)
		defer ticker.Stop()
		for range ticker.C {
			client.mu.Lock()
			err := client.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			client.mu.Unlock()
			if err != nil {
				h.Unregister(tenantID, userID, client.conn)
				_ = client.conn.Close()
				return
			}
		}
	}()
}

func sliceToSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}
	return set
}
