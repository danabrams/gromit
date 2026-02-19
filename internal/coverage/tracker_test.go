package coverage

import "testing"

func TestNewTracker_NextUncoveredReturnsCriterion(t *testing.T) {
	criteria := []Criterion{{Number: 1, Text: "System accepts valid input"}}
	tracker := NewTracker(criteria, 2)

	next := tracker.NextUncovered()

	if next == nil {
		t.Fatal("NextUncovered() = nil, want criterion 1")
	}
	if next.Number != 1 {
		t.Fatalf("NextUncovered().Number = %d, want 1", next.Number)
	}
}
