package display

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

// SPC metric name constants used for ProcessTrend control limits.
const (
	spcMetricRollingSuccessRate   = "rolling_success_rate"
	spcMetricRollingEscalateRate  = "rolling_escalation_rate"
	spcMetricRollingQualityScore  = "rolling_quality_score"
	spcMetricRollingAvgDurationMs = "rolling_avg_duration_ms"
	spcMetricFirstPassSuccessRate = "rolling_first_pass_success_rate"
	spcMetricRollingAvgCostUSD    = "rolling_avg_cost_usd"
	spcMetricEWMASuccessRate      = "ewma_success_rate"
	spcMetricEWMACostUSD          = "ewma_cost_usd"
	spcMetricEWMADurationMs       = "ewma_duration_ms"
	spcMetricEWMAInputTokens      = "ewma_input_tokens"
)

// FormatRun renders the current run state for display.
func FormatRun(status *RunStatus) string {
	if status == nil {
		return "Run: not running"
	}
	if !status.Running {
		var lines []string
		lines = append(lines, "Run: not running")
		if status.Iteration > 0 {
			lines = append(lines, fmt.Sprintf("  Last run: %d iterations completed", status.Iteration))
		}
		return strings.Join(lines, "\n")
	}

	var lines []string
	lines = append(lines, formatIterationPrefix(status))
	lines = append(lines, fmt.Sprintf("  %s", formatElapsedSuffix(status)))

	if status.BeadID != "" {
		lines = append(lines, fmt.Sprintf("  Bead:     %s — %s", status.BeadID, status.BeadTitle))
	}
	if status.Model != "" {
		lines = append(lines, fmt.Sprintf("  Model:    %s", status.Model))
	}

	if rel := formatReliabilityLine(status); rel != "" {
		lines = append(lines, fmt.Sprintf("  %s", rel))
	}
	if esc := formatEscalationBreakdown(status.EscalationRatesByClass); esc != "" {
		lines = append(lines, fmt.Sprintf("  Escalation: %s", esc))
	}
	if rec := formatRecurrenceBreakdown(status.RecurrenceCounters); rec != "" {
		lines = append(lines, fmt.Sprintf("  Recurrence: %s", rec))
	}

	return strings.Join(lines, "\n")
}

// FormatPipeline formats pipeline status for display.
func FormatPipeline(ps *pipeline.PipelineStatus) string {
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

// FormatHealth renders health information for display.
func FormatHealth(lastRetro time.Time, iterationsSinceReview int) string {
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

// FormatRecommendation formats a recommendation string with a command hint.
func FormatRecommendation(rec string) string {
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

// FormatModelPerformance formats per-model performance statistics for display.
func FormatModelPerformance(stats map[string]logger.ModelStats) string {
	lines := []string{"Model Performance:"}

	if len(stats) == 0 {
		return strings.Join(lines, "\n")
	}

	models := make([]string, 0, len(stats))
	for m := range stats {
		models = append(models, m)
	}
	sort.Strings(models)

	for _, m := range models {
		s := stats[m]
		displayModel := m
		if strings.TrimSpace(displayModel) == "" {
			displayModel = "(unknown model)"
		}
		if s.Iterations == 0 {
			lines = append(lines, fmt.Sprintf("  %s: no iterations", displayModel))
			continue
		}
		pct := int(math.Round(s.SuccessRate() * 100))
		costPerIter := s.TotalCostUSD / float64(s.Iterations)
		lines = append(lines, fmt.Sprintf("  %s: %d%% (%d/%d) $%.2f/iter",
			displayModel, pct, s.Successes, s.Iterations, costPerIter))
	}

	return strings.Join(lines, "\n")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatBeadBreakdown builds the comma-separated status breakdown for the Beads line
func formatBeadBreakdown(ps *pipeline.PipelineStatus) string {
	if ps == nil {
		return "none"
	}

	var parts []string

	if ps.ReadyBeadCount > 0 {
		parts = append(parts, fmt.Sprintf("%d ready", ps.ReadyBeadCount))
	}
	if ps.InProgressCount > 0 {
		parts = append(parts, fmt.Sprintf("%d in-progress", ps.InProgressCount))
	}
	if ps.BlockedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", ps.BlockedCount))
	}
	if ps.DeferredCount > 0 {
		parts = append(parts, fmt.Sprintf("%d deferred", ps.DeferredCount))
	}

	if ps.ClosedCount > 0 {
		closedPart := fmt.Sprintf("%d closed", ps.ClosedCount)
		if ps.HasRunInfo && ps.ClosedThisRunCount > 0 {
			closedPart += fmt.Sprintf(" (%d this run)", ps.ClosedThisRunCount)
		}
		parts = append(parts, closedPart)
	}

	if len(parts) == 0 {
		return "none"
	}

	return strings.Join(parts, ", ")
}

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

func formatIterationPrefix(status *RunStatus) string {
	prefix := fmt.Sprintf("Run: iteration %d", status.Iteration)
	total := status.IterationTotal
	if total <= 0 && status.MaxIterations > 0 {
		total = status.MaxIterations
	}
	if total > 0 {
		prefix += fmt.Sprintf(" of %d", total)
	}
	return prefix
}

func formatElapsedSuffix(status *RunStatus) string {
	elapsed := formatDuration(time.Duration(status.ElapsedS) * time.Second)
	if status.TimeBudgetMinutes > 0 {
		budget := fmt.Sprintf("%dm", status.TimeBudgetMinutes)
		return fmt.Sprintf("%s of %s elapsed", elapsed, budget)
	}
	return fmt.Sprintf("%s elapsed", elapsed)
}

func formatReliabilityLine(status *RunStatus) string {
	if status.AutonomyRate == 0 && status.FirstPassSuccessRate == 0 && status.MTTRProxyMs == 0 {
		return ""
	}
	autonomy := int(math.Round(status.AutonomyRate * 100))
	firstPass := int(math.Round(status.FirstPassSuccessRate * 100))
	mttr := time.Duration(status.MTTRProxyMs) * time.Millisecond
	return fmt.Sprintf("Reliability: autonomy %d%% | first-pass %d%% | MTTR %s", autonomy, firstPass, formatDuration(mttr))
}

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
