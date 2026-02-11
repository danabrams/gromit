package runner

import (
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
	"github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/tmux"
)

var errValidationFailed = errors.New("validation failed")

// Runner orchestrates the Gromit loop
type Runner struct {
	cfg          *config.Config
	beads        BeadClient
	claude       ClaudeClient
	router       *provider.Router
	analyzer     FailureAnalyzer
	renderer     PromptRenderer
	logger       IterationLogger
	streamLogger *logger.StreamLogger
	output       io.Writer
	syncOut      *syncWriter // concrete type for WriteOverwrite access
	gromitDir    string
	gitDiffFn    func(string) (string, error) // injectable for testing; defaults to getGitDiff
	labelFilters []string                     // optional spec labels to filter beads
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
		fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
	}

	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return nil, err
	}

	// Determine gromit directory (parent of templates dir)
	gromitDir := filepath.Dir(cfg.Paths.Templates)

	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		cfg.Paths.Specs,
		cfg.Paths.ProjectClaudeMD,
		gromitDir,
	)
	if err != nil {
		return nil, err
	}

	// Wire filter into learnings file
	lf := renderer.GetLearningsFile()
	if lf != nil {
		// Create adapter for claude.Client to match learnings.ClaudeRunner interface
		claudeRunnerAdapter := learnings.NewClaudeRunnerAdapter(claudeClient)
		lf.SetFilter(learnings.NewLLMFilter(claudeRunnerAdapter, "gromit", learnings.ProjectDescriptions.Gromit))
	}

	beadsClient, err := bead.NewClient()
	if err != nil {
		return nil, err
	}

	analyzerObj, err := analyzer.NewAnalyzer(claudeClient, cfg.Models.Validation, renderer)
	if err != nil {
		return nil, err
	}

	// Wrap output in synchronized writer for thread-safe writes
	syncOut := newSyncWriter(output)

	return &Runner{
		cfg:       cfg,
		beads:     beadsClient,
		claude:    claudeClient,
		analyzer:  analyzerObj,
		renderer:  renderer,
		logger:    log,
		output:    syncOut,
		syncOut:   syncOut,
		gromitDir: gromitDir,
		gitDiffFn: getGitDiff,
	}, nil
}

// Deps holds injectable dependencies for a Runner, used for testing.
type Deps struct {
	Beads    BeadClient
	Claude   ClaudeClient
	Router   *provider.Router
	Analyzer FailureAnalyzer
	Renderer PromptRenderer
	Logger   IterationLogger
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

	// Use provided Router, or wrap Claude client for backward compatibility
	router := deps.Router
	if router == nil && deps.Claude != nil {
		// Create a ClaudeProvider wrapping the Claude client
		tierToModel := map[string]string{
			"high":   "opus",
			"medium": "sonnet",
			"low":    "haiku",
		}
		claudeProvider := provider.NewClaudeProvider(deps.Claude, tierToModel)
		router = provider.NewSingleProviderRouter(claudeProvider)
	}

	return &Runner{
		cfg:       cfg,
		beads:     deps.Beads,
		claude:    deps.Claude,
		router:    router,
		analyzer:  deps.Analyzer,
		renderer:  deps.Renderer,
		logger:    iterLogger,
		output:    syncOut,
		syncOut:   syncOut,
		gromitDir: gromitDir,
		gitDiffFn: getGitDiff,
	}, nil
}

// IterationResult captures the outcome of one loop iteration
type IterationResult struct {
	BeadID                string
	BeadTitle             string
	Model                 string
	Success               bool
	Validated             bool
	Duration              time.Duration
	Error                 error
	Escalated             bool
	EscalatedTo           string
	Decomposed            bool
	Output                string
	CostUSD               float64
	InputTokens           int
	OutputTokens          int
	ReviewBrokeValidation bool // true when review fixes broke previously-passing validation
	AlreadyDone           bool // true when ATDD detected work was already complete
	ValidationRetried     bool // true when validation recovery was attempted

	// Diagnostic fields for timeout investigation
	TimeoutType         string // "stall", "bead", "invocation", ""
	TimeToFirstEventMs  int64
	ToolCallCount       int
	StallCount          int
	StallTier           string // "initial" or "active"
	RateLimitHits       int
	RateLimitRecoveryMs int64 // ms to recover from most recent rate limit
}

