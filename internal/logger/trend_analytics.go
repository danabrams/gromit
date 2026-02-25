package logger

import (
	"math"
	"sort"
	"strings"
)

func computeProviderMetrics(entries []IterationMetric) []ProviderMetrics {
	if len(entries) == 0 {
		return []ProviderMetrics{}
	}

	type providerTotals struct {
		totalInvocations  int
		successes         int
		transportFailures int
		fallbacks         int
		totalDurationMs   int64
		totalCostUSD      float64
		totalInputTokens  int
		totalOutputTokens int
	}

	totalsByProvider := map[string]providerTotals{}
	for _, entry := range entries {
		name := resolveProviderName(entry.Provider, entry.Model)
		totals := totalsByProvider[name]
		totals.totalInvocations++
		if entry.Success {
			totals.successes++
		}
		if entry.FailureCategory == transportDisconnectFailure {
			totals.transportFailures++
		}
		if entry.Escalated {
			totals.fallbacks++
		}
		totals.totalDurationMs += entry.DurationMs
		totals.totalCostUSD += entry.CostUSD
		totals.totalInputTokens += entry.InputTokens
		totals.totalOutputTokens += entry.OutputTokens
		totalsByProvider[name] = totals
	}

	metrics := make([]ProviderMetrics, 0, len(totalsByProvider))
	for name, totals := range totalsByProvider {
		metrics = append(metrics, ProviderMetrics{
			Name:                 name,
			TotalInvocations:     totals.totalInvocations,
			Successes:            totals.successes,
			SuccessRate:          fraction(totals.successes, totals.totalInvocations),
			TransportFailures:    totals.transportFailures,
			TransportFailureRate: fraction(totals.transportFailures, totals.totalInvocations),
			FallbacksTriggered:   totals.fallbacks,
			AvgDurationMs:        averageInt64(totals.totalDurationMs, totals.totalInvocations),
			TotalCostUSD:         totals.totalCostUSD,
			TotalInputTokens:     totals.totalInputTokens,
			TotalOutputTokens:    totals.totalOutputTokens,
		})
	}

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})
	return metrics
}

func fraction(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func averageInt64(total int64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return float64(total) / float64(count)
}

func resolveProviderName(providerName, modelName string) string {
	if providerName != "" {
		return providerName
	}
	lowerModel := strings.ToLower(modelName)
	if strings.Contains(lowerModel, "gpt") || strings.Contains(lowerModel, "codex") {
		return "openai"
	}
	if lowerModel == "" {
		return "unknown"
	}
	return "claude"
}

func summarizePromptTokens(metrics []IterationMetric, windowSize int) PromptTokenSummary {
	summary := newPromptTokenSummary()
	if len(metrics) == 0 {
		return summary
	}

	start := len(metrics) - windowSize
	if start < 0 {
		start = 0
	}

	typeTotals := map[string]int{}
	typeCounts := map[string]int{}
	sectionTotals := map[string]int{}
	absDeltaPct := make([]float64, 0, len(metrics)-start)

	for i := start; i < len(metrics); i++ {
		diag := metrics[i].PromptDiagnostics
		if diag == nil {
			continue
		}

		typeTotals[diag.PromptType] += diag.EstimatedTokens
		typeCounts[diag.PromptType]++

		for section, tokens := range diag.SectionTokens {
			sectionTotals[section] += tokens
		}
		for _, action := range diag.ShapeActions {
			summary.BudgetActionFrequency[action]++
		}
		if diag.ReportedTokens > 0 {
			absDeltaPct = append(absDeltaPct, math.Abs(diag.TokenDeltaPct))
		}
	}

	for promptType, count := range typeCounts {
		if count <= 0 {
			continue
		}
		summary.ByPromptType = append(summary.ByPromptType, PromptTypeSummary{
			PromptType:         promptType,
			InvocationCount:    count,
			AvgEstimatedTokens: float64(typeTotals[promptType]) / float64(count),
		})
	}
	sort.Slice(summary.ByPromptType, func(i, j int) bool {
		if summary.ByPromptType[i].AvgEstimatedTokens == summary.ByPromptType[j].AvgEstimatedTokens {
			return summary.ByPromptType[i].PromptType < summary.ByPromptType[j].PromptType
		}
		return summary.ByPromptType[i].AvgEstimatedTokens > summary.ByPromptType[j].AvgEstimatedTokens
	})

	sections := make([]PromptSectionSummary, 0, len(sectionTotals))
	for section, tokens := range sectionTotals {
		sections = append(sections, PromptSectionSummary{
			Section:         section,
			EstimatedTokens: tokens,
		})
	}
	sort.Slice(sections, func(i, j int) bool {
		if sections[i].EstimatedTokens == sections[j].EstimatedTokens {
			return sections[i].Section < sections[j].Section
		}
		return sections[i].EstimatedTokens > sections[j].EstimatedTokens
	})
	if len(sections) > promptSectionTopLimit {
		sections = sections[:promptSectionTopLimit]
	}
	summary.BySectionTop10 = sections

	summary.ReconciliationDrift = ReconciliationDrift{
		SampleCount:          len(absDeltaPct),
		MeanAbsTokenDeltaPct: meanFloat64(absDeltaPct),
		P95AbsTokenDeltaPct:  percentileFloat64(absDeltaPct, p95Percentile),
	}

	return summary
}
