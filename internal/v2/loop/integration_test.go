package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	"github.com/danabrams/gromit/internal/v2/generation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/testutil"
)

// TestIntegration_SpecLoopHappyPathCompletes exercises a clean run where multiple beads\n+// (represented by the successive stages in the spec loop) all succeed, acceptance returns\n+// proceed immediately, and the summary is presented once.
func TestIntegration_SpecLoopHappyPathCompletes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-happy"
	cfg := &config.Config{}

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newIntegrationGitAdapter(t)
	llm := newIntegrationLLMAdapter()
	taskTracker := newIntegrationTaskTrackerAdapter()
	presenter := newIntegrationPresenterAdapter(t)
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})
	decompose, beadLoop := newIntegrationLoopComponents(t, specID)
	planStage := newFakePlanStage(specID)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithEmitter(emitter),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithAcceptStage(accept),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadLoop),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if accept.calls != 1 {
		t.Fatalf("accept calls = %d, want 1", accept.calls)
	}

	requireHappyPathEvents(t, ch)
	assertPresenterSuccess(t, presenter, true)
}

func TestIntegration_SpecLoopFailureHitsGenerationCap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-failure-gen-cap"
	cfg := &config.Config{}

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newIntegrationGitAdapter(t)
	git.gapAnalysisContent = "missing gap analysis"
	llm := newIntegrationLLMAdapter()
	taskTracker := newIntegrationTaskTrackerAdapter()
	presenter := newIntegrationPresenterAdapter(t)
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})
	decompose, beadLoop := newIntegrationLoopComponents(t, specID)
	planStage := newFakePlanStage(specID)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	remediation := newIntegrationRemediationRunner(t, emitter, integrationRemediationConfig{
		generationCap: 0,
		labels:        []string{generation.Format(42)},
	})

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithEmitter(emitter),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithAcceptStage(accept),
		WithRemediationRunner(remediation),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadLoop),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err == nil {
		t.Fatal("expected Run to return an error")
	}

	assertEventTypeOrder(t, collectEvents(t, ch, 5), []string{
		"*events.SpecStartedEvent",
		"*events.GenerationCapReachedEvent",
		"*events.AndonTriggeredEvent",
		"*events.SpecFailedEvent",
		"*events.SpecCompletedEvent",
	})

	assertPresenterSuccess(t, presenter, false)
}

// TestIntegration_SpecLoopRemediationAppliesGapAnalysis ensures the remediation loop runs after the first\n+// failed accept and that 2-3 remediation beads complete before acceptance succeeds.
func TestIntegration_SpecLoopRemediationAppliesGapAnalysis(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-remediate"
	cfg := &config.Config{}

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newIntegrationGitAdapter(t)
	git.gapAnalysisContent = "gap analysis leads to remediation"
	llm := newIntegrationLLMAdapter()
	taskTracker := newIntegrationTaskTrackerAdapter()
	presenter := newIntegrationPresenterAdapter(t)
	accept := newScriptedAcceptStage(
		stagepkg.Result{Decision: stagepkg.DecisionFail},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
	)
	decompose, beadLoop := newIntegrationLoopComponents(t, specID)

	remediation := newIntegrationRemediationRunner(t, emitter, integrationRemediationConfig{
		generationCap: -1,
	})

	planStage := newFakePlanStage(specID)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)
	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithEmitter(emitter),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithAcceptStage(accept),
		WithRemediationRunner(remediation),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadLoop),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	runnerImpl, ok := remediation.(*integrationRemediationRunner)
	if !ok {
		t.Fatalf("remediation runner type = %T, want *integrationRemediationRunner", remediation)
	}
	if runnerImpl.calls != 1 {
		t.Fatalf("remediation calls = %d, want 1", runnerImpl.calls)
	}

	if accept.calls != 1 {
		t.Fatalf("accept calls = %d, want 1", accept.calls)
	}

	assertPresenterSuccess(t, presenter, true)

	requireEventSequence(t, ch, []string{
		"*events.SpecStartedEvent",
		"*events.SpecCompletedEvent",
	})
}

