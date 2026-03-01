package logger

import (
	"fmt"
	"time"
)

// CauseClass enumerates the SPC cause classification assigned to a metric.
type CauseClass string

const (
	CauseClassSpecial CauseClass = "special_cause"
	CauseClassCommon  CauseClass = "common_cause"
	CauseClassStable  CauseClass = "stable"
)

// CauseClassificationRecord captures the trend classification output for a single metric and stratum.
type CauseClassificationRecord struct {
	Metric             string             `json:"metric"`
	Stratum            string             `json:"stratum,omitempty"`
	Class              CauseClass         `json:"class"`
	Latest             float64            `json:"latest"`
	Drift              float64            `json:"drift,omitempty"`
	Limit              *TrendControlLimit `json:"limit,omitempty"`
	PersistenceWindows int                `json:"persistence_windows"`
	DetectedAt         time.Time          `json:"detected_at,omitempty"`
	Severity           string             `json:"severity,omitempty"`
}

func (r CauseClassificationRecord) Identity() string {
	stratum := r.Stratum
	if stratum == "" {
		stratum = "global"
	}
	return fmt.Sprintf("%s|%s|%s", r.Metric, stratum, r.Class)
}

type CauseClassificationContext struct {
	Metrics                 []IterationMetric
	ControlLimits           []TrendControlLimit
	StratifiedControlLimits map[string][]TrendControlLimit
	Anomalies               []TrendAnomaly
	StratifiedAnomalies     map[string][]TrendAnomaly
	EWMAAnomalies           []TrendAnomaly
	PatternViolations       []PatternViolation
}

type causeMetricMetadata struct {
	name  string
	value func(IterationMetric) float64
}

var causeMetrics = []causeMetricMetadata{
	{
		name:  metricRollingAvgInputTokens,
		value: func(m IterationMetric) float64 { return m.RollingAvgInputTokens },
	},
	{
		name:  metricRollingAvgCostUSD,
		value: func(m IterationMetric) float64 { return m.RollingAvgCostUSD },
	},
	{
		name:  metricRollingAvgDurationMs,
		value: func(m IterationMetric) float64 { return m.RollingAvgDurationMs },
	},
	{
		name:  metricRollingAvgValidationMs,
		value: func(m IterationMetric) float64 { return m.RollingAvgValidationMs },
	},
}

