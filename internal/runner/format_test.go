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
				"  Beads:    none",
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
				"  Beads:    1 ready", // Single status shown
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
				ReadyBeads:        []string{},
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
				"  Beads:    4 ready", // Single status shown
			},
		},
		{
			name: "ready beads with data",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    0,
				UnrefinedIdeas:    []string{},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    4,
				ReadyBeads: []string{
					"gromit-abc1 — Implement feature X",
					"gromit-abc2 — Add validation tests",
					"gromit-abc3 — Fix authentication bug",
					"gromit-abc4 — Update documentation",
				},
			},
			want: []string{
				"  Beads:    4 ready", // Single status shown
				"    - gromit-abc1 — Implement feature X",
				"    - gromit-abc2 — Add validation tests",
				"    - gromit-abc3 — Fix authentication bug",
				"    (and 1 more)",
			},
		},
		{
			name: "ready beads under limit",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    0,
				UnrefinedIdeas:    []string{},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    2,
				ReadyBeads: []string{
					"gromit-xyz1 — Small fix",
					"gromit-xyz2 — Refactor helper",
				},
			},
			want: []string{
				"  Beads:    2 ready", // Single status shown
				"    - gromit-xyz1 — Small fix",
				"    - gromit-xyz2 — Refactor helper",
			},
		},
		{
			name: "multiple statuses without run info",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    0,
				UnrefinedIdeas:    []string{},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    14,
				InProgressCount:   2,
				BlockedCount:      5,
				DeferredCount:     0,
				ClosedCount:       543,
				HasRunInfo:        false,
				ReadyBeads:        []string{},
			},
			want: []string{
				"  Beads:    14 ready, 2 in-progress, 5 blocked, 543 closed",
			},
		},
		{
			name: "multiple statuses with this run info",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:     0,
				UnrefinedIdeas:     []string{},
				UnplannedSpecs:     []string{},
				UndecomposedPlans:  []string{},
				ReadyBeadCount:     14,
				InProgressCount:    2,
				BlockedCount:       5,
				DeferredCount:      3,
				ClosedCount:        543,
				ClosedThisRunCount: 23,
				HasRunInfo:         true,
				ReadyBeads:         []string{},
			},
			want: []string{
				"  Beads:    14 ready, 2 in-progress, 5 blocked, 3 deferred, 543 closed (23 this run)",
			},
		},
		{
			name: "sparse statuses - ready and closed only",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    0,
				UnrefinedIdeas:    []string{},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    14,
				InProgressCount:   0,
				BlockedCount:      0,
				DeferredCount:     0,
				ClosedCount:       543,
				HasRunInfo:        false,
				ReadyBeads:        []string{},
			},
			want: []string{
				"  Beads:    14 ready, 543 closed",
			},
		},
		{
			name: "display order - sparse statuses maintain lifecycle order",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    0,
				UnrefinedIdeas:    []string{},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    10,
				InProgressCount:   0,
				BlockedCount:      5,
				DeferredCount:     0,
				ClosedCount:       100,
				HasRunInfo:        false,
				ReadyBeads:        []string{},
			},
			want: []string{
				"  Beads:    10 ready, 5 blocked, 100 closed",
			},
		},
		{
			name: "all bead counts zero shows none",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    5,
				UnrefinedIdeas:    []string{"Some idea"},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    0,
				InProgressCount:   0,
				BlockedCount:      0,
				DeferredCount:     0,
				ClosedCount:       0,
				HasRunInfo:        false,
				ReadyBeads:        []string{},
			},
			want: []string{
				"  Beads:    none",
			},
		},
		{
			name: "multi-status with ready bead IDs displayed",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:    0,
				UnrefinedIdeas:    []string{},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    14,
				InProgressCount:   2,
				BlockedCount:      5,
				ClosedCount:       543,
				HasRunInfo:        false,
				ReadyBeads: []string{
					"gromit-abc1 — Implement feature X",
					"gromit-abc2 — Add validation tests",
					"gromit-abc3 — Fix authentication bug",
					"gromit-abc4 — Update documentation",
				},
			},
			want: []string{
				"  Beads:    14 ready, 2 in-progress, 5 blocked, 543 closed",
				"    - gromit-abc1 — Implement feature X",
				"    - gromit-abc2 — Add validation tests",
				"    - gromit-abc3 — Fix authentication bug",
				"    (and 1 more)",
			},
		},
		{
			name: "closed count without run info - no parenthetical",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:     0,
				UnrefinedIdeas:     []string{},
				UnplannedSpecs:     []string{},
				UndecomposedPlans:  []string{},
				ReadyBeadCount:     10,
				InProgressCount:    0,
				BlockedCount:       0,
				DeferredCount:      0,
				ClosedCount:        543,
				ClosedThisRunCount: 23, // This value is present but HasRunInfo=false
				HasRunInfo:         false,
				ReadyBeads:         []string{},
			},
			want: []string{
				"  Beads:    10 ready, 543 closed",
			},
		},
		{
			name: "has run info but zero closed this run",
			status: &pipeline.PipelineStatus{
				UnrefinedCount:     0,
				UnrefinedIdeas:     []string{},
				UnplannedSpecs:     []string{},
				UndecomposedPlans:  []string{},
				ReadyBeadCount:     10,
				InProgressCount:    0,
				BlockedCount:       0,
				DeferredCount:      0,
				ClosedCount:        543,
				ClosedThisRunCount: 0,
				HasRunInfo:         true,
				ReadyBeads:         []string{},
			},
			want: []string{
				"  Beads:    10 ready, 543 closed",
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

func TestFormatRun_IncludesReliabilityAndAndonSummary(t *testing.T) {
	// Expected failure: Status does not yet include LastFailureClass, LastAndonLevel,
	// LastTrimDecision, AutonomyRate, FirstPassSuccessRate, or MTTRProxyMs fields,
	// and formatRun does not render reliability/Andon summary lines.
	now := time.Now()
	status := &Status{
		Running:              true,
		Iteration:            4,
		BeadID:               "gromit-o27x",
		BeadTitle:            "Add reliability metrics and structured Andon logging",
		Model:                "sonnet",
		StartedAt:            now.Add(-12 * time.Minute),
		ElapsedS:             12 * 60,
		LastFailureClass:     "Quality",
		LastAndonLevel:       "L2",
		LastTrimDecision:     "middle_ellipsis",
		AutonomyRate:         0.67,
		FirstPassSuccessRate: 0.5,
		MTTRProxyMs:          42000,
	}

	got := formatRun(status)
	wantLines := []string{
		"Andon:    Quality @ L2 (trim: middle_ellipsis)",
		"Reliability: autonomy 67% | first-pass 50% | MTTR 42s",
	}

	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Fatalf("formatRun() missing expected reliability line:\nwant: %q\ngot:\n%s", want, got)
		}
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

func TestFormatBeadBreakdown(t *testing.T) {
	tests := []struct {
		name   string
		status *pipeline.PipelineStatus
		want   string
	}{
		{
			name: "all counts zero",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     0,
				InProgressCount:    0,
				BlockedCount:       0,
				DeferredCount:      0,
				ClosedCount:        0,
				ClosedThisRunCount: 0,
				HasRunInfo:         false,
			},
			want: "none",
		},
		{
			name: "single status - ready only",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:  5,
				InProgressCount: 0,
				BlockedCount:    0,
				DeferredCount:   0,
				ClosedCount:     0,
				HasRunInfo:      false,
			},
			want: "5 ready",
		},
		{
			name: "single status - closed only",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:  0,
				InProgressCount: 0,
				BlockedCount:    0,
				DeferredCount:   0,
				ClosedCount:     100,
				HasRunInfo:      false,
			},
			want: "100 closed",
		},
		{
			name: "all non-zero statuses without this run info",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     14,
				InProgressCount:    2,
				BlockedCount:       5,
				DeferredCount:      3,
				ClosedCount:        543,
				ClosedThisRunCount: 0,
				HasRunInfo:         false,
			},
			want: "14 ready, 2 in-progress, 5 blocked, 3 deferred, 543 closed",
		},
		{
			name: "all non-zero statuses with this run info",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     14,
				InProgressCount:    2,
				BlockedCount:       5,
				DeferredCount:      3,
				ClosedCount:        543,
				ClosedThisRunCount: 23,
				HasRunInfo:         true,
			},
			want: "14 ready, 2 in-progress, 5 blocked, 3 deferred, 543 closed (23 this run)",
		},
		{
			name: "zero statuses omitted - ready and closed only",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:  14,
				InProgressCount: 0,
				BlockedCount:    0,
				DeferredCount:   0,
				ClosedCount:     543,
				HasRunInfo:      false,
			},
			want: "14 ready, 543 closed",
		},
		{
			name: "zero statuses omitted - blocked and in-progress only",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:  0,
				InProgressCount: 3,
				BlockedCount:    7,
				DeferredCount:   0,
				ClosedCount:     0,
				HasRunInfo:      false,
			},
			want: "3 in-progress, 7 blocked",
		},
		{
			name: "closed without run info - no parenthetical",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     10,
				InProgressCount:    0,
				BlockedCount:       0,
				DeferredCount:      0,
				ClosedCount:        543,
				ClosedThisRunCount: 23, // Has count but no run info
				HasRunInfo:         false,
			},
			want: "10 ready, 543 closed",
		},
		{
			name: "has run info but zero closed this run",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     10,
				InProgressCount:    0,
				BlockedCount:       0,
				DeferredCount:      0,
				ClosedCount:        543,
				ClosedThisRunCount: 0,
				HasRunInfo:         true,
			},
			want: "10 ready, 543 closed",
		},
		{
			name: "display order verification - all statuses in lifecycle order",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     1,
				InProgressCount:    2,
				BlockedCount:       3,
				DeferredCount:      4,
				ClosedCount:        5,
				ClosedThisRunCount: 1,
				HasRunInfo:         true,
			},
			want: "1 ready, 2 in-progress, 3 blocked, 4 deferred, 5 closed (1 this run)",
		},
		{
			name: "display order verification - sparse statuses maintain order",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:  10,
				InProgressCount: 0,
				BlockedCount:    5,
				DeferredCount:   0,
				ClosedCount:     100,
				HasRunInfo:      false,
			},
			want: "10 ready, 5 blocked, 100 closed",
		},
		{
			name:   "nil status",
			status: nil,
			want:   "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBeadBreakdown(tt.status)
			if got != tt.want {
				t.Errorf("formatBeadBreakdown() = %q, want %q", got, tt.want)
			}
		})
	}
}
