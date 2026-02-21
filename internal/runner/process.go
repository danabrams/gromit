package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/preflight"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

const (
	compilationCheckTimeout = 30 * time.Second
	compilationErrorsPrompt = "\n\n<compilation-errors>\nThe codebase currently has compilation errors. You must fix these as part of your work:\n\n%s\n</compilation-errors>"
)

// setupBeadContext validates runner state, sets up timeouts, captures git state,
// fetches parent bead, and selects the initial model.
func (r *Runner) setupBeadContext(ctx context.Context, b *bead.Bead, iteration int, runDeadline time.Time, scopeEstimate *prompt.ScopeEstimate) (*runtypes.BeadContext, context.Context, context.CancelFunc, error) {
	if r.cfg == nil {
		return nil, nil, nil, fmt.Errorf("runner config is nil")
	}
	if r.beads == nil {
		return nil, nil, nil, fmt.Errorf("runner beads client is nil")
	}
	if r.renderer == nil {
		return nil, nil, nil, fmt.Errorf("runner renderer is nil")
	}
	if r.router == nil {
		return nil, nil, nil, fmt.Errorf("runner router is nil")
	}

	r.ensureEscalationPolicy()
	// Use escalation.SelectTier so low-complexity and test-only routing are honored.
	tier := escalation.SelectTier(r.cfg, b)
	model := escalation.SelectModel(r.cfg, b) // legacy model name for display/timeouts

	_, _, _, beadTimeoutSec := r.cfg.Claude.TimeoutsForModel(model)
	beadTimeout := time.Duration(beadTimeoutSec) * time.Second
	beadCtx, beadCancel := context.WithTimeout(ctx, beadTimeout)

	startCommit, err := r.getHead()
	if err != nil {
		r.log("Warning: could not capture git HEAD: %v", err)
		startCommit = ""
	}

	parent, err := r.beads.GetParent(b)
	if err != nil {
		r.log("Warning: failed to get parent bead: %v", err)
	}

	specID := bead.FindSpecLabel(b.Labels)
	result := &IterationResult{
		BeadID:       b.ID,
		BeadTitle:    b.Title,
		Model:        model,
		SpecID:       specID,
		OriginalTier: tier,
	}

	bc := &runtypes.BeadContext{
		Bead:               b,
		Parent:             parent,
		Result:             result,
		Model:              model, // legacy model name, will be updated by router
		Tier:               tier,
		StartCommit:        startCommit,
		Iteration:          iteration,
		MaxRetries:         r.cfg.Escalation.MaxRetriesPerModel,
		MaxRetriesPerBead:  r.cfg.Escalation.MaxRetriesPerBead,
		MaxAttemptsPerBead: r.cfg.Escalation.MaxRetriesPerBead + 1,
		ParentCtx:          ctx,
		BeadTimeout:        beadTimeout,
		RunDeadline:        runDeadline,
		BeadStartTime:      time.Now(),
		ScopeEstimate:      scopeEstimate,
	}
	if bc.MaxAttemptsPerBead < 1 {
		bc.MaxAttemptsPerBead = 1
	}

	// Preemptive escalation: if scope check is enabled, scope complexity is high,
	// and tier is medium, escalate to high before first invocation
	if r.cfg.ScopeCheck.Enabled && scopeEstimate != nil && scopeEstimate.Complexity == "high" && bc.Tier == provider.TierMedium {
		r.log("Scope check: complexity=high, preemptively escalating from medium to high tier")
		r.escalationHandler.EscalateTier(bc, provider.TierHigh)
	}

	return bc, beadCtx, beadCancel, nil
}

