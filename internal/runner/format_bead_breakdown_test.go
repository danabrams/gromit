package runner

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

// TestFormatPipeline_ExpandedBeadBreakdown verifies formatPipeline shows all bead status counts
func TestFormatPipeline_ExpandedBeadBreakdown(t *testing.T) {
	// Expected failure: formatPipeline does not format expanded bead breakdown yet

	tests := []struct {
		name   string
		status *pipeline.PipelineStatus
		want   string // Expected Beads line
	}{
		{
			name: "all counts non-zero without run info",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     14,
				InProgressCount:    2,
				BlockedCount:       5,
				DeferredCount:      3,
				ClosedCount:        543,
				ClosedThisRunCount: 0,
				HasRunInfo:         false,
			},
			want: "  Beads:    14 ready, 2 in-progress, 5 blocked, 3 deferred, 543 closed",
		},
		{
			name: "all counts non-zero with run info",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     14,
				InProgressCount:    2,
				BlockedCount:       5,
				DeferredCount:      3,
				ClosedCount:        543,
				ClosedThisRunCount: 23,
				HasRunInfo:         true,
			},
			want: "  Beads:    14 ready, 2 in-progress, 5 blocked, 3 deferred, 543 closed (23 this run)",
		},
		{
			name: "some counts zero are omitted",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     14,
				InProgressCount:    0,
				BlockedCount:       5,
				DeferredCount:      0,
				ClosedCount:        543,
				ClosedThisRunCount: 23,
				HasRunInfo:         true,
			},
			want: "  Beads:    14 ready, 5 blocked, 543 closed (23 this run)",
		},
		{
			name: "only ready and closed",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     7,
				InProgressCount:    0,
				BlockedCount:       0,
				DeferredCount:      0,
				ClosedCount:        100,
				ClosedThisRunCount: 0,
				HasRunInfo:         false,
			},
			want: "  Beads:    7 ready, 100 closed",
		},
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
			want: "  Beads:    none",
		},
		{
			name: "no run info omits parenthetical",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     14,
				InProgressCount:    2,
				BlockedCount:       5,
				DeferredCount:      3,
				ClosedCount:        543,
				ClosedThisRunCount: 0,
				HasRunInfo:         false,
			},
			want: "  Beads:    14 ready, 2 in-progress, 5 blocked, 3 deferred, 543 closed",
		},
		{
			name: "run info with zero closed this run shows count without parenthetical",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     10,
				InProgressCount:    0,
				BlockedCount:       2,
				DeferredCount:      0,
				ClosedCount:        50,
				ClosedThisRunCount: 0,
				HasRunInfo:         true,
			},
			want: "  Beads:    10 ready, 2 blocked, 50 closed",
		},
		{
			name: "only in-progress beads",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     0,
				InProgressCount:    3,
				BlockedCount:       0,
				DeferredCount:      0,
				ClosedCount:        0,
				ClosedThisRunCount: 0,
				HasRunInfo:         false,
			},
			want: "  Beads:    3 in-progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPipeline(tt.status)
			if !strings.Contains(got, tt.want) {
				t.Errorf("formatPipeline() missing expected Beads line:\n  want: %q\n  got:\n%s", tt.want, got)
			}
		})
	}
}

// TestFormatPipeline_BeadBreakdownOrder verifies status order is always consistent
func TestFormatPipeline_BeadBreakdownOrder(t *testing.T) {
	// Expected failure: formatPipeline does not format bead breakdown in fixed order yet

	status := &pipeline.PipelineStatus{
		ReadyBeadCount:  14,
		InProgressCount: 2,
		BlockedCount:    5,
		DeferredCount:   3,
		ClosedCount:     543,
		HasRunInfo:      false,
	}

	got := formatPipeline(status)

	// The order must always be: ready, in-progress, blocked, deferred, closed
	// Find the positions of each status in the output
	readyPos := strings.Index(got, "ready")
	inProgressPos := strings.Index(got, "in-progress")
	blockedPos := strings.Index(got, "blocked")
	deferredPos := strings.Index(got, "deferred")
	closedPos := strings.Index(got, "closed")

	if readyPos == -1 || inProgressPos == -1 || blockedPos == -1 || deferredPos == -1 || closedPos == -1 {
		t.Fatalf("formatPipeline() missing one or more status labels in output:\n%s", got)
	}

	// Verify order
	if readyPos > inProgressPos {
		t.Errorf("ready should appear before in-progress, got ready at %d, in-progress at %d", readyPos, inProgressPos)
	}
	if inProgressPos > blockedPos {
		t.Errorf("in-progress should appear before blocked, got in-progress at %d, blocked at %d", inProgressPos, blockedPos)
	}
	if blockedPos > deferredPos {
		t.Errorf("blocked should appear before deferred, got blocked at %d, deferred at %d", blockedPos, deferredPos)
	}
	if deferredPos > closedPos {
		t.Errorf("deferred should appear before closed, got deferred at %d, closed at %d", deferredPos, closedPos)
	}
}

