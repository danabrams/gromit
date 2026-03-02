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

// TestEmitter_ConcurrentEmit_IsSafe tests that concurrent Emit calls are safe.
func TestEmitter_ConcurrentEmit_IsSafe(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	ch := emitter.Subscribe()

	// Launch multiple goroutines emitting concurrently
	const numGoroutines = 10
	const eventsPerGoroutine = 10
	done := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < eventsPerGoroutine; j++ {
				event := &LogEvent{
					Level:   "info",
					Message: "concurrent test",
				}
				emitter.Emit(event)
			}
			done <- struct{}{}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify events were received (some may be dropped due to buffer full, but we should get some)
	received := 0
	deadline := time.After(1 * time.Second)
drain:
	for received < (numGoroutines * eventsPerGoroutine / 2) {
		select {
		case <-ch:
			received++
		case <-deadline:
			break drain
		}
	}

	if received == 0 {
		t.Fatal("no events received from concurrent emits")
	}
}

// TestEmitter_SlowConsumer_DropsEvents tests that events are dropped for slow consumers when buffer is full.
func TestEmitter_SlowConsumer_DropsEvents(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	slowCh := emitter.Subscribe()

	// Fill the subscriber's buffer with events (buffer size is 100)
	const bufferedSize = 100
	for i := 0; i < bufferedSize+50; i++ {
		event := &LogEvent{
			Level:   "info",
			Message: "fill buffer test",
		}
		emitter.Emit(event)
	}

	// Now consume a few events from the slow consumer
	received := 0
	for i := 0; i < 20; i++ {
		select {
		case <-slowCh:
			received++
		case <-time.After(100 * time.Millisecond):
			t.Fatal("failed to receive expected events")
		}
	}

	if received != 20 {
		t.Errorf("expected to receive 20 events, got %d", received)
	}

	// Since we sent 150 events but buffer is only 100, some events must have been dropped
	// This verifies non-blocking drop-on-full behavior
}

// TestEmitter_SubscribeAfterClose_ReturnsClosedChannel tests that subscribing after close
// returns a pre-closed channel so callers exit immediately instead of blocking forever.
func TestEmitter_SubscribeAfterClose_ReturnsClosedChannel(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	emitter.Close()

	// Subscribe after close should return a pre-closed channel
	ch := emitter.Subscribe()
	if ch == nil {
		t.Fatal("Subscribe after close returned nil channel")
	}

	// Reading from a pre-closed channel should return immediately with ok=false
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed (ok=false), got ok=true")
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout: channel was not closed, subscriber would leak")
	}
}

// TestEmitter_UnsubscribeAfterClose_IsNoop tests edge case of unsubscribing after close.
func TestEmitter_UnsubscribeAfterClose_IsNoop(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	ch := emitter.Subscribe()
	emitter.Close()

	// Unsubscribe after close should not panic
	emitter.Unsubscribe(ch)
}

// TestEmitter_CloseIsIdempotent tests that Close can be called multiple times safely.
func TestEmitter_CloseIsIdempotent(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	ch := emitter.Subscribe()

	// First close
	emitter.Close()

	// Second close should not panic
	emitter.Close()

	// Third close should not panic
	emitter.Close()

	// Channel should still be closed
	_, ok := <-ch
	if ok {
		t.Error("channel not closed after multiple Close() calls")
	}
}
