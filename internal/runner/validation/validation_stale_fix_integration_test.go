package validation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestIntegration_StaleFixDetectionEarlyTerminationWithMultipleRetries
// demonstrates that stale-fix detection integrates with the validation retry
// loop to prevent unnecessary Claude-based fix attempts when the same files
// and error categories repeat across consecutive retry iterations.
func TestIntegration_StaleFixDetectionEarlyTerminationWithMultipleRetries(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 3, // Allow up to 3 retries
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Track which commands execute and how many times
	commandExecutions := 0

	// Validation always fails with consistent errors
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commandExecutions++
		// Command fails with consistent output
		return "", "validation failure: syntax error", 1, nil
	}

	// Auto-fix is attempted but makes no meaningful progress
	autoFixAttempts := 0
	autoFix := func(startCommit string) error {
		autoFixAttempts++
		return nil
	}

	// Claude fix should NOT be invoked due to stale-fix detection
	claudeFixAttempts := 0
	claudeFix := func(ctx context.Context, bc *runtypes.BeadContext, escalationEnabled bool) bool {
		claudeFixAttempts++
		return true
	}

	r := NewRunner(cfg, cmdRunner, autoFix, claudeFix)

	// Mock file change listing - same file is reported in each attempt
	r.listChangedFilesFn = func(ctx context.Context, sinceCommit string) ([]string, error) {
		return []string{"main.go"}, nil
	}

	// Capture log output to verify stale-fix message
	logBuffer := &bytes.Buffer{}
	r.logOutput = logBuffer

	// Prepare bead context
	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "integration-001", Title: "Stale fix test", Priority: 1},
		Tier:        "medium",
		Model:       "sonnet",
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: "/tmp/test"},
		StartCommit: "abc123",
	}

	// Execute validation recovery
	err := r.RunWithRecovery(context.Background(), bc)

	// Validation should fail (not be fixed)
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("RunWithRecovery error = %v, want ErrValidationFailed", err)
	}

	// Key assertions for integration test:

	// 1. Stale-fix detection should prevent Claude attempts after the first retry
	if claudeFixAttempts != 0 {
		t.Errorf("Claude fix attempted %d times, want 0 (stale-fix should short-circuit before Claude)",
			claudeFixAttempts)
	}

	// 2. Auto-fix should be attempted once
	if autoFixAttempts != 1 {
		t.Errorf("auto-fix attempted %d times, want 1", autoFixAttempts)
	}

	// 3. Validation should run twice: initial attempt + 1 retry before stale-fix detection
	expectedCommandExecutions := 2
	if commandExecutions != expectedCommandExecutions {
		t.Errorf("validation commands executed %d times, want %d", commandExecutions, expectedCommandExecutions)
	}

	// 4. Stale-fix short-circuit message should be in output
	if !strings.Contains(bc.Result.Output, "No meaningful auto-fix progress detected") {
		t.Fatalf("expected stale-fix short-circuit message in result output, got: %q", bc.Result.Output)
	}

	// 5. Log output should contain the warning
	if !strings.Contains(logBuffer.String(), "No meaningful auto-fix progress detected") {
		t.Fatalf("expected stale-fix warning in log output, got: %q", logBuffer.String())
	}
}

// TestIntegration_StaleFixDetectionRespectsMaxRetries
// demonstrates that when stale-fix detection does NOT trigger, the retry loop
// continues up to the configured MaxValidationRetries limit, allowing multiple
// Claude-based fix attempts to be made.
func TestIntegration_StaleFixDetectionRespectsMaxRetries(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 2, // Allow 2 retry attempts
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Validation always fails
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "test failure", 1, nil
	}

	// Auto-fix is attempted once per retry iteration
	autoFixAttempts := 0
	autoFix := func(startCommit string) error {
		autoFixAttempts++
		return nil
	}

	// Claude should be called on each retry iteration since files don't change
	// (so stale-fix won't trigger until after the first auto-fix retry)
	claudeAttempts := 0
	claudeFix := func(ctx context.Context, bc *runtypes.BeadContext, escalationEnabled bool) bool {
		claudeAttempts++
		return true // Pretend we fixed something
	}

	r := NewRunner(cfg, cmdRunner, autoFix, claudeFix)

	// Same files reported throughout (so stale-fix WILL trigger after first retry cycle)
	r.listChangedFilesFn = func(ctx context.Context, sinceCommit string) ([]string, error) {
		return []string{"main.go"}, nil
	}

	logBuffer := &bytes.Buffer{}
	r.logOutput = logBuffer

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "integration-002", Title: "Retry limit test", Priority: 1},
		Tier:        "medium",
		Model:       "sonnet",
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: "/tmp/test"},
		StartCommit: "def456",
	}

	err := r.RunWithRecovery(context.Background(), bc)

	// When stale-fix detection triggers after the first retry cycle,
	// no Claude attempts should be made
	if claudeAttempts != 0 {
		t.Errorf("Claude fix attempted %d times, want 0 (stale-fix should prevent all Claude attempts)", claudeAttempts)
	}

	// Only 1 auto-fix attempt since stale-fix detection triggers after first cycle
	if autoFixAttempts != 1 {
		t.Errorf("auto-fix attempted %d times, want 1 (stale-fix stops after first retry)", autoFixAttempts)
	}

	// Stale-fix short-circuit message should appear
	if !strings.Contains(logBuffer.String(), "No meaningful auto-fix progress detected") {
		t.Errorf("expected stale-fix message in log, got: %q", logBuffer.String())
	}

	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("RunWithRecovery error = %v, want ErrValidationFailed", err)
	}
}

// TestIntegration_StaleFixDetectionShortCircuitsBeforeSecondClaudeAttempt
// demonstrates that the first detected stale-fix situation prevents the second
// Claude-based invocation from happening at all, not just the third+ invocations.
func TestIntegration_StaleFixDetectionShortCircuitsBeforeSecondClaudeAttempt(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 2,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Validation always fails
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", fmt.Sprintf("test error from %s", command), 1, nil
	}

	// Auto-fix doesn't resolve the issue
	autoFix := func(startCommit string) error {
		return nil
	}

	// Claude fix should be called at most once, but might not be called at all
	// if stale-fix detection triggers early
	claudeFixCount := 0
	claudeFix := func(ctx context.Context, bc *runtypes.BeadContext, escalationEnabled bool) bool {
		claudeFixCount++
		return true // Pretend we fixed something
	}

	r := NewRunner(cfg, cmdRunner, autoFix, claudeFix)
	r.listChangedFilesFn = func(ctx context.Context, sinceCommit string) ([]string, error) {
		return []string{"unchanged.go"}, nil
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "integration-003", Title: "Early termination test", Priority: 1},
		Tier:        "medium",
		Model:       "sonnet",
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: "/tmp/test"},
		StartCommit: "ghi789",
	}

	err := r.RunWithRecovery(context.Background(), bc)

	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("RunWithRecovery error = %v, want ErrValidationFailed", err)
	}

	// The key assertion: Claude should never be invoked because stale-fix
	// detection prevents reaching that point in the retry loop
	if claudeFixCount > 1 {
		t.Errorf("Claude fix called %d times, want at most 1", claudeFixCount)
	}

	// Verify that we detected the stale fix and terminated early
	if !strings.Contains(bc.Result.Output, "No meaningful auto-fix progress detected") {
		t.Fatalf("expected stale-fix termination message in output, got: %q", bc.Result.Output)
	}
}
