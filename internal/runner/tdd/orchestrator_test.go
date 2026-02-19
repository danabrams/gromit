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
	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		if validateCall == 1 {
			return "FAIL: TestFoo", false, nil // Red: tests fail
		}
		return "PASS", true, nil // Green/refactor: tests pass
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		return "green-prompt", nil
	}
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

func TestRunCycles_SingleCycle_CorrectCallSequence(t *testing.T) {
	orch := newTestOrchestrator()

	var phases []string
	validateCall := 0

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		phases = append(phases, "renderRed")
		return "red-prompt", nil
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		phases = append(phases, "renderGreen")
		return "green-prompt", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		switch prompt {
		case "red-prompt":
			phases = append(phases, "invokeRed")
		case "green-prompt":
			phases = append(phases, "invokeGreen")
		}
		return nil
	}
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			phases = append(phases, "validateRed")
			return "FAIL: TestFoo", false, nil // Expected: tests fail after red
		case 2:
			phases = append(phases, "validateGreen")
			return "PASS", true, nil // Expected: tests pass after green
		case 3:
			phases = append(phases, "validateRefactor")
			return "PASS", true, nil // Expected: tests still pass after refactor
		}
		return "", false, nil
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		phases = append(phases, "refactor")
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

	expected := []string{
		"renderRed", "invokeRed", "validateRed",
		"renderGreen", "invokeGreen", "validateGreen",
		"refactor", "validateRefactor",
	}

	if len(phases) != len(expected) {
		t.Fatalf("expected %d phases, got %d: %v", len(expected), len(phases), phases)
	}
	for i, want := range expected {
		if phases[i] != want {
			t.Fatalf("phase[%d]: expected %q, got %q (full: %v)", i, want, phases[i], phases)
		}
	}
}

func TestRunCycles_MultipleCycles_LoopsThroughRequirements(t *testing.T) {
	orch := newTestOrchestrator()

	cycleCount := 0

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		return "green", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		return nil
	}

	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		// Each cycle: red-validate=fail, green-validate=pass, refactor-validate=pass
		switch validateCall % 3 {
		case 1:
			return "FAIL", false, nil
		default:
			return "PASS", true, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		cycleCount++
		return nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}

	state := CycleState{
		CycleNumber: 0,
		MaxCycles:   10,
		Remaining:   []string{"req A", "req B", "req C"},
	}

	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cycleCount != 3 {
		t.Fatalf("expected 3 cycles (one per requirement), got %d", cycleCount)
	}
}

func TestRunCycles_TerminatesAtMaxCycles(t *testing.T) {
	orch := newTestOrchestrator()

	cycleCount := 0

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		return "green", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		return nil
	}
	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall % 3 {
		case 1:
			return "FAIL", false, nil
		default:
			return "PASS", true, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		cycleCount++
		return nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}

	// MaxCycles=2 but 5 requirements: should stop after 2 cycles
	state := CycleState{
		CycleNumber: 0,
		MaxCycles:   2,
		Remaining:   []string{"req A", "req B", "req C", "req D", "req E"},
	}

	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cycleCount != 2 {
		t.Fatalf("expected 2 cycles (max), got %d", cycleCount)
	}
}

func TestRunCycles_CoverageDone_TestsPassUnexpectedlyInRedPhase(t *testing.T) {
	orch := newTestOrchestrator()

	greenCalled := false

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		return nil
	}
	// Red validation: tests pass unexpectedly
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		return "PASS", true, nil
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		greenCalled = true
		return "green", nil
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		return nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}

	state := CycleState{
		CycleNumber: 0,
		MaxCycles:   10,
		Remaining:   []string{"req A", "req B"},
	}

	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if greenCalled {
		t.Fatal("green phase should not be called when red validation passes unexpectedly")
	}
}
