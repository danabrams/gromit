package escalation

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/logger"
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
	_ = ctx
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

	return h.handleTimeoutEscalationOrFail(bc, "stall timeout")
}

// HandleInvocationTimeout escalates once on invocation timeout and, when already
// at the highest tier, attempts decomposition as the terminal recovery option.
func (h *Handler) HandleInvocationTimeout(ctx context.Context, bc *runtypes.BeadContext) (continueLoop bool) {
	if h.handleTimeoutEscalationOrFail(bc, "invocation timeout") {
		return true
	}
	if bc == nil || bc.Result == nil || bc.Result.Error == nil {
		return false
	}
	if bc.Result.Error.Error() != "invocation timeout: no higher tier available" {
		return false
	}

	decomposeCtx := firstNonNilContext(bc.ParentCtx, ctx)
	if decomposeCtx.Err() != nil {
		bc.Result.Error = fmt.Errorf("invocation timeout: no higher tier available (decomposition skipped: parent context canceled: %w)", decomposeCtx.Err())
		return false
	}
	return h.AttemptDecomposition(decomposeCtx, bc, "invocation timeout and no higher tier available")
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

func (h *Handler) handleTimeoutEscalationOrFail(bc *runtypes.BeadContext, failureLabel string) bool {
	if bc == nil || bc.Result == nil {
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

func (h *Handler) HandleBeadTimeout(bc *runtypes.BeadContext) (continueLoop bool) {
	if h.handleTimeoutEscalationOrFail(bc, "bead timeout") {
		return true
	}

	failureReason := fmt.Sprintf("bead timeout: exceeded %v total processing time", bc.BeadTimeout)
	decomposeCtx := firstNonNilContext(bc.ParentCtx)
	if decomposeCtx.Err() != nil {
		bc.Result.Error = fmt.Errorf("%s (decomposition skipped: parent context canceled: %w)", failureReason, decomposeCtx.Err())
		return false
	}
	return h.AttemptDecomposition(decomposeCtx, bc, failureReason)
}

func firstNonNilContext(contexts ...context.Context) context.Context {
	for _, ctx := range contexts {
		if ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func (h *Handler) checkRetryBudgetBeforeAttempt(ctx context.Context, bc *runtypes.BeadContext) (handled bool, err error) {
	if bc == nil {
		return false, fmt.Errorf("bead context is nil")
	}
	if bc.MaxAttemptsPerBead > 0 && bc.AttemptsThisBead >= bc.MaxAttemptsPerBead {
		return false, fmt.Errorf("retry budget exceeded: attempts %d/%d", bc.AttemptsThisBead, bc.MaxAttemptsPerBead)
	}
	if bc.BeadTimeout > 0 && !bc.BeadStartTime.IsZero() {
		elapsed := time.Since(bc.BeadStartTime)
		if elapsed >= bc.BeadTimeout {
			return false, fmt.Errorf("retry budget exceeded: bead wall-clock %s reached (timeout=%s)", elapsed.Round(time.Second), bc.BeadTimeout)
		}
	}
	if h != nil && h.cfg != nil {
		tokenBudgetCap := h.cfg.Claude.TokenBudgetForModel(bc.Model)
		if tokenBudgetCap > 0 && bc.CumulativeInputTokens > tokenBudgetCap {
			failureReason := fmt.Sprintf(
				"retry budget exceeded: cumulative input tokens %d/%d",
				bc.CumulativeInputTokens,
				tokenBudgetCap,
			)
			decomposeCtx := firstNonNilContext(bc.ParentCtx, ctx)
			if decomposeCtx.Err() != nil {
				return false, fmt.Errorf("%s (decomposition skipped: parent context canceled: %w)", failureReason, decomposeCtx.Err())
			}
			h.AttemptDecomposition(decomposeCtx, bc, failureReason)
			return true, nil
		}
	}
	return false, nil
}

func (h *Handler) checkRetryBudgetAfterFailure(bc *runtypes.BeadContext) error {
	if bc == nil {
		return fmt.Errorf("bead context is nil")
	}
	if bc.MaxAttemptsPerBead > 0 && bc.AttemptsThisBead >= bc.MaxAttemptsPerBead {
		return fmt.Errorf("retry budget exceeded after failed attempt: %d/%d", bc.AttemptsThisBead, bc.MaxAttemptsPerBead)
	}
	if bc.BeadTimeout > 0 && !bc.BeadStartTime.IsZero() {
		elapsed := time.Since(bc.BeadStartTime)
		if elapsed >= bc.BeadTimeout {
			return fmt.Errorf("retry budget exceeded after failed attempt: bead wall-clock %s reached (timeout=%s)", elapsed.Round(time.Second), bc.BeadTimeout)
		}
	}
	return nil
}

func (h *Handler) setBuildFailurePhase(bc *runtypes.BeadContext) {
	if bc == nil || bc.Result == nil {
		return
	}
	bc.Result.FailurePhase = failurephase.Build
}

func (h *Handler) setBuildTimeoutFailurePhase(bc *runtypes.BeadContext) {
	if bc == nil || bc.Result == nil {
		return
	}
	bc.Result.FailurePhase = failurephase.Timeout
	bc.Result.TimeoutPhase = "build"
}

func (h *Handler) addInvocationTokensToCumulative(bc *runtypes.BeadContext) {
	if bc == nil || bc.Result == nil {
		return
	}
	bc.CumulativeInputTokens += bc.Result.InputTokens
	bc.CumulativeOutputTokens += bc.Result.OutputTokens
}

func (h *Handler) applyTriageClassification(bc *runtypes.BeadContext, triageResult *TriageResult) {
	if bc == nil || bc.Result == nil || triageResult == nil {
		return
	}
	bc.Result.FailureLayer = string(triageResult.Layer)
	bc.Result.FailureSubCat = triageResult.SubCategory
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
func (h *Handler) HandleEscalation(ctx context.Context, bc *runtypes.BeadContext, claudeResult *claude.Result) (continueLoop bool) {
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

func (h *Handler) shouldEscalateAsSpecialCause(bc *runtypes.BeadContext) bool {
	if bc == nil {
		return true
	}
	// Repeated failures on the same bead are treated as special cause.
	if bc.RetriesThisModel > 0 {
		return true
	}
	limit, ok := h.readModelBuildFailureControlLimit(bc.Model)
	if !ok {
		return false
	}
	return limit.Latest > limit.UCL || limit.Latest < limit.LCL
}

func (h *Handler) readModelBuildFailureControlLimit(model string) (logger.TrendControlLimit, bool) {
	if h == nil || h.cfg == nil {
		return logger.TrendControlLimit{}, false
	}
	trendPath := filepath.Join(h.cfg.Paths.GromitDir, "metrics", "process_trend.json")
	trend, err := logger.ReadProcessTrend(trendPath)
	if err != nil || trend == nil {
		return logger.TrendControlLimit{}, false
	}
	modelKey := modelStratumPrefix + strings.ToLower(strings.TrimSpace(model))
	limits, ok := trend.StratifiedControlLimits[modelKey]
	if !ok {
		return logger.TrendControlLimit{}, false
	}
	for _, limit := range limits {
		if limit.Metric == metricBuildFailureRate {
			return limit, true
		}
	}
	return logger.TrendControlLimit{}, false
}

func (h *Handler) retryCommonCauseFailure(bc *runtypes.BeadContext, failureContext string) bool {
	if bc == nil || bc.Result == nil {
		return false
	}
	l1RetryCap := h.resolveL1RetryCap(bc)
	if bc.RetriesThisModel >= l1RetryCap {
		return false
	}
	bc.RetriesThisModel++
	bc.TotalRetriesThisBead++
	if bc.TotalRetriesThisBead > bc.MaxRetriesPerBead {
		h.log("Max retries per bead exceeded (%d/%d)", bc.TotalRetriesThisBead, bc.MaxRetriesPerBead)
		bc.Result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", bc.MaxRetriesPerBead)
		return false
	}
	if bc.PromptCtx != nil {
		bc.PromptCtx.IsRetry = true
		bc.PromptCtx.FailureContext = h.truncateFailureContext(failureContext)
	}
	h.log("Common-cause failure: retrying on same tier (attempt %d/%d)", bc.RetriesThisModel, l1RetryCap)
	return true
}

// AnalyzeAndHandleFailure runs failure analysis and decides whether to retry, escalate, or stop.
// Returns true if the retry loop should continue, false if processBead should return.
func (h *Handler) AnalyzeAndHandleFailure(ctx context.Context, bc *runtypes.BeadContext, claudeResult *claude.Result) (continueLoop bool) {
	h.log("Build failed, running failure analysis...")
	analysis, err := h.analyzer.Analyze(ctx, bc.Bead, claudeResult.Output)
	if err != nil {
		h.log("Warning: failure analysis failed: %v", err)
		if !h.shouldEscalateAsSpecialCause(bc) && h.retryCommonCauseFailure(bc, claudeResult.Output) {
			return true
		}
		return h.HandleEscalation(ctx, bc, claudeResult)
	}
	if analysis == nil {
		h.log("Warning: failure analysis returned no result")
		if !h.shouldEscalateAsSpecialCause(bc) && h.retryCommonCauseFailure(bc, claudeResult.Output) {
			return true
		}
		return h.HandleEscalation(ctx, bc, claudeResult)
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
		return h.HandleEscalation(ctx, bc, claudeResult)
	}

	if analysis.Recoverable {
		// L1 bounded autonomous retries.
		if !bc.Result.Escalated {
			l1RetryCap := h.resolveL1RetryCap(bc)

			if bc.RetriesThisModel < l1RetryCap {
				bc.RetriesThisModel++
				bc.TotalRetriesThisBead++

				if bc.TotalRetriesThisBead > bc.MaxRetriesPerBead {
					h.log("Max retries per bead exceeded (%d/%d)", bc.TotalRetriesThisBead, bc.MaxRetriesPerBead)
					bc.Result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", bc.MaxRetriesPerBead)
					return false
				}

				if bc.PromptCtx != nil {
					bc.PromptCtx.IsRetry = true
					bc.PromptCtx.FailureContext = h.truncateFailureContext(analysis.Suggestion)
				}

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

		if invResult == nil || invResult.Result == nil {
			bc.Result.Error = fmt.Errorf("claude returned nil result")
			h.setBuildFailurePhase(bc)
			return false
		}

		h.addInvocationTokensToCumulative(bc)

		claudeResult := invResult.Result
		bc.Result.Output = claudeResult.Output

		if claudeResult.Success {
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
		if h.AnalyzeAndHandleFailure(ctx, bc, claudeResult) {
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
