package loop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/presentation"
	v2review "github.com/danabrams/gromit/internal/v2/review"
	"github.com/danabrams/gromit/internal/v2/routing"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	planstage "github.com/danabrams/gromit/internal/v2/stage/plan"
	present "github.com/danabrams/gromit/internal/v2/stage/present"
)

func TestNewSpecLoopValidation(t *testing.T) {
	t.Parallel()

	validAdapters := func(tb testing.TB) adapter.AdapterSet {
		return adapter.AdapterSet{
			Git:         newFakeGitAdapter(tb.(*testing.T)),
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(tb.(*testing.T)),
		}
	}

	cases := []struct {
		name      string
		cfg       *config.Config
		adapters  func(testing.TB) adapter.AdapterSet
		gate      DependencyGate
		wantField string
	}{
		{
			name:      "nil config",
			cfg:       nil,
			adapters:  func(tb testing.TB) adapter.AdapterSet { return validAdapters(tb) },
			gate:      noopDependencyGate{},
			wantField: "config",
		},
		{
			name: "nil git adapter",
			cfg:  &config.Config{},
			adapters: func(tb testing.TB) adapter.AdapterSet {
				a := validAdapters(tb)
				a.Git = nil
				return a
			},
			gate:      noopDependencyGate{},
			wantField: "git",
		},
		{
			name: "nil llm adapter",
			cfg:  &config.Config{},
			adapters: func(tb testing.TB) adapter.AdapterSet {
				a := validAdapters(tb)
				a.LLM = nil
				return a
			},
			gate:      noopDependencyGate{},
			wantField: "llm",
		},
		{
			name: "nil task tracker adapter",
			cfg:  &config.Config{},
			adapters: func(tb testing.TB) adapter.AdapterSet {
				a := validAdapters(tb)
				a.TaskTracker = nil
				return a
			},
			gate:      noopDependencyGate{},
			wantField: "task tracker",
		},
		{
			name: "nil presenter adapter",
			cfg:  &config.Config{},
			adapters: func(tb testing.TB) adapter.AdapterSet {
				a := validAdapters(tb)
				a.Presenter = nil
				return a
			},
			gate:      noopDependencyGate{},
			wantField: "presenter",
		},
		{
			name:      "nil dependency gate",
			cfg:       &config.Config{},
			adapters:  func(tb testing.TB) adapter.AdapterSet { return validAdapters(tb) },
			gate:      nil,
			wantField: "dependency gate",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			adapters := tc.adapters(t)
			_, err := NewSpecLoop(adapters, tc.cfg, tc.gate)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Fatalf("error %q should mention %q", err.Error(), tc.wantField)
			}
		})
	}

	t.Run("valid config succeeds", func(t *testing.T) {
		t.Parallel()
		adapters := validAdapters(t)
		loop, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if loop == nil {
			t.Fatal("expected non-nil loop")
		}
	})
}

func TestSpecLoopHappyPathExecutesPipeline(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-happy"
	cfg := &config.Config{}

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	recorder := newRecordingStageRecorder()

	git := newFakeGitAdapter(t)
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	planStage := newFakePlanStage(specID)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithStageRecorder(recorder),
		WithEmitter(emitter),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if !beadRunner.ran {
		t.Fatalf("expected bead loop to run")
	}
	if !reflect.DeepEqual(beadRunner.lastBeads, decompose.producedBeads) {
		t.Fatalf("beads = %v, want %v", beadRunner.lastBeads, decompose.producedBeads)
	}

	if got := recorder.stageNames(); !reflect.DeepEqual(got, StageSequence) {
		t.Fatalf("stage order = %v, want %v", got, StageSequence)
	}

	received := collectEvents(t, ch, 2)
	started, ok := received[0].(*events.SpecStartedEvent)
	if !ok || started.SpecID != specID {
		t.Fatalf("spec started event = %v", received[0])
	}
	completed, ok := received[1].(*events.SpecCompletedEvent)
	if !ok || completed.SpecID != specID || !completed.Success {
		t.Fatalf("spec completed = %v", received[1])
	}

	summary := presenter.lastSummary
	if !summary.Success {
		t.Fatalf("summary success = %v", summary.Success)
	}
	if summary.Plan != llm.planFor(specID) {
		t.Fatalf("summary plan = %q", summary.Plan)
	}
	if summary.SpecName != specID {
		t.Fatalf("summary spec = %q", summary.SpecName)
	}
	if len(summary.AcceptanceResults) != len(accept.results) {
		t.Fatalf("acceptance results count = %d", len(summary.AcceptanceResults))
	}
}

