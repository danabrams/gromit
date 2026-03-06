package loop

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/generation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

// TestIntegration_SpecLoopHappyPathCompletes exercises a clean run where multiple beads\n+// (represented by the successive stages in the spec loop) all succeed, acceptance returns\n+// proceed immediately, and the summary is presented once.
func TestIntegration_SpecLoopHappyPathCompletes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-happy"

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newFakeGitAdapter(t)
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})
	decompose, beadLoop := newIntegrationLoopComponents(t, specID)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithEmitter(emitter),
		WithAcceptStage(accept),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadLoop),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if accept.calls != 1 {
		t.Fatalf("accept calls = %d, want 1", accept.calls)
	}

	requireHappyPathEvents(t, ch)

	if !presenter.lastSummary.Success {
		t.Fatalf("presenter summary success = %v, want true", presenter.lastSummary.Success)
	}
}

func TestIntegration_SpecLoopFailureHitsGenerationCap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-failure-gen-cap"

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newFakeGitAdapter(t)
	git.gapAnalysisContent = "missing gap analysis"
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})
	decompose, beadLoop := newIntegrationLoopComponents(t, specID)

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

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithEmitter(emitter),
		WithAcceptStage(accept),
		WithRemediationRunner(remediation),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadLoop),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err == nil {
		t.Fatal("expected Run to return an error")
	}

	assertEventTypeOrder(t, collectEvents(t, ch, 5), []string{
		"*events.SpecStartedEvent",
		"*events.GenerationCapReachedEvent",
		"*events.AndonTriggeredEvent",
		"*events.SpecFailedEvent",
		"*events.SpecCompletedEvent",
	})

	if presenter.lastSummary.Success {
		t.Fatalf("presenter summary success = %v, want false", presenter.lastSummary.Success)
	}
}

// TestIntegration_SpecLoopRemediationAppliesGapAnalysis ensures the remediation loop runs after the first\n+// failed accept and that 2-3 remediation beads complete before acceptance succeeds.
func TestIntegration_SpecLoopRemediationAppliesGapAnalysis(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-remediate"

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newFakeGitAdapter(t)
	git.gapAnalysisContent = "gap analysis leads to remediation"
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	accept := newScriptedAcceptStage(
		stagepkg.Result{Decision: stagepkg.DecisionFail},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
	)
	decompose, beadLoop := newIntegrationLoopComponents(t, specID)

	remediation := newIntegrationRemediationRunner(t, emitter, integrationRemediationConfig{
		generationCap: -1,
	})

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithEmitter(emitter),
		WithAcceptStage(accept),
		WithRemediationRunner(remediation),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadLoop),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err != nil {
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

	if !presenter.lastSummary.Success {
		t.Fatalf("presenter summary success = %v, want true", presenter.lastSummary.Success)
	}

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
	return r.beadLoop.Run(ctx, beads)
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