func EvaluateCauseClassifications(ctx CauseClassificationContext) []CauseClassificationRecord {
	if len(ctx.Metrics) == 0 {
		return []CauseClassificationRecord{}
	}

	patterns := buildPatternViolationMap(ctx.PatternViolations)
	globalAnomalies := buildAnomalyMap(ctx.Anomalies)
	ewmaAnomalies := buildAnomalyMap(ctx.EWMAAnomalies)
	stratifiedAnomalies := buildStratifiedAnomalyMap(ctx.StratifiedAnomalies)

	strata := map[string][]IterationMetric{
		"": ctx.Metrics,
	}

	records := make([]CauseClassificationRecord, 0, len(strata)*len(causeMetrics))
	for stratum, metrics := range strata {
		if len(metrics) == 0 {
			continue
		}

		var limits []TrendControlLimit
		if stratum == "" {
			limits = ctx.ControlLimits
		} else if ctx.StratifiedControlLimits != nil {
			limits = ctx.StratifiedControlLimits[stratum]
		}

		stratumAnomalies := stratifiedAnomalies[stratum]

		for _, metric := range causeMetrics {
			latest := metric.value(metrics[len(metrics)-1])
			limit, limitFound := lookupControlLimit(limits, metric.name)

			specialSeverity := ""
			specialFromAnomaly := false

			if stratum == "" {
				if anomaly, ok := globalAnomalies[metric.name]; ok && isSeveritySignificant(anomaly.Severity) {
					specialFromAnomaly = true
					specialSeverity = anomaly.Severity
				} else if anomaly, ok := ewmaAnomalies[metric.name]; ok && isSeveritySignificant(anomaly.Severity) {
					specialFromAnomaly = true
					specialSeverity = anomaly.Severity
				}
			} else if anomaly, ok := stratumAnomalies[metric.name]; ok && isSeveritySignificant(anomaly.Severity) {
				specialFromAnomaly = true
				specialSeverity = anomaly.Severity
			}

			specialCount, specialStartIdx := 0, -1
			if limitFound {
				specialCount, specialStartIdx = countTrailingAnomalyWindows(metrics, metric.value, limit)
			}
			if specialCount == 0 && specialFromAnomaly {
				specialCount = 1
				specialStartIdx = len(metrics) - 1
			}

			patternRun := 0
			patternStartIdx := -1
			if stratum == "" {
				if pattern, ok := patterns[metric.name]; ok && pattern.RunLength > 0 {
					run := pattern.RunLength
					if run > len(metrics) {
						run = len(metrics)
					}
					patternRun = run
					patternStartIdx = len(metrics) - run
				}
			}

			specialPersistence := specialCount
			detectedIdx := specialStartIdx

			if patternRun > specialPersistence {
				specialPersistence = patternRun
				detectedIdx = patternStartIdx
			} else if patternRun > 0 && detectedIdx >= 0 && patternStartIdx >= 0 && patternStartIdx < detectedIdx {
				detectedIdx = patternStartIdx
			} else if patternRun > 0 && specialPersistence == 0 {
				specialPersistence = patternRun
				detectedIdx = patternStartIdx
			}

			var specialDetectedAt time.Time
			if specialPersistence > 0 && detectedIdx >= 0 && detectedIdx < len(metrics) {
				specialDetectedAt = metrics[detectedIdx].Timestamp
			}

			driftPersistence, driftStartIdx := 0, -1
			if limitFound {
				driftPersistence, driftStartIdx = countTrailingDriftWindows(metrics, metric.value, limit.Mean)
			}

			var driftDetectedAt time.Time
			if driftPersistence > 0 && driftStartIdx >= 0 && driftStartIdx < len(metrics) {
				driftDetectedAt = metrics[driftStartIdx].Timestamp
			}

			class := CauseClassStable
			persistence := 0
			detectedAt := time.Time{}
			if specialPersistence > 0 && (specialFromAnomaly || patternRun > 0) {
				class = CauseClassSpecial
				persistence = specialPersistence
				detectedAt = specialDetectedAt
			} else if driftPersistence >= 2 {
				class = CauseClassCommon
				persistence = driftPersistence
				detectedAt = driftDetectedAt
			}

			record := CauseClassificationRecord{
				Metric:             metric.name,
				Stratum:            stratum,
				Class:              class,
				Latest:             latest,
				PersistenceWindows: persistence,
				DetectedAt:         detectedAt,
			}

			if limitFound {
				record.Drift = latest - limit.Mean
			}
			if class == CauseClassSpecial && limitFound {
				limitCopy := limit
				record.Limit = &limitCopy
			}
			if class == CauseClassSpecial && specialSeverity != "" {
				record.Severity = specialSeverity
			}

			records = append(records, record)
		}
	}

	return records
}

func buildAnomalyMap(anomalies []TrendAnomaly) map[string]TrendAnomaly {
	out := map[string]TrendAnomaly{}
	for _, anomaly := range anomalies {
		out[anomaly.Metric] = anomaly
	}
	return out
}

func buildStratifiedAnomalyMap(raw map[string][]TrendAnomaly) map[string]map[string]TrendAnomaly {
	out := map[string]map[string]TrendAnomaly{}
	if len(raw) == 0 {
		return out
	}
	for stratum, anomalies := range raw {
		mapped := map[string]TrendAnomaly{}
		for _, anomaly := range anomalies {
			mapped[anomaly.Metric] = anomaly
		}
		out[stratum] = mapped
	}
	return out
}

func buildPatternViolationMap(patterns []PatternViolation) map[string]PatternViolation {
	out := map[string]PatternViolation{}
	for _, pattern := range patterns {
		out[pattern.Metric] = pattern
	}
	return out
}

func lookupControlLimit(limits []TrendControlLimit, metric string) (TrendControlLimit, bool) {
	for _, limit := range limits {
		if limit.Metric == metric {
			return limit, true
		}
	}
	return TrendControlLimit{}, false
}

func countTrailingAnomalyWindows(metrics []IterationMetric, extract func(IterationMetric) float64, limit TrendControlLimit) (int, int) {
	count := 0
	start := -1
	for i := len(metrics) - 1; i >= 0; i-- {
		value := extract(metrics[i])
		if value > limit.UCL || value < limit.LCL {
			start = i
			count++
			continue
		}
		break
	}
	return count, start
}

func countTrailingDriftWindows(metrics []IterationMetric, extract func(IterationMetric) float64, center float64) (int, int) {
	count := 0
	start := -1
	for i := len(metrics) - 1; i >= 0; i-- {
		if extract(metrics[i]) > center {
			start = i
			count++
			continue
		}
		break
	}
	return count, start
}

func isSeveritySignificant(severity string) bool {
	return severity == anomalySeverityModerate || severity == anomalySeverityHigh
}
