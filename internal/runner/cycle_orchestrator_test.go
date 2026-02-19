package runner

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestRunMethodologyExecution_DelegatesToCycleOrchestratorWhenFreshContextEnabled(t *testing.T) {
	expected := &IterationResult{Success: true}
	orchestratorCalled := false

	r := &Runner{
		cfg: &config.Config{
			Methodology: config.MethodologyConfig{
				FreshContextPerCycle: true,
			},
		},
		cycleOrchestrator: &cycleOrchestrator{
			executeFn: func(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, executeWithRetry func() bool) *IterationResult {
				orchestratorCalled = true
				return expected
			},
		},
	}

	got := r.runMethodologyExecution(
		context.Background(),
		&runtypes.BeadContext{Result: &runtypes.IterationResult{}},
		false,
		true,
		func() bool {
			t.Fatal("legacy executeWithRetry path should not be called when cycle orchestrator is active")
			return false
		},
	)

	if !orchestratorCalled {
		t.Fatal("expected cycle orchestrator to be called")
	}
	if got != expected {
		t.Fatalf("expected delegated result %p, got %p", expected, got)
	}
}

func TestRunMethodologyExecution_UsesLegacyPathWhenFreshContextDisabled(t *testing.T) {
	orchestratorCalled := false
	executeCalled := false

	r := &Runner{
		cfg: &config.Config{
			Methodology: config.MethodologyConfig{
				FreshContextPerCycle: false,
			},
		},
		cycleOrchestrator: &cycleOrchestrator{
			executeFn: func(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, executeWithRetry func() bool) *IterationResult {
				orchestratorCalled = true
				return &IterationResult{}
			},
		},
	}

	result := r.runMethodologyExecution(
		context.Background(),
		&runtypes.BeadContext{Result: &runtypes.IterationResult{}},
		false,
		true,
		func() bool {
			executeCalled = true
			return false
		},
	)

	if orchestratorCalled {
		t.Fatal("did not expect cycle orchestrator to be called when fresh_context_per_cycle is false")
	}
	if !executeCalled {
		t.Fatal("expected legacy executeWithRetry path to run when fresh_context_per_cycle is false")
	}
	if result == nil {
		t.Fatal("expected non-nil iteration result")
	}
}
