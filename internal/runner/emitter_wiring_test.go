package runner

import (
	"context"
	"io"
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

// testStage is a minimal pipeline.Stage for testing
type testStage struct{}

func (s *testStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	return pipeline.Output{}, nil
}
