package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

const coverageTrackerMaxRejections = 2

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
	atddActive = r.methodologyPolicy.IsActive(bc.Bead.Labels, "atdd")
	if r.cfg.Methodology.Granularity == config.MethodologyGranularitySpec {
		if specName := bead.FindSpecLabel(bc.Bead.Labels); specName != "" {
			atddWouldBeActive := r.cfg.Methodology.ATDD
			for _, label := range bc.Bead.Labels {
				if label == "atdd:true" {
					atddWouldBeActive = true
					break
				}
				if label == "atdd:false" {
					atddWouldBeActive = false
					break
				}
			}
			if atddWouldBeActive && !atddActive {
				r.log("Skipping ATDD: spec granularity active for spec:%s", specName)
			}
		}
	}
	if atddActive && bead.IsTestOnlyBead(bc.Bead.Title) {
		r.log("Skipping ATDD: bead is test-only")
		atddActive = false
	}

	if atddActive {
		if !r.runATDDPreBuildPhases(ctx, bc) {
			return atddActive, false, true
		}
	}

	tddActive = r.methodologyPolicy.IsActive(bc.Bead.Labels, "tdd")
	if tddActive {
		if r.cfg.Methodology.FreshContextPerCycle {
			if r.runTDDFreshContextCycles(ctx, bc) {
				return atddActive, tddActive, true
			}
			// Fresh-context couldn't handle this bead (no ExpectedOutputs);
			// fall through to the normal TDD build prompt below.
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

// runTDDFreshContextCycles runs the TDD fresh-context orchestrator for beads
// with ExpectedOutputs. Returns true if it handled the bead (caller should
// return done=true), false if it could not handle it (caller should fall
// through to the normal TDD build prompt path).
func (r *Runner) runTDDFreshContextCycles(ctx context.Context, bc *runtypes.BeadContext) bool {
	if r.tddOrchestrator == nil {
		bc.Result.Error = fmt.Errorf("TDD fresh-context orchestration enabled but tddOrchestrator not wired")
		return true
	}
	effectiveOutputs := tddExpectedOutputsOrTitle(bc.Bead)
	if len(effectiveOutputs) == 0 {
		r.log("TDD fresh-context skipped for bead %s (no ExpectedOutputs); falling back to standard TDD build", bc.Bead.ID)
		return false
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
		bc.Result.Error = fmt.Errorf("tdd fresh-context stopped with unchecked coverage criteria")
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
	if err := r.methodologyExec.RunAcceptanceTestsWithRetry(redCtx, bc); err != nil {
		r.recordPhaseMetric(bc, "red", 1, redPhaseStart, false)
		setPhaseAttribution(bc.Result, "red", err)
		bc.Result.Error = fmt.Errorf("acceptance tests phase failed: %w", err)
		return false
	}
	r.recordPhaseMetric(bc, "red", 1, redPhaseStart, true)

	bc.PromptCtx.IsRetry = false
	bc.PromptCtx.PrevFailure = ""
	bc.PromptCtx.FailureContext = "Acceptance tests have been written and committed. Your job is to make them pass."
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

func (r *Runner) recordPhaseMetric(
	bc *runtypes.BeadContext,
	phase string,
	cycleNumber int,
	phaseStart time.Time,
	success bool,
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
		InputTokens:        bc.Result.InputTokens,
		OutputTokens:       bc.Result.OutputTokens,
		DurationMs:         durationMs,
		Success:            success,
		Escalated:          bc.Result.Escalated,
		CriteriaTotal:      bc.Result.CriteriaTotal,
		CriteriaCovered:    bc.Result.CriteriaCovered,
		CriteriaUntestable: bc.Result.CriteriaUntestable,
	})
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
		if !executeWithRetry() {
			r.recordPhaseMetric(bc, "green", cycleNumber, greenPhaseStart, false)
			if bc.Result.FailurePhase == "" {
				if bc.Result.TimeoutType != "" || isTimeoutOrCanceledError(bc.Result.Error) {
					bc.Result.FailurePhase = failurephase.Timeout
				} else {
					bc.Result.FailurePhase = failurephase.Build
				}
			}
			return bc.Result
		}
		r.recordPhaseMetric(bc, "green", cycleNumber, greenPhaseStart, true)

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

		if atddActive || tddActive {
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
	if guard := checkDeadlineGuard(ctx, minRefactorBudget); guard.Skip {
		r.log("Skipping refactor phase: reason=%s (remaining %s, needed %s)", guard.SkipReason, guard.Remaining.Round(time.Second), guard.Needed)
		return false, nil
	}

	if remaining, _, ok := beadRemaining(bc); ok {
		guard := checkRemainingGuard(remaining, minRefactorBudget)
		if guard.Skip {
			r.log("Skipping refactor phase: reason=%s (remaining %s, needed %s)", guard.SkipReason, remaining.Round(time.Second), minRefactorBudget)
			return false, nil
		}
	}

	refactorTimeoutSec := r.methodologyPolicy.PhaseTimeout("refactor", int(bc.BeadTimeout.Seconds()))
	refactorCtx, refactorCancel, refactorMeta := newPhaseContext(bc, "refactor", refactorTimeoutSec)
	defer refactorCancel()
	r.log("Refactor phase context: timeout=%s source=%s", refactorMeta.EffectiveTimeout.Round(time.Second), refactorMeta.TimeoutSource)

	refactorPhaseStart := time.Now()
	if err := r.methodologyExec.RunRefactorPhase(refactorCtx, bc); err != nil {
		r.recordPhaseMetric(bc, "refactor", cycleNumber, refactorPhaseStart, false)
		r.log("Warning: refactor phase encountered issues: %v", err)
	} else {
		r.recordPhaseMetric(bc, "refactor", cycleNumber, refactorPhaseStart, true)
	}

	if r.cfg.Validation.Enabled {
		// --- Deadline guard: skip decision only ---
		// This block decides whether to skip re-validation due to insufficient
		// bead budget. It sets skipRevalidation=true and has no other effect.
		// It does NOT affect how validation errors are handled below.
		skipRevalidation := false
		minRevalidationBudget := r.methodologyPolicy.MinRevalidationBudget()
		if remaining, _, ok := beadRemaining(bc); ok {
			guard := checkRemainingGuard(remaining, minRevalidationBudget)
			if guard.Skip {
				r.log("Skipping post-refactor re-validation: reason=%s (remaining %s, needed %s)", guard.SkipReason, remaining.Round(time.Second), minRevalidationBudget)
				skipRevalidation = true
			}
		}

		// --- Validation and error propagation ---
		// When the guard allows re-validation to proceed, any validation error
		// is always wrapped and returned as a terminal failure. The deadline
		// guard above cannot suppress real validation failures.
		if !skipRevalidation {
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
		if err := r.methodologyExec.VerifyAcceptanceTestsPass(acceptanceCtx, bc); err != nil {
			r.recordPhaseMetric(bc, "verification", cycleNumber, verificationPhaseStart, false)
			setPhaseAttribution(bc.Result, "acceptance_verification", err)
			if r.handleAcceptanceVerificationFailure(ctx, bc, "acceptance verification failed after refactoring", err) {
				return true, nil
			}
			return false, bc.Result
		}
		r.recordPhaseMetric(bc, "verification", cycleNumber, verificationPhaseStart, true)
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
