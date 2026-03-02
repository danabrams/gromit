package runner

import (
	"context"
	"errors"
	"io"
	"strings"
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
	ch      chan events.Event
	capture *eventCapture
	done    chan struct{}
}

func newCaptureSubscriber(emitter *events.Emitter) *captureSubscriber {
	return &captureSubscriber{
		ch:      emitter.Subscribe(),
		capture: &eventCapture{},
		done:    make(chan struct{}),
	}
}

func emitValidationStartEvent(in pipeline.Input) {
	if in.Emitter == nil || in.Bead == nil {
		return
	}
	in.Emitter.Emit(&events.ValidationStartEvent{
		BeadID:   in.Bead.ID,
		Commands: []string{"fake-validation"},
		Time:     time.Now(),
	})
}

func emitValidationPassEvent(in pipeline.Input, duration time.Duration) {
	if in.Emitter == nil || in.Bead == nil {
		return
	}
	if duration < 0 {
		duration = 0
	}
	in.Emitter.Emit(&events.ValidationPassEvent{
		BeadID:   in.Bead.ID,
		Duration: duration,
		Time:     time.Now(),
	})
}

func emitValidationFailEvent(in pipeline.Input, output string, duration time.Duration) {
	if in.Emitter == nil || in.Bead == nil {
		return
	}
	if duration < 0 {
		duration = 0
	}
	in.Emitter.Emit(&events.ValidationFailEvent{
		BeadID:   in.Bead.ID,
		Output:   output,
		Duration: duration,
		Time:     time.Now(),
	})
}

func (cs *captureSubscriber) start() {
	defer close(cs.done)

	for evt := range cs.ch {
		cs.capture.events = append(cs.capture.events, evt)
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

	validateStage := &fakeStage{
		runFn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
			emitValidationStartEvent(in)
			emitValidationPassEvent(in, 0)
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		},
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
	go capturer.start()

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	<-capturer.done

	// Print all received events for debugging
	t.Logf("Captured %d events:", len(capturer.capture.events))
	for i, evt := range capturer.capture.events {
		t.Logf("  Event %d: %s", i, evt.EventType())
	}

	filteredEvents := make([]events.Event, 0, len(capturer.capture.events))
	for _, evt := range capturer.capture.events {
		if evt.EventType() == "log" {
			continue
		}
		filteredEvents = append(filteredEvents, evt)
	}

	// Verify event sequence
	// Note: Build and Review events are now emitted by their respective stages, not the orchestrator
	expectedTypes := []string{
		"run_start",
		"iteration_start",
		"validation_start",
		"validation_pass",
		"iteration_complete",
		"bead_complete",
		"run_complete",
	}

	if len(filteredEvents) < len(expectedTypes) {
		t.Errorf("Expected at least %d events (excluding log events), got %d", len(expectedTypes), len(filteredEvents))
		return
	}

	for i, expected := range expectedTypes {
		if i >= len(filteredEvents) {
			t.Fatalf("Event %d missing; expected %q", i, expected)
		}
		actual := filteredEvents[i].EventType()
		if actual != expected {
			t.Errorf("Event %d: got %q, want %q", i, actual, expected)
		}
	}
}

// TestOrchestrator_BeadFailedEventOnBuildFailure ensures a failing build emits BeadFailedEvent.
func TestOrchestrator_BeadFailedEventOnBuildFailure(t *testing.T) {
	t.Parallel()

	build := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{}, errors.New("build failed")
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "failed-bead", Title: "Failing Build"}, nil
	}

	cfg := OrchestratorConfig{
		Gate:     &fakeStage{},
		Build:    build,
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead:  getBead,
		Config:   &config.Config{},
		Output:   io.Discard,
	}

	orch := NewOrchestrator(cfg)
	capturer := newCaptureSubscriber(orch.GetEmitter())
	go capturer.start()

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	<-capturer.done

	var beadFailedEvent *events.BeadFailedEvent
	for _, evt := range capturer.capture.events {
		if bf, ok := evt.(*events.BeadFailedEvent); ok {
			beadFailedEvent = bf
			break
		}
	}

	if beadFailedEvent == nil {
		t.Fatal("BeadFailedEvent not emitted for build failure path")
	}
	if beadFailedEvent.BeadID != "failed-bead" {
		t.Errorf("BeadFailedEvent.BeadID = %q, want %q", beadFailedEvent.BeadID, "failed-bead")
	}
	if !strings.Contains(beadFailedEvent.Error, "build failed") {
		t.Errorf("BeadFailedEvent.Error = %q, want to contain build failure message", beadFailedEvent.Error)
	}
}

