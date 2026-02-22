package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/andon"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
	"github.com/danabrams/gromit/internal/specgate"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/tmux"
	"github.com/danabrams/gromit/internal/worktree"
)

// errValidationFailed aliases validation.ErrValidationFailed so runner internals
// and tests share one sentinel for errors.Is checks.
var errValidationFailed = validation.ErrValidationFailed

const (
	l3StopLineMarker                 = "l3 stop-line"
	validationTimedOutMarker         = "timed out"
	l3EscalationFailedCommand        = "build iteration"
	l3MutationHaltMessage            = "halting state-changing actions (close/sync/push/merge) until human decision"
	l4DecisionRequiredMessage        = "L4 decision required"
	escalationPacketHeading          = "Escalation Packet"
	l3EscalationPacketUnavailableLog = "L3 escalation packet unavailable: %v"
	l3EscalationPacketHeadingLog     = "L3 escalation packet"
	continueRetriesOptionTitle       = "Option 1: Continue autonomous retries"
	continueRetriesOptionTradeoff    = "higher throughput, but elevated risk of unsafe state mutation"
	manualRepairOptionTitle          = "Option 2: Perform manual repair and resume"
	manualRepairOptionTradeoff       = "moderate speed, but requires human intervention and context switching"
	decomposeBeadsOptionTitle        = "Option 3: Decompose into smaller beads"
	decomposeBeadsOptionTradeoff     = "lowest immediate risk, but slower delivery due to decomposition overhead"
	l3EscalationRecommendation       = "Escalate to human decision before any close/sync/push/merge actions"
	boundedRetrySummary              = "bounded autonomous retry"
	boundedRetryOutcome              = "insufficient to recover safely"
	tierEscalationSummary            = "tier escalation and bounded recovery"
	tierEscalationOutcome            = "unsafe-state remained"
)

// Runner orchestrates the Gromit loop
//
//nolint:govet // field alignment is intentionally grouped by responsibility.
type Runner struct {
	cfg                *config.Config
	providerCostDefs   map[string]config.ProviderDef // keyed by runtime provider Name(), for cost estimation
	beads              BeadClient
	router             *provider.Router
	invoker            *execution.Invoker
	escalationHandler  *escalation.Handler
	escalationPolicy   policy.EscalationPolicy
	methodologyPolicy  policy.MethodologyPolicy
	validationPolicy   policy.ValidationPolicy
	stuckPolicy        policy.StuckPolicy
	methodologyExec    *methodology.Executor
	tddOrchestrator    *tddOrchestrator
	cycleOrchestrator  *cycleOrchestrator
	validationRunner   *validation.Runner
	reviewer           *reviewpkg.Reviewer
	analyzer           FailureAnalyzer
	renderer           PromptRenderer
	logger             IterationLogger
	streamLogger       *logger.StreamLogger
	trendUpdater       *logger.AsyncTrendUpdater
	output             io.Writer
	syncOut            *syncWriter // concrete type for WriteOverwrite access
	gromitDir          string
	stateFile          *state.File                                                                                                       // promoted from Run() for router state persistence
	gitDiffFn          func(string) (string, error)                                                                                      // injectable for testing; defaults to getGitDiff
	gitHeadFn          func() (string, error)                                                                                            // injectable for testing; defaults to getGitHead
	resetHardFn        func(string) error                                                                                                // injectable for testing; defaults to git reset --hard via runArgv
	cmdRunnerFn        func(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error) // injectable for testing; defaults to defaultCmdRunner
	argvRunnerFn       runtypes.ArgvRunnerFn                                                                                             // injectable for testing; defaults to defaultArgvRunner
	processChecker     func(int) bool                                                                                                    // injectable for testing; defaults to IsProcessAlive
	autoFixFn          func(startCommit string) error                                                                                    // injectable: runs gofmt/goimports on changed files; nil means no auto-fix
	lookupHostFn       func(ctx context.Context, host string) ([]string, error)                                                          // injectable DNS lookup for codex preflight
	lookPathFn         func(file string) (string, error)                                                                                 // injectable binary lookup for codex preflight
	stdinStatFn        func() (os.FileInfo, error)                                                                                       // injectable stdin stat for interactivity checks
	promptYesNoFn      func(question string) (bool, error)                                                                               // injectable yes/no prompt for early-exit flow
	labelFilters       []string                                                                                                          // optional spec labels to filter beads
	validationFailures []string                                                                                                          // recent validation failure summaries from current run, injected into build prompts
	touchedPackages    map[string]bool                                                                                                   // packages touched in the current run, used to filter learning extraction
	worktreeManager    WorktreeManager                                                                                                   // manages interactive worktrees (optional)
	successfulBeads    int                                                                                                               // count of successful bead completions in the current run
	successesSinceFull int                                                                                                               // successful beads since last full validation gate
	specOrchestrator   *SpecOrchestrator                                                                                                 // coordinates spec-level acceptance test authoring when enabled
	specGate           *specgate.Gate                                                                                                    // evaluates spec acceptance criteria when enabled
}

