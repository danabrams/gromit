package loop

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	planstage "github.com/danabrams/gromit/internal/v2/stage/plan"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
)

func TestSpecLoopHappyPathExecutesPipeline(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-happy"

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

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()
	accept := newFakeAcceptStage()

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithStageRecorder(recorder),
		WithEmitter(emitter),
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

	if taskTracker.lastPlan != llm.planFor(specID) {
		t.Fatalf("plan recorded = %q, want %q", taskTracker.lastPlan, llm.planFor(specID))
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

func TestSpecLoopPersistsPlanInGromitDir(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	cfg := &config.Config{}
	loopInstance := &SpecLoop{cfg: cfg}
	plan := "persisted plan"
	if err := loopInstance.writePlanFile(worktree, plan); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	planPath := filepath.Join(worktree, ".gromit", "v2", "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan file: %v", err)
	}
	if string(data) != plan {
		t.Fatalf("plan file = %q, want %q", string(data), plan)
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

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithPlanStage(planStage),
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
	if taskTracker.lastPlan != planStage.plan {
		t.Fatalf("plan recorded = %q, want %q", taskTracker.lastPlan, planStage.plan)
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

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithStageRecorder(recorder),
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

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithEmitter(emitter),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})),
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
	loopInstance, err := NewSpecLoop(adapter.AdapterSet{
		Git:         newFakeGitAdapter(t),
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}, &config.Config{}, noopDependencyGate{},
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

func newFakeDecomposeStage(specID string) *fakeDecomposeStage {
	beads := []*bead.Bead{{ID: specID + "-bead"}}
	return &fakeDecomposeStage{producedBeads: beads}
}

type fakeDecomposeStage struct {
	producedBeads []*bead.Bead
	called        bool
}

func (f *fakeDecomposeStage) Name() string { return "decompose" }

func (f *fakeDecomposeStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	f.called = true
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
}

func (f *fakeBeadRunner) Run(ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) error {
	f.ran = true
	f.lastBeads = append([]*bead.Bead(nil), beads...)
	f.lastStopCh = stopCh
	return nil
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
	return &stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &planstage.PlanArtifacts{
			SpecID: req.Bead.ID,
			Plan:   f.plan,
			Path:   filepath.Join(req.Worktree, ".gromit", "v2", "plan.md"),
			Model:  "opus",
		},
	}, nil
}