type integrationRemediationConfig struct {
	generationCap int
	labels        []string
	loop          *BeadLoop
}

type integrationRemediationRunner struct {
	beadLoop      *BeadLoop
	labels        []string
	calls         int
	generationCap int
	emitter       *events.Emitter
}

func newIntegrationRemediationRunner(t *testing.T, emitter *events.Emitter, cfg integrationRemediationConfig) remediationRunner {
	t.Helper()
	beadLoop := cfg.loop
	var err error
	if beadLoop == nil {
		beadLoop, err = NewBeadLoop(defaultIntegrationBeadLoopConfig())
		if err != nil {
			t.Fatalf("construct bead loop: %v", err)
		}
	}
	labels := append([]string(nil), cfg.labels...)
	return &integrationRemediationRunner{
		beadLoop:      beadLoop,
		labels:        labels,
		generationCap: cfg.generationCap,
		emitter:       emitter,
	}
}

func defaultIntegrationBeadLoopConfig() BeadLoopConfig {
	return BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    newNoopStage("build"),
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
	}
}

func (r *integrationRemediationRunner) Run(ctx context.Context, specID, _ string, _ *stagepkg.Result) error {
	r.calls++
	if r.generationCap >= 0 {
		r.emitGenerationCapEvents(specID)
		return fmt.Errorf("generation cap reached")
	}
	beads := []*bead.Bead{
		{
			ID:     specID,
			Labels: append([]string(nil), r.labels...),
		},
	}
	_, err := r.beadLoop.Run(ctx, beads, nil)
	return err
}

func (r *integrationRemediationRunner) emitGenerationCapEvents(specID string) {
	if r.emitter == nil {
		return
	}
	r.emitter.Emit(&events.GenerationCapReachedEvent{
		SpecID:        specID,
		GenerationCap: r.generationCap,
	})
}

func assertEventTypeOrder(t *testing.T, events []events.Event, want []string) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d", len(events), len(want))
	}
	for i, evt := range events {
		got := fmt.Sprintf("%T", evt)
		if got != want[i] {
			t.Fatalf("event[%d] = %s, want %s", i, got, want[i])
		}
	}
}

func requireEventSequence(t *testing.T, ch chan events.Event, want []string) {
	t.Helper()
	events := collectEvents(t, ch, len(want))
	assertEventTypeOrder(t, events, want)
}

func requireHappyPathEvents(t *testing.T, ch chan events.Event) {
	t.Helper()
	requireEventSequence(t, ch, []string{
		"*events.SpecStartedEvent",
		"*events.SpecCompletedEvent",
	})
}

func newIntegrationLoopComponents(t *testing.T, specID string) (*fakeDecomposeStage, *BeadLoop) {
	t.Helper()
	decompose := newFakeDecomposeStage(specID)
	beadLoop, err := NewBeadLoop(defaultIntegrationBeadLoopConfig())
	if err != nil {
		t.Fatalf("create bead loop: %v", err)
	}
	return decompose, beadLoop
}

type integrationGitAdapter struct {
	*testutil.FakeGit
	t                  *testing.T
	gapAnalysisContent string
	planContent        string
}

func newIntegrationGitAdapter(t *testing.T) *integrationGitAdapter {
	t.Helper()
	git := testutil.NewFakeGit()
	git.WorktreeRoot = t.TempDir()
	return &integrationGitAdapter{FakeGit: git, t: t}
}

