package approval

import (
	"sync"
	"time"
)

// ApprovalEvent 描述审批状态变化
type ApprovalEvent struct {
	ApprovalID   string
	TenantID     string
	ExecutionID  string
	Status       string
	ApprovedBy   string
	AutoApproved bool
	Comment      string
	OccurredAt   time.Time
}

// EventBusConfig 控制事件总线行为
type EventBusConfig struct {
	BufferSize int
}

// ApprovalEventBus 简单本地事件总线
type ApprovalEventBus struct {
	mu     sync.RWMutex
	subs   map[string]map[uint64]chan ApprovalEvent
	seq    uint64
	buffer int
}

// NewApprovalEventBus 创建事件总线
func NewApprovalEventBus(cfg *EventBusConfig) *ApprovalEventBus {
	buffer := 1
	if cfg != nil && cfg.BufferSize > 0 {
		buffer = cfg.BufferSize
	}
	return &ApprovalEventBus{
		subs:   make(map[string]map[uint64]chan ApprovalEvent),
		buffer: buffer,
	}
}

// Publish 发布事件
func (b *ApprovalEventBus) Publish(evt ApprovalEvent) {
	if b == nil {
		return
	}
	b.mu.RLock()
	listeners := b.subs[evt.ApprovalID]
	b.mu.RUnlock()
	if len(listeners) == 0 {
		return
	}
	for key, ch := range listeners {
		select {
		case ch <- evt:
		default:
			// 如果接收方处理慢则丢弃，保持非阻塞
		}
		if ch == nil {
			b.removeListener(evt.ApprovalID, key)
		}
	}
}

// Subscribe 订阅指定审批事件
func (b *ApprovalEventBus) Subscribe(approvalID string) (<-chan ApprovalEvent, func()) {
	if b == nil {
		return nil, nil
	}
	ch := make(chan ApprovalEvent, b.buffer)
	b.mu.Lock()
	b.seq++
	id := b.seq
	if _, ok := b.subs[approvalID]; !ok {
		b.subs[approvalID] = make(map[uint64]chan ApprovalEvent)
	}
	b.subs[approvalID][id] = ch
	b.mu.Unlock()

	cancel := func() {
		b.removeListener(approvalID, id)
	}
	return ch, cancel
}

func (b *ApprovalEventBus) removeListener(approvalID string, id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if listeners, ok := b.subs[approvalID]; ok {
		if ch, exists := listeners[id]; exists {
			delete(listeners, id)
			close(ch)
		}
		if len(listeners) == 0 {
			delete(b.subs, approvalID)
		}
	}
}
