package escalation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
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

const integrityUnsafeStateCategory analyzer.Category = "integrity_unsafe_state"
const failureContextTruncatedPrefix = "[truncated] "
const (
	triageSubCategoryBadPrompt = "bad_prompt"
	triageSubCategoryBadBead   = "bad_bead"
	modelStratumPrefix         = "model:"
	metricBuildFailureRate     = "rolling_build_failure_rate"
	processTrendMetricsDir     = "metrics"
	processTrendFileName       = "process_trend.json"
)

const firstTimeoutBudgetThreshold = 0.75

const (
	timeoutDecompositionOutcomeSuccess = "success"
	timeoutDecompositionOutcomeSkipped = "skipped"
	timeoutDecompositionOutcomeFailed  = "failed"
)

// InvokeFn executes a single Claude invocation. The facade wraps
// execution.Invoker.Execute and returns a runtypes.InvocationResult.
type InvokeFn func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error)

// LogFn is a printf-style logging callback for escalation events.
type LogFn func(format string, args ...interface{})

// ShowPartialProgressFn displays git diff and expected outputs on failure.
type ShowPartialProgressFn func(b *bead.Bead, startCommit string)

// Handler manages retry loops, tier escalation, failure analysis, and decomposition.
type Handler struct {
	cfg                   *config.Config
	analyzer              FailureAnalyzer
	beadClient            BeadClient
	decomposeFn           DecomposeFn
	createSubFn           CreateSubFn
	logFn                 LogFn
	showPartialProgressFn ShowPartialProgressFn
}

