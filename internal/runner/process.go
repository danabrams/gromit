package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/preflight"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// errATDDAlreadyDone is returned by verifyTestsFailWithRetry when acceptance
// tests pass before implementation after retry. This signals that the work is
// already done (e.g., a sibling bead completed it), not that the tests are bad.
var errATDDAlreadyDone = errors.New("atdd: acceptance tests pass — work already done")

// setupBeadContext validates runner state, sets up timeouts, captures git state,
// fetches parent bead, and selects the initial model.
func (r *Runner) setupBeadContext(ctx context.Context, b *bead.Bead, iteration int, runDeadline time.Time, scopeEstimate *prompt.ScopeEstimate) (*runtypes.BeadContext, context.Context, context.CancelFunc, error) {
	if r.cfg == nil {
		return nil, nil, nil, fmt.Errorf("runner config is nil")
	}
	if r.beads == nil {
		return nil, nil, nil, fmt.Errorf("runner beads client is nil")
	}
	if r.renderer == nil {
		return nil, nil, nil, fmt.Errorf("runner renderer is nil")
	}
	if r.router == nil {
		return nil, nil, nil, fmt.Errorf("runner router is nil")
	}

	tier := escalation.SelectTier(r.cfg, b)
	model := escalation.SelectModel(r.cfg, b) // legacy model name for display/timeouts

	_, _, _, beadTimeoutSec := r.cfg.Claude.TimeoutsForModel(model)
	beadTimeout := time.Duration(beadTimeoutSec) * time.Second
	beadCtx, beadCancel := context.WithTimeout(ctx, beadTimeout)

	startCommit, err := getGitHead()
	if err != nil {
		r.log("Warning: could not capture git HEAD: %v", err)
		startCommit = ""
	}

	parent, err := r.beads.GetParent(b)
	if err != nil {
		r.log("Warning: failed to get parent bead: %v", err)
	}

	bc := &runtypes.BeadContext{
		Bead:              b,
		Parent:            parent,
		Result:            &IterationResult{BeadID: b.ID, BeadTitle: b.Title, Model: model},
		Model:             model, // legacy model name, will be updated by router
		Tier:              tier,
		StartCommit:       startCommit,
		Iteration:         iteration,
		MaxRetries:        r.cfg.Escalation.MaxRetriesPerModel,
		MaxRetriesPerBead: r.cfg.Escalation.MaxRetriesPerBead,
		ParentCtx:         ctx,
		BeadTimeout:       beadTimeout,
		RunDeadline:       runDeadline,
		ScopeEstimate:     scopeEstimate,
	}

	// Preemptive escalation: if scope check is enabled, scope complexity is high,
	// and tier is medium, escalate to high before first invocation
	if r.cfg.ScopeCheck.Enabled && scopeEstimate != nil && scopeEstimate.Complexity == "high" && bc.Tier == provider.TierMedium {
		r.log("Scope check: complexity=high, preemptively escalating from medium to high tier")
		r.escalationHandler.EscalateTier(bc, provider.TierHigh)
	}

	return bc, beadCtx, beadCancel, nil
}

// buildPromptForBead performs scope checking (if enabled) and renders the build prompt.
func (r *Runner) buildPromptForBead(ctx context.Context, bc *runtypes.BeadContext, iteration int) error {
	promptCtx, err := r.renderer.BuildContext(bc.Bead, bc.Parent, iteration, bc.Model)
	if err != nil {
		return fmt.Errorf("building prompt context: %w", err)
	}
	if promptCtx == nil {
		return fmt.Errorf("building prompt context: returned nil")
	}
	bc.PromptCtx = promptCtx

	// Inject recent validation failure summaries (last 3) into prompt context
	if n := len(r.validationFailures); n > 0 {
		start := 0
		if n > 3 {
			start = n - 3
		}
		bc.PromptCtx.RecentValidationFailures = r.validationFailures[start:]
	}

	if r.cfg.ScopeCheck.Enabled {
		// Use cached scope estimate if available (from scope gate), otherwise call checkScope
		scopeEstimate := bc.ScopeEstimate
		if scopeEstimate == nil {
			scopeEstimate = r.checkScope(ctx, bc.Bead)
		}
		if scopeEstimate != nil {
			if scopeEstimate.Complexity == "high" {
				r.log("Scope check: complexity=high, auto-escalating to high tier")
				r.escalationHandler.EscalateTier(bc, provider.TierHigh)
			} else {
				r.log("Scope check: complexity=%s", scopeEstimate.Complexity)
			}
		}
	}

	buildPrompt, err := r.renderer.RenderBuild(bc.PromptCtx)
	if err != nil {
		return fmt.Errorf("rendering build prompt: %w", err)
	}
	bc.BuildPrompt = buildPrompt

	return nil
}

