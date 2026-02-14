package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
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
	"github.com/danabrams/gromit/internal/usagelimit"
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
	labelFilters       []string                                                                                                          // optional spec labels to filter beads
	validationFailures []string                                                                                                          // recent validation failure summaries from current run, injected into build prompts
	touchedPackages    map[string]bool                                                                                                   // packages touched in the current run, used to filter learning extraction
	worktreeManager    WorktreeManager                                                                                                   // manages interactive worktrees (optional)
}

// routerAdapter wraps *provider.Router to satisfy execution.Router.
// The adapter narrows the return type from provider.Provider to execution.Provider.
type routerAdapter struct {
	r *provider.Router
}

func (a *routerAdapter) Select(phase, tier string) (execution.Provider, string) {
	p, model := a.r.Select(phase, tier)
	if p == nil {
		return nil, ""
	}
	return p, model
}

func (a *routerAdapter) MarkUnavailable(name string) {
	a.r.MarkUnavailable(name)
}

// makeStallTimeoutFn creates a StallTimeoutFunc that looks up per-model stall
// timeouts from the config. Used to configure the invoker's heartbeat.
func makeStallTimeoutFn(cfg *config.Config) execution.StallTimeoutFunc {
	if cfg == nil {
		return nil
	}
	return func(model string) (time.Duration, time.Duration) {
		_, st, sta, _ := cfg.Claude.TimeoutsForModel(model)
		return time.Duration(st) * time.Second, time.Duration(sta) * time.Second
	}
}

// successLearningRouterAdapter wraps *provider.Router to satisfy escalation.SuccessLearningRouter.
type successLearningRouterAdapter struct {
	r *provider.Router
}

func (a *successLearningRouterAdapter) Select(phase, tier string) (escalation.SuccessLearningProvider, string) {
	p, model := a.r.Select(phase, tier)
	if p == nil {
		return nil, ""
	}
	return &successLearningProviderAdapter{p: p}, model
}

// successLearningProviderAdapter wraps provider.Provider to satisfy escalation.SuccessLearningProvider.
type successLearningProviderAdapter struct {
	p provider.Provider
}

func (a *successLearningProviderAdapter) Run(ctx context.Context, prompt string, tier string) (escalation.SuccessLearningResult, error) {
	result, err := a.p.Run(ctx, prompt, tier)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &successLearningResultAdapter{r: result}, nil
}

// successLearningResultAdapter wraps *provider.Result to satisfy escalation.SuccessLearningResult.
type successLearningResultAdapter struct {
	r *provider.Result
}