func TestBuildSuccessSummaryIncludesOutOfScopeFindings(t *testing.T) {
	t.Parallel()

	loopInstance := &SpecLoop{cfg: &config.Config{}}
	outOfScope := []v2review.Finding{
		{
			Title:         "Audit drift",
			Description:   "Audit guidance drift is outside current acceptance criteria",
			AffectedFiles: []string{"README.md"},
		},
	}

	summary := loopInstance.buildSuccessSummary("spec-id", "worktree", "plan", nil, nil, outOfScope)

	if len(summary.OutOfScopeFindings) != 1 {
		t.Fatalf("expected 1 out-of-scope finding, got %d", len(summary.OutOfScopeFindings))
	}
	if summary.OutOfScopeFindings[0].Title != outOfScope[0].Title {
		t.Fatalf("title = %q, want %q", summary.OutOfScopeFindings[0].Title, outOfScope[0].Title)
	}
	if summary.OutOfScopeFindings[0].AffectedFiles[0] != "README.md" {
		t.Fatalf("affected files = %v", summary.OutOfScopeFindings[0].AffectedFiles)
	}
}

func TestSpecLoopUsesPlanStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-plan-stage"

	git := newFakeGitAdapter(t)
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	planStage := newFakePlanStage(specID)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}
	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if !planStage.called {
		t.Fatalf("plan stage not invoked")
	}
	if planStage.lastRequest == nil {
		t.Fatal("plan stage request missing")
	}
	if planStage.lastRequest.Worktree != git.lastWorktree {
		t.Fatalf("plan request worktree = %q, want %q", planStage.lastRequest.Worktree, git.lastWorktree)
	}
}

func TestSpecLoopStopsWhenContextCanceledBeforeBeadLoop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	specID := "spec-loop-context"
	cfg := &config.Config{}

	recorder := newRecordingStageRecorder()

	git := newFakeGitAdapter(t)
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	planStage := newFakePlanStage(specID)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	decompose := newFakeDecomposeStage(specID)
	decompose.onRun = cancel
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithStageRecorder(recorder),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(ctx, specID, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v, want context canceled", err)
	}

	if beadRunner.ran {
		t.Fatalf("expected bead loop not to run after context cancellation")
	}
}

func TestSpecLoopProvidesStageRequestToAcceptStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-request"

	recorder := newRecordingStageRecorder()

	git := newFakeGitAdapter(t)
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	accept := newFakeAcceptStage()

	cfg := &config.Config{}
	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	planStage := newFakePlanStage(specID)
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}
	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithStageRecorder(recorder),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if accept.lastRequest == nil {
		t.Fatal("accept stage did not receive a request")
	}
	if accept.lastRequest.Bead.ID != specID {
		t.Fatalf("accept request bead = %q, want %q", accept.lastRequest.Bead.ID, specID)
	}
	if accept.lastRequest.Worktree != git.lastWorktree {
		t.Fatalf("accept request worktree = %q, want %q", accept.lastRequest.Worktree, git.lastWorktree)
	}
	if accept.lastRequest.Config != cfg {
		t.Fatalf("accept request config = %p, want %p", accept.lastRequest.Config, cfg)
	}
}

func TestSpecLoopFailureEmitsCompletionAndCleansWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-failure"

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newFakeGitAdapter(t)
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	planStage := newFakePlanStage(specID)
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}
	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithEmitter(emitter),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})),
		WithPreserveOnFailure(false),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err == nil {
		t.Fatal("expected failure from accept stage")
	}

	collected := collectEvents(t, ch, 4)
	completed, ok := collected[len(collected)-1].(*events.SpecCompletedEvent)
	if !ok {
		t.Fatalf("last event = %T, want *events.SpecCompletedEvent", collected[len(collected)-1])
	}
	if completed.Success {
		t.Fatalf("completion success = %v, want false", completed.Success)
	}
	if completed.FailureReason == "" {
		t.Fatalf("expected failure reason, got none")
	}

	if _, statErr := os.Stat(git.lastWorktree); !os.IsNotExist(statErr) {
		t.Fatalf("worktree %q should be removed, stat error = %v", git.lastWorktree, statErr)
	}
}

