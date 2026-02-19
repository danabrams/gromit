package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/runtypes"
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
	if err := r.tddOrchestrator.RunCycles(ctx, bc); err != nil {
		bc.Result.Error = err
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
	if bc.Result != nil {
		bc.Result.PromptDiagnostics = r.renderer.LastDiagnostics()
	}
	return true
}

func (r *Runner) executeBuildAndMethodologyLoop(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, tddActive bool, executeWithRetry func() bool) *IterationResult {
	r.ensureMethodologyPolicy()
	for {
		bc.Result.Error = nil
		bc.Result.AcceptanceFailureSummary = ""
		bc.Result.AcceptanceFailureOutput = ""
		bc.Result.AcceptanceFailureExitCode = 0

		// Inject the explicit scoped test command into the prompt context so the
		// build-phase agent runs tests only on touched packages instead of ./...
		injectScopedTestCommand(bc)

		if !executeWithRetry() {
			if bc.Result.FailurePhase == "" {
				if bc.Result.TimeoutType != "" || isTimeoutOrCanceledError(bc.Result.Error) {
					bc.Result.FailurePhase = failurephase.Timeout
				} else {
					bc.Result.FailurePhase = failurephase.Build
				}
			}
			return bc.Result
		}

		if bc.StartCommit != "" {
			diff, err := r.getDiff(bc.StartCommit)
			if err == nil && diff != "" {
				bc.TouchedPackages = methodology.DetectTouchedPackages(diff)
			}
		}

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

func beadRemaining(bc *runtypes.BeadContext) (remaining time.Duration, elapsed time.Duration, ok bool) {
	if bc == nil || bc.BeadTimeout <= 0 || bc.BeadStartTime.IsZero() {
		return 0, 0, false
	}
	elapsed = time.Since(bc.BeadStartTime)
	return bc.BeadTimeout - elapsed, elapsed, true
}

func (r *Runner) runRefactorAndPostChecks(ctx context.Context, bc *runtypes.BeadContext, atddActive bool) (retry bool, terminal *IterationResult) {
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

	if err := r.methodologyExec.RunRefactorPhase(refactorCtx, bc); err != nil {
		r.log("Warning: refactor phase encountered issues: %v", err)
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
			if bc.StartCommit != "" {
				diff, err := r.getDiff(bc.StartCommit)
				if err == nil && diff != "" {
					bc.TouchedPackages = methodology.DetectTouchedPackages(diff)
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
			setPhaseAttribution(bc.Result, "acceptance_verification", err)
			if r.handleAcceptanceVerificationFailure(ctx, bc, "acceptance verification failed after refactoring", err) {
				return true, nil
			}
			return false, bc.Result
		}
	}

	return false, nil
}