func (a *successLearningResultAdapter) IsSuccess() bool   { return a.r.Success }
func (a *successLearningResultAdapter) GetOutput() string { return a.r.Output }

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
		fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
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
			fmt.Fprintf(output, "Warning: could not create state file: %v\n", err)
		} else if loadErr := sf.Load(); loadErr != nil {
			fmt.Fprintf(output, "Warning: could not load state: %v\n", loadErr)
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
	inv := execution.NewInvoker(&routerAdapter{r: router}, syncOut, nil).
		WithHeartbeat(syncOut, stallTimeoutFn)

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
			fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
		} else {
			iterLogger = log
		}
	}

	// Use provided Router
	router := deps.Router

	// Create invoker with router adapter (nil-safe: if router is nil, invoker handles it)
	stallTimeoutFn := makeStallTimeoutFn(cfg)
	var inv *execution.Invoker
	if router != nil {
		inv = execution.NewInvoker(&routerAdapter{r: router}, syncOut, nil).
			WithHeartbeat(syncOut, stallTimeoutFn)
	} else {
		inv = execution.NewInvoker(nil, syncOut, nil).
			WithHeartbeat(syncOut, stallTimeoutFn)
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
func (r *Runner) Run(ctx context.Context, maxIterations int, deadline time.Time, dryRun bool) error {
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
	if r.validationRunner != nil {
		r.validationRunner.ResetFailures()
	}

	// Set up tmux title management (no-op if not in tmux)
	tmuxMgr, err := tmux.NewManager()
	if err != nil {
		r.log("Warning: could not create tmux manager: %v", err)
	}
	if tmuxMgr != nil {
		defer tmuxMgr.RestoreTitle()
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
		defer r.logger.Close()
		r.log("Logging to: %s", r.logger.FilePath())
	}

	// Create stream logger for firehose streaming
	sl, err := logger.NewStreamLogger(r.cfg.Paths.Logs)
	if err != nil {
		r.log("Warning: could not create stream logger: %v", err)
	} else {
		r.streamLogger = sl
		defer sl.Close()
		r.log("Streaming to: %s (tail -f to watch)", sl.Path())
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

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			r.log("Context cancelled, stopping")
			return ctx.Err()
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
			if r.logger != nil {
				r.logger.LogIteration(&logger.IterationLog{
					Timestamp:  time.Now(),
					Iteration:  iteration + 1,
					BeadID:     b.ID,
					BeadTitle:  b.Title,
					Model:      "haiku",
					Success:    true,
					DurationMs: precheckDuration.Milliseconds(),
					Outcome:    "precheck_skipped",
				})
			}

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
					if r.logger != nil {
						r.logger.LogIteration(&logger.IterationLog{
							Timestamp: time.Now(),
							Iteration: iteration + 1,
							BeadID:    b.ID,
							BeadTitle: b.Title,
							Model:     r.cfg.ScopeCheck.Model,
							Success:   false,
							Outcome:   "scope_blocked",
						})
					}
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
		if statusWriter != nil {
			if err := statusWriter.Write(iteration, b.ID, b.Title, model, true, maxIterations, timeBudgetMinutes); err != nil {
				r.log("Warning: failed to write status.json: %v", err)
			}
		}

		if dryRun {
			r.log("[DRY RUN] Would process bead %s with model %s", b.ID, model)
			continue
		}

		// Process the bead (pass cached scope estimate to avoid duplicate LLM calls)
		result := r.processBead(ctx, b, iteration, deadline, scopeEstimate)

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

func (r *Runner) writeIterationLog(iteration int, result *IterationResult) {
	if r == nil || r.logger == nil || result == nil {
		return
	}

	errStr := ""
	if result.Error != nil {
		errStr = result.Error.Error()
	}

	outcome := ""
	if result.AlreadyDone {
		outcome = "atdd_already_done"
	}

	r.logger.LogIteration(&logger.IterationLog{
		Timestamp:           time.Now(),
		Iteration:           iteration,
		BeadID:              result.BeadID,
		BeadTitle:           result.BeadTitle,
		Model:               result.Model,
		Success:             result.Success,
		Validated:           result.Validated,
		Escalated:           result.Escalated,
		EscalatedTo:         result.EscalatedTo,
		DurationMs:          result.Duration.Milliseconds(),
		CostUSD:             result.CostUSD,
		InputTokens:         result.InputTokens,
		OutputTokens:        result.OutputTokens,
		Error:               errStr,
		Outcome:             outcome,
		ValidationRetried:   result.ValidationRetried,
		TrivialAutoFixed:    result.TrivialAutoFixed,
		UsageLimited:        result.UsageLimited,
		ValidationMode:      result.ValidationMode,
		TimeoutType:         result.TimeoutType,
		TimeToFirstEventMs:  result.TimeToFirstEventMs,
		ToolCallCount:       result.ToolCallCount,
		StallCount:          result.StallCount,
		StallTier:           result.StallTier,
		RateLimitHits:       result.RateLimitHits,
		RateLimitRecoveryMs: result.RateLimitRecoveryMs,
	})
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
		if err := r.methodologyExec.VerifyTestsFailWithRetry(ctx, bc); err != nil {
			if methodology.IsATDDAlreadyDone(err) {
				bc.Result.Success = true
				bc.Result.AlreadyDone = true
				return bc.Result
			}
			bc.Result.Error = err
			return bc.Result
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

	// Main execution loop with retry and escalation — delegated to escalation handler
	invokeFn := r.makeInvokeFn()
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
			bc.Result.Error = fmt.Errorf("post-build acceptance verification: %w", err)
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
				bc.Result.Error = fmt.Errorf("validation failed after refactoring: %w", err)
				return bc.Result
			}
		}

		// ATDD: Re-verify acceptance tests pass after refactoring
		if atddActive && r.methodologyExec != nil {
			if err := r.methodologyExec.VerifyAcceptanceTestsPass(ctx, bc); err != nil {
				bc.Result.Error = fmt.Errorf("acceptance verification failed after refactoring: %w", err)
				return bc.Result
			}
		}
	}

	bc.Result.Success = true
	return bc.Result
}

// makeInvokeFn creates an InvokeFn callback that wraps the Runner's Claude invocation,
// handling cost data, scope-too-large, usage limit detection, and timeout classification.
func (r *Runner) makeInvokeFn() escalation.InvokeFn {
	return func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*escalation.InvocationResult, error) {
		// Re-render build prompt on retry so failure context is included
		if bc.PromptCtx != nil && bc.PromptCtx.IsRetry && r.renderer != nil {
			rendered, renderErr := r.renderer.RenderBuild(bc.PromptCtx)
			if renderErr == nil {
				bc.BuildPrompt = rendered
			}
		}

		r.log("Running Claude with model: %s", bc.Model)

		claudeResult, stats, stallFired, err := r.executeClaudeInvocation(ctx, bc)

		if err != nil {
			if stallFired && ctx.Err() == nil {
				return &escalation.InvocationResult{StallFired: true}, err
			}
			// Classify timeout type
			if ctx.Err() != nil && bc.ParentCtx.Err() == nil {
				bc.Result.TimeoutType = "bead"
				escalation.ExtractTimeoutLearning(bc, r.renderer.GetLearningsFile())
				return nil, fmt.Errorf("bead timeout: exceeded %v total processing time", bc.BeadTimeout)
			} else if bc.ParentCtx.Err() != nil {
				return nil, fmt.Errorf("context cancelled: %w", bc.ParentCtx.Err())
			}
			bc.Result.TimeoutType = "invocation"
			return nil, fmt.Errorf("claude invocation: %w", err)
		}

		if claudeResult == nil {
			return nil, fmt.Errorf("claude returned nil result")
		}

		// Populate cost/token data
		if stats != nil {
			costUSD, inputTokens, outputTokens := stats.CostData()
			bc.Result.CostUSD = costUSD
			bc.Result.InputTokens = inputTokens
			bc.Result.OutputTokens = outputTokens
		}

		// Check scope-too-large
		if isTooLarge, explanation := claude.IsScopeTooLarge(claudeResult); isTooLarge {
			r.handleScopeTooLarge(bc, claudeResult, explanation)
			return &escalation.InvocationResult{Result: claudeResult}, bc.Result.Error
		}

		// Check usage limits
		signals := usagelimit.Signals{
			ExitCode: claudeResult.ExitCode,
			Output:   claudeResult.Output,
		}
		if stats != nil {
			signals.RateLimitHits = stats.RateLimitHits
		}
		if usagelimit.Check(signals, usagelimit.ClaudePatterns()) {
			bc.Result.UsageLimited = true
			r.log("Warning: usage limit detected - stopping retry attempts")
			return nil, fmt.Errorf("usage limit detected: retries or escalation will not resolve this failure (exit code: %d, rate limit events: %d)", claudeResult.ExitCode, signals.RateLimitHits)
		}

		return &escalation.InvocationResult{
			Result:     claudeResult,
			StallFired: false,
		}, nil
	}
}