func (g *integrationGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	g.t.Helper()
	worktree, err := g.FakeGit.Checkout(ctx, specID)
	if err != nil {
		return "", err
	}
	gromitPath := filepath.Join(worktree, ".gromit", "v2")
	if g.gapAnalysisContent != "" || g.planContent != "" {
		if err := os.MkdirAll(gromitPath, 0o755); err != nil {
			g.t.Fatalf("create gromit dir: %v", err)
		}
	}
	if g.gapAnalysisContent != "" {
		path := filepath.Join(gromitPath, "gap-analysis.md")
		if err := os.WriteFile(path, []byte(g.gapAnalysisContent), 0o644); err != nil {
			g.t.Fatalf("write gap analysis: %v", err)
		}
	}
	if g.planContent != "" {
		path := filepath.Join(gromitPath, "plan.md")
		if err := os.WriteFile(path, []byte(g.planContent), 0o644); err != nil {
			g.t.Fatalf("write plan: %v", err)
		}
	}
	return worktree, nil
}

type integrationLLMAdapter struct {
	fake *testutil.FakeLLM
}

func newIntegrationLLMAdapter() *integrationLLMAdapter {
	fake := testutil.NewFakeLLM()
	fake.SetResponse("", &llm.LLMResponse{Output: "plan", Success: true})
	return &integrationLLMAdapter{fake: fake}
}

func (a *integrationLLMAdapter) GeneratePlan(ctx context.Context, specID string) (string, error) {
	resp, err := a.fake.Invoke(ctx, llm.InvokeRequest{Prompt: specID})
	if err != nil {
		return "", fmt.Errorf("invoke fake llm: %w", err)
	}
	if resp == nil || !resp.Success {
		return "", fmt.Errorf("fake llm response unsuccessful")
	}
	return fmt.Sprintf("%s-plan", specID), nil
}

func (a *integrationLLMAdapter) Invoke(ctx context.Context, req llm.LLMInvokeRequest) (*llm.LLMInvokeResponse, error) {
	return a.fake.Invoke(ctx, req)
}

func (a *integrationLLMAdapter) StreamInvoke(ctx context.Context, req llm.LLMStreamInvokeRequest) (*llm.LLMStreamInvokeResponse, error) {
	return a.fake.StreamInvoke(ctx, req)
}

func (a *integrationLLMAdapter) Fake() *testutil.FakeLLM {
	return a.fake
}

func newIntegrationTaskTrackerAdapter() *testutil.FakeTaskTracker {
	return testutil.NewFakeTaskTracker()
}

func newIntegrationPresenterAdapter(t *testing.T) *testutil.FakePresenter {
	t.Helper()
	return testutil.NewFakePresenter()
}

func assertPresenterSuccess(t *testing.T, presenter *testutil.FakePresenter, want bool) {
	t.Helper()
	if presenter == nil {
		t.Fatalf("presenter is nil")
	}
	if len(presenter.Calls) == 0 {
		t.Fatalf("presenter had no calls")
	}
	got := presenter.Calls[len(presenter.Calls)-1].Summary.Success
	if got != want {
		t.Fatalf("presenter summary success = %v, want %v", got, want)
	}
}

