package display

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
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

// FormatHealth renders health information for display.
func FormatHealth(lastRetro time.Time, iterationsSinceReview int) string {
	return "Health:\n"
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