// makeValidationExecuteFn creates a validation.ExecuteFn callback that wraps
// the escalation handler's ExecuteWithRetry for Claude-based validation fix attempts.
func (r *Runner) makeValidationExecuteFn() validation.ExecuteFn {
	return func(ctx context.Context, bc *runtypes.BeadContext) bool {
		// Render a build prompt for the validation fix
		bc.PromptCtx.IsRetry = true
		bc.PromptCtx.PrevFailure = bc.Result.Output
		bc.PromptCtx.FailureContext = "Validation (tests/lint) failed after your build succeeded. Fix the validation errors."

		var renderErr error
		bc.BuildPrompt, renderErr = r.renderer.RenderBuild(bc.PromptCtx)
		if renderErr != nil {
			r.log("Warning: rendering validation fix prompt: %v", renderErr)
			return false
		}

		// Save retry state and enforce single attempt
		savedMaxRetries := bc.MaxRetries
		savedRetriesThisModel := bc.RetriesThisModel
		savedTotalRetries := bc.TotalRetriesThisBead
		bc.MaxRetries = 0
		bc.RetriesThisModel = 0

		success := r.escalationHandler.ExecuteWithRetry(ctx, bc, r.makeInvokeFn())

		// Restore retry state
		bc.MaxRetries = savedMaxRetries
		bc.RetriesThisModel = savedRetriesThisModel
		bc.TotalRetriesThisBead = savedTotalRetries

		return success
	}
}

// makeReviewValidateFn creates a ValidateFn that wraps runDirectValidationCheck
// for use by the reviewpkg.Reviewer's re-validation after review fixes.
func (r *Runner) makeReviewValidateFn() reviewpkg.ValidateFn {
	return func(ctx context.Context, commands []string, workDir string) (bool, error) {
		result, err := r.runDirectValidationCheck(ctx, commands, workDir)
		if err != nil {
			return false, err
		}
		return result != nil && claude.IsValidationPassed(result), nil
	}
}

// makeMethodologyExec creates a methodology.Executor wired with callbacks
// that route through the Runner's provider, escalation, and validation infrastructure.
func (r *Runner) makeMethodologyExec() *methodology.Executor {
	// RenderFn wraps the renderer's acceptance tests prompt rendering
	renderFn := func(ctx *prompt.Context) (string, error) {
		if r.renderer == nil {
			return "", fmt.Errorf("renderer not configured")
		}
		return r.renderer.RenderAcceptanceTests(ctx)
	}

	// InvokeFn wraps the provider chain for acceptance test invocations
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, promptText string) error {
		if r.router == nil {
			return fmt.Errorf("router not configured")
		}
		p, modelName := r.router.Select("build", bc.Tier)
		if p == nil {
			return fmt.Errorf("no providers available for phase=build tier=%s", bc.Tier)
		}
		bc.Model = modelName
		if bc.Result.Escalated && bc.Result.EscalatedTo != "" {
			bc.Result.EscalatedTo = modelName
		}
		result, err := p.Run(ctx, promptText, bc.Tier)
		if err != nil {
			if p.IsUsageLimitError(result, err) {
				r.router.MarkUnavailable(p.Name())
				p2, modelName2 := r.router.Select("build", bc.Tier)
				if p2 != nil {
					bc.Model = modelName2
					result, err = p2.Run(ctx, promptText, bc.Tier)
				}
			}
			if err != nil {
				return err
			}
		}
		if result == nil || !result.Success {
			return fmt.Errorf("acceptance tests failed")
		}
		return nil
	}

	// ValidateDirectFn wraps the validation runner's direct validation
	validateFn := func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
		return r.runDirectValidationCheck(ctx, commands, workDir)
	}

	// EscalateTierFn wraps escalation.Handler.EscalateTier
	escalateTierFn := func(bc *runtypes.BeadContext, nextTier string) {
		if r.escalationHandler != nil {
			r.escalationHandler.EscalateTier(bc, nextTier)
		}
	}

	// AnalyzeFn wraps the failure analyzer, extracting the suggestion string
	var analyzeFn methodology.AnalyzeFn
	if r.analyzer != nil {
		analyzeFn = func(ctx context.Context, b *bead.Bead, failureOutput string) (string, error) {
			analysis, err := r.analyzer.Analyze(ctx, b, failureOutput)
			if err != nil {
				return "", err
			}
			if analysis == nil {
				return "", fmt.Errorf("analysis returned nil")
			}
			return analysis.Suggestion, nil
		}
	}

	// GetDiffFn wraps the runner's git diff
	getDiffFn := func(startCommit string) (string, error) {
		return r.getDiff(startCommit)
	}

	// Create the base executor with ATDD callbacks
	methExec := methodology.NewExecutorWithEscalation(r.cfg, r.output, renderFn, invokeFn, validateFn, escalateTierFn)

	// Wire analysis support for VerifyTestsFailWithRetry
	methExec.SetAnalyzeFn(analyzeFn)
	methExec.SetGetDiffFn(getDiffFn)

	// Wire refactor deps
	methExec.SetRefactorDeps(methodology.NewRefactorDeps(
		getDiffFn,
		func(ctx *prompt.Context) (string, error) {
			if r.renderer == nil {
				return "", fmt.Errorf("renderer not configured")
			}
			return r.renderer.RenderRefactor(ctx)
		},
		r.runRefactorWithRouter,
		validateFn,
		func(commit string) error {
			resetCmd := exec.Command("git", "reset", "--hard", commit)
			return resetCmd.Run()
		},
		getGitHead,
	))

	return methExec
}

func (r *Runner) logResult(result *IterationResult) {
	if r == nil || result == nil {
		return
	}
	if result.AlreadyDone {
		r.log("ALREADY DONE: %s — ATDD tests pass, work previously completed (%v)", result.BeadID, result.Duration)
		return
	}
	if result.Success {
		r.log("SUCCESS: %s completed in %v", result.BeadID, result.Duration)
		if result.Escalated {
			r.log("  (escalated to %s)", result.EscalatedTo)
		}
		if result.Validated {
			r.log("  (validation passed)")
		}
	} else {
		r.log("FAILED: %s - %v", result.BeadID, result.Error)
	}
}

func (r *Runner) log(format string, args ...any) {
	if r.output == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(r.output, msg)
}

// heartbeatInterval is the interval at which Claude's progress is checked and heartbeat status is printed.
const heartbeatInterval = 30 * time.Second

// heartbeatConfig holds timing parameters for the heartbeat goroutine.
type heartbeatConfig struct {
	InitialDelay   time.Duration
	HeartbeatRate  time.Duration
	StallCheckRate time.Duration
}

