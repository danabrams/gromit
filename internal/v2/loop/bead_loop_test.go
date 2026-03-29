package loop

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/stage"
	epiloguestage "github.com/danabrams/gromit/internal/v2/stage/epilogue"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
	reviewstage "github.com/danabrams/gromit/internal/v2/stage/review"
	"github.com/danabrams/gromit/internal/v2/stage/triage"
	stagevalidate "github.com/danabrams/gromit/internal/v2/stage/validate"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
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

	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
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

	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
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

	gate := &decisionStage{name: "gate", decision: stage.DecisionProceed}
	build := &decisionStage{name: "build", decision: stage.DecisionFail}
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

	// Build failure is recoverable — the loop completes without error
	// after skipping the failed bead.
	_, err = loop.Run(context.Background(), []*bead.Bead{{ID: "blocked"}}, nil)
	if err != nil {
		t.Fatalf("expected loop to continue past build fail, got error: %v", err)
	}

	if gate.runCount != 1 {
		t.Fatalf("gate run count = %d, want 1", gate.runCount)
	}
	if build.runCount != 1 {
		t.Fatalf("build run count = %d, want 1", build.runCount)
	}
	if validate.runCount != 0 || review.runCount != 0 {
		t.Fatalf("validate/review should be skipped after failure, got %d/%d", validate.runCount, review.runCount)
	}

	if len(epilogue.requests) != 1 {
		t.Fatalf("epilogue run count = %d, want 1", len(epilogue.requests))
	}
	if epilogue.requests[0].RetryContext == nil {
		t.Fatalf("epilogue called without retry context")
	}
}

func TestBeadLoopRetryWithRunsBuildBeforeRetryAndReportsAttempt(t *testing.T) {
	t.Parallel()

	order := []string{}
	emitter := event.NewEmitter()
	ch := make(chan event.TypedEvent, 16)
	emitter.Subscribe(func(evt event.TypedEvent) {
		select {
		case ch <- evt:
		default:
		}
	})

	gate := newRecordingStage("gate", &order)
	build := newRecordingStage("build", &order)
	validate := newRetryStage("validate", 1, func(beadID string, attempt int) {
		label := "validate:" + beadID
		if attempt > 1 {
			label = "retry-validate:" + beadID
		}
		order = append(order, label)
	})
	validate.retryConfig = stage.RetryConfig{
		MaxRetries: 1,
		RetryWith:  []string{"build"},
	}
	review := newRecordingStage("review", &order)
	epilogue := newRecordingStage("epilogue", &order)

	config := BeadLoopConfig{
		Gate:     gate,
		Build:    build,
		Validate: validate,
		Review:   review,
		Epilogue: epilogue,
		Emitter:  emitter,
	}
	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "retry"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expected := []string{
		"gate:retry",
		"build:retry",
		"validate:retry",
		"build:retry",
		"retry-validate:retry",
		"review:retry",
		"epilogue:retry",
	}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, want := range expected {
		if order[i] != want {
			t.Fatalf("order[%d] = %s, want %s", i, order[i], want)
		}
	}

	if len(validate.requests) != 2 {
		t.Fatalf("validate invoked %d times, want 2", len(validate.requests))
	}
	retryCtx := validate.requests[1].RetryContext
	if retryCtx == nil {
		t.Fatalf("expected retry context on second validate attempt")
	}
	if retryCtx.Attempt != 2 {
		t.Fatalf("retry attempt = %d, want 2", retryCtx.Attempt)
	}
	if len(retryCtx.PriorFailures) != 1 {
		t.Fatalf("prior failures = %d, want 1", len(retryCtx.PriorFailures))
	}
	if retryCtx.PriorFailures[0] != "validate returned fail" {
		t.Fatalf("unexpected prior failure reason: %s", retryCtx.PriorFailures[0])
	}

	var retryEvt event.StageRetryingEvent
	found := false
	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()
	for !found {
		select {
		case evt := <-ch:
			if e, ok := evt.(event.StageRetryingEvent); ok {
				retryEvt = e
				found = true
			}
		case <-timer.C:
			t.Fatal("missing StageRetrying event")
		}
	}
	if retryEvt.StageName != "validate" || retryEvt.Attempt != 2 {
		t.Fatalf("retry event = %#v, want stage=validate attempt=2", retryEvt)
	}
}

func TestBeadLoopValidateRetriesBuildOnFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Validation.Commands = []string{"validate"}
	cfg.Validation.MaxValidationRetries = 1

	runner := &failingValidationRunner{failures: 1}
	validateStage, err := stagevalidate.New(cfg, runner)
	if err != nil {
		t.Fatalf("stagevalidate.New: %v", err)
	}

	gateName := stagedesc.Describe("gate", cfg)
	buildName := stagedesc.Describe("build", cfg)
	reviewName := stagedesc.Describe("review", cfg)
	epilogueName := stagedesc.Describe("epilogue", cfg)
	order := []string{}
	config := BeadLoopConfig{
		Gate:     newRecordingStage(gateName, &order),
		Build:    newRecordingStage(buildName, &order),
		Validate: validateStage,
		Review:   newRecordingStage(reviewName, &order),
		Epilogue: newRecordingStage(epilogueName, &order),
	}
	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "spec"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if runner.runCount != 2 {
		t.Fatalf("validation runner run count = %d, want 2", runner.runCount)
	}

	expected := []string{
		gateName + ":spec",
		buildName + ":spec",
		buildName + ":spec",
		reviewName + ":spec",
		epilogueName + ":spec",
	}
	if !reflect.DeepEqual(order, expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
}

