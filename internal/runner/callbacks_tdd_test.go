package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/coverage"
)

func TestResolveInitialCycleState_NilTrackerNonEmptyExpected(t *testing.T) {
	expected := []string{"output A", "output B"}

	remaining, done := resolveInitialCycleState(expected, nil)

	if len(remaining) != 2 {
		t.Fatalf("remaining len = %d, want 2", len(remaining))
	}
	if remaining[0] != "output A" || remaining[1] != "output B" {
		t.Fatalf("remaining = %v, want %v", remaining, expected)
	}
	if done {
		t.Fatal("done = true, want false for non-empty expectedOutputs")
	}
}

func TestResolveInitialCycleState_NilTrackerEmptyExpected(t *testing.T) {
	remaining, done := resolveInitialCycleState([]string{}, nil)

	if len(remaining) != 0 {
		t.Fatalf("remaining len = %d, want 0", len(remaining))
	}
	if !done {
		t.Fatal("done = false, want true for empty expectedOutputs")
	}
}

func TestResolveInitialCycleState_TrackerWithUncoveredCriteria(t *testing.T) {
	criteria := []coverage.Criterion{
		{Number: 1, Text: "First criterion"},
		{Number: 2, Text: "Second criterion"},
		{Number: 3, Text: "Third criterion"},
	}
	tracker := coverage.NewTracker(criteria, 2)
	tracker.MarkCovered(1)

	expected := []string{"original A", "original B"}
	remaining, done := resolveInitialCycleState(expected, tracker)

	if len(remaining) != 2 {
		t.Fatalf("remaining len = %d, want 2 (uncovered criteria #2 and #3)", len(remaining))
	}
	if remaining[0] != "Second criterion" {
		t.Fatalf("remaining[0] = %q, want %q", remaining[0], "Second criterion")
	}
	if remaining[1] != "Third criterion" {
		t.Fatalf("remaining[1] = %q, want %q", remaining[1], "Third criterion")
	}
	if done {
		t.Fatal("done = true, want false when uncovered criteria exist")
	}
}

func TestResolveInitialCycleState_TrackerAllCoveredPreservesOriginal(t *testing.T) {
	// This is the bug case: when all criteria are covered, the original
	// expectedOutputs must be preserved so at least one verification cycle runs.
	criteria := []coverage.Criterion{
		{Number: 1, Text: "First criterion"},
		{Number: 2, Text: "Second criterion"},
	}
	tracker := coverage.NewTracker(criteria, 2)
	tracker.MarkCovered(1)
	tracker.MarkCovered(2)

	expected := []string{"verify A", "verify B", "verify C"}
	remaining, done := resolveInitialCycleState(expected, tracker)

	if len(remaining) != 3 {
		t.Fatalf("remaining len = %d, want 3 — original expectedOutputs must be preserved when all criteria covered", len(remaining))
	}
	if remaining[0] != "verify A" || remaining[1] != "verify B" || remaining[2] != "verify C" {
		t.Fatalf("remaining = %v, want %v", remaining, expected)
	}
	if !done {
		t.Fatal("done = false, want true when all criteria are covered (IsComplete)")
	}
}

func TestResolveInitialCycleState_TrackerAllUntestablePreservesOriginal(t *testing.T) {
	// When all criteria are untestable (also empty uncovered), original outputs
	// should be preserved.
	criteria := []coverage.Criterion{
		{Number: 1, Text: "Tricky criterion"},
	}
	tracker := coverage.NewTracker(criteria, 2)
	tracker.RecordRejection(1)
	tracker.RecordRejection(1) // Now untestable

	expected := []string{"check something"}
	remaining, done := resolveInitialCycleState(expected, tracker)

	if len(remaining) != 1 {
		t.Fatalf("remaining len = %d, want 1 — original expectedOutputs must be preserved when all criteria untestable", len(remaining))
	}
	if remaining[0] != "check something" {
		t.Fatalf("remaining[0] = %q, want %q", remaining[0], "check something")
	}
	if !done {
		t.Fatal("done = false, want true when all criteria are untestable (IsComplete)")
	}
}

func TestResolveInitialCycleState_DoesNotMutateOriginalSlice(t *testing.T) {
	original := []string{"a", "b", "c"}
	copyBefore := make([]string, len(original))
	copy(copyBefore, original)

	resolveInitialCycleState(original, nil)

	for i, v := range original {
		if v != copyBefore[i] {
			t.Fatalf("original[%d] = %q, want %q — input slice was mutated", i, v, copyBefore[i])
		}
	}
}
