//go:build acceptance

package runner

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// TestValidationWithRecovery_PrevFailureContainsOnlyCurrentAttempt verifies that
// when validation recovery invokes Claude for a fix, PrevFailure contains only
// the current validation attempt's output, not the accumulated output from all
// previous validation attempts.
//
// Expected failure: bc.PromptCtx.PrevFailure currently receives bc.Result.Output
// which accumulates all validation failures across attempts. After the fix,
// it should receive only the current attempt's failure output.
func TestValidationWithRecovery_PrevFailureContainsOnlyCurrentAttempt(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 2,
		},
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
			AnalysisTimeout:    30,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Track validation failure outputs across attempts
	attemptNumber := 0
	cmdRunnerFn := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		attemptNumber++
		// Each validation attempt returns a unique failure message
		failureMsg := "--- FAIL: TestAttempt" + string(rune('A'+attemptNumber-1)) + " (0.01s)\n"
		return "", failureMsg, 1, nil
	}

	// Track what PrevFailure Claude receives on each fix invocation
	var claudeInvocations []string
	executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
		// Capture PrevFailure that Claude would see
		claudeInvocations = append(claudeInvocations, bc.PromptCtx.PrevFailure)
		return true // Claude "fixes" the issue (but validation will still fail)
	}

	valRunner := validation.NewRunner(cfg, cmdRunnerFn, nil, executeFn)

	bc := newBeadContext(t)
	bc.StartCommit = "abc123"

	// Run validation with recovery
	_ = valRunner.RunWithRecovery(context.Background(), bc)

	// We should have had 3 validation attempts total:
	// 1. Initial validation (fails)
	// 2. Auto-fix -> re-validate (fails) -> Claude fix #1
	// 3. Re-validate after Claude fix #1 (fails) -> Claude fix #2
	// 4. Re-validate after Claude fix #2 (fails, exhausted retries)
	// So we should have 2 Claude invocations

	if len(claudeInvocations) != 2 {
		t.Fatalf("expected 2 Claude invocations, got %d", len(claudeInvocations))
	}

	// Expected failure: Currently, the second Claude invocation will contain
	// BOTH "TestAttemptA" and "TestAttemptB" failures in PrevFailure because
	// bc.Result.Output accumulates across attempts.
	//
	// After the fix, each Claude invocation should see ONLY the most recent
	// validation failure, not the accumulated output.

	// First Claude invocation should see only TestAttemptA or TestAttemptB failure
	firstInvocation := claudeInvocations[0]
	if !strings.Contains(firstInvocation, "TestAttempt") {
		t.Errorf("First Claude invocation should contain validation failure output, got: %q", firstInvocation)
	}

	// Second Claude invocation should see only TestAttemptC or later failure,
	// NOT the concatenation of all previous failures
	secondInvocation := claudeInvocations[1]

	// Count how many "TestAttempt" strings appear in the second invocation
	// Currently (before fix): will have multiple (accumulated)
	// After fix: should have only one (current attempt)
	attemptCount := strings.Count(secondInvocation, "TestAttempt")

	if attemptCount > 1 {
		t.Errorf("Second Claude invocation contains accumulated failures from %d attempts; should contain only current attempt.\nGot PrevFailure: %q", attemptCount, secondInvocation)
	}
}

