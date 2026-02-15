package reviewpkg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// Router selects providers for review invocations.
type Router interface {
	Select(phase, tier string) (provider.Provider, string)
	SelectCross(buildProvider, tier string) (provider.Provider, string)
}

// BeadClient creates beads from review findings.
type BeadClient interface {
	CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error)
}

// PromptRenderer renders review prompts and loads context files.
type PromptRenderer interface {
	RenderReview(ctx *prompt.ReviewContext) (string, error)
	RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error)
	LoadClaudeMD() (string, error)
	LoadRulesForPhase(phase string) (string, error)
	LoadSpec(name string) (string, error)
}

// StateAccess provides access to thorough review state tracking.
type StateAccess interface {
	LastReviewCommit() string
	RecordReview(commit string, iteration int) error
}

// IterationLogger writes review log entries.
type IterationLogger interface {
	LogReview(log *logger.ReviewLog) error
}

// ValidateFn runs validation commands and returns whether they passed.
type ValidateFn func(ctx context.Context, commands []string, workDir string) (passed bool, err error)

// Reviewer handles light code reviews, result application, and review logging.
type Reviewer struct {
	cfg        *config.Config
	router     Router
	beads      BeadClient
	renderer   PromptRenderer
	gitDiffFn  runtypes.GitDiffFn
	logger     IterationLogger
	logFn      func(format string, args ...interface{})
	validateFn ValidateFn
}

// NewReviewer creates a Reviewer with narrow dependency interfaces.
func NewReviewer(cfg *config.Config, router Router, beads BeadClient, renderer PromptRenderer, gitDiffFn runtypes.GitDiffFn, iterLogger IterationLogger) *Reviewer {
	return &Reviewer{
		cfg:       cfg,
		router:    router,
		beads:     beads,
		renderer:  renderer,
		gitDiffFn: gitDiffFn,
		logger:    iterLogger,
	}
}

// SetValidateFn sets the validation callback for re-validation after review fixes.
func (r *Reviewer) SetValidateFn(fn ValidateFn) {
	if r != nil {
		r.validateFn = fn
	}
}

// SetLogFn sets the logging callback for the Reviewer.
func (r *Reviewer) SetLogFn(fn func(format string, args ...interface{})) {
	if r != nil {
		r.logFn = fn
	}
}

func (r *Reviewer) log(format string, args ...interface{}) {
	if r != nil && r.logFn != nil {
		r.logFn(format, args...)
	}
}

// SelectReviewTier determines the tier for code review based on build model.
// If buildModel is "opus", returns "high". Otherwise uses config tier selection.
func SelectReviewTier(cfg *config.Config, b *bead.Bead, buildModel string) string {
	if buildModel == "opus" {
		return provider.TierHigh
	}
	if cfg == nil || b == nil {
		return provider.TierMedium
	}
	return cfg.SelectTier(b.Priority, b.Labels)
}