// SubTask represents a single sub-task from task decomposition
type SubTask struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	DependsOn          *int     `json:"depends_on"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
func (s *SubTask) normalizeNilFields() {
	if s == nil {
		return
	}
	if s.AcceptanceCriteria == nil {
		s.AcceptanceCriteria = []string{}
	}
}

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
	if r.claude == nil {
		return fmt.Errorf("runner claude client is nil")
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

	// Load state for review tracking
	sf, err := state.NewFile(r.gromitDir)
	if err != nil {
		r.log("Warning: could not create state file: %v", err)
		sf = nil
	} else if err := sf.Load(); err != nil {
		r.log("Warning: could not load state: %v", err)
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
	if sf != nil && sf.LastReviewCommit() == "" {
		currentCommit, err := getGitHead()
		if err == nil && currentCommit != "" {
			if err := sf.RecordReview(currentCommit, 0); err != nil {
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
		model := r.selectModel(b)
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

		// Run between-iterations command if configured
		r.runBetweenIterationsCommand()

		// Check epic completion trigger
		if b.Parent != "" && r.cfg.Review.Thorough.Enabled && r.cfg.Review.Thorough.ShouldRunOnEpicComplete() {
			hasChildren, err := r.beads.HasOpenChildren(b.Parent)
			if err != nil {
				r.log("Warning: could not check epic children: %v", err)
			} else if !hasChildren {
				r.log("\n=== Thorough Review (epic %s complete) ===", b.Parent)
				if sf != nil {
					r.runThoroughReview(ctx, sf, iteration, deadline)
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
				r.runThoroughReview(ctx, sf, iteration, deadline)
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
	defer func() { bc.result.Duration = time.Since(start) }()
	ctx = beadCtx

	// Build prompt (with optional scope check)
	if err := r.buildPromptForBead(ctx, bc, iteration); err != nil {
		bc.result.Error = err
		return bc.result
	}

	// Check if ATDD is active for this bead
	atddActive := bead.IsMethodologyActive(bc.bead.Labels, "atdd", r.cfg.Methodology.ATDD)

	// Skip ATDD for test-only beads — their deliverable IS tests
	if atddActive && bead.IsTestOnlyBead(bc.bead.Title) {
		r.log("Skipping ATDD: bead is test-only")
		atddActive = false
	}

	// ATDD Phase 1: Write acceptance tests (if ATDD active)
	if atddActive {
		r.log("ATDD enabled, writing acceptance tests first...")
		if err := r.runAcceptanceTestsWithRetry(ctx, bc); err != nil {
			bc.result.Error = fmt.Errorf("acceptance tests phase failed: %w", err)
			return bc.result
		}

		// ATDD Phase 2: Verify tests fail (as expected before implementation)
		if err := r.verifyTestsFailWithRetry(ctx, bc); err != nil {
			if errors.Is(err, errATDDAlreadyDone) {
				bc.result.Success = true
				bc.result.AlreadyDone = true
				return bc.result
			}
			bc.result.Error = err
			return bc.result
		}

		// Update build prompt to indicate acceptance tests are ready
		bc.promptCtx.IsRetry = false // Clear any retry flags
		bc.promptCtx.PrevFailure = ""
		bc.promptCtx.FailureContext = "Acceptance tests have been written and committed. Your job is to make them pass."
		var err error
		bc.buildPrompt, err = r.renderer.RenderATDDBuild(bc.promptCtx)
		if err != nil {
			bc.result.Error = fmt.Errorf("rendering ATDD build prompt: %w", err)
			return bc.result
		}
	}

	// Check if TDD is active for this bead (after ATDD check so TDD overrides when both are active)
	tddActive := bead.IsMethodologyActive(bc.bead.Labels, "tdd", r.cfg.Methodology.TDD)
	if tddActive {
		r.log("TDD enabled, using TDD build prompt with red-green-refactor cycles...")
		var err error
		bc.buildPrompt, err = r.renderer.RenderTDDBuild(bc.promptCtx)
		if err != nil {
			bc.result.Error = fmt.Errorf("rendering TDD build prompt: %w", err)
			return bc.result
		}
	}

	// Main execution loop with retry and escalation
	if !r.executeWithRetry(ctx, bc) {
		return bc.result
	}

	// Run validation if enabled (with recovery on failure)
	if err := r.runValidationWithRecovery(ctx, bc); err != nil {
		bc.result.Error = err
		return bc.result
	}

	// ATDD/TDD Phase 3: Refactor (if either methodology is active)
	if atddActive || tddActive {
		r.log("Running refactor phase...")
		if err := r.runRefactorPhase(ctx, bc); err != nil {
			// Refactor failures are non-blocking, just log
			r.log("Warning: refactor phase encountered issues: %v", err)
		}

		// Re-validate after refactoring (with recovery on failure)
		if r.cfg.Validation.Enabled {
			if err := r.runValidationWithRecovery(ctx, bc); err != nil {
				bc.result.Error = fmt.Errorf("validation failed after refactoring: %w", err)
				return bc.result
			}
		}
	}

	bc.result.Success = true
	return bc.result
}

// executeWithRetry runs the main Claude execution loop with retry, escalation,
// and decomposition logic. Returns true if the build succeeded, false if
// processBead should return bc.result immediately.
func (r *Runner) executeWithRetry(ctx context.Context, bc *beadContext) bool {
	for {
		if bc.retriesThisModel > 0 || bc.totalRetriesThisBead > 0 {
			r.log("Attempt %d/%d...", bc.totalRetriesThisBead+1, bc.maxRetriesPerBead)
		}

		// Check for context cancellation before each invocation
		select {
		case <-ctx.Done():
			r.log("Context cancelled, stopping")
			bc.result.Error = ctx.Err()
			return false
		default:
		}

		r.log("Running Claude with model: %s", bc.model)

		claudeResult, stats, stallFired, err := r.executeClaudeInvocation(ctx, bc)

		// Handle invocation error (stall, timeout, or other failure)
		if err != nil {
			if stallFired && ctx.Err() == nil {
				bc.result.TimeoutType = "stall"
				if r.handleStallTimeout(ctx, bc) {
					continue
				}
				return false
			}
			// Distinguish bead timeout from user Ctrl+C
			if ctx.Err() != nil && bc.parentCtx.Err() == nil {
				bc.result.TimeoutType = "bead"
				// Extract synthetic learning for timeout failure
				r.extractTimeoutLearning(bc)
				bc.result.Error = fmt.Errorf("bead timeout: exceeded %v total processing time", bc.beadTimeout)
			} else if bc.parentCtx.Err() != nil {
				// User-initiated cancellation (Ctrl+C) - don't extract learning
				bc.result.Error = fmt.Errorf("context cancelled: %w", bc.parentCtx.Err())
			} else {
				bc.result.TimeoutType = "invocation"
				bc.result.Error = fmt.Errorf("claude invocation: %w", err)
			}
			return false
		}

		if claudeResult == nil {
			bc.result.Error = fmt.Errorf("claude returned nil result")
			return false
		}

		bc.result.Output = claudeResult.Output

		// Populate cost/token data from stream stats
		if stats != nil {
			costUSD, inputTokens, outputTokens := stats.CostData()
			bc.result.CostUSD = costUSD
			bc.result.InputTokens = inputTokens
			bc.result.OutputTokens = outputTokens
		}

		// Check if scope is too large
		if isTooLarge, explanation := claude.IsScopeTooLarge(claudeResult); isTooLarge {
			r.handleScopeTooLarge(bc, claudeResult, explanation)
			return false
		}

		// Success — exit the retry loop
		if claudeResult.Success {
			return true
		}

		// Check for context cancellation before analysis
		select {
		case <-ctx.Done():
			r.log("Context cancelled, stopping")
			bc.result.Error = ctx.Err()
			return false
		default:
		}

		// Analyze failure and decide: retry, escalate, or stop
		if r.analyzeAndHandleFailure(ctx, bc, claudeResult) {
			continue
		}
		return false
	}
}

func (r *Runner) selectModel(b *bead.Bead) string {
	if b == nil {
		return "sonnet"
	}
	if r.cfg == nil {
		return "sonnet"
	}
	// Experiment: route test-only beads to haiku unless an explicit complexity label overrides
	if bead.IsTestOnlyBead(b.Title) {
		for _, label := range b.Labels {
			if _, ok := r.cfg.Models.Labels[label]; ok {
				return r.cfg.SelectModel(b.Priority, b.Labels)
			}
		}
		return "haiku"
	}
	return r.cfg.SelectModel(b.Priority, b.Labels)
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

	// Load state for last retro time
	sf, err := state.NewFile(r.gromitDir)
	if err != nil {
		return // Silently skip if state can't be created
	}
	if err := sf.Load(); err != nil {
		return // Silently skip if state can't be loaded
	}

	// Compute failure rate from logs
	stats, err := logger.ReadAllLogs(r.cfg.Paths.Logs)
	if err != nil {
		stats = logger.RunStats{} // Use zero stats on error
	}

	should, reason := lf.ShouldSuggestRetro(sf.LastRetro(), stats.FailureRate())
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
	r.log("%s", formatHealth(stateFile.LastRetro(), stateFile.IterationsSinceReview()))
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
	if r.claude == nil {
		return nil, fmt.Errorf("runner claude client is nil")
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

	// Call Claude with opus model
	claudeResult, err := r.claude.Run(ctx, decomposedPrompt, "opus")
	if err != nil {
		return nil, fmt.Errorf("claude decomposition: %w", err)
	}
	if claudeResult == nil {
		return nil, fmt.Errorf("claude decomposition returned nil result")
	}

	if !claudeResult.Success {
		return nil, fmt.Errorf("claude decomposition failed with exit code %d", claudeResult.ExitCode)
	}

	// Parse the output
	subTasks, err := parseDecomposeOutput(claudeResult.Output)
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
		subTasks[i].normalizeNilFields()
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

	if r == nil || b == nil || r.cfg == nil || r.renderer == nil || r.claude == nil {
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

	// Call Claude with configured model and timeout
	precheckTimeout := time.Duration(r.cfg.Precheck.TimeoutSeconds) * time.Second
	precheckCtx2, cancel := context.WithTimeout(ctx, precheckTimeout)
	defer cancel()

	claudeResult, err := r.claude.Run(precheckCtx2, precheckPrompt, r.cfg.Precheck.Model)
	if err != nil {
		r.log("Warning: precheck invocation failed: %v", err)
		return false, time.Since(start)
	}
	if claudeResult == nil {
		r.log("Warning: precheck returned nil result")
		return false, time.Since(start)
	}

	if !claudeResult.Success {
		r.log("Warning: precheck failed with exit code %d", claudeResult.ExitCode)
		return false, time.Since(start)
	}

	// Check for PRECHECK_PASSED signal
	passed := strings.Contains(claudeResult.Output, "PRECHECK_PASSED")

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
	if r == nil || b == nil || r.cfg == nil || r.renderer == nil || r.claude == nil {
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

	// Call Claude with haiku model
	scopeCheckModel := r.cfg.ScopeCheck.Model
	if scopeCheckModel == "" {
		scopeCheckModel = "haiku"
	}

	claudeResult, err := r.claude.Run(ctx, scopePrompt, scopeCheckModel)
	if err != nil {
		r.log("Warning: scope check invocation failed: %v", err)
		return nil
	}
	if claudeResult == nil {
		r.log("Warning: scope check returned nil result")
		return nil
	}

	if !claudeResult.Success {
		r.log("Warning: scope check failed with exit code %d", claudeResult.ExitCode)
		return nil
	}

	// Parse the scope estimate
	estimate, err := prompt.ParseScopeEstimate(claudeResult.Output)
	if err != nil {
		r.log("Warning: failed to parse scope estimate: %v", err)
		return nil
	}

	return estimate
}

// selectReviewModel determines which model to use for code review.
// If match_build_model is true and build used opus, review uses opus.
// Otherwise, uses the configured review.model (default: sonnet).
func selectReviewModel(cfg *config.Config, buildModel string) string {
	if cfg == nil {
		return "sonnet"
	}
	if cfg.Review.ShouldMatchBuildModel() && buildModel == "opus" {
		return "opus"
	}
	return cfg.Review.Model
}

// runLightReview runs a post-iteration code review.
// Gets diff from startCommit, builds ReviewContext, renders prompt, calls Claude, parses result.
// If deadline is set and approaching, may reduce timeout or skip the review.
func (r *Runner) runLightReview(ctx context.Context, b *bead.Bead, parent *bead.Bead, startCommit string, buildModel string, iteration int, deadline time.Time) (*review.ReviewResult, error) {
	if r == nil {
		return nil, fmt.Errorf("runner is nil")
	}
	if r.cfg == nil {
		return nil, fmt.Errorf("runner config is nil")
	}
	if r.renderer == nil {
		return nil, fmt.Errorf("runner renderer is nil")
	}
	if r.claude == nil {
		return nil, fmt.Errorf("runner claude client is nil")
	}
	if b == nil {
		return nil, fmt.Errorf("bead is nil")
	}

	// Check if we have time for a review
	reviewTimeout := time.Duration(r.cfg.Review.Timeout) * time.Second
	if !deadline.IsZero() {
		timeRemaining := time.Until(deadline)
		if timeRemaining <= 0 {
			r.log("Time budget expired, skipping light review")
			return nil, nil
		}
		if timeRemaining < reviewTimeout {
			r.log("Insufficient time remaining for light review (need %v, have %v), skipping", reviewTimeout, timeRemaining)
			return nil, nil
		}
	}

	// Get diff from start commit to current state
	diff, err := r.getDiff(startCommit)
	if err != nil {
		return nil, fmt.Errorf("getting git diff for review: %w", err)
	}
	if strings.TrimSpace(diff) == "" {
		return nil, nil // No changes to review
	}

	// Build review context
	reviewCtx := &prompt.ReviewContext{
		Bead:       b,
		ParentBead: parent,
		Diff:       diff,
		Model:      selectReviewModel(r.cfg, buildModel),
	}

	// Load CLAUDE.md and rules
	reviewCtx.ClaudeMD, _ = r.renderer.LoadClaudeMD()
	reviewCtx.Rules, _ = r.renderer.LoadRules()

	// Load spec if present
	specName := bead.FindSpecLabel(b.Labels)
	if specName == "" && parent != nil {
		specName = bead.FindSpecLabel(parent.Labels)
	}
	if specName != "" {
		reviewCtx.Spec, _ = r.renderer.LoadSpec(specName)
	}

	// Add validation commands to context
	if r.cfg.Validation.Enabled {
		reviewCtx.ValidationCommands = r.cfg.Validation.Commands
	}

	// Render review prompt
	reviewPrompt, err := r.renderer.RenderReview(reviewCtx)
	if err != nil {
		return nil, fmt.Errorf("rendering review prompt: %w", err)
	}

	// Select model for review
	model := selectReviewModel(r.cfg, buildModel)

	// Call Claude with timeout
	timeout := time.Duration(r.cfg.Review.Timeout) * time.Second
	reviewTimeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	claudeResult, err := r.claude.Run(reviewTimeoutCtx, reviewPrompt, model)
	if err != nil {
		return nil, fmt.Errorf("review invocation: %w", err)
	}
	if claudeResult == nil {
		return nil, fmt.Errorf("review returned nil result")
	}

	// Parse review result
	result, err := review.ParseReviewResult(claudeResult.Output)
	if err != nil {
		return nil, fmt.Errorf("parsing review result: %w", err)
	}

	return result, nil
}

// buildReviewBeadLabels constructs the label list for a bead created from a review proposal.
// It always includes "from-review" and adds any additional labels from the proposal (avoiding duplicates).
func buildReviewBeadLabels(proposalLabels []string) []string {
	labels := []string{"from-review"}
	for _, l := range proposalLabels {
		if l != "from-review" { // avoid duplication
			labels = append(labels, l)
		}
	}
	return labels
}

// buildBacklogLabels constructs the label list for a backlog item created from a review.
// Backlog items always get both "from-review" and "backlog" labels.
func buildBacklogLabels() []string {
	return []string{"from-review", "backlog"}
}

// applyReviewResult creates beads from review findings.
// BeadsToCreate entries are created with their specified priority and labels (plus "from-review").
// BacklogItems entries are created as P2 with both "from-review" and "backlog" labels.
// Returns the count of beads created and backlog items created.
func (r *Runner) applyReviewResult(result *review.ReviewResult) (beadsCreated int, backlogCreated int) {
	if r == nil {
		return 0, 0
	}
	if result == nil || r.beads == nil {
		return 0, 0
	}

	// Create regular beads from review proposals
	for _, bp := range result.BeadsToCreate {
		labels := buildReviewBeadLabels(bp.Labels)
		_, err := r.beads.CreateWithParentAndDescription(
			bp.Title,
			bp.Priority,
			labels,
			nil, // no expected outputs
			"",  // no parent
			bp.Description,
		)
		if err != nil {
			r.log("Warning: failed to create review bead: %v", err)
			continue
		}
		beadsCreated++
		r.log("Created review bead: %s (P%d)", bp.Title, bp.Priority)
	}

	// Create backlog items as P2 beads
	for _, bi := range result.BacklogItems {
		labels := buildBacklogLabels()
		// Build description from description + reason
		description := bi.Description
		if bi.Reason != "" {
			if description != "" {
				description += "\n\n"
			}
			description += "Reason for backlog: " + bi.Reason
		}
		_, err := r.beads.CreateWithParentAndDescription(
			bi.Title,
			2, // P2 for backlog
			labels,
			nil, // no expected outputs
			"",  // no parent
			description,
		)
		if err != nil {
			r.log("Warning: failed to create backlog bead: %v", err)
			continue
		}
		backlogCreated++
		r.log("Created backlog bead: %s (reason: %s)", bi.Title, bi.Reason)
	}

	return beadsCreated, backlogCreated
}

// writeReviewLog writes a review result to the iteration log
func (r *Runner) writeReviewLog(iteration int, beadID string, model string, result *review.ReviewResult, beadsCreated, backlogCreated int, duration time.Duration) {
	if r == nil || r.logger == nil || result == nil {
		return
	}
	r.logger.LogReview(&logger.ReviewLog{
		Timestamp:      time.Now(),
		Type:           "review",
		ReviewType:     "light",
		Iteration:      iteration,
		BeadID:         beadID,
		Model:          model,
		Passed:         result.Passed,
		FixesApplied:   len(result.FixesApplied),
		BeadsCreated:   beadsCreated,
		BacklogCreated: backlogCreated,
		DurationMs:     duration.Milliseconds(),
	})
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
	cmd := exec.Command("git", "push")
	cmd.Stdout = r.output
	cmd.Stderr = r.output

	if err := cmd.Run(); err != nil {
		if r.cfg.Git.PushFailure == "stop" {
			return fmt.Errorf("git push failed: %w", err)
		}
		r.log("Warning: git push failed: %v", err)
		return nil
	}

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

// runThoroughReview runs a periodic thorough review of all changes since the last review
// If deadline is set and approaching, may reduce timeout or skip the review
func (r *Runner) runThoroughReview(ctx context.Context, sf *state.File, iteration int, deadline time.Time) {
	start := time.Now()

	// Check if we have time for a review
	if !deadline.IsZero() {
		timeRemaining := time.Until(deadline)
		minReviewTime := time.Duration(r.cfg.Review.Thorough.Timeout) * time.Second
		if timeRemaining <= 0 {
			r.log("Time budget expired, skipping thorough review")
			return
		}
		if timeRemaining < minReviewTime {
			r.log("Insufficient time remaining for thorough review (need %v, have %v), skipping", minReviewTime, timeRemaining)
			return
		}
	}

	// Guard against nil state file
	if sf == nil {
		return
	}

	// Get diff since last review
	fromCommit := sf.LastReviewCommit()
	if fromCommit == "" {
		// No previous review — use a reasonable default
		r.log("No previous review commit found, skipping thorough review scope detection")
		return
	}

	diff, err := r.getDiff(fromCommit)
	if err != nil {
		r.log("Warning: could not get diff for thorough review: %v", err)
		return
	}
	if strings.TrimSpace(diff) == "" {
		r.log("No changes since last thorough review, skipping")
		return
	}

	// Build context
	reviewCtx := &prompt.ThoroughReviewContext{
		Diff:  diff,
		Model: r.cfg.Review.Thorough.Model,
	}
	reviewCtx.ClaudeMD, _ = r.renderer.LoadClaudeMD()
	reviewCtx.Rules, _ = r.renderer.LoadRules()

	// TODO: populate CompletedBeads from iteration logs
	// For now, leave empty - the review can still work from the diff

	// Render prompt
	reviewPrompt, err := r.renderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		r.log("Warning: could not render thorough review prompt: %v", err)
		return
	}

	// Call Claude with opus
	timeout := time.Duration(r.cfg.Review.Thorough.Timeout) * time.Second
	reviewCtxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	model := r.cfg.Review.Thorough.Model
	r.log("Running thorough review with model: %s", model)
	claudeResult, err := r.claude.Run(reviewCtxTimeout, reviewPrompt, model)
	if err != nil {
		r.log("Warning: thorough review failed: %v", err)
		return
	}
	if claudeResult == nil {
		r.log("Warning: thorough review returned nil result")
		return
	}

	// Parse and apply
	result, err := review.ParseReviewResult(claudeResult.Output)
	if err != nil {
		r.log("Warning: could not parse thorough review result: %v", err)
		return
	}

	r.log("Thorough review: %s", result.Summary)
	beadsCreated, backlogCreated := r.applyReviewResult(result)

	// If fixes applied, re-validate
	if len(result.FixesApplied) > 0 && r.cfg.Validation.Enabled {
		r.log("Thorough review applied %d fixes, re-validating...", len(result.FixesApplied))
		workDir, _ := os.Getwd()
		valResult, err := r.claude.RunValidation(ctx, r.cfg.Validation.Commands, r.cfg.Models.Validation, workDir)
		if err != nil || valResult == nil || !claude.IsValidationPassed(valResult) {
			r.log("Warning: thorough review fixes broke validation")
		} else {
			r.log("Re-validation passed")
		}
	}

	// Log review
	if r.logger != nil {
		r.logger.LogReview(&logger.ReviewLog{
			Timestamp:      time.Now(),
			Type:           "review",
			ReviewType:     "thorough",
			Iteration:      iteration,
			Model:          model,
			Passed:         result.Passed,
			FixesApplied:   len(result.FixesApplied),
			BeadsCreated:   beadsCreated,
			BacklogCreated: backlogCreated,
			DurationMs:     time.Since(start).Milliseconds(),
		})
	}

	// Update state
	currentCommit, err := getGitHead()
	if err != nil {
		r.log("Warning: could not get current commit: %v", err)
	} else {
		if err := sf.RecordReview(currentCommit, iteration); err != nil {
			r.log("Warning: could not record review state: %v", err)
		} else {
			r.log("Recorded thorough review at commit %s", currentCommit[:8])
		}
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
