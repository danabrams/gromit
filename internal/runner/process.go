package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/ralph-runner/internal/analyzer"
	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/logger"
	"github.com/danabrams/ralph-runner/internal/preflight"
	"github.com/danabrams/ralph-runner/internal/prompt"
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

	// Retry tracking
	retriesThisModel    int
	totalRetriesThisBead int
	maxRetries          int
	maxRetriesPerBead   int

	// Context management
	parentCtx   context.Context // original context (to distinguish bead timeout from Ctrl+C)
	beadTimeout time.Duration
}

// setupBeadContext validates runner state, sets up timeouts, captures git state,
// fetches parent bead, and selects the initial model.
func (r *Runner) setupBeadContext(ctx context.Context, b *bead.Bead, iteration int) (*beadContext, context.Context, context.CancelFunc, error) {
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
		maxRetries:        r.cfg.Escalation.MaxRetriesPerModel,
		maxRetriesPerBead: r.cfg.Escalation.MaxRetriesPerBead,
		parentCtx:         ctx,
		beadTimeout:       beadTimeout,
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
	return nil
}
