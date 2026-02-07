package runner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/preflight"
	"github.com/danabrams/gromit/internal/prompt"
)

// beadContext holds the shared state for processing a single bead.
// It is passed between the extracted methods that compose processBead.
type beadContext struct {
	bead        *bead.Bead
	parent      *bead.Bead
	result      *IterationResult
	model       string
	promptCtx   *prompt.Context
	buildPrompt string
	startCommit string
	iteration   int

	// Retry tracking
	retriesThisModel     int
	totalRetriesThisBead int
	maxRetries           int
	maxRetriesPerBead    int

	// Context management
	parentCtx   context.Context // original context (to distinguish bead timeout from Ctrl+C)
	beadTimeout time.Duration
	runDeadline time.Time // run deadline for time-budget awareness
}

// setupBeadContext validates runner state, sets up timeouts, captures git state,
// fetches parent bead, and selects the initial model.
func (r *Runner) setupBeadContext(ctx context.Context, b *bead.Bead, iteration int, runDeadline time.Time) (*beadContext, context.Context, context.CancelFunc, error) {
	if r.cfg == nil {
		return nil, nil, nil, fmt.Errorf("runner config is nil")
	}
	if r.beads == nil {
		return nil, nil, nil, fmt.Errorf("runner beads client is nil")
	}
	if r.renderer == nil {
		return nil, nil, nil, fmt.Errorf("runner renderer is nil")
	}
	if r.claude == nil {
		return nil, nil, nil, fmt.Errorf("runner claude client is nil")
	}

	beadTimeout := time.Duration(r.cfg.Claude.BeadTimeout) * time.Second
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

	model := r.selectModel(b)

	bc := &beadContext{
		bead:              b,
		parent:            parent,
		result:            &IterationResult{BeadID: b.ID, BeadTitle: b.Title, Model: model},
		model:             model,
		startCommit:       startCommit,
		iteration:         iteration,
		maxRetries:        r.cfg.Escalation.MaxRetriesPerModel,
		maxRetriesPerBead: r.cfg.Escalation.MaxRetriesPerBead,
		parentCtx:         ctx,
		beadTimeout:       beadTimeout,
		runDeadline:       runDeadline,
	}

	return bc, beadCtx, beadCancel, nil
}

// buildPromptForBead performs scope checking (if enabled) and renders the build prompt.
func (r *Runner) buildPromptForBead(ctx context.Context, bc *beadContext, iteration int) error {
	promptCtx, err := r.renderer.BuildContext(bc.bead, bc.parent, iteration, bc.model)
	if err != nil {
		return fmt.Errorf("building prompt context: %w", err)
	}
	if promptCtx == nil {
		return fmt.Errorf("building prompt context: returned nil")
	}
	bc.promptCtx = promptCtx

	if r.cfg.ScopeCheck.Enabled {
		scopeEstimate := r.checkScope(ctx, bc.bead)
		if scopeEstimate != nil {
			if scopeEstimate.Complexity == "high" {
				r.log("Scope check: complexity=high, auto-escalating to opus")
				bc.model = "opus"
				bc.result.Model = bc.model
				bc.promptCtx.Model = bc.model
			} else {
				r.log("Scope check: complexity=%s", scopeEstimate.Complexity)
			}
		}
	}

	buildPrompt, err := r.renderer.RenderBuild(bc.promptCtx)
	if err != nil {
		return fmt.Errorf("rendering build prompt: %w", err)
	}
	bc.buildPrompt = buildPrompt

	return nil
}

// executeClaudeInvocation runs a single Claude invocation with streaming, heartbeat,
// and stall detection. Returns the Claude result, whether a stall was detected, and any error.
func (r *Runner) executeClaudeInvocation(ctx context.Context, bc *beadContext) (*claude.Result, bool, error) {
	stats, err := logger.NewStreamStats()
	if err != nil {
		return nil, false, err
	}

	childCtx, childCancel := context.WithCancel(ctx)
	stallFired := false

	stallTimeout := time.Duration(r.cfg.Claude.StallTimeout) * time.Second
	stallTimeoutActive := time.Duration(r.cfg.Claude.StallTimeoutActive) * time.Second

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

	claudeResult, err := r.claude.StreamRun(childCtx, bc.buildPrompt, bc.model, r.output, handler, onToolCall)
	stopHeartbeat()
	childCancel()

	return claudeResult, stallFired, err
}

