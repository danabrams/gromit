package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/danabrams/gromit/internal/tmux"
)

func (r *Runner) handleStuckBead(b *bead.Bead, st *runLoopState) {
	stats := st.beadStats[b.ID]
	r.log("Bead %s marked as stuck (exceeded failure threshold), skipping", b.ID)
	comment := fmt.Sprintf(
		"Skipped after %d failures (exceeded threshold of %d). Please review and break down into smaller tasks if needed.",
		stats.Failures,
		r.cfg.Loop.StuckBeadThreshold,
	)
	if err := r.beads.AddComment(b.ID, comment); err != nil {
		r.log("Warning: failed to add comment to stuck bead: %v", err)
	}
	if err := r.beads.Sync(); err != nil {
		r.log("Warning: failed to sync beads: %v", err)
	}
	st.skippedBeads[b.ID] = true
}

func (r *Runner) handlePrecheckSkip(b *bead.Bead, st *runLoopState, precheckDuration time.Duration) error {
	st.iteration++
	r.log("auto-closing bead %s", b.ID)

	if err := r.beads.Close(b.ID); err != nil {
		r.log("Warning: failed to close bead: %v", err)
		st.skippedBeads[b.ID] = true
	}
	if err := r.beads.Sync(); err != nil {
		r.log("Warning: failed to sync beads: %v", err)
	}

	r.logIterationWithWarning(&logger.IterationLog{
		Timestamp:  time.Now(),
		Iteration:  st.iteration,
		BeadID:     b.ID,
		BeadTitle:  b.Title,
		Model:      "precheck",
		Success:    true,
		DurationMs: precheckDuration.Milliseconds(),
		Outcome:    "precheck_skipped",
	})

	st.consecutiveSkips++
	if st.consecutiveSkips >= r.cfg.Loop.MaxConsecutiveSkips {
		return fmt.Errorf("reached max consecutive precheck skips (%d) — bd may not be persisting bead closures correctly", r.cfg.Loop.MaxConsecutiveSkips)
	}
	return nil
}

func (r *Runner) runScopeGate(ctx context.Context, b *bead.Bead, st *runLoopState) (*prompt.ScopeEstimate, bool) {
	if !r.cfg.ScopeCheck.Enabled || !r.cfg.ScopeCheck.ShouldBlockOversized() {
		return nil, false
	}

	estimate := r.checkScope(ctx, b)
	if estimate == nil {
		return nil, false
	}

	blocked := false
	var reason string
	if estimate.EstimatedIterations >= 3 {
		blocked = true
		reason = fmt.Sprintf("scope check: too many iterations needed (complexity=%s, estimated_iterations=%d)", estimate.Complexity, estimate.EstimatedIterations)
	} else if estimate.Complexity == "high" && len(estimate.Blockers) > 0 {
		blocked = true
		reason = fmt.Sprintf("scope check: high complexity with blockers (%s)", strings.Join(estimate.Blockers, "; "))
	}
	if !blocked {
		return estimate, false
	}

	r.log("Blocking bead %s: %s", b.ID, reason)
	comment := fmt.Sprintf("Blocked by scope gate: %s. Please decompose into smaller tasks.", reason)
	if err := r.beads.AddComment(b.ID, comment); err != nil {
		r.log("Warning: failed to add comment to blocked bead: %v", err)
	}
	st.skippedBeads[b.ID] = true
	r.logIterationWithWarning(&logger.IterationLog{
		Timestamp: time.Now(),
		Iteration: st.iteration + 1,
		BeadID:    b.ID,
		BeadTitle: b.Title,
		Model:     r.cfg.ScopeCheck.Model,
		Success:   false,
		Outcome:   "scope_blocked",
	})
	return estimate, true
}

func (r *Runner) maybeAuthorSpecAcceptance(ctx context.Context, b *bead.Bead, st *runLoopState) error {
	if r == nil || b == nil || st == nil {
		return nil
	}
	if len(r.labelFilters) == 0 {
		return nil
	}
	if r.isScopedSpecOrchestrationEnabled() {
		return nil
	}
	if r.specOrchestrator == nil {
		return nil
	}
	specName := bead.FindSpecLabel(b.Labels)
	if specName == "" {
		return nil
	}
	if st.testsAuthoredBySpec[specName] {
		return nil
	}
	if err := r.specOrchestrator.AuthorAcceptanceTests(ctx, specName); err != nil {
		return err
	}
	st.testsAuthoredBySpec[specName] = true
	return nil
}

