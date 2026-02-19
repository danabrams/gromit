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
