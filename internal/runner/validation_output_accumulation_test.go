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
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// setupValidationAccumulationRunner creates a fully-wired Runner that uses the real
// makeValidationExecuteFn so we can test the actual bug: PrevFailure contains
// accumulated bc.Result.Output instead of just the current failure.
func setupValidationAccumulationRunner(t *testing.T, cfg *config.Config) (*Runner, *mockClaudeClient, *strings.Builder) {
	t.Helper()

	if cfg == nil {
		cfg = &config.Config{
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
			Preflight: config.PreflightConfig{},
		}
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	mockClaude := &mockClaudeClient{}
	mockAnalyzer := &mockFailureAnalyzer{}
	var logBuf strings.Builder

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)
	mockRenderer := &mockRenderer{}

	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		analyzer: mockAnalyzer,
		output:   &logBuf,
		log:      logFn,
	}

	// Wire the invoker
	r.invoker = execution.NewInvoker(mockRouter, &logBuf, nil, logFn)

	// Wire the escalation handler
	r.escalationHandler = escalation.NewHandler(cfg, mockAnalyzer, nil, nil, mockRenderer, logFn)

	// Use the REAL makeValidationExecuteFn from the runner
	// This is the key: we're testing the actual facade method that has the bug
	r.validationRunner = validation.NewRunner(cfg, nil, r.autoFixFn, r.makeValidationExecuteFn())

	// Wire reviewer (needed for runValidationWithRecovery)
	r.reviewer = reviewpkg.NewReviewer(cfg, mockRouter, &mockBeadClient{}, mockRenderer, r.gitDiffFn, nil)
	r.reviewer.SetLogFn(logFn)

	return r, mockClaude, &logBuf
}

// TestValidationAccumulation_PrevFailureIsolatesCurrentAttempt verifies that
// when makeValidationExecuteFn is called during validation recovery, it sets
// bc.PromptCtx.PrevFailure to ONLY the current validation failure, not the
// accumulated bc.Result.Output from all previous attempts.
//
// Expected failure: Line 1023 in runner.go currently sets:
//
//	bc.PromptCtx.PrevFailure = bc.Result.Output
//
// This means PrevFailure contains ALL accumulated validation failures.
// After the fix, it should extract only the most recent failure.
func TestValidationAccumulation_PrevFailureIsolatesCurrentAttempt(t *testing.T) {
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
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r, mockClaude, _ := setupValidationAccumulationRunner(t, cfg)

	// Track validation failures returned by each cmdRunner call
	validationCallCount := 0
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCallCount++
		// Each validation attempt returns a unique, identifiable failure
		failureMsg := "VALIDATION_FAILURE_ATTEMPT_" + string(rune('A'+validationCallCount-1))
		return "", failureMsg, 1, nil
	}

	// Track what PrevFailure is set when Claude is invoked
	var capturedPrevFailures []string
	mockClaude.StreamRunFn = func(ctx context.Context, prompt string, systemPrompt string, output io.ReadWriter, heartbeat <-chan struct{}, stall <-chan struct{}) (*claude.Result, error) {
		// This is called by the invoker, which uses bc.BuildPrompt
		// The bc.BuildPrompt was rendered with bc.PromptCtx.PrevFailure
		// We need to capture what PrevFailure was set to before rendering
		// Unfortunately, we can't directly access bc here, so we inspect the prompt
		capturedPrevFailures = append(capturedPrevFailures, prompt)
		return &claude.Result{Success: false, Output: "Claude attempted fix"}, nil
	}

	// Set up a no-op autoFixFn
	r.autoFixFn = func(startCommit string) error {
		return nil // auto-fix doesn't resolve the issue
	}

	bc := newBeadContext(t)
	bc.StartCommit = "abc123"
	bc.ParentCtx = context.Background()
	bc.MaxRetries = 1
	bc.MaxRetriesPerBead = 5

	// Run validation with recovery
	_ = r.runValidationWithRecovery(context.Background(), bc)

	// After the run, bc.Result.Output should contain ALL validation failures (accumulated)
	fullOutput := bc.Result.Output
	if !strings.Contains(fullOutput, "VALIDATION_FAILURE_ATTEMPT_A") {
		t.Errorf("bc.Result.Output should contain first validation failure for logging")
	}
	if !strings.Contains(fullOutput, "VALIDATION_FAILURE_ATTEMPT_B") {
		t.Errorf("bc.Result.Output should contain second validation failure for logging")
	}

	// Expected failure: Currently, the second Claude invocation will receive
	// a prompt containing BOTH VALIDATION_FAILURE_ATTEMPT_A and VALIDATION_FAILURE_ATTEMPT_B
	// because makeValidationExecuteFn sets PrevFailure = bc.Result.Output (accumulated).
	//
	// After the fix, each Claude invocation should see ONLY the most recent failure.
	if len(capturedPrevFailures) < 2 {
		t.Fatalf("Expected at least 2 Claude invocations, got %d", len(capturedPrevFailures))
	}

	secondPrompt := capturedPrevFailures[1]
	attemptACount := strings.Count(secondPrompt, "VALIDATION_FAILURE_ATTEMPT_A")
	attemptBCount := strings.Count(secondPrompt, "VALIDATION_FAILURE_ATTEMPT_B")

	// The second Claude prompt should NOT contain ATTEMPT_A (it was from the first failure)
	if attemptACount > 0 {
		t.Errorf("Second Claude invocation's prompt should not contain VALIDATION_FAILURE_ATTEMPT_A (from first attempt). Found %d occurrences.\nPrompt excerpt: %s",
			attemptACount, secondPrompt[:min(len(secondPrompt), 500)])
	}

	// The second Claude prompt SHOULD contain ATTEMPT_B or later (current failure)
	if attemptBCount == 0 && !strings.Contains(secondPrompt, "VALIDATION_FAILURE_ATTEMPT_C") {
		t.Errorf("Second Claude invocation should contain current failure (ATTEMPT_B or later).\nPrompt excerpt: %s",
			secondPrompt[:min(len(secondPrompt), 500)])
	}
}