func (r *Runner) maybeRunSpecGate(ctx context.Context, b *bead.Bead, st *runLoopState) error {
	if r == nil || r.cfg == nil || b == nil || st == nil {
		return nil
	}
	if r.isScopedSpecOrchestrationEnabled() {
		return nil
	}
	if !r.cfg.SpecGate.IsEnabled() || !r.cfg.SpecGate.IsAutoTrigger() {
		return nil
	}
	if r.specGate == nil || r.beads == nil {
		return nil
	}

	specName := bead.FindSpecLabel(b.Labels)
	if specName == "" {
		return nil
	}

	specsDir := r.cfg.Paths.Specs
	if err := scope.ValidateSpec(specsDir, specName); err != nil {
		return err
	}

	specLabels := scope.ResolveSpec(specName)
	if len(specLabels) == 0 {
		return fmt.Errorf("no label found for spec %q", specName)
	}
	specLabel := specLabels[0]
	if !isScopedRunLabel(r.labelFilters, specLabel) {
		return nil
	}

	labeledBeads, err := r.beads.ListWithLabel(specLabel)
	if err != nil {
		return err
	}
	if hasOpenBeads(labeledBeads) {
		return nil
	}

	currentCycles := st.specGateCycles[specName]
	if r.cfg.SpecGate.MaxCycles > 0 && currentCycles >= r.cfg.SpecGate.MaxCycles {
		return nil
	}

	criteria, _, _, err := loadSpecGateInputs(specsDir, specName)
	if err != nil {
		return err
	}

	verdict, err := r.specGate.Run(ctx, specName, criteria)
	if err != nil {
		return err
	}
	if st.specGateCycles == nil {
		st.specGateCycles = make(map[string]int)
	}
	st.specGateCycles[specName] = currentCycles + 1

	if verdict != nil && !verdict.Passed {
		if _, err := SynthesizeFixBeads(ctx, specName, convertFailedCriteria(verdict.FailedCriteria()), r.beads); err != nil {
			return err
		}
	}

	return nil
}

func hasOpenBeads(beads []*bead.Bead) bool {
	for _, b := range beads {
		if b != nil && strings.EqualFold(b.Status, "open") {
			return true
		}
	}
	return false
}

func (r *Runner) isScopedSpecOrchestrationEnabled() bool {
	return r != nil && r.specOrchestrator != nil && len(r.labelFilters) > 0
}

func (r *Runner) scopedSpecNames() []string {
	if r == nil {
		return nil
	}
	specNames := make([]string, 0, len(r.labelFilters))
	seen := make(map[string]bool, len(r.labelFilters))
	for _, label := range r.labelFilters {
		specName := bead.FindSpecLabel([]string{strings.TrimSpace(label)})
		if specName == "" || seen[specName] {
			continue
		}
		seen[specName] = true
		specNames = append(specNames, specName)
	}
	return specNames
}

func (r *Runner) authorScopedSpecAcceptanceTests(ctx context.Context, st *runLoopState) error {
	if !r.isScopedSpecOrchestrationEnabled() || st == nil {
		return nil
	}
	specsDir := r.cfg.Paths.Specs
	for _, specName := range r.scopedSpecNames() {
		if err := scope.ValidateSpec(specsDir, specName); err != nil {
			return err
		}
		if err := r.specOrchestrator.AuthorAcceptanceTests(ctx, specName); err != nil {
			return err
		}
		st.testsAuthoredBySpec[specName] = true
	}
	return nil
}

func (r *Runner) verifyScopedSpecAcceptance(ctx context.Context, st *runLoopState, noMoreReadyBeads bool) (bool, error) {
	if !noMoreReadyBeads || !r.isScopedSpecOrchestrationEnabled() || r.specGate == nil || r.beads == nil {
		return false, nil
	}
	if !r.cfg.SpecGate.IsEnabled() {
		return false, nil
	}

	maxRetries := r.cfg.Methodology.SpecGateMaxRetries
	if maxRetries <= 0 {
		maxRetries = config.DefaultSpecGateMaxRetries
	}
	shouldRetry := false

	if st != nil && st.specGateRetries == nil {
		st.specGateRetries = make(map[string]int)
	}

	specsDir := r.cfg.Paths.Specs
	for _, specName := range r.scopedSpecNames() {
		if err := scope.ValidateSpec(specsDir, specName); err != nil {
			return false, err
		}
		criteria, _, _, err := loadSpecGateInputs(specsDir, specName)
		if err != nil {
			return false, err
		}
		verdict, err := r.specGate.Run(ctx, specName, criteria)
		if err != nil {
			return false, err
		}
		if verdict != nil && !verdict.Passed {
			failures := convertFailedCriteria(verdict.FailedCriteria())
			retryCount := 0
			if st != nil {
				retryCount = st.specGateRetries[specName]
			}
			if retryCount >= maxRetries {
				r.log("spec_gate_retry_exhausted spec=%s retries=%d max_retries=%d failed_criteria=%d", specName, retryCount, maxRetries, len(failures))
				return false, nil
			}

			ids, err := SynthesizeFixBeads(ctx, specName, failures, r.beads)
			if err != nil {
				return false, err
			}
			if st != nil {
				st.specGateRetries[specName] = retryCount + 1
			}
			r.log("spec_gate_retry_scheduled spec=%s retry=%d max_retries=%d synthesized_fix_beads=%d", specName, retryCount+1, maxRetries, len(ids))
			shouldRetry = true
		}
	}

	return shouldRetry, nil
}

