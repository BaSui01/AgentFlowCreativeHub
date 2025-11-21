package notifications

import (
	"net/http"
	"time"

	"backend/internal/notification"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHandler 管理审批等实时通知的 WebSocket 连接
type WebSocketHandler struct {
	hub      *notification.WebSocketHub
	upgrader websocket.Upgrader
}

// NewWebSocketHandler 创建处理器
func NewWebSocketHandler(hub *notification.WebSocketHub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 5 * time.Second,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Connect 升级连接并注册客户端
func (h *WebSocketHandler) Connect(c *gin.Context) {
	if h == nil || h.hub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WebSocket 服务未就绪"})
		return
	}
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if tenantID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少租户或用户上下文"})
		return
	}
	channels := c.QueryArray("channels")
	if len(channels) == 0 {
		channels = []string{"approvals"}
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	})

	h.hub.Register(tenantID, userID, conn, channels)
	_ = conn.WriteJSON(gin.H{
		"type":     "connected",
		"message":  "WebSocket 已连接",
		"channels": channels,
	})

	go h.readLoop(tenantID, userID, conn)
}

func (h *WebSocketHandler) readLoop(tenantID, userID string, conn *websocket.Conn) {
	defer func() {
		h.hub.Unregister(tenantID, userID, conn)
		_ = conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
