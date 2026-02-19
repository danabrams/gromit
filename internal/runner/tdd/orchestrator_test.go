package tdd

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func newTestOrchestrator() *CycleOrchestrator {
	return &CycleOrchestrator{
		cfg: &config.Config{
			Methodology: config.MethodologyConfig{
				MaxTDDCycles: 10,
			},
		},
		readFileFn: func(path string) (string, error) {
			return "", nil
		},
		getDiffFn: func() (string, error) {
			return "", nil
		},
	}
}

func singleRequirementState() CycleState {
	return CycleState{
		CycleNumber: 0,
		MaxCycles:   10,
		Remaining:   []string{"implement feature X"},
	}
}

func TestRunCycles_ReturnsNilWhenStateAlreadyComplete(t *testing.T) {
	orch := &CycleOrchestrator{
		cfg: &config.Config{
			Methodology: config.MethodologyConfig{
				MaxTDDCycles: 10,
			},
		},
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
	}

	// State is already complete when Done=true
	state := CycleState{
		CycleNumber: 1,
		MaxCycles:   10,
		Done:        true,
	}

	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("expected nil error for complete state, got %v", err)
	}
}

func TestRunCycles_SingleCycle_CallsRedPhase(t *testing.T) {
	orch := newTestOrchestrator()

	var redPrompt string
	var invokedPrompts []string
	var invokedTiers []string

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		redPrompt = "red-prompt"
		return redPrompt, nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		invokedPrompts = append(invokedPrompts, prompt)
		invokedTiers = append(invokedTiers, tier)
		return nil
	}
	// Validation after red: tests fail (expected)
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		return "FAIL: TestFoo", false, nil
	}
	// Green phase
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		return "green-prompt", nil
	}
	// Refactor phase (no-op)
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		return nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}

	state := singleRequirementState()
	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(invokedPrompts) == 0 {
		t.Fatal("expected at least one invocation")
	}
	if invokedPrompts[0] != "red-prompt" {
		t.Fatalf("expected first invocation prompt to be red-prompt, got %q", invokedPrompts[0])
	}
	if invokedTiers[0] != "medium" {
		t.Fatalf("expected first invocation tier to be medium, got %q", invokedTiers[0])
	}
}
