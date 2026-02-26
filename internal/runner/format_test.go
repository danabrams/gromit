package runner

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/display"
)

func TestFormatRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status *Status
		want   []string
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatRun(tt.status)
			for _, substr := range tt.want {
				if !strings.Contains(got, substr) {
					t.Errorf("formatRun() = %q, want substring %q", got, substr)
				}
			}
		})
	}
}

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