func isScopedRunLabel(filters []string, label string) bool {
	if len(filters) == 0 {
		return false
	}
	for _, filter := range filters {
		if strings.EqualFold(strings.TrimSpace(filter), label) {
			return true
		}
	}
	return false
}

func (r *Runner) processSingleBead(
	ctx context.Context,
	b *bead.Bead,
	st *runLoopState,
	maxIterations int,
	deadline time.Time,
	dryRun bool,
	tmuxMgr *tmux.Manager,
	runThoroughReview func(int),
) (bool, error) {
	if r.isStuckBeadWithStats(b, st.beadStats) {
		r.handleStuckBead(b, st)
		return false, nil
	}

	latest, err := r.beads.Show(b.ID)
	if err != nil {
		r.log("Warning: failed to re-check bead status for %s: %v", b.ID, err)
	} else if latest != nil {
		if strings.EqualFold(latest.Status, "closed") {
			r.log("Bead %s is already closed; skipping", b.ID)
			st.skippedBeads[b.ID] = true
			return false, nil
		}
		// bd ready omits the parent field; backfill from bd show
		// so the proactive-decomposition child guard works.
		b.Parent = latest.Parent
	}

	passed, precheckDuration := r.runPrecheck(ctx, b)
	if passed {
		if err := r.handlePrecheckSkip(b, st, precheckDuration); err != nil {
			return true, err
		}
		return false, nil
	}

	if decomposed := r.runProactiveDecomposition(ctx, b, st); decomposed {
		return false, nil
	}

	scopeEstimate, blocked := r.runScopeGate(ctx, b, st)
	if blocked {
		return false, nil
	}

	if st.iteration > 0 {
		r.log("")
	}
	st.iteration++
	r.log("=== Iteration %d ===", st.iteration)
	r.log("Bead: %s - %s", b.ID, b.Title)

	model := escalation.SelectModel(r.cfg, b)
	if tmuxMgr != nil {
		if err := tmuxMgr.SetTitle(tmux.FormatIterationTitle(st.iteration, b.ID, model)); err != nil {
			r.log("Warning: failed to set tmux title: %v", err)
		}
	}

	heartbeat := r.startStatusHeartbeat(st.statusWriter, statusHeartbeatParams{
		iteration:         st.iteration,
		beadID:            b.ID,
		beadTitle:         b.Title,
		model:             model,
		maxIterations:     maxIterations,
		timeBudgetMinutes: st.timeBudgetMinutes,
		onWriteSuccess: func() {
			st.statusWritten = true
		},
	})
	defer heartbeat.Stop()

	if dryRun {
		r.log("[DRY RUN] Would process bead %s with model %s", b.ID, model)
		return false, nil
	}

	if err := r.maybeAuthorSpecAcceptance(ctx, b, st); err != nil {
		return false, err
	}

	bc, result := r.processBeadWithContext(ctx, b, st.iteration, deadline, scopeEstimate)
	r.log("")
	r.logResult(result)
	r.writeIterationLog(st.iteration, result)
	if shouldCaptureRunbookEntry(result) {
		r.captureRunbookEntry(bc, result)
	}

	st.consecutiveSkips = 0
	if result.Decomposed && result.Error == nil {
		r.log("Continuing after decomposition of bead %s", b.ID)
		return false, nil
	}
	if !result.Success {
		r.escalateUnclearPostRecoveryQualityFailure(ctx, b, result)
		if r.shouldExitRunLoopOnStopLine(result) {
			r.haltStateMutationsAtL3StopLine(result)
			st.l3StopLine = true
			return true, nil
		}

		stats := st.beadStats[b.ID]
		stats.BeadID = b.ID
		stats.BeadTitle = b.Title
		stats.Failures++
		stats.TotalRuns++
		stats.LastAttempt = time.Now()
		st.beadStats[b.ID] = stats

		if result.ReviewBrokeValidation {
			return true, fmt.Errorf("bead %s failed: %v", b.ID, result.Error)
		}
		if r.cfg.Loop.StopOnFailure {
			return true, fmt.Errorf("bead %s failed: %v", b.ID, result.Error)
		}
		r.log("Continuing to next bead despite failure")
		return false, nil
	}

	if err := r.handleSuccessfulIteration(ctx, b, st, result, maxIterations, deadline, runThoroughReview); err != nil {
		return true, err
	}
	return false, nil
}