// handleStallTimeout handles the case where a stall timeout was detected during execution.
// Returns true if the retry loop should continue, false if processBead should return.
func (r *Runner) handleStallTimeout(ctx context.Context, bc *beadContext) (continueLoop bool) {
	bc.retriesThisModel++
	bc.totalRetriesThisBead++

	if bc.totalRetriesThisBead > bc.maxRetriesPerBead {
		r.log("Max retries per bead exceeded (%d/%d)", bc.totalRetriesThisBead, bc.maxRetriesPerBead)
		bc.result.Error = fmt.Errorf("stall timeout: exceeded max retries per bead (%d)", bc.maxRetriesPerBead)
		return false
	}

	if bc.retriesThisModel <= bc.maxRetries {
		r.log("Stall timeout, retrying with same model (%d/%d)...", bc.retriesThisModel, bc.maxRetries)
		return true
	}

	r.log("Stall timeout, retries exhausted for %s", bc.model)
	nextModel := r.cfg.NextEscalationModel(bc.model)
	if nextModel == "" {
		r.log("Stall timeout, no more models to escalate to - attempting decomposition")
		return r.attemptDecomposition(ctx, bc, "stall timeout")
	}

	r.log("Escalating from %s to %s after stall", bc.model, nextModel)
	r.escalateModel(bc, nextModel)

	var err error
	bc.buildPrompt, err = r.renderer.RenderBuild(bc.promptCtx)
	if err != nil {
		bc.result.Error = fmt.Errorf("rendering retry prompt: %w", err)
		return false
	}
	return true
}

// handleScopeTooLarge processes the scope-too-large signal from Claude.
// Always sets bc.result.Error and returns false (stop processing).
func (r *Runner) handleScopeTooLarge(bc *beadContext, claudeResult *claude.Result, explanation string) {
	breakdown := claude.GetScopeTooLargeBreakdown(claudeResult)
	if breakdown == "" {
		breakdown = explanation
	}
	comment := fmt.Sprintf("Scope too large: %s\n\nThis task needs to be broken down into smaller, more manageable pieces.", breakdown)
	if err := r.beads.AddComment(bc.bead.ID, comment); err != nil {
		r.log("Warning: failed to add comment to bead: %v", err)
	}

	// Extract synthetic learning for scope-too-large
	r.extractScopeTooLargeLearning(bc, explanation)

	bc.result.Error = fmt.Errorf("scope too large: %s - needs breakdown", explanation)
}

// extractLearning saves a learning from failure analysis to the LEARNINGS.md file.
func (r *Runner) extractLearning(bc *beadContext, analysis *analyzer.Analysis) {
	if analysis.Learning == nil {
		return
	}
	r.log("Learning extracted: %s", *analysis.Learning)
	lf := r.renderer.GetLearningsFile()
	if lf == nil {
		return
	}
	learning, err := lf.Add(bc.bead.ID, *analysis.Learning, analysis.LearningCategory())
	if err != nil {
		r.log("Warning: failed to add learning: %v", err)
	} else if learning != nil {
		r.log("Learning added to LEARNINGS.md")
	}
}

// extractScopeTooLargeLearning saves a synthetic learning for scope-too-large failures.
func (r *Runner) extractScopeTooLargeLearning(bc *beadContext, explanation string) {
	if bc == nil || bc.bead == nil {
		return
	}
	lf := r.renderer.GetLearningsFile()
	if lf == nil {
		return
	}

	// Generate synthetic learning message
	learning := fmt.Sprintf("Bead '%s' was too large for %s — consider splitting beads with more than 3 acceptance criteria", bc.bead.Title, bc.model)
	r.log("Synthetic learning extracted: %s", learning)

	_, err := lf.Add(bc.bead.ID, learning, "patterns")
	if err != nil {
		r.log("Warning: failed to add synthetic learning: %v", err)
	} else {
		r.log("Synthetic learning added to LEARNINGS.md")
	}
}