// executeClaudeInvocation runs a single Claude invocation with streaming, heartbeat,
// and stall detection. Returns the Claude result, stream stats, whether a stall was detected, and any error.
// Delegates the core invocation to execution.Invoker.Execute.
func (r *Runner) executeClaudeInvocation(ctx context.Context, bc *runtypes.BeadContext) (*claude.Result, *logger.StreamStats, bool, error) {
	if r == nil || r.invoker == nil {
		return nil, nil, false, fmt.Errorf("runner invoker is nil")
	}

	invResult, err := r.invoker.Execute(ctx, bc, bc.BuildPrompt)
	if err != nil && invResult == nil {
		return nil, nil, false, err
	}

	var result *claude.Result
	var stats *logger.StreamStats
	stallFired := false
	if invResult != nil {
		result = invResult.Result
		stats = invResult.Stats
		stallFired = invResult.StallFired
	}

	return result, stats, stallFired, err
}

// handleScopeTooLarge processes the scope-too-large signal from Claude.
// Always sets bc.Result.Error and returns false (stop processing).
func (r *Runner) handleScopeTooLarge(bc *runtypes.BeadContext, claudeResult *claude.Result, explanation string) {
	breakdown := claude.GetScopeTooLargeBreakdown(claudeResult)
	if breakdown == "" {
		breakdown = explanation
	}
	comment := fmt.Sprintf("Scope too large: %s\n\nThis task needs to be broken down into smaller, more manageable pieces.", breakdown)
	if err := r.beads.AddComment(bc.Bead.ID, comment); err != nil {
		r.log("Warning: failed to add comment to bead: %v", err)
	}

	// Extract synthetic learning for scope-too-large
	escalation.ExtractScopeTooLargeLearning(bc, explanation, r.renderer.GetLearningsFile())
	r.log("Synthetic learning extracted: Bead '%s' was too large for %s", bc.Bead.Title, bc.Model)

	bc.Result.Error = fmt.Errorf("scope too large: %s - needs breakdown", explanation)
}

// runAcceptanceTestsWithRetry runs the acceptance test phase with retry and escalation logic.
// Returns nil on success or error on failure.
func (r *Runner) runAcceptanceTestsWithRetry(ctx context.Context, bc *runtypes.BeadContext) error {
	retries := 0
	maxRetries := r.cfg.Escalation.MaxRetriesPerModel
	currentTier := bc.Tier

	for {
		if retries > 0 {
			r.log("Retrying acceptance tests (attempt %d/%d)...", retries+1, maxRetries+1)
		}

		err := r.runAcceptanceTests(ctx, bc)
		if err == nil {
			return nil
		}

		// Retry with same tier
		if retries < maxRetries {
			retries++
			// Could add failure analysis here if needed
			continue
		}

		// Escalate tier
		nextTier := r.cfg.NextEscalationTier(currentTier)
		if nextTier == "" {
			return fmt.Errorf("acceptance tests failed with all tiers: %w", err)
		}

		r.log("Escalating acceptance tests from tier %s to %s", currentTier, nextTier)
		r.escalationHandler.EscalateTier(bc, nextTier)
		currentTier = nextTier
		retries = 0
	}
}