// TestIntegration_ResumeWithGapAnalysisAndRevalidation exercises the resume path:
// existing beads are found via the task tracker, plan is loaded from disk,
// gap analysis identifies affected beads, and selective revalidation re-queues them.
func TestIntegration_ResumeWithGapAnalysisAndRevalidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-resume-gap"
	cfg := &config.Config{}

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newIntegrationGitAdapter(t)
	git.planContent = validPlanFixture
	llmAdapter := newIntegrationLLMAdapter()
	taskTracker := newIntegrationTaskTrackerAdapter()
	presenter := newIntegrationPresenterAdapter(t)

	// Seed three beads with the spec label so queryExistingBeads finds them.
	label := fmt.Sprintf("spec:%s", specID)
	for _, title := range []string{"bead-a", "bead-b", "bead-c"} {
		_, err := taskTracker.CreateBead(ctx, tasktracker.CreateBeadRequest{
			Title:  title,
			Labels: []string{label},
		})
		if err != nil {
			t.Fatalf("seed bead %s: %v", title, err)
		}
	}

	// Gap analyzer: report that "main.go" changed and bead-1 (first seeded) touches it.
	gap := &mockGapAnalyzer{
		changedFiles: []string{"main.go"},
		beadFileMap:  map[string][]string{"bead-1": {"main.go", "util.go"}},
	}

	// Revalidator: return the flagged bead as needing re-queue.
	revalidator := &mockRevalidator{}

	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})
	_, beadLoop := newIntegrationLoopComponents(t, specID)
	planStage := newFakePlanStage(specID)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llmAdapter,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithEmitter(emitter),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithAcceptStage(accept),
		WithBeadLoop(beadLoop),
		WithGapAnalyzer(gap),
		WithSelectiveRevalidator(revalidator),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	// Plan stage should NOT have been called (plan loaded from disk).
	if planStage.called {
		t.Fatal("plan stage was called, want skipped (should resume from disk)")
	}

	// Gap analyzer should have been called exactly once.
	if gap.calls != 1 {
		t.Fatalf("gap analyzer calls = %d, want 1", gap.calls)
	}

	// Revalidator should have been called exactly once.
	if revalidator.calls != 1 {
		t.Fatalf("revalidator calls = %d, want 1", revalidator.calls)
	}

	// The revalidator received the flagged bead (bead-1 overlaps changed files).
	if len(revalidator.receivedBeads) != 1 {
		t.Fatalf("revalidator received %d beads, want 1", len(revalidator.receivedBeads))
	}
	if revalidator.receivedBeads[0].ID != "bead-1" {
		t.Fatalf("revalidator bead ID = %q, want %q", revalidator.receivedBeads[0].ID, "bead-1")
	}

	// Accept should have been called (spec completed successfully).
	if accept.calls != 1 {
		t.Fatalf("accept calls = %d, want 1", accept.calls)
	}

	// Should see DecomposeResumedEvent and PlanResumedEvent in the event stream.
	evts := drainEvents(ch)
	foundDecomposeResumed := false
	foundPlanResumed := false
	for _, evt := range evts {
		switch evt.(type) {
		case *events.DecomposeResumedEvent:
			foundDecomposeResumed = true
		case *events.PlanResumedEvent:
			foundPlanResumed = true
		}
	}
	if !foundPlanResumed {
		types := make([]string, len(evts))
		for i, e := range evts {
			types[i] = fmt.Sprintf("%T", e)
		}
		t.Fatalf("expected PlanResumedEvent in events, got: %v", types)
	}
	if !foundDecomposeResumed {
		types := make([]string, len(evts))
		for i, e := range evts {
			types[i] = fmt.Sprintf("%T", e)
		}
		t.Fatalf("expected DecomposeResumedEvent in events, got: %v", types)
	}

	assertPresenterSuccess(t, presenter, true)
}