func TestSpecLoopPassesStopChannelToBeadRunner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "stop-channel"
	stopCh := make(chan struct{})
	defer close(stopCh)

	beadRunner := newFakeBeadRunner()
	planStage := newFakePlanStage(specID)
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}
	loopInstance, err := NewSpecLoop(adapter.AdapterSet{
		Git:         newFakeGitAdapter(t),
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}, &config.Config{}, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(beadRunner),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, stopCh); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if beadRunner.lastStopCh != stopCh {
		t.Fatalf("expected stop channel to propagate to bead runner")
	}
}

func TestEnsureAcceptanceReturnsSuccessAfterRemediation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-ensure-acceptance"

	// Accept fails once, remediation succeeds → ensureAcceptance trusts
	// the remediation result and returns success without re-checking.
	accept := newScriptedAcceptStage(
		stagepkg.Result{Decision: stagepkg.DecisionFail},
	)
	runner := &fakeRemediationRunner{}
	s := &SpecLoop{
		acceptStage:       accept,
		remediationRunner: runner,
	}

	req := stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}}
	res, err := s.ensureAcceptance(ctx, &req, specID)
	if err != nil {
		t.Fatalf("ensure acceptance: %v", err)
	}
	// Result is nil because we trust the remediation runner's success.
	if res != nil {
		t.Fatalf("expected nil result after successful remediation, got %v", res)
	}
	if accept.calls != 1 {
		t.Fatalf("accept stage calls = %d, want 1", accept.calls)
	}
	if runner.calls != 1 {
		t.Fatalf("remediation runner calls = %d, want 1", runner.calls)
	}
}

func TestEnsureAcceptancePropagatesRemediationError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-ensure-acceptance-remediation-error"

	accept := newScriptedAcceptStage(
		stagepkg.Result{Decision: stagepkg.DecisionFail},
	)
	remediationErr := fmt.Errorf("remediation failed")
	runner := &fakeRemediationRunner{err: remediationErr}
	s := &SpecLoop{
		acceptStage:       accept,
		remediationRunner: runner,
	}

	req := stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}}
	_, err := s.ensureAcceptance(ctx, &req, specID)
	if err == nil {
		t.Fatalf("ensure acceptance succeeded unexpectedly")
	}
	if err != remediationErr {
		t.Fatalf("expected remediation error, got %v", err)
	}
	if accept.calls != 1 {
		t.Fatalf("accept stage calls = %d, want 1", accept.calls)
	}
	if runner.calls != 1 {
		t.Fatalf("remediation runner calls = %d, want 1", runner.calls)
	}
}

func TestSpecLoopReusesExistingPlan(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-plan"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	git.planContent = validPlanFixture

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if planStage.called {
		t.Fatal("plan stage should not be called when plan file exists")
	}
}

func TestSpecLoopRunsPlanStageWhenNoPlanFileExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-fresh-plan"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)

	planStage := newFakePlanStage(specID)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}
	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if !planStage.called {
		t.Fatal("plan stage should be called when no plan file exists")
	}
}

func TestSpecLoopReusesExistingBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-beads"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-1", Title: "First bead", Status: "open"},
			{ID: "bead-2", Title: "Second bead", Status: "open"},
		},
	}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTracker,
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if decompose.called {
		t.Fatal("decompose stage should not be called when beads already exist")
	}
	if len(beadRunner.lastBeads) != 2 {
		t.Fatalf("bead runner got %d beads, want 2", len(beadRunner.lastBeads))
	}
	if beadRunner.lastBeads[0].ID != "bead-1" {
		t.Fatalf("first bead ID = %q, want bead-1", beadRunner.lastBeads[0].ID)
	}
}

func TestSpecLoopDecomposesWhenNoExistingBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-fresh-decompose"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	taskTracker := newFakeTaskTrackerAdapter()
	// No queryBeadsResponse set — returns empty

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTracker,
		Presenter:   newFakePresenterAdapter(t),
	}

	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}
	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if !decompose.called {
		t.Fatal("decompose stage should be called when no existing beads")
	}
}

