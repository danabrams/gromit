package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/pipeline"
)

// formatPipeline formats pipeline status for display
func formatPipeline(ps *pipeline.PipelineStatus) string {
	if ps == nil {
		return "Pipeline: (status unavailable)\n"
	}

	var lines []string
	lines = append(lines, "Pipeline:")

	// Backlog
	backlogLine := fmt.Sprintf("  Backlog:  %d unrefined idea", ps.UnrefinedCount)
	if ps.UnrefinedCount != 1 {
		backlogLine += "s"
	}
	lines = append(lines, backlogLine)
	lines = append(lines, formatItems(ps.UnrefinedIdeas, 3)...)

	// Specs
	specCount := len(ps.UnplannedSpecs)
	specLine := fmt.Sprintf("  Specs:    %d unplanned", specCount)
	lines = append(lines, specLine)
	lines = append(lines, formatItems(ps.UnplannedSpecs, 3)...)

	// Plans
	planCount := len(ps.UndecomposedPlans)
	planLine := fmt.Sprintf("  Plans:    %d undecomposed", planCount)
	lines = append(lines, planLine)
	lines = append(lines, formatItems(ps.UndecomposedPlans, 3)...)

	// Beads
	beadLine := fmt.Sprintf("  Beads:    %d ready", ps.ReadyBeadCount)
	lines = append(lines, beadLine)

	return strings.Join(lines, "\n")
}

// formatItems formats a list of items, showing up to maxShow items and an overflow message
func formatItems(items []string, maxShow int) []string {
	if len(items) == 0 {
		return nil
	}

	var lines []string
	show := len(items)
	if show > maxShow {
		show = maxShow
	}

	for i := 0; i < show; i++ {
		lines = append(lines, fmt.Sprintf("    - %s", items[i]))
	}

	if len(items) > maxShow {
		overflow := len(items) - maxShow
		lines = append(lines, fmt.Sprintf("    (and %d more)", overflow))
	}

	return lines
}

// formatRun formats run status for display
func formatRun(s *Status) string {
	if s == nil {
		return "Run: not running\n"
	}

	var lines []string

	if s.Running {
		// Running with or without limits
		lines = append(lines, formatRunningLine(s))
		lines = append(lines, fmt.Sprintf("  Current:  %s — %q", s.BeadID, s.BeadTitle))
		lines = append(lines, fmt.Sprintf("  Model:    %s", s.Model))
	} else {
		// Not running, but we have last run info
		lines = append(lines, "Run: not running")
		if !s.StartedAt.IsZero() {
			elapsed := time.Since(s.StartedAt)
			lastRunLine := fmt.Sprintf("  Last run: %s ago, %d iteration", formatDuration(elapsed), s.Iteration)
			if s.Iteration != 1 {
				lastRunLine += "s"
			}
			lastRunLine += " completed"
			lines = append(lines, lastRunLine)
		}
	}

	return strings.Join(lines, "\n")
}

// formatRunningLine formats the "Run: iteration X/Y, Zm of Wm elapsed" line
func formatRunningLine(s *Status) string {
	elapsed := formatDuration(time.Duration(s.ElapsedS) * time.Second)

	// Build iteration part
	iterPart := fmt.Sprintf("iteration %d", s.Iteration)
	if s.MaxIterations > 0 {
		iterPart += fmt.Sprintf("/%d", s.MaxIterations)
	}

	// Build time part
	timePart := elapsed + " elapsed"
	if s.TimeBudgetMinutes > 0 {
		timePart = fmt.Sprintf("%s of %dm elapsed", elapsed, s.TimeBudgetMinutes)
	}

	return fmt.Sprintf("Run: %s, %s", iterPart, timePart)
}

// formatDuration formats a duration for human-readable display
func formatDuration(d time.Duration) string {
	// Round to nearest second for sub-minute durations
	if d < time.Minute {
		seconds := int(d.Round(time.Second).Seconds())
		return fmt.Sprintf("%ds", seconds)
	}

	// For longer durations, show hours and minutes
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatHealth formats health status for display
func formatHealth(lastRetro time.Time, iterationsSinceReview int) string {
	var lines []string
	lines = append(lines, "Health:")

	// Last retro line
	if lastRetro.IsZero() {
		lines = append(lines, "  Last retro:  never")
	} else {
		elapsed := time.Since(lastRetro)
		lines = append(lines, fmt.Sprintf("  Last retro:  %s ago", formatDuration(elapsed)))
	}

	// Last review line
	if iterationsSinceReview == 0 {
		lines = append(lines, "  Last review: never")
	} else {
		iterLine := fmt.Sprintf("  Last review: %d iteration", iterationsSinceReview)
		if iterationsSinceReview != 1 {
			iterLine += "s"
		}
		iterLine += " ago"
		lines = append(lines, iterLine)
	}

	return strings.Join(lines, "\n")
}

// formatRecommendation formats the recommendation for display
func formatRecommendation(rec string) string {
	if rec == "" {
		return ""
	}

	// Build the command hint based on recommendation content
	var hint string
	switch {
	case strings.Contains(rec, "Refine"):
		hint = " (gromit refine)"
	case strings.Contains(rec, "Plan"):
		hint = " (gromit plan)"
	case strings.Contains(rec, "Decompose"):
		hint = " (gromit decompose)"
	case strings.Contains(rec, "Run"):
		hint = " (gromit run)"
	}

	return fmt.Sprintf("Next action: %s%s", rec, hint)
}