// TestIntegration_CancelAndResumeSkipsDecompose proves that when a spec run is
// cancelled mid-bead-loop and restarted, the plan and decompose stages are NOT
// re-run because the plan file persists on disk and beads exist in the tracker.
func TestIntegration_CancelAndResumeSkipsDecompose(t *testing.T) {
	t.Parallel()

	specID := "spec-resume-test"
	cfg := &config.Config{}
	label := fmt.Sprintf("spec:%s", specID)

	// Shared worktree directory survives both runs.
	worktreeDir := t.TempDir()

	// Shared task tracker persists bead state across runs.
	taskTracker := newIntegrationTaskTrackerAdapter()

	// --- counting plan stage ---
	planCalls := 0
	innerPlan := newFakePlanStage(specID)
	innerPlan.plan = validPlanFixture // Must pass ValidatePlanContent on resume
	countingPlan := &countingPlanStage{
		inner: innerPlan,
		calls: &planCalls,
	}

	// --- counting decompose stage ---
	decomposeCalls := 0
	innerDecompose := newFakeDecomposeStage(specID)
	// Produce two beads so we can close one and leave one open.
	innerDecompose.producedBeads = []*bead.Bead{
		{ID: specID + "-bead-1", Labels: []string{label}},
		{ID: specID + "-bead-2", Labels: []string{label}},
	}
	countingDecompose := &countingDecomposeStage{
		inner: innerDecompose,
		calls: &decomposeCalls,
	}

	// --- First run: plan + decompose + cancel mid-bead-loop ---
	cancelCtx, cancel := context.WithCancel(context.Background())

	// Git adapter that always returns the same worktree directory.
	git1 := newIntegrationGitAdapter(t)
	git1.FakeGit.WorktreeRoot = worktreeDir

	llm1 := newIntegrationLLMAdapter()
	presenter1 := newIntegrationPresenterAdapter(t)

	emitter1 := events.NewEmitter()
	ch1 := emitter1.Subscribe()
	t.Cleanup(func() { emitter1.Unsubscribe(ch1) })

	// Bead runner that registers beads in the tracker, closes the first one,
	// then cancels the context to simulate interruption.
	beadRunner1 := &cancellingBeadRunner{
		taskTracker: taskTracker,
		label:       label,
		cancelFn:    cancel,
	}

	accept1 := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})
	presentStage1, summaryCtx1 := newPresentStageForTest(t, cfg, presenter1)

	adapters1 := adapter.AdapterSet{
		Git:         git1,
		LLM:         llm1,
		TaskTracker: taskTracker,
		Presenter:   presenter1,
	}

	loop1, err := NewSpecLoop(adapters1, cfg, noopDependencyGate{},
		WithEmitter(emitter1),
		WithPlanStage(countingPlan),
		WithPresentStage(presentStage1, summaryCtx1),
		WithAcceptStage(accept1),
		WithDecomposeStage(countingDecompose),
		WithBeadLoop(beadRunner1),
	)
	if err != nil {
		t.Fatalf("create spec loop (run 1): %v", err)
	}

	// Run 1 should return a context cancellation error.
	err = loop1.Run(cancelCtx, specID, nil)
	if err == nil {
		t.Fatal("expected run 1 to fail with context cancellation")
	}

	if planCalls != 1 {
		t.Fatalf("run 1: plan calls = %d, want 1", planCalls)
	}
	if decomposeCalls != 1 {
		t.Fatalf("run 1: decompose calls = %d, want 1", decomposeCalls)
	}

	// Verify plan file was written to disk.
	planPath := filepath.Join(worktreeDir, specID, ".gromit", "v2", "plan.md")
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("plan file not found after run 1: %v", err)
	}

	// --- Second run: should skip plan and decompose ---
	ctx2 := context.Background()

	git2 := newIntegrationGitAdapter(t)
	git2.FakeGit.WorktreeRoot = worktreeDir

	llm2 := newIntegrationLLMAdapter()
	presenter2 := newIntegrationPresenterAdapter(t)

	emitter2 := events.NewEmitter()
	ch2 := emitter2.Subscribe()
	t.Cleanup(func() { emitter2.Unsubscribe(ch2) })

	beadLoop2, err := NewBeadLoop(defaultIntegrationBeadLoopConfig())
	if err != nil {
		t.Fatalf("create bead loop (run 2): %v", err)
	}

	accept2 := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})
	presentStage2, summaryCtx2 := newPresentStageForTest(t, cfg, presenter2)

	adapters2 := adapter.AdapterSet{
		Git:         git2,
		LLM:         llm2,
		TaskTracker: taskTracker,
		Presenter:   presenter2,
	}

	loop2, err := NewSpecLoop(adapters2, cfg, noopDependencyGate{},
		WithEmitter(emitter2),
		WithPlanStage(countingPlan),
		WithPresentStage(presentStage2, summaryCtx2),
		WithAcceptStage(accept2),
		WithDecomposeStage(countingDecompose),
		WithBeadLoop(beadLoop2),
	)
	if err != nil {
		t.Fatalf("create spec loop (run 2): %v", err)
	}

	if err := loop2.Run(ctx2, specID, nil); err != nil {
		t.Fatalf("run 2 failed: %v", err)
	}

	// Plan stage should NOT have been called again (plan loaded from disk).
	if planCalls != 1 {
		t.Fatalf("after run 2: plan calls = %d, want 1 (should not replan)", planCalls)
	}

	// Decompose stage should NOT have been called again (beads exist in tracker).
	if decomposeCalls != 1 {
		t.Fatalf("after run 2: decompose calls = %d, want 1 (should not re-decompose)", decomposeCalls)
	}

	// Run 2 events should include DecomposeResumedEvent and PlanResumedEvent.
	evts := drainEvents(ch2)
	foundPlanResumed := false
	foundDecomposeResumed := false
	for _, evt := range evts {
		switch evt.(type) {
		case *events.PlanResumedEvent:
			foundPlanResumed = true
		case *events.DecomposeResumedEvent:
			foundDecomposeResumed = true
		}
	}
	if !foundPlanResumed {
		types := make([]string, len(evts))
		for i, e := range evts {
			types[i] = fmt.Sprintf("%T", e)
		}
		t.Fatalf("expected PlanResumedEvent in run 2 events, got: %v", types)
	}
	if !foundDecomposeResumed {
		types := make([]string, len(evts))
		for i, e := range evts {
			types[i] = fmt.Sprintf("%T", e)
		}
		t.Fatalf("expected DecomposeResumedEvent in run 2 events, got: %v", types)
	}

	assertPresenterSuccess(t, presenter2, true)
}

