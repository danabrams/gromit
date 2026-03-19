package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	present "github.com/danabrams/gromit/internal/v2/stage/present"
)

func TestRunWithEmptySpecID(t *testing.T) {
	t.Parallel()

	adapters := adapter.AdapterSet{
		Git:         newFakeGitAdapter(t),
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithPlanStage(newFakePlanStage("x")),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage("x")),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty spec ID")
	}
	if !strings.Contains(err.Error(), "spec ID required") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "spec ID required")
	}
}

func TestRunWhenGitCheckoutFails(t *testing.T) {
	t.Parallel()

	checkoutErr := fmt.Errorf("branch not found")
	gitAdapter := &errorGitAdapter{checkoutErr: checkoutErr}

	adapters := adapter.AdapterSet{
		Git:         gitAdapter,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithPlanStage(newFakePlanStage("spec-checkout-fail")),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage("spec-checkout-fail")),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(context.Background(), "spec-checkout-fail", nil)
	if err == nil {
		t.Fatal("expected error when checkout fails")
	}
	if !errors.Is(err, checkoutErr) {
		t.Fatalf("error = %v, want wrapped %v", err, checkoutErr)
	}
	if !strings.Contains(err.Error(), "checkout") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "checkout")
	}
}

func TestRunWhenPlanStageIsNil(t *testing.T) {
	t.Parallel()

	adapters := adapter.AdapterSet{
		Git:         newFakeGitAdapter(t),
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		// No plan stage configured.
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage("spec-no-plan")),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(context.Background(), "spec-no-plan", nil)
	if err == nil {
		t.Fatal("expected error when plan stage is nil")
	}
	if !strings.Contains(err.Error(), "plan stage") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "plan stage")
	}
}

func TestRunWhenBeadRunnerIsNil(t *testing.T) {
	t.Parallel()

	adapters := adapter.AdapterSet{
		Git:         newFakeGitAdapter(t),
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithPlanStage(newFakePlanStage("spec-no-bead")),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage("spec-no-bead")),
		// No bead runner configured.
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(context.Background(), "spec-no-bead", nil)
	if err == nil {
		t.Fatal("expected error when bead runner is nil")
	}
	if !strings.Contains(err.Error(), "bead runner required") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "bead runner required")
	}
}

func TestRunWhenDependencyGateReturnsError(t *testing.T) {
	t.Parallel()

	gateErr := fmt.Errorf("upstream spec not complete")
	gate := errorDependencyGate{err: gateErr}

	adapters := adapter.AdapterSet{
		Git:         newFakeGitAdapter(t),
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, gate,
		WithPlanStage(newFakePlanStage("spec-gate-fail")),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage("spec-gate-fail")),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(context.Background(), "spec-gate-fail", nil)
	if err == nil {
		t.Fatal("expected error when dependency gate fails")
	}
	if !errors.Is(err, gateErr) {
		t.Fatalf("error = %v, want wrapped %v", err, gateErr)
	}
	if !strings.Contains(err.Error(), "dependency gate") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "dependency gate")
	}
}

// --- test helpers ---

// errorGitAdapter is a git adapter that returns an error on Checkout.
type errorGitAdapter struct {
	fakeGitAdapter
	checkoutErr error
}

func (e *errorGitAdapter) Checkout(_ context.Context, _ string) (string, error) {
	return "", e.checkoutErr
}

// errorDependencyGate always returns the configured error.
type errorDependencyGate struct {
	err error
}

func (e errorDependencyGate) EnsureSpecReady(_ context.Context, _ string) error {
	return e.err
}
