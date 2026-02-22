package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func (r *Runner) applyATDDSkipPolicies(bc *runtypes.BeadContext, atddActive bool) bool {
	if !atddActive {
		return false
	}
	if r.cfg.Methodology.Granularity == config.MethodologyGranularitySpec {
		r.log("Skipping ATDD: spec granularity active (per-bead ATDD disabled)")
		return false
	}
	if bead.IsTestOnlyBead(bc.Bead.Title) {
		r.log("Skipping ATDD: bead is test-only")
		return false
	}
	return true
}

func (r *Runner) runATDDPreBuildPhases(ctx context.Context, bc *runtypes.BeadContext) bool {
	if r.methodologyExec == nil {
		bc.Result.Error = fmt.Errorf("ATDD active but methodologyExec not wired")
		return false
	}

	r.log("ATDD enabled, writing acceptance tests first...")
	redTimeoutSec := r.methodologyPolicy.PhaseTimeout("red", int(bc.BeadTimeout.Seconds()))
	redCtx, redCancel, redMeta := newPhaseContext(bc, "red", redTimeoutSec)
	defer redCancel()
	r.log("Red phase context: timeout=%s source=%s", redMeta.EffectiveTimeout.Round(time.Second), redMeta.TimeoutSource)
	redPhaseStart := time.Now()
	redBeforeCostUSD, redBeforeInputTokens, redBeforeOutputTokens := snapshotIterationUsage(bc.Result)
	if err := r.methodologyExec.RunAcceptanceTestsWithRetry(redCtx, bc); err != nil {
		return r.failATDDRedPhase(
			bc,
			redPhaseStart,
			redBeforeCostUSD,
			redBeforeInputTokens,
			redBeforeOutputTokens,
			err,
			"acceptance tests phase failed",
		)
	}

	checkErr := r.methodologyExec.CheckTestsFailWithDiagnostic(redCtx, bc)
	if checkErr != nil {
		if methodology.IsATDDAlreadyDone(checkErr) {
			bc.Result.Success = true
			bc.Result.AlreadyDone = true
			return false
		}

		rewrite, ok := methodology.AsATDDRewrite(checkErr)
		if !ok {
			return r.failATDDRedPhase(
				bc,
				redPhaseStart,
				redBeforeCostUSD,
				redBeforeInputTokens,
				redBeforeOutputTokens,
				checkErr,
				"acceptance tests diagnostic check failed",
			)
		}
		if !r.runATDDRewriteRetryCheck(
			redCtx,
			bc,
			redPhaseStart,
			redBeforeCostUSD,
			redBeforeInputTokens,
			redBeforeOutputTokens,
			rewrite,
		) {
			return false
		}
	}
	r.recordPhaseMetricFromSnapshot(
		bc,
		"red",
		1,
		redPhaseStart,
		true,
		redBeforeCostUSD,
		redBeforeInputTokens,
		redBeforeOutputTokens,
	)

	bc.PromptCtx.IsRetry = false
	bc.PromptCtx.PrevFailure = ""
	buildPrompt, err := r.renderer.RenderATDDBuild(bc.PromptCtx)
	if err != nil {
		bc.Result.Error = fmt.Errorf("rendering ATDD build prompt: %w", err)
		return false
	}
	bc.BuildPrompt = buildPrompt
	if bc.Result != nil {
		bc.Result.PromptDiagnostics = r.renderer.LastDiagnostics()
	}
	return true
}

func (r *Runner) failATDDRedPhase(
	bc *runtypes.BeadContext,
	redPhaseStart time.Time,
	beforeCostUSD float64,
	beforeInputTokens int,
	beforeOutputTokens int,
	phaseErr error,
	message string,
) bool {
	r.recordPhaseMetricFromSnapshot(
		bc,
		"red",
		1,
		redPhaseStart,
		false,
		beforeCostUSD,
		beforeInputTokens,
		beforeOutputTokens,
	)
	setPhaseAttribution(bc.Result, "red", phaseErr)
	bc.Result.Error = fmt.Errorf("%s: %w", message, phaseErr)
	return false
}

