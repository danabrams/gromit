package runner

import (
	"bytes"
	"testing"

	"github.com/danabrams/gromit/internal/coverage"
)

// TestLogCoverageSummary verifies that the coverage summary is logged to output
// when there are uncovered or untestable criteria.
func TestLogCoverageSummary(t *testing.T) {
	var output bytes.Buffer

	// Create a tracker with test criteria
	testCriteria := []coverage.Criterion{
		{Number: 1, Text: "Criterion 1"},
		{Number: 2, Text: "Criterion 2"},
		{Number: 3, Text: "Criterion 3"},
	}
	tracker := coverage.NewTracker(testCriteria, 3)
	tracker.MarkCovered(1)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)
	// Criterion 3 remains uncovered

	// Log the coverage summary
	logCoverageSummary(&output, tracker)

	loggedText := output.String()

	// Verify output contains coverage summary
	if loggedText == "" {
		t.Fatal("want logged coverage summary, got empty output")
	}
	if !containsSubstring(loggedText, "Coverage") {
		t.Errorf("logged output does not contain 'Coverage': %s", loggedText)
	}
}

// TestNoLoggingWhenAllCovered verifies that no logging happens when all criteria are covered.
func TestNoLoggingWhenAllCovered(t *testing.T) {
	var output bytes.Buffer

	testCriteria := []coverage.Criterion{
		{Number: 1, Text: "Criterion 1"},
	}
	tracker := coverage.NewTracker(testCriteria, 3)
	tracker.MarkCovered(1)

	logCoverageSummary(&output, tracker)

	loggedText := output.String()

	if loggedText != "" {
		t.Errorf("want no logging when all covered, got: %s", loggedText)
	}
}
