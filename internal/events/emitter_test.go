//go:build !wasm

package events

import (
	"testing"
	"time"
)

// TestEmitter_Subscribe_ReturnsBufferedChannel tests that Subscribe returns a buffered channel.
func TestEmitter_Subscribe_ReturnsBufferedChannel(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	ch := emitter.Subscribe()
	if ch == nil {
		t.Fatal("Subscribe() returned nil channel")
	}

	// Verify it's buffered (non-blocking single send)
	select {
	case ch <- &LogEvent{Level: "info", Message: "test"}:
		// Good, it's buffered
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Subscribe() returned unbuffered or full channel")
	}
}

// TestEmitter_Emit_FanOutToAllSubscribers tests that Emit sends events to all subscribers.
func TestEmitter_Emit_FanOutToAllSubscribers(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	ch1 := emitter.Subscribe()
	ch2 := emitter.Subscribe()

	event := &LogEvent{Level: "info", Message: "test"}
	emitter.Emit(event)

	// Both subscribers should receive the event
	select {
	case e := <-ch1:
		if e.EventType() != "log" {
			t.Errorf("ch1 got wrong event type: %v", e.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch1 did not receive event")
	}

	select {
	case e := <-ch2:
		if e.EventType() != "log" {
			t.Errorf("ch2 got wrong event type: %v", e.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch2 did not receive event")
	}
}

// TestEmitter_Unsubscribe_StopsDelivery tests that Unsubscribe removes a subscriber.
func TestEmitter_Unsubscribe_StopsDelivery(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	ch := emitter.Subscribe()
	emitter.Unsubscribe(ch)

	event := &LogEvent{Level: "info", Message: "test"}
	emitter.Emit(event)

	// Should not receive event after unsubscribe (channel is closed and empty)
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("ch received event after unsubscribe")
		}
		// ok=false means channel was closed (expected)
	case <-time.After(100 * time.Millisecond):
		t.Error("ch should be closed immediately after unsubscribe")
	}
}

// TestEmitter_Close_ClosesAllChannels tests that Close shuts down all subscriber channels.
func TestEmitter_Close_ClosesAllChannels(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()

	ch1 := emitter.Subscribe()
	ch2 := emitter.Subscribe()

	emitter.Close()

	// Channels should be closed; receiving from closed channel returns zero value and ok=false
	_, ok := <-ch1
	if ok {
		t.Error("ch1 not closed after emitter.Close()")
	}

	_, ok = <-ch2
	if ok {
		t.Error("ch2 not closed after emitter.Close()")
	}
}

// TestEmitter_EmitAfterClose_NoOp tests that Emit after Close is a no-op.
func TestEmitter_EmitAfterClose_NoOp(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	emitter.Close()

	// Should not panic
	emitter.Emit(&LogEvent{Level: "info", Message: "test"})
}
