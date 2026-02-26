package display

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{name: "zero", duration: 0, want: "0s"},
		{name: "seconds", duration: 30 * time.Second, want: "30s"},
		{name: "sub-second rounds up", duration: 500 * time.Millisecond, want: "1s"},
		{name: "sub-second rounds down", duration: 400 * time.Millisecond, want: "0s"},
		{name: "exactly one minute", duration: time.Minute, want: "1m"},
		{name: "minutes only", duration: 5 * time.Minute, want: "5m"},
		{name: "hours and minutes", duration: 2*time.Hour + 15*time.Minute, want: "2h 15m"},
		{name: "hours with zero minutes", duration: 1 * time.Hour, want: "1h 0m"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Fatalf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestFormatRunningLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status *RunStatus
		want   string
	}{
		{
			name: "basic iteration no limits",
			status: &RunStatus{
				Running:   true,
				Iteration: 3,
				ElapsedS:  120,
			},
			want: "Run: iteration 3, 2m elapsed",
		},
		{
			name: "with max iterations and time budget",
			status: &RunStatus{
				Running:           true,
				Iteration:         2,
				MaxIterations:     10,
				TimeBudgetMinutes: 30,
				ElapsedS:          300,
			},
			want: "Run: iteration 2 of 10, 5m of 30m elapsed",
		},
		{
			name: "with explicit iteration total",
			status: &RunStatus{
				Running:        true,
				Iteration:      3,
				IterationTotal: 16,
				ElapsedS:       180,
			},
			want: "Run: iteration 3 of 16, 3m elapsed",
		},
		{
			name: "iteration 1 no limits short elapsed",
			status: &RunStatus{
				Running:   true,
				Iteration: 1,
				ElapsedS:  45,
			},
			want: "Run: iteration 1, 45s elapsed",
		},
		{
			name: "time budget over 60 minutes shown as raw minutes",
			status: &RunStatus{
				Running:           true,
				Iteration:         3,
				TimeBudgetMinutes: 90,
				ElapsedS:          600,
			},
			want: "Run: iteration 3, 10m of 90m elapsed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatRunningLine(tt.status)
			if got != tt.want {
				t.Fatalf("FormatRunningLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatEscalationBreakdown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		rates map[string]float64
		want  string
	}{
		{name: "nil map returns empty", rates: nil, want: ""},
		{name: "empty map returns empty", rates: map[string]float64{}, want: ""},
		{name: "single class", rates: map[string]float64{"timeout": 0.25}, want: "timeout 25%"},
		{
			name: "multiple classes sorted alphabetically",
			rates: map[string]float64{
				"timeout":    0.50,
				"lint":       0.125,
				"test-flake": 0.375,
			},
			want: "lint 13% | test-flake 38% | timeout 50%",
		},
		{name: "rounds half up", rates: map[string]float64{"compile": 0.005}, want: "compile 1%"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatEscalationBreakdown(tt.rates)
			if got != tt.want {
				t.Fatalf("FormatEscalationBreakdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatRecurrenceBreakdown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		counters map[string]int
		want     string
	}{
		{name: "nil map returns empty", counters: nil, want: ""},
		{name: "empty map returns empty", counters: map[string]int{}, want: ""},
		{name: "single class", counters: map[string]int{"timeout": 2}, want: "timeout x2"},
		{
			name: "multiple classes sorted alphabetically",
			counters: map[string]int{
				"timeout": 5,
				"lint":    1,
				"scope":   3,
			},
			want: "lint x1 | scope x3 | timeout x5",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatRecurrenceBreakdown(tt.counters)
			if got != tt.want {
				t.Fatalf("FormatRecurrenceBreakdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatRun_runnerCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status *RunStatus
		want   []string
	}{
		{
			name:   "nil status",
			status: nil,
			want:   []string{"Run: not running"},
		},
		{
			name: "running with iteration and model",
			status: &RunStatus{
				Running:   true,
				Iteration: 3,
				BeadID:    "beads-abc",
				BeadTitle: "Add feature X",
				Model:     "sonnet",
				ElapsedS:  120,
			},
			want: []string{
				"Run: iteration 3",
				"2m elapsed",
				"beads-abc",
				"Add feature X",
				"Model:    sonnet",
			},
		},
		{
			name: "running with max iterations and time budget",
			status: &RunStatus{
				Running:           true,
				Iteration:         2,
				MaxIterations:     10,
				TimeBudgetMinutes: 30,
				BeadID:            "beads-xyz",
				BeadTitle:         "Fix bug Y",
				Model:             "haiku",
				ElapsedS:          300,
			},
			want: []string{
				"Run: iteration 2 of 10",
				"5m of 30m elapsed",
				"beads-xyz",
				"Fix bug Y",
				"Model:    haiku",
			},
		},
		{
			name: "running with escalation recurrence and reliability",
			status: &RunStatus{
				Running:              true,
				Iteration:            5,
				BeadID:               "beads-qrs",
				BeadTitle:            "Improve display",
				Model:                "sonnet",
				ElapsedS:             600,
				AutonomyRate:         0.85,
				FirstPassSuccessRate: 0.70,
				MTTRProxyMs:          30000,
				EscalationRatesByClass: map[string]float64{
					"lint":    0.125,
					"timeout": 0.50,
				},
				RecurrenceCounters: map[string]int{
					"lint":  1,
					"scope": 3,
				},
			},
			want: []string{
				"Run: iteration 5",
				"Reliability: autonomy 85% | first-pass 70% | MTTR 30s",
				"Escalation: lint 13% | timeout 50%",
				"Recurrence: lint x1 | scope x3",
			},
		},
		{
			name: "not running with last run info",
			status: &RunStatus{
				Running:   false,
				Iteration: 5,
			},
			want: []string{
				"Run: not running",
				"Last run:",
				"5 iterations completed",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatRun(tt.status)
			for _, substr := range tt.want {
				if !strings.Contains(got, substr) {
					t.Fatalf("FormatRun() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}

func TestFormatHealth_variations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		lastRetro             time.Time
		iterationsSinceReview int
		wantSubstrings        []string
	}{
		{
			name:                  "never had retro or review",
			lastRetro:             time.Time{},
			iterationsSinceReview: 0,
			wantSubstrings: []string{
				"Health:",
				"Last retro:  never",
				"Last review: never",
			},
		},
		{
			name:                  "retro done review never",
			lastRetro:             time.Now().Add(-2 * time.Hour),
			iterationsSinceReview: 0,
			wantSubstrings: []string{
				"Health:",
				"Last retro:  2h 0m ago",
				"Last review: never",
			},
		},
		{
			name:                  "retro never review done singular",
			lastRetro:             time.Time{},
			iterationsSinceReview: 1,
			wantSubstrings: []string{
				"Health:",
				"Last retro:  never",
				"Last review: 1 iteration ago",
			},
		},
		{
			name:                  "both done plural iterations",
			lastRetro:             time.Now().Add(-30 * time.Minute),
			iterationsSinceReview: 5,
			wantSubstrings: []string{
				"Health:",
				"Last retro:  30m ago",
				"Last review: 5 iterations ago",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatHealth(tt.lastRetro, tt.iterationsSinceReview)
			for _, substr := range tt.wantSubstrings {
				if !strings.Contains(got, substr) {
					t.Fatalf("FormatHealth() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}

func TestFormatPipeline_basic(t *testing.T) {
	t.Parallel()

	got := FormatPipeline(&pipeline.PipelineStatus{})
	if !strings.Contains(got, "Pipeline:") {
		t.Fatalf("FormatPipeline() = %q, want substring %q", got, "Pipeline:")
	}
}

func TestFormatRecommendation_commands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rec  string
		want string
	}{
		{name: "empty recommendation", rec: "", want: ""},
		{name: "refine recommendation", rec: "Refine backlog ideas", want: "Next action: Refine backlog ideas (gromit refine)"},
		{name: "plan recommendation", rec: "Plan spec \"user-profiles\"", want: "Next action: Plan spec \"user-profiles\" (gromit plan)"},
		{name: "decompose recommendation", rec: "Decompose plan \"status-json-staleness\"", want: "Next action: Decompose plan \"status-json-staleness\" (gromit decompose)"},
		{name: "run recommendation", rec: "Run 4 ready bead(s)", want: "Next action: Run 4 ready bead(s) (gromit run)"},
		{name: "no matching command hint", rec: "No work in pipeline", want: "Next action: No work in pipeline"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatRecommendation(tt.rec)
			if got != tt.want {
				t.Fatalf("FormatRecommendation(%q) = %q, want %q", tt.rec, got, tt.want)
			}
		})
	}
}

func TestFormatModelPerformance_singleModel(t *testing.T) {
	t.Parallel()

	stats := map[string]logger.ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   10,
			Successes:    9,
			Failures:     1,
			TotalCostUSD: 20.40,
		},
	}
	got := FormatModelPerformance(stats)
	for _, substr := range []string{"Model Performance", "opus", "90%", "(9/10)", "$2.04/iter"} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatModelPerformance() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatModelPerformance_multipleModels(t *testing.T) {
	t.Parallel()

	stats := map[string]logger.ModelStats{
		"opus":   {Model: "opus", Iterations: 11, Successes: 10, Failures: 1, TotalCostUSD: 22.44},
		"sonnet": {Model: "sonnet", Iterations: 8, Successes: 3, Failures: 5, TotalCostUSD: 3.68},
		"haiku":  {Model: "haiku", Iterations: 8, Successes: 6, Failures: 2, TotalCostUSD: 0.96},
	}
	got := FormatModelPerformance(stats)
	for _, substr := range []string{
		"opus", "91%", "(10/11)", "$2.04/iter",
		"sonnet", "38%", "(3/8)", "$0.46/iter",
		"haiku", "75%", "(6/8)", "$0.12/iter",
	} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatModelPerformance() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatCompatibility_defaultContext(t *testing.T) {
	t.Parallel()

	ctx := config.CompatibilityContext{
		Profile:            config.CompatibilityResolvedValue{Value: "go", Source: "config"},
		TrackerBackend:     config.CompatibilityResolvedValue{Value: "jira", Source: "env"},
		MethodologyAdapter: config.CompatibilityResolvedValue{Value: "go", Source: "default"},
	}
	got := FormatCompatibility(ctx)
	if !strings.Contains(got, "Compatibility:") {
		t.Fatalf("FormatCompatibility() = %q, want substring %q", got, "Compatibility:")
	}
}

func TestSPCMetricConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{name: "success rate", constant: spcMetricRollingSuccessRate, want: "rolling_success_rate"},
		{name: "escalation rate", constant: spcMetricRollingEscalateRate, want: "rolling_escalation_rate"},
		{name: "quality score", constant: spcMetricRollingQualityScore, want: "rolling_quality_score"},
		{name: "avg duration", constant: spcMetricRollingAvgDurationMs, want: "rolling_avg_duration_ms"},
		{name: "first pass", constant: spcMetricFirstPassSuccessRate, want: "rolling_first_pass_success_rate"},
		{name: "avg cost", constant: spcMetricRollingAvgCostUSD, want: "rolling_avg_cost_usd"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.constant != tt.want {
				t.Fatalf("SPC metric constant = %q, want %q", tt.constant, tt.want)
			}
		})
	}
}

func TestFormatSPCSummary_IncludesProviderMetrics(t *testing.T) {
	t.Parallel()
	trend := &logger.ProcessTrend{
		TotalIterations: 3,
		WindowSize:      1,
		ProviderMetrics: []logger.ProviderMetrics{
			{
				Name:                 "openai",
				TotalInvocations:     2,
				Successes:            1,
				SuccessRate:          0.5,
				TransportFailures:    1,
				TransportFailureRate: 0.5,
				FallbacksTriggered:   1,
				AvgDurationMs:        2000,
				TotalCostUSD:         3.14,
			},
		},
		ControlLimits: []logger.TrendControlLimit{
			{Metric: spcMetricRollingSuccessRate, Latest: 0.5, LCL: 0.2, UCL: 0.8},
		},
	}

	got := FormatSPCSummary(trend)
	for _, substr := range []string{
		"Provider metrics:",
		"openai:",
		"2 invocations",
		"50% success",
		"$3.14 total cost",
	} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatSPCSummary() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatSPCSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		trend          *logger.ProcessTrend
		wantSubstrings []string
	}{
		{
			name:           "nil trend returns no-data",
			trend:          nil,
			wantSubstrings: []string{"SPC: (no data)"},
		},
		{
			name:           "zero iterations returns no-data",
			trend:          &logger.ProcessTrend{TotalIterations: 0},
			wantSubstrings: []string{"SPC: (no data)"},
		},
		{
			name: "basic trend with control limits and no anomalies",
			trend: &logger.ProcessTrend{
				TotalIterations: 50,
				WindowSize:      30,
				ControlLimits: []logger.TrendControlLimit{
					{Metric: spcMetricRollingSuccessRate, Latest: 0.85, Mean: 0.80, StdDev: 0.05, UCL: 0.95, LCL: 0.65},
					{Metric: spcMetricRollingEscalateRate, Latest: 0.10, Mean: 0.15, StdDev: 0.03, UCL: 0.24, LCL: 0.06},
					{Metric: spcMetricRollingQualityScore, Latest: 0.90, Mean: 0.85, StdDev: 0.04, UCL: 0.97, LCL: 0.73},
					{Metric: spcMetricRollingAvgDurationMs, Latest: 45000, Mean: 50000, StdDev: 5000, UCL: 65000, LCL: 35000},
				},
				Anomalies: []logger.TrendAnomaly{},
			},
			wantSubstrings: []string{
				"SPC:",
				"Window:   30 iterations (50 total)",
				"Success:", "85%", "limits 65%..95%",
				"Escalate:", "10%", "limits 6%..24%",
				"Quality:", "90%", "limits 73%..97%",
				"Duration:", "limits",
				"Anomaly:  none",
			},
		},
		{
			name: "trend with anomalies shows count and first anomaly details",
			trend: &logger.ProcessTrend{
				TotalIterations: 40,
				WindowSize:      30,
				ControlLimits: []logger.TrendControlLimit{
					{Metric: spcMetricRollingSuccessRate, Latest: 0.50, Mean: 0.80, StdDev: 0.05, UCL: 0.95, LCL: 0.65},
				},
				Anomalies: []logger.TrendAnomaly{
					{Metric: spcMetricRollingSuccessRate, Latest: 0.50, UCL: 0.95, LCL: 0.65, Severity: "high"},
					{Metric: spcMetricRollingEscalateRate, Latest: 0.40, UCL: 0.24, LCL: 0.06, Severity: "moderate"},
				},
			},
			wantSubstrings: []string{
				"SPC:",
				"Window:   30 iterations (40 total)",
				"Anomaly:  2",
				"success",
				"high",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatSPCSummary(tt.trend)
			for _, substr := range tt.wantSubstrings {
				if !strings.Contains(got, substr) {
					t.Fatalf("FormatSPCSummary() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}

func TestFormatSPCSummary_IncludesEWMAValues(t *testing.T) {
	t.Parallel()
	trend := &logger.ProcessTrend{
		TotalIterations: 10,
		WindowSize:      5,
		ControlLimits: []logger.TrendControlLimit{
			{Metric: spcMetricRollingSuccessRate, Latest: 0.5, LCL: 0.2, UCL: 0.8},
		},
		EWMAAnomalies: []logger.TrendAnomaly{
			{
				Metric: spcMetricEWMASuccessRate,
				Latest: 0.7,
				LCL:    0.5,
				UCL:    0.9,
			},
			{
				Metric: spcMetricEWMADurationMs,
				Latest: 70000,
				LCL:    60000,
				UCL:    120000,
			},
		},
	}

	got := FormatSPCSummary(trend)
	for _, substr := range []string{
		"EWMA values:",
		"EWMA success rate",
		"70%",
		"limits 50%..90%",
		"EWMA duration: 1m",
		"limits 1m..2m",
	} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatSPCSummary() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatSPCSummary_IncludesLeadingIndicatorsAndEconomicMetrics(t *testing.T) {
	t.Parallel()
	trend := &logger.ProcessTrend{
		TotalIterations: 5,
		WindowSize:      5,
		LatestWindow: logger.ProcessTrendWindow{
			FirstPassSuccess:  0.75,
			ReworkRate:        0.22,
			EscalationRate:    0.05,
			AvgInputTokens:    1234.56,
			AvgCostUSD:        4.56,
			AvgCostPerBeadUSD: 2.34,
			AvgDurationMs:     62000,
		},
		ControlLimits: []logger.TrendControlLimit{
			{Metric: spcMetricRollingSuccessRate, Latest: 0.85, LCL: 0.6, UCL: 1.0},
		},
	}

	got := FormatSPCSummary(trend)
	for _, substr := range []string{
		"Leading indicators:",
		"first-pass success 75%",
		"rework 22%",
		"escalation 5%",
		"input 1235 tokens",
		"Economic metrics:",
		"Avg $4.56",
		"cost per bead $2.34",
		"avg duration 1m",
	} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatSPCSummary() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatSPCSummary_SortsMetricsDeterministically(t *testing.T) {
	t.Parallel()
	trend := &logger.ProcessTrend{
		TotalIterations: 4,
		WindowSize:      3,
		ControlLimits: []logger.TrendControlLimit{
			{Metric: spcMetricRollingSuccessRate, Latest: 0.8, UCL: 0.97, LCL: 0.65},
			{Metric: spcMetricRollingAvgDurationMs, Latest: 45000, UCL: 65000, LCL: 35000},
			{Metric: spcMetricRollingQualityScore, Latest: 0.9, UCL: 1.0, LCL: 0.7},
			{Metric: spcMetricRollingEscalateRate, Latest: 0.12, UCL: 0.3, LCL: 0.06},
		},
	}

	got := FormatSPCSummary(trend)
	var metricLines []string
	for _, line := range strings.Split(got, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.Contains(trimmed, "limits") {
			continue
		}
		metricLines = append(metricLines, trimmed)
	}

	if len(metricLines) != 4 {
		t.Fatalf("expected 4 metric lines, got %d: %v", len(metricLines), metricLines)
	}

	var labels []string
	for _, line := range metricLines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			t.Fatalf("metric line missing label: %q", line)
		}
		labels = append(labels, fields[0])
	}

	want := []string{"Duration:", "Escalate:", "Quality:", "Success:"}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("metric order = %v, want %v", labels, want)
		}
	}
}

func TestFormatSPCSummary_DisplaysUCLAndLCLInControlLimits(t *testing.T) {
	t.Parallel()
	trend := &logger.ProcessTrend{
		TotalIterations: 20,
		WindowSize:      15,
		ControlLimits: []logger.TrendControlLimit{
			{
				Metric: spcMetricRollingSuccessRate,
				Latest: 0.82,
				Mean:   0.80,
				StdDev: 0.06,
				UCL:    0.98,
				LCL:    0.62,
			},
			{
				Metric: spcMetricRollingAvgDurationMs,
				Latest: 45000,
				Mean:   50000,
				StdDev: 5000,
				UCL:    70000,
				LCL:    30000,
			},
		},
	}

	got := FormatSPCSummary(trend)
	for _, substr := range []string{
		"Success:",
		"82%",
		"limits 62%..98%",
		"Duration:",
		"limits",
	} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatSPCSummary() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatSPCSummary_IncludesNelsonRuleViolations(t *testing.T) {
	t.Parallel()
	trend := &logger.ProcessTrend{
		TotalIterations: 12,
		WindowSize:      10,
		ControlLimits: []logger.TrendControlLimit{
			{Metric: spcMetricRollingSuccessRate, Latest: 0.8, LCL: 0.5, UCL: 0.95},
		},
		PatternViolations: []logger.PatternViolation{
			{
				Metric:    "rolling_success_rate",
				Rule:      "nelson_rule_2",
				Direction: "below",
				RunLength: 9,
				Message:   "9 points below center line",
			},
		},
	}

	got := FormatSPCSummary(trend)
	for _, substr := range []string{
		"Nelson rule violations:",
		"success",
		"nelson_rule_2",
		"9 points below center line",
	} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatSPCSummary() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatSPCSummary_DisplaysMultipleNelsonRuleViolations(t *testing.T) {
	t.Parallel()
	trend := &logger.ProcessTrend{
		TotalIterations: 15,
		WindowSize:      10,
		ControlLimits: []logger.TrendControlLimit{
			{Metric: spcMetricRollingSuccessRate, Latest: 0.7, LCL: 0.5, UCL: 0.95},
			{Metric: spcMetricRollingEscalateRate, Latest: 0.2, LCL: 0.05, UCL: 0.3},
		},
		PatternViolations: []logger.PatternViolation{
			{
				Metric:    "rolling_success_rate",
				Rule:      "nelson_rule_2",
				Direction: "below",
				RunLength: 9,
				Message:   "9 points below center line",
			},
			{
				Metric:    "rolling_escalation_rate",
				Rule:      "nelson_rule_8",
				Direction: "above",
				RunLength: 4,
				Message:   "4 points above 1 std dev",
			},
		},
	}

	got := FormatSPCSummary(trend)
	for _, substr := range []string{
		"Nelson rule violations:",
		"success",
		"nelson_rule_2",
		"9 points below center line",
		"escalation",
		"nelson_rule_8",
		"4 points above 1 std dev",
	} {
		if !strings.Contains(got, substr) {
			t.Fatalf("FormatSPCSummary() = %q, want substring %q", got, substr)
		}
	}
}

func TestFormatSPCSummary_OmitsNelsonSectionWhenNoViolations(t *testing.T) {
	t.Parallel()
	trend := &logger.ProcessTrend{
		TotalIterations: 10,
		WindowSize:      10,
		ControlLimits: []logger.TrendControlLimit{
			{Metric: spcMetricRollingSuccessRate, Latest: 0.85, LCL: 0.65, UCL: 0.95},
		},
		PatternViolations: []logger.PatternViolation{},
	}

	got := FormatSPCSummary(trend)
	if strings.Contains(got, "Nelson rule violations:") {
		t.Fatalf("FormatSPCSummary() should not include 'Nelson rule violations:' when empty, got:\n%s", got)
	}
}

func TestFormatSPCValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		v         float64
		asPercent bool
		want      string
	}{
		{name: "percent 85%", v: 0.85, asPercent: true, want: "85%"},
		{name: "percent 0%", v: 0.0, asPercent: true, want: "0%"},
		{name: "percent 100%", v: 1.0, asPercent: true, want: "100%"},
		{name: "percent rounds up at midpoint", v: 0.855, asPercent: true, want: "86%"},
		{name: "percent clamps negative to 0", v: -0.1, asPercent: true, want: "0%"},
		{name: "percent clamps above 1 to 100", v: 1.5, asPercent: true, want: "100%"},
		{name: "duration 45s", v: 45000, asPercent: false, want: "45s"},
		{name: "duration 0s", v: 0, asPercent: false, want: "0s"},
		{name: "duration negative returns 0s", v: -1000, asPercent: false, want: "0s"},
		{name: "duration 2m", v: 120000, asPercent: false, want: "2m"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatSPCValue(tt.v, tt.asPercent)
			if got != tt.want {
				t.Fatalf("FormatSPCValue(%v, %v) = %q, want %q", tt.v, tt.asPercent, got, tt.want)
			}
		})
	}
}

func TestSimplifySPCMetric(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		metric string
		want   string
	}{
		{name: "success rate", metric: spcMetricRollingSuccessRate, want: "success"},
		{name: "first pass success", metric: spcMetricFirstPassSuccessRate, want: "first-pass"},
		{name: "escalation rate", metric: spcMetricRollingEscalateRate, want: "escalation"},
		{name: "quality score", metric: spcMetricRollingQualityScore, want: "quality"},
		{name: "avg duration", metric: spcMetricRollingAvgDurationMs, want: "duration"},
		{name: "avg cost", metric: spcMetricRollingAvgCostUSD, want: "cost"},
		{name: "ewma success rate", metric: spcMetricEWMASuccessRate, want: "EWMA success rate"},
		{name: "ewma duration", metric: spcMetricEWMADurationMs, want: "EWMA duration"},
		{name: "ewma cost", metric: spcMetricEWMACostUSD, want: "EWMA cost"},
		{name: "ewma input tokens", metric: spcMetricEWMAInputTokens, want: "EWMA input tokens"},
		{name: "unknown metric returns as-is", metric: "some_custom_metric", want: "some_custom_metric"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SimplifySPCMetric(tt.metric)
			if got != tt.want {
				t.Fatalf("SimplifySPCMetric(%q) = %q, want %q", tt.metric, got, tt.want)
			}
		})
	}
}

func TestFormatSPCLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		label      string
		cl         logger.TrendControlLimit
		isDuration bool
		want       string
	}{
		{
			name:       "percentage metric shows percent values and limits",
			label:      "Success:",
			cl:         logger.TrendControlLimit{Latest: 0.85, LCL: 0.65, UCL: 0.95},
			isDuration: false,
			want:       "  Success:   85%, limits 65%..95%",
		},
		{
			name:       "duration metric uses human-friendly format",
			label:      "Duration:",
			cl:         logger.TrendControlLimit{Latest: 45000, LCL: 30000, UCL: 55000},
			isDuration: true,
			want:       "  Duration:  45s, limits 30s..55s",
		},
		{
			name:       "duration metric with minute-scale values",
			label:      "Duration:",
			cl:         logger.TrendControlLimit{Latest: 120000, LCL: 60000, UCL: 180000},
			isDuration: true,
			want:       "  Duration:  2m, limits 1m..3m",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatSPCLine(tt.label, tt.cl, tt.isDuration)
			if got != tt.want {
				t.Fatalf("FormatSPCLine(%q, cl, %v) = %q, want %q", tt.label, tt.isDuration, got, tt.want)
			}
		})
	}
}
