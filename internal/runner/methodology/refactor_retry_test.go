package methodology

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Shared test helpers ---

func newTestConfigWithEscalation() *config.Config {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./...", "go vet ./..."},
			MaxValidationRetries: 2,
		},
		Escalation: config.EscalationConfig{
			Enabled:            true,
			Chain:              []string{"low", "medium", "high"},
			MaxRetriesPerModel: 1,
		},
		Refactor: config.RefactorConfig{
			MinFilesChanged: 3,
		},
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
			AnalysisTimeout:    10,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func newTestBeadContextWithTier(tier string) *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "test-refactor-001", Title: "Test refactor bead", Priority: 1},
		Tier:        tier,
		Model:       "sonnet",
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: "/tmp/test-project"},
		StartCommit: "abc123",
	}
}

// --- ShouldRunRefactor tests ---

// Expected failure: ShouldRunRefactor method does not exist on Executor yet
func TestShouldRunRefactor(t *testing.T) {
	tests := []struct {
		name            string
		tier            string
		diff            string
		minFilesChanged int
		want            bool
	}{
		{
			name:            "skips refactor for low-tier beads",
			tier:            provider.TierLow,
			diff:            "diff --git a/file1.go b/file1.go\ndiff --git a/file2.go b/file2.go\ndiff --git a/file3.go b/file3.go\ndiff --git a/file4.go b/file4.go",
			minFilesChanged: 3,
			want:            false,
		},
		{
			name:            "always runs when threshold is zero",
			tier:            provider.TierMedium,
			diff:            "diff --git a/file1.go b/file1.go",
			minFilesChanged: 0,
			want:            true,
		},
		{
			name:            "runs when files changed exceeds threshold",
			tier:            provider.TierMedium,
			diff:            "diff --git a/file1.go b/file1.go\ndiff --git a/file2.go b/file2.go\ndiff --git a/file3.go b/file3.go\ndiff --git a/file4.go b/file4.go",
			minFilesChanged: 3,
			want:            true,
		},
		{
			name:            "skips when files changed below threshold",
			tier:            provider.TierHigh,
			diff:            "diff --git a/file1.go b/file1.go\ndiff --git a/file2.go b/file2.go",
			minFilesChanged: 3,
			want:            false,
		},
		{
			name:            "runs for high-tier beads when threshold met",
			tier:            provider.TierHigh,
			diff:            "diff --git a/a.go b/a.go\ndiff --git a/b.go b/b.go\ndiff --git a/c.go b/c.go",
			minFilesChanged: 3,
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfigWithEscalation()
			cfg.Refactor.MinFilesChanged = tt.minFilesChanged
			var buf strings.Builder

			exec := NewExecutor(cfg, &buf, nil, nil, nil)

			bc := newTestBeadContextWithTier(tt.tier)

			// Expected failure: ShouldRunRefactor does not exist on Executor yet
			got := exec.ShouldRunRefactor(bc, tt.diff)
			if got != tt.want {
				t.Errorf("ShouldRunRefactor() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- RunRefactorPhase tests ---

// Expected failure: RunRefactorPhase method does not exist on Executor yet
func TestRunRefactorPhase_SkipsWhenNoDiff(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	var buf strings.Builder

	getDiffCalled := false
	getDiffFn := func(startCommit string) (string, error) {
		getDiffCalled = true
		return "", nil // empty diff
	}

	renderRefactorCalled := false
	renderRefactorFn := func(ctx *prompt.Context) (string, error) {
		renderRefactorCalled = true
		return "refactor prompt", nil
	}

	// Expected failure: NewExecutor does not accept getDiffFn, renderRefactorFn, etc.
	exec := NewExecutorWithRefactor(cfg, &buf, NewRefactorDeps(
		getDiffFn,
		renderRefactorFn,
		nil, // refactorInvokeFn
		nil, // validateFn
		nil, // gitResetFn
		nil, // getGitHeadFn
	))

	bc := newTestBeadContextWithTier(provider.TierMedium)

	err := exec.RunRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunRefactorPhase should return nil when diff is empty, got: %v", err)
	}
	if !getDiffCalled {
		t.Error("RunRefactorPhase should call getDiffFn to check for changes")
	}
	if renderRefactorCalled {
		t.Error("RunRefactorPhase should NOT render refactor prompt when diff is empty")
	}
}

// Expected failure: RunRefactorPhase method does not exist on Executor yet
func TestRunRefactorPhase_SkipsWhenBelowFileThreshold(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	cfg.Refactor.MinFilesChanged = 5
	var buf strings.Builder

	getDiffFn := func(startCommit string) (string, error) {
		return "diff --git a/file1.go b/file1.go\n+change\ndiff --git a/file2.go b/file2.go\n+change", nil
	}

	refactorInvokeCalled := false
	refactorInvokeFn := func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
		refactorInvokeCalled = true
		return &claude.Result{Success: true}, nil
	}

	// Expected failure: NewExecutorWithRefactor does not exist yet
	exec := NewExecutorWithRefactor(cfg, &buf, NewRefactorDeps(
		getDiffFn,
		nil, // renderRefactorFn
		refactorInvokeFn,
		nil, // validateFn
		nil, // gitResetFn
		nil, // getGitHeadFn
	))

	bc := newTestBeadContextWithTier(provider.TierMedium)

	err := exec.RunRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunRefactorPhase should return nil when below file threshold, got: %v", err)
	}
	if refactorInvokeCalled {
		t.Error("RunRefactorPhase should NOT invoke refactor when below file threshold")
	}
}

// Expected failure: RunRefactorPhase method does not exist on Executor yet
func TestRunRefactorPhase_ExecutesRefactorAndRevalidates(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	cfg.Refactor.MinFilesChanged = 0 // always run
	var buf strings.Builder

	getDiffFn := func(startCommit string) (string, error) {
		return "diff --git a/file1.go b/file1.go\n+impl code", nil
	}

	renderRefactorFn := func(ctx *prompt.Context) (string, error) {
		return "refactor this code", nil
	}

	var invokedPrompt string
	var invokedTier string
	refactorInvokeFn := func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
		invokedPrompt = prompt
		invokedTier = tier
		return &claude.Result{Success: true}, nil
	}

	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
	}

	getGitHeadFn := func() (string, error) {
		return "deadbeef", nil
	}

	// Expected failure: NewExecutorWithRefactor does not exist yet
	exec := NewExecutorWithRefactor(cfg, &buf, NewRefactorDeps(
		getDiffFn,
		renderRefactorFn,
		refactorInvokeFn,
		validateFn,
		nil, // gitResetFn - not needed when validation passes
		getGitHeadFn,
	))

	bc := newTestBeadContextWithTier(provider.TierMedium)

	err := exec.RunRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunRefactorPhase should return nil on success, got: %v", err)
	}
	if invokedPrompt != "refactor this code" {
		t.Errorf("refactor invocation received prompt %q, want %q", invokedPrompt, "refactor this code")
	}
	if invokedTier != provider.TierMedium {
		t.Errorf("refactor invocation received tier %q, want %q", invokedTier, provider.TierMedium)
	}
}

