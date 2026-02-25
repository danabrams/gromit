package logger

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type metricSeriesDefinition struct {
	name          string
	latestSample  func(IterationMetric) float64
	historySample func(IterationMetric) float64
}

type ewmaSeriesDefinition struct {
	name  string
	state func(IterationMetric) EWMAMetricState
}

var trendControlLimitSeries = []metricSeriesDefinition{
	{
		name:          metricRollingSuccessRate,
		latestSample:  func(m IterationMetric) float64 { return m.RollingSuccessRate },
		historySample: func(m IterationMetric) float64 { return boolToFloat64(m.Success) },
	},
	{
		name:          metricRollingFirstPassSuccess,
		latestSample:  func(m IterationMetric) float64 { return m.RollingFirstPassSuccess },
		historySample: func(m IterationMetric) float64 { return m.RollingFirstPassSuccess },
	},
	{
		name:          metricRollingReworkRate,
		latestSample:  func(m IterationMetric) float64 { return m.RollingReworkRate },
		historySample: func(m IterationMetric) float64 { return boolToFloat64(!m.FirstPassSuccess) },
	},
	{
		name:          metricRollingEscalationRate,
		latestSample:  func(m IterationMetric) float64 { return m.RollingEscalationRate },
		historySample: func(m IterationMetric) float64 { return m.RollingEscalationRate },
	},
	{
		name:          metricRollingQualityScore,
		latestSample:  func(m IterationMetric) float64 { return m.RollingQualityScore },
		historySample: func(m IterationMetric) float64 { return m.QualityScore },
	},
	{
		name:          metricRollingAvgDurationMs,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgDurationMs },
		historySample: func(m IterationMetric) float64 { return float64(m.DurationMs) },
	},
	{
		name:          metricRollingAvgValidationMs,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgValidationMs },
		historySample: func(m IterationMetric) float64 { return m.RollingAvgValidationMs },
	},
	{
		name:          metricRollingAvgCostUSD,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgCostUSD },
		historySample: func(m IterationMetric) float64 { return m.CostUSD },
	},
	{
		name:          metricRollingAvgInputTokens,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgInputTokens },
		historySample: func(m IterationMetric) float64 { return float64(m.InputTokens) },
	},
	{
		name:          metricRollingAvgCostPerBeadUSD,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgCostPerBeadUSD },
		historySample: func(m IterationMetric) float64 { return m.RollingAvgCostPerBeadUSD },
	},
	{
		name:          metricRollingPreflightFailure,
		latestSample:  func(m IterationMetric) float64 { return m.RollingPreflightFailureRate },
		historySample: func(m IterationMetric) float64 { return m.RollingPreflightFailureRate },
	},
	{
		name:          metricRollingBuildFailure,
		latestSample:  func(m IterationMetric) float64 { return m.RollingBuildFailureRate },
		historySample: func(m IterationMetric) float64 { return m.RollingBuildFailureRate },
	},
	{
		name:          metricRollingValidationFailure,
		latestSample:  func(m IterationMetric) float64 { return m.RollingValidationFailureRate },
		historySample: func(m IterationMetric) float64 { return m.RollingValidationFailureRate },
	},
	{
		name:          metricRollingTimeoutFailure,
		latestSample:  func(m IterationMetric) float64 { return m.RollingTimeoutFailureRate },
		historySample: func(m IterationMetric) float64 { return m.RollingTimeoutFailureRate },
	},
}

var trendEWMASeries = []ewmaSeriesDefinition{
	{
		name:  metricEWMASuccessRate,
		state: func(m IterationMetric) EWMAMetricState { return m.EWMASuccessRate },
	},
	{
		name:  metricEWMACostUSD,
		state: func(m IterationMetric) EWMAMetricState { return m.EWMACostUSD },
	},
	{
		name:  metricEWMADurationMs,
		state: func(m IterationMetric) EWMAMetricState { return m.EWMADurationMs },
	},
	{
		name:  metricEWMAInputTokens,
		state: func(m IterationMetric) EWMAMetricState { return m.EWMAInputTokens },
	},
}

func controlLimitFromEWMAState(metric string, state EWMAMetricState) TrendControlLimit {
	return TrendControlLimit{
		Metric: metric,
		Latest: state.Z,
		Mean:   state.Mean,
		StdDev: state.StdDev,
		UCL:    state.UCL,
		LCL:    state.LCL,
	}
}

func computeEWMAState(metric string, value float64, values []float64, previousZ float64, hasPrevious bool) EWMAMetricState {
	z := value
	if hasPrevious {
		z = ewmaLambda*value + (1-ewmaLambda)*previousZ
	}
	mean, stddev := meanAndStdDev(values)
	ewmaControlLimitScale := math.Sqrt(ewmaLambda / (2 - ewmaLambda))
	ucl := mean + ewmaSigmaMultiplier*stddev*ewmaControlLimitScale
	lcl := mean - ewmaSigmaMultiplier*stddev*ewmaControlLimitScale
	if isRateMetric(metric) {
		ucl = clamp(ucl, 0, 1)
		lcl = clamp(lcl, 0, 1)
	}
	return EWMAMetricState{
		Lambda: ewmaLambda,
		L:      ewmaSigmaMultiplier,
		Value:  value,
		Z:      z,
		Mean:   mean,
		StdDev: stddev,
		UCL:    ucl,
		LCL:    lcl,
	}
}

