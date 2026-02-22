package runner

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/logger"
)

func TestFormatDuration(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestFormatRunningLine(t *testing.T) {
	tests := []struct {
		name   string
		status *Status
		want   string
	}{
		{
			name: "basic iteration no limits",
			status: &Status{
				Running:   true,
				Iteration: 3,
				ElapsedS:  120,
			},
			want: "Run: iteration 3, 2m elapsed",
		},
		{
			name: "with max iterations and time budget",
			status: &Status{
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
			status: &Status{
				Running:        true,
				Iteration:      3,
				IterationTotal: 16,
				ElapsedS:       180,
			},
			want: "Run: iteration 3 of 16, 3m elapsed",
		},
		{
			name: "iteration 1 no limits short elapsed",
			status: &Status{
				Running:   true,
				Iteration: 1,
				ElapsedS:  45,
			},
			want: "Run: iteration 1, 45s elapsed",
		},
		{
			name: "time budget over 60 minutes shown as raw minutes",
			status: &Status{
				Running:           true,
				Iteration:         3,
				TimeBudgetMinutes: 90,
				ElapsedS:          600,
			},
			want: "Run: iteration 3, 10m of 90m elapsed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRunningLine(tt.status)
			if got != tt.want {
				t.Errorf("formatRunningLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatEscalationBreakdown(t *testing.T) {
	tests := []struct {
		name  string
		rates map[string]float64
		want  string
	}{
		{
			name:  "nil map returns empty",
			rates: nil,
			want:  "",
		},
		{
			name:  "empty map returns empty",
			rates: map[string]float64{},
			want:  "",
		},
		{
			name:  "single class",
			rates: map[string]float64{"timeout": 0.25},
			want:  "timeout 25%",
		},
		{
			name: "multiple classes sorted alphabetically",
			rates: map[string]float64{
				"timeout":    0.50,
				"lint":       0.125,
				"test-flake": 0.375,
			},
			want: "lint 13% | test-flake 38% | timeout 50%",
		},
		{
			name:  "rounds half up",
			rates: map[string]float64{"compile": 0.005},
			want:  "compile 1%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEscalationBreakdown(tt.rates)
			if got != tt.want {
				t.Errorf("formatEscalationBreakdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatRecurrenceBreakdown(t *testing.T) {
	tests := []struct {
		name     string
		counters map[string]int
		want     string
	}{
		{
			name:     "nil map returns empty",
			counters: nil,
			want:     "",
		},
		{
			name:     "empty map returns empty",
			counters: map[string]int{},
			want:     "",
		},
		{
			name:     "single class",
			counters: map[string]int{"timeout": 2},
			want:     "timeout x2",
		},
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
		t.Run(tt.name, func(t *testing.T) {
			got := formatRecurrenceBreakdown(tt.counters)
			if got != tt.want {
				t.Errorf("formatRecurrenceBreakdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatHealth(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			got := formatHealth(tt.lastRetro, tt.iterationsSinceReview)
			for _, substr := range tt.wantSubstrings {
				if !strings.Contains(got, substr) {
					t.Errorf("formatHealth() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}

func TestFormatRecommendation(t *testing.T) {
	tests := []struct {
		name string
		rec  string
		want string
	}{
		{
			name: "empty recommendation",
			rec:  "",
			want: "",
		},
		{
			name: "refine recommendation",
			rec:  "Refine backlog ideas",
			want: "Next action: Refine backlog ideas (gromit refine)",
		},
		{
			name: "plan recommendation",
			rec:  "Plan spec \"user-profiles\"",
			want: "Next action: Plan spec \"user-profiles\" (gromit plan)",
		},
		{
			name: "decompose recommendation",
			rec:  "Decompose plan \"status-json-staleness\"",
			want: "Next action: Decompose plan \"status-json-staleness\" (gromit decompose)",
		},
		{
			name: "run recommendation",
			rec:  "Run 4 ready bead(s)",
			want: "Next action: Run 4 ready bead(s) (gromit run)",
		},
		{
			name: "no matching command hint",
			rec:  "No work in pipeline",
			want: "Next action: No work in pipeline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRecommendation(tt.rec)
			if got != tt.want {
				t.Errorf("formatRecommendation(%q) = %q, want %q", tt.rec, got, tt.want)
			}
		})
	}
}

func TestFormatModelPerformance(t *testing.T) {
	tests := []struct {
		name           string
		stats          map[string]logger.ModelStats
		wantSubstrings []string
	}{
		{
			name:           "nil stats shows header",
			stats:          nil,
			wantSubstrings: []string{"Model Performance"},
		},
		{
			name:           "empty stats shows header",
			stats:          map[string]logger.ModelStats{},
			wantSubstrings: []string{"Model Performance"},
		},
		{
			name: "single model with success rate and cost",
			stats: map[string]logger.ModelStats{
				"opus": {
					Model: "opus", Iterations: 10, Successes: 9, Failures: 1, TotalCostUSD: 20.40,
				},
			},
			wantSubstrings: []string{"Model Performance", "opus", "90%", "(9/10)", "$2.04/iter"},
		},
		{
			name: "multiple models all present",
			stats: map[string]logger.ModelStats{
				"opus":   {Model: "opus", Iterations: 11, Successes: 10, Failures: 1, TotalCostUSD: 22.44},
				"sonnet": {Model: "sonnet", Iterations: 8, Successes: 3, Failures: 5, TotalCostUSD: 3.68},
				"haiku":  {Model: "haiku", Iterations: 8, Successes: 6, Failures: 2, TotalCostUSD: 0.96},
			},
			wantSubstrings: []string{
				"opus", "91%", "(10/11)", "$2.04/iter",
				"sonnet", "38%", "(3/8)", "$0.46/iter",
				"haiku", "75%", "(6/8)", "$0.12/iter",
			},
		},
		{
			name: "zero iterations no division by zero",
			stats: map[string]logger.ModelStats{
				"opus": {Model: "opus", Iterations: 0, Successes: 0, Failures: 0, TotalCostUSD: 0.0},
			},
			wantSubstrings: []string{"opus"},
		},
		{
			name: "perfect success rate",
			stats: map[string]logger.ModelStats{
				"opus": {Model: "opus", Iterations: 5, Successes: 5, Failures: 0, TotalCostUSD: 10.00},
			},
			wantSubstrings: []string{"100%", "(5/5)", "$2.00/iter"},
		},
		{
			name: "zero success rate",
			stats: map[string]logger.ModelStats{
				"sonnet": {Model: "sonnet", Iterations: 4, Successes: 0, Failures: 4, TotalCostUSD: 1.00},
			},
			wantSubstrings: []string{"0%", "(0/4)", "$0.25/iter"},
		},
		{
			name: "fractional cent cost rounds to two decimals",
			stats: map[string]logger.ModelStats{
				"opus": {Model: "opus", Iterations: 3, Successes: 3, Failures: 0, TotalCostUSD: 1.00},
			},
			wantSubstrings: []string{"$0.33/iter"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatModelPerformance(tt.stats)
			for _, substr := range tt.wantSubstrings {
				if !strings.Contains(got, substr) {
					t.Errorf("formatModelPerformance() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}

func TestSPCMetricConstants(t *testing.T) {
	// Verify runner-package SPC metric constants match the metric names
	// used in ProcessTrend control limits from the logger package.
	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{"success rate", spcMetricRollingSuccessRate, "rolling_success_rate"},
		{"escalation rate", spcMetricRollingEscalateRate, "rolling_escalation_rate"},
		{"quality score", spcMetricRollingQualityScore, "rolling_quality_score"},
		{"avg duration", spcMetricRollingAvgDurationMs, "rolling_avg_duration_ms"},
		{"first pass success", spcMetricFirstPassSuccessRate, "rolling_first_pass_success_rate"},
		{"avg cost", spcMetricRollingAvgCostUSD, "rolling_avg_cost_usd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("SPC metric constant = %q, want %q", tt.constant, tt.want)
			}
		})
	}
}

func TestFormatSPCSummary(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			got := formatSPCSummary(tt.trend)
			for _, substr := range tt.wantSubstrings {
				if !strings.Contains(got, substr) {
					t.Errorf("formatSPCSummary() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}

func TestFormatSPCValue(t *testing.T) {
	tests := []struct {
		name      string
		v         float64
		asPercent bool
		want      string
	}{
		// Percentage formatting
		{"percent 85%", 0.85, true, "85%"},
		{"percent 0%", 0.0, true, "0%"},
		{"percent 100%", 1.0, true, "100%"},
		{"percent rounds up at midpoint", 0.855, true, "86%"},
		{"percent clamps negative to 0", -0.1, true, "0%"},
		{"percent clamps above 1 to 100", 1.5, true, "100%"},

		// Duration formatting (milliseconds)
		{"duration 45s", 45000, false, "45s"},
		{"duration 0s", 0, false, "0s"},
		{"duration negative returns 0s", -1000, false, "0s"},
		{"duration 2m", 120000, false, "2m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSPCValue(tt.v, tt.asPercent)
			if got != tt.want {
				t.Errorf("formatSPCValue(%v, %v) = %q, want %q", tt.v, tt.asPercent, got, tt.want)
			}
		})
	}
}

func TestSimplifySPCMetric(t *testing.T) {
	tests := []struct {
		name   string
		metric string
		want   string
	}{
		{"success rate", spcMetricRollingSuccessRate, "success"},
		{"first pass success", spcMetricFirstPassSuccessRate, "first-pass"},
		{"escalation rate", spcMetricRollingEscalateRate, "escalation"},
		{"quality score", spcMetricRollingQualityScore, "quality"},
		{"avg duration", spcMetricRollingAvgDurationMs, "duration"},
		{"avg cost", spcMetricRollingAvgCostUSD, "cost"},
		{"unknown metric returns as-is", "some_custom_metric", "some_custom_metric"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := simplifySPCMetric(tt.metric)
			if got != tt.want {
				t.Errorf("simplifySPCMetric(%q) = %q, want %q", tt.metric, got, tt.want)
			}
		})
	}
}

func TestFormatSPCLine(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			got := formatSPCLine(tt.label, tt.cl, tt.isDuration)
			if got != tt.want {
				t.Errorf("formatSPCLine(%q, cl, %v) = %q, want %q", tt.label, tt.isDuration, got, tt.want)
			}
		})
	}
}

func TestFormatRun(t *testing.T) {
	tests := []struct {
		name   string
		status *Status
		want   []string // substrings that must appear in output
	}{
		{
			name:   "nil status",
			status: nil,
			want:   []string{"Run: not running"},
		},
		{
			name: "running with iteration and model",
			status: &Status{
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
			status: &Status{
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
			status: &Status{
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
			status: &Status{
				Running:   false,
				Iteration: 5,
				StartedAt: time.Now().Add(-10 * time.Minute),
			},
			want: []string{
				"Run: not running",
				"Last run:",
				"5 iterations completed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRun(tt.status)
			for _, substr := range tt.want {
				if !strings.Contains(got, substr) {
					t.Errorf("formatRun() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}
