//go:build !wasm

package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/eventtest"
	"github.com/danabrams/gromit/internal/runner/execution"
)

// TestCLISubscriber_ConsumesEvents tests that CLISubscriber consumes events and exits on context cancel.
func TestCLISubscriber_ConsumesEvents(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	output := &bytes.Buffer{}
	subscriber := NewCLISubscriber(BasicWriter(output), emitter)

	ctx, cancel := context.WithCancel(context.Background())

	// Run subscriber in a goroutine
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

	// Emit an event
	emitter.Emit(&events.IterationStartEvent{
		Iteration:  1,
		BeadID:     "test-1",
		BeadTitle:  "Test Bead",
	})

	// Wait for subscriber to process using polling
	processCtx, processCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer processCancel()
	if err := eventtest.WaitForCondition(processCtx, func() bool {
		return len(output.String()) > 0
	}); err != nil {
		t.Fatalf("WaitForCondition failed: %v", err)
	}

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
	subscriber := NewCLISubscriber(BasicWriter(output), emitter)

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

	// Emit an unknown event type (one that CLISubscriber doesn't handle)
	unknownEvent := &unknownEventType{}
	emitter.Emit(unknownEvent)

	// For unknown events, we just verify no panic occurs during processing
	// Give a small buffer for processing to complete
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
	subscriber := NewCLISubscriber(BasicWriter(output), emitter)

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

// TestCLISubscriber_HeartbeatUsesWriteOverwrite tests that HeartbeatEvent uses WriteOverwrite semantics.
func TestCLISubscriber_HeartbeatUsesWriteOverwrite(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	mockWriter := &mockOverwriteWriter{}
	subscriber := NewCLISubscriber(mockWriter, emitter)

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

	// Emit a heartbeat event
	emitter.Emit(&events.HeartbeatEvent{
		Elapsed:       1 * time.Second,
		ToolCalls:     3,
		FilesModified: 5,
		RateLimitHits: 0,
	})

	// Wait for subscriber to process
	processCtx, processCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer processCancel()
	if err := eventtest.WaitForCondition(processCtx, func() bool {
		return mockWriter.overwriteCalled
	}); err != nil {
		t.Fatalf("WaitForCondition failed: %v", err)
	}

	cancel()
	err := <-done
	if err != nil {
		t.Fatalf("subscriber.Start() returned error: %v", err)
	}

	// Verify WriteOverwrite was called
	if !mockWriter.overwriteCalled {
		t.Error("Expected WriteOverwrite to be called for HeartbeatEvent, but it was not")
	}

	// Verify regular Write was not called
	if mockWriter.writeCalled {
		t.Error("Expected Write to not be called for HeartbeatEvent, but it was")
	}
}

// mockOverwriteWriter is a test mock that implements execution.OverwriteWriter.
type mockOverwriteWriter struct {
	writeCalled      bool
	overwriteCalled  bool
	writeData        []byte
	overwriteData    []byte
}

func (m *mockOverwriteWriter) Write(p []byte) (int, error) {
	m.writeCalled = true
	m.writeData = append(m.writeData, p...)
	return len(p), nil
}

func (m *mockOverwriteWriter) WriteOverwrite(p []byte) (int, error) {
	m.overwriteCalled = true
	m.overwriteData = append(m.overwriteData, p...)
	return len(p), nil
}

// TestBasicWriterImplementsOverwriteWriter verifies BasicWriter presents an execution.OverwriteWriter.
func TestBasicWriterImplementsOverwriteWriter(t *testing.T) {
	t.Parallel()

	var _ execution.OverwriteWriter = BasicWriter(&bytes.Buffer{})
}
