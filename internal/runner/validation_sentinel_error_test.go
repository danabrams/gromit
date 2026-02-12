package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestValidationSentinelError_RecoveryDistinguishesErrorTypes verifies that
// runValidationWithRecovery correctly distinguishes validation failures
// (which should trigger recovery) from other errors (which should not).
func TestValidationSentinelError_RecoveryDistinguishesErrorTypes(t *testing.T) {
	// Expected failure: errValidationFailed sentinel error does not exist yet
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
			validationCalls := 0
			fixAttempts := 0

			mockClaude := &mockClaudeClient{
				RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
					validationCalls++
					if validationCalls == 1 {
						// First call - return the test error
						if tt.validationErr == errValidationFailed {
							return &claude.Result{
								Success: true,
								Output:  "FAIL: TestSomething\nVALIDATION_FAILED",
							}, nil
						}
						// For non-validation errors, return them directly
						return nil, tt.validationErr
					}
					// Second call (after fix) - pass
					return &claude.Result{
						Success: true,
						Output:  "All tests passed\nVALIDATION_PASSED",
					}, nil
				},
				StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
					fixAttempts++
					return &claude.Result{
						Success: true,
						Output:  "Fixed the validation error",
					}, nil
				},
			}

			cfg := &config.Config{
				Validation: config.ValidationConfig{
					Enabled:        true,
					Commands:       []string{"go test ./..."},
					MaxFixAttempts: 1,
				},
				Models: config.ModelsConfig{
					Validation: "haiku",
				},
				Claude: config.ClaudeConfig{
					StallTimeout:       30,
					StallTimeoutActive: 10,
				},
				Preflight: config.PreflightConfig{},
			}
			cfg.SetDefaults()

			// Create a mock provider for the router
			mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
			mockRouter := provider.NewSingleProviderRouter(mockProvider)

			r := &Runner{
				cfg:      cfg,
				claude:   mockClaude,
				router:   mockRouter,
				renderer: &mockRenderer{},
				analyzer: &mockFailureAnalyzer{},
				output:   &buf,
			}
			bc := &beadContext{
				bead:   &bead.Bead{ID: "test-1", Title: "Test"},
				model:  "sonnet",
				result: &IterationResult{},
				promptCtx: &prompt.Context{
					WorkDir:            t.TempDir(),
					ConfirmedLearnings: []learnings.Learning{},
					RecentLearnings:    []learnings.Learning{},
				},
				maxRetries:        1,
				maxRetriesPerBead: 5,
				parentCtx:         context.Background(),
			}

			err := r.runValidationWithRecovery(context.Background(), bc)

			if tt.expectRecovery {
				if err != nil && !tt.expectWrappedPass {
					t.Errorf("%s: expected no error after recovery, got: %v", tt.description, err)
				}
				if !bc.result.ValidationRetried {
					t.Errorf("%s: ValidationRetried should be true when recovery was attempted", tt.description)
				}
				if fixAttempts == 0 {
					t.Errorf("%s: expected at least one fix attempt", tt.description)
				}
			} else {
				if err == nil {
					t.Errorf("%s: expected error to pass through, got nil", tt.description)
				}
				if bc.result.ValidationRetried {
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
	// Expected failure: errValidationFailed sentinel error does not exist yet
	// Expected failure: runValidation does not return errValidationFailed yet

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
	// Expected failure: runValidation returns fmt.Errorf("validation failed") instead of errValidationFailed
	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  "FAIL: TestSomething\nVALIDATION_FAILED",
			}, nil
		},
	}

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Models: config.ModelsConfig{
			Validation: "haiku",
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		claude:   mockClaude,
		renderer: &mockRenderer{},
		analyzer: &mockFailureAnalyzer{},
		output:   &buf,
	}
	bc := &beadContext{
		bead:   &bead.Bead{ID: "test-1", Title: "Test"},
		model:  "sonnet",
		result: &IterationResult{},
		promptCtx: &prompt.Context{
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
	// Expected failure: This test demonstrates that any change to the error message
	// (like adding context) would break string comparison, but will work with errors.Is()

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