// runAcceptanceTests runs the acceptance test phase for ATDD workflow.
// Uses the same heartbeat/stall detection pattern as executeClaudeInvocation.
// Returns nil on success or error on failure.
func (r *Runner) runAcceptanceTests(ctx context.Context, bc *runtypes.BeadContext) error {
	// Render acceptance tests prompt
	acceptancePrompt, err := r.renderer.RenderAcceptanceTests(bc.PromptCtx)
	if err != nil {
		return fmt.Errorf("rendering acceptance tests prompt: %w", err)
	}

	// Setup streaming stats and heartbeat monitoring
	stats, err := logger.NewStreamStats()
	if err != nil {
		return err
	}

	if r.router == nil {
		return fmt.Errorf("runner router is nil")
	}

	// Determine phase and use tier from bead context
	phase := "build"
	tier := bc.Tier

	// Select provider using router
	p, modelName := r.router.Select(phase, tier)
	if p == nil {
		return fmt.Errorf("no providers available for phase=%s tier=%s", phase, tier)
	}

	// Update bead context with router-selected model
	bc.Model = modelName
	// If this is an escalated invocation, update EscalatedTo with the concrete model name
	if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
		bc.Result.EscalatedTo = modelName
	}

	childCtx, childCancel := context.WithCancel(ctx)
	stallFired := false

	_, stallTimeoutSec, stallTimeoutActiveSec, _ := r.cfg.Claude.TimeoutsForModel(bc.Model)
	stallTimeout := time.Duration(stallTimeoutSec) * time.Second
	stallTimeoutActive := time.Duration(stallTimeoutActiveSec) * time.Second

	toolCallEvents := make(chan claude.ToolEvent, 10)

	stopHeartbeat := r.startHeartbeat(stats, stallTimeout, stallTimeoutActive, func() {
		stallFired = true
		childCancel()
	}, toolCallEvents)

	var handler claude.EventHandler
	if r.streamLogger != nil {
		sl := r.streamLogger
		handler = func(line []byte) {
			logger.ParseAndLogEvent(sl, stats, line)
		}
	}

	onToolCall := func(event claude.ToolEvent) {
		select {
		case toolCallEvents <- event:
		default:
		}
	}

	// Convert claude handler to provider handler type
	var providerHandler provider.EventHandler
	if handler != nil {
		providerHandler = provider.EventHandler(handler)
	}

	providerToolHandler := func(event provider.ToolEvent) {
		onToolCall(claude.ToolEvent{
			ToolName:  event.ToolName,
			FilePath:  event.FilePath,
			Timestamp: event.Timestamp,
		})
	}

	// Call provider.StreamRun with the tier
	providerResult, err := p.StreamRun(childCtx, acceptancePrompt, tier, r.output, providerHandler, providerToolHandler)

	// Check for usage limit error and retry with fallback provider
	if err != nil && p.IsUsageLimitError(providerResult, err) {
		r.router.MarkUnavailable(p.Name())

		// Retry with new provider
		p2, modelName2 := r.router.Select(phase, tier)
		if p2 != nil {
			bc.Model = modelName2
			// If this is an escalated invocation, update EscalatedTo with the concrete model name
			if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
				bc.Result.EscalatedTo = modelName2
			}

			// Retry the invocation with the fallback provider
			providerResult, err = p2.StreamRun(childCtx, acceptancePrompt, tier, r.output, providerHandler, providerToolHandler)
		}
	}

	// Convert provider.Result back to claude.Result for backward compatibility
	var claudeResult *claude.Result
	if providerResult != nil {
		claudeResult = &claude.Result{
			Success:  providerResult.Success,
			Output:   providerResult.Output,
			ExitCode: providerResult.ExitCode,
			Duration: providerResult.Duration,
			Model:    providerResult.Model,
		}
	}

	stopHeartbeat()
	childCancel()

	// Handle stall timeout
	if stallFired {
		return fmt.Errorf("stall timeout during acceptance tests")
	}

	// Handle invocation errors
	if err != nil {
		return fmt.Errorf("acceptance tests invocation: %w", err)
	}

	// Check if Claude succeeded
	if claudeResult == nil || !claudeResult.Success {
		return fmt.Errorf("acceptance tests failed")
	}

	return nil
}

