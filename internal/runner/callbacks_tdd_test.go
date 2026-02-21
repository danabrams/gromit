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

// Tests for red-pass coverage advancement pattern.
// These verify that MarkCovered (called from ValidateFn when red-phase passes)
// correctly advances the tracker so resolveInitialCycleState reflects progress.

func TestRedPassCoverageAdvancement_SingleCriterion(t *testing.T) {
	criteria := []coverage.Criterion{
		{Number: 1, Text: "Criterion A"},
	}
	tracker := coverage.NewTracker(criteria, 2)

	// Simulate red-pass: mark criterion as covered
	tracker.MarkCovered(1)

	remaining, done := resolveInitialCycleState([]string{"original"}, tracker)
	if !done {
		t.Fatal("expected done=true after marking only criterion as covered")
	}
	// When all covered, original outputs preserved
	if len(remaining) != 1 || remaining[0] != "original" {
		t.Fatalf("remaining = %v, want [original]", remaining)
	}
}

func TestRedPassCoverageAdvancement_MultipleCriteriaSequential(t *testing.T) {
	// Simulates multiple criteria being red-pass covered one at a time,
	// verifying tracker advances correctly after each marking.
	criteria := []coverage.Criterion{
		{Number: 1, Text: "Criterion A"},
		{Number: 2, Text: "Criterion B"},
		{Number: 3, Text: "Criterion C"},
	}
	tracker := coverage.NewTracker(criteria, 2)

	// After marking criterion 1
	tracker.MarkCovered(1)
	remaining, done := resolveInitialCycleState([]string{}, tracker)
	if done {
		t.Fatal("expected done=false after covering only 1 of 3 criteria")
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining len = %d, want 2 after covering 1 criterion", len(remaining))
	}
	if remaining[0] != "Criterion B" || remaining[1] != "Criterion C" {
		t.Fatalf("remaining = %v, want [Criterion B, Criterion C]", remaining)
	}

	// After marking criterion 2
	tracker.MarkCovered(2)
	remaining, done = resolveInitialCycleState([]string{}, tracker)
	if done {
		t.Fatal("expected done=false after covering 2 of 3 criteria")
	}
	if len(remaining) != 1 || remaining[0] != "Criterion C" {
		t.Fatalf("remaining = %v, want [Criterion C]", remaining)
	}

	// After marking criterion 3 — all covered
	tracker.MarkCovered(3)
	_, done = resolveInitialCycleState([]string{}, tracker)
	if !done {
		t.Fatal("expected done=true after covering all criteria")
	}
}

func TestRedPassCoverageAdvancement_NextUncoveredAdvancesAfterMark(t *testing.T) {
	// Verifies that NextUncovered (used in RenderRedFn) returns the correct
	// next criterion after each MarkCovered call.
	criteria := []coverage.Criterion{
		{Number: 1, Text: "First"},
		{Number: 2, Text: "Second"},
		{Number: 3, Text: "Third"},
	}
	tracker := coverage.NewTracker(criteria, 2)

	next := tracker.NextUncovered()
	if next == nil || next.Number != 1 {
		t.Fatalf("initial NextUncovered = %v, want criterion #1", next)
	}

	tracker.MarkCovered(1)
	next = tracker.NextUncovered()
	if next == nil || next.Number != 2 {
		t.Fatalf("after marking #1, NextUncovered = %v, want criterion #2", next)
	}

	tracker.MarkCovered(2)
	next = tracker.NextUncovered()
	if next == nil || next.Number != 3 {
		t.Fatalf("after marking #2, NextUncovered = %v, want criterion #3", next)
	}

	tracker.MarkCovered(3)
	next = tracker.NextUncovered()
	if next != nil {
		t.Fatalf("after marking all, NextUncovered = %v, want nil", next)
	}
	if !tracker.IsComplete() {
		t.Fatal("expected tracker.IsComplete() = true after all marked covered")
	}
}

func TestRedPassCoverageAdvancement_MixedCoveredAndUncovered(t *testing.T) {
	// Simulates a mixed scenario: some criteria marked covered (red-pass),
	// some still uncovered (need full red-green-refactor).
	criteria := []coverage.Criterion{
		{Number: 1, Text: "Already implemented"},
		{Number: 2, Text: "Needs full TDD"},
		{Number: 3, Text: "Also implemented"},
		{Number: 4, Text: "Also needs TDD"},
	}
	tracker := coverage.NewTracker(criteria, 2)

	// Mark #1 and #3 as covered (red-pass path)
	tracker.MarkCovered(1)
	tracker.MarkCovered(3)

	remaining, done := resolveInitialCycleState([]string{}, tracker)
	if done {
		t.Fatal("expected done=false when 2 criteria remain uncovered")
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining len = %d, want 2", len(remaining))
	}
	if remaining[0] != "Needs full TDD" || remaining[1] != "Also needs TDD" {
		t.Fatalf("remaining = %v, want [Needs full TDD, Also needs TDD]", remaining)
	}

	// Now mark #2 and #4 as covered
	tracker.MarkCovered(2)
	tracker.MarkCovered(4)
	if !tracker.IsComplete() {
		t.Fatal("expected tracker complete after all criteria covered")
	}
}
