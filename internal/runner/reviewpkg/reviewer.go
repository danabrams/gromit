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
	"github.com/danabrams/gromit/internal/runner/escalation"
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
	LoadClaudeMD() (string, error)
	LoadRulesForPhase(phase string) (string, error)
	LoadSpec(name string) (string, error)
}

// IterationLogger writes review log entries.
type IterationLogger interface {
	LogReview(log *logger.ReviewLog) error
}

// Reviewer handles light code reviews, result application, and review logging.
type Reviewer struct {
	cfg       *config.Config
	router    Router
	beads     BeadClient
	renderer  PromptRenderer
	gitDiffFn runtypes.GitDiffFn
	logger    IterationLogger
	logFn     func(format string, args ...interface{})
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
// If buildModel is "opus", returns "high". Otherwise uses escalation.SelectTier.
func SelectReviewTier(cfg *config.Config, b *bead.Bead, buildModel string) string {
	if buildModel == "opus" {
		return provider.TierHigh
	}
	return escalation.SelectTier(cfg, b)
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
	timeout := time.Duration(r.cfg.Review.Timeout) * time.Second
	reviewTimeoutCtx, cancel := context.WithTimeout(ctx, timeout)
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