func TestSpecLoopPreservesWorktreeOnEarlyErrorWhenPreserveOnFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-preserve-early-err"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)

	// Use a plan stage that returns an error to trigger the deferred cleanup path.
	failingPlan := &failingPlanStage{err: fmt.Errorf("plan generation failed")}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(failingPlan),
		WithPreserveOnFailure(true),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	runErr := loopInstance.Run(ctx, specID, nil)
	if runErr == nil {
		t.Fatal("expected error from failing plan stage")
	}

	// The worktree should NOT have been removed because preserveOnFailure=true.
	if len(git.removedWorktrees) != 0 {
		t.Fatalf("worktree was removed despite preserveOnFailure=true; removed: %v", git.removedWorktrees)
	}
}

func TestSpecLoopPersistsPlanAfterGeneration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-plan-persist"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)

	// Use a plan stage that returns a plan but does NOT write it to disk.
	nonWritingPlan := &nonWritingPlanStage{plan: "generated plan content"}

	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()
	presentStage := &planCheckingPresentStage{expectedPlan: "generated plan content"}
	summaryCtx := &present.SummaryContext{}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(nonWritingPlan),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	// The planCheckingPresentStage verifies that the plan file existed at
	// presentation time (before worktree cleanup).
	if !presentStage.planFileFound {
		t.Fatal("plan file was not persisted before presentation stage")
	}
}

type planCheckingPresentStage struct {
	planFileFound bool
	expectedPlan  string
}

func (f *planCheckingPresentStage) Name() string { return "present" }

func (f *planCheckingPresentStage) Run(_ context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	planPath := filepath.Join(req.Worktree, ".gromit", "v2", "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("plan file not found at presentation time: %w", err)
	}
	if string(data) != f.expectedPlan {
		return nil, fmt.Errorf("plan file = %q, want %q", string(data), f.expectedPlan)
	}
	f.planFileFound = true
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

func TestQueryExistingBeadsMapsAllFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-full-mapping"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{
				ID:          "bead-full",
				Title:       "Full bead",
				Description: "A fully populated bead",
				Priority:    2,
				Labels:      []string{"label-a", "label-b"},
				Status:      "open",
				DependsOn:   []string{"dep-1", "dep-2"},
				BlockedBy:   []string{"blocker-1"},
			},
		},
	}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTracker,
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if len(beadRunner.lastBeads) != 1 {
		t.Fatalf("bead runner got %d beads, want 1", len(beadRunner.lastBeads))
	}

	b := beadRunner.lastBeads[0]
	if b.ID != "bead-full" {
		t.Fatalf("ID = %q, want %q", b.ID, "bead-full")
	}
	if b.Title != "Full bead" {
		t.Fatalf("Title = %q, want %q", b.Title, "Full bead")
	}
	if b.Description != "A fully populated bead" {
		t.Fatalf("Description = %q, want %q", b.Description, "A fully populated bead")
	}
	if b.Priority != 2 {
		t.Fatalf("Priority = %d, want 2", b.Priority)
	}
	if !reflect.DeepEqual(b.Labels, []string{"label-a", "label-b"}) {
		t.Fatalf("Labels = %v, want [label-a label-b]", b.Labels)
	}
	if b.Status != "open" {
		t.Fatalf("Status = %q, want %q", b.Status, "open")
	}
	if len(b.DependsOn) != 2 {
		t.Fatalf("DependsOn len = %d, want 2", len(b.DependsOn))
	}
	if b.DependsOn[0].ID != "dep-1" || b.DependsOn[1].ID != "dep-2" {
		t.Fatalf("DependsOn = %v, want dep-1 and dep-2", b.DependsOn)
	}
	if len(b.BlockedBy) != 1 || b.BlockedBy[0].ID != "blocker-1" {
		t.Fatalf("BlockedBy = %v, want blocker-1", b.BlockedBy)
	}
}

type nonWritingPlanStage struct {
	plan string
}

func (f *nonWritingPlanStage) Name() string { return "plan" }

func (f *nonWritingPlanStage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	return &stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: &planstage.PlanArtifacts{Plan: f.plan},
	}, nil
}

type failingPlanStage struct {
	err error
}

func (f *failingPlanStage) Name() string { return "plan" }

