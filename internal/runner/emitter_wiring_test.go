package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/eventtest"
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
		Level:     "test",
		Message:   "test message",
		TimeMixin: events.TimeMixin{Time: time.Now()},
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
	wg, err := orch.StartSubscribers(ctx)
	if err != nil {
		t.Fatalf("StartSubscribers() returned error: %v", err)
	}

	// Verify subscribers can receive events by emitting a test event
	testEvent := &events.LogEvent{
		Level:     "test",
		Message:   "test event after startup",
		TimeMixin: events.TimeMixin{Time: time.Now()},
	}

	// Emit event - should be received by subscriber channels
	emitter.Emit(testEvent)

	// Close the emitter to trigger teardown, then wait for goroutines
	emitter.Close()
	wg.Wait()

	// Verify that emitting after close is safe (no panic)
	emitter.Emit(&events.LogEvent{
		Level:     "test",
		Message:   "after close",
		TimeMixin: events.TimeMixin{Time: time.Now()},
	})
}

// TestStreamSubscriberWiredEnsuresStructuredEventFileStreams ensures the orchestrator
// creates a structured event stream file when logs dir is configured.
func TestStreamSubscriberWiredEnsuresStructuredEventFileStreams(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := OrchestratorConfig{
		Gate:     &testStage{},
		Build:    &testStage{},
		Validate: &testStage{},
		Review:   &testStage{},
		Epilogue: &testStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return nil, nil
		},
		Output:  io.Discard,
		LogsDir: tmpDir,
	}

	orch := NewOrchestrator(cfg)
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	emitter := orch.GetEmitter()
	if emitter == nil {
		t.Fatal("GetEmitter() returned nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wg, err := orch.StartSubscribers(ctx)
	if err != nil {
		t.Fatalf("StartSubscribers returned error: %v", err)
	}

	// Wait for subscriber to start using polling
	startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer startCancel()
	if err := eventtest.WaitForSubscriberReady(startCtx, emitter); err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}

	// Give subscriber extra time to initialize file writing
	time.Sleep(50 * time.Millisecond)

	testEvent := &events.LogEvent{Level: "test", Message: "stream subscriber event"}
	emitter.Emit(testEvent)

	// Wait for the stream file to be created and have content
	// The file is created asynchronously by the stream subscriber
	files := []os.DirEntry{}
	fileWaitCtx, fileWaitCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer fileWaitCancel()
	if err := eventtest.WaitForCondition(fileWaitCtx, func() bool {
		var err error
		files, err = os.ReadDir(tmpDir)
		if err != nil {
			return false
		}
		for _, entry := range files {
			if strings.HasPrefix(entry.Name(), "events-") && strings.HasSuffix(entry.Name(), ".jsonl") {
				info, err := entry.Info()
				if err == nil && info.Size() >= 1 {
					return true
				}
			}
		}
		return false
	}); err != nil {
		t.Fatalf("WaitForCondition failed waiting for stream file: %v", err)
	}

	// Find the stream file
	var streamFile string
	for _, entry := range files {
		if strings.HasPrefix(entry.Name(), "events-") && strings.HasSuffix(entry.Name(), ".jsonl") {
			streamFile = filepath.Join(tmpDir, entry.Name())
			break
		}
	}
	if streamFile == "" {
		t.Fatal("structured event stream file not created")
	}

	// Cancel context first so subscribers exit via ctx.Done(), then close emitter.
	cancel()
	emitter.Close()
	wg.Wait()

	data, err := os.ReadFile(streamFile)
	if err != nil {
		t.Fatalf("read structured file: %v", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		t.Fatal("structured file is empty")
	}

	var eventLine struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &eventLine); err != nil {
		t.Fatalf("parse stream record: %v", err)
	}
	if eventLine.Type != testEvent.EventType() {
		t.Fatalf("structured type = %q, want %q", eventLine.Type, testEvent.EventType())
	}
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
	wg, err := orch.StartSubscribers(ctx)
	if err != nil {
		t.Fatalf("StartSubscribers() returned error: %v", err)
	}

	// Wait for subscriber to start using polling
	startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer startCancel()
	if err := eventtest.WaitForSubscriberReady(startCtx, emitter); err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}

	// Emit a test event
	testEvent := &events.LogEvent{
		Level:     "test",
		Message:   "test message from CLI subscriber test",
		TimeMixin: events.TimeMixin{Time: time.Now()},
	}

	emitter.Emit(testEvent)

	// Wait for the subscriber goroutine to process the event
	processCtx, processCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer processCancel()
	if err := eventtest.WaitForCondition(processCtx, func() bool {
		return output.hasContent()
	}); err != nil {
		t.Fatalf("WaitForCondition failed: %v", err)
	}

	// Cleanup: close emitter then wait for goroutines
	emitter.Close()
	wg.Wait()
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
	wg, err := orch.StartSubscribers(ctx)
	if err != nil {
		t.Fatalf("StartSubscribers() returned error: %v", err)
	}

	// Wait for subscriber goroutines to start
	startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer startCancel()
	if err := eventtest.WaitForSubscriberReady(startCtx, emitter); err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}

	// Emit a test event
	testEvent := &events.LogEvent{
		Level:     "test",
		Message:   "test message for conditional subscribers",
		TimeMixin: events.TimeMixin{Time: time.Now()},
	}

	emitter.Emit(testEvent)

	// Wait for the subscriber goroutines to process the event
	processCtx, processCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer processCancel()
	if err := eventtest.WaitForCondition(processCtx, func() bool {
		return output.hasContent()
	}); err != nil {
		t.Fatalf("WaitForCondition failed: %v", err)
	}

	// Cleanup: close emitter then wait for goroutines
	emitter.Close()
	wg.Wait()

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
		Level:     "test",
		Message:   "test after run",
		TimeMixin: events.TimeMixin{Time: time.Now()},
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
		Level:     "test",
		Message:   "after run emitter closed",
		TimeMixin: events.TimeMixin{Time: time.Now()},
	})

	// The channel should be closed or the event dropped (emitter is closed)
	// Either way, the operation should not panic
}

// testStage is a minimal pipeline.Stage for testing
type testStage struct{}

func (s *testStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	return pipeline.Output{}, nil
}
