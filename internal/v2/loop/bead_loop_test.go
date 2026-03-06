package loop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/stage"
	reviewstage "github.com/danabrams/gromit/internal/v2/stage/review"
)

func TestBeadLoopRunsStagesInOrder(t *testing.T) {
	t.Parallel()

	order := []string{}
	beads := []*bead.Bead{{ID: "bead-1"}}

	config := BeadLoopConfig{
		Gate:     newRecordingStage("gate", &order),
		Build:    newRecordingStage("build", &order),
		Validate: newRecordingStage("validate", &order),
		Review:   newRecordingStage("review", &order),
		Epilogue: newRecordingStage("epilogue", &order),
	}
	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	if err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expected := []string{"gate:bead-1", "build:bead-1", "validate:bead-1", "review:bead-1", "epilogue:bead-1"}
	for i, name := range expected {
		if i >= len(order) {
			t.Fatalf("missing stage execution %s", name)
		}
		if order[i] != name {
			t.Fatalf("stage %d = %s, want %s", i, order[i], name)
		}
	}
}

func TestBeadLoopRunsBeadsInDependencyOrder(t *testing.T) {
	t.Parallel()

	beadOrder := []string{}

	config := BeadLoopConfig{
		Gate:     &recordingStage{name: "gate", hook: func(id string) { beadOrder = append(beadOrder, id) }},
		Build:    newRecordingStage("build", nil),
		Validate: newRecordingStage("validate", nil),
		Review:   newRecordingStage("review", nil),
		Epilogue: newRecordingStage("epilogue", nil),
	}

	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{
		{ID: "child", DependsOn: []bead.Dependency{{ID: "root"}}},
		{ID: "root"},
	}

	if err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(beadOrder) != 2 {
		t.Fatalf("bead order = %v, want 2 entries", beadOrder)
	}
	if beadOrder[0] != "root" || beadOrder[1] != "child" {
		t.Fatalf("bead order = %v, want [root child]", beadOrder)
	}
}

func TestBeadLoopShortCircuitsToFailurePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		gateDecision  stage.Decision
		buildDecision stage.Decision
	}{
		{name: "gate skip", gateDecision: stage.DecisionSkip, buildDecision: stage.DecisionProceed},
		{name: "build fail", gateDecision: stage.DecisionProceed, buildDecision: stage.DecisionFail},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gate := &decisionStage{name: "gate", decision: tc.gateDecision}
			build := &decisionStage{name: "build", decision: tc.buildDecision}
			validate := &decisionStage{name: "validate", decision: stage.DecisionProceed}
			review := &decisionStage{name: "review", decision: stage.DecisionProceed}
			epilogue := &decisionStage{name: "epilogue", decision: stage.DecisionProceed}

			config := BeadLoopConfig{
				Gate:     gate,
				Build:    build,
				Validate: validate,
				Review:   review,
				Epilogue: epilogue,
			}

			loop, err := NewBeadLoop(config)
			if err != nil {
				t.Fatalf("NewBeadLoop: %v", err)
			}

			err = loop.Run(context.Background(), []*bead.Bead{{ID: "blocked"}}, nil)
			if err == nil {
				t.Fatalf("expected error for case %s", tc.name)
			}

			if gate.runCount != 1 {
				t.Fatalf("gate run count = %d, want 1", gate.runCount)
			}
			if tc.gateDecision == stage.DecisionSkip {
				if build.runCount != 0 || validate.runCount != 0 || review.runCount != 0 {
					t.Fatalf("expected build/validate/review skipped, got %d/%d/%d", build.runCount, validate.runCount, review.runCount)
				}
			} else {
				if build.runCount != 1 {
					t.Fatalf("build run count = %d, want 1", build.runCount)
				}
				if validate.runCount != 0 || review.runCount != 0 {
					t.Fatalf("validate/review should be skipped after failure, got %d/%d", validate.runCount, review.runCount)
				}
			}

			if len(epilogue.requests) != 1 {
				t.Fatalf("epilogue run count = %d, want 1", len(epilogue.requests))
			}
			if epilogue.requests[0].RetryContext == nil {
				t.Fatalf("epilogue called without retry context")
			}
		})
	}
}

