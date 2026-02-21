package tdd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
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
		case 4:
			phases = append(phases, "validateFinal")
			return "PASS", true, nil // Expected: final validation still passes.
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
		"refactor", "validateRefactor", "validateFinal",
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
		if prompt == "red" {
			cycleCount++
		}
		return nil
	}

	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		// Each cycle: red-validate=fail, green-validate=pass
		switch validateCall % 2 {
		case 1:
			return "FAIL", false, nil
		default:
			return "PASS", true, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error { return nil }

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
		if prompt == "red" {
			cycleCount++
		}
		return nil
	}
	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall % 2 {
		case 1:
			return "FAIL", false, nil
		default:
			return "PASS", true, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error { return nil }

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
	refactorCalls := 0

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
		refactorCalls++
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
	if refactorCalls != 0 {
		t.Fatalf("expected refactor phase to not be called when red validation passes unexpectedly, got %d", refactorCalls)
	}
}

func TestRunCycles_EarlyGreen_FinalValidationOnly_NoRefactor(t *testing.T) {
	orch := newTestOrchestrator()

	refactorCalls := 0
	validateCall := 0

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		return nil
	}
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			return "PASS", true, nil // Red validation unexpectedly passes (early-green path)
		case 2:
			return "PASS", true, nil // Final validation passes
		default:
			t.Fatalf("unexpected validation call %d", validateCall)
			return "", false, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		refactorCalls++
		return nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}
	state := singleRequirementState()

	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if refactorCalls != 0 {
		t.Fatalf("expected refactor to not be called on unexpected-pass path, got %d calls", refactorCalls)
	}
	if validateCall != 2 {
		t.Fatalf("expected exactly 2 validation calls (initial RED + final), got %d", validateCall)
	}
}

func TestRunCycles_EarlyGreen_AdvancesToDone(t *testing.T) {
	orch := newTestOrchestrator()

	redInvocations := 0

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		if prompt == "red" {
			redInvocations++
		}
		return nil
	}
	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			return "PASS", true, nil // Red validation unexpectedly passes (early-green path)
		case 2:
			return "PASS", true, nil // Final validation
		default:
			t.Fatalf("unexpected additional validation call %d", validateCall)
			return "", false, nil
		}
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

	if redInvocations != 1 {
		t.Fatalf("expected exactly one red invocation before done, got %d", redInvocations)
	}
	if validateCall != 2 {
		t.Fatalf("expected exactly 2 validation calls (initial RED + final), got %d", validateCall)
	}
}

func TestRunCycles_InitialValidationNonGreen_RunsRefactorAfterGreenValidation(t *testing.T) {
	orch := newTestOrchestrator()

	refactorCalls := 0

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
		switch validateCall {
		case 1:
			return "FAIL", false, nil // Initial validation is non-green.
		case 2:
			return "PASS", true, nil // Green validation passes.
		case 3:
			return "PASS", true, nil // Post-refactor validation.
		case 4:
			return "PASS", true, nil // Final validation.
		default:
			t.Fatalf("unexpected additional validation call %d", validateCall)
			return "", false, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		refactorCalls++
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

	if refactorCalls != 1 {
		t.Fatalf("expected refactor phase to run once after green validation, got %d", refactorCalls)
	}
}

func TestRunCycles_RedEscalation_RetriesThenEscalates(t *testing.T) {
	orch := newTestOrchestrator()

	invokeAttempts := 0
	var invokedTiers []string
	escalated := false

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		return "green", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		invokeAttempts++
		invokedTiers = append(invokedTiers, tier)
		if prompt == "red" {
			// First two attempts fail (original + retry), third succeeds (escalated)
			if invokeAttempts <= 2 {
				return fmt.Errorf("red invocation failed")
			}
		}
		return nil
	}
	orch.escalateTierFn = func(currentTier string) string {
		escalated = true
		return "high"
	}
	orch.getGitHeadFn = func() (string, error) { return "abc123", nil }
	orch.gitResetFn = func(commit string) error { return nil }

	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			return "FAIL", false, nil
		default:
			return "PASS", true, nil
		}
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

	if !escalated {
		t.Fatal("expected escalation after retry failure")
	}
	if len(invokedTiers) < 3 {
		t.Fatalf("expected at least 3 invocations (original + retry + escalated), got %d", len(invokedTiers))
	}
	// First two attempts at medium, third at high
	if invokedTiers[0] != "medium" || invokedTiers[1] != "medium" {
		t.Fatalf("expected first two attempts at medium tier, got %v", invokedTiers[:2])
	}
	if invokedTiers[2] != "high" {
		t.Fatalf("expected third attempt at high tier, got %q", invokedTiers[2])
	}
}