// Expected failure: RunRefactorPhase method does not exist on Executor yet
func TestRunRefactorPhase_RevertsAndRetriesOnValidationFailure(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	cfg.Refactor.MinFilesChanged = 0
	var buf strings.Builder

	getDiffFn := func(startCommit string) (string, error) {
		return "diff --git a/file1.go b/file1.go\n+impl", nil
	}

	renderRefactorFn := func(ctx *prompt.Context) (string, error) {
		return "refactor prompt", nil
	}

	refactorCallCount := 0
	refactorInvokeFn := func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
		refactorCallCount++
		return &claude.Result{Success: true}, nil
	}

	validateCallCount := 0
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		validateCallCount++
		if validateCallCount == 1 {
			// First validation fails (after first refactor)
			return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
		}
		// Second validation passes (after retry refactor)
		return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
	}

	var resetCommit string
	gitResetFn := func(commit string) error {
		resetCommit = commit
		return nil
	}

	getGitHeadFn := func() (string, error) {
		return "pre-refactor-sha", nil
	}

	// Expected failure: NewExecutorWithRefactor does not exist yet
	exec := NewExecutorWithRefactor(cfg, &buf, NewRefactorDeps(
		getDiffFn,
		renderRefactorFn,
		refactorInvokeFn,
		validateFn,
		gitResetFn,
		getGitHeadFn,
	))

	bc := newTestBeadContextWithTier(provider.TierMedium)

	err := exec.RunRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunRefactorPhase should return nil even on validation failure (non-blocking), got: %v", err)
	}
	if resetCommit != "pre-refactor-sha" {
		t.Errorf("git reset should revert to pre-refactor commit %q, got %q", "pre-refactor-sha", resetCommit)
	}
	if refactorCallCount != 2 {
		t.Errorf("refactor should be invoked twice (initial + retry), got %d", refactorCallCount)
	}
	if validateCallCount != 2 {
		t.Errorf("validation should be called twice (initial + after retry), got %d", validateCallCount)
	}
}