// TestMakeValidationExecuteFn_DoesNotPassAccumulatedOutput verifies that
// the makeValidationExecuteFn method in runner.go does not pass the full
// accumulated bc.Result.Output to Claude as PrevFailure. Instead, it should
// extract only the most recent validation failure.
//
// Expected failure: Line 1023 in runner.go sets:
//
//	bc.PromptCtx.PrevFailure = bc.Result.Output
//
// There is no extraction logic yet. After the fix, there should be logic
// to extract the current failure from bc.Result.Output.
func TestMakeValidationExecuteFn_DoesNotPassAccumulatedOutput(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 1,
		},
		Claude: config.ClaudeConfig{
			StallTimeout: 30,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r, mockClaude, _ := setupValidationAccumulationRunner(t, cfg)

	// Pre-populate bc.Result.Output with old accumulated failures
	// Then have validation fail with a new, distinct failure
	// The makeValidationExecuteFn should extract ONLY the new failure
	bc := newBeadContext(t)
	bc.StartCommit = "abc123"
	bc.ParentCtx = context.Background()
	bc.MaxRetries = 1
	bc.MaxRetriesPerBead = 5

	// Simulate that bc.Result.Output already has previous validation output
	bc.Result.Output = "=== VALIDATION OUTPUT ===\nOLD_ACCUMULATED_FAILURE_1\n\n=== VALIDATION OUTPUT ===\nOLD_ACCUMULATED_FAILURE_2"

	callCount := 0
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		callCount++
		// Validation fails with a NEW distinct failure
		return "", "NEW_CURRENT_FAILURE", 1, nil
	}

	r.autoFixFn = func(startCommit string) error {
		return nil
	}

	var capturedPrompt string
	mockClaude.StreamRunFn = func(ctx context.Context, prompt string, systemPrompt string, output io.ReadWriter, heartbeat <-chan struct{}, stall <-chan struct{}) (*claude.Result, error) {
		capturedPrompt = prompt
		return &claude.Result{Success: false, Output: "Claude tried"}, nil
	}

	_ = r.runValidationWithRecovery(context.Background(), bc)

	// Expected failure: capturedPrompt will contain OLD_ACCUMULATED_FAILURE_1 and
	// OLD_ACCUMULATED_FAILURE_2 because makeValidationExecuteFn passes the full
	// bc.Result.Output as PrevFailure.
	//
	// After the fix, capturedPrompt should contain ONLY NEW_CURRENT_FAILURE.
	if strings.Contains(capturedPrompt, "OLD_ACCUMULATED_FAILURE_1") {
		t.Errorf("Prompt should not contain old accumulated failures (OLD_ACCUMULATED_FAILURE_1).\nPrompt excerpt: %s",
			capturedPrompt[:min(len(capturedPrompt), 500)])
	}
	if strings.Contains(capturedPrompt, "OLD_ACCUMULATED_FAILURE_2") {
		t.Errorf("Prompt should not contain old accumulated failures (OLD_ACCUMULATED_FAILURE_2).\nPrompt excerpt: %s",
			capturedPrompt[:min(len(capturedPrompt), 500)])
	}
	if !strings.Contains(capturedPrompt, "NEW_CURRENT_FAILURE") {
		t.Errorf("Prompt should contain the current failure (NEW_CURRENT_FAILURE).\nPrompt excerpt: %s",
			capturedPrompt[:min(len(capturedPrompt), 500)])
	}
}

