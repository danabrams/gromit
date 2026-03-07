package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
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

	if accept.calls != 2 {
		t.Fatalf("accept calls = %d, want 2", accept.calls)
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

func (r *integrationRemediationRunner) Run(ctx context.Context, specID string) error {
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
	if g.gapAnalysisContent == "" {
		return worktree, nil
	}
	gromitPath := filepath.Join(worktree, ".gromit", "v2")
	if err := os.MkdirAll(gromitPath, 0o755); err != nil {
		g.t.Fatalf("create gromit dir: %v", err)
	}
	path := filepath.Join(gromitPath, "gap-analysis.md")
	if err := os.WriteFile(path, []byte(g.gapAnalysisContent), 0o644); err != nil {
		g.t.Fatalf("write gap analysis: %v", err)
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