// verifyTestsFailWithRetry runs the verify-tests-fail phase with retry logic.
// If tests pass (unexpected), retries once with analysis, then fails.
func (r *Runner) verifyTestsFailWithRetry(ctx context.Context, bc *runtypes.BeadContext) error {
	err := r.verifyTestsFail(ctx, bc)
	if err == nil {
		return nil // Tests failed as expected
	}

	// Tests passed before implementation - this is unexpected
	// Run failure analysis to understand why
	r.log("Unexpected: tests passed before implementation. Analyzing...")

	analysisTimeout := time.Duration(r.cfg.Claude.AnalysisTimeout) * time.Second
	analysisCtx, analysisCancel := context.WithTimeout(ctx, analysisTimeout)
	analysis, analyzeErr := r.analyzer.Analyze(analysisCtx, bc.Bead, err.Error())
	analysisCancel()

	if analyzeErr != nil {
		r.log("Warning: failure analysis failed: %v — treating as already done", analyzeErr)
		return errATDDAlreadyDone
	}

	// Retry acceptance tests once with analysis context
	r.log("Retrying acceptance tests with analysis context...")
	bc.PromptCtx.IsRetry = true
	bc.PromptCtx.FailureContext = analysis.Suggestion

	if retryErr := r.runAcceptanceTests(ctx, bc); retryErr != nil {
		return fmt.Errorf("acceptance tests retry failed: %w", retryErr)
	}

	// Verify tests fail again
	err = r.verifyTestsFail(ctx, bc)
	if err == nil {
		return nil // Tests now fail as expected
	}

	// Still passing after retry with analysis — check if this is a false positive
	// by examining the git diff. If only test files changed (no implementation),
	// the tests are likely checking existing behavior, not new behavior.
	if bc.StartCommit != "" {
		diff, diffErr := r.getDiff(bc.StartCommit)
		if diffErr == nil && isTestOnlyDiff(diff) {
			r.log("Tests pass but only test files changed — likely testing existing behavior, retrying...")
			bc.PromptCtx.IsRetry = true
			bc.PromptCtx.FailureContext = "Tests pass but no implementation code was changed — tests are likely checking existing behavior. Rewrite tests to assert on behavior that does not exist yet."

			if retryErr2 := r.runAcceptanceTests(ctx, bc); retryErr2 == nil {
				// Verify tests fail again after diff-aware retry
				if err2 := r.verifyTestsFail(ctx, bc); err2 == nil {
					return nil // Tests now fail as expected
				}
			}
			// If retry failed or tests still pass, fall through to errATDDAlreadyDone
		}
	}

	r.log("Acceptance tests pass after retry — work appears already done")
	return errATDDAlreadyDone
}

// verifyTestsFail runs validation and returns nil when validation fails (expected)
// or an error when validation passes (unexpected - tests should fail before implementation).
// This is used in the ATDD workflow to verify that acceptance tests fail before implementation.
func (r *Runner) verifyTestsFail(ctx context.Context, bc *runtypes.BeadContext) error {
	if !r.cfg.Validation.Enabled {
		return fmt.Errorf("validation is not enabled - cannot verify tests fail")
	}

	checker, err := preflight.NewChecker(r.cfg.Preflight, r.output)
	if err != nil {
		return fmt.Errorf("creating preflight checker: %w", err)
	}
	if err := checker.Check(r.cfg.Validation.Commands); err != nil {
		r.log("Warning: %v", err)
		return fmt.Errorf("preflight check failed: %w", err)
	}

	r.log("Verifying acceptance tests fail (as expected)...")

	valResult, err := r.runDirectValidationCheck(ctx, r.cfg.Validation.Commands, bc.PromptCtx.WorkDir)
	if err != nil {
		return fmt.Errorf("validation invocation: %w", err)
	}
	if valResult == nil {
		return fmt.Errorf("validation returned no result")
	}

	// In ATDD, we expect tests to FAIL before implementation
	if claude.IsValidationPassed(valResult) {
		r.log("\nUnexpected: acceptance tests passed before implementation")
		r.log("Tests should fail until implementation makes them pass")
		return fmt.Errorf("acceptance tests passed before implementation - tests may not be covering new behavior")
	}

	r.log("Acceptance tests failed as expected")
	return nil
}

// parseDiffFiles extracts file paths from git diff output.
// Returns a slice of file paths in the order they appear.
func parseDiffFiles(diff string) []string {
	if diff == "" {
		return nil
	}

	var files []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			// Extract file path from "diff --git a/path b/path"
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				filePath := strings.TrimPrefix(parts[3], "b/")
				files = append(files, filePath)
			}
		}
	}
	return files
}

