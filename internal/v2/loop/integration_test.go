package loop

import (
	"bytes"
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
	"github.com/danabrams/gromit/internal/v2/event"
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

func (r *integrationRemediationRunner) Run(ctx context.Context, specID, _ string) error {
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
	lastWorktree       string
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
	g.lastWorktree = worktree
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
	git.planContent = "existing plan from prior run"
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

func TestIntegration_FileSubscriberPreservesLegacySubscriberFlow(t *testing.T) {
	t.Parallel()

	const subscriptionDelay = 1200 * time.Millisecond
	const warmupTimeout = 5 * time.Second
	ctx := context.Background()
	specID := "spec-file-subscriber-flow"
	cfg := &config.Config{}

	typedEmitter := event.NewEmitter()
	defer typedEmitter.Close()

	legacyEmitter := events.NewEmitter()
	legacyResult := make(chan error, 1)
	legacyDone := make(chan struct{})
	go func() {
		defer close(legacyDone)
		time.Sleep(subscriptionDelay)
		sub := legacyEmitter.Subscribe()
		defer legacyEmitter.Unsubscribe(sub)

		_, err := waitForLegacyEventType[*events.SpecStartedEvent](sub, warmupTimeout)
		legacyResult <- err
	}()
	defer func() {
		legacyEmitter.Close()
		<-legacyDone
	}()

	git := newIntegrationGitAdapter(t)

	planStage := newFakePlanStage(specID)
	presenter := newIntegrationPresenterAdapter(t)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})

	beadLoop, err := NewBeadLoop(BeadLoopConfig{
		Gate:          newNoopStage("gate"),
		Build:         newNoopStage("build"),
		Validate:      newNoopStage("validate"),
		Review:        newNoopStage("review"),
		Epilogue:      newNoopStage("epilogue"),
		Emitter:       typedEmitter,
		LegacyEmitter: legacyEmitter,
	})
	if err != nil {
		t.Fatalf("create bead loop: %v", err)
	}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newIntegrationLLMAdapter(),
		TaskTracker: newIntegrationTaskTrackerAdapter(),
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithEmitter(legacyEmitter),
		WithLegacySubscriberWarmup(),
		WithTypedEmitter(typedEmitter),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(beadLoop),
		WithAcceptStage(accept),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if err := <-legacyResult; err != nil {
		t.Fatalf("warmup event did not reach legacy subscribers: %v", err)
	}

	eventsPath := filepath.Join(git.lastWorktree, ".gromit", "v2", "events.jsonl")
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events file: %v", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		t.Fatalf("events file is empty")
	}
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

func waitForLegacyEventType[T events.Event](ch <-chan events.Event, timeout time.Duration) (T, error) {
	var zero T
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return zero, fmt.Errorf("legacy event channel closed")
			}
			if typed, ok := evt.(T); ok {
				return typed, nil
			}
		case <-deadline:
			return zero, context.DeadlineExceeded
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
