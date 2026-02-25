package coverage

import (
	"strings"
	"testing"
)

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

func TestNextUncovered_ReturnsLowestCriterionNumber(t *testing.T) {
	criteria := []Criterion{
		{Number: 3, Text: "Third"},
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

func TestSummary_RendersHumanReadableSummary(t *testing.T) {
	criteria := []Criterion{
		{Number: 1, Text: "First criterion"},
		{Number: 2, Text: "Second criterion"},
		{Number: 3, Text: "Third criterion"},
	}
	tracker := NewTracker(criteria, 2)
	tracker.MarkCovered(1)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)

	got := tracker.Summary()

	if !strings.Contains(got, "1/3") {
		t.Errorf("Summary() = %q, want it to contain covered count 1/3", got)
	}
	if !strings.Contains(got, "#3") {
		t.Errorf("Summary() = %q, want it to mention uncovered criterion #3", got)
	}
	if !strings.Contains(got, "#2") {
		t.Errorf("Summary() = %q, want it to mention untestable criterion #2", got)
	}
}

func TestFormatCoverageState_RendersTargetingAndRemaining(t *testing.T) {
	criteria := []Criterion{
		{Number: 1, Text: "First criterion"},
		{Number: 2, Text: "Second criterion"},
		{Number: 3, Text: "Third criterion"},
	}
	tracker := NewTracker(criteria, 2)
	tracker.MarkCovered(2)

	got := tracker.FormatCoverageState(1)

	want := "## Coverage State\nTargeting criterion #1: \"First criterion\"\nRemaining uncovered: #1, #3"
	if got != want {
		t.Fatalf("FormatCoverageState() =\n%q\nwant:\n%q", got, want)
	}
}

func TestUntestableCriteria_ReturnsOnlyUntestableItems(t *testing.T) {
	criteria := []Criterion{
		{Number: 1, Text: "First"},
		{Number: 2, Text: "Second"},
	}
	tracker := NewTracker(criteria, 2)
	tracker.MarkCovered(1)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)

	untestable := tracker.UntestableCriteria()

	if len(untestable) != 1 {
		t.Fatalf("UntestableCriteria() len = %d, want 1", len(untestable))
	}
	if untestable[0].Number != 2 {
		t.Fatalf("UntestableCriteria()[0].Number = %d, want 2", untestable[0].Number)
	}
}

func TestUncoveredCriteria_ReturnsOnlyUncheckedItems(t *testing.T) {
	criteria := []Criterion{
		{Number: 1, Text: "First"},
		{Number: 2, Text: "Second"},
		{Number: 3, Text: "Third"},
	}
	tracker := NewTracker(criteria, 2)
	tracker.MarkCovered(1)
	tracker.RecordRejection(3)
	tracker.RecordRejection(3)

	uncovered := tracker.UncoveredCriteria()

	if len(uncovered) != 1 {
		t.Fatalf("UncoveredCriteria() len = %d, want 1", len(uncovered))
	}
	if uncovered[0].Number != 2 {
		t.Fatalf("UncoveredCriteria()[0].Number = %d, want 2", uncovered[0].Number)
	}
}

func TestIsComplete_ReturnsTrueWhenAllCoveredOrUntestable(t *testing.T) {
	criteria := []Criterion{
		{Number: 1, Text: "First"},
		{Number: 2, Text: "Second"},
	}
	tracker := NewTracker(criteria, 2)

	if tracker.IsComplete() {
		t.Fatal("IsComplete() = true before any criteria covered, want false")
	}

	tracker.MarkCovered(1)
	if tracker.IsComplete() {
		t.Fatal("IsComplete() = true with one still unchecked, want false")
	}

	tracker.RecordRejection(2)
	tracker.RecordRejection(2)
	if !tracker.IsComplete() {
		t.Fatal("IsComplete() = false when criterion 1 covered and criterion 2 untestable, want true")
	}
}