// detectTouchedPackages extracts unique Go package paths from git diff output.
// Only considers .go files (excludes README.md, etc.).
// Returns a slice of package paths in the order they appear (deduplicated).
func detectTouchedPackages(diff string) []string {
	files := parseDiffFiles(diff)
	if len(files) == 0 {
		return []string{}
	}

	seen := make(map[string]bool)
	var packages []string

	for _, filePath := range files {
		// Only consider .go files
		if !strings.HasSuffix(filePath, ".go") {
			continue
		}

		// Extract package directory (everything except the filename)
		lastSlash := strings.LastIndex(filePath, "/")
		if lastSlash == -1 {
			// File is in root directory
			continue
		}
		pkgPath := filePath[:lastSlash]

		// Add to results if not already seen
		if !seen[pkgPath] {
			seen[pkgPath] = true
			packages = append(packages, pkgPath)
		}
	}

	return packages
}

// isTestOnlyDiff returns true if the diff is empty or only contains changes
// to test files (*_test.go). This is used to detect ATDD false positives where
// tests pass because they're checking existing behavior rather than new behavior.
func isTestOnlyDiff(diff string) bool {
	if strings.TrimSpace(diff) == "" {
		return true
	}

	files := parseDiffFiles(diff)
	for _, filePath := range files {
		if !strings.HasSuffix(filePath, "_test.go") {
			return false
		}
	}
	return true
}

// runDirectValidationCheck delegates to the validation.Runner's RunDirect method.
func (r *Runner) runDirectValidationCheck(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
	if r.validationRunner == nil {
		return nil, fmt.Errorf("validationRunner not wired — all constructors must wire validationRunner")
	}
	return r.validationRunner.RunDirect(ctx, commands, workDir)
}

// runRefactorWithRouter executes a refactor invocation using the router with automatic fallback.
// Returns the result and any error. This helper centralizes the router selection and usage limit
// fallback pattern used in both runRefactorPhase and handleRefactorValidationFailure.
func (r *Runner) runRefactorWithRouter(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
	if r.router == nil {
		return nil, fmt.Errorf("runner router is nil")
	}

	phase := "build"
	p, modelName := r.router.Select(phase, tier)
	if p == nil {
		return nil, fmt.Errorf("no providers available for phase=%s tier=%s", phase, tier)
	}

	r.log("Running refactor with model: %s", modelName)

	// Call provider.Run with the prompt
	providerResult, err := p.Run(ctx, prompt, tier)

	// Check for usage limit error and retry with fallback provider
	if err != nil && p.IsUsageLimitError(providerResult, err) {
		r.router.MarkUnavailable(p.Name())

		// Retry with new provider
		p2, modelName2 := r.router.Select(phase, tier)
		if p2 != nil {
			r.log("Retrying refactor with model: %s", modelName2)
			providerResult, err = p2.Run(ctx, prompt, tier)
			modelName = modelName2
		}
	}

	// Convert provider.Result to claude.Result for compatibility
	var claudeResult *claude.Result
	if providerResult != nil {
		claudeResult = &claude.Result{
			Success:  providerResult.Success,
			Output:   providerResult.Output,
			ExitCode: providerResult.ExitCode,
			Duration: providerResult.Duration,
			Model:    modelName,
		}
	}

	return claudeResult, err
}

// shouldRunRefactor determines whether the refactor phase should run based on
// bead complexity tier and number of files changed.
func (r *Runner) shouldRunRefactor(bc *runtypes.BeadContext, diff string) bool {
	// Skip refactor for haiku-tier beads
	if bc.Tier == provider.TierLow {
		r.log("Skipping refactor: haiku-tier bead")
		return false
	}

	// Check file count threshold
	minFiles := r.cfg.Refactor.MinFilesChanged
	if minFiles == 0 {
		// Threshold of 0 means always run refactor (no file count check)
		return true
	}

	filesChanged := countChangedFiles(diff)
	if filesChanged < minFiles {
		r.log("Skipping refactor: only %d files changed (threshold: %d)", filesChanged, minFiles)
		return false
	}

	return true
}

