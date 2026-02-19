package coverage

import "testing"

func TestNewTracker_NextUncoveredReturnsNilForEmpty(t *testing.T) {
	tracker := NewTracker([]Criterion{}, 2)

	next := tracker.NextUncovered()

	if next != nil {
		t.Fatalf("NextUncovered() = %v, want nil for empty tracker", next)
	}
}

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

func TestRecordRejection_TransitionsToUntestableAtThreshold(t *testing.T) {
	criteria := []Criterion{{Number: 1, Text: "Tricky criterion"}}
	tracker := NewTracker(criteria, 2)

	tracker.RecordRejection(1)
	if tracker.NextUncovered() == nil {
		t.Fatal("after 1 rejection: NextUncovered() = nil, criterion should still be unchecked")
	}

	tracker.RecordRejection(1)
	if tracker.NextUncovered() != nil {
		t.Fatal("after 2 rejections: NextUncovered() should return nil (criterion is now untestable)")
	}
}

func TestMarkCovered_NextUncoveredSkipsCoveredCriterion(t *testing.T) {
	criteria := []Criterion{
		{Number: 1, Text: "First"},
		{Number: 2, Text: "Second"},
	}
	tracker := NewTracker(criteria, 2)

	tracker.MarkCovered(1)

	next := tracker.NextUncovered()
	if next == nil {
		t.Fatal("NextUncovered() = nil, want criterion 2")
	}
	if next.Number != 2 {
		t.Fatalf("NextUncovered().Number = %d, want 2", next.Number)
	}
}
