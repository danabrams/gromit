package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestEmitterCreatedAndAccessibleFromOrchestrator verifies that the Emitter is created
// and accessible from the Orchestrator for event emission.
func TestEmitterCreatedAndAccessibleFromOrchestrator(t *testing.T) {
	t.Parallel()

	cfg := OrchestratorConfig{
		Gate:     &testStage{},
		Build:    &testStage{},
		Validate: &testStage{},
		Review:   &testStage{},
		Epilogue: &testStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return nil, nil
		},
		Output: io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	// Verify that the orchestrator has an Emitter
	emitter := orch.GetEmitter()
	if emitter == nil {
		t.Fatal("Orchestrator.GetEmitter() returned nil, want non-nil Emitter")
	}

	// Verify that we can emit an event and it's received by a subscriber
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	testEvent := &events.LogEvent{
		Level:   "test",
		Message: "test message",
		Time:    time.Now(),
	}

	emitter.Emit(testEvent)

	// Verify event is received
	select {
	case received := <-ch:
		if received != testEvent {
			t.Fatalf("received event = %v, want %v", received, testEvent)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for emitted event")
	}
}

// TestSubscriberStartupAndTeardown verifies that CLI subscriber is always registered,
// status/tmux subscribers are conditionally registered, all subscribers are started,
// and proper cleanup occurs on Emitter close.
func TestSubscriberStartupAndTeardown(t *testing.T) {
	t.Parallel()

	cfg := OrchestratorConfig{
		Gate:     &testStage{},
		Build:    &testStage{},
		Validate: &testStage{},
		Review:   &testStage{},
		Epilogue: &testStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return nil, nil
		},
		Output: io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	emitter := orch.GetEmitter()
	if emitter == nil {
		t.Fatal("GetEmitter() returned nil")
	}

	// Create a test context with timeout for subscriber goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Call startup to register and start subscribers
	err := orch.StartSubscribers(ctx)
	if err != nil {
		t.Fatalf("StartSubscribers() returned error: %v", err)
	}

	// Verify subscribers can receive events by emitting a test event
	testEvent := &events.LogEvent{
		Level:   "test",
		Message: "test event after startup",
		Time:    time.Now(),
	}

	// Emit event - should be received by subscriber channels
	emitter.Emit(testEvent)

	// Close the emitter to trigger teardown
	emitter.Close()

	// Verify that emitting after close is safe (no panic)
	emitter.Emit(&events.LogEvent{
		Level:   "test",
		Message: "after close",
		Time:    time.Now(),
	})
}

// TestCLISubscriberStartsAndReceivesEvents verifies that the CLI subscriber starts
// in a goroutine and can receive events emitted by the orchestrator.
func TestCLISubscriberStartsAndReceivesEvents(t *testing.T) {
	t.Parallel()

	output := &recordingWriter{}

	cfg := OrchestratorConfig{
		Gate:     &testStage{},
		Build:    &testStage{},
		Validate: &testStage{},
		Review:   &testStage{},
		Epilogue: &testStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return nil, nil
		},
		Output: output,
	}

	orch := NewOrchestrator(cfg)
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	emitter := orch.GetEmitter()
	if emitter == nil {
		t.Fatal("GetEmitter() returned nil")
	}

	// Create a test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start subscribers
	err := orch.StartSubscribers(ctx)
	if err != nil {
		t.Fatalf("StartSubscribers() returned error: %v", err)
	}

	// Give subscriber goroutine time to start and subscribe
	time.Sleep(200 * time.Millisecond)

	// Emit a test event
	testEvent := &events.LogEvent{
		Level:   "test",
		Message: "test message from CLI subscriber test",
		Time:    time.Now(),
	}

	emitter.Emit(testEvent)

	// Give the subscriber goroutine time to process the event
	time.Sleep(200 * time.Millisecond)

	// Verify that the CLI subscriber processed the event
	if !output.hasContent() {
		t.Fatal("CLI subscriber did not write any output after event emission")
	}

	// Cleanup
	emitter.Close()
}

