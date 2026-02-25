package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestAddCoverageCommentWhenUncoveredExists verifies that when there are uncovered
// criteria, a bead comment is added with the coverage summary.
func TestAddCoverageCommentWhenUncoveredExists(t *testing.T) {
	var addedComments []string
	beadsClient := &mockBeadsClient{
		addCommentFn: func(id, comment string) error {
			addedComments = append(addedComments, comment)
			return nil
		},
	}

	// Create a tracker with test criteria
	testCriteria := []coverage.Criterion{
		{Number: 1, Text: "Criterion 1"},
		{Number: 2, Text: "Criterion 2"},
	}
	tracker := coverage.NewTracker(testCriteria, 3)
	tracker.MarkCovered(1)
	// Criterion 2 remains uncovered

	b := &bead.Bead{ID: "test-id", Title: "Test bead"}
	bc := &runtypes.BeadContext{Bead: b, Result: &runtypes.IterationResult{}}

	// Call the function that should add the comment
	addCoverageCommentWithClient(bc, tracker, beadsClient)

	// Verify a comment was added
	if len(addedComments) != 1 {
		t.Fatalf("want 1 comment added, got %d", len(addedComments))
	}

	// Verify the comment contains coverage summary info
	comment := addedComments[0]
	if !containsSubstring(comment, "Coverage") {
		t.Errorf("comment does not contain 'Coverage': %s", comment)
	}
}

// TestNoCommentWhenAllCovered verifies that when all criteria are covered,
// no comment is added.
func TestNoCommentWhenAllCovered(t *testing.T) {
	var addedComments []string
	beadsClient := &mockBeadsClient{
		addCommentFn: func(id, comment string) error {
			addedComments = append(addedComments, comment)
			return nil
		},
	}

	testCriteria := []coverage.Criterion{
		{Number: 1, Text: "Criterion 1"},
	}
	tracker := coverage.NewTracker(testCriteria, 3)
	tracker.MarkCovered(1)

	b := &bead.Bead{ID: "test-id", Title: "Test bead"}
	bc := &runtypes.BeadContext{Bead: b, Result: &runtypes.IterationResult{}}

	addCoverageCommentWithClient(bc, tracker, beadsClient)

	if len(addedComments) != 0 {
		t.Errorf("want no comments when all criteria covered, got %d", len(addedComments))
	}
}

// mockBeadsClient is a test double for bead.Client
type mockBeadsClient struct {
	addCommentFn func(id, comment string) error
}

func (m *mockBeadsClient) AddComment(id, comment string) error {
	if m.addCommentFn == nil {
		return nil
	}
	return m.addCommentFn(id, comment)
}

// containsSubstring checks if haystack contains needle
func containsSubstring(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