func (r *Runner) handleSuccessfulIteration(ctx context.Context, b *bead.Bead, st *runLoopState, result *IterationResult, maxIterations int, deadline time.Time, runThoroughReview func(int)) error {
	if err := r.enforceMandatoryQualityGateCoverage("fast", r.cfg.Validation.FastCommandsOrDefault()); err != nil {
		return err
	}

	r.successfulBeads++
	r.successesSinceFull++
	if r.cfg.Validation.FullValidationEveryN > 0 && r.successesSinceFull >= r.cfg.Validation.FullValidationEveryN {
		if err := r.enforceMandatoryQualityGateCoverage("full", r.cfg.Validation.FullCommandsOrDefault()); err != nil {
			return err
		}
	}
	if err := r.maybeRunPeriodicFullValidation(ctx, b.ID, st.iteration); err != nil {
		return err
	}

	if err := r.beads.Close(b.ID); err != nil {
		r.log("Warning: failed to close bead: %v", err)
	}
	if err := r.beads.Sync(); err != nil {
		r.log("Warning: failed to sync beads: %v", err)
	}

	if err := r.maybeRunSpecGate(ctx, b, st); err != nil {
		return err
	}

	if st.statusWriter != nil {
		if err := st.statusWriter.Write(st.iteration, b.ID, b.Title, result.Model, true, maxIterations, st.timeBudgetMinutes); err != nil {
			r.log("Warning: failed to write status.json: %v", err)
		}
	}

	if err := r.mergeInteractiveBranches(); err != nil {
		return err
	}
	r.runBetweenIterationsCommand()

	if err := r.maybeRunThoroughReviewForEpicCompletion(b, st, runThoroughReview); err != nil {
		return err
	}
	if err := r.maybeRunThoroughReviewByFrequency(st, runThoroughReview); err != nil {
		return err
	}

	return nil
}

func (r *Runner) maybeRunThoroughReviewForEpicCompletion(b *bead.Bead, st *runLoopState, runThoroughReview func(int)) error {
	if b.Parent == "" || !r.cfg.Review.Thorough.Enabled || !r.cfg.Review.Thorough.ShouldRunOnEpicComplete() {
		return nil
	}
	hasChildren, err := r.beads.HasOpenChildren(b.Parent)
	if err != nil {
		r.log("Warning: could not check epic children: %v", err)
		return nil
	}
	if hasChildren {
		return nil
	}
	r.log("\n=== Thorough Review (epic %s complete) ===", b.Parent)
	if st.interactiveFile != nil {
		runThoroughReview(st.iteration)
	}
	return nil
}

func (r *Runner) maybeRunThoroughReviewByFrequency(st *runLoopState, runThoroughReview func(int)) error {
	if st.sf == nil {
		return nil
	}

	st.sf.IncrementIterationsSinceReview()
	if err := st.sf.Save(); err != nil {
		r.log("Warning: could not save state: %v", err)
	}

	if !r.cfg.Review.Thorough.Enabled || st.sf.IterationsSinceReview() < r.cfg.Review.Thorough.EveryNIterations {
		return nil
	}
	r.log("\n=== Thorough Review (every %d iterations) ===", r.cfg.Review.Thorough.EveryNIterations)
	if st.interactiveFile != nil {
		runThoroughReview(st.iteration)
	}
	return nil
}

// runProactiveDecomposition checks if the bead is a proactive decomposition candidate
// based on title keywords or description type-definition count. If so, it decomposes
// the bead before first attempt. Returns true if decomposition occurred (caller should skip).
func (r *Runner) runProactiveDecomposition(ctx context.Context, b *bead.Bead, st *runLoopState) bool {
	// Child beads were already scoped down by prior decomposition.
	// Without this guard, children with inherited keywords (e.g. "refactor") loop.
	if b.Parent != "" {
		return false
	}

	if !bead.IsProactiveDecompositionCandidateWithDesc(b.Title, b.Description) {
		return false
	}

	r.log("Bead %s flagged for proactive decomposition (title: %q)", b.ID, b.Title)

	subTasks, err := r.DecomposeTask(ctx, b)
	if err != nil {
		r.log("Warning: proactive decomposition attempt 1 failed for bead %s: %v", b.ID, err)
		// Retry once — LLM occasionally returns non-JSON on first attempt
		subTasks, err = r.DecomposeTask(ctx, b)
		if err != nil {
			r.log("Warning: proactive decomposition attempt 2 failed for bead %s: %v", b.ID, err)
			return false
		}
	}

	if err := r.CreateSubBeads(ctx, b, subTasks); err != nil {
		r.log("Warning: failed to create sub-beads for proactive decomposition of %s: %v", b.ID, err)
		return false
	}

	r.log("Proactively decomposed bead %s into %d sub-tasks", b.ID, len(subTasks))
	st.skippedBeads[b.ID] = true
	return true
}