var defaultHeartbeatConfig = heartbeatConfig{
	InitialDelay:   15 * time.Second,
	HeartbeatRate:  heartbeatInterval,
	StallCheckRate: 10 * time.Second,
}

// startHeartbeat launches a goroutine that prints periodic status updates and listens
// for tool call events to update the display in real-time. It also optionally detects
// stalls using two-tier timeouts:
//   - stallTimeout: used before Claude has made any tool calls (detecting true hangs)
//   - stallTimeoutActive: used after tool activity (longer, allows thinking pauses)
//
// The toolCallEvents channel (optional, can be nil) feeds real-time tool call notifications.
// Returns a function to stop the heartbeat.
func (r *Runner) startHeartbeat(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), toolCallEvents <-chan claude.ToolEvent) func() {
	return r.startHeartbeatWithConfig(stats, stallTimeout, stallTimeoutActive, onStall, defaultHeartbeatConfig, toolCallEvents)
}

func (r *Runner) startHeartbeatWithConfig(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), cfg heartbeatConfig, toolCallEvents <-chan claude.ToolEvent) func() {
	if r == nil || stats == nil {
		return func() {}
	}
	done := make(chan struct{})
	usedOverwrite := make(chan bool)
	lastHeartbeatLine := "" // Track last printed line for overwriting
	go func() {
		// First heartbeat sooner so hangs are visible quickly
		select {
		case <-done:
			usedOverwrite <- false
			return
		case <-time.After(cfg.InitialDelay):
		}
		lastHeartbeatLine = r.printHeartbeat(stats)

		heartbeatTicker := time.NewTicker(cfg.HeartbeatRate)
		defer heartbeatTicker.Stop()

		// Stall check runs at shorter intervals for reasonable precision
		var stallTicker *time.Ticker
		if stallTimeout > 0 && onStall != nil {
			stallTicker = time.NewTicker(cfg.StallCheckRate)
			defer stallTicker.Stop()
		}

		wasOverwritten := false
		for {
			if stallTicker != nil {
				select {
				case <-done:
					usedOverwrite <- wasOverwritten
					return
				case <-heartbeatTicker.C:
					lastHeartbeatLine = r.printHeartbeat(stats)
					wasOverwritten = false
				case <-stallTicker.C:
					// Only check for stalls after the first stream event arrives.
					// Before that, Claude CLI is still starting up — the startup
					// monitor in claude.go handles that phase separately.
					if stats.HasReceivedEvent() {
						// Two-tier timeout: use longer timeout once Claude has
						// started making tool calls (reading files, editing, etc.)
						threshold := stallTimeout
						tier := "initial"
						if stats.HasToolActivity() && stallTimeoutActive > 0 {
							threshold = stallTimeoutActive
							tier = "active"
						}
						if stats.TimeSinceLastEvent() >= threshold {
							stats.RecordStall(tier)
							r.log("STALL DETECTED (%s): No output from Claude for %v (threshold: %v)",
								tier, stats.TimeSinceLastEvent().Round(time.Second), threshold)
							onStall()
							usedOverwrite <- wasOverwritten
							return
						}
					}
				case <-toolCallEvents:
					// On tool call event, update heartbeat line in place
					lastHeartbeatLine = r.overwriteHeartbeat(stats, lastHeartbeatLine)
					wasOverwritten = true
				}
			} else {
				select {
				case <-done:
					usedOverwrite <- wasOverwritten
					return
				case <-heartbeatTicker.C:
					lastHeartbeatLine = r.printHeartbeat(stats)
					wasOverwritten = false
				case <-toolCallEvents:
					// On tool call event, update heartbeat line in place
					lastHeartbeatLine = r.overwriteHeartbeat(stats, lastHeartbeatLine)
					wasOverwritten = true
				}
			}
		}
	}()
	return func() {
		close(done)
		// Wait for the goroutine to signal completion
		// syncWriter handles newline transition automatically
		<-usedOverwrite
	}
}

func (r *Runner) printHeartbeat(stats *logger.StreamStats) string {
	if r == nil || stats == nil {
		return ""
	}
	toolCalls, filesModified, elapsed := stats.Snapshot()
	minutes := int(elapsed.Minutes())
	seconds := int(elapsed.Seconds()) % 60
	var line string
	if toolCalls == 0 {
		line = fmt.Sprintf("[%dm%02ds] Waiting for Claude to respond (may be thinking)...", minutes, seconds)
	} else {
		line = fmt.Sprintf("[%dm%02ds] %d tool calls, %d files modified", minutes, seconds, toolCalls, filesModified)
	}
	r.log("%s", line)
	return line
}

// overwriteHeartbeat updates the heartbeat line in place using carriage return and padding.
// lastLine is the previously printed line (for padding calculation).
// Returns the new line that was printed.
func (r *Runner) overwriteHeartbeat(stats *logger.StreamStats, lastLine string) string {
	if r == nil || r.syncOut == nil || stats == nil {
		return ""
	}
	toolCalls, filesModified, elapsed := stats.Snapshot()
	minutes := int(elapsed.Minutes())
	seconds := int(elapsed.Seconds()) % 60
	var newLine string
	if toolCalls == 0 {
		newLine = fmt.Sprintf("[%dm%02ds] Waiting for Claude to respond (may be thinking)...", minutes, seconds)
	} else {
		newLine = fmt.Sprintf("[%dm%02ds] %d tool calls, %d files modified", minutes, seconds, toolCalls, filesModified)
	}

	// Use carriage return to overwrite the line, pad to clear old content
	padding := ""
	if len(lastLine) > len(newLine) {
		padding = strings.Repeat(" ", len(lastLine)-len(newLine))
	}
	r.syncOut.WriteOverwrite([]byte(fmt.Sprintf("\r%s%s", newLine, padding)))

	return newLine
}

