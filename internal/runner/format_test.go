package runner

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/pipeline"
)

func TestFormatPipeline(t *testing.T) {
	tests := []struct {
		name   string
		status *pipeline.PipelineStatus
		want   []string // Expected lines to be present
	}{
		{
			name:   "nil status",
			status: nil,
			want:   []string{"Pipeline: (status unavailable)"},
		},
		{
			name: "empty pipeline",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    0,
				UnrefinedIdeas:    []string{},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    0,
			},
			want: []string{
				"Pipeline:",
				"  Backlog:  0 unrefined ideas",
				"  Specs:    0 unplanned",
				"  Plans:    0 undecomposed",
				"  Beads:    0 ready",
			},
		},
		{
			name: "single item (singular form)",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    1,
				UnrefinedIdeas:    []string{"Add rate limiting"},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    1,
			},
			want: []string{
				"  Backlog:  1 unrefined idea",
				"    - Add rate limiting",
				"  Beads:    1 ready",
			},
		},
		{
			name: "multiple items with overflow",
			status: &pipeline.PipelineStatus{
				UnrefinedCount: 5,
				UnrefinedIdeas: []string{
					"Add rate limiting to API",
					"Support webhook notifications",
					"Improve error messages",
					"Add logging",
					"Fix bug in auth",
				},
				UnplannedSpecs:    []string{"user-profiles", "notifications", "logging", "auth-fix"},
				UndecomposedPlans: []string{"status-json-staleness"},
				ReadyBeadCount:    4,
			},
			want: []string{
				"  Backlog:  5 unrefined ideas",
				"    - Add rate limiting to API",
				"    - Support webhook notifications",
				"    - Improve error messages",
				"    (and 2 more)",
				"  Specs:    4 unplanned",
				"    - user-profiles",
				"    - notifications",
				"    - logging",
				"    (and 1 more)",
				"  Plans:    1 undecomposed",
				"    - status-json-staleness",
				"  Beads:    4 ready",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPipeline(tt.status)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("formatPipeline() missing expected line:\n  want: %q\n  got:\n%s", want, got)
				}
			}
		})
	}
}

