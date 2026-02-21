package tdd

import "testing"

func TestCycleStateIsCompleteWhenDone(t *testing.T) {
	state := CycleState{
		CycleNumber: 1,
		MaxCycles:   3,
		Done:        true,
	}

	if !state.IsComplete() {
		t.Fatalf("expected IsComplete to be true when done is true")
	}
}

func TestCycleStateIsCompleteWhenCycleReachesMax(t *testing.T) {
	state := CycleState{
		CycleNumber: 3,
		MaxCycles:   3,
		Done:        false,
	}

	if !state.IsComplete() {
		t.Fatalf("expected IsComplete to be true when cycle number reaches max cycles")
	}
}

func TestCycleStateIsNotCompleteWhenZeroCyclesAndEmptyRemaining(t *testing.T) {
	state := CycleState{
		CycleNumber: 0,
		MaxCycles:   3,
		Done:        false,
		Remaining:   []string{},
	}

	if state.IsComplete() {
		t.Fatalf("expected IsComplete to be false when no cycles have run, even with empty remaining")
	}
}

func TestCycleStateIsCompleteWhenOneCycleAndEmptyRemaining(t *testing.T) {
	state := CycleState{
		CycleNumber: 1,
		MaxCycles:   3,
		Done:        false,
		Remaining:   []string{},
	}

	if !state.IsComplete() {
		t.Fatalf("expected IsComplete to be true when at least one cycle ran and no requirements remain")
	}
}

func TestCycleStateIsCompleteWhenDoneRegardlessOfState(t *testing.T) {
	state := CycleState{
		CycleNumber: 0,
		MaxCycles:   5,
		Done:        true,
		Remaining:   []string{"something"},
	}

	if !state.IsComplete() {
		t.Fatalf("expected IsComplete to be true when Done is true, regardless of other state")
	}
}

func TestCycleStateIsNotCompleteWhenZeroCyclesWithRemaining(t *testing.T) {
	state := CycleState{
		CycleNumber: 0,
		MaxCycles:   3,
		Done:        false,
		Remaining:   []string{"implement feature X"},
	}

	if state.IsComplete() {
		t.Fatalf("expected IsComplete to be false when no cycles have run and requirements remain")
	}
}

func TestRedHandoffNilMapsAreSafeToRead(t *testing.T) {
	var handoff RedHandoff

	if len(handoff.TestFiles) != 0 {
		t.Fatalf("expected zero test files, got %d", len(handoff.TestFiles))
	}
	if len(handoff.ImplFiles) != 0 {
		t.Fatalf("expected zero impl files, got %d", len(handoff.ImplFiles))
	}
}

func TestGreenHandoffNilMapIsSafeToRead(t *testing.T) {
	var handoff GreenHandoff

	if len(handoff.ImplFiles) != 0 {
		t.Fatalf("expected zero impl files, got %d", len(handoff.ImplFiles))
	}
}

func TestRefactorHandoffNilMapsAreSafeToRead(t *testing.T) {
	var handoff RefactorHandoff

	if len(handoff.TestFiles) != 0 {
		t.Fatalf("expected zero test files, got %d", len(handoff.TestFiles))
	}
	if len(handoff.ImplFiles) != 0 {
		t.Fatalf("expected zero impl files, got %d", len(handoff.ImplFiles))
	}
}