func (f *failingPlanStage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	return nil, f.err
}

func newFakeDecomposeStage(specID string) *fakeDecomposeStage {
	beads := []*bead.Bead{{ID: specID + "-bead"}}
	return &fakeDecomposeStage{producedBeads: beads}
}

type fakeDecomposeStage struct {
	producedBeads []*bead.Bead
	called        bool
	lastRequest   *stagepkg.Request
	onRun         func()
}

func (f *fakeDecomposeStage) Name() string { return "decompose" }

func (f *fakeDecomposeStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	f.called = true
	f.lastRequest = req
	if f.onRun != nil {
		f.onRun()
	}
	return &stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: &stagepkg.DecomposeArtifacts{Beads: append([]*bead.Bead(nil), f.producedBeads...)},
	}, nil
}

func newFakeBeadRunner() *fakeBeadRunner {
	return &fakeBeadRunner{}
}

type fakeBeadRunner struct {
	ran        bool
	lastBeads  []*bead.Bead
	lastStopCh <-chan struct{}
	result     BeadLoopResult
}

func (f *fakeBeadRunner) Run(ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) (BeadLoopResult, error) {
	f.ran = true
	f.lastBeads = append([]*bead.Bead(nil), beads...)
	f.lastStopCh = stopCh
	return f.result, nil
}

func newFakeAcceptStage() *fakeAcceptStage {
	return &fakeAcceptStage{
		results: []presentation.AcceptanceResult{{Title: "criterion", Description: "desc"}},
	}
}

type fakeAcceptStage struct {
	results     []presentation.AcceptanceResult
	called      bool
	lastRequest *stagepkg.Request
}

func (f *fakeAcceptStage) Name() string { return "accept" }

func (f *fakeAcceptStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	f.called = true
	f.lastRequest = req
	return &stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: &stageaccept.AcceptArtifacts{Results: append([]presentation.AcceptanceResult(nil), f.results...)},
	}, nil
}

type fakePlanStage struct {
	plan        string
	called      bool
	lastRequest *stagepkg.Request
}

func newFakePlanStage(specID string) *fakePlanStage {
	plan := specID + "-plan"
	return &fakePlanStage{plan: plan}
}

func (f *fakePlanStage) Name() string { return "plan" }

func (f *fakePlanStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	f.called = true
	f.lastRequest = req
	planPath := filepath.Join(req.Worktree, ".gromit", "v2", "plan.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		return nil, fmt.Errorf("create plan directory: %w", err)
	}
	if err := os.WriteFile(planPath, []byte(f.plan), 0o644); err != nil {
		return nil, fmt.Errorf("write plan file: %w", err)
	}
	return &stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &planstage.PlanArtifacts{
			SpecID: req.Bead.ID,
			Plan:   f.plan,
			Path:   planPath,
			Model:  "opus",
		},
	}, nil
}

type fakePresentStage struct {
	called bool
}

func newFakePresentStage() *fakePresentStage {
	return &fakePresentStage{}
}

func (f *fakePresentStage) Name() string { return "present" }

func (f *fakePresentStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	f.called = true
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

func TestWithTypedEmitterSetsField(t *testing.T) {
	t.Parallel()
	em := event.NewEmitter()
	defer em.Close()
	s := &SpecLoop{}
	WithTypedEmitter(em)(s)
	if s.typedEmitter != em {
		t.Fatalf("typedEmitter not set")
	}
}

func TestWithStageCommitterSetsField(t *testing.T) {
	t.Parallel()
	sc := &fakeSpecStageCommitter{}
	s := &SpecLoop{}
	WithStageCommitter(sc)(s)
	if s.stageCommitter != sc {
		t.Fatalf("stageCommitter not set")
	}
}

// fakeSpecStageCommitter records CommitStage calls for assertion in spec loop tests.
type fakeSpecStageCommitter struct {
	calls []specStageCommitCall
}

type specStageCommitCall struct {
	stageName string
	beadID    string
}

func (f *fakeSpecStageCommitter) CommitStage(_ context.Context, _, beadID, stageName string, _ int, _ string) error {
	f.calls = append(f.calls, specStageCommitCall{stageName: stageName, beadID: beadID})
	return nil
}

func (f *fakeSpecStageCommitter) hasCall(stageName string) bool {
	for _, c := range f.calls {
		if c.stageName == stageName {
			return true
		}
	}
	return false
}

func TestSpecLoopCommitsUseEmptyBeadIDForSpecLevelStages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-commit-empty-bead-id"
	cfg := &config.Config{}

	sc := &fakeSpecStageCommitter{}
	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         newFakeGitAdapter(t),
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithStageCommitter(sc),
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, c := range sc.calls {
		if c.beadID != "" {
			t.Errorf("CommitStage for stage %q got beadID=%q, want empty string (spec-level scope)", c.stageName, c.beadID)
		}
	}
}

func TestSpecLoopCommitsAfterPresentStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-commit-present"
	cfg := &config.Config{}

	sc := &fakeSpecStageCommitter{}
	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         newFakeGitAdapter(t),
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithStageCommitter(sc),
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !sc.hasCall("present") {
		t.Fatalf("CommitStage not called with 'present'; calls: %v", sc.calls)
	}
}

func TestSpecLoopCommitsAfterAcceptStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-commit-accept"
	cfg := &config.Config{}

	sc := &fakeSpecStageCommitter{}
	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         newFakeGitAdapter(t),
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithStageCommitter(sc),
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !sc.hasCall("accept") {
		t.Fatalf("CommitStage not called with 'accept'; calls: %v", sc.calls)
	}
}

func TestSpecLoopCommitsAfterDecomposeStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-commit-decompose"
	cfg := &config.Config{}

	sc := &fakeSpecStageCommitter{}
	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         newFakeGitAdapter(t),
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithStageCommitter(sc),
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !sc.hasCall("decompose") {
		t.Fatalf("CommitStage not called with 'decompose'; calls: %v", sc.calls)
	}
}

func TestSpecLoopCommitsAfterPlanStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-commit-plan"
	cfg := &config.Config{}

	sc := &fakeSpecStageCommitter{}
	git := newFakeGitAdapter(t)
	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithStageCommitter(sc),
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !sc.hasCall("plan") {
		t.Fatalf("CommitStage not called with 'plan'; calls: %v", sc.calls)
	}
}

func TestSpecLoopCreatesEventsFileWhenTypedEmitterSet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-events-file"
	cfg := &config.Config{}

	em := event.NewEmitter()

	git := newFakeGitAdapter(t)
	git.noRemove = true // preserve worktree so events file survives cleanup
	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithTypedEmitter(em),
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	em.Close() // flush all events to disk

	eventsPath := filepath.Join(git.lastWorktree, ".gromit", "v2", "events.jsonl")
	if _, statErr := os.Stat(eventsPath); os.IsNotExist(statErr) {
		t.Fatalf("events file not created at %s", eventsPath)
	}
}

func TestResumeWithGapAnalysis_FailureSummaryPopulated(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-gap-analysis"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	git.planContent = validPlanFixture
	git.gapAnalysisContent = "remaining: implement step X"

	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-remaining", Title: "Remaining bead", Status: "open"},
		},
	}

	fp := newFakePresenterAdapter(t)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, fp)

	// Accept fails once with no remediation runner — triggers handleFailure + gap analysis.
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: taskTracker,
			Presenter:   fp,
		},
		cfg, noopDependencyGate{},
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err == nil {
		t.Fatal("expected failure from accept stage")
	}

	requireGapAnalysisInFailureSummary(t, fp, "remaining: implement step X")
}

func TestResumeWithExistingPlanAndBeads_BeadListCorrect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-plan-and-beads"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	git.planContent = validPlanFixture

	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-alpha", Title: "Alpha bead", Status: "open"},
			{ID: "bead-beta", Title: "Beta bead", Status: "open"},
		},
	}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: taskTracker,
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if planStage.called {
		t.Fatal("plan stage should not be called when plan file exists")
	}
	if decompose.called {
		t.Fatal("decompose stage should not be called when beads already exist")
	}
	requireBeadIDs(t, beadRunner, []string{"bead-alpha", "bead-beta"})
}

