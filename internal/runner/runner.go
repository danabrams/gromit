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
	"sync"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/tmux"
	"github.com/danabrams/gromit/internal/worktree"
)

var errValidationFailed = errors.New("validation failed")

// defaultTierToModelMap defines the default Claude model tier mapping.
// This is used for backward compatibility when no providers are configured.
var defaultTierToModelMap = map[string]string{
	"high":   "opus",
	"medium": "sonnet",
	"low":    "haiku",
}

// defaultCodexTierToModelMap defines the default Codex model tier mapping.
var defaultCodexTierToModelMap = map[string]string{
	"high":   "gpt-5.3-codex",
	"medium": "gpt-5.3-codex",
	"low":    "gpt-5.3-codex",
}

// Runner orchestrates the Gromit loop
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

// NewRunner creates a new runner
func NewRunner(cfg *config.Config, output io.Writer) (*Runner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if output == nil {
		output = os.Stdout
	}
	// Create logger (ignore error - logging is optional)
	log, err := logger.NewLogger(cfg.Paths.Logs)
	if err != nil {
		_, _ = fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
	}

	// Determine gromit directory (parent of templates dir)
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

	beadsClient, err := bead.NewClient()
	if err != nil {
		return nil, err
	}

	// Wrap output in synchronized writer for thread-safe writes
	syncOut := newSyncWriter(output)

	// Create router: either from providers config or wrap Claude client
	var router *provider.Router
	var claudeProviderForLearnings provider.Provider
	var sf *state.File
	if cfg.HasProviders() {
		// Apply config defaults to ensure routing fields are populated
		cfg.SetDefaults()
		cfg.NormalizeNilFields()

		// Build provider map from config
		providers := make(map[string]provider.Provider)
		for name, def := range cfg.Providers {
			switch {
			case name == "claude" || def.Binary == "claude":
				tierMap := def.Models
				if len(tierMap) == 0 {
					tierMap = defaultTierToModelMap
				}
				client, cErr := claude.NewClient(def.Binary, def.Flags, cfg.Claude.Timeout)
				if cErr != nil {
					return nil, cErr
				}
				providers[name] = provider.NewClaudeProvider(client, tierMap)
			case name == "codex" || name == "openai" || def.Binary == "codex":
				tierMap := def.Models
				if len(tierMap) == 0 {
					tierMap = defaultCodexTierToModelMap
				}
				providers[name] = provider.NewCodexProvider(def.Binary, def.Flags, tierMap)
			default:
				return nil, fmt.Errorf("unrecognized provider %q: supported providers are \"claude\" and \"codex\"", name)
			}
		}

		// Parse cooldown duration
		var cooldown time.Duration
		if cfg.Routing.Fallback.Enabled && cfg.Routing.Fallback.Cooldown != "" {
			cd, parseErr := time.ParseDuration(cfg.Routing.Fallback.Cooldown)
			if parseErr != nil {
				cooldown = 30 * time.Minute
			} else {
				cooldown = cd
			}
		}

		// Create and load state file for router initialization
		sf, err = state.NewFile(gromitDir)
		if err != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not create state file: %v\n", err)
		} else if loadErr := sf.Load(); loadErr != nil {
			_, _ = fmt.Fprintf(output, "Warning: could not load state: %v\n", loadErr)
		}

		router = provider.NewRouter(providers, cfg.Routing.PhasePreferences, cfg.Routing.Ratio, cooldown, sf)

		// Set claudeProviderForLearnings: prefer claude, fallback to first available
		if cp, ok := providers["claude"]; ok {
			claudeProviderForLearnings = cp
		} else {
			for _, p := range providers {
				claudeProviderForLearnings = p
				break
			}
		}
	} else {
		// Backward compatibility: wrap Claude client in single-provider router
		claudeClient, cErr := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
		if cErr != nil {
			return nil, cErr
		}
		claudeProvider := provider.NewClaudeProvider(claudeClient, defaultTierToModelMap)
		claudeProviderForLearnings = claudeProvider
		router = provider.NewSingleProviderRouter(claudeProvider)
	}

	// Wire filter into learnings file (after router creation so we can use the provider)
	lf := renderer.GetLearningsFile()
	if lf != nil && claudeProviderForLearnings != nil {
		// Create adapter for Provider to match learnings.ClaudeRunner interface
		providerRunnerAdapter := learnings.NewProviderRunnerAdapter(claudeProviderForLearnings)
		lf.SetFilter(learnings.NewLLMFilter(providerRunnerAdapter, "gromit", learnings.ProjectDescriptions.Gromit))
	}

	// Create analyzer using provider
	analyzerObj, err := analyzer.NewAnalyzer(claudeProviderForLearnings, cfg.Models.Validation, renderer)
	if err != nil {
		return nil, err
	}

	stallTimeoutFn := makeStallTimeoutFn(cfg)
	invocationTimeoutFn := makeInvocationTimeoutFn(cfg)
	inv := execution.NewInvoker(&routerAdapter{r: router}, syncOut, nil).
		WithHeartbeat(syncOut, stallTimeoutFn).
		WithInvocationTimeout(invocationTimeoutFn)

	r := &Runner{
		cfg:         cfg,
		beads:       beadsClient,
		router:      router,
		invoker:     inv,
		analyzer:    analyzerObj,
		renderer:    renderer,
		logger:      log,
		output:      syncOut,
		syncOut:     syncOut,
		gromitDir:   gromitDir,
		stateFile:   sf,
		gitDiffFn:   getGitDiff,
		cmdRunnerFn: defaultCmdRunner,
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return net.DefaultResolver.LookupHost(ctx, host)
		},
		lookPathFn: exec.LookPath,
	}
	if cfg.Worktree.IsEnabled() {
		manager, mgrErr := worktree.NewManager(mainDir)
		if mgrErr != nil {
			return nil, mgrErr
		}
		r.worktreeManager = manager
	}
	r.escalationHandler = escalation.NewHandler(cfg, analyzerObj, beadsClient, r.DecomposeTask, r.CreateSubBeads, r.log, r.showPartialProgress)
	r.validationRunner = validation.NewRunner(cfg, defaultCmdRunner, r.autoFixFn, r.makeValidationExecuteFn())
	r.methodologyExec = r.makeMethodologyExec()
	r.reviewer = reviewpkg.NewReviewer(cfg, router, beadsClient, renderer, r.gitDiffFn, log)
	r.reviewer.SetLogFn(r.log)
	r.reviewer.SetValidateFn(r.makeReviewValidateFn())
	return r, nil
}