// runRefactorPhase runs the refactoring phase after validation passes.
// Returns nil on success or if refactoring is skipped. Does not return an error
// if refactoring fails - it logs a warning and continues (working code without
// refactoring is better than broken code).
func (r *Runner) runRefactorPhase(ctx context.Context, bc *runtypes.BeadContext) error {
	// Check if there are any changes to refactor
	diff, err := r.getDiff(bc.StartCommit)
	if err != nil {
		r.log("Warning: could not get git diff: %v", err)
		return nil // Skip refactoring, not an error
	}
	if diff == "" {
		r.log("No changes to refactor, skipping refactor phase")
		return nil
	}

	// Check if refactor should run based on complexity and file count
	if !r.shouldRunRefactor(bc, diff) {
		return nil
	}

	// Capture pre-refactor commit for potential revert
	preRefactorCommit, err := getGitHead()
	if err != nil {
		r.log("Warning: could not capture pre-refactor commit: %v", err)
		return nil // Skip refactoring, not an error
	}

	// Render refactor prompt
	refactorPrompt, err := r.renderer.RenderRefactor(bc.PromptCtx)
	if err != nil {
		r.log("Warning: could not render refactor prompt: %v", err)
		return nil // Skip refactoring, not an error
	}

	// Execute refactor using router
	claudeResult, err := r.runRefactorWithRouter(ctx, refactorPrompt, bc.Tier)
	if err != nil {
		r.log("Warning: refactor invocation failed: %v", err)
		return nil // Skip refactoring, not an error
	}
	if claudeResult == nil || !claudeResult.Success {
		r.log("Warning: refactor phase failed")
		return nil // Skip refactoring, not an error
	}

	r.log("Refactor phase complete, re-validating...")

	// Re-validate after refactoring
	if !r.cfg.Validation.Enabled {
		r.log("Validation not enabled, cannot verify refactoring")
		return nil
	}

	valResult, err := r.runDirectValidationCheck(ctx, r.cfg.Validation.Commands, bc.PromptCtx.WorkDir)
	if err != nil {
		r.log("Warning: refactor re-validation invocation failed: %v", err)
		return r.handleRefactorValidationFailure(ctx, bc, preRefactorCommit, "re-validation invocation failed")
	}

	if valResult == nil || !claude.IsValidationPassed(valResult) {
		return r.handleRefactorValidationFailure(ctx, bc, preRefactorCommit, "tests failed after refactoring")
	}

	r.log("Refactor re-validation passed")
	return nil
}

// handleRefactorValidationFailure reverts the refactor changes and retries once.
// Returns nil (not an error) after handling - refactor failures are non-blocking.
func (r *Runner) handleRefactorValidationFailure(ctx context.Context, bc *runtypes.BeadContext, preRefactorCommit string, reason string) error {
	r.log("Refactor validation failed: %s", reason)
	r.log("Reverting to pre-refactor state: %s", preRefactorCommit)

	// Revert to pre-refactor commit
	revertCmd := exec.Command("git", "reset", "--hard", preRefactorCommit)
	if err := revertCmd.Run(); err != nil {
		r.log("Warning: could not revert refactor changes: %v", err)
		return nil // Can't revert, but don't fail the bead
	}

	r.log("Reverted to pre-refactor state, retrying refactor once...")

	// Retry refactor with analysis context
	bc.PromptCtx.IsRetry = true
	bc.PromptCtx.FailureContext = fmt.Sprintf("Previous refactoring broke tests: %s. Be more conservative this time.", reason)

	refactorPrompt, err := r.renderer.RenderRefactor(bc.PromptCtx)
	if err != nil {
		r.log("Warning: could not render retry refactor prompt: %v", err)
		return nil // Skip refactoring, not an error
	}

	// Execute retry refactor using router
	claudeResult, err := r.runRefactorWithRouter(ctx, refactorPrompt, bc.Tier)
	if err != nil {
		r.log("Warning: retry refactor invocation failed: %v - skipping refactoring", err)
		return nil
	}
	if claudeResult == nil || !claudeResult.Success {
		r.log("Warning: retry refactor failed - skipping refactoring")
		return nil
	}

	r.log("Retry refactor complete, re-validating...")

	valResult, err := r.runDirectValidationCheck(ctx, r.cfg.Validation.Commands, bc.PromptCtx.WorkDir)

	if err != nil || valResult == nil || !claude.IsValidationPassed(valResult) {
		r.log("Warning: retry refactor also failed validation - skipping refactoring")
		// Revert again
		revertCmd := exec.Command("git", "reset", "--hard", preRefactorCommit)
		if err := revertCmd.Run(); err != nil {
			r.log("Warning: could not revert retry refactor changes: %v", err)
		}
		return nil
	}

	r.log("Retry refactor re-validation passed")
	return nil
}