func TestBeadLoopEmitsLifecycleEvents(t *testing.T) {
	t.Parallel()

	emitter := event.NewEmitter()
	ch := make(chan event.TypedEvent, 4)
	emitter.Subscribe(func(evt event.TypedEvent) {
		ch <- evt
	})

	config := BeadLoopConfig{
		Gate:     newRecordingStage("gate", nil),
		Build:    newRecordingStage("build", nil),
		Validate: newRecordingStage("validate", nil),
		Review:   newRecordingStage("review", nil),
		Epilogue: newRecordingStage("epilogue", nil),
		Emitter:  emitter,
	}

	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "event-bead", Title: "Eventful"}}
	if err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	startedEvt := <-ch
	started, ok := startedEvt.(event.BeadStartedEvent)
	if !ok {
		t.Fatalf("first event type = %T, want event.BeadStartedEvent", startedEvt)
	}
	if started.BeadID != "event-bead" || started.BeadTitle != "Eventful" {
		t.Fatalf("unexpected started event: %#v", started)
	}

	var completed event.BeadCompletedEvent
	for {
		next := <-ch
		if evt, ok := next.(event.BeadCompletedEvent); ok {
			completed = evt
			break
		}
	}
	if !completed.Success {
		t.Fatalf("expected success completion event, got %#v", completed)
	}
}

