package runner

import (
	"context"
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

// setupAutoFixRunner creates a Runner wired for auto-fix validation tests.
// It builds on setupDirectValidationRunner patterns but adds tracking for
// auto-fix tool invocations (gofmt, goimports) and Claude build-fix calls.
func setupAutoFixRunner(t *testing.T, cfg *config.Config) (*Runner, *mockClaudeClient, *mockFailureAnalyzer, *strings.Builder) {
	t.Helper()

	mockClaude := &mockClaudeClient{}
	mockAnalyzer := &mockFailureAnalyzer{}

	if cfg == nil {
		cfg = &config.Config{
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./...", "go vet ./..."},
			},
			Preflight: config.PreflightConfig{},
			Claude: config.ClaudeConfig{
				StallTimeout:       30,
				StallTimeoutActive: 10,
				AnalysisTimeout:    30,
			},
		}
	}
	cfg.SetDefaults()

	var buf strings.Builder
	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: &mockRenderer{},
		analyzer: mockAnalyzer,
		output:   &buf,
	}

	return r, mockClaude, mockAnalyzer, &buf
}

// TestAutoFix_GofmtAndGoimportsRunBeforeClaudeReinvocation verifies AC1:
// Before re-invoking Claude on validation failure, gofmt -w and goimports -w
// run automatically on changed files.
//
// Expected failure: autoFixFn does not exist on Runner yet. The implementation
// must add an injectable function that runs gofmt -w and goimports -w on
// changed Go files before attempting a Claude-based fix.
func TestAutoFix_GofmtAndGoimportsRunBeforeClaudeReinvocation(t *testing.T) {
	r, mockClaude, _, _ := setupAutoFixRunner(t, nil)
	r.cfg.Validation.MaxFixAttempts = 1

	bc := newBeadContext(t)
	bc.maxRetries = 1
	bc.maxRetriesPerBead = 5
	bc.parentCtx = context.Background()
	bc.startCommit = "abc123"

	autoFixCalled := false
	claudeFixCalled := false
	callOrder := []string{}

	// Expected failure: autoFixFn does not exist on Runner yet.
	// After implementation, this injectable function runs gofmt/goimports
	// on changed files before Claude is invoked for a fix.
	r.autoFixFn = func(startCommit string) error {
		autoFixCalled = true
		callOrder = append(callOrder, "autofix")
		return nil
	}

	mockClaude.StreamRunFn = func(ctx context.Context, p string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
		claudeFixCalled = true
		callOrder = append(callOrder, "claude")
		return &claude.Result{Success: true, Output: "Fixed"}, nil
	}

	validationCallCount := 0
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCallCount++
		if validationCallCount <= 2 { // First two calls fail (initial + after autofix)
			return "", "formatting error", 1, nil
		}
		return "ok", "", 0, nil // Third call passes (after Claude fix)
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	if !autoFixCalled {
		t.Error("expected autoFixFn to be called before Claude re-invocation")
	}
	if !claudeFixCalled {
		t.Error("expected Claude to be called for fix after autofix didn't fully resolve")
	}
	if len(callOrder) < 2 || callOrder[0] != "autofix" || callOrder[1] != "claude" {
		t.Errorf("expected autofix before claude, got call order: %v", callOrder)
	}
}

// TestAutoFix_PassesStartCommitToAutoFixFn verifies AC1: the auto-fix step
// receives the startCommit so it can determine which files changed.
//
// Expected failure: autoFixFn does not exist on Runner yet.
func TestAutoFix_PassesStartCommitToAutoFixFn(t *testing.T) {
	r, mockClaude, _, _ := setupAutoFixRunner(t, nil)
	r.cfg.Validation.MaxFixAttempts = 1

	bc := newBeadContext(t)
	bc.maxRetries = 1
	bc.maxRetriesPerBead = 5
	bc.parentCtx = context.Background()
	bc.startCommit = "deadbeef"

	var receivedCommit string

	// Expected failure: autoFixFn does not exist on Runner yet.
	r.autoFixFn = func(startCommit string) error {
		receivedCommit = startCommit
		return nil
	}

	mockClaude.StreamRunFn = func(ctx context.Context, p string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
		return &claude.Result{Success: true, Output: "Fixed"}, nil
	}

	validationCallCount := 0
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCallCount++
		if validationCallCount == 1 {
			return "", "test failure", 1, nil
		}
		return "ok", "", 0, nil
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	if receivedCommit != "deadbeef" {
		t.Errorf("expected autoFixFn to receive startCommit %q, got %q", "deadbeef", receivedCommit)
	}
}