// checkRetroSuggestion checks if a retro should be suggested and prints a message
func (r *Runner) checkRetroSuggestion() {
	if r.cfg == nil {
		return
	}
	// Load learnings
	lf, err := learnings.NewFile(r.gromitDir)
	if err != nil {
		return // Silently skip if learnings can't be created
	}
	if err := lf.Load(); err != nil {
		return // Silently skip if learnings can't be loaded
	}

	// Load interactive state for last retro time
	interactiveFile, err := state.NewInteractiveFile(r.gromitDir)
	if err != nil {
		return // Silently skip if interactive state can't be created
	}
	if err := interactiveFile.Load(); err != nil {
		return // Silently skip if interactive state can't be loaded
	}

	// Compute failure rate from logs
	stats, err := logger.ReadAllLogs(r.cfg.Paths.Logs)
	if err != nil {
		stats = logger.RunStats{} // Use zero stats on error
	}

	should, reason := lf.ShouldSuggestRetro(interactiveFile.LastRetro(), stats.FailureRate())
	if !should {
		return
	}

	confirmedCount, provisionalCount := lf.Stats()
	r.log("\nRetro suggested: %d provisional learnings, %d confirmed patterns (%s). Run: gromit retro",
		provisionalCount, confirmedCount, reason)
}

// isStuckBeadWithStats checks if a bead has failed too many times across runs
// using pre-loaded bead statistics (call ReadPerBeadStats once before the loop for efficiency)
func (r *Runner) isStuckBeadWithStats(b *bead.Bead, beadStats map[string]logger.BeadStats) bool {
	if r == nil || b == nil || r.cfg == nil {
		return false
	}

	// If threshold is 0 or negative, stuck-bead detection is disabled
	if r.cfg.Loop.StuckBeadThreshold <= 0 {
		return false
	}

	stats, exists := beadStats[b.ID]
	if !exists {
		// No history for this bead, not stuck
		return false
	}

	// Mark as stuck if failures >= threshold
	return stats.Failures >= r.cfg.Loop.StuckBeadThreshold
}

// Status returns the current queue status
func (r *Runner) Status() error {
	if r == nil {
		return fmt.Errorf("runner is nil")
	}

	// Read status.json
	status, err := ReadStatus(r.gromitDir)
	if err != nil {
		return fmt.Errorf("reading status: %w", err)
	}

	// Check if status is stale (process not alive)
	if status != nil && status.Running && !IsProcessAlive(status.PID) {
		elapsed := time.Since(status.StartedAt)
		r.log("Warning: stale run detected from %s (%s ago)",
			status.StartedAt.Format(time.RFC3339),
			elapsed.Round(time.Second))
		r.log("  Bead: %s - %s", status.BeadID, status.BeadTitle)
		r.log("  Removing stale status file")

		// Delete the stale file
		sw, err := NewStatusWriter(r.gromitDir)
		if err == nil {
			_ = sw.Delete() // Ignore error - we'll proceed anyway
		}
		r.log("")
		status = nil // Treat as if no status exists
	}

	// Read pipeline status
	// Pass startedAt from status.json if available for "closed this run" count
	var startedAt *time.Time
	if status != nil && !status.StartedAt.IsZero() {
		startedAt = &status.StartedAt
	}
	pipelineStatus, err := pipeline.ReadStatus(r.gromitDir, r.cfg.Paths.Specs, r.cfg.Paths.Plans, startedAt)
	if err != nil {
		return fmt.Errorf("reading pipeline status: %w", err)
	}

	// Load state file for health data
	stateFile, err := state.NewFile(r.gromitDir)
	if err != nil {
		return fmt.Errorf("creating state file: %w", err)
	}
	if err := stateFile.Load(); err != nil {
		return fmt.Errorf("loading state file: %w", err)
	}

	// Load interactive state file for last retro data
	interactiveFile, err := state.NewInteractiveFile(r.gromitDir)
	if err != nil {
		return fmt.Errorf("creating interactive state file: %w", err)
	}
	if err := interactiveFile.Load(); err != nil {
		return fmt.Errorf("loading interactive state file: %w", err)
	}

	// Read model performance stats
	modelStats, err := logger.ReadModelStats(r.cfg.Paths.Logs)
	if err != nil {
		// Log warning but continue - model stats are informational only
		r.log("Warning: could not read model stats: %v", err)
		modelStats = make(map[string]logger.ModelStats)
	}

	// Format and print all sections
	r.log("%s", formatPipeline(pipelineStatus))
	r.log("")
	r.log("%s", formatRun(status))
	r.log("")
	r.log("%s", formatHealth(interactiveFile.LastRetro(), stateFile.IterationsSinceReview()))
	r.log("")
	r.log("%s", formatModelPerformance(modelStats))
	r.log("")
	r.log("%s", formatRecommendation(pipelineStatus.Recommendation))

	return nil
}

// getGitHead returns the current git HEAD commit
func getGitHead() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// getGitDiffStat returns the git diff --stat output from a given commit to the
// current working tree state (including both committed and uncommitted changes).
func getGitDiffStat(fromCommit string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", fromCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat: %w", err)
	}
	return string(out), nil
}

// getGitDiff returns the full diff from fromCommit to the current working tree
func getGitDiff(fromCommit string) (string, error) {
	cmd := exec.Command("git", "diff", fromCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// getDiff calls the injectable gitDiffFn, falling back to getGitDiff if unset.
func (r *Runner) getDiff(fromCommit string) (string, error) {
	if r.gitDiffFn != nil {
		return r.gitDiffFn(fromCommit)
	}
	return getGitDiff(fromCommit)
}

// hasNewPackages returns true if any package in the list is not in the runner's
// touched packages map. Returns true if the map is nil or empty.
func (r *Runner) hasNewPackages(packages []string) bool {
	if r.touchedPackages == nil || len(r.touchedPackages) == 0 {
		return len(packages) > 0
	}
	for _, pkg := range packages {
		if !r.touchedPackages[pkg] {
			return true
		}
	}
	return false
}

// updateTouchedPackages adds the given packages to the runner's touched packages map.
func (r *Runner) updateTouchedPackages(packages []string) {
	if r.touchedPackages == nil {
		r.touchedPackages = make(map[string]bool)
	}
	for _, pkg := range packages {
		r.touchedPackages[pkg] = true
	}
}

// defaultCmdRunner executes a shell command and returns stdout, stderr, exit code.
// Non-zero exit codes are returned as exitCode (not as an error).
func defaultCmdRunner(ctx context.Context, command string, workDir string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.String(), stderr.String(), exitErr.ExitCode(), nil
		}
		return stdout.String(), stderr.String(), -1, err
	}
	return stdout.String(), stderr.String(), 0, nil
}