// TestValidationRecovery_ExtractCurrentFailureFromAccumulatedOutput verifies
// that when validation fails multiple times, each Claude fix invocation receives
// only the current attempt's failure, not all previous failures concatenated.
//
// Expected failure: The extractCurrentValidationFailure function or equivalent
// logic does not exist yet. The code at line 1023 in runner.go directly assigns:
//
//	bc.PromptCtx.PrevFailure = bc.Result.Output
//
// After implementation, there should be logic to extract the most recent
// "=== VALIDATION OUTPUT ===" section from bc.Result.Output.
func TestValidationRecovery_ExtractCurrentFailureFromAccumulatedOutput(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 2,
		},
		Claude: config.ClaudeConfig{
			StallTimeout: 30,
		},
		Preflight: config.PreflightConfig{},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r, mockClaude, _ := setupValidationAccumulationRunner(t, cfg)

	failureMessages := []string{
		"FIRST_FAILURE_MESSAGE",
		"SECOND_FAILURE_MESSAGE",
		"THIRD_FAILURE_MESSAGE",
	}
	callIndex := 0

	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if callIndex >= len(failureMessages) {
			// Eventually pass validation
			return "ok", "", 0, nil
		}
		msg := failureMessages[callIndex]
		callIndex++
		return "", msg, 1, nil
	}

	r.autoFixFn = func(startCommit string) error {
		return nil
	}

	var claudePrompts []string
	mockClaude.StreamRunFn = func(ctx context.Context, prompt string, systemPrompt string, output io.ReadWriter, heartbeat <-chan struct{}, stall <-chan struct{}) (*claude.Result, error) {
		claudePrompts = append(claudePrompts, prompt)
		return &claude.Result{Success: false, Output: "fix attempt"}, nil
	}

	bc := newBeadContext(t)
	bc.StartCommit = "abc123"
	bc.ParentCtx = context.Background()
	bc.MaxRetries = 2
	bc.MaxRetriesPerBead = 5

	_ = r.runValidationWithRecovery(context.Background(), bc)

	// bc.Result.Output should contain ALL failures (for logging)
	if !strings.Contains(bc.Result.Output, "FIRST_FAILURE_MESSAGE") {
		t.Errorf("bc.Result.Output should accumulate first failure for logging")
	}
	if !strings.Contains(bc.Result.Output, "SECOND_FAILURE_MESSAGE") {
		t.Errorf("bc.Result.Output should accumulate second failure for logging")
	}

	// But each Claude invocation should see ONLY the current failure
	// Expected failure: Currently all failures accumulate in PrevFailure
	if len(claudePrompts) < 1 {
		t.Fatal("Expected at least one Claude invocation")
	}

	for i, prompt := range claudePrompts {
		// Count how many distinct failure messages appear in this prompt
		failureCount := 0
		for _, msg := range failureMessages {
			if strings.Contains(prompt, msg) {
				failureCount++
			}
		}

		// Each prompt should contain at most ONE failure message (the current one)
		if failureCount > 1 {
			t.Errorf("Claude invocation %d: prompt contains %d accumulated failures; should contain only the current failure.\nPrompt excerpt: %s",
				i+1, failureCount, prompt[:min(len(prompt), 500)])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