func (r *Runner) runATDDRewriteRetryCheck(
	redCtx context.Context,
	bc *runtypes.BeadContext,
	redPhaseStart time.Time,
	beforeCostUSD float64,
	beforeInputTokens int,
	beforeOutputTokens int,
	rewrite *methodology.ErrATDDRewrite,
) bool {
	bc.PromptCtx.FailureContext = rewrite.Feedback
	bc.PromptCtx.IsRetry = true

	if err := r.methodologyExec.RunAcceptanceTests(redCtx, bc); err != nil {
		return r.failATDDRedPhase(
			bc,
			redPhaseStart,
			beforeCostUSD,
			beforeInputTokens,
			beforeOutputTokens,
			err,
			"acceptance tests rewrite phase failed",
		)
	}

	r.refreshTouchedPackagesFromStartCommit(bc)
	acceptanceCommands := methodology.AcceptanceCommands(r.cfg.Validation.FastCommandsOrDefault(), bc.TouchedPackages)
	validationResult, err := r.runDirectValidationCheck(redCtx, acceptanceCommands, bc.PromptCtx.WorkDir)
	if err != nil {
		return r.failATDDRedPhase(
			bc,
			redPhaseStart,
			beforeCostUSD,
			beforeInputTokens,
			beforeOutputTokens,
			err,
			"post-rewrite acceptance validation invocation",
		)
	}
	if validationResult == nil {
		r.recordPhaseMetricFromSnapshot(
			bc,
			"red",
			1,
			redPhaseStart,
			false,
			beforeCostUSD,
			beforeInputTokens,
			beforeOutputTokens,
		)
		bc.Result.Error = fmt.Errorf("post-rewrite acceptance validation returned no result")
		return false
	}
	if provider.IsValidationPassed(validationResult) {
		bc.Result.Success = true
		bc.Result.AlreadyDone = true
		return false
	}
	return true
}

// runATDDPostRefactorVerification runs the ATDD acceptance verification phase
// that follows a successful refactor. Returns (retry, terminal).
func (r *Runner) runATDDPostRefactorVerification(ctx context.Context, bc *runtypes.BeadContext, cycleNumber int) (retry bool, terminal *IterationResult) {
	if remaining, elapsed, ok := beadRemaining(bc); ok {
		if remaining <= 0 {
			r.log("Warning: bead timeout budget exhausted before post-refactor acceptance verification; using parent context fallback")
		} else {
			r.log("Post-refactor acceptance verification budget: %s remaining (elapsed %s of %s)", remaining.Round(time.Second), elapsed.Round(time.Second), bc.BeadTimeout.Round(time.Second))
		}
	}

	acceptTimeoutSec := r.cfg.Validation.ResolvePhaseTimeoutSeconds(int(bc.BeadTimeout.Seconds()))
	acceptanceCtx, acceptanceCancel, acceptMeta := newPhaseContext(bc, "acceptance_verification", acceptTimeoutSec)
	defer acceptanceCancel()
	r.log("Acceptance verification phase context: timeout=%s source=%s", acceptMeta.EffectiveTimeout.Round(time.Second), acceptMeta.TimeoutSource)
	verificationPhaseStart := time.Now()
	verificationBeforeCostUSD, verificationBeforeInputTokens, verificationBeforeOutputTokens := snapshotIterationUsage(bc.Result)
	if err := r.methodologyExec.VerifyAcceptanceTestsPass(acceptanceCtx, bc); err != nil {
		r.recordPhaseMetricFromSnapshot(
			bc,
			"verification",
			cycleNumber,
			verificationPhaseStart,
			false,
			verificationBeforeCostUSD,
			verificationBeforeInputTokens,
			verificationBeforeOutputTokens,
		)
		setPhaseAttribution(bc.Result, "acceptance_verification", err)
		if r.handleAcceptanceVerificationFailure(ctx, bc, "acceptance verification failed after refactoring", err) {
			return true, nil
		}
		return false, bc.Result
	}
	r.recordPhaseMetricFromSnapshot(
		bc,
		"verification",
		cycleNumber,
		verificationPhaseStart,
		true,
		verificationBeforeCostUSD,
		verificationBeforeInputTokens,
		verificationBeforeOutputTokens,
	)
	return false, nil
}