// runValidation runs the validation step (tests/lint) after a successful build.
// Delegates core command execution and failure accumulation to the validation.Runner,
// then handles facade concerns: preflight, logging, failure analysis, and post-success stages.
func (r *Runner) runValidation(ctx context.Context, bc *runtypes.BeadContext) error {
	if !r.cfg.Validation.Enabled {
		return nil
	}

	checker, err := preflight.NewChecker(r.cfg.Preflight, r.output)
	if err != nil {
		return fmt.Errorf("creating preflight checker: %w", err)
	}
	if err := checker.Check(r.cfg.Validation.Commands); err != nil {
		r.log("Warning: %v", err)
		bc.Result.Validated = false
		return nil // Skip validation, not an error
	}

	r.log("Running validation commands directly...")

	// Capture output before validation to extract failure output afterward
	outputBefore := bc.Result.Output

	// Delegate core validation (command execution + failure accumulation) to validation.Runner
	valErr := r.validationRunner.Validate(ctx, bc)

	// Sync failure summaries from validation.Runner to facade
	r.validationFailures = r.validationRunner.Failures()

	if valErr != nil && errors.Is(valErr, validation.ErrValidationFailed) {
		// Extract the failure output appended by the validation runner
		failureOutput := strings.TrimPrefix(bc.Result.Output, outputBefore)
		failureOutput = strings.TrimPrefix(failureOutput, "\n\n=== VALIDATION OUTPUT ===\n")

		r.log("\nValidation failed. Output:")
		r.log("%s", failureOutput)

		logPath, logErr := logger.WriteValidationLog(r.cfg.Paths.Logs, failureOutput)
		if logErr != nil {
			r.log("Warning: could not save validation log: %v", logErr)
		} else {
			r.log("\nFull output saved to: %s", logPath)
		}

		if bc.StartCommit != "" {
			r.showPartialProgress(bc.Bead, bc.StartCommit)
		}

		// Run failure analysis (Claude only for failure interpretation)
		r.log("Running failure analysis...")
		valAnalysisCtx, valAnalysisCancel := context.WithTimeout(ctx, time.Duration(r.cfg.Claude.AnalysisTimeout)*time.Second)
		analysis, analyzeErr := r.analyzer.Analyze(valAnalysisCtx, bc.Bead, failureOutput)
		valAnalysisCancel()
		if analyzeErr == nil && analysis != nil && r.renderer != nil {
			escalation.ExtractLearning(bc, analysis, r.renderer.GetLearningsFile())
		}

		return errValidationFailed
	} else if valErr != nil {
		// Non-validation error (e.g., command execution failure)
		return valErr
	}

	r.log("Validation passed")

	// Update runner's touched packages map for learning extraction filtering
	if bc.TouchedPackages != nil && len(bc.TouchedPackages) > 0 {
		r.updateTouchedPackages(bc.TouchedPackages)
	}

	// Run post-success stages sequentially
	learningEnabled := r.cfg != nil && r.cfg.Loop.ShouldLearnFromSuccess()
	reviewEnabled := r.cfg != nil && r.cfg.Review.Enabled

	if learningEnabled && r.renderer != nil && r.router != nil {
		lf := r.renderer.GetLearningsFile()
		adapter := &successLearningRouterAdapter{r: r.router}
		escalation.ExtractSuccessLearning(ctx, bc, r.cfg, lf, adapter, r.log, r.touchedPackages)
	}

	if reviewEnabled {
		return r.runPostSuccessReview(ctx, bc)
	}

	return nil
}