func TestSpecLoop_ResumeSkipsDecomposeWhenClosedBeadsExist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-closed-beads"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	git.planContent = validPlanFixture

	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-done-1", Title: "Done bead", Status: "closed"},
			{ID: "bead-done-2", Title: "Another done bead", Status: "closed"},
		},
	}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: taskTracker,
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if planStage.called {
		t.Fatal("plan stage should not be called when plan file exists")
	}
	if decompose.called {
		t.Fatal("decompose stage should not be called when closed beads exist for the spec")
	}
}

func TestSpecLoop_ResumeSkipsDecomposeWhenMixedStatusBeadsExist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-mixed-beads"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	git.planContent = validPlanFixture

	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-closed-1", Title: "Closed bead 1", Status: "closed"},
			{ID: "bead-closed-2", Title: "Closed bead 2", Status: "closed"},
			{ID: "bead-open-1", Title: "Open bead", Status: "open"},
		},
	}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: taskTracker,
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if decompose.called {
		t.Fatal("decompose stage should not be called when beads already exist (mixed status)")
	}
	requireBeadIDs(t, beadRunner, []string{"bead-open-1"})
}

func TestSpecLoop_FirstRunDecomposesWhenNoBeadsExist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-first-run-decompose"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	// No planContent — forces plan stage to run

	taskTracker := newFakeTaskTrackerAdapter()
	// No queryBeadsResponse — no beads exist, forces decompose

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: taskTracker,
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if !planStage.called {
		t.Fatal("plan stage should be called on first run (no plan file)")
	}
	if !decompose.called {
		t.Fatal("decompose stage should be called on first run (no beads)")
	}
}

func TestRemediationRunnerReceivesCorrectWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-remediation-worktree"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	runner := &recordingRemediationRunner{}

	accept := newScriptedAcceptStage(
		stagepkg.Result{Decision: stagepkg.DecisionFail},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
	)

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(accept),
		WithRemediationRunner(runner),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("remediation calls = %d, want 1", runner.calls)
	}
	if runner.lastWorktree != git.lastWorktree {
		t.Fatalf("remediation worktree = %q, want %q", runner.lastWorktree, git.lastWorktree)
	}
	if runner.lastSpecID != specID {
		t.Fatalf("remediation specID = %q, want %q", runner.lastSpecID, specID)
	}
}

func newPresentStageForTest(t *testing.T, cfg *config.Config, presenter adapter.PresenterAdapter) (stagepkg.Stage, *present.SummaryContext) {
	t.Helper()
	summaryCtx := &present.SummaryContext{}
	stage, err := present.New(cfg, presenter, summaryCtx)
	if err != nil {
		t.Fatalf("create present stage: %v", err)
	}
	return stage, summaryCtx
}

func TestSelectiveRevalidation_RequeuesFailedBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-revalidation"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-open-1", Title: "Open bead", Status: "open"},
		},
	}

	revalidatedBead := &bead.Bead{ID: "bead-completed-failed", Title: "Regressed bead"}
	revalidator := &fakeSelectiveRevalidator{requeueBeads: []*bead.Bead{revalidatedBead}}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTracker,
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
		WithSelectiveRevalidator(revalidator),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	wantIDs := []string{"bead-open-1", "bead-completed-failed"}
	requireBeadIDs(t, beadRunner, wantIDs)
}

func TestSelectiveRevalidation_ErrorPropagates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-revalidation-error"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-1", Title: "A bead", Status: "open"},
		},
	}

	revalidator := &fakeSelectiveRevalidator{err: fmt.Errorf("validation infrastructure failed")}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTracker,
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
		WithSelectiveRevalidator(revalidator),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(ctx, specID, nil)
	if err == nil {
		t.Fatal("expected error from revalidator, got nil")
	}
	if !strings.Contains(err.Error(), "selective revalidation") {
		t.Fatalf("error = %q, want to contain 'selective revalidation'", err.Error())
	}
}

func TestSelectiveRevalidation_SkippedOnFreshRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-fresh-revalidation"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	taskTracker := newFakeTaskTrackerAdapter()
	// No queryBeadsResponse — fresh run, no existing beads

	revalidator := &fakeSelectiveRevalidator{}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTracker,
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
		WithSelectiveRevalidator(revalidator),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if revalidator.calls != 0 {
		t.Fatalf("revalidator called %d times on fresh run, want 0", revalidator.calls)
	}
}