func TestFormatRun(t *testing.T) {
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)
	thirtyMinAgo := now.Add(-30 * time.Minute)

	tests := []struct {
		name   string
		status *Status
		want   []string // Expected lines to be present
	}{
		{
			name:   "nil status",
			status: nil,
			want:   []string{"Run: not running"},
		},
		{
			name: "running without limits",
			status: &Status{
				Running:   true,
				Iteration: 68,
				BeadID:    "gromit-abc123",
				BeadTitle: "Create PROMPT_refactor.md template",
				Model:     "sonnet",
				StartedAt: now.Add(-210 * time.Minute), // 3h 30m ago
				ElapsedS:  12600,                       // 3h 30m in seconds
			},
			want: []string{
				"Run: iteration 68, 3h 30m elapsed",
				"  Current:  gromit-abc123 — \"Create PROMPT_refactor.md template\"",
				"  Model:    sonnet",
			},
		},
		{
			name: "running with limits",
			status: &Status{
				Running:           true,
				Iteration:         12,
				BeadID:            "gromit-ja5m",
				BeadTitle:         "Add validation tests",
				Model:             "sonnet",
				StartedAt:         now.Add(-18 * time.Minute),
				ElapsedS:          1080, // 18m
				MaxIterations:     50,
				TimeBudgetMinutes: 30,
			},
			want: []string{
				"Run: iteration 12/50, 18m of 30m elapsed",
				"  Current:  gromit-ja5m — \"Add validation tests\"",
				"  Model:    sonnet",
			},
		},
		{
			name: "not running with history",
			status: &Status{
				Running:   false,
				Iteration: 12,
				StartedAt: twoHoursAgo,
			},
			want: []string{
				"Run: not running",
				"Last run: 2h 0m ago, 12 iterations completed",
			},
		},
		{
			name: "not running with single iteration",
			status: &Status{
				Running:   false,
				Iteration: 1,
				StartedAt: thirtyMinAgo,
			},
			want: []string{
				"Run: not running",
				"Last run: 30m ago, 1 iteration completed",
			},
		},
		{
			name: "not running without history (never run)",
			status: &Status{
				Running:   false,
				Iteration: 0,
				StartedAt: time.Time{}, // Zero time
			},
			want: []string{
				"Run: not running",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRun(tt.status)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("formatRun() missing expected line:\n  want: %q\n  got:\n%s", want, got)
				}
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		dur  time.Duration
		want string
	}{
		{"seconds", 45 * time.Second, "45s"},
		{"one minute", 1 * time.Minute, "1m"},
		{"minutes", 18 * time.Minute, "18m"},
		{"hours and minutes", 3*time.Hour + 30*time.Minute, "3h 30m"},
		{"exact hours", 2 * time.Hour, "2h 0m"},
		{"sub-second rounds to nearest", 500 * time.Millisecond, "1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.dur)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"seconds ago", now.Add(-45 * time.Second), "45s ago"},
		{"one minute ago", now.Add(-1 * time.Minute), "1m ago"},
		{"minutes ago", now.Add(-18 * time.Minute), "18m ago"},
		{"hours and minutes ago", now.Add(-3*time.Hour - 30*time.Minute), "3h 30m ago"},
		{"exact hours ago", now.Add(-2 * time.Hour), "2h 0m ago"},
		{"days ago (shown as hours)", now.Add(-3 * 24 * time.Hour), "72h 0m ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimeAgo(tt.t)
			if got != tt.want {
				t.Errorf("formatTimeAgo(%v) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}
}

func TestFormatItems(t *testing.T) {
	tests := []struct {
		name    string
		items   []string
		maxShow int
		want    []string
	}{
		{
			name:    "empty list",
			items:   []string{},
			maxShow: 3,
			want:    nil,
		},
		{
			name:    "under limit",
			items:   []string{"item1", "item2"},
			maxShow: 3,
			want:    []string{"    - item1", "    - item2"},
		},
		{
			name:    "exactly at limit",
			items:   []string{"item1", "item2", "item3"},
			maxShow: 3,
			want:    []string{"    - item1", "    - item2", "    - item3"},
		},
		{
			name:    "over limit",
			items:   []string{"item1", "item2", "item3", "item4", "item5"},
			maxShow: 3,
			want:    []string{"    - item1", "    - item2", "    - item3", "    (and 2 more)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatItems(tt.items, tt.maxShow)
			if len(got) != len(tt.want) {
				t.Fatalf("formatItems() returned %d lines, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("formatItems() line %d = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestFormatHealth(t *testing.T) {
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	tests := []struct {
		name                  string
		lastRetro             time.Time
		iterationsSinceReview int
		want                  []string
	}{
		{
			name:                  "never had retro or review",
			lastRetro:             time.Time{},
			iterationsSinceReview: 0,
			want: []string{
				"Health:",
				"  Last retro:  never",
				"  Last review: never",
			},
		},
		{
			name:                  "retro done, review never",
			lastRetro:             twoHoursAgo,
			iterationsSinceReview: 0,
			want: []string{
				"Health:",
				"  Last retro:  2h 0m ago",
				"  Last review: never",
			},
		},
		{
			name:                  "retro never, review done",
			lastRetro:             time.Time{},
			iterationsSinceReview: 5,
			want: []string{
				"Health:",
				"  Last retro:  never",
				"  Last review: 5 iterations ago",
			},
		},
		{
			name:                  "both done, singular iteration",
			lastRetro:             twoHoursAgo,
			iterationsSinceReview: 1,
			want: []string{
				"Health:",
				"  Last retro:  2h 0m ago",
				"  Last review: 1 iteration ago",
			},
		},
		{
			name:                  "both done, multiple iterations",
			lastRetro:             twoHoursAgo,
			iterationsSinceReview: 12,
			want: []string{
				"Health:",
				"  Last retro:  2h 0m ago",
				"  Last review: 12 iterations ago",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHealth(tt.lastRetro, tt.iterationsSinceReview)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("formatHealth() missing expected line:\n  want: %q\n  got:\n%s", want, got)
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
			name: "refine with specific idea",
			rec:  "Refine idea: Add rate limiting to API",
			want: "Next action: Refine idea: Add rate limiting to API (gromit refine)",
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
			name: "no work recommendation",
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
