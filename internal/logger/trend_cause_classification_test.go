package logger

import (
	"testing"
	"time"
)

func TestCauseClassificationRecordIdentity(t *testing.T) {
	testCases := []struct {
		name   string
		record CauseClassificationRecord
		want   string
	}{
		{
			name: "global",
			record: CauseClassificationRecord{
				Metric: "rolling_avg_cost_usd",
				Class:  CauseClassSpecial,
			},
			want: "rolling_avg_cost_usd|global|special_cause",
		},
		{
			name: "provider stratum",
			record: CauseClassificationRecord{
				Metric:  "rolling_avg_duration_ms",
				Stratum: "provider:claude",
				Class:   CauseClassCommon,
			},
			want: "rolling_avg_duration_ms|provider:claude|common_cause",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.record.Identity(); got != tc.want {
				t.Fatalf("got identity %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEvaluateCauseClassifications_SpecialCauseFromAnomaly(t *testing.T) {
	baseTime := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	metrics := []IterationMetric{
		{
			Timestamp:         baseTime.Add(-time.Minute),
			RollingAvgCostUSD: 120,
		},
		{
			Timestamp:         baseTime,
			RollingAvgCostUSD: 125,
		},
	}
	ctx := CauseClassificationContext{
		Metrics: metrics,
		ControlLimits: []TrendControlLimit{
			{
				Metric: metricRollingAvgCostUSD,
				Latest: 125,
				Mean:   100,
				StdDev: 5,
				UCL:    115,
				LCL:    85,
			},
		},
		Anomalies: []TrendAnomaly{
			{
				Metric:   metricRollingAvgCostUSD,
				Latest:   125,
				UCL:      115,
				LCL:      85,
				Severity: anomalySeverityModerate,
			},
		},
	}

	recs := EvaluateCauseClassifications(ctx)
	rec := findClassificationRecord(t, recs, metricRollingAvgCostUSD, "")

	if rec.Class != CauseClassSpecial {
		t.Fatalf("class = %s, want %s", rec.Class, CauseClassSpecial)
	}
	if rec.PersistenceWindows != 2 {
		t.Fatalf("persistence windows = %d, want 2", rec.PersistenceWindows)
	}
	if rec.Latest != 125 {
		t.Fatalf("latest = %f, want 125", rec.Latest)
	}
	if rec.Drift != 25 {
		t.Fatalf("drift = %f, want 25", rec.Drift)
	}
	if rec.Severity != anomalySeverityModerate {
		t.Fatalf("severity = %s, want %s", rec.Severity, anomalySeverityModerate)
	}
	if rec.Limit == nil || rec.Limit.Metric != metricRollingAvgCostUSD {
		t.Fatalf("limit metric = %v, want %s", rec.Limit, metricRollingAvgCostUSD)
	}
	if !rec.DetectedAt.Equal(metrics[0].Timestamp) {
		t.Fatalf("detected at = %s, want %s", rec.DetectedAt, metrics[0].Timestamp)
	}
}

func TestEvaluateCauseClassifications_CommonCauseDrift(t *testing.T) {
	baseTime := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	metrics := []IterationMetric{
		{
			Timestamp:            baseTime.Add(-2 * time.Minute),
			RollingAvgDurationMs: 210,
		},
		{
			Timestamp:            baseTime.Add(-time.Minute),
			RollingAvgDurationMs: 215,
		},
		{
			Timestamp:            baseTime,
			RollingAvgDurationMs: 220,
		},
	}

	ctx := CauseClassificationContext{
		Metrics: metrics,
		ControlLimits: []TrendControlLimit{
			{
				Metric: metricRollingAvgDurationMs,
				Mean:   200,
				UCL:    230,
				LCL:    170,
			},
		},
	}

	recs := EvaluateCauseClassifications(ctx)
	rec := findClassificationRecord(t, recs, metricRollingAvgDurationMs, "")

	if rec.Class != CauseClassCommon {
		t.Fatalf("class = %s, want %s", rec.Class, CauseClassCommon)
	}
	if rec.PersistenceWindows != 3 {
		t.Fatalf("persistence windows = %d, want 3", rec.PersistenceWindows)
	}
	if rec.Drift != 20 {
		t.Fatalf("drift = %f, want 20", rec.Drift)
	}
	if rec.Severity != "" {
		t.Fatalf("expected empty severity for common cause, got %q", rec.Severity)
	}
	if rec.Limit != nil {
		t.Fatalf("unexpected limit for common cause: %+v", rec.Limit)
	}
	if !rec.DetectedAt.Equal(metrics[0].Timestamp) {
		t.Fatalf("detected at = %s, want %s", rec.DetectedAt, metrics[0].Timestamp)
	}
}

func TestEvaluateCauseClassifications_SpecialCauseFromPatternViolation(t *testing.T) {
	baseTime := time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)
	metrics := make([]IterationMetric, 10)
	for i := range metrics {
		metrics[i] = IterationMetric{
			Timestamp:              baseTime.Add(time.Duration(i) * time.Minute),
			RollingAvgValidationMs: 210 + float64(i),
			RollingAvgDurationMs:   0,
			RollingAvgCostUSD:      0,
			RollingAvgInputTokens:  0,
		}
	}

	ctx := CauseClassificationContext{
		Metrics: metrics,
		ControlLimits: []TrendControlLimit{
			{
				Metric: metricRollingAvgValidationMs,
				Mean:   200,
				UCL:    260,
				LCL:    170,
			},
		},
		PatternViolations: []PatternViolation{
			{
				Metric:    metricRollingAvgValidationMs,
				Rule:      nelsonRule2Name,
				RunLength: 9,
			},
		},
	}

	recs := EvaluateCauseClassifications(ctx)
	rec := findClassificationRecord(t, recs, metricRollingAvgValidationMs, "")

	if rec.Class != CauseClassSpecial {
		t.Fatalf("class = %s, want %s", rec.Class, CauseClassSpecial)
	}
	if rec.PersistenceWindows != 9 {
		t.Fatalf("persistence windows = %d, want 9", rec.PersistenceWindows)
	}
	expectedDetectedAt := metrics[len(metrics)-9].Timestamp
	if !rec.DetectedAt.Equal(expectedDetectedAt) {
		t.Fatalf("detected at = %s, want %s", rec.DetectedAt, expectedDetectedAt)
	}
	if rec.Limit == nil {
		t.Fatalf("expected limit for special cause pattern, got nil")
	}
	if rec.Severity != "" {
		t.Fatalf("expected empty severity for pattern-based special cause, got %q", rec.Severity)
	}
}

func TestEvaluateCauseClassifications_ProviderStratumSpecialCause(t *testing.T) {
	baseTime := time.Date(2026, time.March, 4, 0, 0, 0, 0, time.UTC)
	metrics := []IterationMetric{
		{
			Timestamp:            baseTime.Add(-time.Minute),
			RollingAvgCostUSD:    180,
			Provider:             "claude",
			RollingAvgDurationMs: 0,
		},
		{
			Timestamp:            baseTime,
			RollingAvgCostUSD:    195,
			Provider:             "claude",
			RollingAvgDurationMs: 0,
		},
	}

	ctx := CauseClassificationContext{
		Metrics: metrics,
		StratifiedControlLimits: map[string][]TrendControlLimit{
			"provider:claude": {
				{
					Metric: metricRollingAvgCostUSD,
					Mean:   150,
					UCL:    175,
					LCL:    120,
				},
			},
		},
		StratifiedAnomalies: map[string][]TrendAnomaly{
			"provider:claude": {
				{
					Metric:   metricRollingAvgCostUSD,
					Severity: anomalySeverityHigh,
				},
			},
		},
	}

	recs := EvaluateCauseClassifications(ctx)
	rec := findClassificationRecord(t, recs, metricRollingAvgCostUSD, "provider:claude")

	if rec.Class != CauseClassSpecial {
		t.Fatalf("class = %s, want %s", rec.Class, CauseClassSpecial)
	}
	if rec.Stratum != "provider:claude" {
		t.Fatalf("stratum = %s, want provider:claude", rec.Stratum)
	}
	if identity := rec.Identity(); identity != "rolling_avg_cost_usd|provider:claude|special_cause" {
		t.Fatalf("identity = %s, want %s", identity, "rolling_avg_cost_usd|provider:claude|special_cause")
	}
}

func findClassificationRecord(t *testing.T, records []CauseClassificationRecord, metric, stratum string) *CauseClassificationRecord {
	for _, rec := range records {
		if rec.Metric == metric && rec.Stratum == stratum {
			return &rec
		}
	}
	t.Fatalf("classification record not found for %s/%s", metric, stratum)
	return nil
}