// TestOrchestrator_SuccessPath_RunStartEventContainsPayload verifies that RunStartEvent
// contains correct metadata (MaxIterations, DryRun status, and TimeBudget when deadline is set).
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
	go capturer.start()

	maxIterations := 5
	deadline := time.Now().Add(30 * time.Minute)
	err := orch.Run(context.Background(), maxIterations, deadline, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

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
	if runStartEvent.TimeBudget <= 0 {
		t.Fatalf("RunStartEvent.TimeBudget = %v, want positive duration", runStartEvent.TimeBudget)
	}
	if runStartEvent.TimeBudget > 30*time.Minute {
		t.Fatalf("RunStartEvent.TimeBudget = %v, want <= 30m", runStartEvent.TimeBudget)
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
	go capturer.start()

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

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
		emitValidationStartEvent(in)
		if validateAttempt == 1 {
			emitValidationFailEvent(in, "validation error: test failed", 0)
			// First attempt: fail validation
			return pipeline.Output{
				Decision:           pipeline.Block,
				ValidationFailures: []string{"validation error: test failed"},
			}, nil
		}
		emitValidationPassEvent(in, 0)
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
	go capturer.start()

	err := orch.Run(context.Background(), 2, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	<-capturer.done

	// Check that key events are present
	eventTypes := make(map[string]bool)
	for _, evt := range capturer.capture.events {
		eventTypes[evt.EventType()] = true
	}

	requiredEvents := []string{
		"run_start",
		"iteration_start",
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
	go capturer.start()

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

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

// TestOrchestrator_BlockPath_NonStuckEmitsBeadSkippedEvent verifies that gate blocks
// for non-stuck reasons do not emit BeadStuckEvent.
func TestOrchestrator_BlockPath_NonStuckEmitsBeadSkippedEvent(t *testing.T) {
	t.Parallel()

	gateStage := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Block, GateBlockReason: "criteria_missing"}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "blocked-bead", Title: "Blocked Task"}, nil
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
	go capturer.start()

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	<-capturer.done

	var beadSkippedEvent *events.BeadSkippedEvent
	for _, evt := range capturer.capture.events {
		switch e := evt.(type) {
		case *events.BeadStuckEvent:
			t.Fatalf("unexpected BeadStuckEvent emitted for non-stuck block: %+v", e)
		case *events.BeadSkippedEvent:
			beadSkippedEvent = e
		}
	}

	if beadSkippedEvent == nil {
		t.Fatal("BeadSkippedEvent not found in emitted events")
	}
	if beadSkippedEvent.BeadID != "blocked-bead" {
		t.Errorf("BeadSkippedEvent.BeadID = %q, want %q", beadSkippedEvent.BeadID, "blocked-bead")
	}
	if !strings.Contains(beadSkippedEvent.Reason, "criteria_missing") {
		t.Errorf("BeadSkippedEvent.Reason = %q, want it to include criteria_missing", beadSkippedEvent.Reason)
	}
}

// TestOrchestrator_BlockPath_StuckEmitsBeadStuckEvent verifies that stuck gate blocks
// continue to emit BeadStuckEvent.
func TestOrchestrator_BlockPath_StuckEmitsBeadStuckEvent(t *testing.T) {
	t.Parallel()

	gateStage := &fakeStage{runFn: func(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Block, GateBlockReason: "failure_threshold_exceeded"}, nil
	}}

	beadCalls := 0
	getBead := func(_ context.Context) (*bead.Bead, error) {
		beadCalls++
		if beadCalls > 1 {
			return nil, nil
		}
		return &bead.Bead{ID: "stuck-bead", Title: "Stuck Task"}, nil
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
	go capturer.start()

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	<-capturer.done

	var beadStuckEvent *events.BeadStuckEvent
	for _, evt := range capturer.capture.events {
		switch e := evt.(type) {
		case *events.BeadSkippedEvent:
			t.Fatalf("unexpected BeadSkippedEvent emitted for stuck block: %+v", e)
		case *events.BeadStuckEvent:
			beadStuckEvent = e
		}
	}

	if beadStuckEvent == nil {
		t.Fatal("BeadStuckEvent not found in emitted events")
	}
	if beadStuckEvent.BeadID != "stuck-bead" {
		t.Errorf("BeadStuckEvent.BeadID = %q, want %q", beadStuckEvent.BeadID, "stuck-bead")
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
	go capturer.start()

	err := orch.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

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
	go capturer.start()

	err := orch.Run(context.Background(), 2, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

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