// Expected failure: RunRefactorPhase method does not exist on Executor yet
func TestRunRefactorPhase_RevertsOnBothValidationFailures(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	cfg.Refactor.MinFilesChanged = 0
	var buf strings.Builder

	getDiffFn := func(startCommit string) (string, error) {
		return "diff --git a/file1.go b/file1.go\n+impl", nil
	}

	renderRefactorFn := func(ctx *prompt.Context) (string, error) {
		return "refactor prompt", nil
	}

	refactorInvokeFn := func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
		return &claude.Result{Success: true}, nil
	}

	// Both validations fail
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
	}

	resetCount := 0
	gitResetFn := func(commit string) error {
		resetCount++
		return nil
	}

	getGitHeadFn := func() (string, error) {
		return "pre-refactor-sha", nil
	}

	// Expected failure: NewExecutorWithRefactor does not exist yet
	exec := NewExecutorWithRefactor(cfg, &buf, NewRefactorDeps(
		getDiffFn,
		renderRefactorFn,
		refactorInvokeFn,
		validateFn,
		gitResetFn,
		getGitHeadFn,
	))

	bc := newTestBeadContextWithTier(provider.TierMedium)

	err := exec.RunRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunRefactorPhase should return nil even when both attempts fail (non-blocking), got: %v", err)
	}
	// Should revert twice: once before retry, once after retry failure
	if resetCount != 2 {
		t.Errorf("git reset should be called 2 times (revert + revert retry), got %d", resetCount)
	}
}

// --- RunAcceptanceTestsWithRetry tests ---

// Expected failure: RunAcceptanceTestsWithRetry method does not exist on Executor yet
func TestRunAcceptanceTestsWithRetry_SucceedsOnFirstAttempt(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	var buf strings.Builder

	invokeCount := 0
	renderFn := func(ctx *prompt.Context) (string, error) {
		return "acceptance prompt", nil
	}
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		invokeCount++
		return nil // succeeds immediately
	}

	exec := NewExecutor(cfg, &buf, renderFn, invokeFn, nil)

	bc := newTestBeadContextWithTier(provider.TierLow)

	// Expected failure: RunAcceptanceTestsWithRetry does not exist on Executor yet
	err := exec.RunAcceptanceTestsWithRetry(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunAcceptanceTestsWithRetry should return nil on first success, got: %v", err)
	}
	if invokeCount != 1 {
		t.Errorf("should invoke acceptance tests once on success, got %d", invokeCount)
	}
}

// Expected failure: RunAcceptanceTestsWithRetry method does not exist on Executor yet
func TestRunAcceptanceTestsWithRetry_RetriesBeforeEscalating(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	cfg.Escalation.MaxRetriesPerModel = 2
	var buf strings.Builder

	invokeCount := 0
	renderFn := func(ctx *prompt.Context) (string, error) {
		return "acceptance prompt", nil
	}
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		invokeCount++
		if invokeCount <= 3 {
			return fmt.Errorf("acceptance tests failed attempt %d", invokeCount)
		}
		return nil // succeeds on 4th attempt (after escalation)
	}

	escalateTierCalled := false
	var escalatedToTier string
	escalateTierFn := func(bc *runtypes.BeadContext, nextTier string) {
		escalateTierCalled = true
		escalatedToTier = nextTier
		bc.Tier = nextTier
	}

	// Expected failure: NewExecutorWithEscalation does not exist on Executor yet
	exec := NewExecutorWithEscalation(cfg, &buf, renderFn, invokeFn, nil, escalateTierFn)

	bc := newTestBeadContextWithTier(provider.TierLow)

	err := exec.RunAcceptanceTestsWithRetry(context.Background(), bc)
	if err != nil {
		t.Fatalf("RunAcceptanceTestsWithRetry should succeed after escalation, got: %v", err)
	}
	if !escalateTierCalled {
		t.Error("should escalate tier after exhausting retries at current tier")
	}
	if escalatedToTier != provider.TierMedium {
		t.Errorf("should escalate from low to medium, got %q", escalatedToTier)
	}
	// 3 attempts at low (1 initial + 2 retries), then 1 at medium = 4
	if invokeCount != 4 {
		t.Errorf("should invoke 4 times (3 at low + 1 at medium), got %d", invokeCount)
	}
}

