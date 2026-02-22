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
	"github.com/danabrams/gromit/internal/prompt"
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

// runTDDFreshContextCycles runs the TDD fresh-context orchestrator.
// It always handles TDD execution when fresh-context mode is enabled.
func (r *Runner) runTDDFreshContextCycles(ctx context.Context, bc *runtypes.BeadContext) bool {
	if r.tddOrchestrator == nil {
		bc.Result.Error = fmt.Errorf("TDD fresh-context orchestration enabled but tddOrchestrator not wired")
		return true
	}
	layer1Active := len(bc.Bead.ExpectedOutputs) > 0
	effectiveOutputs := tddExpectedOutputsOrTitle(bc.Bead)
	if len(effectiveOutputs) <= 1 && r.router != nil {
		invoke := func(innerCtx context.Context, prompt, tier string) (*provider.Result, error) {
			p, _ := r.router.Select("build", tier)
			if p == nil {
				return nil, fmt.Errorf("no provider available for tier %s", tier)
			}
			return p.Run(innerCtx, prompt, tier)
		}
		if updated, activated := applyLayer3Requirements(ctx, effectiveOutputs, bc.Bead.Title, bc.Bead.Description, invoke); activated {
			r.log("TDD fresh-context: bead %s using layer3 (LLM extraction, %d outputs)", bc.Bead.ID, len(updated))
			effectiveOutputs = updated
		}
	}
	if len(effectiveOutputs) == 0 {
		bc.Result.Error = fmt.Errorf("TDD fresh-context requires ExpectedOutputs or a non-empty bead title (bead=%s)", bc.Bead.ID)
		return true
	}
	if layer1Active {
		r.log("TDD fresh-context: bead %s using layer1 (ExpectedOutputs, %d outputs)", bc.Bead.ID, len(effectiveOutputs))
	} else {
		if len(extractRequirementsFromDescription(bc.Bead.Description)) > 0 {
			r.log("TDD fresh-context: bead %s using layer2 (description parsing, %d outputs)", bc.Bead.ID, len(effectiveOutputs))
		} else {
			r.log("TDD fresh-context: bead %s using title fallback", bc.Bead.ID)
		}
		bc.Bead.ExpectedOutputs = append([]string(nil), effectiveOutputs...)
	}

	r.log("TDD fresh-context: starting for bead %s (%d outputs)", bc.Bead.ID, len(bc.Bead.ExpectedOutputs))

	coverageTracker, coverageCriteria, err := buildCoverageTrackerFromSpec(bc)
	if err != nil {
		bc.Result.Error = fmt.Errorf("building TDD coverage tracker: %w", err)
		return true
	}
	defer r.finalizeTDDCoverageSummary(bc, coverageTracker)
	updateIterationCoverageMetrics(bc.Result, coverageTracker)

	maxOrchestratorPasses := resolveMaxTDDCycles(r.cfg)
	tddStart := time.Now()

	for pass := 0; pass < maxOrchestratorPasses; pass++ {
		r.log("TDD fresh-context: orchestrator pass %d/%d", pass+1, maxOrchestratorPasses)
		if err := r.tddOrchestrator.RunCycles(ctx, bc, coverageTracker, coverageCriteria); err != nil {
			r.log("TDD fresh-context: failed after %v — %v", time.Since(tddStart).Round(time.Second), err)
			if bc.StartCommit != "" {
				if resetErr := r.resetHard(bc.StartCommit); resetErr != nil {
					r.log("Warning: failed to reset to %s after TDD failure: %v", bc.StartCommit, resetErr)
				}
			}
			bc.Result.Error = err
			return true
		}
		aggregateTDDPhaseMetricsToResult(bc)
		updateIterationCoverageMetrics(bc.Result, coverageTracker)
		if coverageTracker == nil || coverageTracker.IsComplete() {
			break
		}
		r.log("TDD coverage tracker reports unchecked criteria after cycle pass %d; injecting additional cycles", pass+1)
	}
	if coverageTracker != nil && !coverageTracker.IsComplete() {
		r.log("TDD fresh-context: coverage incomplete after %v", time.Since(tddStart).Round(time.Second))
		if bc.StartCommit != "" {
			if resetErr := r.resetHard(bc.StartCommit); resetErr != nil {
				r.log("Warning: failed to reset to %s after TDD coverage incomplete: %v", bc.StartCommit, resetErr)
			}
		}
		bc.Result.Error = errors.New(tddFreshContextCoverageIncompleteErrorMsg)
		return true
	}
	if r.cfg.Validation.Enabled && r.validationRunner != nil {
		r.log("TDD fresh-context: running final validation")
		if err := r.runValidationWithRecoveryForStage(ctx, bc, true); err != nil {
			r.log("TDD fresh-context: final validation failed — %v", err)
			bc.Result.Error = err
			return true
		}
	}
	r.log("TDD fresh-context: completed successfully in %v", time.Since(tddStart).Round(time.Second))
	// Update diagnostics to reflect that TDD fresh-context was the actual
	// methodology used, overriding the initial "build" prompt_type set by
	// buildPromptForBead.
	if bc.Result != nil {
		bc.Result.PromptDiagnostics = prompt.NewDiagnostics("tdd_fresh_context", nil)
	}
	bc.Result.Success = true
	bc.Result.FirstPassSuccess = true
	return true
}

