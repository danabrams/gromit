package escalation

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// FailureAnalyzer is the narrow interface for failure analysis.
type FailureAnalyzer interface {
	Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error)
}

// BeadClient is the narrow interface for bead operations needed by escalation.
type BeadClient interface {
	AddComment(id, comment string) error
}

// DecomposeFn decomposes a bead into sub-tasks.
type DecomposeFn func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error)

// CreateSubFn creates sub-beads from decomposed sub-tasks.
type CreateSubFn func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error

// InvocationResult captures the outcome of a single Claude invocation.
// This is a local type mirroring execution.InvocationResult to avoid
// importing the execution sibling package.
type InvocationResult struct {
	Result     *claude.Result
	StallFired bool
}

// InvokeFn executes a single Claude invocation. The facade wraps
// execution.Invoker.Execute and maps the result to this package's
// InvocationResult type.
type InvokeFn func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error)

// LogFn is a printf-style logging callback for escalation events.
type LogFn func(format string, args ...interface{})

// Handler manages retry loops, tier escalation, failure analysis, and decomposition.
type Handler struct {
	cfg         *config.Config
	analyzer    FailureAnalyzer
	beadClient  BeadClient
	decomposeFn DecomposeFn
	createSubFn CreateSubFn
	logFn       LogFn
}

// NewHandler creates a Handler with narrow dependency interfaces.
func NewHandler(cfg *config.Config, analyzer FailureAnalyzer, beadClient BeadClient, decomposeFn DecomposeFn, createSubFn CreateSubFn, logFn LogFn) *Handler {
	return &Handler{
		cfg:         cfg,
		analyzer:    analyzer,
		beadClient:  beadClient,
		decomposeFn: decomposeFn,
		createSubFn: createSubFn,
		logFn:       logFn,
	}
}

// log calls the logging callback if set.
func (h *Handler) log(format string, args ...interface{}) {
	if h.logFn != nil {
		h.logFn(format, args...)
	}
}

// EscalateTier updates the bead context to use a new tier after escalation.
// The router will select the concrete model on the next invocation.
func (h *Handler) EscalateTier(bc *runtypes.BeadContext, nextTier string) {
	bc.Result.Escalated = true
	bc.Tier = nextTier
	bc.RetriesThisModel = 0
	legacyModel := provider.TierToLegacyModel(nextTier)
	bc.Model = legacyModel
	bc.Result.Model = legacyModel
	bc.Result.EscalatedTo = legacyModel
	if bc.PromptCtx != nil {
		bc.PromptCtx.Model = legacyModel
	}
}

// HandleStallTimeout handles the case where a stall timeout was detected during execution.
// Returns true if the retry loop should continue, false if processBead should return.
func (h *Handler) HandleStallTimeout(ctx context.Context, bc *runtypes.BeadContext) (continueLoop bool) {
	bc.RetriesThisModel++
	bc.TotalRetriesThisBead++

	if bc.TotalRetriesThisBead > bc.MaxRetriesPerBead {
		h.log("Stall timeout: max retries per bead exceeded (%d/%d)", bc.TotalRetriesThisBead, bc.MaxRetriesPerBead)
		bc.Result.Error = fmt.Errorf("stall timeout: exceeded max retries per bead (%d)", bc.MaxRetriesPerBead)
		return false
	}

	if bc.RetriesThisModel <= bc.MaxRetries {
		h.log("Stall timeout detected, retrying with same model (attempt %d/%d)", bc.RetriesThisModel, bc.MaxRetries)
		return true
	}

	nextTier := h.cfg.NextEscalationTier(bc.Tier)
	if nextTier == "" {
		h.log("Stall timeout: no more tiers to escalate to, attempting decomposition")
		return h.AttemptDecomposition(ctx, bc, "stall timeout")
	}

	h.log("Stall timeout detected, escalating from %s to %s", bc.Tier, nextTier)
	h.EscalateTier(bc, nextTier)
	return true
}

// HandleEscalation tries to escalate to the next tier or decompose the task.
// Returns true if the retry loop should continue, false if processBead should return.
func (h *Handler) HandleEscalation(ctx context.Context, bc *runtypes.BeadContext, claudeResult *claude.Result) (continueLoop bool) {
	nextTier := h.cfg.NextEscalationTier(bc.Tier)
	if nextTier == "" {
		h.log("Build failed, no more tiers to escalate to - attempting decomposition")
		return h.AttemptDecomposition(ctx, bc, "build failed with all models")
	}

	if bc.TotalRetriesThisBead > bc.MaxRetriesPerBead {
		h.log("Cannot escalate: max retries per bead reached (%d/%d)", bc.TotalRetriesThisBead, bc.MaxRetriesPerBead)
		bc.Result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", bc.MaxRetriesPerBead)
		return false
	}

	h.log("Escalating from tier %s to %s", bc.Tier, nextTier)
	h.EscalateTier(bc, nextTier)
	return true
}