// NewHandler creates a Handler with narrow dependency interfaces.
func NewHandler(cfg *config.Config, analyzer FailureAnalyzer, beadClient BeadClient, decomposeFn DecomposeFn, createSubFn CreateSubFn, logFn LogFn, showPartialProgressFn ShowPartialProgressFn) *Handler {
	return &Handler{
		cfg:                   cfg,
		analyzer:              analyzer,
		beadClient:            beadClient,
		decomposeFn:           decomposeFn,
		createSubFn:           createSubFn,
		logFn:                 logFn,
		showPartialProgressFn: showPartialProgressFn,
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
	if bc.MaxRetriesPerBead > 0 && bc.TotalRetriesThisBead >= bc.MaxRetriesPerBead {
		h.log("Stall timeout: max retries per bead exceeded (%d/%d)", bc.TotalRetriesThisBead, bc.MaxRetriesPerBead)
		bc.Result.Error = fmt.Errorf("stall timeout: exceeded max retries per bead (%d)", bc.MaxRetriesPerBead)
		return false
	}

	hasToolActivity := bc != nil && bc.Result != nil && bc.Result.ToolCallCount > 0

	// Only allow a same-tier stall retry when there has been no tool activity,
	// and cap that to a single retry.
	if !hasToolActivity && !bc.StallRetryWithoutToolUsed {
		bc.StallRetryWithoutToolUsed = true
		bc.RetriesThisModel++
		bc.TotalRetriesThisBead++
		h.log("Stall timeout detected with no tool activity, retrying once on same tier")
		return true
	}

	return h.handleTimeoutEscalationOrFail(ctx, bc, "stall timeout")
}

// HandleInvocationTimeout applies timeout-first decomposition.
func (h *Handler) HandleInvocationTimeout(ctx context.Context, bc *runtypes.BeadContext) (continueLoop bool) {
	if bc == nil || bc.Result == nil {
		return false
	}
	now := time.Now().UTC()
	bc.Result.TimeoutDecompositionAttempted = true
	bc.Result.TimeoutDecompositionAttemptTime = now

	decomposeCtx := firstNonNilContext(bc.ParentCtx, ctx)
	if decomposeCtx.Err() != nil {
		bc.Result.TimeoutDecompositionOutcome = timeoutDecompositionOutcomeSkipped
		bc.Result.TimeoutDecompositionReason = fmt.Sprintf("invocation timeout skipped: parent context canceled: %v", decomposeCtx.Err())
		bc.Result.Error = fmt.Errorf("invocation timeout (decomposition skipped: parent context canceled: %w)", decomposeCtx.Err())
		return false
	}
	continueLoop = h.AttemptDecomposition(decomposeCtx, bc, "invocation timeout")
	bc.Result.TimeoutDecompositionSucceeded = bc.Result.Decomposed
	if bc.Result.Decomposed {
		bc.Result.TimeoutDecompositionOutcome = timeoutDecompositionOutcomeSuccess
		bc.Result.TimeoutDecompositionReason = "invocation timeout decomposition succeeded"
	} else {
		bc.Result.TimeoutDecompositionOutcome = timeoutDecompositionOutcomeFailed
		bc.Result.TimeoutDecompositionReason = fmt.Sprintf("invocation timeout decomposition failed: %v", bc.Result.Error)
	}
	return continueLoop
}

func (h *Handler) resolveL1RetryCap(bc *runtypes.BeadContext) int {
	l1RetryCap := h.cfg.Andon.L1RetryCap
	if l1RetryCap <= 0 {
		l1RetryCap = bc.MaxRetries
	}
	if l1RetryCap <= 0 {
		l1RetryCap = 1
	}
	return l1RetryCap
}

func (h *Handler) handleTimeoutEscalationOrFail(ctx context.Context, bc *runtypes.BeadContext, failureLabel string) bool {
	if bc == nil || bc.Result == nil {
		return false
	}
	if decision := h.handleFirstTimeoutDecomposition(ctx, bc, failureLabel); decision == firstTimeoutDecisionStop {
		return false
	}
	if bc.TimeoutEscalationsThisBead >= 1 {
		bc.Result.Error = fmt.Errorf("%s: timeout escalation limit reached (1 per bead)", failureLabel)
		return false
	}
	nextTier := h.cfg.NextEscalationTier(bc.Tier)
	if nextTier == "" {
		bc.Result.Error = fmt.Errorf("%s: no higher tier available", failureLabel)
		return false
	}
	bc.TimeoutEscalationsThisBead++
	h.log("%s detected, escalating from %s to %s", failureLabel, bc.Tier, nextTier)
	h.EscalateTier(bc, nextTier)
	return true
}

type firstTimeoutDecision int

const (
	firstTimeoutDecisionNone firstTimeoutDecision = iota
	firstTimeoutDecisionEscalate
	firstTimeoutDecisionStop
)

func (h *Handler) handleFirstTimeoutDecomposition(ctx context.Context, bc *runtypes.BeadContext, failureLabel string) firstTimeoutDecision {
	if bc == nil || bc.Result == nil {
		return firstTimeoutDecisionNone
	}
	if bc.TimeoutEscalationsThisBead > 0 {
		return firstTimeoutDecisionNone
	}
	if bc.Result.TimeoutDecompositionAttempted {
		return firstTimeoutDecisionNone
	}
	if h.decomposeFn == nil {
		now := time.Now().UTC()
		bc.Result.TimeoutDecompositionAttempted = true
		bc.Result.TimeoutDecompositionAttemptTime = now
		bc.Result.TimeoutDecompositionOutcome = timeoutDecompositionOutcomeSkipped
		bc.Result.TimeoutDecompositionReason = fmt.Sprintf("first timeout (%s) skipped: decomposition unavailable", failureLabel)
		return firstTimeoutDecisionEscalate
	}

	now := time.Now().UTC()
	bc.Result.TimeoutDecompositionAttempted = true
	bc.Result.TimeoutDecompositionAttemptTime = now

	if exceeded, usage := firstTimeoutBudgetUsage(bc); exceeded {
		percent := usage * 100
		if percent > 100 {
			percent = 100
		}
		bc.Result.TimeoutDecompositionOutcome = timeoutDecompositionOutcomeSkipped
		bc.Result.TimeoutDecompositionReason = fmt.Sprintf("first timeout (%s) skipped: %.0f%% of bead budget used before escalation", failureLabel, percent)
		return firstTimeoutDecisionEscalate
	}

	decomposeCtx := firstNonNilContext(bc.ParentCtx, ctx)
	failureReason := fmt.Sprintf("first timeout (%s)", failureLabel)
	if err := decomposeCtx.Err(); err != nil {
		bc.Result.TimeoutDecompositionOutcome = timeoutDecompositionOutcomeSkipped
		bc.Result.TimeoutDecompositionReason = fmt.Sprintf("%s skipped: parent context canceled: %v", failureReason, err)
		bc.Result.Error = fmt.Errorf("%s (decomposition skipped: parent context canceled: %w)", failureReason, err)
		return firstTimeoutDecisionStop
	}

	h.AttemptDecomposition(decomposeCtx, bc, failureReason)
	bc.Result.TimeoutDecompositionSucceeded = bc.Result.Decomposed
	if bc.Result.Decomposed {
		bc.Result.TimeoutDecompositionOutcome = timeoutDecompositionOutcomeSuccess
		bc.Result.TimeoutDecompositionReason = fmt.Sprintf("%s decomposition succeeded", failureReason)
	} else {
		bc.Result.TimeoutDecompositionOutcome = timeoutDecompositionOutcomeFailed
		bc.Result.TimeoutDecompositionReason = fmt.Sprintf("%s decomposition failed: %v", failureReason, bc.Result.Error)
	}

	return firstTimeoutDecisionStop
}

func firstTimeoutBudgetUsage(bc *runtypes.BeadContext) (bool, float64) {
	if bc == nil || bc.BeadTimeout <= 0 || bc.BeadStartTime.IsZero() {
		return false, 0
	}
	elapsed := time.Since(bc.BeadStartTime)
	if elapsed <= 0 {
		return false, 0
	}
	usage := float64(elapsed) / float64(bc.BeadTimeout)
	return usage >= firstTimeoutBudgetThreshold, usage
}

func (h *Handler) HandleBeadTimeout(bc *runtypes.BeadContext) (continueLoop bool) {
	failureReason := fmt.Sprintf("bead timeout: exceeded %v total processing time", bc.BeadTimeout)
	if bc != nil && bc.Result != nil {
		bc.Result.TimeoutDecompositionAttempted = true
	}
	decomposeCtx := firstNonNilContext(bc.ParentCtx)
	if decomposeCtx.Err() != nil {
		bc.Result.Error = fmt.Errorf("%s (decomposition skipped: parent context canceled: %w)", failureReason, decomposeCtx.Err())
		return false
	}
	continueLoop = h.AttemptDecomposition(decomposeCtx, bc, failureReason)
	if bc != nil && bc.Result != nil {
		bc.Result.TimeoutDecompositionSucceeded = bc.Result.Decomposed
	}
	return continueLoop
}

func (h *Handler) handleTriageResult(bc *runtypes.BeadContext, triageResult *TriageResult) (continueLoop bool, handled bool) {
	if triageResult == nil {
		return false, false
	}
	h.applyTriageClassification(bc, triageResult)
	switch triageResult.Layer {
	case LayerProviderTransport:
		return h.handleProviderTransportTriage(bc, triageResult)
	case LayerEnvironment:
		bc.Result.Error = fmt.Errorf("environment error: %s", triageResult.Detail)
		h.setBuildFailurePhase(bc)
		return false, true
	case LayerOrchestration:
		bc.Result.Error = h.orchestrationTriageError(triageResult)
		h.setBuildFailurePhase(bc)
		return false, true
	case LayerCode:
		// Fall through to existing analyzer flow for code-level failures.
		return false, false
	default:
		return false, false
	}
}

func (h *Handler) handleProviderTransportTriage(bc *runtypes.BeadContext, triageResult *TriageResult) (continueLoop bool, handled bool) {
	if triageResult.Retryable {
		bc.RetriesThisModel++
		bc.TotalRetriesThisBead++
		return true, true
	}
	bc.Result.Error = fmt.Errorf("provider authentication failed: check API credentials and provider access")
	h.setBuildFailurePhase(bc)
	return false, true
}

func (h *Handler) orchestrationTriageError(triageResult *TriageResult) error {
	switch triageResult.SubCategory {
	case triageSubCategoryBadPrompt:
		return fmt.Errorf("orchestration error: build prompt is empty")
	case triageSubCategoryBadBead:
		return fmt.Errorf("orchestration error: bead description is empty")
	default:
		return fmt.Errorf("orchestration error: %s", triageResult.Detail)
	}
}

func (h *Handler) truncateFailureContext(failureContext string) string {
	if h == nil || h.cfg == nil {
		return failureContext
	}
	maxChars := h.cfg.Claude.MaxFailureContextChars
	if maxChars <= 0 || len(failureContext) <= maxChars {
		return failureContext
	}
	if maxChars <= len(failureContextTruncatedPrefix) {
		return failureContextTruncatedPrefix[:maxChars]
	}
	tailLen := maxChars - len(failureContextTruncatedPrefix)
	return failureContextTruncatedPrefix + failureContext[len(failureContext)-tailLen:]
}

// HandleEscalation tries to escalate to the next tier or decompose the task.
// Returns true if the retry loop should continue, false if processBead should return.
func (h *Handler) HandleEscalation(ctx context.Context, bc *runtypes.BeadContext, providerResult *provider.Result) (continueLoop bool) {
	_ = providerResult
	nextTier := h.cfg.NextEscalationTier(bc.Tier)
	if nextTier == "" {
		h.log("Andon L4 option selected: no more tiers to escalate to, attempting decomposition")
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

func (h *Handler) retryOrEscalateWhenAnalysisUnavailable(ctx context.Context, bc *runtypes.BeadContext, providerResult *provider.Result) bool {
	if !h.shouldEscalateAsSpecialCause(bc) && h.retryCommonCauseFailure(bc, providerResult.Output) {
		return true
	}
	return h.HandleEscalation(ctx, bc, providerResult)
}

// AnalyzeAndHandleFailure runs failure analysis and decides whether to retry, escalate, or stop.
// Returns true if the retry loop should continue, false if processBead should return.
func (h *Handler) AnalyzeAndHandleFailure(ctx context.Context, bc *runtypes.BeadContext, providerResult *provider.Result) (continueLoop bool) {
	h.log("Build failed, running failure analysis...")
	analysis, err := h.analyzer.Analyze(ctx, bc.Bead, providerResult.Output)
	if err != nil {
		h.log("Warning: failure analysis failed: %v", err)
		return h.retryOrEscalateWhenAnalysisUnavailable(ctx, bc, providerResult)
	}
	if analysis == nil {
		h.log("Warning: failure analysis returned no result")
		return h.retryOrEscalateWhenAnalysisUnavailable(ctx, bc, providerResult)
	}

	h.log("Analysis: category=%s, recoverable=%v", analysis.Category, analysis.Recoverable)
	h.log("Root cause: %s", analysis.RootCause)
	bc.Result.FailureCategory = string(analysis.Category)

	if analysis.Category == integrityUnsafeStateCategory {
		bc.Result.Error = fmt.Errorf("L3 stop-line: integrity/unsafe-state - %s", analysis.RootCause)
		return false
	}

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

	if analysis.Category == analyzer.CategoryHardStopAction {
		if !bc.HardStopApproval.Approved {
			bc.Result.HardStopPendingApproval = true
			bc.Result.Error = fmt.Errorf("hard-stop action requires explicit approval: %s", analysis.RootCause)
			return false
		}

		bc.Result.HardStopPendingApproval = false
		return h.HandleEscalation(ctx, bc, providerResult)
	}

	if analysis.Recoverable {
		// L1 bounded autonomous retries.
		if !bc.Result.Escalated {
			l1RetryCap := h.resolveL1RetryCap(bc)

			if bc.RetriesThisModel < l1RetryCap {
				if !h.incrementRetryCounters(bc) {
					return false
				}
				h.setRetryPromptContext(bc, analysis.Suggestion)

				h.log("Andon L1: recoverable failure retrying (attempt %d/%d)", bc.RetriesThisModel, l1RetryCap)
				return true
			}

			// L1 exhausted -> transition to L2 via one escalation.
			nextTier := h.cfg.NextEscalationTier(bc.Tier)
			if nextTier == "" {
				h.log("Andon L4 option selected: recoverable failure exhausted L1 with no higher tier, attempting decomposition")
				return h.AttemptDecomposition(ctx, bc, "recoverable failure exhausted L1 and no higher tier available")
			}
			h.log("Andon L1->L2: escalating from %s to %s", bc.Tier, nextTier)
			h.EscalateTier(bc, nextTier)
			return true
		}

		// Recoverable failure persisted after L2 transition.
		bc.Result.Error = fmt.Errorf("L3 stop-line: recoverable failure persisted after L2 bounded recovery")
		return false
	}

	if !h.shouldEscalateAsSpecialCause(bc) && h.retryCommonCauseFailure(bc, analysis.Suggestion) {
		return true
	}
	return h.HandleEscalation(ctx, bc, providerResult)
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
	bc.Result.Error = nil
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
			h.setBuildTimeoutFailurePhase(bc)
			return false
		default:
		}

		// Check retry gate: block same-scope retries after timeout without decomposition/escalation
		if gateErr := h.CheckRetryGate(bc); gateErr != nil {
			bc.Result.Error = gateErr
			h.setBuildTimeoutFailurePhase(bc)
			return false
		}

		budgetHandled, budgetErr := h.checkRetryBudgetBeforeAttempt(ctx, bc)
		if budgetErr != nil {
			bc.Result.Error = budgetErr
			h.setBuildFailurePhase(bc)
			return false
		}
		if budgetHandled {
			if bc.Result != nil && bc.Result.Error != nil && bc.Result.FailurePhase == "" {
				h.setBuildFailurePhase(bc)
			}
			return false
		}

		// Check for proactive decomposition on high-risk beads at 60% elapsed budget
		if !h.CheckProactiveDecomposition(ctx, bc) {
			if bc.Result != nil && bc.Result.FailurePhase == "" {
				h.setBuildFailurePhase(bc)
			}
			return false
		}

		// Check pre-execution scope gate on first attempt only
		if bc.AttemptsThisBead == 0 {
			if !h.CheckPreExecutionScopeGate(ctx, bc) {
				if bc.Result != nil && bc.Result.FailurePhase == "" {
					h.setBuildFailurePhase(bc)
				}
				return false
			}
		}

		bc.AttemptsThisBead++

		invResult, err := invokeFn(ctx, bc, bc.BuildPrompt)

		if err != nil {
			if invResult != nil && invResult.StallFired && ctx.Err() == nil {
				bc.Result.TimeoutType = "stall"
				if h.HandleStallTimeout(ctx, bc) {
					continue
				}
				h.setBuildTimeoutFailurePhase(bc)
				return false
			}
			if invResult != nil && invResult.TimeoutType == "invocation" {
				bc.Result.TimeoutType = invResult.TimeoutType
				if h.HandleInvocationTimeout(ctx, bc) {
					continue
				}
				h.setBuildTimeoutFailurePhase(bc)
				return false
			}
			if invResult != nil && invResult.TimeoutType == "bead" {
				bc.Result.TimeoutType = "bead"
				if h.HandleBeadTimeout(bc) {
					continue
				}
				h.setBuildTimeoutFailurePhase(bc)
				return false
			}
			bc.Result.Error = fmt.Errorf("claude invocation: %w", err)
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				h.setBuildTimeoutFailurePhase(bc)
			} else {
				h.setBuildFailurePhase(bc)
			}
			return false
		}

		if invResult == nil {
			bc.Result.Error = fmt.Errorf("claude returned nil result")
			h.setBuildFailurePhase(bc)
			return false
		}

		h.addInvocationTokensToCumulative(bc)

		providerResult := invResult.ProviderResult
		if providerResult == nil && invResult.Result != nil {
			claudeResult := invResult.Result
			providerResult = &provider.Result{
				Success:      claudeResult.Success,
				Output:       claudeResult.Output,
				Model:        claudeResult.Model,
				CostUSD:      claudeResult.CostUSD,
				InputTokens:  claudeResult.InputTokens,
				OutputTokens: claudeResult.OutputTokens,
			}
		}
		if providerResult == nil {
			bc.Result.Error = fmt.Errorf("claude returned nil result")
			h.setBuildFailurePhase(bc)
			return false
		}
		bc.Result.Output = providerResult.Output

		if providerResult.Success {
			return true
		}

		if budgetErr := h.checkRetryBudgetAfterFailure(bc); budgetErr != nil {
			bc.Result.Error = budgetErr
			h.setBuildFailurePhase(bc)
			return false
		}

		// Show partial progress on build failure (git diff --stat)
		if h.showPartialProgressFn != nil && bc.StartCommit != "" {
			h.showPartialProgressFn(bc.Bead, bc.StartCommit)
		}

		// Check for context cancellation before analysis
		select {
		case <-ctx.Done():
			bc.Result.Error = ctx.Err()
			h.setBuildTimeoutFailurePhase(bc)
			return false
		default:
		}

		triageResult := Triage(invResult, bc)
		if continueLoop, handled := h.handleTriageResult(bc, triageResult); handled {
			if continueLoop {
				continue
			}
			return false
		}

		// Analyze failure and decide: retry, escalate, or stop
		if h.AnalyzeAndHandleFailure(ctx, bc, providerResult) {
			continue
		}
		if bc.Result != nil && bc.Result.FailurePhase == "" {
			h.setBuildFailurePhase(bc)
		}
		return false
	}
}

// ExecuteWithRetryWithEscalation runs the build loop with optional escalation behavior; when escalationEnabled is false, it performs retries without tier escalation.
func (h *Handler) ExecuteWithRetryWithEscalation(ctx context.Context, bc *runtypes.BeadContext, invokeFn InvokeFn, escalationEnabled bool) bool {
	if h == nil {
		return false
	}
	if escalationEnabled {
		return h.ExecuteWithRetry(ctx, bc, invokeFn)
	}
	if h.cfg == nil {
		return h.ExecuteWithRetry(ctx, bc, invokeFn)
	}
	cfgCopy := *h.cfg
	cfgCopy.Escalation = h.cfg.Escalation
	cfgCopy.Escalation.Enabled = false

	handlerCopy := *h
	handlerCopy.cfg = &cfgCopy
	return handlerCopy.ExecuteWithRetry(ctx, bc, invokeFn)
}
