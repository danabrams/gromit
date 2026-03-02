//go:build !wasm

package status

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/eventtest"
)

// mockStatusWriter is a test double for status writing.
type mockStatusWriter struct {
	updates map[string]interface{}
}

func newMockStatusWriter() *mockStatusWriter {
	return &mockStatusWriter{
		updates: make(map[string]interface{}),
	}
}

func (m *mockStatusWriter) Write(key string, value interface{}) error {
	m.updates[key] = value
	return nil
}

// TestStatusSubscriber_UpdatesOnIterationStart tests that status is updated on iteration start.
func TestStatusSubscriber_UpdatesOnIterationStart(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	writer := newMockStatusWriter()
	subscriber := NewStatusSubscriber(writer, emitter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	// Wait for subscriber to start using polling
	startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer startCancel()
	if err := eventtest.WaitForSubscriberReady(startCtx, emitter); err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}

	// Emit iteration start event
	emitter.Emit(&events.IterationStartEvent{
		Iteration: 1,
		BeadID:    "test-1",
		BeadTitle: "Test Bead",
	})

	// Wait for subscriber to process
	processCtx, processCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer processCancel()
	if err := eventtest.WaitForCondition(processCtx, func() bool {
		_, ok := writer.updates["iteration"]
		return ok
	}); err != nil {
		t.Fatalf("WaitForCondition failed: %v", err)
	}

	cancel()
	_ = <-done

	// Verify status was updated
	if val, ok := writer.updates["iteration"]; !ok || val != 1 {
		t.Errorf("Expected iteration to be 1, got %v", val)
	}
	if val, ok := writer.updates["bead_id"]; !ok || val != "test-1" {
		t.Errorf("Expected bead_id to be test-1, got %v", val)
	}
}

// TestStatusSubscriber_ConsumesEvents tests that StatusSubscriber consumes events.
func TestStatusSubscriber_ConsumesEvents(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	writer := newMockStatusWriter()
	subscriber := NewStatusSubscriber(writer, emitter)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	// Wait for subscriber to start using polling
	startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer startCancel()
	if err := eventtest.WaitForSubscriberReady(startCtx, emitter); err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}

	// Emit run start event
	emitter.Emit(&events.RunStartEvent{
		MaxIterations: 10,
		TimeBudget:    1 * time.Hour,
	})

	// Give a small buffer for processing to complete before canceling
	time.Sleep(10 * time.Millisecond)

	cancel()
	_ = <-done

	// Should not error
}

// TestStatusSubscriber_EmitterClosed_ExitsGracefully tests that StatusSubscriber exits when emitter is closed.
func TestStatusSubscriber_EmitterClosed_ExitsGracefully(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	writer := newMockStatusWriter()
	subscriber := NewStatusSubscriber(writer, emitter)

	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	// Wait for subscriber to start using polling
	startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer startCancel()
	if err := eventtest.WaitForSubscriberReady(startCtx, emitter); err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}

	// Close the emitter
	emitter.Close()

	// Subscriber should exit
	err := <-done
	if err != nil {
		t.Fatalf("subscriber.Start() returned error: %v", err)
	}
}

// TestStatusSubscriber_IgnoresUnknownEvents tests that unknown events are safely ignored without error.
func TestStatusSubscriber_IgnoresUnknownEvents(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	writer := newMockStatusWriter()
	subscriber := NewStatusSubscriber(writer, emitter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	// Wait for subscriber to start using polling
	startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer startCancel()
	if err := eventtest.WaitForSubscriberReady(startCtx, emitter); err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}

	// Emit unknown event (mockEvent is not a recognized event type)
	mockUnknownEvent := &mockUnknownEvent{}
	emitter.Emit(mockUnknownEvent)

	// Give a small buffer for processing to complete before canceling
	time.Sleep(10 * time.Millisecond)

	cancel()
	err := <-done

	// Should not error, should handle gracefully
	if err != nil {
		t.Errorf("Expected no error for unknown event, got: %v", err)
	}

	// Verify that writer was never called for the unknown event
	if len(writer.updates) > 0 {
		t.Errorf("Expected no updates for unknown event, got %d updates", len(writer.updates))
	}
}

// mockUnknownEvent is a test event type that is not handled by the subscriber.
type mockUnknownEvent struct {
	time time.Time
}

func (e *mockUnknownEvent) EventType() string {
	return "unknown_event"
}

func (e *mockUnknownEvent) EventTime() time.Time {
	if e.time.IsZero() {
		return time.Now()
	}
	return e.time
}