// buildPromptForBead performs scope checking (if enabled) and renders the build prompt.
func (r *Runner) buildPromptForBead(ctx context.Context, bc *runtypes.BeadContext, iteration int) error {
	promptCtx, err := r.renderer.BuildContext(bc.Bead, bc.Parent, iteration, bc.Model)
	if err != nil {
		return fmt.Errorf("building prompt context: %w", err)
	}
	if promptCtx == nil {
		return fmt.Errorf("building prompt context: returned nil")
	}
	bc.PromptCtx = promptCtx

	// Inject recent validation failure summaries (last 3) into prompt context
	if n := len(r.validationFailures); n > 0 {
		start := 0
		if n > 3 {
			start = n - 3
		}
		bc.PromptCtx.RecentValidationFailures = r.validationFailures[start:]
	}

	if r.cfg.ScopeCheck.Enabled {
		// Use cached scope estimate if available (from scope gate), otherwise call checkScope
		scopeEstimate := bc.ScopeEstimate
		if scopeEstimate == nil {
			scopeEstimate = r.checkScope(ctx, bc.Bead)
		}
		if scopeEstimate != nil {
			if scopeEstimate.Complexity == "high" {
				if bc.Tier != provider.TierHigh {
					r.log("Scope check: complexity=high, auto-escalating to high tier")
					r.escalationHandler.EscalateTier(bc, provider.TierHigh)
				} else {
					r.log("Scope check: complexity=high")
				}
			} else {
				r.log("Scope check: complexity=%s", scopeEstimate.Complexity)
			}
		}
	}

	buildPrompt, err := r.renderer.RenderBuild(bc.PromptCtx)
	if err != nil {
		return fmt.Errorf("rendering build prompt: %w", err)
	}
	bc.BuildPrompt = buildPrompt
	if bc.Result != nil {
		bc.Result.PromptDiagnostics = r.renderer.LastDiagnostics()
	}

	return nil
}

// executeClaudeInvocation runs a single Claude invocation with streaming, heartbeat,
// and stall detection. Returns the InvocationResult, raw provider result, and any error.
// Delegates the core invocation to execution.Invoker.Execute.
func (r *Runner) executeClaudeInvocation(ctx context.Context, bc *runtypes.BeadContext) (*runtypes.InvocationResult, *provider.Result, error) {
	if r == nil || r.invoker == nil {
		return nil, nil, fmt.Errorf("runner invoker is nil")
	}

	invResult, err := r.invoker.Execute(ctx, bc, bc.BuildPrompt)
	if err != nil && invResult == nil {
		return nil, nil, err
	}

	if invResult == nil {
		return nil, nil, err
	}
	return invResult, invResult.ProviderResult, err
}

// handleScopeTooLarge processes the scope-too-large signal from Claude.
// Always sets bc.Result.Error and returns false (stop processing).
func (r *Runner) handleScopeTooLarge(bc *runtypes.BeadContext, claudeResult *claude.Result, explanation string) {
	breakdown := claude.GetScopeTooLargeBreakdown(claudeResult)
	if breakdown == "" {
		breakdown = explanation
	}
	comment := fmt.Sprintf("Scope too large: %s\n\nThis task needs to be broken down into smaller, more manageable pieces.", breakdown)
	if err := r.beads.AddComment(bc.Bead.ID, comment); err != nil {
		r.log("Warning: failed to add comment to bead: %v", err)
	}

	// Extract synthetic learning for scope-too-large
	escalation.ExtractScopeTooLargeLearning(bc, explanation, r.renderer.GetLearningsFile())
	r.log("Synthetic learning extracted: Bead '%s' was too large for %s", bc.Bead.Title, bc.Model)

	bc.Result.Error = fmt.Errorf("scope too large: %s - needs breakdown", explanation)
}

// computeScopedTestCommand constructs an explicit "go test ./pkg/..." command
// for the given touched package paths. Returns an empty string when packages
// is nil or empty, so callers can use the generic fallback instruction.
func computeScopedTestCommand(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	targets := make([]string, 0, len(packages))
	seen := make(map[string]bool, len(packages))
	for _, pkg := range packages {
		pkg = strings.TrimPrefix(pkg, "./")
		pkg = strings.TrimSuffix(pkg, "/...")
		pkg = strings.TrimSuffix(pkg, "/")
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		if pkg == "." {
			targets = append(targets, "./...")
		} else {
			targets = append(targets, "./"+pkg+"/...")
		}
	}
	if len(targets) == 0 {
		return ""
	}
	return "go test " + strings.Join(targets, " ")
}

// injectScopedTestCommand updates bc.PromptCtx.ScopedTestCommand from bc.TouchedPackages.
// When TouchedPackages is non-empty, sets an explicit "go test ./pkg/..." command so
// build-phase prompts show the exact command instead of relying on the AI to scope tests.
// When TouchedPackages is empty, clears ScopedTestCommand so templates fall back to generic guidance.
func injectScopedTestCommand(bc *runtypes.BeadContext) {
	if bc == nil || bc.PromptCtx == nil {
		return
	}
	bc.PromptCtx.ScopedTestCommand = computeScopedTestCommand(bc.TouchedPackages)
}

