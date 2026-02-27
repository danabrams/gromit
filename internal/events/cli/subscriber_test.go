//go:build !wasm

package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

// TestCLISubscriber_ConsumesEvents tests that CLISubscriber consumes events and exits on context cancel.
func TestCLISubscriber_ConsumesEvents(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	output := &bytes.Buffer{}
	subscriber := NewCLISubscriber(output, emitter)

	ctx, cancel := context.WithCancel(context.Background())

	// Run subscriber in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	// Give subscriber time to start
	time.Sleep(10 * time.Millisecond)

	// Emit an event
	emitter.Emit(&events.IterationStartEvent{
		Iteration:  1,
		BeadID:     "test-1",
		BeadTitle:  "Test Bead",
	})

	// Give subscriber time to process
	time.Sleep(10 * time.Millisecond)

	// Cancel context to stop subscriber
	cancel()

	// Wait for subscriber to exit
	err := <-done
	if err != nil {
		t.Fatalf("subscriber.Start() returned error: %v", err)
	}

	// Verify output was written
	output_str := output.String()
	if len(output_str) == 0 {
		t.Error("Expected output but got none")
	}
}

// TestCLISubscriber_IgnoresUnknownEvents tests that CLISubscriber ignores unknown event types.
func TestCLISubscriber_IgnoresUnknownEvents(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	output := &bytes.Buffer{}
	subscriber := NewCLISubscriber(output, emitter)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- subscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit an unknown event type (one that CLISubscriber doesn't handle)
	unknownEvent := &unknownEventType{}
	emitter.Emit(unknownEvent)

	time.Sleep(10 * time.Millisecond)

	cancel()
	_ = <-done

	// Should not panic and should produce minimal output
	_ = output.String()
}

// unknownEventType is a test event that CLISubscriber doesn't handle.
type unknownEventType struct{}

func (e *unknownEventType) EventType() string {
	return "unknown"
}

func (e *unknownEventType) EventTime() time.Time {
	return time.Now()
}

// TestCLISubscriber_EmitterClosed_ExitsGracefully tests that CLISubscriber exits when emitter is closed.
func TestCLISubscriber_EmitterClosed_ExitsGracefully(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	output := &bytes.Buffer{}
	subscriber := NewCLISubscriber(output, emitter)

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