func TestIsComplete_ReturnsTrueForEmptyTracker(t *testing.T) {
	tracker := NewTracker([]Criterion{}, 2)

	if !tracker.IsComplete() {
		t.Fatal("IsComplete() = false for empty tracker, want true")
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

func TestRecordRejection_DoesNotChangeCoveredCriterion(t *testing.T) {
	criteria := []Criterion{{Number: 1, Text: "First"}}
	tracker := NewTracker(criteria, 2)

	tracker.MarkCovered(1)
	tracker.RecordRejection(1)
	tracker.RecordRejection(1)

	if !tracker.IsComplete() {
		t.Fatal("IsComplete() = false, want true after covered criterion receives rejections")
	}
	uncovered := tracker.UncoveredCriteria()
	if len(uncovered) != 0 {
		t.Fatalf("UncoveredCriteria() len = %d, want 0", len(uncovered))
	}
	untestable := tracker.UntestableCriteria()
	if len(untestable) != 0 {
		t.Fatalf("UntestableCriteria() len = %d, want 0", len(untestable))
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

func TestMarkCovered_DoesNotChangeUntestableCriterion(t *testing.T) {
	criteria := []Criterion{{Number: 1, Text: "First"}}
	tracker := NewTracker(criteria, 2)

	tracker.RecordRejection(1)
	tracker.RecordRejection(1)
	tracker.MarkCovered(1)

	untestable := tracker.UntestableCriteria()
	if len(untestable) != 1 {
		t.Fatalf("UntestableCriteria() len = %d, want 1", len(untestable))
	}
	if untestable[0].Number != 1 {
		t.Fatalf("UntestableCriteria()[0].Number = %d, want 1", untestable[0].Number)
	}
}

func TestNewTracker_InitialStateIsPending(t *testing.T) {
	tracker := NewTracker([]Criterion{}, 2)

	if tracker.State() != StatePending {
		t.Fatalf("tracker.State() = %v, want %v", tracker.State(), StatePending)
	}
}

func TestTracker_ToCollectingTransition(t *testing.T) {
	tracker := NewTracker([]Criterion{{Number: 1, Text: "First"}}, 2)

	tracker.ToCollecting()

	if tracker.State() != StateCollecting {
		t.Fatalf("tracker.State() = %v, want %v", tracker.State(), StateCollecting)
	}
}

func TestTracker_ToValidatingTransition(t *testing.T) {
	tracker := NewTracker([]Criterion{{Number: 1, Text: "First"}}, 2)
	tracker.ToCollecting()

	tracker.ToValidating()

	if tracker.State() != StateValidating {
		t.Fatalf("tracker.State() = %v, want %v", tracker.State(), StateValidating)
	}
}

func TestTracker_ToCompleteTransition(t *testing.T) {
	tracker := NewTracker([]Criterion{{Number: 1, Text: "First"}}, 2)
	tracker.ToCollecting()
	tracker.ToValidating()

	tracker.ToComplete()

	if tracker.State() != StateComplete {
		t.Fatalf("tracker.State() = %v, want %v", tracker.State(), StateComplete)
	}
}

func TestTracker_ToErrorTransition(t *testing.T) {
	tracker := NewTracker([]Criterion{{Number: 1, Text: "First"}}, 2)
	tracker.ToCollecting()

	tracker.ToError()

	if tracker.State() != StateError {
		t.Fatalf("tracker.State() = %v, want %v", tracker.State(), StateError)
	}
}

func TestTracker_ResetFromErrorStateToPending(t *testing.T) {
	tracker := NewTracker([]Criterion{{Number: 1, Text: "First"}}, 2)
	tracker.ToCollecting()
	tracker.ToError()

	tracker.Reset()

	if tracker.State() != StatePending {
		t.Fatalf("tracker.State() after reset = %v, want %v", tracker.State(), StatePending)
	}
}

func TestTrackerStateTransitions_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(*CoverageTracker)
		transition      func(*CoverageTracker)
		expectedState   State
		description     string
	}{
		{
			name: "PendingToCollecting",
			setup: func(tr *CoverageTracker) {
				// Already in pending state
			},
			transition: func(tr *CoverageTracker) {
				tr.ToCollecting()
			},
			expectedState: StateCollecting,
			description:   "Pending -> Collecting transition",
		},
		{
			name: "CollectingToValidating",
			setup: func(tr *CoverageTracker) {
				tr.ToCollecting()
			},
			transition: func(tr *CoverageTracker) {
				tr.ToValidating()
			},
			expectedState: StateValidating,
			description:   "Collecting -> Validating transition",
		},
		{
			name: "ValidatingToComplete",
			setup: func(tr *CoverageTracker) {
				tr.ToCollecting()
				tr.ToValidating()
			},
			transition: func(tr *CoverageTracker) {
				tr.ToComplete()
			},
			expectedState: StateComplete,
			description:   "Validating -> Complete transition",
		},
		{
			name: "CollectingToError",
			setup: func(tr *CoverageTracker) {
				tr.ToCollecting()
			},
			transition: func(tr *CoverageTracker) {
				tr.ToError()
			},
			expectedState: StateError,
			description:   "Collecting -> Error transition (error from any state)",
		},
		{
			name: "ValidatingToError",
			setup: func(tr *CoverageTracker) {
				tr.ToCollecting()
				tr.ToValidating()
			},
			transition: func(tr *CoverageTracker) {
				tr.ToError()
			},
			expectedState: StateError,
			description:   "Validating -> Error transition",
		},
		{
			name: "ErrorToPending",
			setup: func(tr *CoverageTracker) {
				tr.ToCollecting()
				tr.ToError()
			},
			transition: func(tr *CoverageTracker) {
				tr.Reset()
			},
			expectedState: StatePending,
			description:   "Error -> Pending transition (reset path)",
		},
		{
			name: "InvalidTransitionToCollectingFromCollecting",
			setup: func(tr *CoverageTracker) {
				tr.ToCollecting()
			},
			transition: func(tr *CoverageTracker) {
				tr.ToCollecting()
			},
			expectedState: StateCollecting,
			description:   "ToCollecting from collecting should remain in collecting",
		},
		{
			name: "InvalidTransitionToValidatingFromPending",
			setup: func(tr *CoverageTracker) {
				// stays in pending
			},
			transition: func(tr *CoverageTracker) {
				tr.ToValidating()
			},
			expectedState: StatePending,
			description:   "ToValidating from pending should remain in pending (invalid transition)",
		},
		{
			name: "InvalidTransitionToCompleteFromPending",
			setup: func(tr *CoverageTracker) {
				// stays in pending
			},
			transition: func(tr *CoverageTracker) {
				tr.ToComplete()
			},
			expectedState: StatePending,
			description:   "ToComplete from pending should remain in pending (invalid transition)",
		},
		{
			name: "ResetFromPendingHasNoEffect",
			setup: func(tr *CoverageTracker) {
				// stays in pending
			},
			transition: func(tr *CoverageTracker) {
				tr.Reset()
			},
			expectedState: StatePending,
			description:   "Reset from pending should remain in pending (only valid from error)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker([]Criterion{{Number: 1, Text: "First"}}, 2)
			tt.setup(tracker)
			tt.transition(tracker)

			if tracker.State() != tt.expectedState {
				t.Errorf("%s: got state %v, want %v", tt.description, tracker.State(), tt.expectedState)
			}
		})
	}
}