// TestAutoFix_ValidationRetryCappedAt2 verifies AC2: validation retry is
// capped at 2 attempts per bead; after 2 failures the bead is marked failed.
//
// Expected failure: MaxValidationRetries config field does not exist yet.
// The current MaxFixAttempts defaults to 1. The implementation must add a
// MaxValidationRetries field (defaulting to 2) that caps total validation
// retry attempts including both auto-fix and Claude-based fix cycles.
func TestAutoFix_ValidationRetryCappedAt2(t *testing.T) {
	tests := []struct {
		name                 string
		maxRetries           int
		validationAlwaysFail bool
		expectClaudeCalls    int
		expectError          bool
	}{
		{
			name:                 "capped at 2 retries - all fail",
			maxRetries:           2,
			validationAlwaysFail: true,
			expectClaudeCalls:    2,
			expectError:          true,
		},
		{
			name:                 "succeeds on second retry",
			maxRetries:           2,
			validationAlwaysFail: false, // will succeed on attempt 3
			expectClaudeCalls:    2,
			expectError:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Validation: config.ValidationConfig{
					Enabled:  true,
					Commands: []string{"go test ./..."},
					// Expected failure: MaxValidationRetries does not exist yet.
					// After implementation, this field caps total validation retry attempts.
					MaxValidationRetries: tt.maxRetries,
				},
				Preflight: config.PreflightConfig{},
				Claude: config.ClaudeConfig{
					StallTimeout:       30,
					StallTimeoutActive: 10,
					AnalysisTimeout:    30,
				},
			}
			cfg.SetDefaults()

			r, mockClaude, _, _ := setupAutoFixRunner(t, cfg)

			claudeCallCount := 0
			mockClaude.StreamRunFn = func(ctx context.Context, p string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
				claudeCallCount++
				return &claude.Result{Success: true, Output: "Fixed"}, nil
			}

			// Expected failure: autoFixFn does not exist on Runner yet.
			r.autoFixFn = func(startCommit string) error { return nil }

			validationCallCount := 0
			r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
				validationCallCount++
				if tt.validationAlwaysFail {
					return "", "FAIL: test", 1, nil
				}
				// Fail first 2, pass on 3rd
				if validationCallCount <= 2 {
					return "", "FAIL: test", 1, nil
				}
				return "ok", "", 0, nil
			}

			bc := newBeadContext(t)
			bc.maxRetries = 1
			bc.maxRetriesPerBead = 5
			bc.parentCtx = context.Background()
			bc.startCommit = "abc123"

			err := r.runValidationWithRecovery(context.Background(), bc)

			if tt.expectError && err == nil {
				t.Error("expected error after exhausting validation retries")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected success, got error: %v", err)
			}

			if claudeCallCount > tt.expectClaudeCalls {
				t.Errorf("expected at most %d Claude fix calls, got %d — retry cap not enforced",
					tt.expectClaudeCalls, claudeCallCount)
			}
		})
	}
}

// TestAutoFix_TrivialFixResolvedWithoutClaude verifies AC3: when gofmt/goimports
// resolve the validation failure, no Claude invocation is needed for the fix.
//
// Expected failure: autoFixFn does not exist on Runner yet. After implementation,
// if the auto-fix step (gofmt + goimports) resolves the validation failure,
// runValidationWithRecovery should skip the Claude build-fix invocation entirely.
func TestAutoFix_TrivialFixResolvedWithoutClaude(t *testing.T) {
	r, mockClaude, _, _ := setupAutoFixRunner(t, nil)
	r.cfg.Validation.MaxFixAttempts = 1

	bc := newBeadContext(t)
	bc.maxRetries = 1
	bc.maxRetriesPerBead = 5
	bc.parentCtx = context.Background()
	bc.startCommit = "abc123"

	claudeFixCalled := false
	mockClaude.StreamRunFn = func(ctx context.Context, p string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
		claudeFixCalled = true
		return &claude.Result{Success: true, Output: "Fixed"}, nil
	}

	// Expected failure: autoFixFn does not exist on Runner yet.
	r.autoFixFn = func(startCommit string) error {
		return nil // auto-fix "succeeds" (runs gofmt/goimports)
	}

	validationCallCount := 0
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCallCount++
		if validationCallCount == 1 {
			return "", "formatting error in main.go", 1, nil // First validation fails
		}
		return "ok", "", 0, nil // After autofix, validation passes
	}

	err := r.runValidationWithRecovery(context.Background(), bc)

	if err != nil {
		t.Errorf("expected no error when trivial fix resolves issue, got: %v", err)
	}
	if claudeFixCalled {
		t.Error("expected Claude NOT to be called when auto-fix resolved the validation failure — trivial fixes should not require a Claude invocation")
	}
}