func TestRunCycles_UnrecoverableFailure_NoHigherTier(t *testing.T) {
	orch := newTestOrchestrator()

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		return fmt.Errorf("invocation failed")
	}
	// Already at highest tier, no escalation possible
	orch.escalateTierFn = func(currentTier string) string {
		return "" // no higher tier
	}
	orch.getGitHeadFn = func() (string, error) { return "abc123", nil }
	orch.gitResetFn = func(commit string) error { return nil }

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "high",
	}

	state := singleRequirementState()
	err := orch.RunCycles(context.Background(), bc, state)
	if err == nil {
		t.Fatal("expected error for unrecoverable failure")
	}

	want := "invocation failed after retry and escalation"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %q", want, err.Error())
	}
}

func TestRunCycles_InitialValidationNonGreen_RefactorRunsAndDoesNotRevertOnPassingPostChecks(t *testing.T) {
	orch := newTestOrchestrator()

	var resetToCommit string
	refactorCalled := false

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
		switch validateCall {
		case 1:
			return "FAIL", false, nil // Red: fail
		case 2:
			return "PASS", true, nil // Green: pass
		case 3:
			return "PASS", true, nil // Post-refactor validation: pass
		case 4:
			return "PASS", true, nil // Final validation: pass
		}
		return "PASS", true, nil
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		refactorCalled = true
		return nil
	}
	orch.getGitHeadFn = func() (string, error) { return "pre-refactor-commit", nil }
	orch.gitResetFn = func(commit string) error {
		resetToCommit = commit
		return nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}

	state := singleRequirementState()
	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("expected no error (refactor failure is non-blocking), got %v", err)
	}

	if !refactorCalled {
		t.Fatal("expected refactor phase to run after green validation")
	}
	if resetToCommit != "" {
		t.Fatalf("expected no git reset when post-refactor validation passes, got %q", resetToCommit)
	}
}

func TestExecuteRefactorPhase_ValidationFailure_ResetsToPreRefactorCommit(t *testing.T) {
	orch := newTestOrchestrator()

	refactorCalls := 0
	validateCalls := 0
	resetToCommit := ""

	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		refactorCalls++
		return nil
	}
	orch.getGitHeadFn = func() (string, error) { return "pre-refactor-commit", nil }
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCalls++
		return "FAIL", false, nil
	}
	orch.gitResetFn = func(commit string) error {
		resetToCommit = commit
		return nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
	}

	outcome := orch.executeRefactorPhase(context.Background(), bc)

	if refactorCalls != 1 {
		t.Fatalf("expected runRefactorFn to be called exactly once, got %d", refactorCalls)
	}
	if validateCalls != 1 {
		t.Fatalf("expected validateFn to be called exactly once, got %d", validateCalls)
	}
	if resetToCommit != "pre-refactor-commit" {
		t.Fatalf("expected git reset to pre-refactor commit, got %q", resetToCommit)
	}
	if outcome != refactorOutcomeRevertedContinue {
		t.Fatalf("expected refactor outcome %q, got %q", refactorOutcomeRevertedContinue, outcome)
	}
}