// runDirectValidationCheck delegates to the validation.Runner's RunDirect method.
func (r *Runner) runDirectValidationCheck(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
	if r.validationRunner == nil {
		return nil, fmt.Errorf("validationRunner not wired — all constructors must wire validationRunner")
	}
	return r.validationRunner.RunDirect(ctx, commands, workDir)
}

// runRefactorWithRouter executes a refactor invocation using the router with automatic fallback.
// Returns the result, stream stats, and any error. This helper centralizes the router selection
// and usage limit fallback pattern used by the methodology.Executor's refactor callback.
func (r *Runner) runRefactorWithRouter(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
	if r.router == nil {
		return nil, nil, fmt.Errorf("runner router is nil")
	}

	phase := "build"
	p, modelName := r.router.Select(phase, tier)
	if p == nil {
		return nil, nil, fmt.Errorf("no providers available for phase=%s tier=%s", phase, tier)
	}

	r.log("Running refactor with model: %s", modelName)

	// Stream refactor events so this phase has the same live visibility as build.
	stats, statsErr := logger.NewStreamStats()
	if statsErr != nil {
		r.log("Warning: could not create stream stats for refactor: %v", statsErr)
	}
	streamHandler := provider.EventHandler(func(line []byte) {
		logger.ParseAndLogEvent(r.streamLogger, stats, line)
	})

	providerResult, err := p.StreamRun(ctx, prompt, tier, r.output, streamHandler, nil)

	// Check for usage limit error and retry with fallback provider
	if err != nil && p.IsUsageLimitError(providerResult, err) {
		r.router.MarkUnavailable(p.Name())

		// Retry with new provider
		p2, modelName2 := r.router.Select(phase, tier)
		if p2 != nil {
			r.log("Retrying refactor with model: %s", modelName2)
			providerResult, err = p2.StreamRun(ctx, prompt, tier, r.output, streamHandler, nil)
			modelName = modelName2
		}
	}

	// Convert provider.Result to claude.Result for compatibility
	var claudeResult *claude.Result
	if providerResult != nil {
		claudeResult = &claude.Result{
			Success:  providerResult.Success,
			Output:   providerResult.Output,
			ExitCode: providerResult.ExitCode,
			Duration: providerResult.Duration,
			Model:    modelName,
		}
	}

	return claudeResult, stats, err
}

// validationPreflight checks whether validation should run and verifies prerequisites.
// Returns true if validation should proceed, false if it should be skipped.
func (r *Runner) validationPreflight(bc *runtypes.BeadContext) (bool, error) {
	if !r.cfg.Validation.Enabled {
		return false, nil
	}

	checker, err := preflight.NewChecker(r.cfg.Preflight, r.output)
	if err != nil {
		if bc != nil && bc.Result != nil {
			bc.Result.FailurePhase = failurephase.Preflight
		}
		return false, fmt.Errorf("creating preflight checker: %w", err)
	}
	if err := checker.Check(r.cfg.Validation.FastCommandsOrDefault()); err != nil {
		r.log("Warning: %v", err)
		bc.Result.Validated = false
		return false, nil // Skip validation, not an error
	}

	return true, nil
}