// Expected failure: RunAcceptanceTestsWithRetry method does not exist on Executor yet
func TestRunAcceptanceTestsWithRetry_FailsWhenAllTiersExhausted(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	cfg.Escalation.MaxRetriesPerModel = 0  // no retries, escalate immediately
	cfg.Escalation.Chain = []string{"low"} // only one tier, no escalation possible
	var buf strings.Builder

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "acceptance prompt", nil
	}
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		return fmt.Errorf("always fails")
	}

	escalateTierFn := func(bc *runtypes.BeadContext, nextTier string) {
		bc.Tier = nextTier
	}

	// Expected failure: NewExecutorWithEscalation does not exist on Executor yet
	exec := NewExecutorWithEscalation(cfg, &buf, renderFn, invokeFn, nil, escalateTierFn)

	bc := newTestBeadContextWithTier(provider.TierLow)

	err := exec.RunAcceptanceTestsWithRetry(context.Background(), bc)
	if err == nil {
		t.Fatal("RunAcceptanceTestsWithRetry should return error when all tiers exhausted")
	}
	if !strings.Contains(err.Error(), "all tiers") {
		t.Errorf("error should mention all tiers exhausted, got: %v", err)
	}
}

// --- VerifyTestsFailWithRetry tests ---

// Expected failure: VerifyTestsFailWithRetry method does not exist on Executor yet
func TestVerifyTestsFailWithRetry_ReturnsNilWhenTestsFail(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	var buf strings.Builder

	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
	}

	exec := NewExecutor(cfg, &buf, nil, nil, validateFn)

	bc := newTestBeadContextWithTier(provider.TierMedium)

	// Expected failure: VerifyTestsFailWithRetry does not exist on Executor yet
	err := exec.VerifyTestsFailWithRetry(context.Background(), bc)
	if err != nil {
		t.Fatalf("VerifyTestsFailWithRetry should return nil when tests fail as expected, got: %v", err)
	}
}

// Expected failure: VerifyTestsFailWithRetry method does not exist on Executor yet
func TestVerifyTestsFailWithRetry_RetriesWithAnalysisWhenTestsPass(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	var buf strings.Builder

	verifyCallCount := 0
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		verifyCallCount++
		if verifyCallCount <= 1 {
			// First call: tests pass unexpectedly
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		}
		// After retry with analysis: tests now fail as expected
		return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
	}

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "retry acceptance prompt", nil
	}

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		return nil
	}

	analyzeCalled := false
	analyzeFn := func(ctx context.Context, b *bead.Bead, failureOutput string) (string, error) {
		analyzeCalled = true
		return "Tests pass because existing code already satisfies criterion", nil
	}

	// Expected failure: NewExecutorWithAnalysis does not exist yet
	exec := NewExecutorWithAnalysis(cfg, &buf, renderFn, invokeFn, validateFn, analyzeFn, nil)

	bc := newTestBeadContextWithTier(provider.TierMedium)

	err := exec.VerifyTestsFailWithRetry(context.Background(), bc)
	if err != nil {
		t.Fatalf("VerifyTestsFailWithRetry should succeed after retry with analysis, got: %v", err)
	}
	if !analyzeCalled {
		t.Error("should call analyze when tests pass unexpectedly")
	}
	if verifyCallCount < 2 {
		t.Errorf("should validate at least twice (initial + after retry), got %d", verifyCallCount)
	}
}

