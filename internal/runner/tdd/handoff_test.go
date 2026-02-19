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