// extractTimeoutLearning saves a synthetic learning for timeout failures.
func (r *Runner) extractTimeoutLearning(bc *beadContext) {
	if bc == nil || bc.bead == nil {
		return
	}
	lf := r.renderer.GetLearningsFile()
	if lf == nil {
		return
	}

	// Generate synthetic learning message
	learning := fmt.Sprintf("Bead '%s' timed out on %s — may need simpler scope or higher model tier", bc.bead.Title, bc.model)
	r.log("Synthetic learning extracted: %s", learning)

	_, err := lf.Add(bc.bead.ID, learning, "patterns")
	if err != nil {
		r.log("Warning: failed to add synthetic learning: %v", err)
	} else {
		r.log("Synthetic learning added to LEARNINGS.md")
	}
}

// extractSuccessLearning calls Claude to extract a learning from a successful iteration.
// Uses haiku with a lightweight prompt asking what codebase pattern/convention/gotcha was encountered.
func (r *Runner) extractSuccessLearning(ctx context.Context, bc *beadContext) {
	if r == nil || bc == nil {
		return
	}
	if r.cfg == nil || !r.cfg.Loop.ShouldLearnFromSuccess() {
		return
	}
	if r.claude == nil || r.renderer == nil {
		return
	}

	// Build a brief summary of what was done (use bead title + first line of description)
	summary := bc.bead.Title
	if bc.bead.Description != "" {
		// Take first line only
		lines := strings.Split(bc.bead.Description, "\n")
		if len(lines) > 0 && lines[0] != "" {
			summary = bc.bead.Title + ": " + lines[0]
		}
	}

	learnCtx := &prompt.LearnContext{
		BeadID:          bc.bead.ID,
		BeadTitle:       bc.bead.Title,
		BeadDescription: bc.bead.Description,
		Summary:         summary,
	}

	learnPrompt, err := r.renderer.RenderLearn(learnCtx)
	if err != nil {
		r.log("Warning: failed to render learning prompt: %v", err)
		return
	}

	// Call Claude with haiku (lightweight, fast)
	// Use a short timeout - learning extraction should be quick
	learnTimeout := 30 * time.Second
	learnCtxTimeout, cancel := context.WithTimeout(ctx, learnTimeout)
	defer cancel()

	claudeResult, err := r.claude.Run(learnCtxTimeout, learnPrompt, "haiku")
	if err != nil {
		// Learning extraction is optional - don't fail the iteration
		return
	}
	if claudeResult == nil || !claudeResult.Success {
		return
	}

	// Parse the result
	successLearning, err := prompt.ParseSuccessLearning(claudeResult.Output)
	if err != nil {
		return
	}

	if successLearning.Learning == nil {
		return
	}

	r.log("Success learning extracted: %s", *successLearning.Learning)
	lf := r.renderer.GetLearningsFile()
	if lf == nil {
		return
	}

	learning, err := lf.Add(bc.bead.ID, *successLearning.Learning, successLearning.Category)
	if err != nil {
		r.log("Warning: failed to add success learning: %v", err)
	} else if learning != nil {
		r.log("Success learning added to LEARNINGS.md")
	}
}