// Expected failure: VerifyTestsFailWithRetry method does not exist on Executor yet
func TestVerifyTestsFailWithRetry_ReturnsAlreadyDoneWhenRetryAlsoPass(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	var buf strings.Builder

	// Tests always pass — both initial and retry
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
	}

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "prompt", nil
	}
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		return nil
	}

	analyzeFn := func(ctx context.Context, b *bead.Bead, failureOutput string) (string, error) {
		return "Tests check existing behavior", nil
	}

	// Diff has implementation files (not test-only), so no extra retry
	getDiffFn := func(startCommit string) (string, error) {
		return "diff --git a/impl.go b/impl.go\n+code", nil
	}

	// Expected failure: NewExecutorWithAnalysis does not exist yet
	exec := NewExecutorWithAnalysis(cfg, &buf, renderFn, invokeFn, validateFn, analyzeFn, getDiffFn)

	bc := newTestBeadContextWithTier(provider.TierMedium)

	err := exec.VerifyTestsFailWithRetry(context.Background(), bc)
	if err == nil {
		t.Fatal("VerifyTestsFailWithRetry should return error when tests still pass after all retries")
	}
	// Expected failure: ErrATDDAlreadyDone sentinel does not exist in methodology package yet
	if !IsATDDAlreadyDone(err) {
		t.Errorf("error should be ATDD already done sentinel, got: %v", err)
	}
}

// Expected failure: VerifyTestsFailWithRetry method does not exist on Executor yet
func TestVerifyTestsFailWithRetry_TestOnlyDiffTriggersExtraRetry(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	var buf strings.Builder

	verifyCallCount := 0
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		verifyCallCount++
		if verifyCallCount <= 2 {
			// Initial and post-analysis-retry: tests pass
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		}
		// After diff-aware retry: tests now fail
		return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
	}

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "prompt", nil
	}

	invokeCount := 0
	var lastFailureContext string
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		invokeCount++
		lastFailureContext = bc.PromptCtx.FailureContext
		return nil
	}

	analyzeFn := func(ctx context.Context, b *bead.Bead, failureOutput string) (string, error) {
		return "analysis suggestion", nil
	}

	// Diff shows only test files — triggers extra diff-aware retry
	getDiffFn := func(startCommit string) (string, error) {
		return "diff --git a/pkg/handler_test.go b/pkg/handler_test.go\n+test code", nil
	}

	// Expected failure: NewExecutorWithAnalysis does not exist yet
	exec := NewExecutorWithAnalysis(cfg, &buf, renderFn, invokeFn, validateFn, analyzeFn, getDiffFn)

	bc := newTestBeadContextWithTier(provider.TierMedium)

	err := exec.VerifyTestsFailWithRetry(context.Background(), bc)
	if err != nil {
		t.Fatalf("VerifyTestsFailWithRetry should succeed after diff-aware retry, got: %v", err)
	}
	// Should have retried at least twice (analysis retry + diff-aware retry)
	if invokeCount < 2 {
		t.Errorf("should invoke acceptance tests at least twice for retries, got %d", invokeCount)
	}
	// The diff-aware retry should set specific failure context about existing behavior
	if !strings.Contains(lastFailureContext, "existing behavior") {
		t.Errorf("diff-aware retry should set failure context about existing behavior, got: %q", lastFailureContext)
	}
}

// Expected failure: VerifyTestsFailWithRetry method does not exist on Executor yet
func TestVerifyTestsFailWithRetry_SetsRetryContextOnPrompt(t *testing.T) {
	cfg := newTestConfigWithEscalation()
	var buf strings.Builder

	validateCallCount := 0
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		validateCallCount++
		if validateCallCount == 1 {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED", ExitCode: 0}, nil
		}
		return &claude.Result{Success: true, Output: "FAIL", ExitCode: 1}, nil
	}

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "prompt", nil
	}

	var capturedIsRetry bool
	var capturedFailureContext string
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		capturedIsRetry = bc.PromptCtx.IsRetry
		capturedFailureContext = bc.PromptCtx.FailureContext
		return nil
	}

	analyzeFn := func(ctx context.Context, b *bead.Bead, failureOutput string) (string, error) {
		return "rewrite tests to check new behavior", nil
	}

	// Expected failure: NewExecutorWithAnalysis does not exist yet
	exec := NewExecutorWithAnalysis(cfg, &buf, renderFn, invokeFn, validateFn, analyzeFn, nil)

	bc := newTestBeadContextWithTier(provider.TierMedium)

	_ = exec.VerifyTestsFailWithRetry(context.Background(), bc)

	if !capturedIsRetry {
		t.Error("VerifyTestsFailWithRetry should set IsRetry=true on prompt context before retry")
	}
	if capturedFailureContext != "rewrite tests to check new behavior" {
		t.Errorf("should set FailureContext from analysis suggestion, got: %q", capturedFailureContext)
	}
}