func TestRunCycles_GreenEscalation_RetriesThenEscalates(t *testing.T) {
	orch := newTestOrchestrator()

	greenInvokeAttempts := 0
	var greenTiers []string
	escalated := false

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		return "green", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		if prompt == "green" {
			greenInvokeAttempts++
			greenTiers = append(greenTiers, tier)
			// First two green attempts fail, third succeeds (after escalation)
			if greenInvokeAttempts <= 2 {
				return fmt.Errorf("green invocation failed")
			}
		}
		return nil
	}
	orch.escalateTierFn = func(currentTier string) string {
		escalated = true
		return "high"
	}
	orch.getGitHeadFn = func() (string, error) { return "abc", nil }
	orch.gitResetFn = func(commit string) error { return nil }

	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			return "FAIL", false, nil
		default:
			return "PASS", true, nil
		}
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

	if !escalated {
		t.Fatal("expected escalation during green phase")
	}
	if len(greenTiers) < 3 {
		t.Fatalf("expected at least 3 green invocations, got %d", len(greenTiers))
	}
	if greenTiers[0] != "medium" || greenTiers[1] != "medium" {
		t.Fatalf("expected first two green attempts at medium, got %v", greenTiers[:2])
	}
	if greenTiers[2] != "high" {
		t.Fatalf("expected third green attempt at high, got %q", greenTiers[2])
	}
}

func TestRunCycles_RefactorFnError_IsTerminal(t *testing.T) {
	orch := newTestOrchestrator()

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
		switch validateCall {
		case 1:
			return "FAIL", false, nil // Red: fail
		case 2:
			return "PASS", true, nil // Green: pass
		case 3:
			return "PASS", true, nil // Refactor: pass (refactor fn error shouldn't prevent validation)
		}
		return "PASS", true, nil
	}
	// Refactor function itself returns error
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		return fmt.Errorf("refactor crashed")
	}
	orch.getGitHeadFn = func() (string, error) { return "abc", nil }
	orch.gitResetFn = func(commit string) error { return nil }

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}

	state := singleRequirementState()
	err := orch.RunCycles(context.Background(), bc, state)
	if err == nil {
		t.Fatal("expected refactor error to be terminal")
	}
}

func TestRunCycles_EarlyGreen_FinalValidationRunsWithoutRefactor(t *testing.T) {
	orch := newTestOrchestrator()

	var phases []string
	validateCall := 0
	refactorCalled := false

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		return nil
	}
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			phases = append(phases, "initialValidation")
			return "PASS", true, nil // Early green: red validation passes
		case 2:
			phases = append(phases, "finalValidation")
			return "PASS", true, nil // Final validation (no refactor in between)
		default:
			t.Fatalf("unexpected validation call %d", validateCall)
		}
		return "", false, nil
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		refactorCalled = true
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

	// Verify exactly 2 validation calls: initial RED + final
	if validateCall != 2 {
		t.Fatalf("expected 2 validation calls (initial, final), got %d", validateCall)
	}

	// Verify refactor was never called
	if refactorCalled {
		t.Fatal("expected refactor to not be called on unexpected-pass path")
	}

	// Verify sequence: initial → final (no refactor step)
	expectedOrder := []string{"initialValidation", "finalValidation"}
	if len(phases) != len(expectedOrder) {
		t.Fatalf("expected phases %v, got %v", expectedOrder, phases)
	}
	for i, want := range expectedOrder {
		if phases[i] != want {
			t.Fatalf("phase[%d]: expected %q, got %q (full: %v)", i, want, phases[i], phases)
		}
	}
}

func TestRunCycles_EarlyGreen_FinalValidationFailure_ReturnsError(t *testing.T) {
	orch := newTestOrchestrator()

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		return nil
	}
	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			return "PASS", true, nil // Early green: red validation passes unexpectedly
		case 2:
			return "FAIL", false, nil // Final validation fails
		}
		return "", false, nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}
	state := singleRequirementState()

	err := orch.RunCycles(context.Background(), bc, state)
	if err == nil {
		t.Fatal("expected error when final validation fails, got nil")
	}
	if !strings.Contains(err.Error(), "final validation") {
		t.Fatalf("expected error to mention 'final validation', got %q", err.Error())
	}
}

