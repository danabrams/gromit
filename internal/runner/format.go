package runner

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

const (
	spcMetricRollingSuccessRate   = "rolling_success_rate"
	spcMetricRollingEscalateRate  = "rolling_escalation_rate"
	spcMetricRollingQualityScore  = "rolling_quality_score"
	spcMetricRollingAvgDurationMs = "rolling_avg_duration_ms"
	spcMetricFirstPassSuccessRate = "rolling_first_pass_success_rate"
	spcMetricRollingAvgCostUSD    = "rolling_avg_cost_usd"
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
	beadLine := fmt.Sprintf("  Beads:    %s", formatBeadBreakdown(ps))
	lines = append(lines, beadLine)
	lines = append(lines, formatItems(ps.ReadyBeads, 3)...)

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
		if s.LastFailureClass != "" && s.LastAndonLevel != "" {
			line := fmt.Sprintf("  Andon:    %s @ %s", s.LastFailureClass, s.LastAndonLevel)
			if s.LastTrimDecision != "" {
				line += fmt.Sprintf(" (trim: %s)", s.LastTrimDecision)
			}
			lines = append(lines, line)
		}
		if s.AutonomyRate > 0 || s.FirstPassSuccessRate > 0 || s.MTTRProxyMs > 0 {
			lines = append(lines,
				fmt.Sprintf("  Reliability: autonomy %d%% | first-pass %d%% | MTTR %s",
					int(s.AutonomyRate*100+0.5),
					int(s.FirstPassSuccessRate*100+0.5),
					formatDuration(time.Duration(s.MTTRProxyMs)*time.Millisecond),
				),
			)
		}
		if line := formatEscalationBreakdown(s.EscalationRatesByClass); line != "" {
			lines = append(lines, "  Escalation: "+line)
		}
		if line := formatRecurrenceBreakdown(s.RecurrenceCounters); line != "" {
			lines = append(lines, "  Recurrence: "+line)
		}
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

func formatEscalationBreakdown(rates map[string]float64) string {
	if len(rates) == 0 {
		return ""
	}
	keys := make([]string, 0, len(rates))
	for key := range rates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s %d%%", key, int(rates[key]*100+0.5)))
	}
	return strings.Join(parts, " | ")
}

func formatRecurrenceBreakdown(counters map[string]int) string {
	if len(counters) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counters))
	for key := range counters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s x%d", key, counters[key]))
	}
	return strings.Join(parts, " | ")
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

// formatTimeAgo formats a timestamp as a human-readable relative time
func formatTimeAgo(t time.Time) string {
	elapsed := time.Since(t)
	return formatDuration(elapsed) + " ago"
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

// formatModelPerformance formats model performance stats for display
func formatModelPerformance(stats map[string]logger.ModelStats) string {
	if len(stats) == 0 {
		return "Model Performance: (no data)"
	}

	var lines []string
	lines = append(lines, "Model Performance:")

	// Get model names and sort them for consistent ordering (tier order: opus, sonnet, haiku)
	modelOrder := []string{"opus", "sonnet", "haiku"}
	var models []string
	for _, m := range modelOrder {
		if _, exists := stats[m]; exists {
			models = append(models, m)
		}
	}

	// Add any models not in the standard list (alphabetically)
	var otherModels []string
	for m := range stats {
		if m != "opus" && m != "sonnet" && m != "haiku" {
			otherModels = append(otherModels, m)
		}
	}
	sort.Strings(otherModels)
	models = append(models, otherModels...)

	// Format each model's stats
	for _, model := range models {
		s := stats[model]

		// Calculate success rate percentage (rounded to nearest integer)
		successRate := 0
		if s.Iterations > 0 {
			successRate = int(float64(s.Successes)/float64(s.Iterations)*100 + 0.5)
		}

		// Calculate average cost per iteration
		avgCost := 0.0
		if s.Iterations > 0 {
			avgCost = s.TotalCostUSD / float64(s.Iterations)
		}

		// Format: "  model    XX% success  (X/Y)  avg $X.XX/iter"
		line := fmt.Sprintf("  %-6s  %3d%% success  (%d/%d)  avg $%.2f/iter",
			model, successRate, s.Successes, s.Iterations, avgCost)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func formatSPCSummary(trend *logger.ProcessTrend) string {
	if trend == nil || trend.TotalIterations == 0 {
		return "SPC: (no data)"
	}

	limits := make(map[string]logger.TrendControlLimit, len(trend.ControlLimits))
	for _, limit := range trend.ControlLimits {
		limits[limit.Metric] = limit
	}

	var lines []string
	lines = append(lines, "SPC:")
	lines = append(lines, fmt.Sprintf("  Window:   %d iterations (%d total)", trend.WindowSize, trend.TotalIterations))
	lines = append(lines, formatSPCLine("Success", limits[spcMetricRollingSuccessRate], true))
	lines = append(lines, formatSPCLine("Escalate", limits[spcMetricRollingEscalateRate], true))
	lines = append(lines, formatSPCLine("Quality", limits[spcMetricRollingQualityScore], true))
	lines = append(lines, formatSPCLine("Duration", limits[spcMetricRollingAvgDurationMs], false))

	if len(trend.Anomalies) == 0 {
		lines = append(lines, "  Anomaly:  none")
		return strings.Join(lines, "\n")
	}

	first := trend.Anomalies[0]
	lines = append(lines, fmt.Sprintf("  Anomaly:  %d (%s, %s)", len(trend.Anomalies), simplifySPCMetric(first.Metric), first.Severity))
	return strings.Join(lines, "\n")
}

func formatSPCLine(label string, limit logger.TrendControlLimit, asPercent bool) string {
	if limit.Metric == "" {
		return fmt.Sprintf("  %-8s n/a", label+":")
	}

	latest := formatSPCValue(limit.Latest, asPercent)
	lcl := formatSPCValue(limit.LCL, asPercent)
	ucl := formatSPCValue(limit.UCL, asPercent)
	return fmt.Sprintf("  %-8s %s (limits %s..%s)", label+":", latest, lcl, ucl)
}

func formatSPCValue(v float64, asPercent bool) string {
	if asPercent {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return fmt.Sprintf("%d%%", int(v*100+0.5))
	}

	if v <= 0 {
		return "0s"
	}
	return formatDuration(time.Duration(v) * time.Millisecond)
}

func simplifySPCMetric(metric string) string {
	switch metric {
	case spcMetricRollingSuccessRate:
		return "success"
	case spcMetricFirstPassSuccessRate:
		return "first-pass"
	case spcMetricRollingEscalateRate:
		return "escalation"
	case spcMetricRollingQualityScore:
		return "quality"
	case spcMetricRollingAvgDurationMs:
		return "duration"
	case spcMetricRollingAvgCostUSD:
		return "cost"
	default:
		return metric
	}
}
