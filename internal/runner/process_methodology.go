package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func (r *Runner) prepareMethodologyForBead(ctx context.Context, bc *runtypes.BeadContext) (atddActive bool, tddActive bool, done bool) {
	atddActive = bead.IsMethodologyActive(bc.Bead.Labels, "atdd", r.cfg.Methodology.ATDD)
	if atddActive && bead.IsTestOnlyBead(bc.Bead.Title) {
		r.log("Skipping ATDD: bead is test-only")
		atddActive = false
	}

	if atddActive {
		if !r.runATDDPreBuildPhases(ctx, bc) {
			return atddActive, false, true
		}
	}

	tddActive = bead.IsMethodologyActive(bc.Bead.Labels, "tdd", r.cfg.Methodology.TDD)
	if tddActive {
		r.log("TDD enabled, using TDD build prompt with red-green-refactor cycles...")
		buildPrompt, err := r.renderer.RenderTDDBuild(bc.PromptCtx)
		if err != nil {
			bc.Result.Error = fmt.Errorf("rendering TDD build prompt: %w", err)
			return atddActive, tddActive, true
		}
		bc.BuildPrompt = buildPrompt
	}

	return atddActive, tddActive, false
}

func (r *Runner) runATDDPreBuildPhases(ctx context.Context, bc *runtypes.BeadContext) bool {
	if r.methodologyExec == nil {
		bc.Result.Error = fmt.Errorf("ATDD active but methodologyExec not wired")
		return false
	}

	r.log("ATDD enabled, writing acceptance tests first...")
	redTimeoutSec := r.cfg.Methodology.ResolvePhaseTimeoutSeconds("red", int(bc.BeadTimeout.Seconds()))
	redCtx, redCancel, redMeta := newPhaseContext(bc, "red", redTimeoutSec)
	defer redCancel()
	r.log("Red phase context: timeout=%s source=%s", redMeta.EffectiveTimeout.Round(time.Second), redMeta.TimeoutSource)
	if err := r.methodologyExec.RunAcceptanceTestsWithRetry(redCtx, bc); err != nil {
		setPhaseAttribution(bc.Result, "red", err)
		bc.Result.Error = fmt.Errorf("acceptance tests phase failed: %w", err)
		return false
	}

	bc.PromptCtx.IsRetry = false
	bc.PromptCtx.PrevFailure = ""
	bc.PromptCtx.FailureContext = "Acceptance tests have been written and committed. Your job is to make them pass."
	buildPrompt, err := r.renderer.RenderATDDBuild(bc.PromptCtx)
	if err != nil {
		bc.Result.Error = fmt.Errorf("rendering ATDD build prompt: %w", err)
		return false
	}
	bc.BuildPrompt = buildPrompt
	return true
}

