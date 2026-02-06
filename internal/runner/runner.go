package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/ralph-runner/internal/analyzer"
	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/learnings"
	"github.com/danabrams/ralph-runner/internal/logger"
	"github.com/danabrams/ralph-runner/internal/preflight"
	"github.com/danabrams/ralph-runner/internal/prompt"
	"github.com/danabrams/ralph-runner/internal/state"
)

// Runner orchestrates the Ralph loop
type Runner struct {
	cfg          *config.Config
	beads        *bead.Client
	claude       *claude.Client
	analyzer     *analyzer.Analyzer
	renderer     *prompt.Renderer
	logger       *logger.Logger
	streamLogger *logger.StreamLogger
	output       io.Writer
	ralphDir     string
}

// NewRunner creates a new runner
func NewRunner(cfg *config.Config, output io.Writer) *Runner {
	if cfg == nil {
		return nil
	}
	if output == nil {
		output = os.Stdout
	}
	// Create logger (ignore error - logging is optional)
	log, err := logger.NewLogger(cfg.Paths.Logs)
	if err != nil {
		fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
	}

	claudeClient := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)

	// Determine ralph directory (parent of templates dir)
	ralphDir := filepath.Dir(cfg.Paths.Templates)

	renderer := prompt.NewRenderer(
		cfg.Paths.Templates,
		cfg.Paths.Specs,
		cfg.Paths.ProjectClaudeMD,
		ralphDir,
	)

	return &Runner{
		cfg:      cfg,
		beads:    bead.NewClient(),
		claude:   claudeClient,
		analyzer: analyzer.NewAnalyzer(claudeClient, cfg.Models.Validation, renderer),
		renderer: renderer,
		logger:   log,
		output:   output,
		ralphDir: ralphDir,
	}
}

// IterationResult captures the outcome of one loop iteration
type IterationResult struct {
	BeadID       string
	BeadTitle    string
	Model        string
	Success      bool
	Validated    bool
	Duration     time.Duration
	Error        error
	Escalated    bool
	EscalatedTo  string
	Output       string
}

