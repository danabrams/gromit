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

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/presentation"
	v2review "github.com/danabrams/gromit/internal/v2/review"
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

func TestEnsureAcceptanceRetriesRemediationUntilSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-ensure-acceptance"

	accept := newScriptedAcceptStage(
		stagepkg.Result{Decision: stagepkg.DecisionFail},
		stagepkg.Result{Decision: stagepkg.DecisionFail},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
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
	if res == nil || res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("accept decision = %v, want proceed", res)
	}
	if accept.calls != 3 {
		t.Fatalf("accept stage calls = %d, want 3", accept.calls)
	}
	if runner.calls != 2 {
		t.Fatalf("remediation runner calls = %d, want 2", runner.calls)
	}
}

func TestEnsureAcceptanceStopsAfterMaxRetries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-ensure-acceptance-max-retries"

	failures := make([]stagepkg.Result, maxAcceptanceRetries+1)
	for i := range failures {
		failures[i].Decision = stagepkg.DecisionFail
	}
	accept := newScriptedAcceptStage(failures...)
	runner := &fakeRemediationRunner{}
	s := &SpecLoop{
		acceptStage:       accept,
		remediationRunner: runner,
	}

	req := stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}}
	res, err := s.ensureAcceptance(ctx, &req, specID)
	if err == nil {
		t.Fatalf("ensure acceptance succeeded unexpectedly")
	}
	if !errors.Is(err, ErrAcceptanceRetriesExceeded) {
		t.Fatalf("expected error %v, got %v", ErrAcceptanceRetriesExceeded, err)
	}
	if res == nil || res.Decision != stagepkg.DecisionFail {
		t.Fatalf("accept decision = %v, want fail", res)
	}
	if accept.calls != maxAcceptanceRetries+1 {
		t.Fatalf("accept stage calls = %d, want %d", accept.calls, maxAcceptanceRetries+1)
	}
	if runner.calls != maxAcceptanceRetries {
		t.Fatalf("remediation runner calls = %d, want %d", runner.calls, maxAcceptanceRetries)
	}
}

func TestSpecLoopReusesExistingPlan(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-plan"
	cfg := &config.Config{}

	git := newFakeGitAdapter(t)
	git.planContent = "existing plan from prior run"

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
			{ID: "bead-1", Title: "First bead"},
			{ID: "bead-2", Title: "Second bead"},
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

func newFakeDecomposeStage(specID string) *fakeDecomposeStage {
	beads := []*bead.Bead{{ID: specID + "-bead"}}
	return &fakeDecomposeStage{producedBeads: beads}
}

type fakeDecomposeStage struct {
	producedBeads []*bead.Bead
	called        bool
	onRun         func()
}

func (f *fakeDecomposeStage) Name() string { return "decompose" }

func (f *fakeDecomposeStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	f.called = true
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

func newPresentStageForTest(t *testing.T, cfg *config.Config, presenter adapter.PresenterAdapter) (stagepkg.Stage, *present.SummaryContext) {
	t.Helper()
	summaryCtx := &present.SummaryContext{}
	stage, err := present.New(cfg, presenter, summaryCtx)
	if err != nil {
		t.Fatalf("create present stage: %v", err)
	}
	return stage, summaryCtx
}
