package loop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestSpecLoopHappyPathUsesAdaptersAndEmitsEvents(t *testing.T) {
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

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{}, WithStageRecorder(recorder), WithEmitter(emitter))
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if !presenter.planFileVerified {
		t.Fatalf("presenter never validated plan file")
	}

	if taskTracker.lastSpecID != specID || taskTracker.lastPlan != llm.planFor(specID) {
		t.Fatalf("task tracker recorded wrong plan: got %v/%v, want %v/%v", taskTracker.lastSpecID, taskTracker.lastPlan, specID, llm.planFor(specID))
	}

	requireStageSequence(t, recorder)

	received := collectEvents(t, ch, 2)
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}

	started, ok := received[0].(*events.SpecStartedEvent)
	if !ok {
		t.Fatalf("first event = %T, want *events.SpecStartedEvent", received[0])
	}
	if started.SpecID != specID || started.Worktree != git.lastWorktree {
		t.Fatalf("spec started event mismatch = %#v", started)
	}

	completed, ok := received[1].(*events.SpecCompletedEvent)
	if !ok {
		t.Fatalf("second event = %T, want *events.SpecCompletedEvent", received[1])
	}
	if completed.SpecID != specID || completed.Worktree != git.lastWorktree || !completed.Success || completed.FailureReason != "" {
		t.Fatalf("unexpected spec completed event = %#v", completed)
	}

	if _, err := os.Stat(git.lastWorktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree still exists after run: %v", err)
	}
}

func TestSpecLoopFailurePathPreservesWorktreeAndEmitsEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-failure"

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	recorder := newRecordingStageRecorder()

	git := newFakeGitAdapter(t)
	gapContent := "  missing validation coverage  "
	git.gapAnalysisContent = gapContent
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	runnerErr := errors.New("generation cap reached")
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})
	runner := &fakeRemediationRunner{err: runnerErr}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{}, WithStageRecorder(recorder), WithEmitter(emitter), WithAcceptStage(accept), WithRemediationRunner(runner))
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err == nil {
		t.Fatal("expected Run to return an error")
	}

	if accept.calls != 1 {
		t.Fatalf("accept runs = %d, want 1", accept.calls)
	}
	if runner.calls != 1 {
		t.Fatalf("remediation runs = %d, want 1", runner.calls)
	}

	wantStages := []string{"plan", "decompose", "gate", "build", "validate", "review", "epilogue", "accept", "gap-analysis", "decompose", "bead-loop", "present"}
	if got := recorder.stageNames(); !reflect.DeepEqual(got, wantStages) {
		t.Fatalf("stages = %v, want %v", got, wantStages)
	}

	if presenter.lastSummary.Success {
		t.Fatalf("presenter summary success = true, want false")
	}
	if !strings.Contains(presenter.lastSummary.FailureSummary, runnerErr.Error()) {
		t.Fatalf("failure summary missing runner error: %q", presenter.lastSummary.FailureSummary)
	}
	trimmedGap := strings.TrimSpace(gapContent)
	if len(presenter.lastSummary.RemainingWork) != 1 || presenter.lastSummary.RemainingWork[0] != trimmedGap {
		t.Fatalf("remaining work = %v, want [%q]", presenter.lastSummary.RemainingWork, trimmedGap)
	}

	if _, err := os.Stat(git.lastWorktree); err != nil {
		t.Fatalf("worktree removed after failure: %v", err)
	}

	received := collectEvents(t, ch, 3)
	if _, ok := received[0].(*events.SpecStartedEvent); !ok {
		t.Fatalf("first event = %T, want *events.SpecStartedEvent", received[0])
	}
	andon, ok := received[1].(*events.AndonTriggeredEvent)
	if !ok {
		t.Fatalf("second event = %T, want *events.AndonTriggeredEvent", received[1])
	}
	if andon.SpecID != specID {
		t.Fatalf("andon event spec = %s, want %s", andon.SpecID, specID)
	}
	failed, ok := received[2].(*events.SpecFailedEvent)
	if !ok {
		t.Fatalf("third event = %T, want *events.SpecFailedEvent", received[2])
	}
	if failed.SpecID != specID {
		t.Fatalf("spec failed event spec = %s, want %s", failed.SpecID, specID)
	}
}

func TestSpecLoopInvokesRemediationWhenAcceptFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-remediate"

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newFakeGitAdapter(t)
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})
	runner := &fakeRemediationRunner{}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{}, WithEmitter(emitter), WithAcceptStage(accept), WithRemediationRunner(runner))
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if accept.calls != 1 {
		t.Fatalf("accept runs = %d, want 1", accept.calls)
	}
	if runner.calls != 1 {
		t.Fatalf("remediation runs = %d, want 1", runner.calls)
	}
	if !presenter.lastSummary.Success {
		t.Fatalf("presenter summary success = %v, want true", presenter.lastSummary.Success)
	}
	if _, err := os.Stat(git.lastWorktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree still exists after success: %v", err)
	}
}