// Run executes the Ralph loop
func (r *Runner) Run(ctx context.Context, maxIterations int, dryRun bool) error {
	if r == nil {
		return fmt.Errorf("runner is nil")
	}
	if r.cfg == nil {
		return fmt.Errorf("runner config is nil")
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

		// Get next bead
		b, err := r.beads.Ready()
		if err != nil {
			return fmt.Errorf("getting next bead: %w", err)
		}

		if b == nil {
			r.log("No more work available, stopping")
			break
		}

		// Check if bead is stuck (too many cross-run failures)
		if r.isStuckBead(b) {
			r.log("Bead %s marked as stuck (exceeded failure threshold), skipping", b.ID)
			// Mark as complete to remove from queue
			if err := r.beads.Close(b.ID); err != nil {
				r.log("Warning: failed to close stuck bead: %v", err)
			}
			if err := r.beads.Sync(); err != nil {
				r.log("Warning: failed to sync beads: %v", err)
			}
			continue
		}

		iteration++
		r.log("\n=== Iteration %d ===", iteration)
		r.log("Bead: %s - %s", b.ID, b.Title)

		if dryRun {
			r.log("[DRY RUN] Would process bead %s with model %s", b.ID, r.selectModel(b))
			continue
		}

		// Process the bead
		result := r.processBead(ctx, b, iteration)

		// Log result to console
		r.logResult(result)

		// Log result to file
		r.writeIterationLog(iteration, result)

		// Handle failure
		if !result.Success {
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
	}

	r.log("\nRalph loop complete. Processed %d iterations.", iteration)

	// Check if retro should be suggested
	r.checkRetroSuggestion()

	return nil
}

func (r *Runner) writeIterationLog(iteration int, result *IterationResult) {
	if r.logger == nil || result == nil {
		return
	}

	errStr := ""
	if result.Error != nil {
		errStr = result.Error.Error()
	}

	r.logger.LogIteration(&logger.IterationLog{
		Timestamp:   time.Now(),
		Iteration:   iteration,
		BeadID:      result.BeadID,
		BeadTitle:   result.BeadTitle,
		Model:       result.Model,
		Success:     result.Success,
		Validated:   result.Validated,
		Escalated:   result.Escalated,
		EscalatedTo: result.EscalatedTo,
		DurationMs:  result.Duration.Milliseconds(),
		Error:       errStr,
	})
}

func (r *Runner) processBead(ctx context.Context, b *bead.Bead, iteration int) *IterationResult {
	result := &IterationResult{
		BeadID:    b.ID,
		BeadTitle: b.Title,
	}

	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	if r.cfg == nil {
		result.Error = fmt.Errorf("runner config is nil")
		return result
	}

	// Apply per-bead timeout to prevent unbounded accumulation of retries/analysis/validation
	beadTimeout := time.Duration(r.cfg.Claude.BeadTimeout) * time.Second
	parentCtx := ctx // Keep reference to detect bead timeout vs user Ctrl+C
	beadCtx, beadCancel := context.WithTimeout(ctx, beadTimeout)
	defer beadCancel()
	ctx = beadCtx

	// Capture git HEAD before starting work (for partial progress detection)
	startCommit, err := getGitHead()
	if err != nil {
		r.log("Warning: could not capture git HEAD: %v", err)
		startCommit = "" // Continue anyway
	}

	// Get parent bead if exists
	parent, err := r.beads.GetParent(b)
	if err != nil {
		r.log("Warning: failed to get parent bead: %v", err)
	}

	// Select initial model
	model := r.selectModel(b)
	result.Model = model

	// Build prompt context
	promptCtx, err := r.renderer.BuildContext(b, parent, iteration, model)
	if err != nil {
		result.Error = fmt.Errorf("building prompt context: %w", err)
		return result
	}
	if promptCtx == nil {
		result.Error = fmt.Errorf("building prompt context: returned nil")
		return result
	}

	// Render build prompt
	buildPrompt, err := r.renderer.RenderBuild(promptCtx)
	if err != nil {
		result.Error = fmt.Errorf("rendering build prompt: %w", err)
		return result
	}

	// Execute with analysis and escalation on failure
	var claudeResult *claude.Result
	retriesThisModel := 0
	totalRetriesThisBead := 0
	maxRetries := r.cfg.Escalation.MaxRetriesPerModel
	maxRetriesPerBead := r.cfg.Escalation.MaxRetriesPerBead
	for {
		// Check for context cancellation before each invocation
		select {
		case <-ctx.Done():
			r.log("Context cancelled, stopping")
			result.Error = ctx.Err()
			return result
		default:
		}

		r.log("Running Claude with model: %s", model)

		// Set up stream stats and heartbeat for this invocation
		stats := logger.NewStreamStats()

		// Create child context so stall cancel doesn't kill the parent
		childCtx, childCancel := context.WithCancel(ctx)
		stallFired := false

		stallTimeout := time.Duration(r.cfg.Claude.StallTimeout) * time.Second
		stallTimeoutActive := time.Duration(r.cfg.Claude.StallTimeoutActive) * time.Second
		stopHeartbeat := r.startHeartbeat(stats, stallTimeout, stallTimeoutActive, func() {
			stallFired = true
			childCancel()
		})

		// Build event handler for firehose streaming
		var handler claude.EventHandler
		if r.streamLogger != nil {
			sl := r.streamLogger
			handler = func(line []byte) {
				logger.ParseAndLogEvent(sl, stats, line)
			}
		}

		claudeResult, err = r.claude.StreamRun(childCtx, buildPrompt, model, r.output, handler)
		stopHeartbeat()
		childCancel() // Ensure cleanup

		if err != nil {
			if stallFired && ctx.Err() == nil {
				// Stall timeout — recoverable, retry or escalate
				retriesThisModel++
				totalRetriesThisBead++

				// Check global bead retry limit first
				if totalRetriesThisBead > maxRetriesPerBead {
					r.log("Max retries per bead exceeded (%d/%d)", totalRetriesThisBead, maxRetriesPerBead)
					result.Error = fmt.Errorf("stall timeout: exceeded max retries per bead (%d)", maxRetriesPerBead)
					return result
				}

				if retriesThisModel <= maxRetries {
					r.log("Stall timeout, retrying with same model (%d/%d)...", retriesThisModel, maxRetries)
					continue
				}
				r.log("Stall timeout, retries exhausted for %s", model)
				// Try escalation
				nextModel := r.cfg.NextEscalationModel(model)
				if nextModel == "" {
					result.Error = fmt.Errorf("stall timeout: exhausted all models")
					return result
				}
				r.log("Escalating from %s to %s after stall", model, nextModel)
				result.Escalated = true
				result.EscalatedTo = nextModel
				model = nextModel
				retriesThisModel = 0
				result.Model = model
				promptCtx.Model = model
				buildPrompt, err = r.renderer.RenderBuild(promptCtx)
				if err != nil {
					result.Error = fmt.Errorf("rendering retry prompt: %w", err)
					return result
				}
				continue
			}
			// Distinguish bead timeout from user Ctrl+C
			if ctx.Err() != nil && parentCtx.Err() == nil {
				result.Error = fmt.Errorf("bead timeout: exceeded %v total processing time", beadTimeout)
			} else {
				result.Error = fmt.Errorf("claude invocation: %w", err)
			}
			return result
		}

		result.Output = claudeResult.Output

		// Check if scope is too large before checking success
		if isTooLarge, explanation := claude.IsScopeTooLarge(claudeResult); isTooLarge {
			// Add comment to bead with the breakdown suggestion
			comment := fmt.Sprintf("Scope too large: %s\n\nThis task needs to be broken down into smaller, more manageable pieces.", explanation)
			if err := r.beads.AddComment(b.ID, comment); err != nil {
				r.log("Warning: failed to add comment to bead: %v", err)
			}
			result.Error = fmt.Errorf("scope too large: %s - needs breakdown", explanation)
			return result
		}

		if claudeResult.Success {
			break // Success, no need to analyze or escalate
		}

		// Check for context cancellation before analysis
		select {
		case <-ctx.Done():
			r.log("Context cancelled, stopping")
			result.Error = ctx.Err()
			return result
		default:
		}

		// Run failure analysis with capped timeout (diagnostic, not productive work)
		r.log("Build failed, running failure analysis...")
		analysisTimeout := time.Duration(r.cfg.Claude.AnalysisTimeout) * time.Second
		analysisCtx, analysisCancel := context.WithTimeout(ctx, analysisTimeout)
		analysis, err := r.analyzer.Analyze(analysisCtx, b, claudeResult.Output)
		analysisCancel()
		if err != nil {
			r.log("Warning: failure analysis failed: %v", err)
		} else if analysis == nil {
			r.log("Warning: failure analysis returned no result")
		} else {
			r.log("Analysis: category=%s, recoverable=%v", analysis.Category, analysis.Recoverable)
			r.log("Root cause: %s", analysis.RootCause)

			// Add learning if extracted
			if analysis.Learning != nil {
				r.log("Learning extracted: %s", *analysis.Learning)
				lf := r.renderer.GetLearningsFile()
				if lf != nil {
					learning, err := lf.Add(b.ID, *analysis.Learning, analysis.LearningCategory())
					if err != nil {
						r.log("Warning: failed to add learning: %v", err)
					} else if learning != nil {
						r.log("Learning added to LEARNINGS.md")
					}
				}
			}

			// Handle unclear spec - skip this bead
			if analysis.Category == analyzer.CategoryUnclearSpec {
				result.Error = fmt.Errorf("spec unclear: %s - needs human review", analysis.RootCause)
				return result
			}

			// Handle task too complex - skip this bead
			if analysis.Category == analyzer.CategoryTaskTooComplex {
				// Add comment to bead explaining it needs decomposition
				comment := fmt.Sprintf("Task too complex: %s\n\nThis task needs to be broken down into smaller, more manageable pieces.", analysis.RootCause)
				if err := r.beads.AddComment(b.ID, comment); err != nil {
					r.log("Warning: failed to add comment to bead: %v", err)
				}

				// Extract and save learning if present
				if analysis.Learning != nil {
					r.log("Learning extracted: %s", *analysis.Learning)
					lf := r.renderer.GetLearningsFile()
					if lf != nil {
						learning, err := lf.Add(b.ID, *analysis.Learning, analysis.LearningCategory())
						if err != nil {
							r.log("Warning: failed to add learning: %v", err)
						} else if learning != nil {
							r.log("Learning added to LEARNINGS.md")
						}
					}
				}

				result.Error = fmt.Errorf("task too complex: %s - needs breakdown", analysis.RootCause)
				return result
			}

			// If recoverable and under retry limit, retry with context (same model)
			if analysis.Recoverable && retriesThisModel < maxRetries {
				retriesThisModel++
				totalRetriesThisBead++

				// Check global bead retry limit
				if totalRetriesThisBead > maxRetriesPerBead {
					r.log("Max retries per bead exceeded (%d/%d)", totalRetriesThisBead, maxRetriesPerBead)
					result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", maxRetriesPerBead)
					return result
				}

				r.log("Failure is recoverable, retrying with context (attempt %d/%d)...", retriesThisModel, maxRetries)
				promptCtx.IsRetry = true
				promptCtx.PrevFailure = claudeResult.Output
				promptCtx.FailureContext = analysis.Suggestion

				buildPrompt, err = r.renderer.RenderBuild(promptCtx)
				if err != nil {
					result.Error = fmt.Errorf("rendering retry prompt: %w", err)
					return result
				}
				continue // Retry with same model
			}

			if analysis.Recoverable {
				r.log("Retry limit reached for model %s (%d attempts)", model, retriesThisModel)
			}
		}

		// Not recoverable or retries exhausted - try escalation
		nextModel := r.cfg.NextEscalationModel(model)
		if nextModel == "" {
			r.log("Build failed, no more models to escalate to")
			if startCommit != "" {
				r.showPartialProgress(b, startCommit)
			}
			result.Error = fmt.Errorf("build failed with all models")
			return result
		}

		// Check if we can escalate without exceeding bead limit
		// (Escalation itself doesn't count as a retry, but will likely trigger retries)
		if totalRetriesThisBead >= maxRetriesPerBead {
			r.log("Cannot escalate: max retries per bead reached (%d/%d)", totalRetriesThisBead, maxRetriesPerBead)
			result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", maxRetriesPerBead)
			return result
		}

		r.log("Escalating from %s to %s", model, nextModel)
		result.Escalated = true
		result.EscalatedTo = nextModel
		model = nextModel
		retriesThisModel = 0 // Reset retry counter for new model
		result.Model = model

		// Update prompt context with failure info for escalation
		promptCtx.IsRetry = true
		promptCtx.PrevFailure = claudeResult.Output
		promptCtx.Model = model

		buildPrompt, err = r.renderer.RenderBuild(promptCtx)
		if err != nil {
			result.Error = fmt.Errorf("rendering retry prompt: %w", err)
			return result
		}
	}

	// Run validation if enabled
	if r.cfg.Validation.Enabled {
		// Pre-flight check: verify required tools are available
		checker := preflight.NewChecker(r.cfg.Preflight, r.output)
		if err := checker.Check(r.cfg.Validation.Commands); err != nil {
			// If tools are missing and user chooses to skip, continue without validation
			r.log("Warning: %v", err)
			// Don't mark as error - skip validation and continue
			result.Success = true
			result.Validated = false
			return result
		}

		r.log("Running validation with model: %s", r.cfg.Models.Validation)

		valResult, err := r.claude.RunValidation(
			ctx,
			r.cfg.Validation.Commands,
			r.cfg.Models.Validation,
			promptCtx.WorkDir,
		)
		if err != nil {
			result.Error = fmt.Errorf("validation invocation: %w", err)
			return result
		}

		if valResult == nil {
			result.Error = fmt.Errorf("validation returned no result")
			return result
		}

		if !claude.IsValidationPassed(valResult) {
			// Display validation output in the terminal
			r.log("\nValidation failed. Output:")
			r.log("%s", valResult.Output)

			// Save full output to a dedicated validation log file
			logPath, err := logger.WriteValidationLog(r.cfg.Paths.Logs, valResult.Output)
			if err != nil {
				r.log("Warning: could not save validation log: %v", err)
			} else {
				r.log("\nFull output saved to: %s", logPath)
			}

			// Show partial progress (git diff + expected outputs)
			if startCommit != "" {
				r.showPartialProgress(b, startCommit)
			}

			// Run analysis on validation failure with capped timeout
			r.log("Running failure analysis...")
			valAnalysisCtx, valAnalysisCancel := context.WithTimeout(ctx, time.Duration(r.cfg.Claude.AnalysisTimeout)*time.Second)
			analysis, err := r.analyzer.Analyze(valAnalysisCtx, b, valResult.Output)
			valAnalysisCancel()
			if err == nil && analysis != nil && analysis.Learning != nil {
				lf := r.renderer.GetLearningsFile()
				if lf != nil {
					lf.Add(b.ID, *analysis.Learning, analysis.LearningCategory())
				}
			}

			result.Error = fmt.Errorf("validation failed")
			result.Output += "\n\n=== VALIDATION OUTPUT ===\n" + valResult.Output
			return result
		}

		result.Validated = true
		r.log("Validation passed")
	}

	result.Success = true
	return result
}

func (r *Runner) selectModel(b *bead.Bead) string {
	if b == nil {
		return "sonnet"
	}
	return r.cfg.SelectModel(b.Priority, b.Labels)
}

func (r *Runner) logResult(result *IterationResult) {
	if result == nil {
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

// startHeartbeat launches a goroutine that prints periodic status updates
// and optionally detects stalls using two-tier timeouts:
//   - stallTimeout: used before Claude has made any tool calls (detecting true hangs)
//   - stallTimeoutActive: used after tool activity (longer, allows thinking pauses)
//
// Returns a function to stop the heartbeat.
func (r *Runner) startHeartbeat(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func()) func() {
	return r.startHeartbeatWithConfig(stats, stallTimeout, stallTimeoutActive, onStall, defaultHeartbeatConfig)
}

func (r *Runner) startHeartbeatWithConfig(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), cfg heartbeatConfig) func() {
	done := make(chan struct{})
	go func() {
		// First heartbeat sooner so hangs are visible quickly
		select {
		case <-done:
			return
		case <-time.After(cfg.InitialDelay):
		}
		r.printHeartbeat(stats)

		heartbeatTicker := time.NewTicker(cfg.HeartbeatRate)
		defer heartbeatTicker.Stop()

		// Stall check runs at shorter intervals for reasonable precision
		var stallTicker *time.Ticker
		if stallTimeout > 0 && onStall != nil {
			stallTicker = time.NewTicker(cfg.StallCheckRate)
			defer stallTicker.Stop()
		}

		for {
			if stallTicker != nil {
				select {
				case <-done:
					return
				case <-heartbeatTicker.C:
					r.printHeartbeat(stats)
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
							r.log("STALL DETECTED (%s): No output from Claude for %v (threshold: %v)",
								tier, stats.TimeSinceLastEvent().Round(time.Second), threshold)
							onStall()
							return
						}
					}
				}
			} else {
				select {
				case <-done:
					return
				case <-heartbeatTicker.C:
					r.printHeartbeat(stats)
				}
			}
		}
	}()
	return func() { close(done) }
}

