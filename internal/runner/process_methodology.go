package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

const (
	coverageTrackerMaxRejections              = 2
	tddFreshContextCoverageIncompleteErrorMsg = "tdd fresh-context stopped with unchecked coverage criteria"
	methodologyATDD                           = "atdd"
	methodologyTDD                            = "tdd"
)

func (r *Runner) ensureMethodologyPolicy() {
	if r == nil {
		return
	}
	if r.methodologyPolicy != nil {
		return
	}
	cfg := r.cfg
	if cfg == nil {
		cfg = &config.Config{}
	}
	r.methodologyPolicy = policy.NewConfigMethodologyPolicy(cfg)
}

func (r *Runner) prepareMethodologyForBead(ctx context.Context, bc *runtypes.BeadContext) (atddActive bool, tddActive bool, done bool) {
	r.ensureMethodologyPolicy()
	atddActive = r.methodologyPolicy.IsActive(bc.Bead.Labels, methodologyATDD)
	atddActive = r.applyATDDSkipPolicies(bc, atddActive)

	if atddActive {
		if !r.runATDDPreBuildPhases(ctx, bc) {
			return atddActive, false, true
		}
	}

	tddActive = r.methodologyPolicy.IsActive(bc.Bead.Labels, methodologyTDD)
	if tddActive {
		if r.cfg.Methodology.FreshContextPerCycle {
			r.runTDDFreshContextCycles(ctx, bc)
			return atddActive, tddActive, true
		}
		r.log("TDD enabled, using TDD build prompt with red-green-refactor cycles...")
		buildPrompt, err := r.renderer.RenderTDDBuild(bc.PromptCtx)
		if err != nil {
			bc.Result.Error = fmt.Errorf("rendering TDD build prompt: %w", err)
			return atddActive, tddActive, true
		}
		bc.BuildPrompt = buildPrompt
		if bc.Result != nil {
			bc.Result.PromptDiagnostics = r.renderer.LastDiagnostics()
		}
	}

	return atddActive, tddActive, false
}

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

// runTDDFreshContextCycles runs the TDD fresh-context orchestrator.
// It always handles TDD execution when fresh-context mode is enabled.
func (r *Runner) runTDDFreshContextCycles(ctx context.Context, bc *runtypes.BeadContext) bool {
	if r.tddOrchestrator == nil {
		bc.Result.Error = fmt.Errorf("TDD fresh-context orchestration enabled but tddOrchestrator not wired")
		return true
	}
	effectiveOutputs := tddExpectedOutputsOrTitle(bc.Bead)
	if len(effectiveOutputs) == 0 {
		bc.Result.Error = fmt.Errorf("TDD fresh-context requires ExpectedOutputs or a non-empty bead title (bead=%s)", bc.Bead.ID)
		return true
	}
	if len(bc.Bead.ExpectedOutputs) == 0 {
		r.log("TDD fresh-context using title fallback for bead %s because ExpectedOutputs are empty", bc.Bead.ID)
		bc.Bead.ExpectedOutputs = append([]string(nil), effectiveOutputs...)
	}

	coverageTracker, coverageCriteria, err := buildCoverageTrackerFromSpec(bc)
	if err != nil {
		bc.Result.Error = fmt.Errorf("building TDD coverage tracker: %w", err)
		return true
	}
	defer r.finalizeTDDCoverageSummary(bc, coverageTracker)
	updateIterationCoverageMetrics(bc.Result, coverageTracker)

	maxOrchestratorPasses := resolveMaxTDDCycles(r.cfg)

	for pass := 0; pass < maxOrchestratorPasses; pass++ {
		if err := r.tddOrchestrator.RunCycles(ctx, bc, coverageTracker, coverageCriteria); err != nil {
			if bc.StartCommit != "" {
				if resetErr := r.resetHard(bc.StartCommit); resetErr != nil {
					r.log("Warning: failed to reset to %s after TDD failure: %v", bc.StartCommit, resetErr)
				}
			}
			bc.Result.Error = err
			return true
		}
		updateIterationCoverageMetrics(bc.Result, coverageTracker)
		if coverageTracker == nil || coverageTracker.IsComplete() {
			break
		}
		r.log("TDD coverage tracker reports unchecked criteria after cycle pass %d; injecting additional cycles", pass+1)
	}
	if coverageTracker != nil && !coverageTracker.IsComplete() {
		if bc.StartCommit != "" {
			if resetErr := r.resetHard(bc.StartCommit); resetErr != nil {
				r.log("Warning: failed to reset to %s after TDD coverage incomplete: %v", bc.StartCommit, resetErr)
			}
		}
		bc.Result.Error = errors.New(tddFreshContextCoverageIncompleteErrorMsg)
		return true
	}
	if r.cfg.Validation.Enabled && r.validationRunner != nil {
		if err := r.runValidationWithRecoveryForStage(ctx, bc, true); err != nil {
			bc.Result.Error = err
			return true
		}
	}
	bc.Result.Success = true
	bc.Result.FirstPassSuccess = true
	return true
}