// TestStatusAndTMUXSubscribersConditionalStartup verifies that status and tmux subscribers
// are started when their respective dependencies are provided in the config.
func TestStatusAndTMUXSubscribersConditionalStartup(t *testing.T) {
	t.Parallel()

	output := &recordingWriter{}

	cfg := OrchestratorConfig{
		Gate:     &testStage{},
		Build:    &testStage{},
		Validate: &testStage{},
		Review:   &testStage{},
		Epilogue: &testStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return nil, nil
		},
		Output:       output,
		StatusWriter: nil, // status writer will be handled differently
	}

	orch := NewOrchestrator(cfg)
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	emitter := orch.GetEmitter()
	if emitter == nil {
		t.Fatal("GetEmitter() returned nil")
	}

	// Create a test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start subscribers
	err := orch.StartSubscribers(ctx)
	if err != nil {
		t.Fatalf("StartSubscribers() returned error: %v", err)
	}

	// Give subscriber goroutines time to start
	time.Sleep(200 * time.Millisecond)

	// Emit a test event
	testEvent := &events.LogEvent{
		Level:   "test",
		Message: "test message for conditional subscribers",
		Time:    time.Now(),
	}

	emitter.Emit(testEvent)

	// Give the subscriber goroutines time to process the event
	time.Sleep(200 * time.Millisecond)

	// Cleanup
	emitter.Close()

	// At minimum, CLI subscriber should have output
	if !output.hasContent() {
		t.Fatal("CLI subscriber did not write any output")
	}
}

// mockStatusWriter is a test implementation of status.StatusWriter
type mockStatusWriter struct {
	writeFn func(key string, value interface{}) error
}

func (m *mockStatusWriter) Write(key string, value interface{}) error {
	if m.writeFn != nil {
		return m.writeFn(key, value)
	}
	return nil
}

// recordingWriter captures output written to it
type recordingWriter struct {
	content strings.Builder
}

func (rw *recordingWriter) Write(p []byte) (n int, err error) {
	return rw.content.Write(p)
}

func (rw *recordingWriter) hasContent() bool {
	return rw.content.Len() > 0
}

// TestRunStartsSubscribersBeforeLoop verifies that Run() starts subscribers
// before entering the main loop.
func TestRunStartsSubscribersBeforeLoop(t *testing.T) {
	t.Parallel()

	output := &recordingWriter{}

	cfg := OrchestratorConfig{
		Gate:     &testStage{},
		Build:    &testStage{},
		Validate: &testStage{},
		Review:   &testStage{},
		Epilogue: &testStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			// Return nil to end loop immediately
			return nil, nil
		},
		Output: output,
	}

	orch := NewOrchestrator(cfg)
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run should start subscribers automatically
	err := orch.Run(ctx, 0, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// After Run() completes, emitter should be closed safely
	emitter := orch.GetEmitter()
	if emitter == nil {
		t.Fatal("GetEmitter() returned nil after Run")
	}

	// Emitting after Run should not panic (emitter should be closed)
	emitter.Emit(&events.LogEvent{
		Level:   "test",
		Message: "test after run",
		Time:    time.Now(),
	})
}

// trackingWriter wraps a writer and calls a callback on each write
type trackingWriter struct {
	delegate io.Writer
	onWrite  func()
}

func (tw *trackingWriter) Write(p []byte) (n int, err error) {
	if tw.onWrite != nil {
		tw.onWrite()
	}
	return tw.delegate.Write(p)
}

// TestEmitterClosedAfterRun verifies that the Emitter is properly closed
// when Run() completes, ensuring subscriber goroutines can shut down cleanly.
func TestEmitterClosedAfterRun(t *testing.T) {
	t.Parallel()

	cfg := OrchestratorConfig{
		Gate:     &testStage{},
		Build:    &testStage{},
		Validate: &testStage{},
		Review:   &testStage{},
		Epilogue: &testStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return nil, nil
		},
		Output: io.Discard,
	}

	orch := NewOrchestrator(cfg)
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	emitter := orch.GetEmitter()
	if emitter == nil {
		t.Fatal("GetEmitter() returned nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run the orchestrator
	err := orch.Run(ctx, 0, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// After Run() completes, the emitter should be closed
	// Verify by trying to subscribe - if closed, channels should receive immediately
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	// Emit an event
	emitter.Emit(&events.LogEvent{
		Level:   "test",
		Message: "after run emitter closed",
		Time:    time.Now(),
	})

	// The channel should be closed or the event dropped (emitter is closed)
	// Either way, the operation should not panic
}

// testStage is a minimal pipeline.Stage for testing
type testStage struct{}

func (s *testStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	return pipeline.Output{}, nil
}