func (r *Runner) recordPhaseMetricWithUsage(
	bc *runtypes.BeadContext,
	phase string,
	cycleNumber int,
	phaseStart time.Time,
	success bool,
	costUSD float64,
	inputTokens int,
	outputTokens int,
) {
	if bc == nil || bc.Result == nil || bc.Bead == nil {
		return
	}
	durationMs := int64(0)
	if !phaseStart.IsZero() {
		durationMs = time.Since(phaseStart).Milliseconds()
		if durationMs < 0 {
			durationMs = 0
		}
	}
	if cycleNumber < 1 {
		cycleNumber = 1
	}
	bc.Result.PhaseMetrics = append(bc.Result.PhaseMetrics, runtypes.PhaseMetric{
		Phase:              phase,
		CycleNumber:        cycleNumber,
		BeadID:             bc.Bead.ID,
		Model:              bc.Model,
		Tier:               bc.Tier,
		CostUSD:            costUSD,
		InputTokens:        inputTokens,
		OutputTokens:       outputTokens,
		DurationMs:         durationMs,
		Success:            success,
		Escalated:          bc.Result.Escalated,
		CriteriaTotal:      bc.Result.CriteriaTotal,
		CriteriaCovered:    bc.Result.CriteriaCovered,
		CriteriaUntestable: bc.Result.CriteriaUntestable,
	})
}

func snapshotIterationUsage(result *runtypes.IterationResult) (costUSD float64, inputTokens int, outputTokens int) {
	if result == nil {
		return 0, 0, 0
	}
	return result.CostUSD, result.InputTokens, result.OutputTokens
}

func phaseUsageDelta(
	result *runtypes.IterationResult,
	beforeCostUSD float64,
	beforeInputTokens int,
	beforeOutputTokens int,
) (costUSD float64, inputTokens int, outputTokens int) {
	afterCostUSD, afterInputTokens, afterOutputTokens := snapshotIterationUsage(result)
	costUSD = afterCostUSD - beforeCostUSD
	inputTokens = afterInputTokens - beforeInputTokens
	outputTokens = afterOutputTokens - beforeOutputTokens
	if costUSD < 0 {
		costUSD = 0
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	return costUSD, inputTokens, outputTokens
}

func (r *Runner) recordPhaseMetricFromSnapshot(
	bc *runtypes.BeadContext,
	phase string,
	cycleNumber int,
	phaseStart time.Time,
	success bool,
	beforeCostUSD float64,
	beforeInputTokens int,
	beforeOutputTokens int,
) {
	costUSD, inputTokens, outputTokens := phaseUsageDelta(
		bc.Result,
		beforeCostUSD,
		beforeInputTokens,
		beforeOutputTokens,
	)
	r.recordPhaseMetricWithUsage(
		bc,
		phase,
		cycleNumber,
		phaseStart,
		success,
		costUSD,
		inputTokens,
		outputTokens,
	)
}

func logBudgetSkip(r *Runner, phase string, guard deadlineGuard) {
	if r == nil {
		return
	}
	r.log("Skipping %s: reason=%s (remaining %s, needed %s)", phase, guard.SkipReason, guard.Remaining.Round(time.Second), guard.Needed)
}

func shouldSkipRefactorForBudget(ctx context.Context, bc *runtypes.BeadContext, minBudget time.Duration) deadlineGuard {
	if guard := checkDeadlineGuard(ctx, minBudget); guard.Skip {
		return guard
	}
	if remaining, _, ok := beadRemaining(bc); ok {
		guard := checkRemainingGuard(remaining, minBudget)
		if guard.Skip {
			return deadlineGuard{
				Skip:       true,
				SkipReason: guard.SkipReason,
				Remaining:  remaining,
				Needed:     minBudget,
			}
		}
	}
	return deadlineGuard{}
}

func shouldSkipRevalidationForBudget(bc *runtypes.BeadContext, minBudget time.Duration) deadlineGuard {
	if remaining, _, ok := beadRemaining(bc); ok {
		guard := checkRemainingGuard(remaining, minBudget)
		if guard.Skip {
			return deadlineGuard{
				Skip:       true,
				SkipReason: guard.SkipReason,
				Remaining:  remaining,
				Needed:     minBudget,
			}
		}
	}
	return deadlineGuard{}
}