// runValidationWithRecovery delegates validation and recovery entirely to the
// validation.Runner's RunWithRecovery method. The facade handles only
// orchestration concerns: preflight checking, logging, failure analysis,
// learning extraction, and post-success stages (review).
func (r *Runner) runValidationWithRecovery(ctx context.Context, bc *runtypes.BeadContext) error {
	if !r.cfg.Validation.Enabled {
		return nil
	}

	checker, err := preflight.NewChecker(r.cfg.Preflight, r.output)
	if err != nil {
		return fmt.Errorf("creating preflight checker: %w", err)
	}
	if err := checker.Check(r.cfg.Validation.Commands); err != nil {
		r.log("Warning: %v", err)
		bc.Result.Validated = false
		return nil // Skip validation, not an error
	}

	r.log("Running validation commands directly...")

	// Delegate core validation + recovery to validation.Runner
	valErr := r.validationRunner.RunWithRecovery(ctx, bc)

	// Sync failure summaries from validation.Runner to facade
	r.validationFailures = r.validationRunner.Failures()

	if valErr != nil && errors.Is(valErr, validation.ErrValidationFailed) {
		// Extract the failure output from bc.Result.Output for logging
		r.log("\nValidation failed.")

		logPath, logErr := logger.WriteValidationLog(r.cfg.Paths.Logs, bc.Result.Output)
		if logErr != nil {
			r.log("Warning: could not save validation log: %v", logErr)
		} else {
			r.log("\nFull output saved to: %s", logPath)
		}

		if bc.StartCommit != "" {
			r.showPartialProgress(bc.Bead, bc.StartCommit)
		}

		// Run failure analysis (Claude only for failure interpretation)
		r.log("Running failure analysis...")
		valAnalysisCtx, valAnalysisCancel := context.WithTimeout(ctx, time.Duration(r.cfg.Claude.AnalysisTimeout)*time.Second)
		analysis, analyzeErr := r.analyzer.Analyze(valAnalysisCtx, bc.Bead, bc.Result.Output)
		valAnalysisCancel()
		if analyzeErr == nil && analysis != nil && r.renderer != nil {
			escalation.ExtractLearning(bc, analysis, r.renderer.GetLearningsFile())
		}

		return errValidationFailed
	} else if valErr != nil {
		return valErr
	}

	r.log("Validation passed")

	// Update runner's touched packages map for learning extraction filtering
	if bc.TouchedPackages != nil && len(bc.TouchedPackages) > 0 {
		r.updateTouchedPackages(bc.TouchedPackages)
	}

	// Run post-success stages sequentially
	learningEnabled := r.cfg != nil && r.cfg.Loop.ShouldLearnFromSuccess()
	reviewEnabled := r.cfg != nil && r.cfg.Review.Enabled

	if learningEnabled && r.renderer != nil && r.router != nil {
		lf := r.renderer.GetLearningsFile()
		adapter := &successLearningRouterAdapter{r: r.router}
		escalation.ExtractSuccessLearning(ctx, bc, r.cfg, lf, adapter, r.log, r.touchedPackages)
	}

	if reviewEnabled {
		return r.runPostSuccessReview(ctx, bc)
	}

	return nil
}

// countChangedFiles counts the number of files in a git diff output.
func countChangedFiles(diff string) int {
	return len(parseDiffFiles(diff))
}

// runPostSuccessReview runs only the review stage (when learning is disabled).
func (r *Runner) runPostSuccessReview(ctx context.Context, bc *runtypes.BeadContext) error {
	reviewStart := time.Now()
	r.log("Running post-iteration review with model: %s", selectReviewModel(r.cfg, bc.Model))

	reviewResult, err := r.runLightReview(ctx, bc.Bead, bc.Parent, bc.StartCommit, bc.Model, bc.Iteration, bc.RunDeadline, bc.BuildProvider)
	if err != nil {
		r.log("Warning: review failed: %v", err)
		return nil // Review failure is non-blocking
	}

	if reviewResult != nil {
		r.log("Review: %s", reviewResult.Summary)

		// If fixes were applied, re-validate
		if len(reviewResult.FixesApplied) > 0 {
			r.log("Review applied %d fixes, re-validating...", len(reviewResult.FixesApplied))

			if r.cfg.Validation.Enabled {
				valResult, err := r.runDirectValidationCheck(ctx, r.cfg.Validation.Commands, bc.PromptCtx.WorkDir)
				if err != nil {
					return fmt.Errorf("review re-validation invocation: %w", err)
				}

				if valResult == nil || !claude.IsValidationPassed(valResult) {
					bc.Result.Output += "\n\n=== REVIEW RE-VALIDATION FAILED ===\n"
					if valResult != nil {
						bc.Result.Output += valResult.Output
					}
					bc.Result.ReviewBrokeValidation = true
					return fmt.Errorf("review fixes broke validation")
				}
				r.log("Re-validation passed")
			}
		}

		// Create beads/backlog from review findings
		beadsCreated, backlogCreated := r.applyReviewResult(reviewResult)

		// Log review result
		reviewDuration := time.Since(reviewStart)
		r.writeReviewLog(bc.Iteration, bc.Bead.ID, selectReviewModel(r.cfg, bc.Model), reviewResult, beadsCreated, backlogCreated, reviewDuration)
	}

	return nil
}