func buildCoverageTrackerFromSpec(bc *runtypes.BeadContext) (*coverage.CoverageTracker, []coverage.Criterion, error) {
	if bc == nil || bc.Bead == nil || bc.PromptCtx == nil {
		return nil, nil, nil
	}
	// Skip per-bead coverage tracking when spec-granularity is active —
	// the spec gate handles system-level criteria after all beads complete.
	if bc.PromptCtx.MethodologyGranularity == config.MethodologyGranularitySpec {
		return nil, nil, nil
	}
	if bead.FindSpecLabel(bc.Bead.Labels) == "" {
		return nil, nil, nil
	}
	specContent := strings.TrimSpace(bc.PromptCtx.Spec)
	if specContent == "" {
		return nil, nil, nil
	}
	criteria, err := coverage.ParseCriteria(specContent)
	if err != nil {
		return nil, nil, err
	}
	if len(criteria) == 0 {
		return nil, nil, nil
	}
	return coverage.NewTracker(criteria, coverageTrackerMaxRejections), criteria, nil
}

func resolveMaxTDDCycles(cfg *config.Config) int {
	maxCycles := config.DefaultMaxTDDCycles
	if cfg != nil && cfg.Methodology.MaxTDDCycles > 0 {
		maxCycles = cfg.Methodology.MaxTDDCycles
	}
	if maxCycles < 1 {
		return 1
	}
	return maxCycles
}

func updateIterationCoverageMetrics(result *runtypes.IterationResult, tracker *coverage.CoverageTracker) {
	if result == nil {
		return
	}
	result.CriteriaTotal = 0
	result.CriteriaCovered = 0
	result.CriteriaUntestable = 0
	result.UncoveredCriteria = []string{}

	if tracker == nil {
		return
	}

	uncovered := tracker.UncoveredCriteria()
	result.CriteriaTotal = tracker.TotalCriteria()
	result.CriteriaUntestable = len(tracker.UntestableCriteria())
	result.CriteriaCovered = len(tracker.CoveredCriteria())
	result.UncoveredCriteria = criterionTexts(uncovered)
}

func (r *Runner) finalizeTDDCoverageSummary(bc *runtypes.BeadContext, tracker *coverage.CoverageTracker) {
	if r == nil || bc == nil || bc.Result == nil || bc.Bead == nil || tracker == nil {
		return
	}

	updateIterationCoverageMetrics(bc.Result, tracker)
	summary := tracker.Summary()
	r.log("TDD coverage summary for bead %s:\n%s", bc.Bead.ID, summary)

	if !hasCoverageGaps(tracker) {
		return
	}
	if r.beads == nil {
		return
	}
	if err := r.beads.AddComment(bc.Bead.ID, summary); err != nil {
		r.log("Warning: failed to add coverage summary comment: %v", err)
	}
}

func criterionTexts(criteria []coverage.CriterionState) []string {
	texts := make([]string, 0, len(criteria))
	for _, criterion := range criteria {
		texts = append(texts, criterion.Text)
	}
	return texts
}

func hasCoverageGaps(tracker *coverage.CoverageTracker) bool {
	if tracker == nil {
		return false
	}
	return len(tracker.UncoveredCriteria()) > 0 || len(tracker.UntestableCriteria()) > 0
}