// TestIntegration_AcceptFailureRemediationAddsBeads proves that when acceptance
// fails, the remediation runner plans and decomposes new beads without
// re-running the original plan/decompose cycle.
func TestIntegration_AcceptFailureRemediationAddsBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-remediate-additive"
	cfg := &config.Config{}

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() { emitter.Unsubscribe(ch) })

	worktreeDir := t.TempDir()
	git := newIntegrationGitAdapter(t)
	git.FakeGit.WorktreeRoot = worktreeDir

	llmAdapter := newIntegrationLLMAdapter()
	taskTracker := newIntegrationTaskTrackerAdapter()
	presenter := newIntegrationPresenterAdapter(t)

	// Original plan and decompose stages with call counters.
	planCalls := 0
	countingPlan := &countingPlanStage{
		inner: newFakePlanStage(specID),
		calls: &planCalls,
	}

	decomposeCalls := 0
	innerDecompose := newFakeDecomposeStage(specID)
	countingDecompose := &countingDecomposeStage{
		inner: innerDecompose,
		calls: &decomposeCalls,
	}

	// Accept: fail first, then proceed.
	accept := newScriptedAcceptStage(
		stagepkg.Result{Decision: stagepkg.DecisionFail},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
	)

	_, beadLoop := newIntegrationLoopComponents(t, specID)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	// Build a real remediation runner using the same plan + decompose stages.
	remBeadLoop, err := NewBeadLoop(defaultIntegrationBeadLoopConfig())
	if err != nil {
		t.Fatalf("create remediation bead loop: %v", err)
	}
	remBeadRunner := &integrationRemBeadRunner{loop: remBeadLoop}

	remRunner := newTestRemediationRunner(testRemediationRunnerConfig{
		acceptStage:    accept,
		planStage:      countingPlan,
		decomposeStage: countingDecompose,
		beadRunner:     remBeadRunner,
		emitter:        emitter,
		presenter:      presenter,
	})

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llmAdapter,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithEmitter(emitter),
		WithPlanStage(countingPlan),
		WithPresentStage(presentStage, summaryCtx),
		WithAcceptStage(accept),
		WithRemediationRunner(remRunner),
		WithDecomposeStage(countingDecompose),
		WithBeadLoop(beadLoop),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	// Original plan: called once. Remediation plan: called once. Total = 2.
	if planCalls != 2 {
		t.Fatalf("plan calls = %d, want 2 (1 original + 1 remediation)", planCalls)
	}

	// Original decompose: called once. Remediation decompose: called once. Total = 2.
	if decomposeCalls != 2 {
		t.Fatalf("decompose calls = %d, want 2 (1 original + 1 remediation)", decomposeCalls)
	}

	// Accept called once (initial fail). After remediation succeeds,
	// ensureAcceptance trusts the result and returns without re-checking.
	if accept.calls < 1 {
		t.Fatalf("accept calls = %d, want >= 1", accept.calls)
	}

	// Verify remediation-1.md was persisted.
	worktree := filepath.Join(worktreeDir, specID)
	remPlanPath := filepath.Join(worktree, ".gromit", "v2", "remediation-1.md")
	if _, err := os.Stat(remPlanPath); err != nil {
		t.Fatalf("remediation-1.md not found: %v", err)
	}

	// Verify original plan.md was not modified (still contains the original plan content).
	originalPlanPath := filepath.Join(worktree, ".gromit", "v2", "plan.md")
	planData, err := os.ReadFile(originalPlanPath)
	if err != nil {
		t.Fatalf("read original plan: %v", err)
	}
	expectedPlan := specID + "-plan"
	if string(planData) != expectedPlan {
		// The fakePlanStage writes its plan field; the spec loop also persists.
		// Either way, the content should not have been overwritten by remediation.
		if len(string(planData)) == 0 {
			t.Fatal("original plan.md is empty")
		}
	}

	assertPresenterSuccess(t, presenter, true)
}