func (r *Runner) executeBuildAndMethodologyLoop(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, tddActive bool, executeWithRetry func() bool) *IterationResult {
	for {
		bc.Result.Error = nil
		bc.Result.AcceptanceFailureSummary = ""
		bc.Result.AcceptanceFailureOutput = ""
		bc.Result.AcceptanceFailureExitCode = 0

		// Inject the explicit scoped test command into the prompt context so the
		// build-phase agent runs tests only on touched packages instead of ./...
		injectScopedTestCommand(bc)

		if !executeWithRetry() {
			return bc.Result
		}

		if bc.StartCommit != "" {
			diff, err := r.getDiff(bc.StartCommit)
			if err == nil && diff != "" {
				bc.TouchedPackages = detectTouchedPackages(diff)
			}
		}

		// In methodology mode, this validation is an intermediate gate before refactor.
		// Defer post-success stages (review/learning) until final validation completes.
		// Use a phase context so intermediate validation is not pre-canceled by bead timeout.
		runPostSuccess := !atddActive && !tddActive
		validationGateCtx := ctx
		var validationGateCancel context.CancelFunc
		if atddActive || tddActive {
			valTimeoutSec := r.cfg.Validation.ResolvePhaseTimeoutSeconds(int(bc.BeadTimeout.Seconds()))
			var valMeta phaseContextMeta
			validationGateCtx, validationGateCancel, valMeta = newPhaseContext(bc, "validation_gate", valTimeoutSec)
			r.log("Intermediate validation phase context: timeout=%s source=%s", valMeta.EffectiveTimeout.Round(time.Second), valMeta.TimeoutSource)
		}
		if err := r.runValidationWithRecoveryForStage(validationGateCtx, bc, runPostSuccess); err != nil {
			if validationGateCancel != nil {
				validationGateCancel()
			}
			bc.Result.Error = err
			return bc.Result
		}
		if validationGateCancel != nil {
			validationGateCancel()
		}

		if atddActive || tddActive {
			retry, terminal := r.runRefactorAndPostChecks(ctx, bc, atddActive)
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

// deadlineGuard holds the result of a deadline check for an optional phase.
type deadlineGuard struct {
	Skip       bool
	Remaining  time.Duration
	Needed     time.Duration
	SkipReason string
}

const (
	skipReasonDeadlineExpired           = "deadline_expired"
	skipReasonInsufficientTimeRemaining = "insufficient_time_remaining"
)

// checkDeadlineGuard inspects ctx.Deadline() and determines whether enough time
// remains to run a phase that requires the given needed duration. If the context
// has no deadline, the phase is allowed to run (Skip=false). If the deadline has
// passed or insufficient time remains, Skip=true with a reason set.
func checkDeadlineGuard(ctx context.Context, needed time.Duration) deadlineGuard {
	deadline, ok := ctx.Deadline()
	if !ok {
		return deadlineGuard{Skip: false, Needed: needed}
	}
	return checkRemainingGuard(time.Until(deadline), needed)
}

func checkRemainingGuard(remaining time.Duration, needed time.Duration) deadlineGuard {
	if remaining <= 0 {
		return deadlineGuard{
			Skip:       true,
			Remaining:  remaining,
			Needed:     needed,
			SkipReason: skipReasonDeadlineExpired,
		}
	}
	if remaining < needed {
		return deadlineGuard{
			Skip:       true,
			Remaining:  remaining,
			Needed:     needed,
			SkipReason: skipReasonInsufficientTimeRemaining,
		}
	}
	return deadlineGuard{Skip: false, Remaining: remaining, Needed: needed}
}

// minRefactorTime is the minimum remaining bead budget required to start the
// refactor phase. Skipping refactor when nearly out of time avoids beginning
// a Claude invocation that is unlikely to complete within budget.
const minRefactorTime = 60 * time.Second

// minRevalidationTime is the minimum remaining bead budget required to run
// post-refactor re-validation. Skipping re-validation when nearly out of time
// avoids starting a validation run that is unlikely to complete.
const minRevalidationTime = 30 * time.Second

func beadRemaining(bc *runtypes.BeadContext) (remaining time.Duration, elapsed time.Duration, ok bool) {
	if bc == nil || bc.BeadTimeout <= 0 || bc.BeadStartTime.IsZero() {
		return 0, 0, false
	}
	elapsed = time.Since(bc.BeadStartTime)
	return bc.BeadTimeout - elapsed, elapsed, true
}

func (r *Runner) runRefactorAndPostChecks(ctx context.Context, bc *runtypes.BeadContext, atddActive bool) (retry bool, terminal *IterationResult) {
	r.log("Running refactor phase...")
	if r.methodologyExec == nil {
		bc.Result.Error = fmt.Errorf("refactor phase active but methodologyExec not wired")
		return false, bc.Result
	}

	if guard := checkDeadlineGuard(ctx, minRefactorTime); guard.Skip {
		r.log("Skipping refactor phase: reason=%s (remaining %s, needed %s)", guard.SkipReason, guard.Remaining.Round(time.Second), guard.Needed)
		return false, nil
	}

	if remaining, _, ok := beadRemaining(bc); ok {
		guard := checkRemainingGuard(remaining, minRefactorTime)
		if guard.Skip {
			r.log("Skipping refactor phase: reason=%s (remaining %s, needed %s)", guard.SkipReason, remaining.Round(time.Second), minRefactorTime)
			return false, nil
		}
	}

	refactorTimeoutSec := r.cfg.Methodology.ResolvePhaseTimeoutSeconds("refactor", int(bc.BeadTimeout.Seconds()))
	refactorCtx, refactorCancel, refactorMeta := newPhaseContext(bc, "refactor", refactorTimeoutSec)
	defer refactorCancel()
	r.log("Refactor phase context: timeout=%s source=%s", refactorMeta.EffectiveTimeout.Round(time.Second), refactorMeta.TimeoutSource)

	if err := r.methodologyExec.RunRefactorPhase(refactorCtx, bc); err != nil {
		r.log("Warning: refactor phase encountered issues: %v", err)
	}

	if r.cfg.Validation.Enabled {
		// --- Deadline guard: skip decision only ---
		// This block decides whether to skip re-validation due to insufficient
		// bead budget. It sets skipRevalidation=true and has no other effect.
		// It does NOT affect how validation errors are handled below.
		skipRevalidation := false
		if remaining, _, ok := beadRemaining(bc); ok {
			guard := checkRemainingGuard(remaining, minRevalidationTime)
			if guard.Skip {
				r.log("Skipping post-refactor re-validation: reason=%s (remaining %s, needed %s)", guard.SkipReason, remaining.Round(time.Second), minRevalidationTime)
				skipRevalidation = true
			}
		}

		// --- Validation and error propagation ---
		// When the guard allows re-validation to proceed, any validation error
		// is always wrapped and returned as a terminal failure. The deadline
		// guard above cannot suppress real validation failures.
		if !skipRevalidation {
			if bc.StartCommit != "" {
				diff, err := r.getDiff(bc.StartCommit)
				if err == nil && diff != "" {
					bc.TouchedPackages = detectTouchedPackages(diff)
				}
			}

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
		if err := r.methodologyExec.VerifyAcceptanceTestsPass(acceptanceCtx, bc); err != nil {
			if r.handleAcceptanceVerificationFailure(ctx, bc, "acceptance verification failed after refactoring", err) {
				return true, nil
			}
			return false, bc.Result
		}
	}

	return false, nil
}