func buildCoverageTrackerFromSpec(bc *runtypes.BeadContext) (*coverage.CoverageTracker, []coverage.Criterion, error) {
	if bc == nil || bc.Bead == nil || bc.PromptCtx == nil {
		return nil, nil, nil
	}
	// Spec-level criteria can't be satisfied by individual beads and waste
	// 30-40 min per bead. The spec gate handles them after all beads complete.
	return nil, nil, nil
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

var requirementHeaders = []string{"Requirements:", "Includes:", "Delivers:"}

func isRequirementHeader(line string) bool {
	for _, h := range requirementHeaders {
		if line == h {
			return true
		}
	}
	return false
}

func extractRequirementsFromDescription(description string) []string {
	var results []string
	inHeaderSection := false
	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			inHeaderSection = false
			continue
		}
		// Header-prefixed section: "Requirements:", "Includes:", "Delivers:"
		if isRequirementHeader(line) {
			inHeaderSection = true
			continue
		}
		// Numbered list: "1. item"
		if len(line) >= 3 {
			i := 0
			for i < len(line) && line[i] >= '0' && line[i] <= '9' {
				i++
			}
			if i > 0 && i < len(line) && line[i] == '.' {
				item := strings.TrimSpace(line[i+1:])
				if item != "" {
					inHeaderSection = false
					results = append(results, item)
					continue
				}
			}
		}
		// Bulleted list: "- item", "* item", "+ item"
		if len(line) >= 2 && (line[0] == '-' || line[0] == '*' || line[0] == '+') && line[1] == ' ' {
			item := strings.TrimSpace(line[1:])
			if item != "" {
				inHeaderSection = false
				results = append(results, item)
				continue
			}
		}
		// Semicolon-separated items
		if strings.Contains(line, ";") {
			for _, part := range strings.Split(line, ";") {
				item := strings.TrimSpace(part)
				if item != "" {
					results = append(results, item)
				}
			}
			inHeaderSection = false
			continue
		}
		// Plain line following a header
		if inHeaderSection {
			results = append(results, line)
		}
	}
	return results
}

func extractRequirementsViaLLM(ctx context.Context, title, description string, invoke func(ctx context.Context, prompt, tier string) (*provider.Result, error)) []string {
	const maxDescLen = 2000
	desc := description
	if len(desc) > maxDescLen {
		desc = desc[:maxDescLen]
	}
	promptText := fmt.Sprintf("Extract the requirements from the following task.\nTitle: %s\nDescription: %s\n\nReturn each requirement on its own line.", title, desc)

	invokeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := invoke(invokeCtx, promptText, provider.TierLow)
	if err != nil {
		return nil
	}

	var items []string
	for _, line := range strings.Split(result.Output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}
	if len(items) < 2 {
		return nil
	}
	return items
}

func tddExpectedOutputsOrTitle(b *bead.Bead) []string {
	if b == nil {
		return []string{}
	}
	if len(b.ExpectedOutputs) > 0 {
		return append([]string(nil), b.ExpectedOutputs...)
	}
	if parsed := extractRequirementsFromDescription(b.Description); len(parsed) > 0 {
		return parsed
	}
	trimmedTitle := strings.TrimSpace(b.Title)
	if trimmedTitle == "" {
		return []string{}
	}
	return []string{trimmedTitle}
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
		return r.runATDDPostRefactorVerification(ctx, bc, cycleNumber)
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