// runCmd calls the injectable cmdRunnerFn, falling back to defaultCmdRunner if unset.
func (r *Runner) runCmd(ctx context.Context, command string, workDir string) (string, string, int, error) {
	if r.cmdRunnerFn != nil {
		return r.cmdRunnerFn(ctx, command, workDir)
	}
	return defaultCmdRunner(ctx, command, workDir)
}

// checkExpectedOutputs checks if expected files exist and returns a summary
func checkExpectedOutputs(expectedOutputs []string) string {
	if len(expectedOutputs) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "\nExpected outputs:")

	for _, path := range expectedOutputs {
		_, err := os.Stat(path)
		if err == nil {
			lines = append(lines, fmt.Sprintf("  ✓ %s (exists)", path))
		} else if os.IsNotExist(err) {
			lines = append(lines, fmt.Sprintf("  ✗ %s (not found)", path))
		} else {
			lines = append(lines, fmt.Sprintf("  ? %s (error: %v)", path, err))
		}
	}

	return strings.Join(lines, "\n")
}

// showPartialProgress displays git diff and expected outputs on failure
func (r *Runner) showPartialProgress(b *bead.Bead, startCommit string) {
	if r == nil || b == nil {
		return
	}
	// Always show git diff summary
	diffStat, err := getGitDiffStat(startCommit)
	if err != nil {
		r.log("Warning: could not get git diff: %v", err)
	} else if strings.TrimSpace(diffStat) != "" {
		r.log("\nChanges detected:")
		r.log("%s", diffStat)
	} else {
		r.log("\nNo git changes detected - Claude may not have completed any work.")
	}

	// Show expected outputs if specified
	if len(b.ExpectedOutputs) > 0 {
		summary := checkExpectedOutputs(b.ExpectedOutputs)
		r.log("%s", summary)
	}
}

