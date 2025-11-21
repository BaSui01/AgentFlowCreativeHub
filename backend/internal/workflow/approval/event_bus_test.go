package approval

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApprovalEventBus(t *testing.T) {
	bus := NewApprovalEventBus(&EventBusConfig{BufferSize: 2})
	ch, cancel := bus.Subscribe("approval-1")
	t.Cleanup(func() {
		if cancel != nil {
			cancel()
		}
	})

	expected := ApprovalEvent{ApprovalID: "approval-1", Status: "approved"}
	bus.Publish(expected)

	select {
	case evt := <-ch:
		require.Equal(t, expected.ApprovalID, evt.ApprovalID)
		require.Equal(t, expected.Status, evt.Status)
	default:
		t.Fatal("expected event to be delivered")
	}

	// ensure cancel removes listener without panic
	cancel()
	bus.Publish(expected)
	select {
	case <-ch:
		// channel closed after cancel
	case <-time.After(50 * time.Millisecond):
		// ok
	}
}
