package runner

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/runner/display"
)

func TestToDisplayRunStatus_convertsFields(t *testing.T) {
	t.Parallel()

	status := &Status{
		Running:                true,
		Iteration:              3,
		IterationTotal:         10,
		MaxIterations:          10,
		TimeBudgetMinutes:      30,
		BeadID:                 "beads-123",
		BeadTitle:              "Add feature",
		Model:                  "sonnet",
		ElapsedS:               120,
		AutonomyRate:           0.95,
		FirstPassSuccessRate:   0.88,
		MTTRProxyMs:            5000,
		EscalationRatesByClass: map[string]float64{"warning": 0.05},
		RecurrenceCounters:     map[string]int{"class-A": 2},
	}

	got := toDisplayRunStatus(status)
	if got == nil {
		t.Fatalf("toDisplayRunStatus() returned nil")
	}
	if _, ok := interface{}(got).(*display.RunStatus); !ok {
		t.Fatalf("toDisplayRunStatus() did not return *display.RunStatus")
	}
	if got.Running != true {
		t.Errorf("toDisplayRunStatus().Running = %v, want true", got.Running)
	}
	if got.Iteration != 3 {
		t.Errorf("toDisplayRunStatus().Iteration = %d, want 3", got.Iteration)
	}
	if got.BeadID != "beads-123" {
		t.Errorf("toDisplayRunStatus().BeadID = %q, want beads-123", got.BeadID)
	}
	if got.Model != "sonnet" {
		t.Errorf("toDisplayRunStatus().Model = %q, want sonnet", got.Model)
	}
}

func TestFormatItems_clampsNegativeMaxShow(t *testing.T) {
	t.Parallel()

	items := []string{"one", "two"}
	got := formatItems(items, -1)
	want := []string{"    (and 2 more)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("formatItems() = %v, want %v", got, want)
	}
}

func TestSPCMetricConstants_AreFromDisplayPackage(t *testing.T) {
	t.Parallel()

	// Verify that format.go uses the exported SPC metric constants from display package
	// rather than maintaining its own duplicates. This test ensures we reference the
	// display package constants throughout.
	tests := []struct {
		name                 string
		formatConstantValue  string
		displayConstantValue string
		displayConstantName  string
	}{
		{
			name:                 "rolling_success_rate",
			formatConstantValue:  spcMetricRollingSuccessRate,
			displayConstantValue: display.SPCMetricRollingSuccessRate,
			displayConstantName:  "SPCMetricRollingSuccessRate",
		},
		{
			name:                 "rolling_escalation_rate",
			formatConstantValue:  spcMetricRollingEscalateRate,
			displayConstantValue: display.SPCMetricRollingEscalateRate,
			displayConstantName:  "SPCMetricRollingEscalateRate",
		},
		{
			name:                 "rolling_quality_score",
			formatConstantValue:  spcMetricRollingQualityScore,
			displayConstantValue: display.SPCMetricRollingQualityScore,
			displayConstantName:  "SPCMetricRollingQualityScore",
		},
		{
			name:                 "rolling_avg_duration_ms",
			formatConstantValue:  spcMetricRollingAvgDurationMs,
			displayConstantValue: display.SPCMetricRollingAvgDurationMs,
			displayConstantName:  "SPCMetricRollingAvgDurationMs",
		},
		{
			name:                 "rolling_first_pass_success_rate",
			formatConstantValue:  spcMetricFirstPassSuccessRate,
			displayConstantValue: display.SPCMetricFirstPassSuccessRate,
			displayConstantName:  "SPCMetricFirstPassSuccessRate",
		},
		{
			name:                 "rolling_avg_cost_usd",
			formatConstantValue:  spcMetricRollingAvgCostUSD,
			displayConstantValue: display.SPCMetricRollingAvgCostUSD,
			displayConstantName:  "SPCMetricRollingAvgCostUSD",
		},
		{
			name:                 "ewma_success_rate",
			formatConstantValue:  spcMetricEWMASuccessRate,
			displayConstantValue: display.SPCMetricEWMASuccessRate,
			displayConstantName:  "SPCMetricEWMASuccessRate",
		},
		{
			name:                 "ewma_cost_usd",
			formatConstantValue:  spcMetricEWMACostUSD,
			displayConstantValue: display.SPCMetricEWMACostUSD,
			displayConstantName:  "SPCMetricEWMACostUSD",
		},
		{
			name:                 "ewma_duration_ms",
			formatConstantValue:  spcMetricEWMADurationMs,
			displayConstantValue: display.SPCMetricEWMADurationMs,
			displayConstantName:  "SPCMetricEWMADurationMs",
		},
		{
			name:                 "ewma_input_tokens",
			formatConstantValue:  spcMetricEWMAInputTokens,
			displayConstantValue: display.SPCMetricEWMAInputTokens,
			displayConstantName:  "SPCMetricEWMAInputTokens",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.formatConstantValue != tt.displayConstantValue {
				t.Errorf("format.%s (%q) != display.%s (%q)",
					tt.name, tt.formatConstantValue,
					tt.displayConstantName, tt.displayConstantValue)
			}
		})
	}
}