func TestBeadLoopBridgesToLegacyEmitter(t *testing.T) {
	t.Parallel()

	typed := event.NewEmitter()
	typedEvents := make(chan event.TypedEvent, 8)
	typed.Subscribe(func(evt event.TypedEvent) {
		select {
		case typedEvents <- evt:
		default:
		}
	})
	legacy := events.NewEmitter()
	ch := legacy.Subscribe()
	defer legacy.Unsubscribe(ch)

	config := BeadLoopConfig{
		Gate:          newNoopStage("gate"),
		Build:         newNoopStage("build"),
		Validate:      newNoopStage("validate"),
		Review:        newNoopStage("review"),
		Epilogue:      newNoopStage("epilogue"),
		Emitter:       typed,
		LegacyEmitter: legacy,
	}

	beads := []*bead.Bead{{ID: "bridge-bead"}}
	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	if err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	select {
	case evt := <-typedEvents:
		if _, ok := evt.(event.BeadStartedEvent); !ok {
			t.Fatalf("typed event type = %T, want event.BeadStartedEvent", evt)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected typed emitter to emit events")
	}
	select {
	case evt := <-ch:
		if _, ok := evt.(*events.IterationStartEvent); !ok {
			t.Fatalf("legacy event type = %T, want *events.IterationStartEvent", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for legacy event")
	}
}

func TestBeadLoopPopulatesIterationOnStageRequests(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{{ID: "first"}, {ID: "second"}}

	gate := newCapturingStage("gate")
	build := newCapturingStage("build")
	validate := newCapturingStage("validate")
	review := newCapturingStage("review")
	epilogue := newCapturingStage("epilogue")

	config := BeadLoopConfig{
		Gate:     gate,
		Build:    build,
		Validate: validate,
		Review:   review,
		Epilogue: epilogue,
	}
	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	if err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	stages := map[string]*capturingStage{
		"gate":     gate,
		"build":    build,
		"validate": validate,
		"review":   review,
		"epilogue": epilogue,
	}

	for name, stage := range stages {
		if len(stage.requests) != len(beads) {
			t.Fatalf("stage %s executed = %d, want %d", name, len(stage.requests), len(beads))
		}
		for idx, req := range stage.requests {
			iteration := idx + 1
			if req.Iteration != iteration {
				t.Fatalf("stage %s iteration = %d, want %d", name, req.Iteration, iteration)
			}
		}
	}
}

func TestBeadLoopEnforcesGenerationCap(t *testing.T) {
	t.Parallel()

	emitter := event.NewEmitter()
	ch := make(chan event.TypedEvent, 4)
	emitter.Subscribe(func(evt event.TypedEvent) {
		ch <- evt
	})

	config := BeadLoopConfig{
		Gate:            newNoopStage("gate"),
		Build:           newNoopStage("build"),
		Validate:        newNoopStage("validate"),
		Review:          newNoopStage("review"),
		Epilogue:        newNoopStage("epilogue"),
		Emitter:         emitter,
		GenerationCap:   3,
		StartGeneration: 0,
	}
	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	// Beads at generation 2, which means next generation would be 3 (at cap)
	beads := []*bead.Bead{
		{ID: "bead-at-gen-2", Labels: []string{"gen:2"}},
	}

	err = loop.Run(context.Background(), beads, nil)
	if err == nil {
		t.Fatal("expected Run to return error when generation cap reached")
	}

	// Check that GenerationCapReached event was emitted
	var foundEvent bool
	for i := 0; i < 2; i++ {
		select {
		case evt := <-ch:
			if _, ok := evt.(event.GenerationCapReachedEvent); ok {
				foundEvent = true
				break
			}
		case <-time.After(100 * time.Millisecond):
			break
		}
	}
	if !foundEvent {
		t.Fatal("expected GenerationCapReached event to be emitted")
	}
}

func TestBeadLoopStopsWhenReviewCreatesBeadsAtGenerationCap(t *testing.T) {
	t.Parallel()

	emitter := event.NewEmitter()
	ch := make(chan event.TypedEvent, 4)
	emitter.Subscribe(func(evt event.TypedEvent) {
		ch <- evt
	})

	reviewStage := &scriptedReviewStage{
		name: "review",
		result: &stage.Result{
			Decision: stage.DecisionProceed,
			Artifacts: &reviewstage.ReviewArtifacts{
				CreatedBeads: []*tasktracker.Bead{{
					ID:     "child",
					Labels: []string{generation.Format(2)},
				}},
			},
		},
	}

	config := BeadLoopConfig{
		Gate:            newNoopStage("gate"),
		Build:           newNoopStage("build"),
		Validate:        newNoopStage("validate"),
		Review:          reviewStage,
		Epilogue:        newNoopStage("epilogue"),
		Emitter:         emitter,
		GenerationCap:   2,
		StartGeneration: 0,
	}
	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "parent", Labels: []string{generation.Format(0)}}}
	err = loop.Run(context.Background(), beads, nil)
	if !errors.Is(err, ErrGenerationCapReached) {
		t.Fatalf("Run error = %v, want ErrGenerationCapReached", err)
	}

	var foundEvent bool
	for i := 0; i < 2; i++ {
		select {
		case evt := <-ch:
			if _, ok := evt.(event.GenerationCapReachedEvent); ok {
				foundEvent = true
				break
			}
		case <-time.After(100 * time.Millisecond):
			break
		}
	}
	if !foundEvent {
		t.Fatal("expected GenerationCapReached event to be emitted")
	}
}

func TestBeadLoopStopsWhenStopChannelCloses(t *testing.T) {
	t.Parallel()

	stopCh := make(chan struct{})
	gate := &closingStage{name: "gate", stopCh: stopCh}
	build := &decisionStage{name: "build", decision: stage.DecisionProceed}
	validate := &decisionStage{name: "validate", decision: stage.DecisionProceed}
	review := &decisionStage{name: "review", decision: stage.DecisionProceed}
	epilogue := &decisionStage{name: "epilogue", decision: stage.DecisionProceed}

	config := BeadLoopConfig{
		Gate:     gate,
		Build:    build,
		Validate: validate,
		Review:   review,
		Epilogue: epilogue,
	}

	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{
		{ID: "first"},
		{ID: "second"},
	}

	if err := loop.Run(context.Background(), beads, stopCh); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if gate.runCount != 1 {
		t.Fatalf("gate run count = %d, want 1", gate.runCount)
	}
	if build.runCount != 1 {
		t.Fatalf("build run count = %d, want 1", build.runCount)
	}
	if review.runCount != 1 {
		t.Fatalf("review run count = %d, want 1", review.runCount)
	}
}

func collectLifecycleEvents(ch chan event.TypedEvent, expected int) []event.TypedEvent {
	collected := make([]event.TypedEvent, 0, expected)
	for len(collected) < expected {
		collected = append(collected, <-ch)
	}
	return collected
}

func newRecordingStage(name string, order *[]string) stage.Stage {
	hook := func(string) {}
	if order != nil {
		hook = func(beadID string) {
			*order = append(*order, name+":"+beadID)
		}
	}
	return &recordingStage{name: name, hook: hook}
}

type recordingStage struct {
	name string
	hook func(string)
}

func (s *recordingStage) Name() string {
	return s.name
}

func (s *recordingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if s.hook != nil {
		s.hook(req.Bead.ID)
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type noopStage struct {
	name string
}

func newNoopStage(name string) stage.Stage {
	return &noopStage{name: name}
}

func (s *noopStage) Name() string {
	return s.name
}

func (s *noopStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type decisionStage struct {
	name     string
	decision stage.Decision
	err      error
	runCount int
	requests []*stage.Request
}

func (s *decisionStage) Name() string {
	return s.name
}

func (s *decisionStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	return &stage.Result{Decision: s.decision}, nil
}

type capturingStage struct {
	name     string
	requests []stage.Request
}

func newCapturingStage(name string) *capturingStage {
	return &capturingStage{name: name}
}

func (s *capturingStage) Name() string { return s.name }

func (s *capturingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if req != nil {
		s.requests = append(s.requests, *req)
	} else {
		s.requests = append(s.requests, stage.Request{})
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type closingStage struct {
	name     string
	stopCh   chan struct{}
	runCount int
}

func (s *closingStage) Name() string { return s.name }

func (s *closingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	if s.runCount == 1 && s.stopCh != nil {
		close(s.stopCh)
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type scriptedReviewStage struct {
	name   string
	result *stage.Result
	runCount int
}

func (s *scriptedReviewStage) Name() string { return s.name }

func (s *scriptedReviewStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	return s.result, nil
}