// handleValidationResult processes the result of a validation run, handling both
// failure (logging, analysis, learning extraction) and success (touched packages,
// success learning, review). failureOutput is the text passed to the log and analyzer.
func (r *Runner) handleValidationResult(ctx context.Context, bc *runtypes.BeadContext, valErr error, failureOutput string, runPostSuccess bool) error {
	// Sync failure summaries from validation.Runner to facade
	r.validationFailures = r.validationRunner.Failures()

	if valErr != nil && errors.Is(valErr, validation.ErrValidationFailed) {
		bc.Result.FailurePhase = failurephase.Validation
		r.log("\nValidation failed.")
		if failureOutput != "" {
			r.log("%s", failureOutput)
		}

		logPath, logErr := logger.WriteValidationLog(r.cfg.Paths.Logs, failureOutput)
		if logErr != nil {
			r.log("Warning: could not save validation log: %v", logErr)
		} else {
			r.log("\nFull output saved to: %s", logPath)
		}

		if bc.StartCommit != "" {
			r.showPartialProgress(bc.Bead, bc.StartCommit)
		}

		// Run failure analysis (Claude only for failure interpretation)
		r.log("Running failure analysis...")
		valAnalysisCtx, valAnalysisCancel := context.WithTimeout(ctx, time.Duration(r.cfg.Claude.AnalysisTimeout)*time.Second)
		analysis, analyzeErr := r.analyzer.Analyze(valAnalysisCtx, bc.Bead, failureOutput)
		valAnalysisCancel()
		if analyzeErr == nil && analysis != nil {
			bc.Result.FailureCategory = string(analysis.Category)
			if r.renderer != nil {
				escalation.ExtractLearning(bc, analysis, r.renderer.GetLearningsFile())
			}
		}

		return errValidationFailed
	} else if valErr != nil {
		// Non-validation error (e.g., command execution failure)
		if isTimeoutOrCanceledError(valErr) {
			setPhaseAttribution(bc.Result, "validation", valErr)
		} else {
			bc.Result.FailurePhase = failurephase.Validation
		}
		return valErr
	}

	r.log("Validation passed")

	// Update runner's touched packages map for learning extraction filtering
	if len(bc.TouchedPackages) > 0 {
		r.updateTouchedPackages(bc.TouchedPackages)
	}

	// Run post-success stages sequentially.
	// For methodology flows, the first validation pass is an intermediate gate;
	// post-success stages (learning/review) should run only after final validation.
	if !runPostSuccess {
		return nil
	}

	// Run post-success stages sequentially
	learningEnabled := r.cfg != nil && r.cfg.Loop.ShouldLearnFromSuccess()
	reviewEnabled := r.cfg != nil && r.cfg.Review.Enabled

	if learningEnabled && r.renderer != nil && r.router != nil {
		lf := r.renderer.GetLearningsFile()
		adapter := &successLearningRouterAdapter{r: r.router}
		escalation.ExtractSuccessLearning(ctx, bc, r.cfg, lf, adapter, r.log, r.touchedPackages)
	}

	if reviewEnabled {
		return r.reviewer.RunPostSuccess(ctx, bc)
	}

	return nil
}

// runValidation runs the validation step (tests/lint) after a successful build.
// Delegates core command execution and failure accumulation to the validation.Runner,
// then handles facade concerns: preflight, logging, failure analysis, and post-success stages.
func (r *Runner) runValidation(ctx context.Context, bc *runtypes.BeadContext) error {
	proceed, err := r.validationPreflight(bc)
	if !proceed || err != nil {
		return err
	}

	r.log("Running validation commands directly (fast gate)...")
	commands := r.cfg.Validation.FastCommandsOrDefault()
	commands = config.ScopeGoTestCommands(commands, bc.TouchedPackages)

	// Capture output before validation to extract failure output afterward
	outputBefore := bc.Result.Output

	// Delegate core validation (command execution + failure accumulation) to validation.Runner
	valErr := r.runValidationCommandsWithElapsed(ctx, bc, commands, "fast")

	// Extract the failure output appended by the validation runner
	failureOutput := strings.TrimPrefix(bc.Result.Output, outputBefore)
	failureOutput = strings.TrimPrefix(failureOutput, runtypes.ValidationOutputHeader)

	return r.handleValidationResult(ctx, bc, valErr, failureOutput, true)
}

// runValidationWithRecovery delegates validation and recovery entirely to the
// validation.Runner's RunWithRecovery method. The facade handles only
// orchestration concerns: preflight checking, logging, failure analysis,
// learning extraction, and post-success stages (review).
func (r *Runner) runValidationWithRecovery(ctx context.Context, bc *runtypes.BeadContext) error {
	return r.runValidationWithRecoveryForStage(ctx, bc, true)
}

