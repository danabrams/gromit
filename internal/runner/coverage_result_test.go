package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestPopulateCoverageResult verifies that populateCoverageResult correctly
// populates coverage fields in IterationResult from a CoverageTracker.
func TestPopulateCoverageResult(t *testing.T) {
	// Create a tracker with test criteria
	testCriteria := []coverage.Criterion{
		{Number: 1, Text: "Criterion 1"},
		{Number: 2, Text: "Criterion 2"},
		{Number: 3, Text: "Criterion 3"},
	}
	tracker := coverage.NewTracker(testCriteria, 3)

	// Populate tracker state to simulate coverage tracking
	tracker.MarkCovered(1)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)
	// Criterion 3 remains uncovered

	// Create a BeadContext with IterationResult
	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{},
	}

	// Call populateCoverageResult
	populateCoverageResult(bc, tracker)

	// Verify coverage fields are populated
	if bc.Result.CriteriaTotal != 3 {
		t.Errorf("CriteriaTotal = %d, want 3", bc.Result.CriteriaTotal)
	}
	if bc.Result.CriteriaCovered != 1 {
		t.Errorf("CriteriaCovered = %d, want 1", bc.Result.CriteriaCovered)
	}
	if bc.Result.CriteriaUntestable != 1 {
		t.Errorf("CriteriaUntestable = %d, want 1", bc.Result.CriteriaUntestable)
	}
	if len(bc.Result.UncoveredCriteria) != 1 {
		t.Errorf("len(UncoveredCriteria) = %d, want 1", len(bc.Result.UncoveredCriteria))
	}
	if len(bc.Result.UncoveredCriteria) > 0 && bc.Result.UncoveredCriteria[0] != "Criterion 3" {
		t.Errorf("UncoveredCriteria[0] = %q, want %q", bc.Result.UncoveredCriteria[0], "Criterion 3")
	}
}