// Deps holds injectable dependencies for a Runner, used for testing.
type Deps struct {
	Beads     BeadClient
	Router    *provider.Router
	Analyzer  FailureAnalyzer
	Renderer  PromptRenderer
	Logger    IterationLogger
	CmdRunner func(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error)
}

// NewRunnerWithDeps creates a runner with explicitly provided dependencies.
// This is primarily intended for testing, where you want to inject mocks.
func NewRunnerWithDeps(cfg *config.Config, output io.Writer, gromitDir string, deps Deps) (*Runner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	// Apply config defaults to ensure consistent behavior even when config is
	// partially initialized in tests. This prevents accidental precheck execution
	// or other features in tests that don't explicitly test them.
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if output == nil {
		output = os.Stdout
	}
	// Wrap output in synchronized writer for thread-safe writes
	syncOut := newSyncWriter(output)

	// Create logger if not provided and Logs path is configured
	iterLogger := deps.Logger
	if iterLogger == nil && cfg.Paths.Logs != "" {
		log, err := logger.NewLogger(cfg.Paths.Logs)
		if err != nil {
			// Log warning but continue - logging is optional
			_, _ = fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
		} else {
			iterLogger = log
		}
	}

	// Use provided Router
	router := deps.Router

	// Create invoker with router adapter (nil-safe: if router is nil, invoker handles it)
	stallTimeoutFn := makeStallTimeoutFn(cfg)
	invocationTimeoutFn := makeInvocationTimeoutFn(cfg)
	var inv *execution.Invoker
	if router != nil {
		inv = execution.NewInvoker(&routerAdapter{r: router}, syncOut, nil).
			WithHeartbeat(syncOut, stallTimeoutFn).
			WithInvocationTimeout(invocationTimeoutFn)
	} else {
		inv = execution.NewInvoker(nil, syncOut, nil).
			WithHeartbeat(syncOut, stallTimeoutFn).
			WithInvocationTimeout(invocationTimeoutFn)
	}

	cmdRunner := defaultCmdRunner
	if deps.CmdRunner != nil {
		cmdRunner = deps.CmdRunner
	}

	r := &Runner{
		cfg:         cfg,
		beads:       deps.Beads,
		router:      router,
		invoker:     inv,
		analyzer:    deps.Analyzer,
		renderer:    deps.Renderer,
		logger:      iterLogger,
		output:      syncOut,
		syncOut:     syncOut,
		gromitDir:   gromitDir,
		gitDiffFn:   getGitDiff,
		cmdRunnerFn: cmdRunner,
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return net.DefaultResolver.LookupHost(ctx, host)
		},
		lookPathFn: exec.LookPath,
	}
	r.escalationHandler = escalation.NewHandler(cfg, deps.Analyzer, deps.Beads, r.DecomposeTask, r.CreateSubBeads, r.log, r.showPartialProgress)
	r.validationRunner = validation.NewRunner(cfg, cmdRunner, r.autoFixFn, r.makeValidationExecuteFn())
	r.methodologyExec = r.makeMethodologyExec()
	r.reviewer = reviewpkg.NewReviewer(cfg, router, deps.Beads, deps.Renderer, r.gitDiffFn, iterLogger)
	r.reviewer.SetLogFn(r.log)
	r.reviewer.SetValidateFn(r.makeReviewValidateFn())
	return r, nil
}

