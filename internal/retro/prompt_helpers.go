package retro

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
)

func diagnosticJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

func buildRetroSectionTokens(
	rules, learnings string,
	runStats logger.RunStats,
	beadStats map[string]logger.BeadStats,
	efficiency *logger.EfficiencyReport,
	processTrend *logger.ProcessTrend,
) map[string]int {
	return map[string]int{
		prompt.SectionRules:              prompt.EstimateTokens(rules),
		prompt.SectionConfirmedLearnings: prompt.EstimateTokens(learnings),
		prompt.SectionRunStats:           prompt.EstimateTokens(diagnosticJSON(runStats)),
		prompt.SectionBeadStats:          prompt.EstimateTokens(diagnosticJSON(beadStats)),
		sectionEfficiency:                prompt.EstimateTokens(diagnosticJSON(efficiency)),
		sectionProcessTrend:              prompt.EstimateTokens(diagnosticJSON(processTrend)),
	}
}

// PromptDiagnostics returns the last prompt diagnostics snapshot.
func (r *Retro) PromptDiagnostics() *prompt.PromptDiagnostics {
	if r == nil || r.diagnostics == nil {
		return nil
	}

	sectionTokens := make(map[string]int, len(r.diagnostics.SectionTokens))
	for key, value := range r.diagnostics.SectionTokens {
		sectionTokens[key] = value
	}

	diagnostics := *r.diagnostics
	diagnostics.SectionTokens = sectionTokens
	diagnostics.ShapeActions = append([]string{}, r.diagnostics.ShapeActions...)
	return &diagnostics
}

func selectExperimentMetrics(exp *Experiment, efficiency *logger.EfficiencyReport) *ExperimentMetrics {
	if exp == nil || efficiency == nil {
		return nil
	}

	family := normalizeProviderFamily(exp.PrimaryConcern)
	if family != "" {
		if stats, ok := efficiency.CurrentProviderFamilies[family]; ok {
			return experimentMetricsFromStats(family, stats)
		}
	}

	return &ExperimentMetrics{
		ProviderFamily:            providerFamilyMixed,
		CurrentAvgCostPerBead:     efficiency.CurrentAvgCostPerBead,
		CurrentAvgDurationPerBead: efficiency.CurrentAvgDurationPerBead,
	}
}

func experimentMetricsFromStats(family string, stats logger.ModelEfficiency) *ExperimentMetrics {
	return &ExperimentMetrics{
		ProviderFamily:            family,
		CurrentAvgCostPerBead:     stats.AvgCostUSD,
		CurrentAvgDurationPerBead: stats.AvgDuration,
	}
}

func normalizeProviderFamily(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case providerFamilyClaude:
		return providerFamilyClaude
	case providerFamilyCodex:
		return providerFamilyCodex
	default:
		return ""
	}
}
