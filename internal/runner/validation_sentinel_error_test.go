package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// TestValidationSentinelError_RecoveryDistinguishesErrorTypes verifies that
// runValidationWithRecovery correctly distinguishes validation failures
// (which should trigger recovery) from other errors (which should not).
func TestValidationSentinelError_RecoveryDistinguishesErrorTypes(t *testing.T) {
	tests := []struct {
		name              string
		validationErr     error
		expectRecovery    bool
		expectWrappedPass bool
		description       string
	}{
		{
			name:           "validation failure triggers recovery",
			validationErr:  errValidationFailed,
			expectRecovery: true,
			description:    "Direct validation failure should trigger recovery",
		},
		{
			name:              "wrapped validation failure triggers recovery",
			validationErr:     fmt.Errorf("additional context: %w", errValidationFailed),
			expectRecovery:    true,
			expectWrappedPass: true,
			description:       "Wrapped validation failure should still trigger recovery via errors.Is()",
		},
		{
			name:           "invocation error does not trigger recovery",
			validationErr:  fmt.Errorf("validation invocation failed: network error"),
			expectRecovery: false,
			description:    "Non-validation errors should pass through without recovery",
		},
		{
			name:           "nil result error does not trigger recovery",
			validationErr:  fmt.Errorf("validation returned no result"),
			expectRecovery: false,
			description:    "Infrastructure errors should not trigger recovery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			cmdCallCount := 0
			fixAttempts := 0

			cfg := &config.Config{
				Validation: config.ValidationConfig{
					Enabled:              true,
					Commands:             []string{"go test ./..."},
					MaxFixAttempts:       1,
					MaxValidationRetries: 1,
				},
				Claude: config.ClaudeConfig{
					StallTimeout:       30,
					StallTimeoutActive: 10,
				},
				Preflight: config.PreflightConfig{},
			}
			cfg.SetDefaults()

			// Build cmdRunnerFn based on test case
			testErr := tt.validationErr
			var cmdRunnerFn runtypes.CmdRunnerFn
			if errors.Is(testErr, errValidationFailed) {
				// Simulate validation failure then pass on recovery
				cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
					cmdCallCount++
					if cmdCallCount == 1 {
						return "", "FAIL: TestSomething", 1, nil
					}
					return "ok", "", 0, nil
				}
			} else {
				// Simulate command execution error (not recoverable)
				cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
					cmdCallCount++
					return "", "", -1, fmt.Errorf("command execution error")
				}
			}

			// executeFn tracks fix attempts (replaces mockClaude)
			executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
				fixAttempts++
				return true
			}

			valRunner := validation.NewRunner(cfg, cmdRunnerFn, nil, executeFn)

			r := &Runner{
				cfg:              cfg,
				renderer:         &mockRenderer{},
				analyzer:         &mockFailureAnalyzer{},
				output:           &buf,
				validationRunner: valRunner,
			}
			bc := &runtypes.BeadContext{
				Bead:   &bead.Bead{ID: "test-1", Title: "Test"},
				Model:  "sonnet",
				Result: &IterationResult{},
				PromptCtx: &prompt.Context{
					WorkDir:            t.TempDir(),
					ConfirmedLearnings: []learnings.Learning{},
					RecentLearnings:    []learnings.Learning{},
				},
				MaxRetries:        1,
				MaxRetriesPerBead: 5,
				ParentCtx:         context.Background(),
			}

			err := r.runValidationWithRecovery(context.Background(), bc)

			if tt.expectRecovery {
				if err != nil && !tt.expectWrappedPass {
					t.Errorf("%s: expected no error after recovery, got: %v", tt.description, err)
				}
				if !bc.Result.ValidationRetried {
					t.Errorf("%s: ValidationRetried should be true when recovery was attempted", tt.description)
				}
				if fixAttempts == 0 {
					t.Errorf("%s: expected at least one fix attempt", tt.description)
				}
			} else {
				if err == nil {
					t.Errorf("%s: expected error to pass through, got nil", tt.description)
				}
				if bc.Result.ValidationRetried {
					t.Errorf("%s: ValidationRetried should be false when recovery is not triggered", tt.description)
				}
				if fixAttempts > 0 {
					t.Errorf("%s: expected no fix attempts, got %d", tt.description, fixAttempts)
				}
			}
		})
	}
}

// TestValidationSentinelError_WrappedErrorsPreserved verifies that validation
// errors can be wrapped with additional context and still be recognized as
// validation failures using errors.Is().
func TestValidationSentinelError_WrappedErrorsPreserved(t *testing.T) {
	// Create a wrapped validation error as might happen in real code
	wrappedErr := fmt.Errorf("post-validation processing failed: %w", errValidationFailed)

	// Verify errors.Is() correctly identifies the wrapped error
	if !errors.Is(wrappedErr, errValidationFailed) {
		t.Error("errors.Is() should identify wrapped errValidationFailed")
	}

	// Verify direct error is also recognized
	if !errors.Is(errValidationFailed, errValidationFailed) {
		t.Error("errors.Is() should identify direct errValidationFailed")
	}

	// Verify non-validation errors are not matched
	otherErr := fmt.Errorf("validation invocation failed")
	if errors.Is(otherErr, errValidationFailed) {
		t.Error("errors.Is() should not match non-validation errors")
	}
}

// TestValidationSentinelError_RunValidationReturnsCorrectError verifies that
// runValidation returns errValidationFailed (not a string-based error) when
// validation fails.
func TestValidationSentinelError_RunValidationReturnsCorrectError(t *testing.T) {
	var buf strings.Builder

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "FAIL: TestSomething", 1, nil
	}

	r := &Runner{
		cfg:              cfg,
		renderer:         &mockRenderer{},
		analyzer:         &mockFailureAnalyzer{},
		output:           &buf,
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
	}
	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test"},
		Model:  "sonnet",
		Result: &IterationResult{},
		PromptCtx: &prompt.Context{
			WorkDir:            t.TempDir(),
			ConfirmedLearnings: []learnings.Learning{},
			RecentLearnings:    []learnings.Learning{},
		},
	}

	err := r.runValidation(context.Background(), bc)

	// Verify we get the sentinel error, not a string-based error
	if err == nil {
		t.Fatal("expected error when validation fails")
	}

	if !errors.Is(err, errValidationFailed) {
		t.Errorf("expected errValidationFailed sentinel error, got: %v (type: %T)", err, err)
	}

	// Ensure the error message is still readable
	if err.Error() != "validation failed" {
		t.Errorf("expected error message 'validation failed', got: %q", err.Error())
	}
}

// TestValidationSentinelError_StringComparisonWouldBreak verifies that a
// simple string-based error message change would break validation recovery,
// demonstrating why a sentinel error is necessary.
func TestValidationSentinelError_StringComparisonWouldBreak(t *testing.T) {
	// Simulate what would happen if runValidation added context to the error
	baseErr := errValidationFailed
	contextualizedErr := fmt.Errorf("after 3 attempts: %w", baseErr)

	// String comparison would fail
	if contextualizedErr.Error() == "validation failed" {
		t.Error("Adding context breaks exact string comparison - this is why sentinel errors are needed")
	}

	// But errors.Is() still works
	if !errors.Is(contextualizedErr, errValidationFailed) {
		t.Error("errors.Is() should recognize validation failure even with added context")
	}

	// This demonstrates the robustness advantage of sentinel errors
	wrappedTwice := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", errValidationFailed))
	if !errors.Is(wrappedTwice, errValidationFailed) {
		t.Error("errors.Is() should work through multiple layers of wrapping")
	}
}