// TestFormatPipeline_ReadyBeadIDsStillShown verifies ready bead ID list is preserved
func TestFormatPipeline_ReadyBeadIDsStillShown(t *testing.T) {
	// Expected failure: formatPipeline may not preserve ready bead ID list with new format

	status := &pipeline.PipelineStatus{
		ReadyBeadCount:  5,
		InProgressCount: 2,
		BlockedCount:    3,
		ClosedCount:     100,
		ReadyBeads: []string{
			"gromit-abc1 — Implement feature X",
			"gromit-abc2 — Add validation tests",
			"gromit-abc3 — Fix authentication bug",
			"gromit-abc4 — Update documentation",
			"gromit-abc5 — Refactor helper",
		},
		HasRunInfo: false,
	}

	got := formatPipeline(status)

	// Verify the Beads line has expanded format
	if !strings.Contains(got, "5 ready") {
		t.Errorf("formatPipeline() missing ready count in Beads line")
	}

	// Verify the ready bead IDs are still shown below the Beads line
	expectedIDs := []string{
		"gromit-abc1 — Implement feature X",
		"gromit-abc2 — Add validation tests",
		"gromit-abc3 — Fix authentication bug",
	}

	for _, id := range expectedIDs {
		if !strings.Contains(got, id) {
			t.Errorf("formatPipeline() missing expected ready bead ID:\n  want: %q\n  got:\n%s", id, got)
		}
	}

	// Verify overflow message is shown (5 beads, 3 shown)
	if !strings.Contains(got, "(and 2 more)") {
		t.Errorf("formatPipeline() missing overflow message for ready beads")
	}
}

// TestFormatPipeline_ThisRunParenthetical verifies "(X this run)" formatting
func TestFormatPipeline_ThisRunParenthetical(t *testing.T) {
	// Expected failure: formatPipeline does not add "(X this run)" parenthetical yet

	tests := []struct {
		name       string
		status     *pipeline.PipelineStatus
		wantInLine string
		notInLine  string
	}{
		{
			name: "with run info and closed this run",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     10,
				ClosedCount:        543,
				ClosedThisRunCount: 23,
				HasRunInfo:         true,
			},
			wantInLine: "543 closed (23 this run)",
			notInLine:  "",
		},
		{
			name: "without run info",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     10,
				ClosedCount:        543,
				ClosedThisRunCount: 0,
				HasRunInfo:         false,
			},
			wantInLine: "543 closed",
			notInLine:  "this run",
		},
		{
			name: "with run info but zero closed this run",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount:     10,
				ClosedCount:        543,
				ClosedThisRunCount: 0,
				HasRunInfo:         true,
			},
			wantInLine: "543 closed",
			notInLine:  "this run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPipeline(tt.status)

			if !strings.Contains(got, tt.wantInLine) {
				t.Errorf("formatPipeline() missing expected text:\n  want: %q\n  got:\n%s", tt.wantInLine, got)
			}

			if tt.notInLine != "" && strings.Contains(got, tt.notInLine) {
				t.Errorf("formatPipeline() should not contain:\n  unwanted: %q\n  got:\n%s", tt.notInLine, got)
			}
		})
	}
}