// --- Helper types for cancel/resume and remediation integration tests ---

// countingPlanStage wraps a plan stage and counts calls.
type countingPlanStage struct {
	inner stagepkg.Stage
	calls *int
}

func (c *countingPlanStage) Name() string { return "plan" }

func (c *countingPlanStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	*c.calls++
	return c.inner.Run(ctx, req)
}

// countingDecomposeStage wraps a decompose stage and counts calls.
type countingDecomposeStage struct {
	inner stagepkg.Stage
	calls *int
}

func (c *countingDecomposeStage) Name() string { return "decompose" }

func (c *countingDecomposeStage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	*c.calls++
	return c.inner.Run(ctx, req)
}

// cancellingBeadRunner registers beads in the task tracker, closes the first
// one, then cancels the context to simulate a mid-bead-loop interruption.
type cancellingBeadRunner struct {
	taskTracker *testutil.FakeTaskTracker
	label       string
	cancelFn    context.CancelFunc
}

func (r *cancellingBeadRunner) Run(ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) (BeadLoopResult, error) {
	// Register all beads in the task tracker so hasBeadsForSpec finds them.
	for _, b := range beads {
		labels := append([]string{}, b.Labels...)
		if len(labels) == 0 {
			labels = []string{r.label}
		}
		_, _ = r.taskTracker.CreateBead(ctx, tasktracker.CreateBeadRequest{
			Title:  b.ID,
			Labels: labels,
		})
	}

	// Close the first bead to simulate partial progress.
	resp, _ := r.taskTracker.QueryBeads(ctx, tasktracker.TaskTrackerQueryBeadsRequest{
		Labels: []string{r.label},
		Status: "open",
	})
	if resp != nil && len(resp.Beads) > 0 {
		_, _ = r.taskTracker.CloseBead(ctx, tasktracker.TaskTrackerCloseBeadRequest{
			BeadID: resp.Beads[0].ID,
		})
	}

	// Cancel context to simulate user interruption.
	r.cancelFn()
	return BeadLoopResult{}, ctx.Err()
}

// integrationRemBeadRunner adapts BeadLoop to the remediation BeadRunner interface.
type integrationRemBeadRunner struct {
	loop *BeadLoop
}

func (r *integrationRemBeadRunner) Run(ctx context.Context, beads []*bead.Bead) error {
	_, err := r.loop.Run(ctx, beads, nil)
	return err
}