// AnalyzeAndHandleFailure runs failure analysis and decides whether to retry, escalate, or stop.
// Returns true if the retry loop should continue, false if processBead should return.
func (h *Handler) AnalyzeAndHandleFailure(ctx context.Context, bc *runtypes.BeadContext, claudeResult *claude.Result) (continueLoop bool) {
	h.log("Build failed, running failure analysis...")
	analysis, err := h.analyzer.Analyze(ctx, bc.Bead, claudeResult.Output)
	if err != nil {
		h.log("Warning: failure analysis failed: %v", err)
		return h.HandleEscalation(ctx, bc, claudeResult)
	}
	if analysis == nil {
		h.log("Warning: failure analysis returned no result")
		return h.HandleEscalation(ctx, bc, claudeResult)
	}

	h.log("Analysis: category=%s, recoverable=%v", analysis.Category, analysis.Recoverable)
	h.log("Root cause: %s", analysis.RootCause)

	if analysis.Category == analyzer.CategoryUnclearSpec {
		bc.Result.Error = fmt.Errorf("spec unclear: %s - needs human review", analysis.RootCause)
		return false
	}

	if analysis.Category == analyzer.CategoryTaskTooComplex {
		comment := fmt.Sprintf("Task too complex: %s\n\nThis task needs to be broken down into smaller, more manageable pieces.", analysis.RootCause)
		_ = h.beadClient.AddComment(bc.Bead.ID, comment) // best-effort; failure doesn't block escalation
		bc.Result.Error = fmt.Errorf("task too complex: %s - needs breakdown", analysis.RootCause)
		return false
	}

	if analysis.Recoverable && bc.RetriesThisModel < bc.MaxRetries {
		bc.RetriesThisModel++
		bc.TotalRetriesThisBead++

		if bc.TotalRetriesThisBead > bc.MaxRetriesPerBead {
			h.log("Max retries per bead exceeded (%d/%d)", bc.TotalRetriesThisBead, bc.MaxRetriesPerBead)
			bc.Result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", bc.MaxRetriesPerBead)
			return false
		}

		h.log("Failure is recoverable, retrying (attempt %d/%d)", bc.RetriesThisModel, bc.MaxRetries)
		return true
	}

	if analysis.Recoverable {
		h.log("Retry limit reached for model %s (%d attempts)", bc.Model, bc.RetriesThisModel)
	}

	return h.HandleEscalation(ctx, bc, claudeResult)
}

// AttemptDecomposition tries to decompose the task into sub-beads.
// On success, sets result.Decomposed=true. On failure, sets result.Error.
// Always returns false (processBead should return after this).
func (h *Handler) AttemptDecomposition(ctx context.Context, bc *runtypes.BeadContext, failureReason string) (continueLoop bool) {
	if h.decomposeFn == nil {
		bc.Result.Error = fmt.Errorf("%s and decomposition not available", failureReason)
		return false
	}

	h.log("Attempting to decompose task after: %s", failureReason)
	subTasks, err := h.decomposeFn(ctx, bc.Bead)
	if err != nil {
		h.log("Decomposition failed: %v", err)
		bc.Result.Error = fmt.Errorf("%s and decomposition failed: %w", failureReason, err)
		return false
	}

	if h.createSubFn == nil {
		bc.Result.Error = fmt.Errorf("%s decomposition succeeded but sub-bead creation not available", failureReason)
		return false
	}

	if err := h.createSubFn(ctx, bc.Bead, subTasks); err != nil {
		h.log("Failed to create sub-beads: %v", err)
		bc.Result.Error = fmt.Errorf("%s decomposition succeeded but failed to create sub-beads: %w", failureReason, err)
		return false
	}

	h.log("Task successfully decomposed into %d sub-tasks", len(subTasks))
	bc.Result.Decomposed = true
	return false
}

// ExecuteWithRetry runs the build loop with retry and escalation logic.
// Returns true if the build succeeded, false otherwise.
func (h *Handler) ExecuteWithRetry(ctx context.Context, bc *runtypes.BeadContext, invokeFn InvokeFn) bool {
	for {
		// Check for context cancellation before each invocation
		select {
		case <-ctx.Done():
			bc.Result.Error = ctx.Err()
			return false
		default:
		}

		invResult, err := invokeFn(ctx, bc, bc.BuildPrompt)

		if err != nil {
			if invResult != nil && invResult.StallFired && ctx.Err() == nil {
				bc.Result.TimeoutType = "stall"
				if h.HandleStallTimeout(ctx, bc) {
					continue
				}
				return false
			}
			bc.Result.Error = fmt.Errorf("claude invocation: %w", err)
			return false
		}

		if invResult == nil || invResult.Result == nil {
			bc.Result.Error = fmt.Errorf("claude returned nil result")
			return false
		}

		claudeResult := invResult.Result
		bc.Result.Output = claudeResult.Output

		if claudeResult.Success {
			return true
		}

		// Check for context cancellation before analysis
		select {
		case <-ctx.Done():
			bc.Result.Error = ctx.Err()
			return false
		default:
		}

		// Analyze failure and decide: retry, escalate, or stop
		if h.AnalyzeAndHandleFailure(ctx, bc, claudeResult) {
			continue
		}
		return false
	}
}
