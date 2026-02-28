//go:build acceptance

package acceptance_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/cli"
	"github.com/danabrams/gromit/internal/events/status"
	"github.com/danabrams/gromit/internal/events/tmux"
)

// mockStatusWriter for testing status updates.
type testStatusWriter struct {
	updates map[string]interface{}
}

func (w *testStatusWriter) Write(key string, value interface{}) error {
	w.updates[key] = value
	return nil
}

// mockTmuxManager for testing tmux updates.
type testTmuxManager struct {
	titles []string
}

func (m *testTmuxManager) SetTitle(title string) error {
	m.titles = append(m.titles, title)
	return nil
}

// TestEventOrdering_SuccessPath tests event ordering for a successful iteration.
func TestEventOrdering_SuccessPath(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()

	// Capture event order
	var eventSequence []string
	captureSubscriber := &eventCaptureSubscriber{
		emitter:  emitter,
		sequence: &eventSequence,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start capture subscriber
	go func() {
		_ = captureSubscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit success path events
	emitter.Emit(&events.RunStartEvent{MaxIterations: 1})
	emitter.Emit(&events.IterationStartEvent{Iteration: 1, BeadID: "b1", BeadTitle: "Test"})
	emitter.Emit(&events.BuildStartEvent{BeadID: "b1", Model: "opus", Attempt: 1})
	emitter.Emit(&events.BuildCompleteEvent{BeadID: "b1", Success: true})
	emitter.Emit(&events.ValidationStartEvent{BeadID: "b1"})
	emitter.Emit(&events.ValidationPassEvent{BeadID: "b1"})
	emitter.Emit(&events.ReviewStartEvent{BeadID: "b1", Model: "opus"})
	emitter.Emit(&events.ReviewCompleteEvent{BeadID: "b1", Verdict: "pass"})
	emitter.Emit(&events.IterationCompleteEvent{Iteration: 1, BeadID: "b1", Success: true})
	emitter.Emit(&events.BeadCompleteEvent{BeadID: "b1", BeadTitle: "Test"})
	emitter.Emit(&events.RunCompleteEvent{IterationsCompleted: 1, Reason: "success"})

	time.Sleep(50 * time.Millisecond)
	cancel()

	// Verify event ordering
	expectedSequence := []string{
		"run_start",
		"iteration_start",
		"build_start",
		"build_complete",
		"validation_start",
		"validation_pass",
		"review_start",
		"review_complete",
		"iteration_complete",
		"bead_complete",
		"run_complete",
	}

	if len(eventSequence) != len(expectedSequence) {
		t.Errorf("Event sequence length mismatch: got %d, expected %d", len(eventSequence), len(expectedSequence))
	}

	for i, expectedType := range expectedSequence {
		if i < len(eventSequence) && eventSequence[i] != expectedType {
			t.Errorf("Event %d: got %q, expected %q", i, eventSequence[i], expectedType)
		}
	}
}

// TestEventOrdering_FailurePath tests event ordering for a failed iteration with recovery.
func TestEventOrdering_FailurePath(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()

	var eventSequence []string
	captureSubscriber := &eventCaptureSubscriber{
		emitter:  emitter,
		sequence: &eventSequence,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = captureSubscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit failure path with escalation
	emitter.Emit(&events.RunStartEvent{MaxIterations: 2})
	emitter.Emit(&events.IterationStartEvent{Iteration: 1, BeadID: "b1", BeadTitle: "Test"})
	emitter.Emit(&events.BuildStartEvent{BeadID: "b1", Model: "haiku", Attempt: 1})
	emitter.Emit(&events.BuildCompleteEvent{BeadID: "b1", Success: false})
	emitter.Emit(&events.AnalysisStartEvent{BeadID: "b1"})
	emitter.Emit(&events.AnalysisCompleteEvent{BeadID: "b1", Category: "logic_error", Recoverable: true})
	emitter.Emit(&events.EscalationEvent{FromModel: "haiku", ToModel: "opus", Attempt: 2})
	emitter.Emit(&events.BuildStartEvent{BeadID: "b1", Model: "opus", Attempt: 2})
	emitter.Emit(&events.BuildCompleteEvent{BeadID: "b1", Success: true})
	emitter.Emit(&events.ValidationStartEvent{BeadID: "b1"})
	emitter.Emit(&events.ValidationPassEvent{BeadID: "b1"})
	emitter.Emit(&events.IterationCompleteEvent{Iteration: 1, BeadID: "b1", Success: true})
	emitter.Emit(&events.RunCompleteEvent{IterationsCompleted: 1, Reason: "success"})

	time.Sleep(50 * time.Millisecond)
	cancel()

	// Verify key events in sequence
	expectedPresent := []string{
		"run_start",
		"iteration_start",
		"build_start",
		"build_complete",
		"analysis_start",
		"analysis_complete",
		"escalation",
		"build_start",
		"validation_start",
		"iteration_complete",
		"run_complete",
	}

	for _, expected := range expectedPresent {
		found := false
		for _, got := range eventSequence {
			if got == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event %q not found in sequence", expected)
		}
	}
}

// TestCLIOutputParity tests that CLI output matches expected format.
func TestCLIOutputParity(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()

	output := &bytes.Buffer{}
	subscriber := cli.NewCLISubscriber(cli.BasicWriter(output), emitter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = subscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit representative events
	emitter.Emit(&events.IterationStartEvent{Iteration: 1, BeadID: "b1", BeadTitle: "Auth Task"})
	emitter.Emit(&events.BuildStartEvent{BeadID: "b1", Model: "opus", Attempt: 1})
	emitter.Emit(&events.HeartbeatEvent{Elapsed: 5 * time.Second, ToolCalls: 3})
	emitter.Emit(&events.ValidationStartEvent{BeadID: "b1"})
	emitter.Emit(&events.ValidationPassEvent{BeadID: "b1", Duration: 2 * time.Second})

	time.Sleep(50 * time.Millisecond)
	cancel()

	outputStr := output.String()

	// Verify key output patterns
	expectedPatterns := []string{
		"Iteration 1",
		"Auth Task",
		"Building",
		"opus",
		"Validating",
		"Validation PASS",
	}

	for _, pattern := range expectedPatterns {
		if !bytes.Contains([]byte(outputStr), []byte(pattern)) {
			t.Errorf("Expected output to contain %q, but got:\n%s", pattern, outputStr)
		}
	}
}

// TestSubscriberDrivenStatusUpdates tests that status.json is updated only by subscriber.
func TestSubscriberDrivenStatusUpdates(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()

	statusWriter := &testStatusWriter{updates: make(map[string]interface{})}
	subscriber := status.NewStatusSubscriber(statusWriter, emitter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = subscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit events
	emitter.Emit(&events.RunStartEvent{MaxIterations: 5})
	time.Sleep(20 * time.Millisecond)

	// Verify status was updated by subscriber
	if running, ok := statusWriter.updates["running"]; !ok || running != true {
		t.Errorf("Expected running=true in status, got %v", statusWriter.updates["running"])
	}

	if maxIter, ok := statusWriter.updates["max_iterations"]; !ok || maxIter != 5 {
		t.Errorf("Expected max_iterations=5 in status, got %v", statusWriter.updates["max_iterations"])
	}

	// Emit iteration event
	emitter.Emit(&events.IterationStartEvent{Iteration: 1, BeadID: "b1", BeadTitle: "Test"})
	time.Sleep(20 * time.Millisecond)

	if iter, ok := statusWriter.updates["iteration"]; !ok || iter != 1 {
		t.Errorf("Expected iteration=1 in status, got %v", statusWriter.updates["iteration"])
	}

	if beadID, ok := statusWriter.updates["bead_id"]; !ok || beadID != "b1" {
		t.Errorf("Expected bead_id=b1 in status, got %v", statusWriter.updates["bead_id"])
	}

	cancel()
}

// TestSubscriberDrivenTmuxUpdates tests that tmux is updated only by subscriber.
func TestSubscriberDrivenTmuxUpdates(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()

	tmuxManager := &testTmuxManager{}
	subscriber, err := tmux.NewTMUXSubscriber(tmuxManager, emitter)
	if err != nil {
		t.Fatalf("failed to create tmux subscriber: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = subscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit events
	emitter.Emit(&events.RunStartEvent{MaxIterations: 1})
	time.Sleep(20 * time.Millisecond)

	// Verify tmux title was updated
	if len(tmuxManager.titles) == 0 {
		t.Error("Expected tmux title to be updated, but no titles were set")
	}

	initialTitle := tmuxManager.titles[len(tmuxManager.titles)-1]
	if !bytes.Contains([]byte(initialTitle), []byte("gromit")) {
		t.Errorf("Expected tmux title to contain 'gromit', got %q", initialTitle)
	}

	// Emit iteration event
	emitter.Emit(&events.IterationStartEvent{Iteration: 2, BeadID: "b2", BeadTitle: "Checkout"})
	time.Sleep(20 * time.Millisecond)

	// Verify title was updated with iteration info
	latestTitle := tmuxManager.titles[len(tmuxManager.titles)-1]
	if !bytes.Contains([]byte(latestTitle), []byte("iteration")) {
		t.Errorf("Expected tmux title to contain 'iteration', got %q", latestTitle)
	}

	cancel()
}

// TestHeartbeatEventOrdering tests that heartbeat events are properly sequenced during build.
func TestHeartbeatEventOrdering(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	defer emitter.Close()

	var eventSequence []string
	captureSubscriber := &eventCaptureSubscriber{
		emitter:  emitter,
		sequence: &eventSequence,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = captureSubscriber.Start(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	// Emit build with heartbeats
	emitter.Emit(&events.IterationStartEvent{Iteration: 1, BeadID: "b1", BeadTitle: "Test"})
	emitter.Emit(&events.BuildStartEvent{BeadID: "b1", Model: "opus", Attempt: 1})
	emitter.Emit(&events.HeartbeatEvent{Elapsed: 1 * time.Second, ToolCalls: 1})
	emitter.Emit(&events.HeartbeatEvent{Elapsed: 2 * time.Second, ToolCalls: 2})
	emitter.Emit(&events.HeartbeatEvent{Elapsed: 3 * time.Second, ToolCalls: 3})
	emitter.Emit(&events.BuildCompleteEvent{BeadID: "b1", Success: true})

	time.Sleep(50 * time.Millisecond)
	cancel()

	// Verify heartbeats are interspersed between build events
	heartbeatCount := 0
	buildStartSeen := false
	buildCompleteSeen := false
	buildStartIdx := -1
	buildCompleteIdx := -1

	for i, eventType := range eventSequence {
		if eventType == "build_start" {
			buildStartIdx = i
			buildStartSeen = true
		}
		if eventType == "heartbeat" {
			heartbeatCount++
		}
		if eventType == "build_complete" {
			buildCompleteIdx = i
			buildCompleteSeen = true
		}
	}

	if !buildStartSeen || !buildCompleteSeen {
		t.Error("Missing build_start or build_complete events")
	}

	if buildStartIdx >= 0 && buildCompleteIdx >= 0 && buildStartIdx < buildCompleteIdx {
		// Heartbeats should be between build_start and build_complete
		heartbeatsBetween := 0
		for i := buildStartIdx + 1; i < buildCompleteIdx; i++ {
			if eventSequence[i] == "heartbeat" {
				heartbeatsBetween++
			}
		}
		if heartbeatsBetween != heartbeatCount {
			t.Errorf("Expected %d heartbeats between build_start and build_complete, got %d", heartbeatCount, heartbeatsBetween)
		}
	}
}

// eventCaptureSubscriber captures event types for testing.
type eventCaptureSubscriber struct {
	emitter  *events.Emitter
	sequence *[]string
}

func (e *eventCaptureSubscriber) Start(ctx context.Context) error {
	ch := e.emitter.Subscribe()
	defer e.emitter.Unsubscribe(ch)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			*e.sequence = append(*e.sequence, event.EventType())
		case <-ctx.Done():
			return nil
		}
	}
}
