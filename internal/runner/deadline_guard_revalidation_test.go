package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestDeadlineGuard_JustAboveMinRevalidation_NonTimeoutErrorIsTerminal verifies
// that when bead remaining time is just above minRevalidationTime (so the
// re-validation deadline guard does NOT skip), validation runs and returns a
// non-timeout error, and the error is properly wrapped and returned as a
// terminal failure.
//
// This ensures the skipRevalidation guard is strictly scoped to the skip
// decision and cannot interfere with how actual validation errors are propagated.
func TestDeadlineGuard_JustAboveMinRevalidation_NonTimeoutErrorIsTerminal(t *testing.T) {
	validationRan := false

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationRan = true
		return "", "FAIL: test suite failed: 3 tests failed", 1, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	r.cfg.Refactor.MinFilesChanged = 0
	r.methodologyExec = r.makeMethodologyExec()
	r.methodologyExec.SetRefactorDeps(methodology.NewRefactorDeps(
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		func(ctx *prompt.Context) (string, error) {
			return "refactor prompt", nil
		},
		nil,
		nil,
		func(commit string) error { return nil },
		func() (string, error) { return "abc123", nil },
	))

	// Set bead budget so remaining time is just above minRevalidationTime (30s).
	// We need remaining > minRefactorTime (60s) so the refactor guard also passes,
	// then refactor completes instantly in tests, and re-validation guard sees
	// remaining ≈ 61s > 30s → does NOT skip.
	// BeadTimeout = 200s, elapsed ≈ 139s → remaining ≈ 61s.
	bc := &runtypes.BeadContext{
		StartCommit:   "abc123",
		BeadTimeout:   200 * time.Second,
		BeadStartTime: time.Now().Add(-139 * time.Second),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	retry, terminal := r.runRefactorAndPostChecks(context.Background(), bc, false, 1)

	if retry {
		t.Fatal("expected retry=false")
	}
	if terminal == nil {
		t.Fatal("expected terminal result (non-nil), got nil — deadline guard may have suppressed the validation error")
	}
	if terminal.Error == nil {
		t.Fatal("expected terminal.Error to be set with wrapped validation failure")
	}
	if !strings.Contains(terminal.Error.Error(), "validation failed after refactoring") {
		t.Errorf("expected error to contain 'validation failed after refactoring', got: %q", terminal.Error.Error())
	}
	if !validationRan {
		t.Error("expected validation to run (re-validation guard should not have skipped with ~61s remaining vs 30s threshold)")
	}
}
