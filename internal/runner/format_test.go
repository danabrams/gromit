package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
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

func TestFormatRunWrapper_DelegatesToDisplay(t *testing.T) {
	t.Parallel()

	status := &Status{
		Running:   true,
		Iteration: 2,
		BeadID:    "beads-xyz",
		BeadTitle: "Test bead",
		ElapsedS:  60,
	}

	got := formatRun(status)
	want := display.FormatRun(toDisplayRunStatus(status))
	if got != want {
		t.Errorf("formatRun() = %q, want %q", got, want)
	}
}

func TestFormatPipelineWrapper_DelegatesToDisplay(t *testing.T) {
	t.Parallel()

	ps := &pipeline.PipelineStatus{}
	got := formatPipeline(ps)
	want := display.FormatPipeline(ps)
	if got != want {
		t.Errorf("formatPipeline() = %q, want %q", got, want)
	}
}

func TestFormatSPCSummaryWrapper_DelegatesToDisplay(t *testing.T) {
	t.Parallel()

	trend := (*logger.ProcessTrend)(nil)
	got := formatSPCSummary(trend)
	want := display.FormatSPCSummary(trend)
	if got != want {
		t.Errorf("formatSPCSummary() = %q, want %q", got, want)
	}
}