// analyzeAndHandleFailure runs failure analysis and decides whether to retry, escalate, or stop.
// Returns true if the retry loop should continue, false if processBead should return.
func (r *Runner) analyzeAndHandleFailure(ctx context.Context, bc *beadContext, claudeResult *claude.Result) (continueLoop bool) {
	r.log("Build failed, running failure analysis...")
	analysisTimeout := time.Duration(r.cfg.Claude.AnalysisTimeout) * time.Second
	analysisCtx, analysisCancel := context.WithTimeout(ctx, analysisTimeout)
	analysis, err := r.analyzer.Analyze(analysisCtx, bc.bead, claudeResult.Output)
	analysisCancel()

	if err != nil {
		r.log("Warning: failure analysis failed: %v", err)
		return r.handleEscalation(ctx, bc, claudeResult)
	}
	if analysis == nil {
		r.log("Warning: failure analysis returned no result")
		return r.handleEscalation(ctx, bc, claudeResult)
	}

	r.log("Analysis: category=%s, recoverable=%v", analysis.Category, analysis.Recoverable)
	r.log("Root cause: %s", analysis.RootCause)

	r.extractLearning(bc, analysis)

	if analysis.Category == analyzer.CategoryUnclearSpec {
		bc.result.Error = fmt.Errorf("spec unclear: %s - needs human review", analysis.RootCause)
		return false
	}

	if analysis.Category == analyzer.CategoryTaskTooComplex {
		comment := fmt.Sprintf("Task too complex: %s\n\nThis task needs to be broken down into smaller, more manageable pieces.", analysis.RootCause)
		if err := r.beads.AddComment(bc.bead.ID, comment); err != nil {
			r.log("Warning: failed to add comment to bead: %v", err)
		}
		r.extractLearning(bc, analysis)
		bc.result.Error = fmt.Errorf("task too complex: %s - needs breakdown", analysis.RootCause)
		return false
	}

	if analysis.Recoverable && bc.retriesThisModel < bc.maxRetries {
		bc.retriesThisModel++
		bc.totalRetriesThisBead++

		if bc.totalRetriesThisBead > bc.maxRetriesPerBead {
			r.log("Max retries per bead exceeded (%d/%d)", bc.totalRetriesThisBead, bc.maxRetriesPerBead)
			bc.result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", bc.maxRetriesPerBead)
			return false
		}

		r.log("Failure is recoverable, retrying with context (attempt %d/%d)...", bc.retriesThisModel, bc.maxRetries)
		bc.promptCtx.IsRetry = true
		bc.promptCtx.PrevFailure = claudeResult.Output
		bc.promptCtx.FailureContext = analysis.Suggestion

		var err error
		bc.buildPrompt, err = r.renderer.RenderBuild(bc.promptCtx)
		if err != nil {
			bc.result.Error = fmt.Errorf("rendering retry prompt: %w", err)
			return false
		}
		return true
	}

	if analysis.Recoverable {
		r.log("Retry limit reached for model %s (%d attempts)", bc.model, bc.retriesThisModel)
	}

	return r.handleEscalation(ctx, bc, claudeResult)
}

// handleEscalation tries to escalate to the next model or decompose the task.
// Returns true if the retry loop should continue, false if processBead should return.
func (r *Runner) handleEscalation(ctx context.Context, bc *beadContext, claudeResult *claude.Result) (continueLoop bool) {
	nextModel := r.cfg.NextEscalationModel(bc.model)
	if nextModel == "" {
		r.log("Build failed, no more models to escalate to - attempting decomposition")
		return r.attemptDecomposition(ctx, bc, "build failed with all models")
	}

	if bc.totalRetriesThisBead >= bc.maxRetriesPerBead {
		r.log("Cannot escalate: max retries per bead reached (%d/%d)", bc.totalRetriesThisBead, bc.maxRetriesPerBead)
		if bc.startCommit != "" {
			r.showPartialProgress(bc.bead, bc.startCommit)
		}
		bc.result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", bc.maxRetriesPerBead)
		return false
	}

	r.log("Escalating from %s to %s", bc.model, nextModel)
	r.escalateModel(bc, nextModel)

	bc.promptCtx.IsRetry = true
	bc.promptCtx.PrevFailure = claudeResult.Output
	bc.promptCtx.Model = bc.model

	var err error
	bc.buildPrompt, err = r.renderer.RenderBuild(bc.promptCtx)
	if err != nil {
		bc.result.Error = fmt.Errorf("rendering retry prompt: %w", err)
		return false
	}
	return true
}