// TestAutoFix_TrivialAutoFixedFieldSetOnResult verifies AC3: when a trivial
// fix resolves validation, the result records that it was auto-fixed.
//
// Expected failure: TrivialAutoFixed field does not exist on IterationResult yet.
// After implementation, the result should indicate that the fix was trivial.
func TestAutoFix_TrivialAutoFixedFieldSetOnResult(t *testing.T) {
	r, _, _, _ := setupAutoFixRunner(t, nil)
	r.cfg.Validation.MaxFixAttempts = 1

	bc := newBeadContext(t)
	bc.maxRetries = 1
	bc.maxRetriesPerBead = 5
	bc.parentCtx = context.Background()
	bc.startCommit = "abc123"

	// Expected failure: autoFixFn does not exist on Runner yet.
	r.autoFixFn = func(startCommit string) error { return nil }

	validationCallCount := 0
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCallCount++
		if validationCallCount == 1 {
			return "", "formatting error", 1, nil
		}
		return "ok", "", 0, nil
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	// Expected failure: TrivialAutoFixed field does not exist on IterationResult yet.
	if !bc.result.TrivialAutoFixed {
		t.Error("expected TrivialAutoFixed=true when auto-fix resolved validation without Claude")
	}
}

// TestAutoFix_RevalidatesAfterAutoFixBeforeCallingClaude verifies AC1+AC3:
// after running gofmt/goimports, validation is re-run. Only if it still fails
// does Claude get invoked for a fix.
//
// Expected failure: autoFixFn does not exist on Runner yet.
func TestAutoFix_RevalidatesAfterAutoFixBeforeCallingClaude(t *testing.T) {
	r, mockClaude, _, _ := setupAutoFixRunner(t, nil)
	r.cfg.Validation.MaxFixAttempts = 1

	bc := newBeadContext(t)
	bc.maxRetries = 1
	bc.maxRetriesPerBead = 5
	bc.parentCtx = context.Background()
	bc.startCommit = "abc123"

	callSequence := []string{}

	// Expected failure: autoFixFn does not exist on Runner yet.
	r.autoFixFn = func(startCommit string) error {
		callSequence = append(callSequence, "autofix")
		return nil
	}

	mockClaude.StreamRunFn = func(ctx context.Context, p string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
		callSequence = append(callSequence, "claude")
		return &claude.Result{Success: true, Output: "Fixed"}, nil
	}

	validationCallCount := 0
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCallCount++
		callSequence = append(callSequence, "validate")
		if validationCallCount <= 2 { // Fail on initial and after autofix
			return "", "test failure", 1, nil
		}
		return "ok", "", 0, nil
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	// Expected sequence: validate(fail) -> autofix -> validate(fail) -> claude -> validate(pass)
	expectedPrefixOrder := []string{"validate", "autofix", "validate"}
	if len(callSequence) < 3 {
		t.Fatalf("expected at least 3 calls, got %d: %v", len(callSequence), callSequence)
	}
	for i, expected := range expectedPrefixOrder {
		if callSequence[i] != expected {
			t.Errorf("callSequence[%d]: expected %q, got %q (full: %v)", i, expected, callSequence[i], callSequence)
		}
	}
}

// TestAutoFix_MaxValidationRetriesDefault verifies AC2: the default value for
// MaxValidationRetries is 2, meaning validation retries are capped at 2 by default.
//
// Expected failure: MaxValidationRetries field does not exist on ValidationConfig yet.
func TestAutoFix_MaxValidationRetriesDefault(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	// Expected failure: MaxValidationRetries field does not exist yet.
	if cfg.Validation.MaxValidationRetries != 2 {
		t.Errorf("expected MaxValidationRetries default of 2, got %d", cfg.Validation.MaxValidationRetries)
	}
}

// TestAutoFix_BeadMarkedFailedAfterMaxRetries verifies AC2: after exhausting
// validation retries, the bead result indicates failure.
//
// Expected failure: autoFixFn and MaxValidationRetries do not exist yet.
func TestAutoFix_BeadMarkedFailedAfterMaxRetries(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
			// Expected failure: MaxValidationRetries does not exist yet.
			MaxValidationRetries: 2,
		},
		Preflight: config.PreflightConfig{},
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
			AnalysisTimeout:    30,
		},
	}
	cfg.SetDefaults()

	r, mockClaude, _, _ := setupAutoFixRunner(t, cfg)

	mockClaude.StreamRunFn = func(ctx context.Context, p string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
		return &claude.Result{Success: true, Output: "Fixed"}, nil
	}

	// Expected failure: autoFixFn does not exist on Runner yet.
	r.autoFixFn = func(startCommit string) error { return nil }

	// Validation always fails
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "FAIL: always", 1, nil
	}

	bc := newBeadContext(t)
	bc.maxRetries = 1
	bc.maxRetriesPerBead = 5
	bc.parentCtx = context.Background()
	bc.startCommit = "abc123"

	err := r.runValidationWithRecovery(context.Background(), bc)

	if err == nil {
		t.Error("expected error when all validation retries exhausted")
	}
	// The bead should NOT be retried further — the error should be terminal
	if !bc.result.ValidationRetried {
		t.Error("expected ValidationRetried=true after exhausting retries")
	}
}

// Suppress unused import warnings — these imports are used by test types above.
var _ = (*bead.Bead)(nil)
var _ = (*learnings.Learning)(nil)
var _ = (*prompt.Context)(nil)
var _ = (*provider.Router)(nil)