func TestRunCycles_EarlyGreen_NoRefactorCalledOnUnexpectedPass(t *testing.T) {
	orch := newTestOrchestrator()

	refactorCalls := 0
	validateCall := 0

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		return "red", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		return nil
	}
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		// Both calls return passed=true: first is unexpected RED pass, second is final validation
		return "PASS", true, nil
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		refactorCalls++
		return nil
	}

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}
	state := singleRequirementState()

	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if refactorCalls != 0 {
		t.Fatalf("expected refactor to not be called when RED passes unexpectedly, got %d calls", refactorCalls)
	}
	if validateCall != 2 {
		t.Fatalf("expected validateFn called twice (initial RED + final), got %d", validateCall)
	}
}

func TestRunCycles_WritesPhaseStatusToOutput(t *testing.T) {
	orch := newTestOrchestrator()

	var buf bytes.Buffer
	orch.output = &buf

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
		switch validateCall {
		case 1:
			return "FAIL", false, nil
		default:
			return "PASS", true, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		return nil
	}
	orch.getGitHeadFn = func() (string, error) { return "abc", nil }
	orch.gitResetFn = func(commit string) error { return nil }

	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
		Tier:   "medium",
	}

	state := singleRequirementState()
	err := orch.RunCycles(context.Background(), bc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "red") {
		t.Fatalf("expected output to contain 'red' phase info, got %q", output)
	}
	if !strings.Contains(output, "green") {
		t.Fatalf("expected output to contain 'green' phase info, got %q", output)
	}
}

func TestRunCycles_InitialValidationNonGreen_RunsRefactorAndFinalValidation(t *testing.T) {
	orch := newTestOrchestrator()

	var phases []string
	var buf bytes.Buffer
	orch.output = &buf

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		phases = append(phases, "renderRed")
		return "red", nil
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		phases = append(phases, "renderGreen")
		return "green", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		switch prompt {
		case "red":
			phases = append(phases, "invokeRed")
		case "green":
			phases = append(phases, "invokeGreen")
		}
		return nil
	}

	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			phases = append(phases, "validateInitialNonGreen")
			return "FAIL", false, nil
		case 2:
			phases = append(phases, "validateGreen")
			return "PASS", true, nil
		case 3:
			phases = append(phases, "validatePostRefactor")
			return "PASS", true, nil
		case 4:
			phases = append(phases, "validateFinal")
			return "PASS", true, nil
		default:
			phases = append(phases, "unexpectedValidation")
			t.Fatalf("unexpected validation call %d", validateCall)
			return "", false, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		phases = append(phases, "refactor")
		return nil
	}
	orch.getGitHeadFn = func() (string, error) {
		phases = append(phases, "getGitHead")
		return "abc", nil
	}
	orch.gitResetFn = func(commit string) error {
		phases = append(phases, "gitReset")
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

	expectedPhases := []string{
		"renderRed",
		"invokeRed",
		"validateInitialNonGreen",
		"renderGreen",
		"invokeGreen",
		"validateGreen",
		"getGitHead",
		"refactor",
		"validatePostRefactor",
		"validateFinal",
	}
	if len(phases) != len(expectedPhases) {
		t.Fatalf("phases length = %d, want %d: %v", len(phases), len(expectedPhases), phases)
	}
	for i, want := range expectedPhases {
		if phases[i] != want {
			t.Fatalf("phases[%d] = %q, want %q (all: %v)", i, phases[i], want, phases)
		}
	}

	expectedOutput := "cycle 1: red phase — writing failing test\ncycle 1: green phase — implementing to pass\n"
	if buf.String() != expectedOutput {
		t.Fatalf("output mismatch:\n got: %q\nwant: %q", buf.String(), expectedOutput)
	}
}

