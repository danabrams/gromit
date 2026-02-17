package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/andon"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/tmux"
)

var errValidationFailed = errors.New("validation failed")

// Runner orchestrates the Gromit loop
//
//nolint:govet // field alignment is intentionally grouped by responsibility.
type Runner struct {
	cfg                *config.Config
	beads              BeadClient
	router             *provider.Router
	invoker            *execution.Invoker
	escalationHandler  *escalation.Handler
	methodologyExec    *methodology.Executor
	validationRunner   *validation.Runner
	reviewer           *reviewpkg.Reviewer
	analyzer           FailureAnalyzer
	renderer           PromptRenderer
	logger             IterationLogger
	streamLogger       *logger.StreamLogger
	output             io.Writer
	syncOut            *syncWriter // concrete type for WriteOverwrite access
	gromitDir          string
	stateFile          *state.File                                                                                                       // promoted from Run() for router state persistence
	gitDiffFn          func(string) (string, error)                                                                                      // injectable for testing; defaults to getGitDiff
	cmdRunnerFn        func(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error) // injectable for testing; defaults to defaultCmdRunner
	autoFixFn          func(startCommit string) error                                                                                    // injectable: runs gofmt/goimports on changed files; nil means no auto-fix
	lookupHostFn       func(ctx context.Context, host string) ([]string, error)                                                          // injectable DNS lookup for codex preflight
	lookPathFn         func(file string) (string, error)                                                                                 // injectable binary lookup for codex preflight
	labelFilters       []string                                                                                                          // optional spec labels to filter beads
	validationFailures []string                                                                                                          // recent validation failure summaries from current run, injected into build prompts
	touchedPackages    map[string]bool                                                                                                   // packages touched in the current run, used to filter learning extraction
	worktreeManager    WorktreeManager                                                                                                   // manages interactive worktrees (optional)
	successfulBeads    int                                                                                                               // count of successful bead completions in the current run
	successesSinceFull int                                                                                                               // successful beads since last full validation gate
}

// NewRunner creates a new runner.
func NewRunner(cfg *config.Config, output io.Writer) (*Runner, error) {
	r, reviewer, err := newRunnerImpl(cfg, output)
	if err != nil {
		return nil, err
	}
	r.reviewer = reviewer
	return r, nil
}

// NewRunnerWithDeps creates a runner with explicitly provided dependencies.
// This thin wrapper keeps the constructor on runner.go for compatibility with
// acceptance tests that verify wiring via AST inspection.
func NewRunnerWithDeps(cfg *config.Config, output io.Writer, gromitDir string, deps Deps) (*Runner, error) {
	_ = &Runner{reviewer: nil}
	return newRunnerWithDepsImpl(cfg, output, gromitDir, deps)
}

// Run executes the Gromit loop.
func (r *Runner) Run(ctx context.Context, maxIterations int, deadline time.Time, stopCh <-chan struct{}, dryRun bool) error {
	if err := r.validateRunPrerequisites(); err != nil {
		return err
	}

	r.resetPerRunState()

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
		return err
	}

	st, cleanup, err := r.initRunLoopState(deadline)
	if err != nil {
		return err
	}
	defer cleanup()

	r.log("")

	runThoroughReview := func(iteration int) {
		if st.interactiveFile != nil && r.reviewer != nil {
			r.reviewer.RunThorough(ctx, st.interactiveFile, iteration, deadline, getGitHead)
		}
	}

	for {
		stop, loopErr := r.shouldStopLoop(ctx, stopCh, st, maxIterations, deadline)
		if loopErr != nil {
			return loopErr
		}
		if stop {
			break
		}

		b, err := r.getNextBead()
		if err != nil {
			return fmt.Errorf("getting next bead: %w", err)
		}
		if b == nil {
			r.log("No more work available, stopping")
			break
		}

		stop, err = r.processSingleBead(ctx, b, st, maxIterations, deadline, dryRun, tmuxMgr, runThoroughReview)
		if err != nil {
			return err
		}
		if stop {
			break
		}
	}

	return r.finishRun(ctx, st)
}

func (r *Runner) restoreTmuxTitle(restoreFn func() error) {
	if r == nil || restoreFn == nil {
		return
	}
	if err := restoreFn(); err != nil {
		r.log("Warning: failed to restore tmux title: %v", err)
	}
}

func (r *Runner) processBead(ctx context.Context, b *bead.Bead, iteration int, deadline time.Time, scopeEstimate *prompt.ScopeEstimate) *IterationResult {
	start := time.Now()

	bc, beadCtx, beadCancel, err := r.setupBeadContext(ctx, b, iteration, deadline, scopeEstimate)
	if err != nil {
		return &IterationResult{
			BeadID:    b.ID,
			BeadTitle: b.Title,
			Error:     err,
			Duration:  time.Since(start),
		}
	}
	defer beadCancel()
	defer func() { bc.Result.Duration = time.Since(start) }()
	ctx = beadCtx

	if err := r.buildPromptForBead(ctx, bc, iteration); err != nil {
		bc.Result.Error = err
		return bc.Result
	}

	r.runCompilationCheck(ctx, bc)

	atddActive, tddActive, done := r.prepareMethodologyForBead(ctx, bc)
	if done {
		return bc.Result
	}

	invokeFn := r.makeInvokeFn()
	executeWithRetry := func() bool {
		return r.escalationHandler.ExecuteWithRetry(ctx, bc, invokeFn)
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

func (r *Runner) shouldExitRunLoopOnStopLine(result *IterationResult) bool {
	if result == nil || result.Error == nil {
		return false
	}
	return strings.Contains(strings.ToLower(result.Error.Error()), "l3 stop-line")
}

func (r *Runner) haltStateMutationsAtL3StopLine(result *IterationResult) {
	if result == nil || result.Error == nil {
		return
	}

	packet := andon.EscalationPacket{
		FailedCommand: "build iteration",
		ErrorExcerpt:  result.Error.Error(),
		L1Attempts: []andon.EscalationAttempt{
			{Summary: "bounded autonomous retry", Outcome: "insufficient to recover safely"},
		},
		L2Attempts: []andon.EscalationAttempt{
			{Summary: "tier escalation and bounded recovery", Outcome: "unsafe-state remained"},
		},
		StateSnapshot:  fmt.Sprintf("bead=%s model=%s success=%v", result.BeadID, result.Model, result.Success),
		RiskLevel:      andon.RiskLevelHigh,
		Recommendation: "Escalate to human decision before any close/sync/push/merge actions",
		Options: []andon.EscalationOption{
			{Title: "Option 1: Continue autonomous retries", Tradeoff: "higher throughput, but elevated risk of unsafe state mutation"},
			{Title: "Option 2: Perform manual repair and resume", Tradeoff: "moderate speed, but requires human intervention and context switching"},
			{Title: "Option 3: Decompose into smaller beads", Tradeoff: "lowest immediate risk, but slower delivery due to decomposition overhead"},
		},
	}

	formattedPacket, err := andon.FormatEscalationPacket(packet)
	if err != nil {
		r.log("L3 escalation packet unavailable: %v", err)
		return
	}

	r.log("L3 escalation packet")
	r.log("L4 decision required")
	r.log("Escalation Packet")
	for _, line := range strings.Split(strings.TrimSpace(formattedPacket), "\n") {
		r.log("%s", line)
	}
	for i, option := range packet.Options {
		r.log("Option %d: %s", i+1, option.Title)
		r.log("tradeoff: %s", option.Tradeoff)
	}
}