func requireStageSequence(t *testing.T, recorder *recordingStageRecorder) {
	t.Helper()
	if got := recorder.stageNames(); !reflect.DeepEqual(got, StageSequence) {
		t.Fatalf("stage order = %v, want %v", got, StageSequence)
	}
}

func collectEvents(t *testing.T, ch chan events.Event, target int) []events.Event {
	t.Helper()
	collected := make([]events.Event, 0, target)
	deadline := time.After(time.Second)
	for len(collected) < target {
		select {
		case evt := <-ch:
			collected = append(collected, evt)
		case <-deadline:
			t.Fatalf("timed out waiting for events, got %d", len(collected))
		}
	}
	return collected
}

type recordingStageRecorder struct {
	mu    sync.Mutex
	names []string
}

func newRecordingStageRecorder() *recordingStageRecorder {
	return &recordingStageRecorder{}
}

func (r *recordingStageRecorder) RecordStage(name string) {
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
}

func (r *recordingStageRecorder) stageNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, len(r.names))
	copy(names, r.names)
	return names
}

type fakeGitAdapter struct {
	t           *testing.T
	lastWorktree     string
	gapAnalysisContent string
}

func newFakeGitAdapter(t *testing.T) *fakeGitAdapter {
	return &fakeGitAdapter{t: t}
}

func (f *fakeGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	f.t.Helper()
	worktree := filepath.Join(f.t.TempDir(), specID)
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		f.t.Fatalf("mkdir worktree: %v", err)
	}
	if f.gapAnalysisContent != "" {
		path := filepath.Join(worktree, "gap-analysis.md")
		if err := os.WriteFile(path, []byte(f.gapAnalysisContent), 0o644); err != nil {
			f.t.Fatalf("write gap analysis: %v", err)
		}
	}
	f.lastWorktree = worktree
	return worktree, nil
}

type fakeLLMAdapter struct{}

func newFakeLLMAdapter() *fakeLLMAdapter {
	return &fakeLLMAdapter{}
}

func (f *fakeLLMAdapter) GeneratePlan(ctx context.Context, specID string) (string, error) {
	return specID + "-plan", nil
}

func (f *fakeLLMAdapter) planFor(specID string) string {
	return specID + "-plan"
}

type fakeTaskTrackerAdapter struct {
	lastSpecID string
	lastPlan   string
}

func newFakeTaskTrackerAdapter() *fakeTaskTrackerAdapter {
	return &fakeTaskTrackerAdapter{}
}

func (f *fakeTaskTrackerAdapter) RecordPlan(ctx context.Context, specID, plan string) error {
	f.lastSpecID = specID
	f.lastPlan = plan
	return nil
}

type fakePresenterAdapter struct {
	t             *testing.T
	lastSummary   presentation.PresentationSummary
	planFileVerified bool
}

func newFakePresenterAdapter(t *testing.T) *fakePresenterAdapter {
	return &fakePresenterAdapter{t: t}
}

func (f *fakePresenterAdapter) PresentSummary(ctx context.Context, specID string, summary presentation.PresentationSummary) error {
	f.t.Helper()
	f.lastSummary = summary
	planPath := filepath.Join(summary.Worktree, "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		f.t.Fatalf("read plan file: %v", err)
	}
	if string(data) != summary.Plan {
		f.t.Fatalf("plan file mismatch = %q, want %q", string(data), summary.Plan)
	}
	f.planFileVerified = true
	return nil
}

type noopDependencyGate struct{}

func (noopDependencyGate) EnsureSpecReady(ctx context.Context, specID string) error {
	return nil
}

type fakeRemediationRunner struct {
	calls int
	err   error
}

func (f *fakeRemediationRunner) Run(_ context.Context, _ string) error {
	f.calls++
	return f.err
}

type scriptedAcceptStage struct {
	calls   int
	results []stagepkg.Result
	err     error
}

func newScriptedAcceptStage(results ...stagepkg.Result) *scriptedAcceptStage {
	copied := append([]stagepkg.Result(nil), results...)
	return &scriptedAcceptStage{results: copied}
}

func (s *scriptedAcceptStage) Name() string { return "accept" }

func (s *scriptedAcceptStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if len(s.results) == 0 {
		return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
	}
	res := s.results[0]
	s.results = s.results[1:]
	result := res
	return &result, nil
}
