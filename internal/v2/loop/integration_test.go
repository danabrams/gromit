package loop

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/generation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

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

	remediation := newIntegrationRemediationRunner(t, emitter, integrationRemediationConfig{
		stages: []StageSpec{
			{
				Stage: &failingStage{name: "remediation", failUntil: 999},
				Retry: stagepkg.RetryConfig{MaxRetries: 0},
			},
		},
		generationCap: 0,
		labels:        []string{generation.Format(42)},
	})

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{}, WithEmitter(emitter), WithAcceptStage(accept), WithRemediationRunner(remediation))
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err == nil {
		t.Fatal("expected Run to return an error")
	}

	assertEventTypeOrder(t, collectEvents(t, ch, 4), []string{
		"*events.SpecStartedEvent",
		"*events.GenerationCapReachedEvent",
		"*events.AndonTriggeredEvent",
		"*events.SpecFailedEvent",
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

	remediation := newIntegrationRemediationRunner(t, emitter, integrationRemediationConfig{
		stages: []StageSpec{
			{Stage: &successStage{name: "bead-1"}, Retry: stagepkg.RetryConfig{MaxRetries: 0}},
			{Stage: &successStage{name: "bead-2"}, Retry: stagepkg.RetryConfig{MaxRetries: 0}},
			{Stage: &successStage{name: "bead-3"}, Retry: stagepkg.RetryConfig{MaxRetries: 0}},
		},
		generationCap: -1,
	})

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{}, WithEmitter(emitter), WithAcceptStage(accept), WithRemediationRunner(remediation))
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
	stages        []StageSpec
	generationCap int
	labels        []string
}

type integrationRemediationRunner struct {
	beadLoop *BeadLoop
	labels   []string
	calls    int
}

func newIntegrationRemediationRunner(t *testing.T, emitter *events.Emitter, cfg integrationRemediationConfig) remediationRunner {
	t.Helper()
	beadLoop, err := NewBeadLoop(cfg.stages)
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}
	if cfg.generationCap >= 0 {
		beadLoop.GenerationCap = cfg.generationCap
	}
	beadLoop.Emitter = emitter
	labels := append([]string(nil), cfg.labels...)
	return &integrationRemediationRunner{
		beadLoop: beadLoop,
		labels:   labels,
	}
}

func (r *integrationRemediationRunner) Run(ctx context.Context, specID string) error {
	r.calls++
	req := stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:     specID,
			Labels: append([]string(nil), r.labels...),
		},
	}
	_, err := r.beadLoop.Run(ctx, req)
	return err
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