func buildStratifiedControlLimits(metrics []IterationMetric) (map[string][]TrendControlLimit, map[string][]TrendAnomaly) {
	limitsByStratum := map[string][]TrendControlLimit{}
	anomaliesByStratum := map[string][]TrendAnomaly{}
	if len(metrics) == 0 {
		return limitsByStratum, anomaliesByStratum
	}

	byStratum := partitionMetricsByStratum(metrics)
	strata := make([]string, 0, len(byStratum))
	for key := range byStratum {
		strata = append(strata, key)
	}
	sort.Strings(strata)

	for _, key := range strata {
		stratumMetrics := byStratum[key]
		if len(stratumMetrics) == 0 {
			continue
		}

		latestMetric := stratumMetrics[len(stratumMetrics)-1]
		limits := make([]TrendControlLimit, 0, len(trendControlLimitSeries))
		anomalies := []TrendAnomaly{}
		for _, metric := range trendControlLimitSeries {
			latestValue := metric.latestSample(latestMetric)
			historyValues := extractMetric(stratumMetrics, metric.historySample)
			limit := computeControlLimit(metric.name, latestValue, historyValues)
			limits = append(limits, limit)
			if anomaly, ok := detectAnomaly(limit); ok {
				anomalies = append(anomalies, anomaly)
			}
		}

		sort.Slice(limits, func(i, j int) bool { return limits[i].Metric < limits[j].Metric })
		sort.Slice(anomalies, func(i, j int) bool { return anomalies[i].Metric < anomalies[j].Metric })
		limitsByStratum[key] = limits
		anomaliesByStratum[key] = anomalies
	}

	return limitsByStratum, anomaliesByStratum
}

func partitionMetricsByStratum(metrics []IterationMetric) map[string][]IterationMetric {
	byStratum := map[string][]IterationMetric{}
	for _, m := range metrics {
		provider := resolveProviderName(m.Provider, m.Model)
		providerKey := providerStratumKey(provider)
		byStratum[providerKey] = append(byStratum[providerKey], m)

		modelKey := modelStratumKey(resolveModelStratumName(m.Model))
		byStratum[modelKey] = append(byStratum[modelKey], m)
	}
	return byStratum
}

func providerStratumKey(providerName string) string {
	return stratumProviderPrefix + providerName
}

func modelStratumKey(modelName string) string {
	return stratumModelPrefix + modelName
}

func resolveModelStratumName(modelName string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func extractMetric(metrics []IterationMetric, pick func(IterationMetric) float64) []float64 {
	values := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		values = append(values, pick(m))
	}
	return values
}

func boolToFloat64(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func computeControlLimit(metric string, latest float64, values []float64) TrendControlLimit {
	mean, stddev := meanAndStdDev(values)
	ucl := mean + controlLimitSigmaMultiplier*stddev
	lcl := mean - controlLimitSigmaMultiplier*stddev

	if isRateMetric(metric) {
		ucl = clamp(ucl, 0, 1)
		lcl = clamp(lcl, 0, 1)
	}

	return TrendControlLimit{
		Metric: metric,
		Latest: latest,
		Mean:   mean,
		StdDev: stddev,
		UCL:    ucl,
		LCL:    lcl,
	}
}

func detectAnomaly(limit TrendControlLimit) (TrendAnomaly, bool) {
	if limit.Latest <= limit.UCL && limit.Latest >= limit.LCL {
		return TrendAnomaly{}, false
	}

	severity := anomalySeverityModerate
	if limit.StdDev == 0 {
		severity = anomalySeverityHigh
	} else {
		distance := math.Abs(limit.Latest-limit.Mean) / limit.StdDev
		if distance >= highSeveritySigmaThreshold {
			severity = anomalySeverityHigh
		}
	}

	dir := anomalyDirectionAbove
	if limit.Latest < limit.LCL {
		dir = anomalyDirectionBelow
	}
	return TrendAnomaly{
		Metric:   limit.Metric,
		Latest:   limit.Latest,
		UCL:      limit.UCL,
		LCL:      limit.LCL,
		Severity: severity,
		Message:  fmt.Sprintf("latest value %.4f is %s control limits [%.4f, %.4f]", limit.Latest, dir, limit.LCL, limit.UCL),
	}, true
}

func detectPatternViolations(metric string, values []float64, centerLine float64) []PatternViolation {
	if len(values) < nelsonRule2MinRunLength {
		return []PatternViolation{}
	}

	runAbove := trailingRunLength(values, func(v float64) bool { return v > centerLine })
	if runAbove >= nelsonRule2MinRunLength {
		return []PatternViolation{newRule2Violation(metric, anomalyDirectionAbove, runAbove, centerLine)}
	}

	runBelow := trailingRunLength(values, func(v float64) bool { return v < centerLine })
	if runBelow >= nelsonRule2MinRunLength {
		return []PatternViolation{newRule2Violation(metric, anomalyDirectionBelow, runBelow, centerLine)}
	}

	return []PatternViolation{}
}

func newRule2Violation(metric, direction string, runLength int, centerLine float64) PatternViolation {
	return PatternViolation{
		Metric:     metric,
		Rule:       nelsonRule2Name,
		Direction:  direction,
		RunLength:  runLength,
		CenterLine: centerLine,
		Message: fmt.Sprintf(
			"latest %d points are %s center line %.4f (Nelson Rule 2)",
			runLength,
			direction,
			centerLine,
		),
	}
}

func trailingRunLength(values []float64, match func(float64) bool) int {
	runLength := 0
	for i := len(values) - 1; i >= 0; i-- {
		if !match(values[i]) {
			break
		}
		runLength++
	}
	return runLength
}

func isRateMetric(metric string) bool {
	m := strings.ToLower(metric)
	return strings.Contains(m, "rate")
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