// DecomposeTask calls Claude (opus) to decompose a task and returns parsed sub-tasks
// Does NOT create beads - just gets the decomposition
func (r *Runner) DecomposeTask(ctx context.Context, b *bead.Bead) ([]SubTask, error) {
	if r == nil {
		return nil, fmt.Errorf("runner is nil")
	}
	if b == nil {
		return nil, fmt.Errorf("bead is nil")
	}
	if r.beads == nil {
		return nil, fmt.Errorf("runner beads client is nil")
	}
	if r.renderer == nil {
		return nil, fmt.Errorf("runner renderer is nil")
	}
	if r.router == nil {
		return nil, fmt.Errorf("runner router is nil")
	}

	// Get parent bead if exists
	parent, err := r.beads.GetParent(b)
	if err != nil {
		// Log warning but continue - decomposition can work without parent
		r.log("Warning: failed to get parent bead: %v", err)
	}

	// Build decompose context
	atddActive := bead.IsMethodologyActive(b.Labels, "atdd", r.cfg.Methodology.ATDD)
	decomposeCtx := &prompt.DecomposeContext{
		Bead:       b,
		ParentBead: parent,
		ATDDActive: atddActive,
	}

	// Render decompose prompt
	decomposedPrompt, err := r.renderer.RenderDecompose(decomposeCtx)
	if err != nil {
		return nil, fmt.Errorf("rendering decompose prompt: %w", err)
	}

	// Select provider via router (phase="decompose", tier="high" for opus-level complexity)
	p, _ := r.router.Select("decompose", provider.TierHigh)
	if p == nil {
		return nil, fmt.Errorf("no provider available for decomposition")
	}

	// Invoke provider with high tier
	result, err := p.Run(ctx, decomposedPrompt, provider.TierHigh)
	if err != nil {
		return nil, fmt.Errorf("decomposition invocation: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("decomposition returned nil result")
	}

	if !result.Success {
		return nil, fmt.Errorf("decomposition failed with exit code %d", result.ExitCode)
	}

	// Parse the output
	subTasks, err := parseDecomposeOutput(result.Output)
	if err != nil {
		return nil, fmt.Errorf("parsing decomposition: %w", err)
	}

	return subTasks, nil
}

// parseDecomposeOutput parses Claude's JSON array decompose output into []SubTask
// It's resilient to non-pure JSON output (e.g., explanatory text before/after the JSON)
func parseDecomposeOutput(output string) ([]SubTask, error) {
	if output == "" {
		return nil, fmt.Errorf("decompose output is empty")
	}

	var subTasks []SubTask
	if err := jsonutil.ExtractArray(output, &subTasks); err != nil {
		return nil, fmt.Errorf("parsing decompose output: %w", err)
	}

	if len(subTasks) == 0 {
		return nil, fmt.Errorf("decompose output contains no sub-tasks")
	}

	for i := range subTasks {
		subTasks[i].NormalizeNilFields()
	}

	return subTasks, nil
}

// CreateSubBeads creates child beads from decomposed sub-tasks, comments on the original
// bead with the new sub-bead IDs, and closes the original bead.
func (r *Runner) CreateSubBeads(ctx context.Context, b *bead.Bead, subTasks []SubTask) error {
	if r == nil {
		return fmt.Errorf("runner is nil")
	}
	if b == nil {
		return fmt.Errorf("bead is nil")
	}
	if len(subTasks) == 0 {
		return fmt.Errorf("no sub-tasks to create")
	}
	if r.beads == nil {
		return fmt.Errorf("runner beads client is nil")
	}

	// Create beads for each sub-task
	var createdIDs []string
	for i, subTask := range subTasks {
		r.log("Creating sub-bead %d/%d: %s", i+1, len(subTasks), subTask.Title)

		// Build description from description and acceptance criteria
		var description string
		if subTask.Description != "" {
			description = subTask.Description
			if len(subTask.AcceptanceCriteria) > 0 {
				description += "\n\nAcceptance criteria:\n"
				for _, ac := range subTask.AcceptanceCriteria {
					description += "- " + ac + "\n"
				}
			}
		}

		// Inherit labels from parent and inject methodology labels if needed
		labels := r.injectMethodologyLabels(b.Labels)

		createdBead, err := r.beads.CreateWithParentAndDescription(
			subTask.Title,
			b.Priority, // Inherit priority from parent
			labels,     // Inherit labels from parent with methodology injection
			nil,        // No expected outputs
			b.ID,       // Set parent to original bead
			description,
		)
		if err != nil {
			r.log("Warning: failed to create sub-bead: %v", err)
			continue
		}

		createdIDs = append(createdIDs, createdBead.ID)
		r.log("Created sub-bead: %s", createdBead.ID)

		// Log warning about DependsOn not yet supported
		if subTask.DependsOn != nil {
			r.log("Warning: DependsOn field not yet supported for sub-task %d (index %d)", *subTask.DependsOn, i)
		}
	}

	if len(createdIDs) == 0 {
		return fmt.Errorf("failed to create any sub-beads")
	}

	// Comment on original bead listing the new sub-bead IDs
	comment := fmt.Sprintf("Decomposed into %d sub-beads:\n", len(createdIDs))
	for i, id := range createdIDs {
		comment += fmt.Sprintf("%d. %s\n", i+1, id)
	}
	if err := r.beads.AddComment(b.ID, comment); err != nil {
		r.log("Warning: failed to add comment to bead: %v", err)
	}

	// Close the original bead
	if err := r.beads.Close(b.ID); err != nil {
		r.log("Warning: failed to close bead: %v", err)
	}

	// Sync bd state
	if err := r.beads.Sync(); err != nil {
		r.log("Warning: failed to sync beads: %v", err)
	}

	r.log("Successfully created %d sub-beads", len(createdIDs))
	return nil
}

// injectMethodologyLabels takes parent labels and adds methodology labels when
// the methodology is globally active but not already present in the parent's labels.
// This ensures sub-beads inherit methodology even when set globally rather than via labels.
func (r *Runner) injectMethodologyLabels(parentLabels []string) []string {
	if r == nil || r.cfg == nil {
		return parentLabels
	}

	// Start with a copy of parent labels
	labels := make([]string, len(parentLabels))
	copy(labels, parentLabels)

	// Check if ATDD label should be added
	if r.cfg.Methodology.ATDD {
		hasATDDLabel := false
		for _, label := range labels {
			if label == "atdd:true" || label == "atdd:false" {
				hasATDDLabel = true
				break
			}
		}
		if !hasATDDLabel {
			labels = append(labels, "atdd:true")
		}
	}

	// Check if TDD label should be added
	if r.cfg.Methodology.TDD {
		hasTDDLabel := false
		for _, label := range labels {
			if label == "tdd:true" || label == "tdd:false" {
				hasTDDLabel = true
				break
			}
		}
		if !hasTDDLabel {
			labels = append(labels, "tdd:true")
		}
	}

	return labels
}

// runPrecheck calls configured model with precheck prompt to check if acceptance criteria are already met.
// Returns true if precheck passed (criteria already satisfied), and the duration it took.
// Non-blocking: logs warnings on errors and returns false.
func (r *Runner) runPrecheck(ctx context.Context, b *bead.Bead) (bool, time.Duration) {
	start := time.Now()

	if r == nil || b == nil || r.cfg == nil || r.renderer == nil || r.router == nil {
		return false, 0
	}

	// Check if precheck is enabled
	if !r.cfg.Precheck.IsEnabled() {
		return false, 0
	}

	// Get parent bead if exists
	parent, err := r.beads.GetParent(b)
	if err != nil {
		r.log("Warning: failed to get parent bead for precheck: %v", err)
	}

	// Build precheck context
	precheckCtx := &prompt.PrecheckContext{
		Bead:       b,
		ParentBead: parent,
	}

	// Render precheck prompt
	precheckPrompt, err := r.renderer.RenderPrecheck(precheckCtx)
	if err != nil {
		r.log("Warning: failed to render precheck prompt: %v", err)
		return false, time.Since(start)
	}

	// Call router to select provider and model
	precheckTimeout := time.Duration(r.cfg.Precheck.TimeoutSeconds) * time.Second
	precheckCtx2, cancel := context.WithTimeout(ctx, precheckTimeout)
	defer cancel()

	// Select provider via router (phase="precheck", tier="low")
	p, _ := r.router.Select("precheck", provider.TierLow)
	if p == nil {
		r.log("Warning: no provider available for precheck")
		return false, time.Since(start)
	}

	// Invoke provider with low tier
	result, err := p.Run(precheckCtx2, precheckPrompt, provider.TierLow)
	if err != nil {
		r.log("Warning: precheck invocation failed: %v", err)
		return false, time.Since(start)
	}
	if result == nil {
		r.log("Warning: precheck returned nil result")
		return false, time.Since(start)
	}

	if !result.Success {
		r.log("Warning: precheck failed with exit code %d", result.ExitCode)
		return false, time.Since(start)
	}

	// Check for PRECHECK_PASSED signal
	passed := strings.Contains(result.Output, "PRECHECK_PASSED")

	if passed {
		r.log("Pre-check: acceptance criteria already met")
	} else {
		r.log("Pre-check: acceptance criteria not yet met")
	}

	return passed, time.Since(start)
}

// checkScope calls haiku with scope prompt and returns ScopeEstimate.
// If scope check fails, logs a warning and continues (non-blocking).
func (r *Runner) checkScope(ctx context.Context, b *bead.Bead) *prompt.ScopeEstimate {
	if r == nil || b == nil || r.cfg == nil || r.renderer == nil || r.router == nil {
		return nil
	}

	// Get parent bead if exists
	parent, err := r.beads.GetParent(b)
	if err != nil {
		r.log("Warning: failed to get parent bead for scope check: %v", err)
	}

	// Build scope context
	scopeCtx := &prompt.ScopeContext{
		Bead:       b,
		ParentBead: parent,
	}

	// Render scope prompt
	scopePrompt, err := r.renderer.RenderScope(scopeCtx)
	if err != nil {
		r.log("Warning: failed to render scope prompt: %v", err)
		return nil
	}

	// Select provider via router (phase="scope_check", tier="low")
	p, _ := r.router.Select("scope_check", provider.TierLow)
	if p == nil {
		r.log("Warning: no provider available for scope check")
		return nil
	}

	// Invoke provider with low tier
	result, err := p.Run(ctx, scopePrompt, provider.TierLow)
	if err != nil {
		r.log("Warning: scope check invocation failed: %v", err)
		return nil
	}
	if result == nil {
		r.log("Warning: scope check returned nil result")
		return nil
	}

	if !result.Success {
		r.log("Warning: scope check failed with exit code %d", result.ExitCode)
		return nil
	}

	// Parse the scope estimate
	estimate, err := prompt.ParseScopeEstimate(result.Output)
	if err != nil {
		r.log("Warning: failed to parse scope estimate: %v", err)
		return nil
	}

	return estimate
}

// runGitAutoPush pushes the current branch to its upstream tracking ref after bead completion.
// If auto_push is disabled, does nothing. If push fails and push_failure is "warn", logs a warning
// and continues. If push fails and push_failure is "stop", returns an error to halt the loop.
func (r *Runner) runGitAutoPush() error {
	if r == nil || r.cfg == nil {
		return nil
	}
	if !r.cfg.Git.IsAutoPushEnabled() {
		return nil
	}

	r.log("Pushing to remote...")
	_, stderr, exitCode, err := r.runCmd(context.Background(), "git push", "")
	if err != nil {
		if r.cfg.Git.PushFailure == "stop" {
			return fmt.Errorf("git push failed: %w", err)
		}
		r.log("Warning: git push failed: %v", err)
		return nil
	}
	if exitCode != 0 {
		if r.cfg.Git.PushFailure == "stop" {
			return fmt.Errorf("git push failed (exit %d): %s", exitCode, stderr)
		}
		r.log("Warning: git push failed (exit %d): %s", exitCode, stderr)
		return nil
	}

	return nil
}

func (r *Runner) mergeInteractiveBranches() error {
	if r == nil || r.cfg == nil {
		return nil
	}
	if !r.cfg.Worktree.IsEnabled() || !r.cfg.Worktree.IsAutoMergeEnabled() {
		return nil
	}
	if r.worktreeManager == nil {
		return nil
	}

	branches, err := r.worktreeManager.PendingBranches()
	if err != nil {
		return r.handleMergeFailure(fmt.Errorf("list pending branches: %w", err))
	}

	for _, branch := range branches {
		if err := r.worktreeManager.MergeBack(branch); err != nil {
			if err := r.handleMergeFailure(fmt.Errorf("merge back %s: %w", branch, err)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Runner) handleMergeFailure(err error) error {
	if r == nil || r.cfg == nil {
		return err
	}
	if strings.EqualFold(r.cfg.Worktree.MergeFailure, "stop") {
		return err
	}
	r.log("Warning: merge back failed: %v", err)
	return nil
}

// runBetweenIterationsCommand runs the user-configured command between iterations.
// If the command is empty, does nothing. If the command fails, logs a warning but does not
// stop the loop or return an error.
func (r *Runner) runBetweenIterationsCommand() {
	if r == nil || r.cfg == nil {
		return
	}
	command := r.cfg.Loop.BetweenIterationsCommand
	if command == "" {
		return
	}

	r.log("Running between-iterations command: %s", command)
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = r.output
	cmd.Stderr = r.output

	if err := cmd.Run(); err != nil {
		r.log("Warning: between-iterations command failed: %v", err)
	}
}

// SetLabelFilters sets optional spec labels to filter beads by
func (r *Runner) SetLabelFilters(labels []string) {
	r.labelFilters = labels
}

// getNextBead gets the next bead to process, optionally filtering by labels
func (r *Runner) getNextBead() (*bead.Bead, error) {
	if r == nil || r.beads == nil {
		return nil, fmt.Errorf("runner or beads client is nil")
	}

	// If no label filters, use Ready() as before
	if len(r.labelFilters) == 0 {
		return r.beads.Ready()
	}

	// Collect beads from all labels
	var candidates []*bead.Bead
	for _, label := range r.labelFilters {
		b, err := r.beads.ReadyWithLabel(label)
		if err != nil {
			return nil, fmt.Errorf("getting bead with label %s: %w", label, err)
		}
		if b != nil {
			candidates = append(candidates, b)
		}
	}

	// If no beads found for any label
	if len(candidates) == 0 {
		return nil, nil
	}

	// Return the highest priority bead (lowest priority number)
	highestPriority := candidates[0]
	for _, b := range candidates[1:] {
		if b.Priority < highestPriority.Priority {
			highestPriority = b
		}
	}

	return highestPriority, nil
}

// updateGlobalStats reads current run's model stats and merges them into ~/.gromit/stats.json
func (r *Runner) updateGlobalStats() {
	// Get current run ID from logger
	runID := r.logger.RunID()
	if runID == "" {
		r.log("Warning: could not determine run ID for global stats update")
		return
	}

	// Read model stats for current run
	runStats, err := logger.ReadRunModelStats(r.cfg.Paths.Logs, runID)
	if err != nil {
		r.log("Warning: could not read run model stats for global stats update: %v", err)
		return
	}

	// Resolve global stats path using user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		r.log("Warning: could not determine user home directory for global stats: %v", err)
		return
	}
	globalStatsPath := filepath.Join(homeDir, ".gromit", "stats.json")

	// Update global stats
	if err := logger.UpdateGlobalStats(globalStatsPath, runStats); err != nil {
		r.log("Warning: could not update global stats: %v", err)
		return
	}
}
