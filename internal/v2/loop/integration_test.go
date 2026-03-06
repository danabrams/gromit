package loop

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

// TestIntegration_HappyPathMultipleBeadsAcceptAndPresent exercises the full spec loop
// with multiple beads that all pass and accept succeeds.
func TestIntegration_HappyPathMultipleBeadsAcceptAndPresent(t *testing.T) {
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
	beadLoopRunner := newIntegrationBeadLoopRunner()

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{}, WithEmitter(emitter), WithAcceptStage(accept), WithRemediationRunner(beadLoopRunner))
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	// Verify accept was called exactly once
	if accept.calls != 1 {
		t.Fatalf("accept calls = %d, want 1", accept.calls)
	}

	// Verify presenter was called
	if !presenter.lastSummary.Success {
		t.Fatalf("presenter summary success = %v, want true", presenter.lastSummary.Success)
	}

	// Verify event ordering
	received := collectEvents(t, ch, 2)
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}

	started, ok := received[0].(*events.SpecStartedEvent)
	if !ok {
		t.Fatalf("first event = %T, want *events.SpecStartedEvent", received[0])
	}
	if started.SpecID != specID {
		t.Fatalf("spec started event spec = %s, want %s", started.SpecID, specID)
	}

	completed, ok := received[1].(*events.SpecCompletedEvent)
	if !ok {
		t.Fatalf("second event = %T, want *events.SpecCompletedEvent", received[1])
	}
	if completed.SpecID != specID || !completed.Success {
		t.Fatalf("unexpected spec completed event = %#v", completed)
	}
}

// TestIntegration_RemediationPathWithBeadLoop exercises the remediation path where
// Accept fails initially, then remediation produces a bead loop that succeeds,
// and then Accept passes on second invocation.
func TestIntegration_RemediationPathWithBeadLoop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-remediate"

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newFakeGitAdapter(t)
	git.gapAnalysisContent = "gap analysis content from failed accept"
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	// First accept fails, second succeeds
	accept := newScriptedAcceptStage(
		stagepkg.Result{Decision: stagepkg.DecisionFail},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
	)
	// Remediation runner succeeds
	beadLoopRunner := newIntegrationBeadLoopRunner()

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{}, WithEmitter(emitter), WithAcceptStage(accept), WithRemediationRunner(beadLoopRunner))
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	// Verify accept was called twice (fails first, succeeds on remediation)
	if accept.calls != 2 {
		t.Fatalf("accept calls = %d, want 2", accept.calls)
	}

	// Verify bead loop runner (remediation) was called once
	if beadLoopRunner.calls != 1 {
		t.Fatalf("bead loop runs = %d, want 1", beadLoopRunner.calls)
	}

	// Verify presenter was called and shows success
	if !presenter.lastSummary.Success {
		t.Fatalf("presenter summary success = %v, want true", presenter.lastSummary.Success)
	}

	// Verify event stream: SpecStarted, SpecCompleted
	received := collectEvents(t, ch, 2)
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}

	started, ok := received[0].(*events.SpecStartedEvent)
	if !ok {
		t.Fatalf("first event = %T, want *events.SpecStartedEvent", received[0])
	}
	if started.SpecID != specID {
		t.Fatalf("spec started event spec = %s, want %s", started.SpecID, specID)
	}

	completed, ok := received[1].(*events.SpecCompletedEvent)
	if !ok {
		t.Fatalf("second event = %T, want *events.SpecCompletedEvent", received[1])
	}
	if completed.SpecID != specID || !completed.Success {
		t.Fatalf("unexpected spec completed event = %#v", completed)
	}
}

// TestIntegration_FailurePathWithGenerationCapAndAndon exercises the failure path where
// the generation cap is hit, Andon is emitted, and failure is presented.
func TestIntegration_FailurePathWithGenerationCapAndAndon(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-failure"

	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	git := newFakeGitAdapter(t)
	git.gapAnalysisContent = "gap analysis for remediation"
	llm := newFakeLLMAdapter()
	taskTracker := newFakeTaskTrackerAdapter()
	presenter := newFakePresenterAdapter(t)
	// Accept fails
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})
	// Remediation runner fails with generation cap error
	beadLoopRunner := &fakeIntegrationBeadLoopRunner{failWithErr: "generation cap reached"}

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llm,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{}, WithEmitter(emitter), WithAcceptStage(accept), WithRemediationRunner(beadLoopRunner))
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	// Run should return error
	if err := loopInstance.Run(ctx, specID); err == nil {
		t.Fatal("expected Run to return an error")
	}

	// Verify accept was called once
	if accept.calls != 1 {
		t.Fatalf("accept calls = %d, want 1", accept.calls)
	}

	// Verify presenter was called and shows failure
	if presenter.lastSummary.Success {
		t.Fatalf("presenter summary success = %v, want false", presenter.lastSummary.Success)
	}

	// Verify events include SpecStarted, AndonTriggered, SpecFailed
	received := collectEvents(t, ch, 3)
	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}

	started, ok := received[0].(*events.SpecStartedEvent)
	if !ok {
		t.Fatalf("first event = %T, want *events.SpecStartedEvent", received[0])
	}
	if started.SpecID != specID {
		t.Fatalf("spec started event spec = %s, want %s", started.SpecID, specID)
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

// fakeIntegrationBeadLoopRunner simulates a bead loop that can succeed or fail.
type fakeIntegrationBeadLoopRunner struct {
	calls      int
	failWithErr string
}

func newIntegrationBeadLoopRunner() *fakeIntegrationBeadLoopRunner {
	return &fakeIntegrationBeadLoopRunner{}
}

func (f *fakeIntegrationBeadLoopRunner) Run(_ context.Context, _ string) error {
	f.calls++
	if f.failWithErr != "" {
		return errorWithMessage(f.failWithErr)
	}
	return nil
}

func errorWithMessage(msg string) error {
	return fmt.Errorf("%s", msg)
}