func TestBeadLoopStageRequestIncludesWorktree(t *testing.T) {
	t.Parallel()

	worktree := "/tmp/worktree"
	validate := newRetryStage("validate", 0, nil)
	config := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    newNoopStage("build"),
		Validate: validate,
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
	}
	loop, err := NewBeadLoop(config)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}
	loop.SetWorktree(worktree)

	if _, err := loop.Run(context.Background(), []*bead.Bead{{ID: "spec"}}, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(validate.requests) == 0 {
		t.Fatalf("validate stage not run")
	}
	if need := worktree; validate.requests[0].Worktree != need {
		t.Fatalf("validate worktree = %q, want %q", validate.requests[0].Worktree, need)
	}
}

func TestBeadLoopStageErrorPropagation(t *testing.T) {
	t.Parallel()

	stageErr := fmt.Errorf("infrastructure failure")

	// Fatal errors: gate and epilogue infrastructure errors abort the loop.
	fatalCases := []struct {
		name       string
		errStage   string
		wantSubstr string
	}{
		{name: "gate error", errStage: "gate", wantSubstr: "gate"},
		{name: "epilogue error", errStage: "epilogue", wantSubstr: "epilogue"},
	}

	for _, tc := range fatalCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			makeStage := func(name string) stage.Stage {
				if name == tc.errStage {
					return &decisionStage{name: name, err: stageErr}
				}
				return newNoopStage(name)
			}

			cfg := BeadLoopConfig{
				Gate:     makeStage("gate"),
				Build:    makeStage("build"),
				Validate: makeStage("validate"),
				Review:   makeStage("review"),
				Epilogue: makeStage("epilogue"),
			}

			loop, err := NewBeadLoop(cfg)
			if err != nil {
				t.Fatalf("NewBeadLoop: %v", err)
			}

			_, err = loop.Run(context.Background(), []*bead.Bead{{ID: "err-bead"}}, nil)
			if err == nil {
				t.Fatal("expected error from Run, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("error %q should mention %q", err.Error(), tc.wantSubstr)
			}
			if !errors.Is(err, stageErr) && !strings.Contains(err.Error(), stageErr.Error()) {
				t.Fatalf("error %q should contain original error %q", err.Error(), stageErr.Error())
			}
		})
	}

	// Recoverable errors: build/validate/review failures skip the bead
	// and the loop completes without error.
	recoverableCases := []struct {
		name     string
		errStage string
	}{
		{name: "build error", errStage: "build"},
		{name: "validate error", errStage: "validate"},
		{name: "review error", errStage: "review"},
	}

	for _, tc := range recoverableCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			makeStage := func(name string) stage.Stage {
				if name == tc.errStage {
					return &decisionStage{name: name, err: stageErr}
				}
				return newNoopStage(name)
			}

			cfg := BeadLoopConfig{
				Gate:     makeStage("gate"),
				Build:    makeStage("build"),
				Validate: makeStage("validate"),
				Review:   makeStage("review"),
				Epilogue: makeStage("epilogue"),
			}

			loop, err := NewBeadLoop(cfg)
			if err != nil {
				t.Fatalf("NewBeadLoop: %v", err)
			}

			_, err = loop.Run(context.Background(), []*bead.Bead{{ID: "err-bead"}}, nil)
			if err != nil {
				t.Fatalf("expected loop to continue past %s failure, got error: %v", tc.errStage, err)
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
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
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

	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
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

	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
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

	// Beads at generation 3 (start 0 + cap 3), which triggers the cap immediately
	beads := []*bead.Bead{
		{ID: "bead-at-gen-3", Labels: []string{generation.Format(3)}},
	}

	_, err = loop.Run(context.Background(), beads, nil)
	if !errors.Is(err, ErrGenerationCapReached) {
		t.Fatalf("Run error = %v, want ErrGenerationCapReached", err)
	}

	// Check that GenerationCapReached event was emitted
	waitForGenerationCapEvent(t, ch)
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
	_, err = loop.Run(context.Background(), beads, nil)
	if !errors.Is(err, ErrGenerationCapReached) {
		t.Fatalf("Run error = %v, want ErrGenerationCapReached", err)
	}

	waitForGenerationCapEvent(t, ch)
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

	if _, err := loop.Run(context.Background(), beads, stopCh); err != nil {
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

func TestBeadLoopStopsWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gate := &cancelingStage{name: "gate", cancelAfter: 1, cancelFn: cancel}
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

	_, err = loop.Run(ctx, beads, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context canceled", err)
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

type retryStage struct {
	name        string
	failTimes   int
	runCount    int
	hook        func(string, int)
	retryConfig stage.RetryConfig
	requests    []stage.Request
}

func newRetryStage(name string, failTimes int, hook func(string, int)) *retryStage {
	return &retryStage{name: name, failTimes: failTimes, hook: hook}
}

func (s *retryStage) Name() string { return s.name }

func (s *retryStage) RetryConfig() stage.RetryConfig { return s.retryConfig }

func (s *retryStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	if req != nil {
		if s.hook != nil {
			s.hook(req.Bead.ID, s.runCount)
		}
		s.requests = append(s.requests, *req)
	}
	if s.runCount <= s.failTimes {
		return &stage.Result{Decision: stage.DecisionFail}, nil
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type failingValidationRunner struct {
	failures int
	runCount int
}

func (r *failingValidationRunner) Run(ctx context.Context, command, worktree string) error {
	r.runCount++
	if r.runCount <= r.failures {
		return errors.New("validation failure")
	}
	return nil
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

type cancelingStage struct {
	name        string
	cancelAfter int
	cancelFn    func()
	runCount    int
}

func (s *cancelingStage) Name() string { return s.name }

func (s *cancelingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	if s.cancelFn != nil && s.cancelAfter > 0 && s.runCount == s.cancelAfter {
		s.cancelFn()
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type scriptedReviewStage struct {
	name     string
	result   *stage.Result
	runCount int
}

func (s *scriptedReviewStage) Name() string { return s.name }

func (s *scriptedReviewStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	return s.result, nil
}

// mockGitCommitter records Status and Commit calls for testing.
type mockGitCommitter struct {
	statusOutput string
	statusErr    error
	commitHash   string
	commitErr    error
	statusCalls  int
	commitCalls  []string // commit messages
}

func (m *mockGitCommitter) Status(ctx context.Context, worktree string) (string, error) {
	m.statusCalls++
	return m.statusOutput, m.statusErr
}

func (m *mockGitCommitter) Commit(ctx context.Context, worktree, message string) (string, error) {
	m.commitCalls = append(m.commitCalls, message)
	return m.commitHash, m.commitErr
}

func TestBeadLoopNoGitAdapterSkipsCommit(t *testing.T) {
	t.Parallel()

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    newNoopStage("build"),
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
		// Git is nil — no GitCommitter provided
	}
	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "no-git"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run should succeed without git adapter, got: %v", err)
	}
}

func waitForGenerationCapEvent(t *testing.T, ch <-chan event.TypedEvent) {
	t.Helper()
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case evt := <-ch:
			if _, ok := evt.(event.GenerationCapReachedEvent); ok {
				return
			}
		case <-timer.C:
			t.Fatal("expected GenerationCapReached event to be emitted")
		}
	}
}

// fakeTriageStage implements stage.Stage and returns a configured triage result.
type fakeTriageStage struct {
	name      string
	category  triage.Category
	reasoning string
	runCount  int
	requests  []stage.Request
}

func (s *fakeTriageStage) Name() string { return s.name }

func (s *fakeTriageStage) Run(_ context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	if req != nil {
		s.requests = append(s.requests, *req)
	}
	return &stage.Result{
		Decision: stage.DecisionProceed,
		Artifacts: &triage.TriageArtifacts{
			Category:  s.category,
			Reasoning: s.reasoning,
		},
	}, nil
}

// fakeBeadLoopDecomposeStage implements stage.Stage and returns configured sub-beads.
type fakeBeadLoopDecomposeStage struct {
	name     string
	beads    []*bead.Bead
	runCount int
	requests []stage.Request
}

func (s *fakeBeadLoopDecomposeStage) Name() string { return s.name }

func (s *fakeBeadLoopDecomposeStage) Run(_ context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	if req != nil {
		s.requests = append(s.requests, *req)
	}
	return &stage.Result{
		Decision: stage.DecisionProceed,
		Artifacts: &stage.DecomposeArtifacts{
			Beads: s.beads,
		},
	}, nil
}

func TestBeadLoopTriageDecomposeSplitsAndRunsSubBeads(t *testing.T) {
	t.Parallel()

	order := []string{}
	buildAttempt := 0
	build := &recordingStage{
		name: "build",
		hook: func(beadID string) {
			buildAttempt++
			order = append(order, fmt.Sprintf("build:%s:%d", beadID, buildAttempt))
		},
	}
	// Make build fail for the parent bead only
	parentBuild := &retryStage{
		name:      "build",
		failTimes: 1, // fails first time
		hook: func(beadID string, attempt int) {
			order = append(order, fmt.Sprintf("build:%s:%d", beadID, attempt))
		},
	}
	// Use a scripted build that fails for parent, succeeds for children
	scriptedBuild := &scriptedBuildStage{
		name:     "build",
		failIDs:  map[string]bool{"parent": true},
		order:    &order,
		attempts: map[string]int{},
	}
	_ = build // unused, use scriptedBuild instead
	_ = parentBuild

	subBeads := []*bead.Bead{
		{ID: "sub-1", Labels: []string{generation.Format(1)}},
		{ID: "sub-2", Labels: []string{generation.Format(1)}, DependsOn: []bead.Dependency{{ID: "sub-1"}}},
	}

	triageStage := &fakeTriageStage{
		name:      "triage",
		category:  triage.CategoryDecompose,
		reasoning: "too complex, split it",
	}
	decomposeStage := &fakeBeadLoopDecomposeStage{
		name:  "decompose",
		beads: subBeads,
	}

	cfg := BeadLoopConfig{
		Gate:      newNoopStage("gate"),
		Build:     scriptedBuild,
		Validate:  newNoopStage("validate"),
		Review:    newNoopStage("review"),
		Epilogue:  newNoopStage("epilogue"),
		Triage:    triageStage,
		Decompose: decomposeStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "parent", Labels: []string{generation.Format(0)}}}
	_, err = loop.Run(context.Background(), beads, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if triageStage.runCount != 1 {
		t.Fatalf("triage run count = %d, want 1", triageStage.runCount)
	}
	if decomposeStage.runCount != 1 {
		t.Fatalf("decompose run count = %d, want 1", decomposeStage.runCount)
	}
	// Sub-beads should have been processed
	if scriptedBuild.attempts["sub-1"] == 0 || scriptedBuild.attempts["sub-2"] == 0 {
		t.Fatalf("sub-beads not processed: attempts = %v", scriptedBuild.attempts)
	}
}

func TestBeadLoopTriageRetryDoesNotCountAgainstRetryLimit(t *testing.T) {
	t.Parallel()

	triageCallCount := 0
	// Build that fails twice then succeeds
	buildStage := &retryStage{
		name:      "build",
		failTimes: 2,
	}
	buildStage.retryConfig = stage.RetryConfig{MaxRetries: 0} // no normal retries

	// Triage returns retry on first call, then we won't need it
	triageStage := &fakeTriageStage{
		name:      "triage",
		category:  triage.CategoryRetry,
		reasoning: "transient error",
	}

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    buildStage,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
		Triage:   triageStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "retry-bead"}}
	_, err = loop.Run(context.Background(), beads, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	_ = triageCallCount
	// Build was configured with MaxRetries=0, but triage said retry,
	// so it should have run 3 times total (2 failures + 1 success)
	if buildStage.runCount != 3 {
		t.Fatalf("build run count = %d, want 3", buildStage.runCount)
	}
	// Triage should have been called twice (once per failure)
	if triageStage.runCount != 2 {
		t.Fatalf("triage run count = %d, want 2", triageStage.runCount)
	}
}

func TestBeadLoopTriageUnclearSpecReturnsError(t *testing.T) {
	t.Parallel()

	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &fakeTriageStage{
		name:      "triage",
		category:  triage.CategoryUnclearSpec,
		reasoning: "acceptance criteria are ambiguous",
	}

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    buildStage,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
		Triage:   triageStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	_, err = loop.Run(context.Background(), []*bead.Bead{{ID: "unclear"}}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "spec unclear") {
		t.Fatalf("error should mention spec unclear, got: %v", err)
	}
	if !strings.Contains(err.Error(), "acceptance criteria are ambiguous") {
		t.Fatalf("error should contain reasoning, got: %v", err)
	}
}

func TestBeadLoopTriageUnsafeReturnsError(t *testing.T) {
	t.Parallel()

	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &fakeTriageStage{
		name:      "triage",
		category:  triage.CategoryUnsafe,
		reasoning: "modifies production database",
	}

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    buildStage,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
		Triage:   triageStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	_, err = loop.Run(context.Background(), []*bead.Bead{{ID: "dangerous"}}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsafe operation") {
		t.Fatalf("error should mention unsafe operation, got: %v", err)
	}
	if !strings.Contains(err.Error(), "modifies production database") {
		t.Fatalf("error should contain reasoning, got: %v", err)
	}
}

func TestBeadLoopTriageNotConfiguredFallsBackToExistingBehavior(t *testing.T) {
	t.Parallel()

	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	epilogueStage := &decisionStage{name: "epilogue", decision: stage.DecisionProceed}

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    buildStage,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: epilogueStage,
		// Triage is nil — not configured
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	// Build failure without triage is recoverable — loop completes.
	_, err = loop.Run(context.Background(), []*bead.Bead{{ID: "no-triage"}}, nil)
	if err != nil {
		t.Fatalf("expected loop to continue past build failure, got: %v", err)
	}
	// Epilogue should have been called with retry context (existing behavior)
	if len(epilogueStage.requests) != 1 {
		t.Fatalf("epilogue run count = %d, want 1", len(epilogueStage.requests))
	}
}

func TestBeadLoopTriageDecomposeRespectsGenerationCap(t *testing.T) {
	t.Parallel()

	emitter := event.NewEmitter()
	ch := make(chan event.TypedEvent, 32)
	emitter.Subscribe(func(evt event.TypedEvent) {
		select {
		case ch <- evt:
		default:
		}
	})

	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &fakeTriageStage{
		name:      "triage",
		category:  triage.CategoryDecompose,
		reasoning: "needs splitting",
	}
	// Sub-beads at generation 3, which is at the cap
	decomposeStage := &fakeBeadLoopDecomposeStage{
		name: "decompose",
		beads: []*bead.Bead{
			{ID: "sub-at-cap", Labels: []string{generation.Format(3)}},
		},
	}

	cfg := BeadLoopConfig{
		Gate:                       newNoopStage("gate"),
		Build:                      buildStage,
		Validate:                   newNoopStage("validate"),
		Review:                     newNoopStage("review"),
		Epilogue:                   newNoopStage("epilogue"),
		Triage:                     triageStage,
		Decompose:                  decomposeStage,
		Emitter:                    emitter,
		DecompositionGenerationCap: 3,
		StartGeneration:            0,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "parent", Labels: []string{generation.Format(2)}}}
	_, err = loop.Run(context.Background(), beads, nil)
	if !errors.Is(err, ErrGenerationCapReached) {
		t.Fatalf("Run error = %v, want ErrGenerationCapReached", err)
	}

	waitForGenerationCapEvent(t, ch)
}

// scriptedBuildStage fails for specific bead IDs, succeeds for others.
type scriptedBuildStage struct {
	name     string
	failIDs  map[string]bool
	order    *[]string
	attempts map[string]int
}

func (s *scriptedBuildStage) Name() string { return s.name }

func (s *scriptedBuildStage) Run(_ context.Context, req *stage.Request) (*stage.Result, error) {
	id := req.Bead.ID
	s.attempts[id]++
	if s.order != nil {
		*s.order = append(*s.order, fmt.Sprintf("build:%s:%d", id, s.attempts[id]))
	}
	if s.failIDs[id] {
		return &stage.Result{Decision: stage.DecisionFail}, nil
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

// nilResultTriageStage returns (nil, nil) from runStage to exercise nil-result guard.
type nilResultTriageStage struct {
	name     string
	runCount int
}

func (s *nilResultTriageStage) Name() string { return s.name }

func (s *nilResultTriageStage) Run(_ context.Context, _ *stage.Request) (*stage.Result, error) {
	s.runCount++
	return nil, nil
}

// --- Bug C1: Triage returns nil result ---

func TestBeadLoopTriageNilResultReturnsError(t *testing.T) {
	t.Parallel()

	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &nilResultTriageStage{name: "triage"}

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    buildStage,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
		Triage:   triageStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	_, err = loop.Run(context.Background(), []*bead.Bead{{ID: "nil-triage"}}, nil)
	if err == nil {
		t.Fatal("expected error when triage returns nil result, got nil")
	}
	if !strings.Contains(err.Error(), "triage stage returned no result") {
		t.Fatalf("error should mention nil result, got: %v", err)
	}
	if triageStage.runCount != 1 {
		t.Fatalf("triage run count = %d, want 1", triageStage.runCount)
	}
}

// --- Bug I1: Gate skip/block should not halt the loop ---

func TestBeadLoopGateSkipContinuesLoop(t *testing.T) {
	t.Parallel()

	// Gate that skips the first bead, proceeds for the second
	gateStage := &scriptedGateStage{
		name:      "gate",
		decisions: map[string]stage.Decision{"skip-me": stage.DecisionSkip},
	}
	build := &decisionStage{name: "build", decision: stage.DecisionProceed}
	epilogue := &decisionStage{name: "epilogue", decision: stage.DecisionProceed}

	cfg := BeadLoopConfig{
		Gate:     gateStage,
		Build:    build,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: epilogue,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{
		{ID: "skip-me"},
		{ID: "run-me"},
	}
	_, err = loop.Run(context.Background(), beads, nil)
	if err != nil {
		t.Fatalf("Run should succeed, got: %v", err)
	}

	// Build should only run for "run-me", not "skip-me"
	if build.runCount != 1 {
		t.Fatalf("build run count = %d, want 1 (only run-me)", build.runCount)
	}
	// Gate should have been called twice
	if gateStage.runCount != 2 {
		t.Fatalf("gate run count = %d, want 2", gateStage.runCount)
	}
}

func TestBeadLoopGateBlockDefersBeadAndContinues(t *testing.T) {
	t.Parallel()

	// Gate that blocks "block-me" on every call, proceeds for everything else.
	// With re-queue logic: "run-me" makes progress in pass 1, "block-me" is
	// re-queued, then retried in pass 2 where it still blocks with no
	// progress → loop returns a descriptive error.
	gateStage := &scriptedGateStage{
		name:      "gate",
		decisions: map[string]stage.Decision{"block-me": stage.DecisionBlock},
	}
	build := &decisionStage{name: "build", decision: stage.DecisionProceed}

	cfg := BeadLoopConfig{
		Gate:     gateStage,
		Build:    build,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	// block-me has no deps, run-me depends on nothing
	beads := []*bead.Bead{
		{ID: "run-me"},
		{ID: "block-me"},
	}
	_, err = loop.Run(context.Background(), beads, nil)
	// "run-me" makes progress in pass 1; "block-me" is retried in pass 2
	// and still blocks → loop returns error for permanently blocked bead.
	if err == nil {
		t.Fatal("expected error when blocked bead cannot proceed after retry, got nil")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected error to mention 'blocked', got: %v", err)
	}

	// Build should only run for "run-me" — never for the blocked bead
	if build.runCount != 1 {
		t.Fatalf("build run count = %d, want 1 (only run-me)", build.runCount)
	}
}

func TestBeadLoopBlockedBeadDeferredToEndOfPass(t *testing.T) {
	t.Parallel()

	order := []string{}
	gate := newRecordingGateStage("gate", &order, map[string]stage.Decision{"block-me": stage.DecisionBlock})

	cfg := BeadLoopConfig{
		Gate:     gate,
		Build:    newNoopStage("build"),
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{
		{ID: "block-me"},
		{ID: "run-me"},
	}

	_, err = loop.Run(context.Background(), beads, nil)
	if err == nil {
		t.Fatal("expected blocked bead to eventually fail after requeueing, got nil")
	}

	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("error should mention blocked status, got: %v", err)
	}

	wantOrder := []string{"block-me:1", "run-me:1", "block-me:2"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("gate run order = %v, want %v", order, wantOrder)
	}
}

// TestBeadLoopAllBeadsBlockedReturnsError verifies that when all beads are
// blocked by the gate in a full pass with no progress, the loop stops with
// a descriptive error instead of completing silently.
func TestBeadLoopAllBeadsBlockedReturnsError(t *testing.T) {
	t.Parallel()

	gateStage := &scriptedGateStage{
		name: "gate",
		decisions: map[string]stage.Decision{
			"bead-1": stage.DecisionBlock,
			"bead-2": stage.DecisionBlock,
		},
	}

	cfg := BeadLoopConfig{
		Gate:     gateStage,
		Build:    newNoopStage("build"),
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{
		{ID: "bead-1"},
		{ID: "bead-2"},
	}

	_, err = loop.Run(context.Background(), beads, nil)
	if err == nil {
		t.Fatal("expected error when all beads blocked, got nil")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected error to mention 'blocked', got: %v", err)
	}
}

// TestBeadLoopBlockedBeadRetriedAfterProgress verifies that a bead blocked by
// the gate is retried in the next pass after another bead makes progress.
// If the previously-blocked bead proceeds on retry, the loop completes without error.
func TestBeadLoopBlockedBeadRetriedAfterProgress(t *testing.T) {
	t.Parallel()

	// Gate that blocks "retry-me" on the first call, proceeds on subsequent calls.
	callCounts := map[string]int{}
	gateStage := &callCountingGateStage{
		name:       "gate",
		callCounts: callCounts,
		// Block "retry-me" only on its first gate invocation
		firstCallBlock: map[string]bool{"retry-me": true},
	}

	cfg := BeadLoopConfig{
		Gate:     gateStage,
		Build:    newNoopStage("build"),
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{
		{ID: "proceed-me"},
		{ID: "retry-me"},
	}

	_, err = loop.Run(context.Background(), beads, nil)
	if err != nil {
		t.Fatalf("expected loop to succeed after blocked bead unblocks on retry, got: %v", err)
	}

	// Gate should have been called twice for "retry-me": once blocked, once proceeds
	if callCounts["retry-me"] != 2 {
		t.Fatalf("gate call count for retry-me = %d, want 2 (blocked then proceed)", callCounts["retry-me"])
	}
}

func TestBeadLoopBlockedBeadProceedsAfterDependencyCompletes(t *testing.T) {
	t.Parallel()

	order := []string{}
	gate := newRecordingGateStage("gate", &order, nil)
	gate.decisionFn = func(id string, attempt int) stage.Decision {
		if id == "blocked" && attempt == 1 {
			return stage.DecisionBlock
		}
		return stage.DecisionProceed
	}

	cfg := BeadLoopConfig{
		Gate:     gate,
		Build:    newNoopStage("build"),
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{
		{ID: "depends"},
		{ID: "blocked", DependsOn: []bead.Dependency{{ID: "depends"}}},
	}

	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("expected loop to succeed after dependency unblocks gate, got: %v", err)
	}

	wantOrder := []string{"depends:1", "blocked:1", "blocked:2"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("gate run order = %v, want %v", order, wantOrder)
	}
}

// callCountingGateStage is a gate that blocks a bead on its first invocation
// and proceeds on all subsequent invocations.
type callCountingGateStage struct {
	name           string
	callCounts     map[string]int
	firstCallBlock map[string]bool
}

func (s *callCountingGateStage) Name() string { return s.name }

func (s *callCountingGateStage) Run(_ context.Context, req *stage.Request) (*stage.Result, error) {
	s.callCounts[req.Bead.ID]++
	if s.firstCallBlock[req.Bead.ID] && s.callCounts[req.Bead.ID] == 1 {
		return &stage.Result{Decision: stage.DecisionBlock}, nil
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

// scriptedGateStage returns configured decisions per bead ID; defaults to Proceed.
type scriptedGateStage struct {
	name      string
	decisions map[string]stage.Decision
	runCount  int
}

func (s *scriptedGateStage) Name() string { return s.name }

func (s *scriptedGateStage) Run(_ context.Context, req *stage.Request) (*stage.Result, error) {
	s.runCount++
	if d, ok := s.decisions[req.Bead.ID]; ok {
		return &stage.Result{Decision: d}, nil
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type recordingGateStage struct {
	name       string
	order      *[]string
	decisions  map[string]stage.Decision
	callCounts map[string]int
}

func newRecordingGateStage(name string, order *[]string, decisions map[string]stage.Decision) *recordingGateStage {
	return &recordingGateStage{
		name:       name,
		order:      order,
		decisions:  decisions,
		callCounts: make(map[string]int),
	}
}

func (s *recordingGateStage) Name() string { return s.name }

func (s *recordingGateStage) Run(_ context.Context, req *stage.Request) (*stage.Result, error) {
	id := req.Bead.ID
	s.callCounts[id]++
	if s.order != nil {
		*s.order = append(*s.order, fmt.Sprintf("%s:%d", id, s.callCounts[id]))
	}
	if d, ok := s.decisions[id]; ok {
		return &stage.Result{Decision: d}, nil
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

// --- Bug M4: Triage retry capped ---

func TestBeadLoopTriageRetryCapped(t *testing.T) {
	t.Parallel()

	// Build always fails
	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}

	// Triage always returns retry
	triageStage := &fakeTriageStage{
		name:      "triage",
		category:  triage.CategoryRetry,
		reasoning: "transient error",
	}

	epilogue := &decisionStage{name: "epilogue", decision: stage.DecisionProceed}

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    buildStage,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: epilogue,
		Triage:   triageStage,
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	// Triage retry exhaustion goes through failWithReason, which is
	// recoverable — the loop completes after skipping the failed bead.
	_, err = loop.Run(context.Background(), []*bead.Bead{{ID: "stuck"}}, nil)
	if err != nil {
		t.Fatalf("expected loop to continue past triage retry exhaustion, got: %v", err)
	}

	// maxTriageRetries is 3, so triage should be called at most maxTriageRetries+1 times
	// (the +1 is the call that exceeds the cap).
	// Build attempts: 1 initial + 3 retries (from triage) + 0 from cap exceeded = 4
	// then the 4th failure triggers triage again (4th triage call) which exceeds cap,
	// falls through to failWithReason.
	if triageStage.runCount > maxTriageRetries+1 {
		t.Fatalf("triage run count = %d, want at most %d (capped)", triageStage.runCount, maxTriageRetries+1)
	}
	// Build should not run indefinitely
	if buildStage.runCount > maxTriageRetries+2 {
		t.Fatalf("build run count = %d, should be capped", buildStage.runCount)
	}
}

// closeTrackingTracker records CloseBead calls and implements trackertypes.TaskTracker.
type closeTrackingTracker struct {
	closeCalls []string
	closeErr   error
}

func (t *closeTrackingTracker) NextBead(_ context.Context, _ trackertypes.TaskTrackerNextBeadRequest) (*trackertypes.TaskTrackerNextBeadResponse, error) {
	return nil, nil
}

func (t *closeTrackingTracker) ShowBead(_ context.Context, _ string) (*trackertypes.Bead, error) {
	return nil, nil
}

func (t *closeTrackingTracker) CreateBead(_ context.Context, _ trackertypes.TaskTrackerCreateBeadRequest) (*trackertypes.TaskTrackerCreateBeadResponse, error) {
	return &trackertypes.TaskTrackerCreateBeadResponse{}, nil
}

func (t *closeTrackingTracker) CloseBead(_ context.Context, req trackertypes.TaskTrackerCloseBeadRequest) (*trackertypes.TaskTrackerCloseBeadResponse, error) {
	t.closeCalls = append(t.closeCalls, req.BeadID)
	if t.closeErr != nil {
		return nil, t.closeErr
	}
	return &trackertypes.TaskTrackerCloseBeadResponse{Closed: true}, nil
}

func (t *closeTrackingTracker) QueryBeads(_ context.Context, _ trackertypes.TaskTrackerQueryBeadsRequest) (*trackertypes.TaskTrackerQueryBeadsResponse, error) {
	return &trackertypes.TaskTrackerQueryBeadsResponse{}, nil
}

func TestBeadLoopClosesParentAfterReviewCreatesChildren(t *testing.T) {
	t.Parallel()

	parentID := "parent-1"

	// Child beads created by review use review-source label, NOT dependencies.
	childBeads := []*tasktracker.Bead{
		{
			ID:     "child-1",
			Title:  "Fix naming",
			Labels: []string{"review-source:" + parentID},
		},
		{
			ID:     "child-2",
			Title:  "Add tests",
			Labels: []string{"review-source:" + parentID},
		},
	}

	reviewStage := &scriptedReviewStage{
		name: "review",
		result: &stage.Result{
			Decision: stage.DecisionProceed,
			Artifacts: &reviewstage.ReviewArtifacts{
				CreatedBeads: childBeads,
			},
		},
	}

	tracker := &closeTrackingTracker{}
	epilogue, err := epiloguestage.New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("epilogue.New: %v", err)
	}

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    newNoopStage("build"),
		Validate: newNoopStage("validate"),
		Review:   reviewStage,
		Epilogue: epilogue,
	}
	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: parentID}}
	_, err = loop.Run(context.Background(), beads, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Assert CloseBead was called for the parent bead.
	if len(tracker.closeCalls) != 1 {
		t.Fatalf("CloseBead call count = %d, want 1", len(tracker.closeCalls))
	}
	if tracker.closeCalls[0] != parentID {
		t.Fatalf("CloseBead called with %q, want %q", tracker.closeCalls[0], parentID)
	}

	// These assertions document the expected contract for review-created child beads:
	// labels carry provenance (review-source:<parentID>), and DependsOn is empty
	// to avoid blocking parent closure. They verify the fixture, not runtime behavior —
	// the scriptedReviewStage injects these artifacts directly.
	for i, child := range childBeads {
		if len(child.DependsOn) != 0 {
			t.Fatalf("child[%d] %q has dependencies %v, want none", i, child.ID, child.DependsOn)
		}
		hasLabel := false
		for _, label := range child.Labels {
			if label == "review-source:"+parentID {
				hasLabel = true
			}
		}
		if !hasLabel {
			t.Fatalf("child[%d] %q missing review-source:%s label", i, child.ID, parentID)
		}
	}
}

// mockStageCommitter records CommitStage calls for testing.
type mockStageCommitter struct {
	calls []stageCommitCall
	err   error
}

type stageCommitCall struct {
	beadID    string
	stageName string
	iteration int
	decision  string
}

func (m *mockStageCommitter) CommitStage(ctx context.Context, worktree, beadID, stageName string, iteration int, decision string) error {
	m.calls = append(m.calls, stageCommitCall{
		beadID:    beadID,
		stageName: stageName,
		iteration: iteration,
		decision:  decision,
	})
	return m.err
}

func TestBeadLoopSkipsLegacyCommitBeadWorkWhenStageCommitterConfigured(t *testing.T) {
	t.Parallel()

	git := &mockGitCommitter{statusOutput: " M file.go\n", commitHash: "abc123"}
	sc := &mockStageCommitter{}
	cfg := BeadLoopConfig{
		Gate:           newNoopStage("gate"),
		Build:          newNoopStage("build"),
		Validate:       newNoopStage("validate"),
		Review:         newNoopStage("review"),
		Epilogue:       newNoopStage("epilogue"),
		Git:            git,
		StageCommitter: sc,
	}
	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "bead-sc"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Legacy commitBeadWork should not be called when StageCommitter is set
	if git.statusCalls != 0 {
		t.Fatalf("git.Status called %d times, want 0 (legacy commit skipped)", git.statusCalls)
	}
	if len(git.commitCalls) != 0 {
		t.Fatalf("git.Commit called %d times, want 0 (legacy commit skipped)", len(git.commitCalls))
	}
}

func TestBeadLoopCallsStageCommitterAfterSuccessfulStage(t *testing.T) {
	t.Parallel()

	sc := &mockStageCommitter{}
	cfg := BeadLoopConfig{
		Gate:           newNoopStage("gate"),
		Build:          newNoopStage("build"),
		Validate:       newNoopStage("validate"),
		Review:         newNoopStage("review"),
		Epilogue:       newNoopStage("epilogue"),
		StageCommitter: sc,
	}
	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "bead-1"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have been called after gate, build, validate, review (4 pipeline stages)
	if len(sc.calls) != 4 {
		t.Fatalf("CommitStage called %d times, want 4", len(sc.calls))
	}
	if sc.calls[0].stageName != "gate" {
		t.Fatalf("first call stage = %q, want %q", sc.calls[0].stageName, "gate")
	}
	if sc.calls[1].stageName != "build" {
		t.Fatalf("second call stage = %q, want %q", sc.calls[1].stageName, "build")
	}
	if sc.calls[0].beadID != "bead-1" {
		t.Fatalf("first call bead ID = %q, want %q", sc.calls[0].beadID, "bead-1")
	}
	if sc.calls[0].iteration != 1 {
		t.Fatalf("first call iteration = %d, want 1", sc.calls[0].iteration)
	}
	if sc.calls[2].stageName != "validate" {
		t.Fatalf("third call stage = %q, want %q", sc.calls[2].stageName, "validate")
	}
	if sc.calls[3].stageName != "review" {
		t.Fatalf("fourth call stage = %q, want %q", sc.calls[3].stageName, "review")
	}
}

func TestBeadLoopLegacyGitCommitRemovedWithoutStageCommitter(t *testing.T) {
	t.Parallel()

	git := &mockGitCommitter{statusOutput: " M file.go\n", commitHash: "abc123"}
	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    newNoopStage("build"),
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
		Git:      git,
		// No StageCommitter — legacy commitBeadWork should no longer exist
	}
	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "bead-2"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if git.statusCalls != 0 {
		t.Fatalf("git.Status called %d times, want 0 (legacy commitBeadWork removed)", git.statusCalls)
	}
}

// TestCheckBudgetIsNoOpStub documents that checkBudget is a no-op stub
// returning nil. It will be replaced with real budget policy checks when
// the Andon spec is implemented. This test ensures the stub contract is
// explicit: checkBudget must not block the pipeline or return an error.
func TestCheckBudgetIsNoOpStub(t *testing.T) {
	t.Parallel()

	loop, err := NewBeadLoop(BeadLoopConfig{
		Gate:     newRecordingStage("gate", nil),
		Build:    newRecordingStage("build", nil),
		Validate: newRecordingStage("validate", nil),
		Review:   newRecordingStage("review", nil),
		Epilogue: newRecordingStage("epilogue", nil),
	})
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	// checkBudget is a no-op stub for future Andon budget policy.
	// It must return nil so it never blocks the bead pipeline.
	if err := loop.checkBudget(context.Background()); err != nil {
		t.Fatalf("checkBudget returned error: %v (expected nil — no-op stub)", err)
	}

	// Also verify it handles a nil context without panicking,
	// matching the guard pattern used elsewhere in the loop.
	if err := loop.checkBudget(nil); err != nil {
		t.Fatalf("checkBudget(nil ctx) returned error: %v (expected nil — no-op stub)", err)
	}
}