type fakeSelectiveRevalidator struct {
	requeueBeads []*bead.Bead
	err          error
	calls        int
	lastBeads    []*bead.Bead
	lastWorktree string
}

func (f *fakeSelectiveRevalidator) Revalidate(ctx context.Context, beads []*bead.Bead, worktree string) ([]*bead.Bead, error) {
	f.calls++
	f.lastBeads = append([]*bead.Bead(nil), beads...)
	f.lastWorktree = worktree
	return f.requeueBeads, f.err
}

func TestSpecLoopRoutingErrorFallsBackToDefault(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-routing-fallback"
	cfg := &config.Config{}

	// Router with no providers — Select() returns ErrNoProviders.
	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{}, // empty: triggers ErrNoProviders
		Cooldown:  time.Minute,
	})

	git := newFakeGitAdapter(t)
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	planStage := newFakePlanStage(specID)
	presentStage := newFakePresentStage()

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()
	recorder := newRecordingStageRecorder()

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithStageRecorder(recorder),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, &present.SummaryContext{}),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
		WithRouter(r),
		WithPhaseModels(map[string]string{
			"plan":      "high",
			"decompose": "medium",
			"accept":    "high",
		}),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	// All three routed stages must still execute (graceful degradation).
	if !planStage.called {
		t.Fatal("plan stage was not called")
	}
	if !decompose.called {
		t.Fatal("decompose stage was not called")
	}
	if !accept.called {
		t.Fatal("accept stage was not called")
	}

	// Provider and Model should remain empty since routing failed.
	if planStage.lastRequest.Provider != nil {
		t.Fatal("plan Provider should be nil when routing returns an error")
	}
	if planStage.lastRequest.Model != "" {
		t.Fatalf("plan Model = %q, want empty", planStage.lastRequest.Model)
	}
	if decompose.lastRequest.Provider != nil {
		t.Fatal("decompose Provider should be nil when routing returns an error")
	}
	if decompose.lastRequest.Model != "" {
		t.Fatalf("decompose Model = %q, want empty", decompose.lastRequest.Model)
	}
	if accept.lastRequest.Provider != nil {
		t.Fatal("accept Provider should be nil when routing returns an error")
	}
	if accept.lastRequest.Model != "" {
		t.Fatalf("accept Model = %q, want empty", accept.lastRequest.Model)
	}
}

func TestSpecLoop_HasBeadsForSpecError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-has-beads-error"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	git.planContent = validPlanFixture

	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsErr = fmt.Errorf("tracker unavailable")

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: taskTracker,
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(ctx, specID, nil)
	if err == nil {
		t.Fatal("expected error from QueryBeads, got nil")
	}
	if !strings.Contains(err.Error(), "check existing beads") {
		t.Fatalf("error = %q, want to contain 'check existing beads'", err.Error())
	}
}

func TestSpecLoop_BeadsForSpecError_OnResumePath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-beads-error-resume"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	// Pre-populate plan so the plan stage is skipped (resume path).
	git.planContent = validPlanFixture

	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsErr = fmt.Errorf("tracker connection lost")

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: taskTracker,
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(ctx, specID, nil)
	if err == nil {
		t.Fatal("expected error from beads query on resume path, got nil")
	}
	if !strings.Contains(err.Error(), "check existing beads") {
		t.Fatalf("error = %q, want to contain 'check existing beads'", err.Error())
	}
	if !strings.Contains(err.Error(), "tracker connection lost") {
		t.Fatalf("error = %q, want to contain wrapped cause 'tracker connection lost'", err.Error())
	}
}

func TestSpecLoop_ResumeSkipsPlanWhenPlanFileExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-skip-plan"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	git.planContent = validPlanFixture

	taskTracker := newFakeTaskTrackerAdapter()
	taskTracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-open-1", Title: "Open bead 1", Status: "open"},
			{ID: "bead-open-2", Title: "Open bead 2", Status: "open"},
		},
	}

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: taskTracker,
			Presenter:   newFakePresenterAdapter(t),
		},
		cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if planStage.called {
		t.Fatal("plan stage should NOT be called when plan file already exists")
	}
	if decompose.called {
		t.Fatal("decompose stage should NOT be called when open beads already exist")
	}
	requireBeadIDs(t, beadRunner, []string{"bead-open-1", "bead-open-2"})
}