func TestRunCycles_RefactorValidationFailure_RevertContinue_AdvancesToNextCycle(t *testing.T) {
	orch := newTestOrchestrator()

	var phases []string
	var redSpecExcerpts []string
	redInvocations := 0
	refactorCalls := 0
	resetCalls := 0

	orch.renderRedFn = func(handoff *RedHandoff, bc *runtypes.BeadContext) (string, error) {
		redSpecExcerpts = append(redSpecExcerpts, handoff.SpecExcerpt)
		if redInvocations == 0 {
			phases = append(phases, "renderRedCycle1")
		} else {
			phases = append(phases, "renderRedCycle2")
		}
		return "red", nil
	}
	orch.renderGreenFn = func(handoff *GreenHandoff, bc *runtypes.BeadContext) (string, error) {
		return "green", nil
	}
	orch.invokeFn = func(ctx context.Context, prompt, tier string) error {
		if prompt == "red" {
			redInvocations++
		}
		return nil
	}
	validateCall := 0
	orch.validateFn = func(ctx context.Context, commands []string, workDir string) (string, bool, error) {
		validateCall++
		switch validateCall {
		case 1:
			phases = append(phases, "validateRedCycle1")
			return "FAIL", false, nil // Cycle 1 red validation.
		case 2:
			phases = append(phases, "validateGreenCycle1")
			return "PASS", true, nil // Cycle 1 green validation.
		case 3:
			phases = append(phases, "validatePostRefactorCycle1")
			return "FAIL", false, nil // Cycle 1 post-refactor validation triggers revert.
		case 4:
			phases = append(phases, "validateFinalCycle1")
			return "PASS", true, nil // Cycle 1 final validation after revert.
		case 5:
			phases = append(phases, "validateRedCycle2")
			return "FAIL", false, nil // Cycle 2 red validation.
		case 6:
			phases = append(phases, "validateGreenCycle2")
			return "PASS", true, nil // Cycle 2 green validation.
		case 7:
			phases = append(phases, "validatePostRefactorCycle2")
			return "PASS", true, nil // Cycle 2 post-refactor validation.
		case 8:
			phases = append(phases, "validateFinalCycle2")
			return "PASS", true, nil // Cycle 2 final validation.
		default:
			t.Fatalf("unexpected validation call %d", validateCall)
			return "", false, nil
		}
	}
	orch.runRefactorFn = func(ctx context.Context, bc *runtypes.BeadContext) error {
		refactorCalls++
		return nil
	}
	orch.getGitHeadFn = func() (string, error) { return "pre-refactor-commit", nil }
	orch.gitResetFn = func(commit string) error {
		resetCalls++
		if commit != "pre-refactor-commit" {
			t.Fatalf("git reset commit = %q, want %q", commit, "pre-refactor-commit")
		}
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

	if redInvocations != 2 {
		t.Fatalf("expected two completed cycles, got %d red invocations", redInvocations)
	}
	if refactorCalls != 2 {
		t.Fatalf("expected refactor to run once per green cycle, got %d", refactorCalls)
	}
	if resetCalls != 1 {
		t.Fatalf("expected one revert after failed post-refactor validation, got %d", resetCalls)
	}

	if len(redSpecExcerpts) != 2 {
		t.Fatalf("expected red phase to run two cycles, got spec excerpts %v", redSpecExcerpts)
	}
	if redSpecExcerpts[0] != "req A" || redSpecExcerpts[1] != "req B" {
		t.Fatalf("expected cycle state progression req A -> req B, got %v", redSpecExcerpts)
	}

	expectedPhases := []string{
		"renderRedCycle1",
		"validateRedCycle1",
		"validateGreenCycle1",
		"validatePostRefactorCycle1",
		"validateFinalCycle1",
		"renderRedCycle2",
		"validateRedCycle2",
		"validateGreenCycle2",
		"validatePostRefactorCycle2",
		"validateFinalCycle2",
	}
	if len(phases) != len(expectedPhases) {
		t.Fatalf("phases length = %d, want %d: %v", len(phases), len(expectedPhases), phases)
	}
	for i, want := range expectedPhases {
		if phases[i] != want {
			t.Fatalf("phases[%d] = %q, want %q (all: %v)", i, phases[i], want, phases)
		}
	}
}