// NewRunner creates a new Orchestrator that sequences the 5-stage pipeline.
func NewRunner(cfg *config.Config, output io.Writer, labels ...string) (*Orchestrator, error) {
	return newRunnerImpl(cfg, output, labels)
}

// NewRunnerWithDeps creates a runner with explicitly provided dependencies.
// This thin wrapper keeps the constructor on runner.go for compatibility with
// acceptance tests that verify wiring via AST inspection.
func NewRunnerWithDeps(cfg *config.Config, output io.Writer, gromitDir string, deps Deps) (*Runner, error) {
	return newRunnerWithDepsImpl(cfg, output, gromitDir, deps)
}

// NewRunnerLegacy creates a Runner using the old constructor for backward compatibility
// with internal tests. This function is not part of the public API and will be removed
// when all internal tests are converted to acceptance tests.
// TODO: Remove after all internal tests are converted to acceptance tests.
func NewRunnerLegacy(cfg *config.Config, output io.Writer) (*Runner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if output == nil {
		output = os.Stdout
	}

	log, err := logger.NewLogger(cfg.Paths.Logs)
	if err != nil {
		_, _ = fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
	}

	gromitDir := filepath.Dir(cfg.Paths.Templates)
	mainDir := filepath.Dir(gromitDir)

	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		cfg.Paths.Specs,
		cfg.Paths.ProjectClaudeMD,
		gromitDir,
	)
	if err != nil {
		return nil, err
	}
	renderer.SetMaxLearningChars(cfg.Learnings.MaxLearningChars)
	renderer.SetSkipBuildLearnings(cfg.Learnings.SkipBuildLearnings)
	renderer.SetBudgetConfig(cfg.Prompt.Budget.MaxChars, cfg.Prompt.Budget.LearningCapChars)

	beadsClient, err := bead.NewClient()
	if err != nil {
		return nil, err
	}

	syncOut := newSyncWriter(output)

	router, learningsProvider, sf, costDefs, err := buildRouterAndLearningsProvider(cfg, gromitDir, output)
	if err != nil {
		return nil, err
	}

	wireLearningsFilter(renderer, learningsProvider)
	wireSiblingEnrichmentResolver(renderer, cfg.Paths.Logs)

	analyzerObj, err := analyzer.NewAnalyzer(learningsProvider, cfg.Models.Validation, renderer)
	if err != nil {
		return nil, err
	}

	stallTimeoutFn := makeStallTimeoutFn(cfg)
	inv := buildInvoker(router, syncOut, stallTimeoutFn, cfg)
	policies := newRunnerPolicies(cfg)

	r := &Runner{
		cfg:               cfg,
		providerCostDefs:  costDefs,
		beads:             beadsClient,
		router:            router,
		invoker:           inv,
		analyzer:          analyzerObj,
		renderer:          renderer,
		logger:            log,
		escalationPolicy:  policies.escalation,
		methodologyPolicy: policies.methodology,
		validationPolicy:  policies.validation,
		stuckPolicy:       policies.stuck,
		output:            syncOut,
		syncOut:           syncOut,
		gromitDir:         gromitDir,
		stateFile:         sf,
		gitDiffFn:         getGitDiff,
		gitHeadFn:         getGitHead,
		cmdRunnerFn:       defaultCmdRunner,
		argvRunnerFn:      defaultArgvRunner,
		processChecker:    IsProcessAlive,
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return net.DefaultResolver.LookupHost(ctx, host)
		},
		lookPathFn:    exec.LookPath,
		stdinStatFn:   os.Stdin.Stat,
		promptYesNoFn: func(question string) (bool, error) { return promptYesNo(os.Stdin, syncOut, question) },
	}
	if cfg.Worktree.IsEnabled() {
		manager, mgrErr := worktree.NewManager(mainDir)
		if mgrErr != nil {
			return nil, mgrErr
		}
		r.worktreeManager = manager
	}

	// Note: The legacy runner constructor still initializes all the old fields.
	// These should be replaced with proper pipeline stages eventually.
	// For now, we provide minimal initialization.
	r.escalationHandler = escalation.NewHandler(cfg, analyzerObj, beadsClient, r.DecomposeTask, r.CreateSubBeads, r.log, r.showPartialProgress)
	r.validationRunner = validation.NewRunner(cfg, defaultCmdRunner, r.autoFixFn, r.makeValidationExecuteFn())
	r.methodologyExec = r.makeMethodologyExec()
	r.tddOrchestrator = r.makeTDDOrchestrator()
	r.cycleOrchestrator = &cycleOrchestrator{runner: r}
	if err := initializeSpecOrchestration(cfg, r); err != nil {
		return nil, err
	}

	reviewer := reviewpkg.NewReviewer(cfg, router, beadsClient, renderer, r.gitDiffFn, log)
	reviewer.SetLogFn(r.log)
	reviewer.SetValidateFn(r.makeReviewValidateFn())
	r.reviewer = reviewer

	return r, nil
}

