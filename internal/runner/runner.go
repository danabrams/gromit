package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/logger"
	"github.com/danabrams/ralph-runner/internal/prompt"
)

// Runner orchestrates the Ralph loop
type Runner struct {
	cfg      *config.Config
	beads    *bead.Client
	claude   *claude.Client
	renderer *prompt.Renderer
	logger   *logger.Logger
	output   io.Writer
}

// NewRunner creates a new runner
func NewRunner(cfg *config.Config, output io.Writer) *Runner {
	// Create logger (ignore error - logging is optional)
	log, err := logger.NewLogger(cfg.Paths.Logs)
	if err != nil {
		fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
	}

	return &Runner{
		cfg:    cfg,
		beads:  bead.NewClient(),
		claude: claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout),
		renderer: prompt.NewRenderer(
			cfg.Paths.Templates,
			cfg.Paths.Specs,
			cfg.Paths.ProjectClaudeMD,
		),
		logger: log,
		output: output,
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

	// Execute with escalation on failure
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
			break // Success, no need to escalate
		}

		// Try escalation
		nextModel := r.cfg.NextEscalationModel(model)
		if nextModel == "" {
			r.log("Build failed, no more models to escalate to")
			result.Error = fmt.Errorf("build failed with all models")
			return result
		}

		r.log("Build failed with %s, escalating to %s", model, nextModel)
		result.Escalated = true
		result.EscalatedTo = nextModel
		model = nextModel
		result.Model = model

		// Update prompt context with failure info for retry
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