// IterationResult captures the outcome of one loop iteration.
// Type alias for backward compatibility — canonical definition is in runtypes.
type IterationResult = runtypes.IterationResult

// SubTask represents a single sub-task from task decomposition.
// Type alias for backward compatibility — canonical definition is in runtypes.
type SubTask = runtypes.SubTask

// Run executes the Gromit loop
func (r *Runner) Run(ctx context.Context, maxIterations int, deadline time.Time, stopCh <-chan struct{}, dryRun bool) error {
	if r == nil {
		return fmt.Errorf("runner is nil")
	}
	if r.cfg == nil {
		return fmt.Errorf("runner config is nil")
	}
	if r.beads == nil {
		return fmt.Errorf("runner beads client is nil")
	}
	if r.renderer == nil {
		return fmt.Errorf("runner renderer is nil")
	}
	if r.router == nil {
		return fmt.Errorf("runner router is nil")
	}

	// Reset per-run state
	r.validationFailures = []string{}
	r.touchedPackages = make(map[string]bool)
	r.successfulBeads = 0
	r.successesSinceFull = 0
	if r.validationRunner != nil {
		r.validationRunner.ResetFailures()
	}

	// Set up tmux title management (no-op if not in tmux)
	tmuxMgr, err := tmux.NewManager()
	if err != nil {
		r.log("Warning: could not create tmux manager: %v", err)
	}
	if tmuxMgr != nil {
		defer r.restoreTmuxTitle(tmuxMgr.RestoreTitle)
	}

	// Set up status file management
	statusWriter, err := NewStatusWriter(r.gromitDir)
	if err != nil {
		r.log("Warning: could not create status writer: %v", err)
	}
	var finalIteration *int
	if statusWriter != nil {
		defer func() {
			if finalIteration != nil {
				_ = statusWriter.WriteFinal(*finalIteration)
			}
		}()
	}

	// Calculate time budget in minutes if deadline is set
	var timeBudgetMinutes int
	if !deadline.IsZero() {
		timeBudgetMinutes = int(time.Until(deadline).Minutes())
	}

	// Ensure logger is closed when done
	if r.logger != nil {
		defer func() { _ = r.logger.Close() }()
		r.log("Logging to: %s", r.logger.FilePath())
	}

	// Create stream logger for firehose streaming
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

	iteration := 0
	consecutiveSkips := 0

	// Read per-bead statistics once before the main loop for efficiency
	beadStats, err := logger.ReadPerBeadStats(r.cfg.Paths.Logs)
	if err != nil {
		r.log("Warning: could not read bead stats: %v", err)
		beadStats = make(map[string]logger.BeadStats) // Use empty stats on error
	}

	// Track skipped bead IDs to avoid infinite loops
	skippedBeads := make(map[string]bool)

	// Load state for review tracking — reuse state file from NewRunner if available
	sf := r.stateFile
	if sf == nil {
		sf, err = state.NewFile(r.gromitDir)
		if err != nil {
			r.log("Warning: could not create state file: %v", err)
			sf = nil
		} else if err := sf.Load(); err != nil {
			r.log("Warning: could not load state: %v", err)
		}
	}

	// Load interactive state for review baseline and retro tracking
	var interactiveFile *state.InteractiveFile
	interactiveFile, err = state.NewInteractiveFile(r.gromitDir)
	if err != nil {
		r.log("Warning: could not create interactive state file: %v", err)
		interactiveFile = nil
	} else if err := interactiveFile.Load(); err != nil {
		r.log("Warning: could not load interactive state: %v", err)
	}

	// Check for stale state and auto-heal if needed
	if sf != nil {
		if isStale, reason := sf.CheckStaleness(r.cfg.State.StaleThreshold); isStale {
			r.log("Warning: %s — auto-healing state (resetting iteration counters)", reason)
			sf.AutoHeal()
		}

		// Set clean_exit to false and save immediately
		sf.SetCleanExit(false)
		if err := sf.Save(); err != nil {
			r.log("Warning: could not save state after setting clean_exit: %v", err)
		}
	}

	// Initialize review baseline if not set
	if interactiveFile != nil && interactiveFile.LastReviewCommit() == "" {
		currentCommit, err := getGitHead()
		if err == nil && currentCommit != "" {
			if err := interactiveFile.RecordReview(currentCommit, 0); err != nil {
				r.log("Warning: could not initialize review baseline: %v", err)
			} else {
				r.log("Initialized review baseline at commit %s", currentCommit[:8])
			}
		}
	}

	r.log("")

runLoop:
	for {
		// Check for context cancellation or graceful stop
		select {
		case <-ctx.Done():
			r.log("Context cancelled, stopping")
			return ctx.Err()
		case <-stopCh:
			r.log("Graceful stop requested, exiting after current bead")
			break runLoop
		default:
		}

		// Check iteration limit
		if maxIterations > 0 && iteration >= maxIterations {
			r.log("Reached max iterations (%d), stopping", maxIterations)
			break
		}

		// Check deadline
		if !deadline.IsZero() && time.Now().After(deadline) {
			r.log("Time budget expired, stopping")
			break
		}

		// Get next bead (with optional label filtering)
		b, err := r.getNextBead()
		if err != nil {
			return fmt.Errorf("getting next bead: %w", err)
		}

		if b == nil {
			r.log("No more work available, stopping")
			break
		}

		// Check if we've already skipped this bead in this run (e.g., Close failure after precheck)
		if skippedBeads[b.ID] {
			r.log("All ready beads are stuck and have been skipped. Stopping loop.")
			break
		}

		// Check if bead is stuck (too many cross-run failures)
		if r.isStuckBeadWithStats(b, beadStats) {

			stats := beadStats[b.ID]
			r.log("Bead %s marked as stuck (exceeded failure threshold), skipping", b.ID)
			// Add comment explaining why we're skipping this bead (do NOT close it)
			comment := fmt.Sprintf("Skipped after %d failures (exceeded threshold of %d). Please review and break down into smaller tasks if needed.", stats.Failures, r.cfg.Loop.StuckBeadThreshold)
			if err := r.beads.AddComment(b.ID, comment); err != nil {
				r.log("Warning: failed to add comment to stuck bead: %v", err)
			}
			if err := r.beads.Sync(); err != nil {
				r.log("Warning: failed to sync beads: %v", err)
			}

			// Mark this bead as skipped in this run to avoid infinite loop
			skippedBeads[b.ID] = true
			continue
		}

		// Run precheck to see if acceptance criteria are already met
		passed, precheckDuration := r.runPrecheck(ctx, b)
		if passed {
			// Precheck skips count as iterations so -n limits bound total work
			// (real builds + auto-closes), preventing large unexpected cascades.
			iteration++
			r.log("auto-closing bead %s", b.ID)

			// Close the bead
			if err := r.beads.Close(b.ID); err != nil {
				r.log("Warning: failed to close bead: %v", err)
				// Add to skippedBeads so stuck-bead detection will catch it on next iteration
				skippedBeads[b.ID] = true
			}

			// Sync bd state
			if err := r.beads.Sync(); err != nil {
				r.log("Warning: failed to sync beads: %v", err)
			}

			// Write iteration log with precheck_skipped outcome
			// Note: we don't increment iteration counter for skipped beads
			r.logIterationWithWarning(&logger.IterationLog{
				Timestamp:  time.Now(),
				Iteration:  iteration,
				BeadID:     b.ID,
				BeadTitle:  b.Title,
				Model:      "precheck",
				Success:    true,
				DurationMs: precheckDuration.Milliseconds(),
				Outcome:    "precheck_skipped",
			})

			// Increment consecutive skip counter and check limit
			consecutiveSkips++
			if consecutiveSkips >= r.cfg.Loop.MaxConsecutiveSkips {
				return fmt.Errorf("reached max consecutive precheck skips (%d) — bd may not be persisting bead closures correctly", r.cfg.Loop.MaxConsecutiveSkips)
			}

			continue
		}

		// Scope gate: block over-scoped beads before spending compute
		// Store the estimate so it can be reused in buildPromptForBead (avoid duplicate LLM calls)
		var scopeEstimate *prompt.ScopeEstimate
		if r.cfg.ScopeCheck.Enabled && r.cfg.ScopeCheck.ShouldBlockOversized() {
			estimate := r.checkScope(ctx, b)
			scopeEstimate = estimate // Cache for later use
			if estimate != nil {
				blocked := false
				var reason string
				if !estimate.CanCompleteInSingleIteration {
					blocked = true
					reason = fmt.Sprintf("scope check: cannot complete in single iteration (complexity=%s, estimated_iterations=%d)", estimate.Complexity, estimate.EstimatedIterations)
				} else if estimate.Complexity == "high" && len(estimate.Blockers) > 0 {
					blocked = true
					reason = fmt.Sprintf("scope check: high complexity with blockers (%s)", strings.Join(estimate.Blockers, "; "))
				}
				if blocked {
					r.log("Blocking bead %s: %s", b.ID, reason)
					comment := fmt.Sprintf("Blocked by scope gate: %s. Please decompose into smaller tasks.", reason)
					if err := r.beads.AddComment(b.ID, comment); err != nil {
						r.log("Warning: failed to add comment to blocked bead: %v", err)
					}
					skippedBeads[b.ID] = true
					r.logIterationWithWarning(&logger.IterationLog{
						Timestamp: time.Now(),
						Iteration: iteration + 1,
						BeadID:    b.ID,
						BeadTitle: b.Title,
						Model:     r.cfg.ScopeCheck.Model,
						Success:   false,
						Outcome:   "scope_blocked",
					})
					continue
				}
			}
		}

		// Print separator between iterations (not before first)
		if iteration > 0 {
			r.log("")
		}

		iteration++
		r.log("=== Iteration %d ===", iteration)
		r.log("Bead: %s - %s", b.ID, b.Title)

		// Update tmux pane title with iteration info
		model := escalation.SelectModel(r.cfg, b)
		if tmuxMgr != nil {
			if err := tmuxMgr.SetTitle(tmux.FormatIterationTitle(iteration, b.ID, model)); err != nil {
				// Log but don't fail on tmux error
				r.log("Warning: failed to set tmux title: %v", err)
			}
		}

		// Write status.json at iteration start
		var stopStatusHeartbeat func()
		if statusWriter != nil {
			if err := statusWriter.Write(iteration, b.ID, b.Title, model, true, maxIterations, timeBudgetMinutes); err != nil {
				r.log("Warning: failed to write status.json: %v", err)
			}

			stopCh := make(chan struct{})
			var stopOnce sync.Once
			stopStatusHeartbeat = func() {
				stopOnce.Do(func() { close(stopCh) })
			}
			go func(iteration int, beadID, beadTitle, model string) {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if err := statusWriter.Write(iteration, beadID, beadTitle, model, true, maxIterations, timeBudgetMinutes); err != nil {
							r.log("Warning: failed to write status.json: %v", err)
						}
					case <-stopCh:
						return
					}
				}
			}(iteration, b.ID, b.Title, model)
		}

		if dryRun {
			if stopStatusHeartbeat != nil {
				stopStatusHeartbeat()
			}
			r.log("[DRY RUN] Would process bead %s with model %s", b.ID, model)
			continue
		}

		// Process the bead (pass cached scope estimate to avoid duplicate LLM calls)
		result := r.processBead(ctx, b, iteration, deadline, scopeEstimate)
		if stopStatusHeartbeat != nil {
			stopStatusHeartbeat()
		}

		r.log("")

		// Log result to console
		r.logResult(result)

		// Log result to file
		r.writeIterationLog(iteration, result)

		// Reset consecutive skip counter after any real build attempt
		consecutiveSkips = 0

		// Handle failure
		if !result.Success {
			// Update in-memory stats so stuck bead detection works within this run
			stats := beadStats[b.ID]
			stats.BeadID = b.ID
			stats.BeadTitle = b.Title
			stats.Failures++
			stats.TotalRuns++
			stats.LastAttempt = time.Now()
			beadStats[b.ID] = stats

			// Review re-validation failures are always critical: the working tree
			// is in a broken state after the review applied bad fixes.
			if result.ReviewBrokeValidation {
				return fmt.Errorf("bead %s failed: %v", b.ID, result.Error)
			}

			if r.cfg.Loop.StopOnFailure {
				return fmt.Errorf("bead %s failed: %v", b.ID, result.Error)
			}
			r.log("Continuing to next bead despite failure")
			continue
		}

		r.successfulBeads++
		r.successesSinceFull++
		if err := r.maybeRunPeriodicFullValidation(ctx, b.ID, iteration); err != nil {
			return err
		}

		// Mark bead as complete
		if err := r.beads.Close(b.ID); err != nil {
			r.log("Warning: failed to close bead: %v", err)
		}

		// Sync bd state
		if err := r.beads.Sync(); err != nil {
			r.log("Warning: failed to sync beads: %v", err)
		}

		// Update status.json after completion
		if statusWriter != nil {
			if err := statusWriter.Write(iteration, b.ID, b.Title, result.Model, true, maxIterations, timeBudgetMinutes); err != nil {
				r.log("Warning: failed to write status.json: %v", err)
			}
		}

		// Push to remote if configured
		if err := r.runGitAutoPush(); err != nil {
			return fmt.Errorf("git auto-push failed: %w", err)
		}

		if err := r.mergeInteractiveBranches(); err != nil {
			return err
		}

		// Run between-iterations command if configured
		r.runBetweenIterationsCommand()

		// Check epic completion trigger
		if b.Parent != "" && r.cfg.Review.Thorough.Enabled && r.cfg.Review.Thorough.ShouldRunOnEpicComplete() {
			hasChildren, err := r.beads.HasOpenChildren(b.Parent)
			if err != nil {
				r.log("Warning: could not check epic children: %v", err)
			} else if !hasChildren {
				r.log("\n=== Thorough Review (epic %s complete) ===", b.Parent)
				if interactiveFile != nil {
					r.reviewer.RunThorough(ctx, interactiveFile, iteration, deadline, getGitHead)
				}
			}
		}

		// Increment iterations since last review
		if sf != nil {
			sf.IncrementIterationsSinceReview()
			if err := sf.Save(); err != nil {
				r.log("Warning: could not save state: %v", err)
			}

			// Check thorough review trigger
			if r.cfg.Review.Thorough.Enabled && sf.IterationsSinceReview() >= r.cfg.Review.Thorough.EveryNIterations {
				r.log("\n=== Thorough Review (every %d iterations) ===", r.cfg.Review.Thorough.EveryNIterations)
				if interactiveFile != nil {
					r.reviewer.RunThorough(ctx, interactiveFile, iteration, deadline, getGitHead)
				}
			}
		}
	}

	r.log("\nGromit loop complete. Processed %d iterations.", iteration)

	if err := r.maybeRunFinalFullValidation(ctx); err != nil {
		return err
	}

	// Update global stats if at least one iteration processed
	if iteration > 0 && r.logger != nil {
		r.updateGlobalStats()
	}

	// Set final iteration count for deferred status write
	finalIteration = &iteration

	// Mark clean exit before returning
	if sf != nil {
		sf.SetCleanExit(true)
		if err := sf.Save(); err != nil {
			r.log("Warning: could not save state after clean exit: %v", err)
		}
	}

	// Check if retro should be suggested
	r.checkRetroSuggestion()

	return nil
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

	// Set up bead context: validate state, timeouts, git capture, model selection
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

	// Build prompt (with optional scope check)
	if err := r.buildPromptForBead(ctx, bc, iteration); err != nil {
		bc.Result.Error = err
		return bc.Result
	}

	// Check if ATDD is active for this bead
	atddActive := bead.IsMethodologyActive(bc.Bead.Labels, "atdd", r.cfg.Methodology.ATDD)

	// Skip ATDD for test-only beads — their deliverable IS tests
	if atddActive && bead.IsTestOnlyBead(bc.Bead.Title) {
		r.log("Skipping ATDD: bead is test-only")
		atddActive = false
	}

	// ATDD Phase 1: Write acceptance tests (if ATDD active)
	if atddActive {
		if r.methodologyExec == nil {
			bc.Result.Error = fmt.Errorf("ATDD active but methodologyExec not wired")
			return bc.Result
		}
		r.log("ATDD enabled, writing acceptance tests first...")
		if err := r.methodologyExec.RunAcceptanceTestsWithRetry(ctx, bc); err != nil {
			bc.Result.Error = fmt.Errorf("acceptance tests phase failed: %w", err)
			return bc.Result
		}

		// ATDD Phase 2: Verify tests fail (as expected before implementation)
		// Skip for file-creation beads — acceptance criteria like "build passes"
		// are tautologically true before AND after refactoring, so verify-fail
		// always triggers false "already done" auto-closes.
		skipVerifyFail := false
		if parsed := extractExpectedFiles(bc.Bead.Description); len(parsed) > 0 && anyFileMissing(parsed) {
			r.log("Skipping ATDD verify-fail: bead creates files that don't exist yet (structural change)")
			skipVerifyFail = true
		}

		if !skipVerifyFail {
			if err := r.methodologyExec.VerifyTestsFailWithRetry(ctx, bc); err != nil {
				if methodology.IsATDDAlreadyDone(err) {
					bc.Result.Success = true
					bc.Result.AlreadyDone = true
					return bc.Result
				}
				bc.Result.Error = err
				return bc.Result
			}
		}

		// Update build prompt to indicate acceptance tests are ready
		bc.PromptCtx.IsRetry = false // Clear any retry flags
		bc.PromptCtx.PrevFailure = ""
		bc.PromptCtx.FailureContext = "Acceptance tests have been written and committed. Your job is to make them pass."
		var err error
		bc.BuildPrompt, err = r.renderer.RenderATDDBuild(bc.PromptCtx)
		if err != nil {
			bc.Result.Error = fmt.Errorf("rendering ATDD build prompt: %w", err)
			return bc.Result
		}
	}

	// Check if TDD is active for this bead (after ATDD check so TDD overrides when both are active)
	tddActive := bead.IsMethodologyActive(bc.Bead.Labels, "tdd", r.cfg.Methodology.TDD)
	if tddActive {
		r.log("TDD enabled, using TDD build prompt with red-green-refactor cycles...")
		var err error
		bc.BuildPrompt, err = r.renderer.RenderTDDBuild(bc.PromptCtx)
		if err != nil {
			bc.Result.Error = fmt.Errorf("rendering TDD build prompt: %w", err)
			return bc.Result
		}
	}

	// Main execution loop with retry and escalation — delegated to escalation handler.
	// Post-build ATDD acceptance verification failures are routed through the same
	// analysis/retry path so recoverable failures can get a fresh invocation context.
	invokeFn := r.makeInvokeFn()
	for {
		bc.Result.Error = nil
		bc.Result.AcceptanceFailureSummary = ""
		bc.Result.AcceptanceFailureOutput = ""

		if !r.escalationHandler.ExecuteWithRetry(ctx, bc, invokeFn) {
			return bc.Result
		}

		// Capture touched packages for learning extraction filtering
		if bc.StartCommit != "" {
			diff, err := r.getDiff(bc.StartCommit)
			if err == nil && diff != "" {
				bc.TouchedPackages = detectTouchedPackages(diff)
			}
		}

		// Run validation if enabled (with recovery on failure)
		if err := r.runValidationWithRecovery(ctx, bc); err != nil {
			bc.Result.Error = err
			return bc.Result
		}

		// ATDD: Verify acceptance tests pass after build + regular validation
		if atddActive && r.methodologyExec != nil {
			if err := r.methodologyExec.VerifyAcceptanceTestsPass(ctx, bc); err != nil {
				if r.handleAcceptanceVerificationFailure(ctx, bc, "post-build acceptance verification", err) {
					continue
				}
				return bc.Result
			}
		}

		// ATDD/TDD Phase 3: Refactor (if either methodology is active)
		if atddActive || tddActive {
			r.log("Running refactor phase...")
			if r.methodologyExec == nil {
				bc.Result.Error = fmt.Errorf("refactor phase active but methodologyExec not wired")
				return bc.Result
			}
			if err := r.methodologyExec.RunRefactorPhase(ctx, bc); err != nil {
				r.log("Warning: refactor phase encountered issues: %v", err)
			}

			// Re-validate after refactoring (with recovery on failure)
			if r.cfg.Validation.Enabled {
				if err := r.runValidationWithRecovery(ctx, bc); err != nil {
					bc.Result.Error = wrapRefactorValidationError(err)
					return bc.Result
				}
			}

			// ATDD: Re-verify acceptance tests pass after refactoring
			if atddActive && r.methodologyExec != nil {
				if err := r.methodologyExec.VerifyAcceptanceTestsPass(ctx, bc); err != nil {
					if r.handleAcceptanceVerificationFailure(ctx, bc, "acceptance verification failed after refactoring", err) {
						continue
					}
					return bc.Result
				}
			}
		}

		bc.Result.Success = true
		return bc.Result
	}
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
