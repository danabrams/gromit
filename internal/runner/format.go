package runner

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/pipeline"
)

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	secs := int(math.Round(d.Seconds()))
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	minutes := secs / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	mins := minutes % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// FormatPipeline formats pipeline status for display.
// Exported for acceptance tests in internal/runner/acceptance/.
func FormatPipeline(ps *pipeline.PipelineStatus) string {
	return formatPipeline(ps)
}

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
	beadLine := fmt.Sprintf("  Beads:    %s", formatBeadBreakdown(ps))
	lines = append(lines, beadLine)
	lines = append(lines, formatItems(ps.ReadyBeads, 3)...)

	return strings.Join(lines, "\n")
}

// formatIterationPrefix returns "Run: iteration N" or "Run: iteration N/M".
func formatIterationPrefix(s *Status) string {
	prefix := fmt.Sprintf("Run: iteration %d", s.Iteration)
	if s.MaxIterations > 0 {
		prefix += fmt.Sprintf("/%d", s.MaxIterations)
	}
	return prefix
}

// formatElapsedSuffix returns an elapsed-time string like "2m elapsed" or "5m of 30m elapsed".
func formatElapsedSuffix(s *Status) string {
	elapsed := formatDuration(time.Duration(s.ElapsedS) * time.Second)
	if s.TimeBudgetMinutes > 0 {
		budget := fmt.Sprintf("%dm", s.TimeBudgetMinutes)
		return fmt.Sprintf("%s of %s elapsed", elapsed, budget)
	}
	return fmt.Sprintf("%s elapsed", elapsed)
}

// formatRunningLine formats a single-line summary of iteration and elapsed time.
func formatRunningLine(s *Status) string {
	return formatIterationPrefix(s) + ", " + formatElapsedSuffix(s)
}

// formatRun formats run status for display
func formatRun(s *Status) string {
	if s == nil {
		return "Run: not running"
	}
	if !s.Running {
		var lines []string
		lines = append(lines, "Run: not running")
		if s.Iteration > 0 {
			lines = append(lines, fmt.Sprintf("  Last run: %d iterations completed", s.Iteration))
		}
		return strings.Join(lines, "\n")
	}

	var lines []string
	lines = append(lines, formatIterationPrefix(s))
	lines = append(lines, fmt.Sprintf("  %s", formatElapsedSuffix(s)))

	if s.BeadID != "" {
		lines = append(lines, fmt.Sprintf("  Bead:     %s — %s", s.BeadID, s.BeadTitle))
	}
	if s.Model != "" {
		lines = append(lines, fmt.Sprintf("  Model:    %s", s.Model))
	}

	if rel := formatReliabilityLine(s); rel != "" {
		lines = append(lines, fmt.Sprintf("  %s", rel))
	}
	if esc := formatEscalationBreakdown(s.EscalationRatesByClass); esc != "" {
		lines = append(lines, fmt.Sprintf("  Escalation: %s", esc))
	}
	if rec := formatRecurrenceBreakdown(s.RecurrenceCounters); rec != "" {
		lines = append(lines, fmt.Sprintf("  Recurrence: %s", rec))
	}

	return strings.Join(lines, "\n")
}

// formatReliabilityLine formats the reliability summary line.
// Returns empty string if no reliability data is present.
func formatReliabilityLine(s *Status) string {
	if s.AutonomyRate == 0 && s.FirstPassSuccessRate == 0 && s.MTTRProxyMs == 0 {
		return ""
	}
	autonomy := int(math.Round(s.AutonomyRate * 100))
	firstPass := int(math.Round(s.FirstPassSuccessRate * 100))
	mttr := time.Duration(s.MTTRProxyMs) * time.Millisecond
	return fmt.Sprintf("Reliability: autonomy %d%% | first-pass %d%% | MTTR %s", autonomy, firstPass, formatDuration(mttr))
}

// formatEscalationBreakdown formats escalation rates as "class N% | class N%" sorted alphabetically.
func formatEscalationBreakdown(rates map[string]float64) string {
	if len(rates) == 0 {
		return ""
	}
	keys := make([]string, 0, len(rates))
	for k := range rates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		pct := int(math.Round(rates[k] * 100))
		parts = append(parts, fmt.Sprintf("%s %d%%", k, pct))
	}
	return strings.Join(parts, " | ")
}

// formatRecurrenceBreakdown formats recurrence counters as "class xN | class xN" sorted alphabetically.
func formatRecurrenceBreakdown(counters map[string]int) string {
	if len(counters) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counters))
	for k := range counters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s x%d", k, counters[k]))
	}
	return strings.Join(parts, " | ")
}

// formatHealth formats health status for display
func formatHealth(lastRetro time.Time, iterationsSinceReview int) string {
	var lines []string
	lines = append(lines, "Health:")

	retroStr := "never"
	if !lastRetro.IsZero() {
		retroStr = formatDuration(time.Since(lastRetro)) + " ago"
	}
	lines = append(lines, fmt.Sprintf("  Last retro:  %s", retroStr))

	reviewStr := "never"
	if iterationsSinceReview > 0 {
		unit := "iterations"
		if iterationsSinceReview == 1 {
			unit = "iteration"
		}
		reviewStr = fmt.Sprintf("%d %s ago", iterationsSinceReview, unit)
	}
	lines = append(lines, fmt.Sprintf("  Last review: %s", reviewStr))

	return strings.Join(lines, "\n") + "\n"
}

// formatRecommendation formats a recommendation string with a command hint.
func formatRecommendation(rec string) string {
	if rec == "" {
		return ""
	}
	hint := ""
	for _, mapping := range []struct {
		prefix string
		cmd    string
	}{
		{"Refine", "gromit refine"},
		{"Plan", "gromit plan"},
		{"Decompose", "gromit decompose"},
		{"Run", "gromit run"},
	} {
		if strings.HasPrefix(rec, mapping.prefix) {
			hint = " (" + mapping.cmd + ")"
			break
		}
	}
	return "Next action: " + rec + hint
}

// formatItems formats a list of items, showing up to maxShow items and an overflow message
func formatItems(items []string, maxShow int) []string {
	if len(items) == 0 {
		return nil
	}

	var lines []string
	show := min(len(items), maxShow)

	for i := 0; i < show; i++ {
		lines = append(lines, fmt.Sprintf("    - %s", items[i]))
	}

	if len(items) > maxShow {
		overflow := len(items) - maxShow
		lines = append(lines, fmt.Sprintf("    (and %d more)", overflow))
	}

	return lines
}
