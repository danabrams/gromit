package runner

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/display"
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

// formatPipeline formats pipeline status for display (thin wrapper, delegates to display package)
func formatPipeline(ps *pipeline.PipelineStatus) string {
	return display.FormatPipeline(ps)
}

// formatIterationPrefix returns "Run: iteration N" or "Run: iteration N of M".
func formatIterationPrefix(s *Status) string {
	prefix := fmt.Sprintf("Run: iteration %d", s.Iteration)
	total := s.IterationTotal
	if total <= 0 && s.MaxIterations > 0 {
		total = s.MaxIterations
	}
	if total > 0 {
		prefix += fmt.Sprintf(" of %d", total)
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

// formatRun formats run status for display (thin wrapper, delegates to display package)
func formatRun(s *Status) string {
	return display.FormatRun(toDisplayRunStatus(s))
}

func formatCompatibility(ctx config.CompatibilityContext) string {
	return display.FormatCompatibility(ctx)
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

// formatHealth formats health status for display (thin wrapper, delegates to display package)
func formatHealth(lastRetro time.Time, iterationsSinceReview int) string {
	return display.FormatHealth(lastRetro, iterationsSinceReview)
}

// formatRecommendation formats a recommendation string with a command hint (thin wrapper, delegates to display package)
func formatRecommendation(rec string) string {
	return display.FormatRecommendation(rec)
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

// FormatSPCSummary formats SPC (Statistical Process Control) trend data for display.
// Exported for use by commands like status --spc.
func FormatSPCSummary(trend *logger.ProcessTrend) string {
	return formatSPCSummary(trend)
}

// formatSPCSummary formats SPC (Statistical Process Control) trend data for display (thin wrapper, delegates to display package)
func formatSPCSummary(trend *logger.ProcessTrend) string {
	return display.FormatSPCSummary(trend)
}

// formatSPCLine formats a single SPC control-limit line for display.
// When isDuration is true, values are shown as milliseconds; otherwise as percentages.
func formatSPCLine(label string, cl logger.TrendControlLimit, isDuration bool) string {
	if isDuration {
		return fmt.Sprintf("  %-10s %s, limits %s..%s",
			label, formatSPCValue(cl.Latest, false), formatSPCValue(cl.LCL, false), formatSPCValue(cl.UCL, false))
	}
	return fmt.Sprintf("  %-10s %s, limits %s..%s",
		label, formatSPCValue(cl.Latest, true), formatSPCValue(cl.LCL, true), formatSPCValue(cl.UCL, true))
}

func formatSPCProviderMetrics(metrics []logger.ProviderMetrics) []string {
	if len(metrics) == 0 {
		return nil
	}

	lines := []string{"  Provider metrics:"}
	for _, pm := range metrics {
		avgDuration := time.Duration(math.Round(pm.AvgDurationMs)) * time.Millisecond
		successPct := int(math.Round(pm.SuccessRate * 100))
		transportPct := int(math.Round(pm.TransportFailureRate * 100))
		lines = append(lines, fmt.Sprintf("    %s: %d invocations, %d%% success (%d/%d), transport %d%% (%d), fallbacks %d, avg %s, $%.2f total cost",
			pm.Name,
			pm.TotalInvocations,
			successPct,
			pm.Successes,
			pm.TotalInvocations,
			transportPct,
			pm.TransportFailures,
			pm.FallbacksTriggered,
			formatDuration(avgDuration),
			pm.TotalCostUSD,
		))
	}

	return lines
}

func formatSPCLeadingIndicators(window logger.ProcessTrendWindow) []string {
	lines := []string{}
	if window.FirstPassSuccess > 0 {
		lines = append(lines, fmt.Sprintf("    first-pass success %d%%", int(math.Round(window.FirstPassSuccess*100))))
	}
	if window.ReworkRate > 0 {
		lines = append(lines, fmt.Sprintf("    rework %d%%", int(math.Round(window.ReworkRate*100))))
	}
	if window.EscalationRate > 0 {
		lines = append(lines, fmt.Sprintf("    escalation %d%%", int(math.Round(window.EscalationRate*100))))
	}
	if window.AvgInputTokens > 0 {
		lines = append(lines, fmt.Sprintf("    input %d tokens", int(math.Round(window.AvgInputTokens))))
	}
	if len(lines) == 0 {
		return nil
	}
	return append([]string{"  Leading indicators:"}, lines...)
}

func formatSPCEconomicMetrics(window logger.ProcessTrendWindow) []string {
	shouldShow := window.AvgCostUSD > 0 || window.AvgCostPerBeadUSD > 0 || window.AvgDurationMs > 0
	if !shouldShow {
		return nil
	}

	lines := []string{"  Economic metrics:"}
	if window.AvgCostUSD > 0 {
		lines = append(lines, fmt.Sprintf("    Avg $%.2f", window.AvgCostUSD))
	}
	if window.AvgCostPerBeadUSD > 0 {
		lines = append(lines, fmt.Sprintf("    cost per bead $%.2f", window.AvgCostPerBeadUSD))
	}
	if window.AvgDurationMs > 0 {
		lines = append(lines, fmt.Sprintf("    avg duration %s", formatDuration(time.Duration(window.AvgDurationMs)*time.Millisecond)))
	}
	return lines
}

func formatSPCNelsonViolations(violations []logger.PatternViolation) []string {
	if len(violations) == 0 {
		return nil
	}

	lines := []string{"  Nelson rule violations:"}
	for _, v := range violations {
		lines = append(lines, fmt.Sprintf("    %s (%s): %s", simplifySPCMetric(v.Metric), v.Rule, v.Message))
	}

	return lines
}

func formatSPCEWMAValues(anomalies []logger.TrendAnomaly) []string {
	if len(anomalies) == 0 {
		return nil
	}

	sorted := make([]logger.TrendAnomaly, len(anomalies))
	copy(sorted, anomalies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Metric < sorted[j].Metric
	})

	lines := []string{"  EWMA values:"}
	for _, anomaly := range sorted {
		lines = append(lines, fmt.Sprintf("    %s: %s, limits %s..%s",
			simplifySPCMetric(anomaly.Metric),
			formatSPCEWMAValue(anomaly.Metric, anomaly.Latest),
			formatSPCEWMAValue(anomaly.Metric, anomaly.LCL),
			formatSPCEWMAValue(anomaly.Metric, anomaly.UCL),
		))
	}

	return lines
}

func formatSPCEWMAValue(metric string, value float64) string {
	switch metric {
	case spcMetricEWMASuccessRate:
		return formatSPCValue(value, true)
	case spcMetricEWMADurationMs:
		return formatSPCValue(value, false)
	case spcMetricEWMACostUSD:
		return fmt.Sprintf("$%.2f", value)
	case spcMetricEWMAInputTokens:
		return fmt.Sprintf("%d tokens", int(math.Round(value)))
	default:
		return fmt.Sprintf("%.2f", value)
	}
}

// formatSPCValue formats a single SPC metric value for display.
// When asPercent is true, v is treated as a ratio (0.0–1.0) and formatted as a percentage.
// When asPercent is false, v is treated as milliseconds and formatted as a duration.
func formatSPCValue(v float64, asPercent bool) string {
	if asPercent {
		pct := v * 100
		if pct < 0 {
			pct = 0
		} else if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf("%d%%", int(math.Round(pct)))
	}
	// Duration in milliseconds.
	if v < 0 {
		v = 0
	}
	d := time.Duration(v) * time.Millisecond
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// simplifySPCMetric returns a short human-friendly label for a known SPC metric
// constant, or returns the metric string unchanged if unrecognized.
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
	case spcMetricEWMASuccessRate:
		return "EWMA success rate"
	case spcMetricEWMACostUSD:
		return "EWMA cost"
	case spcMetricEWMADurationMs:
		return "EWMA duration"
	case spcMetricEWMAInputTokens:
		return "EWMA input tokens"
	default:
		return metric
	}
}

// formatModelPerformance formats per-model performance statistics for display.
func formatModelPerformance(stats map[string]logger.ModelStats) string {
	return display.FormatModelPerformance(stats)
}

// toDisplayRunStatus converts a *runner.Status to a display.RunStatus for rendering.
func toDisplayRunStatus(s *Status) *display.RunStatus {
	if s == nil {
		return nil
	}
	return &display.RunStatus{
		Running:                s.Running,
		Iteration:              s.Iteration,
		IterationTotal:         s.IterationTotal,
		MaxIterations:          s.MaxIterations,
		TimeBudgetMinutes:      s.TimeBudgetMinutes,
		BeadID:                 s.BeadID,
		BeadTitle:              s.BeadTitle,
		Model:                  s.Model,
		ElapsedS:               s.ElapsedS,
		AutonomyRate:           s.AutonomyRate,
		FirstPassSuccessRate:   s.FirstPassSuccessRate,
		MTTRProxyMs:            s.MTTRProxyMs,
		EscalationRatesByClass: s.EscalationRatesByClass,
		RecurrenceCounters:     s.RecurrenceCounters,
	}
}
