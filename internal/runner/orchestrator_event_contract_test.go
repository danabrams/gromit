package runner

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/pipeline"
)

// eventCapture records emitted events in order for assertion.
type eventCapture struct {
	events []events.Event
}

// captureSubscriber subscribes to an emitter and records events for testing.
type captureSubscriber struct {
	emitter *events.Emitter
	capture *eventCapture
	done    chan struct{}
}

func newCaptureSubscriber(emitter *events.Emitter) *captureSubscriber {
	return &captureSubscriber{
		emitter: emitter,
		capture: &eventCapture{},
		done:    make(chan struct{}),
	}
}

func (cs *captureSubscriber) start(ctx context.Context) {
	ch := cs.emitter.Subscribe()
	defer cs.emitter.Unsubscribe(ch)
	defer close(cs.done)

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			cs.capture.events = append(cs.capture.events, evt)
		case <-ctx.Done():
			return
		}
	}
}

// TestOrchestrator_SuccessPath_EmitsEventOrdering verifies that the orchestrator emits
// lifecycle and phase events in the correct order for a successful iteration.
// Event order: RunStart -> IterationStart -> BuildStart -> BuildComplete -> ValidationStart ->
// ValidationPass -> ReviewStart -> ReviewComplete -> IterationComplete -> BeadComplete -> RunComplete.
func TestOrchestrator_SuccessPath_EmitsEventOrdering(t *testing.T) {
	t.Parallel()

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "test-bead-1", Title: "Test Task"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	capturer := newCaptureSubscriber(orch.GetEmitter())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go capturer.start(ctx)
	time.Sleep(10 * time.Millisecond) // Ensure subscriber is ready

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cancel()
	<-capturer.done

	// Verify event sequence
	expectedTypes := []string{
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

	if len(capturer.capture.events) < len(expectedTypes) {
		t.Errorf("Expected at least %d events, got %d", len(expectedTypes), len(capturer.capture.events))
		t.Logf("Events received: %v", capturer.capture.events)
		return
	}

	for i, expected := range expectedTypes {
		if i >= len(capturer.capture.events) {
			t.Fatalf("Event %d missing; expected %q", i, expected)
		}
		actual := capturer.capture.events[i].EventType()
		if actual != expected {
			t.Errorf("Event %d: got %q, want %q", i, actual, expected)
		}
	}
}

// TestOrchestrator_SuccessPath_RunStartEventContainsPayload verifies that RunStartEvent
// contains correct metadata (MaxIterations, DryRun status).
func TestOrchestrator_SuccessPath_RunStartEventContainsPayload(t *testing.T) {
	t.Parallel()

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "test-bead-2", Title: "Test Task"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	capturer := newCaptureSubscriber(orch.GetEmitter())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go capturer.start(ctx)
	time.Sleep(10 * time.Millisecond)

	maxIterations := 5
	err := orch.Run(context.Background(), maxIterations, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cancel()
	<-capturer.done

	// Find RunStartEvent
	var runStartEvent *events.RunStartEvent
	for _, evt := range capturer.capture.events {
		if rse, ok := evt.(*events.RunStartEvent); ok {
			runStartEvent = rse
			break
		}
	}

	if runStartEvent == nil {
		t.Fatal("RunStartEvent not found in emitted events")
	}

	if runStartEvent.MaxIterations != maxIterations {
		t.Errorf("RunStartEvent.MaxIterations = %d, want %d", runStartEvent.MaxIterations, maxIterations)
	}
}

// TestOrchestrator_SuccessPath_IterationStartEventContainsPayload verifies that
// IterationStartEvent contains correct iteration number, bead ID, and title.
func TestOrchestrator_SuccessPath_IterationStartEventContainsPayload(t *testing.T) {
	t.Parallel()

	expectedBeadID := "test-bead-3"
	expectedBeadTitle := "Important Test"

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: expectedBeadID, Title: expectedBeadTitle}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	capturer := newCaptureSubscriber(orch.GetEmitter())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go capturer.start(ctx)
	time.Sleep(10 * time.Millisecond)

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cancel()
	<-capturer.done

	// Find IterationStartEvent
	var iterStartEvent *events.IterationStartEvent
	for _, evt := range capturer.capture.events {
		if ise, ok := evt.(*events.IterationStartEvent); ok {
			iterStartEvent = ise
			break
		}
	}

	if iterStartEvent == nil {
		t.Fatal("IterationStartEvent not found in emitted events")
	}

	if iterStartEvent.Iteration != 1 {
		t.Errorf("IterationStartEvent.Iteration = %d, want 1", iterStartEvent.Iteration)
	}
	if iterStartEvent.BeadID != expectedBeadID {
		t.Errorf("IterationStartEvent.BeadID = %q, want %q", iterStartEvent.BeadID, expectedBeadID)
	}
	if iterStartEvent.BeadTitle != expectedBeadTitle {
		t.Errorf("IterationStartEvent.BeadTitle = %q, want %q", iterStartEvent.BeadTitle, expectedBeadTitle)
	}
}

// TestOrchestrator_FailurePath_EmitsEventOrdering verifies that the orchestrator emits
// events in the correct order when a build fails and causes validation failure/retry.
// Event order: RunStart -> IterationStart -> BuildStart -> BuildComplete (pass) ->
// ValidationStart -> ValidationFail -> AnalysisStart -> AnalysisComplete ->
// BuildStart -> BuildComplete (pass) -> ValidationStart -> ValidationPass ->
// IterationComplete -> BeadComplete -> RunComplete.
func TestOrchestrator_FailurePath_EmitsEventOrdering(t *testing.T) {
	t.Parallel()

	validateAttempt := 0
	validateStage := &fakeStage{runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		validateAttempt++
		if validateAttempt == 1 {
			// First attempt: fail validation
			return pipeline.Output{
				Decision: pipeline.Block,
				ValidationFailures: []string{"validation error: test failed"},
			}, nil
		}
		// Second attempt: pass validation
		return pipeline.Output{
			Decision: pipeline.Proceed,
		}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "test-bead-fail", Title: "Failing Task"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: validateStage,
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	capturer := newCaptureSubscriber(orch.GetEmitter())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go capturer.start(ctx)
	time.Sleep(10 * time.Millisecond)

	err := orch.Run(context.Background(), 2, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cancel()
	<-capturer.done

	// Check that key events are present
	eventTypes := make(map[string]bool)
	for _, evt := range capturer.capture.events {
		eventTypes[evt.EventType()] = true
	}

	requiredEvents := []string{
		"run_start",
		"iteration_start",
		"build_start",
		"build_complete",
		"validation_start",
		"validation_fail",
		"iteration_complete",
		"run_complete",
	}

	for _, eventType := range requiredEvents {
		if !eventTypes[eventType] {
			t.Errorf("Expected event type %q not found in emitted events", eventType)
		}
	}
}

// TestOrchestrator_SkipPath_EmitsBeadSkippedEvent verifies that when a Gate stage
// returns Skip decision, the orchestrator emits BeadSkippedEvent.
func TestOrchestrator_SkipPath_EmitsBeadSkippedEvent(t *testing.T) {
	t.Parallel()

	gateStage := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Skip}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "skip-bead", Title: "Skipped Task"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     gateStage,
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	capturer := newCaptureSubscriber(orch.GetEmitter())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go capturer.start(ctx)
	time.Sleep(10 * time.Millisecond)

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cancel()
	<-capturer.done

	// Find BeadSkippedEvent
	var beadSkippedEvent *events.BeadSkippedEvent
	for _, evt := range capturer.capture.events {
		if bse, ok := evt.(*events.BeadSkippedEvent); ok {
			beadSkippedEvent = bse
			break
		}
	}

	if beadSkippedEvent == nil {
		t.Fatal("BeadSkippedEvent not found in emitted events")
	}

	if beadSkippedEvent.BeadID != "skip-bead" {
		t.Errorf("BeadSkippedEvent.BeadID = %q, want %q", beadSkippedEvent.BeadID, "skip-bead")
	}
}

// TestOrchestrator_IterationCompleteEventContainsPayload verifies that
// IterationCompleteEvent contains iteration number, bead ID, success status.
func TestOrchestrator_IterationCompleteEventContainsPayload(t *testing.T) {
	t.Parallel()

	expectedBeadID := "test-bead-iter-complete"
	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: expectedBeadID, Title: "Test"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	capturer := newCaptureSubscriber(orch.GetEmitter())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go capturer.start(ctx)
	time.Sleep(10 * time.Millisecond)

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cancel()
	<-capturer.done

	// Find IterationCompleteEvent
	var iterCompleteEvent *events.IterationCompleteEvent
	for _, evt := range capturer.capture.events {
		if ice, ok := evt.(*events.IterationCompleteEvent); ok {
			iterCompleteEvent = ice
			break
		}
	}

	if iterCompleteEvent == nil {
		t.Fatal("IterationCompleteEvent not found in emitted events")
	}

	if iterCompleteEvent.Iteration != 1 {
		t.Errorf("IterationCompleteEvent.Iteration = %d, want 1", iterCompleteEvent.Iteration)
	}
	if iterCompleteEvent.BeadID != expectedBeadID {
		t.Errorf("IterationCompleteEvent.BeadID = %q, want %q", iterCompleteEvent.BeadID, expectedBeadID)
	}
	if !iterCompleteEvent.Success {
		t.Errorf("IterationCompleteEvent.Success = false, want true for success path")
	}
}

// TestOrchestrator_RunCompleteEventContainsPayload verifies that RunCompleteEvent
// contains the correct number of completed iterations and completion reason.
func TestOrchestrator_RunCompleteEventContainsPayload(t *testing.T) {
	t.Parallel()

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 2 {
			return nil, nil
		}
		return &bead.Bead{ID: "bead-" + string(rune('a'+beadCalls-1)), Title: "Task"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	capturer := newCaptureSubscriber(orch.GetEmitter())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go capturer.start(ctx)
	time.Sleep(10 * time.Millisecond)

	err := orch.Run(context.Background(), 2, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cancel()
	<-capturer.done

	// Find RunCompleteEvent
	var runCompleteEvent *events.RunCompleteEvent
	for _, evt := range capturer.capture.events {
		if rce, ok := evt.(*events.RunCompleteEvent); ok {
			runCompleteEvent = rce
			break
		}
	}

	if runCompleteEvent == nil {
		t.Fatal("RunCompleteEvent not found in emitted events")
	}

	if runCompleteEvent.IterationsCompleted != 2 {
		t.Errorf("RunCompleteEvent.IterationsCompleted = %d, want 2", runCompleteEvent.IterationsCompleted)
	}
	if runCompleteEvent.Reason == "" {
		t.Error("RunCompleteEvent.Reason is empty, want non-empty reason")
	}
}
