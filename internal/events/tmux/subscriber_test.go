//go:build !wasm

package tmux

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
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
	subscriber := NewTMUXSubscriber(manager, emitter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit iteration start event
	emitter.Emit(&events.IterationStartEvent{
		Iteration:  1,
		BeadID:     "test-1",
		BeadTitle:  "Test Bead",
	})

	time.Sleep(10 * time.Millisecond)
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
	subscriber := NewTMUXSubscriber(manager, emitter)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit run start event
	emitter.Emit(&events.RunStartEvent{
		MaxIterations: 10,
		TimeBudget:    1 * time.Hour,
	})

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
	subscriber := NewTMUXSubscriber(manager, emitter)

	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Close the emitter
	emitter.Close()

	// Subscriber should exit
	err := <-done
	if err != nil {
		t.Fatalf("subscriber.Start() returned error: %v", err)
	}
}
