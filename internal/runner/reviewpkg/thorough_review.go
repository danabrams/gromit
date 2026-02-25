package reviewpkg

import (
	"context"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/review"
)

const (
	thoroughReviewPhase = "thorough_review"
	reviewTypeThorough  = "thorough"
)

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
	claudeMD, claudeErr := r.renderer.LoadClaudeMD()
	if claudeErr != nil {
		r.log("Warning: could not load CLAUDE.md for thorough review: %v", claudeErr)
	}
	reviewCtx.ClaudeMD = claudeMD
	rules, rulesErr := r.renderer.LoadRulesForPhase(thoroughReviewPhase)
	if rulesErr != nil {
		r.log("Warning: could not load rules for phase %q: %v", thoroughReviewPhase, rulesErr)
	}
	reviewCtx.Rules = rules
	prompt.ApplyReviewPhaseProfile(reviewCtx, thoroughReviewPhase)
	reviewPrompt, err := r.renderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		r.log("Warning: could not render thorough review prompt: %v", err)
		return
	}

	thoroughTier := r.cfg.Review.Thorough.Tier
	p, _ := r.router.Select(thoroughReviewPhase, thoroughTier)
	if p == nil {
		r.log("Warning: no provider available for thorough review")
		return
	}

	// Call provider with timeout
	reviewCtxTimeout, cancel := context.WithTimeout(ctx, thoroughTimeout)
	defer cancel()

	r.log("Running thorough review with tier: %s", thoroughTier)
	providerResult, err := p.Run(reviewCtxTimeout, reviewPrompt, thoroughTier)
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
			ReviewType:     reviewTypeThorough,
			Iteration:      iteration,
			Model:          providerResult.Model,
			Passed:         result.Passed,
			FixesApplied:   len(result.FixesApplied),
			FixCategories:  append([]string(nil), result.FixCategories...),
			BeadsCreated:   beadsCreated,
			BacklogCreated: backlogCreated,
			DurationMs:     execute.DurationMsFromDuration(time.Since(start)),
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
