//go:build !wasm

package tmux

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/eventtest"
)

// mockTmuxManager is a test double for tmux operations.
type mockTmuxManager struct {
	titles map[string]string
}

func newMockTmuxManager() *mockTmuxManager {
	return &mockTmuxManager{
		titles: make(map[string]string),
	}
}

func (m *mockTmuxManager) SetTitle(title string) error {
	m.titles["active"] = title
	return nil
}

// TestTMUXSubscriber_UpdatesOnIterationStart tests that tmux title is updated on iteration start.
func TestTMUXSubscriber_UpdatesOnIterationStart(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	manager := newMockTmuxManager()
	subscriber, err := NewTMUXSubscriber(manager, emitter)
	if err != nil {
		t.Fatalf("failed to create tmux subscriber: %v", err)
	}

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
		Iteration:  1,
		BeadID:     "test-1",
		BeadTitle:  "Test Bead",
	})

	// Wait for subscriber to process
	processCtx, processCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer processCancel()
	if err := eventtest.WaitForCondition(processCtx, func() bool {
		_, ok := manager.titles["active"]
		return ok
	}); err != nil {
		t.Fatalf("WaitForCondition failed: %v", err)
	}

	cancel()
	_ = <-done

	// Verify title was updated
	if title, ok := manager.titles["active"]; !ok || title == "" {
		t.Errorf("Expected tmux title to be set, got %q", title)
	}
}

// TestTMUXSubscriber_ConsumesEvents tests that TMUXSubscriber consumes events.
func TestTMUXSubscriber_ConsumesEvents(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	manager := newMockTmuxManager()
	subscriber, err := NewTMUXSubscriber(manager, emitter)
	if err != nil {
		t.Fatalf("failed to create tmux subscriber: %v", err)
	}

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

// TestTMUXSubscriber_EmitterClosed_ExitsGracefully tests that TMUXSubscriber exits when emitter is closed.
func TestTMUXSubscriber_EmitterClosed_ExitsGracefully(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	manager := newMockTmuxManager()
	subscriber, err := NewTMUXSubscriber(manager, emitter)
	if err != nil {
		t.Fatalf("failed to create tmux subscriber: %v", err)
	}

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
	err = <-done
	if err != nil {
		t.Fatalf("subscriber.Start() returned error: %v", err)
	}
}

// TestTMUXSubscriber_IgnoresUnknownEvents tests that unknown events are safely ignored without error.
func TestTMUXSubscriber_IgnoresUnknownEvents(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	manager := newMockTmuxManager()
	subscriber, err := NewTMUXSubscriber(manager, emitter)
	if err != nil {
		t.Fatalf("failed to create tmux subscriber: %v", err)
	}

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

	// Emit unknown event (mockUnknownEvent is not a recognized event type)
	mockUnknownEvent := &mockUnknownEvent{}
	emitter.Emit(mockUnknownEvent)

	// Give a small buffer for processing to complete before canceling
	time.Sleep(10 * time.Millisecond)

	cancel()
	err = <-done

	// Should not error, should handle gracefully
	if err != nil {
		t.Errorf("Expected no error for unknown event, got: %v", err)
	}

	// Verify that manager was never called for the unknown event
	if len(manager.titles) > 0 {
		t.Errorf("Expected no title updates for unknown event, got %d updates", len(manager.titles))
	}
}

// TestNewTMUXSubscriber_InvalidManager ensures invalid managers produce an error instead of panicking.
func TestNewTMUXSubscriber_InvalidManager(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()

	if _, err := NewTMUXSubscriber("not-a-manager", emitter); err == nil {
		t.Fatalf("expected an error when manager does not implement TMUXManager")
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