func tddExpectedOutputsOrTitle(b *bead.Bead) []string {
	if b == nil {
		return []string{}
	}
	if len(b.ExpectedOutputs) > 0 {
		return append([]string(nil), b.ExpectedOutputs...)
	}
	trimmedTitle := strings.TrimSpace(b.Title)
	if trimmedTitle == "" {
		return []string{}
	}
	return []string{trimmedTitle}
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

func (r *Runner) executeBuildAndMethodologyLoop(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, tddActive bool, executeWithRetry func() bool) *IterationResult {
	r.ensureMethodologyPolicy()
	cycleNumber := 0
	for {
		cycleNumber++
		bc.Result.Error = nil
		bc.Result.AcceptanceFailureSummary = ""
		bc.Result.AcceptanceFailureOutput = ""
		bc.Result.AcceptanceFailureExitCode = 0

		// Inject the explicit scoped test command into the prompt context so the
		// build-phase agent runs tests only on touched packages instead of ./...
		injectScopedTestCommand(bc)

		greenPhaseStart := time.Now()
		greenBeforeCostUSD, greenBeforeInputTokens, greenBeforeOutputTokens := snapshotIterationUsage(bc.Result)
		greenSuccess := executeWithRetry()
		if !greenSuccess {
			r.recordPhaseMetricFromSnapshot(
				bc,
				"green",
				cycleNumber,
				greenPhaseStart,
				false,
				greenBeforeCostUSD,
				greenBeforeInputTokens,
				greenBeforeOutputTokens,
			)
			if bc.Result.FailurePhase == "" {
				if bc.Result.TimeoutType != "" || isTimeoutOrCanceledError(bc.Result.Error) {
					bc.Result.FailurePhase = failurephase.Timeout
				} else {
					bc.Result.FailurePhase = failurephase.Build
				}
			}
			return bc.Result
		}
		r.recordPhaseMetricFromSnapshot(
			bc,
			"green",
			cycleNumber,
			greenPhaseStart,
			true,
			greenBeforeCostUSD,
			greenBeforeInputTokens,
			greenBeforeOutputTokens,
		)

		r.refreshTouchedPackagesFromStartCommit(bc)

		// In methodology mode, this validation is an intermediate gate before refactor.
		// Defer post-success stages (review/learning) until final validation completes.
		// Use a phase context so intermediate validation is not pre-canceled by bead timeout.
		runPostSuccess := r.methodologyPolicy.ShouldDeferPostSuccess(atddActive, tddActive)
		validationGateCtx := ctx
		if atddActive || tddActive {
			valTimeoutSec := r.cfg.Validation.ResolvePhaseTimeoutSeconds(int(bc.BeadTimeout.Seconds()))
			var validationGateCancel context.CancelFunc
			var valMeta phaseContextMeta
			validationGateCtx, validationGateCancel, valMeta = newPhaseContext(bc, "validation_gate", valTimeoutSec)
			defer validationGateCancel()
			r.log("Intermediate validation phase context: timeout=%s source=%s", valMeta.EffectiveTimeout.Round(time.Second), valMeta.TimeoutSource)
		}
		if err := r.runValidationWithRecoveryForStage(validationGateCtx, bc, runPostSuccess); err != nil {
			setPhaseAttribution(bc.Result, "validation_gate", err)
			if bc.Result.FailurePhase == "" {
				if isTimeoutOrCanceledError(err) {
					bc.Result.FailurePhase = failurephase.Timeout
				} else {
					bc.Result.FailurePhase = failurephase.Validation
				}
			}
			bc.Result.Error = err
			return bc.Result
		}

		if tddActive {
			retry, terminal := r.runRefactorAndPostChecks(ctx, bc, atddActive, cycleNumber)
			if retry {
				continue
			}
			if terminal != nil {
				return terminal
			}
		}

		bc.Result.Success = true
		bc.Result.FirstPassSuccess = bc.TotalRetriesThisBead == 0 && !bc.Result.Escalated
		return bc.Result
	}
}

func (r *Runner) runRefactorAndPostChecks(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, cycleNumber int) (retry bool, terminal *IterationResult) {
	r.ensureMethodologyPolicy()
	r.log("Running refactor phase...")
	if r.methodologyExec == nil {
		bc.Result.Error = fmt.Errorf("refactor phase active but methodologyExec not wired")
		return false, bc.Result
	}

	minRefactorBudget := r.methodologyPolicy.MinRefactorBudget()
	if guard := shouldSkipRefactorForBudget(ctx, bc, minRefactorBudget); guard.Skip {
		logBudgetSkip(r, "refactor phase", guard)
		return false, nil
	}

	refactorTimeoutSec := r.methodologyPolicy.PhaseTimeout("refactor", int(bc.BeadTimeout.Seconds()))
	refactorCtx, refactorCancel, refactorMeta := newPhaseContext(bc, "refactor", refactorTimeoutSec)
	defer refactorCancel()
	r.log("Refactor phase context: timeout=%s source=%s", refactorMeta.EffectiveTimeout.Round(time.Second), refactorMeta.TimeoutSource)

	refactorPhaseStart := time.Now()
	refactorBeforeCostUSD, refactorBeforeInputTokens, refactorBeforeOutputTokens := snapshotIterationUsage(bc.Result)
	refactorResult := r.methodologyExec.RunRefactorPhaseWithResult(refactorCtx, bc)
	if !refactorResult.Successful {
		r.recordPhaseMetricFromSnapshot(
			bc,
			"refactor",
			cycleNumber,
			refactorPhaseStart,
			false,
			refactorBeforeCostUSD,
			refactorBeforeInputTokens,
			refactorBeforeOutputTokens,
		)
		bc.Result.FailurePhase = failurephase.Build
		bc.Result.Error = fmt.Errorf("refactor phase failed: %s", refactorResult.Reason)
		return false, bc.Result
	}
	r.recordPhaseMetricFromSnapshot(
		bc,
		"refactor",
		cycleNumber,
		refactorPhaseStart,
		true,
		refactorBeforeCostUSD,
		refactorBeforeInputTokens,
		refactorBeforeOutputTokens,
	)

	if r.cfg.Validation.Enabled {
		// --- Deadline guard: skip decision only ---
		// This block decides whether to skip re-validation due to insufficient
		// bead budget. It sets skipRevalidation=true and has no other effect.
		// It does NOT affect how validation errors are handled below.
		minRevalidationBudget := r.methodologyPolicy.MinRevalidationBudget()
		skipRevalidation := shouldSkipRevalidationForBudget(bc, minRevalidationBudget)
		if skipRevalidation.Skip {
			logBudgetSkip(r, "post-refactor re-validation", skipRevalidation)
		}

		// --- Validation and error propagation ---
		// When the guard allows re-validation to proceed, any validation error
		// is always wrapped and returned as a terminal failure. The deadline
		// guard above cannot suppress real validation failures.
		if !skipRevalidation.Skip {
			r.refreshTouchedPackagesFromStartCommit(bc)

			valTimeoutSec := r.cfg.Validation.ResolvePhaseTimeoutSeconds(int(bc.BeadTimeout.Seconds()))
			validationCtx, validationCancel, valMeta := newPhaseContext(bc, "validation", valTimeoutSec)
			defer validationCancel()
			r.log("Validation phase context: timeout=%s source=%s", valMeta.EffectiveTimeout.Round(time.Second), valMeta.TimeoutSource)
			// This is the final validation pass after refactor, so post-success stages
			// must run here.
			if err := r.runValidationWithRecoveryForStage(validationCtx, bc, true); err != nil {
				setPhaseAttribution(bc.Result, "validation", err)
				bc.Result.Error = wrapRefactorValidationError(err)
				return false, bc.Result
			}
		}
	}

	if atddActive && r.methodologyExec != nil {
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
	}

	return false, nil
}

func (r *Runner) refreshTouchedPackagesFromStartCommit(bc *runtypes.BeadContext) {
	if r == nil || bc == nil || bc.StartCommit == "" {
		return
	}
	diff, err := r.getDiff(bc.StartCommit)
	if err != nil || diff == "" {
		return
	}
	bc.TouchedPackages = methodology.DetectTouchedPackages(diff)
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
