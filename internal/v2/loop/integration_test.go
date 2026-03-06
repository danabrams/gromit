package loop

import (
    "context"
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
        generationCap:  0,
        generationLabel: generation.Format(42),
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
