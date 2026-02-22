package runner

import (
	"strings"
	"testing"
	"time"
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
			want: "Run: iteration 2/10, 5m of 30m elapsed",
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
				"Run: iteration 2/10",
				"5m of 30m elapsed",
				"beads-xyz",
				"Fix bug Y",
				"Model:    haiku",
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