// runValidationWithRecoveryForStage runs validation and controls whether
// post-success stages (success learning + light review) should execute.
func (r *Runner) runValidationWithRecoveryForStage(ctx context.Context, bc *runtypes.BeadContext, runPostSuccess bool) error {
	proceed, err := r.validationPreflight(bc)
	if !proceed || err != nil {
		return err
	}

	r.log("Running validation commands directly (fast gate)...")
	commands := r.cfg.Validation.FastCommandsOrDefault()
	commands = config.ScopeGoTestCommands(commands, bc.TouchedPackages)

	// Delegate core validation + recovery to validation.Runner
	valErr := r.runValidationCommandsWithElapsed(ctx, bc, commands, "fast")

	return r.handleValidationResult(ctx, bc, valErr, bc.Result.Output, runPostSuccess)
}

func (r *Runner) maybeRunPeriodicFullValidation(ctx context.Context, beadID string, iteration int) error {
	if r == nil || r.cfg == nil || !r.cfg.Validation.Enabled {
		return nil
	}
	valPolicy := r.ensureValidationPolicy()
	if valPolicy == nil {
		return nil
	}
	if valPolicy.SelectGate(r.successesSinceFull) != policy.GateFull {
		return nil
	}
	if err := r.runFullValidationGate(ctx, beadID, iteration); err != nil {
		return err
	}
	r.successesSinceFull = 0
	return nil
}

func (r *Runner) maybeRunFinalFullValidation(ctx context.Context) error {
	if r == nil || r.cfg == nil || !r.cfg.Validation.Enabled {
		return nil
	}
	if !r.cfg.Validation.ShouldRunFinalFullGate() {
		return nil
	}
	if r.successfulBeads == 0 || len(r.cfg.Validation.FullCommandsOrDefault()) == 0 {
		return nil
	}
	if r.successesSinceFull == 0 {
		return nil
	}
	return r.runFullValidationGate(ctx, "final", 0)
}

// runCompilationCheck runs the configured compile_command before Claude invocation.
// If compilation fails, the errors are appended to the build prompt so the
// agent can fix them. Non-blocking: never prevents the bead from proceeding.
// A blank CompileCommand disables the check.
func (r *Runner) runCompilationCheck(ctx context.Context, bc *runtypes.BeadContext) {
	if r.cfg.Preflight.CompileCommand == "" {
		return
	}

	buildCtx, cancel := context.WithTimeout(ctx, compilationCheckTimeout)
	defer cancel()

	_, stderr, exitCode, _ := r.runCmd(buildCtx, r.cfg.Preflight.CompileCommand, ".")
	if exitCode == 0 {
		return
	}

	r.log("Pre-build compilation check found errors, injecting into prompt")
	bc.Result.CompilationErrors = true
	bc.BuildPrompt += fmt.Sprintf(compilationErrorsPrompt, stderr)
}

func (r *Runner) runFullValidationGate(ctx context.Context, beadID string, iteration int) error {
	workDir := "."
	b := &bead.Bead{ID: beadID, Title: "full validation gate"}
	bc := &runtypes.BeadContext{
		Bead:      b,
		ParentCtx: ctx,
		PromptCtx: &prompt.Context{WorkDir: workDir, Bead: b},
		Result: &runtypes.IterationResult{
			BeadID:    beadID,
			BeadTitle: "full validation gate",
		},
	}

	commands := r.cfg.Validation.FullCommandsOrDefault()
	if len(commands) == 0 {
		return nil
	}

	label := "periodic"
	if iteration == 0 {
		label = "final"
	}
	r.log("Running %s full validation gate (%d commands)", label, len(commands))
	valErr := r.runValidationCommandsWithElapsed(ctx, bc, commands, "full")
	if valErr == nil {
		if label == "final" {
			r.log("Final full validation gate passed")
		} else {
			r.log("Periodic full validation gate passed")
		}
		return nil
	}
	return r.handleValidationResult(ctx, bc, valErr, bc.Result.Output, false)
}

// runValidationCommandsWithElapsed executes a validation command sequence and
// accumulates elapsed wall time into the iteration result.
func (r *Runner) runValidationCommandsWithElapsed(ctx context.Context, bc *runtypes.BeadContext, commands []string, stage string) error {
	r.validationRunner.ResetElapsed()
	valErr := r.validationRunner.RunWithRecoveryForCommands(ctx, bc, commands, stage)
	bc.Result.ValidationDurationMs += r.validationRunner.ElapsedMs()
	return valErr
}