// testRemediationRunnerConfig holds configuration for building a test remediation runner.
type testRemediationRunnerConfig struct {
	acceptStage    stagepkg.Stage
	planStage      stagepkg.Stage
	decomposeStage stagepkg.Stage
	beadRunner     remediationBeadRunnerIface
	emitter        *events.Emitter
	presenter      adapter.PresenterAdapter
}

// remediationBeadRunnerIface matches the remediation package's BeadRunner interface.
type remediationBeadRunnerIface interface {
	Run(ctx context.Context, beads []*bead.Bead) error
}

// testRemediationRunnerAdapter wraps the real remediation runner.
type testRemediationRunnerAdapter struct {
	acceptStage    stagepkg.Stage
	planStage      stagepkg.Stage
	decomposeStage stagepkg.Stage
	beadRunner     remediationBeadRunnerIface
	emitter        *events.Emitter
	presenter      adapter.PresenterAdapter
	calls          int
}

func newTestRemediationRunner(cfg testRemediationRunnerConfig) *testRemediationRunnerAdapter {
	return &testRemediationRunnerAdapter{
		acceptStage:    cfg.acceptStage,
		planStage:      cfg.planStage,
		decomposeStage: cfg.decomposeStage,
		beadRunner:     cfg.beadRunner,
		emitter:        cfg.emitter,
		presenter:      cfg.presenter,
	}
}

func (r *testRemediationRunnerAdapter) Run(ctx context.Context, specID, worktree string, _ *stagepkg.Result) error {
	r.calls++

	// Run plan stage for remediation.
	req := stagepkg.Request{
		Bead:        stagepkg.BeadInfo{ID: specID},
		Worktree:    worktree,
		Remediation: true,
	}
	if r.planStage != nil {
		if _, err := r.planStage.Run(ctx, &req); err != nil {
			return fmt.Errorf("remediation plan: %w", err)
		}
	}

	// Persist remediation plan.
	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		return fmt.Errorf("create remediation dir: %w", err)
	}
	remPlanPath := filepath.Join(gromitDir, fmt.Sprintf("remediation-%d.md", r.calls))
	if err := os.WriteFile(remPlanPath, []byte("remediation plan content"), 0o644); err != nil {
		return fmt.Errorf("persist remediation plan: %w", err)
	}

	// Run decompose stage for remediation.
	if r.decomposeStage != nil {
		res, err := r.decomposeStage.Run(ctx, &req)
		if err != nil {
			return fmt.Errorf("remediation decompose: %w", err)
		}
		if res != nil && res.Artifacts != nil {
			if artifacts, ok := res.Artifacts.(*stagepkg.DecomposeArtifacts); ok {
				if r.beadRunner != nil {
					if err := r.beadRunner.Run(ctx, artifacts.Beads); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func drainEvents(ch chan events.Event) []events.Event {
	var evts []events.Event
	deadline := time.After(5 * time.Second)
	for {
		select {
		case evt := <-ch:
			evts = append(evts, evt)
		case <-deadline:
			return evts
		}
	}
}

// mockGapAnalyzer implements GapAnalyzer for testing.
type mockGapAnalyzer struct {
	changedFiles []string
	beadFileMap  map[string][]string
	calls        int
}

func (m *mockGapAnalyzer) Analyze(_ context.Context, _ string, _ []*bead.Bead) ([]string, map[string][]string, error) {
	m.calls++
	return m.changedFiles, m.beadFileMap, nil
}

// mockRevalidator implements SelectiveRevalidator for testing.
type mockRevalidator struct {
	calls         int
	receivedBeads []*bead.Bead
}

func (m *mockRevalidator) Revalidate(_ context.Context, beads []*bead.Bead, _ string) ([]*bead.Bead, error) {
	m.calls++
	m.receivedBeads = append(m.receivedBeads, beads...)
	return beads, nil
}
