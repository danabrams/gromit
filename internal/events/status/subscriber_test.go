//go:build !wasm

package status

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
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

	time.Sleep(10 * time.Millisecond)

	// Close the emitter
	emitter.Close()

	// Subscriber should exit
	err := <-done
	if err != nil {
		t.Fatalf("subscriber.Start() returned error: %v", err)
	}
}