func (r *Runner) printHeartbeat(stats *logger.StreamStats) {
	toolCalls, filesModified, elapsed := stats.Snapshot()
	minutes := int(elapsed.Minutes())
	seconds := int(elapsed.Seconds()) % 60
	if toolCalls == 0 {
		r.log("[%dm%02ds] Waiting for Claude to respond (may be thinking)...", minutes, seconds)
	} else {
		r.log("[%dm%02ds] %d tool calls, %d files modified", minutes, seconds, toolCalls, filesModified)
	}
}

// checkRetroSuggestion checks if a retro should be suggested and prints a message
func (r *Runner) checkRetroSuggestion() {
	// Load learnings
	lf := learnings.NewFile(r.ralphDir)
	if err := lf.Load(); err != nil {
		return // Silently skip if learnings can't be loaded
	}

	// Load state for last retro time
	sf := state.NewFile(r.ralphDir)
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
	r.log("\nRetro suggested: %d provisional learnings, %d confirmed patterns (%s). Run: ralph retro",
		provisionalCount, confirmedCount, reason)
}

// isStuckBead checks if a bead has failed too many times across runs
func (r *Runner) isStuckBead(b *bead.Bead) bool {
	if r == nil || b == nil || r.cfg == nil {
		return false
	}

	// If threshold is 0 or negative, stuck-bead detection is disabled
	if r.cfg.Loop.StuckBeadThreshold <= 0 {
		return false
	}

	// Read per-bead statistics from logs
	beadStats, err := logger.ReadPerBeadStats(r.cfg.Paths.Logs)
	if err != nil {
		// If we can't read stats, don't mark as stuck
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
	b, err := r.beads.Ready()
	if err != nil {
		return fmt.Errorf("getting ready beads: %w", err)
	}

	if b == nil {
		r.log("No beads ready for work")
		return nil
	}

	r.log("Next bead: %s - %s", b.ID, b.Title)
	r.log("  Priority: P%d", b.Priority)
	r.log("  Labels: %s", strings.Join(b.Labels, ", "))
	r.log("  Model: %s", r.selectModel(b))

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
	if b == nil {
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