// Run executes the Gromit loop.
func (r *Runner) Run(ctx context.Context, maxIterations int, deadline time.Time, stopCh <-chan struct{}, dryRun bool) error {
	if err := r.validateRunPrerequisites(); err != nil {
		return r.handleRunError(err)
	}

	// session.iterations overrides loop.max_iterations when set (> 0)
	effectiveMaxIterations := r.resolveMaxIterations(maxIterations)

	r.resetPerRunState()
	r.startTrendUpdater()
	defer r.stopTrendUpdater()

	tmuxMgr, err := tmux.NewManager()
	if err != nil {
		r.log("Warning: could not create tmux manager: %v", err)
	}
	if tmuxMgr != nil {
		defer r.restoreTmuxTitle(tmuxMgr.RestoreTitle)
	}

	if r.logger != nil {
		defer func() { _ = r.logger.Close() }()
	}

	sl, err := logger.NewStreamLogger(r.cfg.Paths.Logs)
	if err != nil {
		r.log("Warning: could not create stream logger: %v", err)
	} else {
		r.streamLogger = sl
		defer func() { _ = sl.Close() }()
		r.log("Streaming to: %s (tail -f to watch)", sl.Path())
	}

	if err := r.preflightCodex(ctx); err != nil {
		return r.handleRunError(err)
	}

	st, cleanup, err := r.initRunLoopState(deadline)
	if err != nil {
		return r.handleRunError(err)
	}
	defer cleanup()

	if err := r.authorScopedSpecAcceptanceTests(ctx, st); err != nil {
		return r.handleRunError(err)
	}

	r.log("")

	runThoroughReview := func(iteration int) {
		if st.interactiveFile != nil && r.reviewer != nil {
			// Reviewer owns review prompt construction, including review-phase rules.
			r.reviewer.RunThorough(ctx, st.interactiveFile, iteration, deadline, r.getHead)
		}
	}

runLoop:
	for {
		select {
		case <-ctx.Done():
			r.log("Context cancelled, stopping")
			return r.handleRunError(ctx.Err())
		case <-stopCh:
			r.log("Graceful stop requested, exiting after current bead")
			break runLoop
		default:
		}

		stop, loopErr := r.shouldStopLoop(st, effectiveMaxIterations, deadline)
		if loopErr != nil {
			return r.handleRunError(loopErr)
		}
		if stop {
			break runLoop
		}

		b, err := r.getNextBead(st.skippedBeads)
		if err != nil {
			return r.handleRunError(fmt.Errorf("getting next bead: %w", err))
		}
		if b == nil {
			shouldRetryScoped, err := r.verifyScopedSpecAcceptance(ctx, st, true)
			if err != nil {
				return r.handleRunError(err)
			}
			if shouldRetryScoped {
				r.log("Scoped spec gate synthesized fix beads; resuming scoped bead loop")
				continue
			}

			if len(st.skippedBeads) > 0 {
				r.log("All ready beads are blocked or stuck. Stopping loop.")
			} else {
				r.log("No more work available, stopping")
			}
			break
		}

		stop, err = r.processSingleBead(ctx, b, st, effectiveMaxIterations, deadline, dryRun, tmuxMgr, runThoroughReview)
		if err != nil {
			return r.handleRunError(err)
		}
		if stop {
			break runLoop
		}
	}

	return r.finishRun(ctx, st)
}