// TestFormatPipeline_BackwardCompatibility verifies old status structs still work
func TestFormatPipeline_BackwardCompatibility(t *testing.T) {
	// Expected failure: formatPipeline may require new fields to be present

	// Old-style PipelineStatus without new fields should still format correctly
	// (new fields will be zero-valued)
	status := &pipeline.PipelineStatus{
		UnrefinedCount:    2,
		UnrefinedIdeas:    []string{"Add feature A", "Fix bug B"},
		UnplannedSpecs:    []string{"spec-1"},
		UndecomposedPlans: []string{"plan-1"},
		ReadyBeadCount:    5,
		ReadyBeads:        []string{"gromit-001", "gromit-002"},
		Recommendation:    "Run 5 ready bead(s)",
	}

	got := formatPipeline(status)

	// Should not panic or error, and should include at minimum the ready count
	if !strings.Contains(got, "5 ready") {
		t.Errorf("formatPipeline() with old-style status missing ready count")
	}

	// When new fields are zero, they should be omitted from output
	// (except we'd show "none" if ALL counts are zero)
	// With ready=5, we should show "5 ready" and omit other zero counts
}

// TestFormatPipeline_NoneWhenAllZero verifies "none" is shown when all bead counts are zero
func TestFormatPipeline_NoneWhenAllZero(t *testing.T) {
	// Expected failure: formatPipeline does not show "none" for all-zero bead counts yet

	status := &pipeline.PipelineStatus{
		ReadyBeadCount:  0,
		InProgressCount: 0,
		BlockedCount:    0,
		DeferredCount:   0,
		ClosedCount:     0,
		HasRunInfo:      false,
	}

	got := formatPipeline(status)

	if !strings.Contains(got, "Beads:    none") {
		t.Errorf("formatPipeline() should show 'none' when all bead counts are zero:\n%s", got)
	}

	// Should not show any status labels like "ready", "blocked", etc.
	unwantedLabels := []string{"ready", "in-progress", "blocked", "deferred"}
	for _, label := range unwantedLabels {
		// Check that the label doesn't appear in the Beads line
		// (it might appear elsewhere like in "ready" for "Next action" line)
		lines := strings.Split(got, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Beads:") && strings.Contains(line, label) && !strings.Contains(line, "none") {
				t.Errorf("formatPipeline() Beads line should not contain %q when showing 'none':\n%s", label, line)
			}
		}
	}
}

// TestFormatPipeline_SingleStatusShown verifies output when only one status has non-zero count
func TestFormatPipeline_SingleStatusShown(t *testing.T) {
	// Expected failure: formatPipeline does not handle single-status output correctly yet

	tests := []struct {
		name   string
		status *pipeline.PipelineStatus
		want   string
	}{
		{
			name: "only ready beads",
			status: &pipeline.PipelineStatus{
				ReadyBeadCount: 10,
			},
			want: "  Beads:    10 ready",
		},
		{
			name: "only in-progress beads",
			status: &pipeline.PipelineStatus{
				InProgressCount: 3,
			},
			want: "  Beads:    3 in-progress",
		},
		{
			name: "only blocked beads",
			status: &pipeline.PipelineStatus{
				BlockedCount: 5,
			},
			want: "  Beads:    5 blocked",
		},
		{
			name: "only deferred beads",
			status: &pipeline.PipelineStatus{
				DeferredCount: 2,
			},
			want: "  Beads:    2 deferred",
		},
		{
			name: "only closed beads",
			status: &pipeline.PipelineStatus{
				ClosedCount: 100,
			},
			want: "  Beads:    100 closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPipeline(tt.status)
			if !strings.Contains(got, tt.want) {
				t.Errorf("formatPipeline() missing expected line:\n  want: %q\n  got:\n%s", tt.want, got)
			}

			// Verify no other status labels appear in the Beads line
			lines := strings.Split(got, "\n")
			var beadsLine string
			for _, line := range lines {
				if strings.Contains(line, "Beads:") {
					beadsLine = line
					break
				}
			}

			if beadsLine == "" {
				t.Fatalf("formatPipeline() missing Beads line in output")
			}

			// Count commas - there should be 0 commas for a single status
			commaCount := strings.Count(beadsLine, ",")
			if commaCount > 0 {
				t.Errorf("formatPipeline() should show single status without commas, got %d commas in: %q", commaCount, beadsLine)
			}
		})
	}
}
