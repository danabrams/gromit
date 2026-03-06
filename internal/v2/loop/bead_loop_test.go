package loop

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
	reviewstage "github.com/danabrams/gromit/internal/v2/stage/review"
	stagevalidate "github.com/danabrams/gromit/internal/v2/stage/validate"
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
	if err := loop.Run(context.Background(), beads, nil); err != nil {
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
	if err := loop.Run(context.Background(), beads, nil); err != nil {
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

	// Beads at generation 3 (start 0 + cap 3), which triggers the cap immediately
	beads := []*bead.Bead{
		{ID: "bead-at-gen-3", Labels: []string{generation.Format(3)}},
	}

	err = loop.Run(context.Background(), beads, nil)
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
	err = loop.Run(context.Background(), beads, nil)
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

	err = loop.Run(ctx, beads, nil)
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

func (r *failingValidationRunner) Run(ctx context.Context, command string) error {
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