func (r *Runner) resolveMaxIterations(cliMax int) int {
	maxIterations := cliMax
	if r.cfg.Loop.MaxIterations > 0 {
		maxIterations = r.cfg.Loop.MaxIterations
	}
	if r.cfg.Session.Iterations > 0 {
		maxIterations = r.cfg.Session.Iterations
	}
	return maxIterations
}

func (r *Runner) restoreTmuxTitle(restoreFn func() error) {
	if r == nil || restoreFn == nil {
		return
	}
	if err := restoreFn(); err != nil {
		r.log("Warning: failed to restore tmux title: %v", err)
	}
}

func (r *Runner) processBeadWithContext(ctx context.Context, b *bead.Bead, iteration int, deadline time.Time, scopeEstimate *prompt.ScopeEstimate) (*runtypes.BeadContext, *IterationResult) {
	start := time.Now()

	bc, beadCtx, beadCancel, err := r.setupBeadContext(ctx, b, iteration, deadline, scopeEstimate)
	if err != nil {
		return nil, &IterationResult{
			BeadID:       b.ID,
			BeadTitle:    b.Title,
			Error:        err,
			FailurePhase: failurephase.Preflight,
			Duration:     time.Since(start),
		}
	}
	defer beadCancel()
	defer func() { bc.Result.Duration = time.Since(start) }()
	defer func() {
		if bc.Result != nil {
			bc.Result.ActualTier = bc.Tier
		}
	}()
	defer func() {
		if bc.StartCommit != "" {
			if diff, err := r.getDiff(bc.StartCommit); err == nil {
				bc.Result.FilesTouched = len(methodology.ParseDiffFiles(diff))
			}
		}
		bc.Result.TouchedPackages = append([]string(nil), bc.TouchedPackages...)
	}()
	ctx = beadCtx

	if err := r.buildPromptForBead(ctx, bc, iteration); err != nil {
		bc.Result.Error = err
		bc.Result.FailurePhase = failurephase.Preflight
		return bc, bc.Result
	}

	r.runCompilationCheck(ctx, bc)

	atddActive, tddActive, done := r.prepareMethodologyForBead(ctx, bc)
	if done {
		return bc, bc.Result
	}

	invokeFn := r.makeInvokeFn()
	executeWithRetry := func() bool {
		return r.escalationHandler.ExecuteWithRetry(ctx, bc, invokeFn)
	}

	return bc, r.runMethodologyExecution(ctx, bc, atddActive, tddActive, executeWithRetry)
}

func (r *Runner) runMethodologyExecution(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, tddActive bool, executeWithRetry func() bool) *IterationResult {
	if tddActive && r.cfg.Methodology.FreshContextPerCycle && r.cycleOrchestrator != nil {
		return r.cycleOrchestrator.Execute(ctx, bc, atddActive, executeWithRetry)
	}
	return r.executeBuildAndMethodologyLoop(ctx, bc, atddActive, tddActive, executeWithRetry)
}

func (r *Runner) handleAcceptanceVerificationFailure(ctx context.Context, bc *runtypes.BeadContext, stage string, err error) bool {
	var acceptanceErr *methodology.AcceptanceVerificationError
	if !errors.As(err, &acceptanceErr) {
		bc.Result.Error = fmt.Errorf("%s: %w", stage, err)
		return false
	}

	bc.Result.AcceptanceFailureSummary = acceptanceErr.Error()
	bc.Result.AcceptanceFailureOutput = acceptanceErr.Output
	bc.Result.AcceptanceFailureExitCode = acceptanceErr.ExitCode
	bc.Result.Error = fmt.Errorf("%s: %w", stage, err)

	if r.escalationHandler == nil {
		return false
	}

	failureOutput := acceptanceErr.Output
	if strings.TrimSpace(failureOutput) == "" {
		failureOutput = acceptanceErr.Error()
	}

	r.log("%s, running failure analysis for retry/escalation...", stage)
	return r.escalationHandler.AnalyzeAndHandleFailure(ctx, bc, &claude.Result{
		Success: false,
		Output:  failureOutput,
	})
}

func wrapRefactorValidationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("validation after refactoring aborted due to timeout budget exhaustion: %w", err)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("validation after refactoring aborted: %w", err)
	}
	return fmt.Errorf("validation failed after refactoring: %w", err)
}

func isTimeoutOrCanceledError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// setPhaseAttribution sets TimeoutPhase on the result when the error is a
// timeout or cancellation, attributing which phase was active when it occurred.
func setPhaseAttribution(result *IterationResult, phase string, err error) {
	if result == nil || err == nil {
		return
	}
	if isTimeoutOrCanceledError(err) {
		result.TimeoutPhase = phase
		result.FailurePhase = failurephase.Timeout
	}
}

func recordFallbackAttempt(result *IterationResult) {
	if result == nil {
		return
	}
	result.FallbackAttempts++
}

func recordFallbackOutcome(result *IterationResult, success bool) {
	if result == nil {
		return
	}
	if success {
		result.FallbackSuccesses++
		return
	}
	result.FallbackFailures++
}

func recordValidationTimeout(result *IterationResult, err error) {
	if result == nil || err == nil {
		return
	}
	if isTimeoutOrCanceledError(err) {
		result.ValidationTimeouts++
		return
	}
	if errors.Is(err, errValidationFailed) && strings.Contains(strings.ToLower(result.Output), validationTimedOutMarker) {
		result.ValidationTimeouts++
	}
}

func (r *Runner) shouldExitRunLoopOnStopLine(result *IterationResult) bool {
	if result == nil || result.Error == nil {
		return false
	}
	return strings.Contains(strings.ToLower(result.Error.Error()), l3StopLineMarker)
}

func (r *Runner) haltStateMutationsAtL3StopLine(result *IterationResult) {
	if result == nil || result.Error == nil {
		return
	}

	packet := andon.EscalationPacket{
		FailedCommand: l3EscalationFailedCommand,
		ErrorExcerpt:  result.Error.Error(),
		L1Attempts: []andon.EscalationAttempt{
			{Summary: boundedRetrySummary, Outcome: boundedRetryOutcome},
		},
		L2Attempts: []andon.EscalationAttempt{
			{Summary: tierEscalationSummary, Outcome: tierEscalationOutcome},
		},
		StateSnapshot:  fmt.Sprintf("bead=%s model=%s success=%v", result.BeadID, result.Model, result.Success),
		RiskLevel:      andon.RiskLevelHigh,
		Recommendation: l3EscalationRecommendation,
		Options: []andon.EscalationOption{
			{Title: continueRetriesOptionTitle, Tradeoff: continueRetriesOptionTradeoff},
			{Title: manualRepairOptionTitle, Tradeoff: manualRepairOptionTradeoff},
			{Title: decomposeBeadsOptionTitle, Tradeoff: decomposeBeadsOptionTradeoff},
		},
	}

	formattedPacket, err := andon.FormatEscalationPacket(packet)
	if err != nil {
		r.log(l3EscalationPacketUnavailableLog, err)
		return
	}

	r.emitEscalationPacketDetails(formattedPacket)
	r.renderL4DecisionOptions(packet.Options)
}

func (r *Runner) emitEscalationPacketDetails(formattedPacket string) {
	r.log(l3EscalationPacketHeadingLog)
	r.log(l3MutationHaltMessage)
	r.log(l4DecisionRequiredMessage)
	r.log(escalationPacketHeading)
	for _, line := range strings.Split(strings.TrimSpace(formattedPacket), "\n") {
		r.log("%s", line)
	}
}

func (r *Runner) renderL4DecisionOptions(options []andon.EscalationOption) {
	for i, option := range options {
		r.log("Option %d: %s", i+1, option.Title)
		r.log("tradeoff: %s", option.Tradeoff)
	}
}