// TestValidationWithRecovery_OutputNotAccumulatedInPrevFailure verifies that
// bc.Result.Output continues to accumulate all validation failures for logging
// purposes, but PrevFailure sent to Claude contains only the current failure.
//
// Expected failure: makeValidationExecuteFn currently sets
// bc.PromptCtx.PrevFailure = bc.Result.Output, which means Claude receives
// the full accumulated output. After the fix, it should extract only the
// current attempt's failure output.
func TestValidationWithRecovery_OutputNotAccumulatedInPrevFailure(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 2,
		},
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	failureOutputs := []string{
		"FIRST VALIDATION FAILURE: TestFoo failed",
		"SECOND VALIDATION FAILURE: TestBar failed",
		"THIRD VALIDATION FAILURE: TestBaz failed",
	}
	callCount := 0

	cmdRunnerFn := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if callCount >= len(failureOutputs) {
			callCount++
			return "ok", "", 0, nil
		}
		output := failureOutputs[callCount]
		callCount++
		return "", output, 1, nil
	}

	var capturedPrevFailures []string
	executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
		capturedPrevFailures = append(capturedPrevFailures, bc.PromptCtx.PrevFailure)
		// Check that bc.Result.Output still accumulates everything
		// (this is desired for logging/analysis)
		if !strings.Contains(bc.Result.Output, "VALIDATION OUTPUT") {
			t.Errorf("bc.Result.Output should still contain validation output marker")
		}
		return true
	}

	valRunner := validation.NewRunner(cfg, cmdRunnerFn, nil, executeFn)

	bc := newBeadContext(t)
	bc.StartCommit = "abc123"

	_ = valRunner.RunWithRecovery(context.Background(), bc)

	// Verify that bc.Result.Output accumulated everything (for logging)
	fullOutput := bc.Result.Output
	for _, expectedFragment := range []string{"FIRST VALIDATION", "SECOND VALIDATION", "THIRD VALIDATION"} {
		if !strings.Contains(fullOutput, expectedFragment) {
			t.Errorf("bc.Result.Output should contain %q for logging, but got: %q", expectedFragment, fullOutput)
		}
	}

	// Verify that each PrevFailure sent to Claude contains ONLY the current attempt
	// Expected failure: Currently PrevFailure will contain accumulated output
	if len(capturedPrevFailures) < 1 {
		t.Fatal("expected at least one Claude invocation")
	}

	for i, prevFailure := range capturedPrevFailures {
		// Each PrevFailure should NOT contain failures from previous attempts
		// Check that it contains at most one of the failure markers
		containsCount := 0
		for _, marker := range []string{"FIRST", "SECOND", "THIRD"} {
			if strings.Contains(prevFailure, marker+" VALIDATION") {
				containsCount++
			}
		}
		if containsCount > 1 {
			t.Errorf("Claude invocation #%d: PrevFailure contains %d validation failures (accumulated); should contain only current attempt's failure.\nGot: %q", i+1, containsCount, prevFailure)
		}
	}
}

// TestValidationWithRecovery_ExtractCurrentFailureFromResult verifies that
// the validation recovery mechanism extracts the current attempt's failure
// output from bc.Result.Output to pass to Claude, rather than passing the
// entire accumulated output.
//
// Expected failure: There is currently no extraction logic in
// makeValidationExecuteFn or runValidationWithRecovery to isolate the
// current failure from the accumulated bc.Result.Output.
func TestValidationWithRecovery_ExtractCurrentFailureFromResult(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			StallTimeout: 30,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Simulate validation failing twice with distinct outputs
	callCount := 0
	cmdRunnerFn := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		callCount++
		if callCount == 1 {
			return "", "ERROR_FROM_FIRST_ATTEMPT", 1, nil
		}
		return "", "ERROR_FROM_SECOND_ATTEMPT", 1, nil
	}

	var prevFailureOnSecondAttempt string
	executeFn := func(ctx context.Context, bc *runtypes.BeadContext) bool {
		prevFailureOnSecondAttempt = bc.PromptCtx.PrevFailure
		return true
	}

	valRunner := validation.NewRunner(cfg, cmdRunnerFn, nil, executeFn)

	bc := newBeadContext(t)
	bc.StartCommit = "abc123"

	_ = valRunner.RunWithRecovery(context.Background(), bc)

	// Expected failure: Currently prevFailureOnSecondAttempt will contain
	// "ERROR_FROM_FIRST_ATTEMPT" + "ERROR_FROM_SECOND_ATTEMPT" (accumulated).
	// After the fix, it should contain ONLY "ERROR_FROM_SECOND_ATTEMPT".

	if strings.Contains(prevFailureOnSecondAttempt, "ERROR_FROM_FIRST_ATTEMPT") {
		t.Errorf("PrevFailure should NOT contain first attempt's error; should contain only current attempt.\nGot: %q", prevFailureOnSecondAttempt)
	}

	if !strings.Contains(prevFailureOnSecondAttempt, "ERROR_FROM_SECOND_ATTEMPT") {
		t.Errorf("PrevFailure should contain second attempt's error.\nGot: %q", prevFailureOnSecondAttempt)
	}
}

