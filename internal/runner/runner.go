package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/ralph-runner/internal/analyzer"
	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/learnings"
	"github.com/danabrams/ralph-runner/internal/logger"
	"github.com/danabrams/ralph-runner/internal/prompt"
	"github.com/danabrams/ralph-runner/internal/state"
)

// Runner orchestrates the Ralph loop
type Runner struct {
	cfg      *config.Config
	beads    *bead.Client
	claude   *claude.Client
	analyzer *analyzer.Analyzer
	renderer *prompt.Renderer
	logger   *logger.Logger
	output   io.Writer
	ralphDir string
}

// NewRunner creates a new runner
func NewRunner(cfg *config.Config, output io.Writer) *Runner {
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
	// Ensure logger is closed when done
	if r.logger != nil {
		defer r.logger.Close()
		r.log("Logging to: %s", r.logger.FilePath())
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
	if r.logger == nil {
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

	// Render build prompt
	buildPrompt, err := r.renderer.RenderBuild(promptCtx)
	if err != nil {
		result.Error = fmt.Errorf("rendering build prompt: %w", err)
		return result
	}

	// Execute with analysis and escalation on failure
	var claudeResult *claude.Result
	for {
		r.log("Running Claude with model: %s", model)

		claudeResult, err = r.claude.StreamRun(ctx, buildPrompt, model, r.output)
		if err != nil {
			result.Error = fmt.Errorf("claude invocation: %w", err)
			return result
		}

		result.Output = claudeResult.Output

		if claudeResult.Success {
			break // Success, no need to analyze or escalate
		}

		// Run failure analysis
		r.log("Build failed, running failure analysis...")
		analysis, err := r.analyzer.Analyze(ctx, b, claudeResult.Output)
		if err != nil {
			r.log("Warning: failure analysis failed: %v", err)
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

			// If recoverable, retry with context (same model)
			if analysis.Recoverable {
				r.log("Failure is recoverable, retrying with context...")
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
		}

		// Not recoverable - try escalation
		nextModel := r.cfg.NextEscalationModel(model)
		if nextModel == "" {
			r.log("Build failed, no more models to escalate to")
			result.Error = fmt.Errorf("build failed with all models")
			return result
		}

		r.log("Escalating from %s to %s", model, nextModel)
		result.Escalated = true
		result.EscalatedTo = nextModel
		model = nextModel
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

		if !claude.IsValidationPassed(valResult) {
			// Run analysis on validation failure too
			r.log("Validation failed, running failure analysis...")
			analysis, err := r.analyzer.Analyze(ctx, b, valResult.Output)
			if err == nil && analysis.Learning != nil {
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
	return r.cfg.SelectModel(b.Priority, b.Labels)
}

func (r *Runner) logResult(result *IterationResult) {
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
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(r.output, msg)
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

// Status returns the current queue status
func (r *Runner) Status() error {
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