// RunLight runs a post-iteration code review.
// Gets diff from startCommit, builds ReviewContext, renders prompt, calls Claude, parses result.
// If deadline is set and approaching, may skip the review.
func (r *Reviewer) RunLight(ctx context.Context, b *bead.Bead, parent *bead.Bead, startCommit string, buildModel string, iteration int, deadline time.Time, buildProvider string) (*review.ReviewResult, error) {
	if r == nil {
		return nil, fmt.Errorf("reviewer is nil")
	}
	if r.cfg == nil {
		return nil, fmt.Errorf("reviewer config is nil")
	}
	if r.renderer == nil {
		return nil, fmt.Errorf("reviewer renderer is nil")
	}
	if r.router == nil {
		return nil, fmt.Errorf("reviewer router is nil")
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
	diff, err := r.gitDiffFn(startCommit)
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
	reviewCtx.Rules, _ = r.renderer.LoadRulesForPhase("review")

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

	// Select tier for review
	tier := SelectReviewTier(r.cfg, b, buildModel)

	// Select provider via router — use cross-review if configured
	var p provider.Provider
	if r.cfg.Routing.PhasePreferences["review"] == "cross" && buildProvider != "" {
		p, _ = r.router.SelectCross(buildProvider, tier)
	} else {
		p, _ = r.router.Select("review", tier)
	}
	if p == nil {
		return nil, fmt.Errorf("no provider available for review")
	}

	// Call provider with timeout
	reviewTimeoutCtx, cancel := context.WithTimeout(ctx, reviewTimeout)
	defer cancel()

	providerResult, err := p.Run(reviewTimeoutCtx, reviewPrompt, tier)
	if err != nil {
		return nil, fmt.Errorf("review invocation: %w", err)
	}
	if providerResult == nil {
		return nil, fmt.Errorf("review returned nil result")
	}

	// Parse review result
	result, err := review.ParseReviewResult(providerResult.Output)
	if err != nil {
		return nil, fmt.Errorf("parsing review result: %w", err)
	}

	return result, nil
}

// selectReviewModel determines which model to use for code review.
func selectReviewModel(cfg *config.Config, buildModel string) string {
	if cfg == nil {
		return "sonnet"
	}
	if cfg.Review.ShouldMatchBuildModel() && buildModel == "opus" {
		return "opus"
	}
	return cfg.Review.Model
}

// ApplyResult creates beads from review findings.
// Returns the count of beads created and backlog items created.
func (r *Reviewer) ApplyResult(result *review.ReviewResult) (beadsCreated int, backlogCreated int) {
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

// buildReviewBeadLabels constructs the label list for a bead created from a review proposal.
func buildReviewBeadLabels(proposalLabels []string) []string {
	labels := []string{"from-review"}
	for _, l := range proposalLabels {
		if l != "from-review" {
			labels = append(labels, l)
		}
	}
	return labels
}

// buildBacklogLabels constructs the label list for a backlog item created from a review.
func buildBacklogLabels() []string {
	return []string{"from-review", "backlog"}
}

// WriteReviewLog writes a review result to the iteration log.
func (r *Reviewer) WriteReviewLog(iteration int, beadID string, model string, result *review.ReviewResult, beadsCreated, backlogCreated int, duration time.Duration) {
	if r == nil || r.logger == nil || result == nil {
		return
	}
	_ = r.logger.LogReview(&logger.ReviewLog{
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

// RunThorough runs a periodic thorough review of all changes since the last review.
// Uses state to track the last review commit. If deadline is set and approaching, skips.
func (r *Reviewer) RunThorough(ctx context.Context, sa StateAccess, iteration int, deadline time.Time, getGitHeadFn func() (string, error)) {
	if r == nil || r.cfg == nil {
		return
	}

	start := time.Now()

	// Check deadline
	thoroughTimeout := time.Duration(r.cfg.Review.Thorough.Timeout) * time.Second
	if !deadline.IsZero() {
		timeRemaining := time.Until(deadline)
		if timeRemaining <= 0 {
			r.log("Time budget expired, skipping thorough review")
			return
		}
		if timeRemaining < thoroughTimeout {
			r.log("Insufficient time remaining for thorough review (need %v, have %v), skipping", thoroughTimeout, timeRemaining)
			return
		}
	}

	// Guard against nil state
	if sa == nil {
		return
	}

	// Get diff since last review
	fromCommit := sa.LastReviewCommit()
	if fromCommit == "" {
		r.log("No previous review commit found, skipping thorough review scope detection")
		return
	}

	diff, err := r.gitDiffFn(fromCommit)
	if err != nil {
		r.log("Warning: could not get diff for thorough review: %v", err)
		return
	}
	if strings.TrimSpace(diff) == "" {
		r.log("No changes since last thorough review, skipping")
		return
	}

	// Build context and render prompt
	if r.renderer == nil {
		r.log("Warning: renderer is nil, cannot render thorough review prompt")
		return
	}
	reviewCtx := &prompt.ThoroughReviewContext{
		Diff:  diff,
		Model: r.cfg.Review.Thorough.Model,
	}
	reviewCtx.ClaudeMD, _ = r.renderer.LoadClaudeMD()
	reviewCtx.Rules, _ = r.renderer.LoadRulesForPhase("review")
	reviewPrompt, err := r.renderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		r.log("Warning: could not render thorough review prompt: %v", err)
		return
	}

	// Select provider (always high tier for thorough review)
	p, _ := r.router.Select("review", provider.TierHigh)
	if p == nil {
		r.log("Warning: no provider available for thorough review")
		return
	}

	// Call provider with timeout
	reviewCtxTimeout, cancel := context.WithTimeout(ctx, thoroughTimeout)
	defer cancel()

	r.log("Running thorough review with tier: high")
	providerResult, err := p.Run(reviewCtxTimeout, reviewPrompt, provider.TierHigh)
	if err != nil {
		r.log("Warning: thorough review failed: %v", err)
		return
	}
	if providerResult == nil {
		r.log("Warning: thorough review returned nil result")
		return
	}

	// Parse and apply
	result, err := review.ParseReviewResult(providerResult.Output)
	if err != nil {
		r.log("Warning: could not parse thorough review result: %v", err)
		return
	}

	r.log("Thorough review: %s", result.Summary)
	beadsCreated, backlogCreated := r.ApplyResult(result)

	// Log review
	if r.logger != nil {
		_ = r.logger.LogReview(&logger.ReviewLog{
			Timestamp:      time.Now(),
			Type:           "review",
			ReviewType:     "thorough",
			Iteration:      iteration,
			Model:          providerResult.Model,
			Passed:         result.Passed,
			FixesApplied:   len(result.FixesApplied),
			BeadsCreated:   beadsCreated,
			BacklogCreated: backlogCreated,
			DurationMs:     time.Since(start).Milliseconds(),
		})
	}

	// Update state
	if getGitHeadFn != nil {
		currentCommit, err := getGitHeadFn()
		if err != nil {
			r.log("Warning: could not get current commit: %v", err)
		} else {
			if err := sa.RecordReview(currentCommit, iteration); err != nil {
				r.log("Warning: could not record review state: %v", err)
			} else if len(currentCommit) >= 8 {
				r.log("Recorded thorough review at commit %s", currentCommit[:8])
			}
		}
	}
}

// RunPostSuccess runs only the review stage after a successful build.
// Calls RunLight, applies the review result, logs it, and re-validates if fixes were applied.
func (r *Reviewer) RunPostSuccess(ctx context.Context, bc *runtypes.BeadContext) error {
	if r == nil || bc == nil {
		return nil
	}

	reviewStart := time.Now()
	r.log("Running post-iteration review with model: %s", selectReviewModel(r.cfg, bc.Model))

	reviewResult, err := r.RunLight(ctx, bc.Bead, bc.Parent, bc.StartCommit, bc.Model, bc.Iteration, bc.RunDeadline, bc.BuildProvider)
	if err != nil {
		r.log("Warning: review failed: %v", err)
		return nil // Review failure is non-blocking
	}

	if reviewResult == nil {
		return nil
	}

	r.log("Review: %s", reviewResult.Summary)

	// If fixes were applied, re-validate
	if len(reviewResult.FixesApplied) > 0 && r.cfg.Validation.Enabled && r.validateFn != nil {
		r.log("Review applied %d fixes, re-validating...", len(reviewResult.FixesApplied))

		passed, err := r.validateFn(ctx, r.cfg.Validation.Commands, bc.PromptCtx.WorkDir)
		if err != nil {
			return fmt.Errorf("review re-validation invocation: %w", err)
		}

		if !passed {
			bc.Result.Output += "\n\n=== REVIEW RE-VALIDATION FAILED ===\n"
			bc.Result.ReviewBrokeValidation = true
			return fmt.Errorf("review fixes broke validation")
		}
		r.log("Re-validation passed")
	}

	// Create beads/backlog from review findings
	beadsCreated, backlogCreated := r.ApplyResult(reviewResult)

	// Log review result
	reviewDuration := time.Since(reviewStart)
	r.WriteReviewLog(bc.Iteration, bc.Bead.ID, selectReviewModel(r.cfg, bc.Model), reviewResult, beadsCreated, backlogCreated, reviewDuration)

	return nil
}