// TestMakeValidationExecuteFn_IsolatesCurrentFailureOutput verifies that
// the makeValidationExecuteFn helper extracts only the current validation
// failure output to pass as PrevFailure to Claude, rather than passing
// the accumulated bc.Result.Output.
//
// Expected failure: The makeValidationExecuteFn function in runner.go
// currently sets bc.PromptCtx.PrevFailure = bc.Result.Output directly
// without extracting the current failure. A helper function or logic
// to extract the current failure output does not exist yet.
func TestMakeValidationExecuteFn_IsolatesCurrentFailureOutput(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			StallTimeout:       30,
			StallTimeoutActive: 10,
			AnalysisTimeout:    30,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var logBuf strings.Builder
	r, mockClaude, _, _ := setupAutoFixRunner(t, cfg)

	// Make Result.Output contain accumulated failures from multiple attempts
	bc := newBeadContext(t)
	bc.StartCommit = "abc123"
	bc.ParentCtx = context.Background()
	bc.Result.Output = "=== VALIDATION OUTPUT ===\nOLD_FAILURE_1\n\n=== VALIDATION OUTPUT ===\nOLD_FAILURE_2"

	// Set up mock to capture what PrevFailure is passed
	var capturedPrevFailure string
	mockClaude.StreamRunFn = func(ctx context.Context, prompt, systemPrompt string, readerWriter io.ReadWriter, heartbeat, stall <-chan struct{}) (*claude.Result, error) {
		// The prompt should NOT contain OLD_FAILURE_1 and OLD_FAILURE_2 anymore
		capturedPrevFailure = bc.PromptCtx.PrevFailure
		return &claude.Result{Success: false, Output: "Claude tried to fix"}, nil
	}

	// Set up validation to fail with NEW_FAILURE_3
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "NEW_FAILURE_3", 1, nil
	}

	// Set up autoFixFn to not resolve the issue
	r.autoFixFn = func(startCommit string) error {
		return nil
	}

	r.output = &logBuf

	// Set up mock router for validation recovery
	mockProviderForVal := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouterForVal := provider.NewSingleProviderRouter(mockProviderForVal)
	r.router = mockRouterForVal
	r.escalationHandler = escalation.NewHandler(cfg, &mockFailureAnalyzer{}, nil, nil, nil, func(format string, args ...interface{}) {})

	// Run validation with recovery
	_ = r.runValidationWithRecovery(context.Background(), bc)

	// Expected failure: capturedPrevFailure will contain the full accumulated
	// bc.Result.Output (OLD_FAILURE_1 + OLD_FAILURE_2 + NEW_FAILURE_3).
	// After the fix, it should contain ONLY NEW_FAILURE_3.

	if strings.Contains(capturedPrevFailure, "OLD_FAILURE_1") || strings.Contains(capturedPrevFailure, "OLD_FAILURE_2") {
		t.Errorf("PrevFailure should not contain old accumulated failures, only the current failure.\nGot PrevFailure: %q", capturedPrevFailure)
	}

	if !strings.Contains(capturedPrevFailure, "NEW_FAILURE_3") {
		t.Errorf("PrevFailure should contain the current failure NEW_FAILURE_3.\nGot PrevFailure: %q", capturedPrevFailure)
	}
}