// attemptDecomposition tries to decompose the task into sub-beads.
// On success, sets result.Decomposed=true. On failure, sets result.Error.
// Always returns false (processBead should return after this).
func (r *Runner) attemptDecomposition(ctx context.Context, bc *beadContext, failureReason string) (continueLoop bool) {
	subTasks, err := r.DecomposeTask(ctx, bc.bead)
	if err != nil {
		r.log("Decomposition failed: %v, falling back to error", err)
		if bc.startCommit != "" {
			r.showPartialProgress(bc.bead, bc.startCommit)
		}
		bc.result.Error = fmt.Errorf("%s and decomposition failed: %w", failureReason, err)
		return false
	}

	if err := r.CreateSubBeads(ctx, bc.bead, subTasks); err != nil {
		r.log("Failed to create sub-beads: %v", err)
		if bc.startCommit != "" {
			r.showPartialProgress(bc.bead, bc.startCommit)
		}
		bc.result.Error = fmt.Errorf("%s decomposition succeeded but failed to create sub-beads: %w", failureReason, err)
		return false
	}

	r.log("Task successfully decomposed into %d sub-tasks", len(subTasks))
	bc.result.Decomposed = true
	return false
}

// escalateModel updates the bead context to use a new model after escalation.
func (r *Runner) escalateModel(bc *beadContext, nextModel string) {
	bc.result.Escalated = true
	bc.result.EscalatedTo = nextModel
	bc.model = nextModel
	bc.retriesThisModel = 0
	bc.result.Model = bc.model
	bc.promptCtx.Model = bc.model
}

// runAcceptanceTestsWithRetry runs the acceptance test phase with retry and escalation logic.
// Returns nil on success or error on failure.
func (r *Runner) runAcceptanceTestsWithRetry(ctx context.Context, bc *beadContext) error {
	retries := 0
	maxRetries := r.cfg.Escalation.MaxRetriesPerModel
	currentModel := bc.model

	for {
		if retries > 0 {
			r.log("Retrying acceptance tests (attempt %d/%d)...", retries+1, maxRetries+1)
		}

		err := r.runAcceptanceTests(ctx, bc)
		if err == nil {
			return nil
		}

		// Retry with same model
		if retries < maxRetries {
			retries++
			// Could add failure analysis here if needed
			continue
		}

		// Escalate model
		nextModel := r.cfg.NextEscalationModel(currentModel)
		if nextModel == "" {
			return fmt.Errorf("acceptance tests failed with all models: %w", err)
		}

		r.log("Escalating acceptance tests from %s to %s", currentModel, nextModel)
		currentModel = nextModel
		bc.model = nextModel
		bc.promptCtx.Model = nextModel
		retries = 0
	}
}

