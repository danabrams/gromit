package loop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/stage"
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

	if err := loop.Run(context.Background(), beads); err != nil {
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

	if err := loop.Run(context.Background(), beads); err != nil {
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

			err = loop.Run(context.Background(), []*bead.Bead{{ID: "blocked"}})
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
	if err := loop.Run(context.Background(), beads); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	events := collectLifecycleEvents(ch, 2)
	started, ok := events[0].(event.BeadStartedEvent)
	if !ok {
		t.Fatalf("first event type = %T, want event.BeadStartedEvent", events[0])
	}
	if started.BeadID != "event-bead" || started.BeadTitle != "Eventful" {
		t.Fatalf("unexpected started event: %#v", started)
	}

	completed, ok := events[1].(event.BeadCompletedEvent)
	if !ok {
		t.Fatalf("second event type = %T, want event.BeadCompletedEvent", events[1])
	}
	if !completed.Success {
		t.Fatalf("expected success completion event, got %#v", completed)
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