// runAcceptanceTests runs the acceptance test phase for ATDD workflow.
// Uses the same heartbeat/stall detection pattern as executeClaudeInvocation.
// Returns nil on success or error on failure.
func (r *Runner) runAcceptanceTests(ctx context.Context, bc *beadContext) error {
	// Render acceptance tests prompt
	acceptancePrompt, err := r.renderer.RenderAcceptanceTests(bc.promptCtx)
	if err != nil {
		return fmt.Errorf("rendering acceptance tests prompt: %w", err)
	}

	// Setup streaming stats and heartbeat monitoring
	stats, err := logger.NewStreamStats()
	if err != nil {
		return err
	}

	childCtx, childCancel := context.WithCancel(ctx)
	stallFired := false

	stallTimeout := time.Duration(r.cfg.Claude.StallTimeout) * time.Second
	stallTimeoutActive := time.Duration(r.cfg.Claude.StallTimeoutActive) * time.Second

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

	// Call Claude with the acceptance tests prompt
	claudeResult, err := r.claude.StreamRun(childCtx, acceptancePrompt, bc.model, r.output, handler, onToolCall)
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
func (r *Runner) verifyTestsFailWithRetry(ctx context.Context, bc *beadContext) error {
	err := r.verifyTestsFail(ctx, bc)
	if err == nil {
		return nil // Tests failed as expected
	}

	// Tests passed before implementation - this is unexpected
	// Run failure analysis to understand why
	r.log("Unexpected: tests passed before implementation. Analyzing...")

	analysisTimeout := time.Duration(r.cfg.Claude.AnalysisTimeout) * time.Second
	analysisCtx, analysisCancel := context.WithTimeout(ctx, analysisTimeout)
	analysis, analyzeErr := r.analyzer.Analyze(analysisCtx, bc.bead, err.Error())
	analysisCancel()

	if analyzeErr != nil {
		r.log("Warning: failure analysis failed: %v", analyzeErr)
		return fmt.Errorf("tests passed before implementation (analysis failed): %w", err)
	}

	// Retry acceptance tests once with analysis context
	r.log("Retrying acceptance tests with analysis context...")
	bc.promptCtx.IsRetry = true
	bc.promptCtx.FailureContext = analysis.Suggestion

	if retryErr := r.runAcceptanceTests(ctx, bc); retryErr != nil {
		return fmt.Errorf("acceptance tests retry failed: %w", retryErr)
	}

	// Verify tests fail again
	err = r.verifyTestsFail(ctx, bc)
	if err == nil {
		return nil // Tests now fail as expected
	}

	// Still passing - fail the bead
	return fmt.Errorf("acceptance tests passed before implementation after retry - tests may not be covering new behavior")
}

// verifyTestsFail runs validation and returns nil when validation fails (expected)
// or an error when validation passes (unexpected - tests should fail before implementation).
// This is used in the ATDD workflow to verify that acceptance tests fail before implementation.
func (r *Runner) verifyTestsFail(ctx context.Context, bc *beadContext) error {
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

	valResult, err := r.claude.RunValidation(
		ctx,
		r.cfg.Validation.Commands,
		r.cfg.Models.Validation,
		bc.promptCtx.WorkDir,
	)
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

// runRefactorPhase runs the refactoring phase after validation passes.
// Returns nil on success or if refactoring is skipped. Does not return an error
// if refactoring fails - it logs a warning and continues (working code without
// refactoring is better than broken code).
func (r *Runner) runRefactorPhase(ctx context.Context, bc *beadContext) error {
	// Check if there are any changes to refactor
	diff, err := getGitDiff(bc.startCommit)
	if err != nil {
		r.log("Warning: could not get git diff: %v", err)
		return nil // Skip refactoring, not an error
	}
	if diff == "" {
		r.log("No changes to refactor, skipping refactor phase")
		return nil
	}

	// Capture pre-refactor commit for potential revert
	preRefactorCommit, err := getGitHead()
	if err != nil {
		r.log("Warning: could not capture pre-refactor commit: %v", err)
		return nil // Skip refactoring, not an error
	}

	// Render refactor prompt
	refactorPrompt, err := r.renderer.RenderRefactor(bc.promptCtx)
	if err != nil {
		r.log("Warning: could not render refactor prompt: %v", err)
		return nil // Skip refactoring, not an error
	}

	r.log("Running refactor phase with model: %s", bc.model)

	// Call Claude with refactor prompt (use Run, not StreamRun - refactoring is typically simpler)
	claudeResult, err := r.claude.Run(ctx, refactorPrompt, bc.model)
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

	valResult, err := r.claude.RunValidation(
		ctx,
		r.cfg.Validation.Commands,
		r.cfg.Models.Validation,
		bc.promptCtx.WorkDir,
	)
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
func (r *Runner) handleRefactorValidationFailure(ctx context.Context, bc *beadContext, preRefactorCommit string, reason string) error {
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
	bc.promptCtx.IsRetry = true
	bc.promptCtx.FailureContext = fmt.Sprintf("Previous refactoring broke tests: %s. Be more conservative this time.", reason)

	refactorPrompt, err := r.renderer.RenderRefactor(bc.promptCtx)
	if err != nil {
		r.log("Warning: could not render retry refactor prompt: %v", err)
		return nil // Skip refactoring, not an error
	}

	claudeResult, err := r.claude.Run(ctx, refactorPrompt, bc.model)
	if err != nil {
		r.log("Warning: retry refactor invocation failed: %v - skipping refactoring", err)
		return nil
	}
	if claudeResult == nil || !claudeResult.Success {
		r.log("Warning: retry refactor failed - skipping refactoring")
		return nil
	}

	r.log("Retry refactor complete, re-validating...")

	// Re-validate after retry refactor
	valResult, err := r.claude.RunValidation(
		ctx,
		r.cfg.Validation.Commands,
		r.cfg.Models.Validation,
		bc.promptCtx.WorkDir,
	)
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
// Returns an error if validation fails or encounters an error.
func (r *Runner) runValidation(ctx context.Context, bc *beadContext) error {
	if !r.cfg.Validation.Enabled {
		return nil
	}

	checker, err := preflight.NewChecker(r.cfg.Preflight, r.output)
	if err != nil {
		return fmt.Errorf("creating preflight checker: %w", err)
	}
	if err := checker.Check(r.cfg.Validation.Commands); err != nil {
		r.log("Warning: %v", err)
		bc.result.Validated = false
		return nil // Skip validation, not an error
	}

	r.log("Running validation with model: %s", r.cfg.Models.Validation)

	valResult, err := r.claude.RunValidation(
		ctx,
		r.cfg.Validation.Commands,
		r.cfg.Models.Validation,
		bc.promptCtx.WorkDir,
	)
	if err != nil {
		return fmt.Errorf("validation invocation: %w", err)
	}
	if valResult == nil {
		return fmt.Errorf("validation returned no result")
	}

	if !claude.IsValidationPassed(valResult) {
		r.log("\nValidation failed. Output:")
		r.log("%s", valResult.Output)

		logPath, err := logger.WriteValidationLog(r.cfg.Paths.Logs, valResult.Output)
		if err != nil {
			r.log("Warning: could not save validation log: %v", err)
		} else {
			r.log("\nFull output saved to: %s", logPath)
		}

		if bc.startCommit != "" {
			r.showPartialProgress(bc.bead, bc.startCommit)
		}

		r.log("Running failure analysis...")
		valAnalysisCtx, valAnalysisCancel := context.WithTimeout(ctx, time.Duration(r.cfg.Claude.AnalysisTimeout)*time.Second)
		analysis, err := r.analyzer.Analyze(valAnalysisCtx, bc.bead, valResult.Output)
		valAnalysisCancel()
		if err == nil && analysis != nil {
			r.extractLearning(bc, analysis)
		}

		bc.result.Output += "\n\n=== VALIDATION OUTPUT ===\n" + valResult.Output
		return fmt.Errorf("validation failed")
	}

	bc.result.Validated = true
	r.log("Validation passed")

	// Extract success learning if enabled
	r.extractSuccessLearning(ctx, bc)

	// Run light review if enabled
	if r.cfg.Review.Enabled {
		reviewStart := time.Now()
		r.log("Running post-iteration review with model: %s", selectReviewModel(r.cfg, bc.model))

		reviewResult, err := r.runLightReview(ctx, bc.bead, bc.parent, bc.startCommit, bc.model, bc.iteration, bc.runDeadline)
		if err != nil {
			r.log("Warning: review failed: %v", err)
			// Review failure is non-blocking — continue
		} else if reviewResult != nil {
			r.log("Review: %s", reviewResult.Summary)

			// If fixes were applied, re-validate
			if len(reviewResult.FixesApplied) > 0 {
				r.log("Review applied %d fixes, re-validating...", len(reviewResult.FixesApplied))

				if r.cfg.Validation.Enabled {
					valResult, err := r.claude.RunValidation(ctx, r.cfg.Validation.Commands, r.cfg.Models.Validation, bc.promptCtx.WorkDir)
					if err != nil {
						return fmt.Errorf("review re-validation invocation: %w", err)
					}
					if valResult == nil || !claude.IsValidationPassed(valResult) {
						bc.result.Output += "\n\n=== REVIEW RE-VALIDATION FAILED ===\n"
						if valResult != nil {
							bc.result.Output += valResult.Output
						}
						return fmt.Errorf("review fixes broke validation")
					}
					r.log("Re-validation passed")
				}
			}

			// Create beads/backlog from review findings
			beadsCreated, backlogCreated := r.applyReviewResult(reviewResult)

			// Log review result
			reviewDuration := time.Since(reviewStart)
			r.writeReviewLog(bc.iteration, bc.bead.ID, selectReviewModel(r.cfg, bc.model), reviewResult, beadsCreated, backlogCreated, reviewDuration)
		}
	}

	return nil
}
