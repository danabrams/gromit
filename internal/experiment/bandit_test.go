package experiment

import (
	"testing"
)

func TestBanditStateCanBeCreated(t *testing.T) {
	// Verify that BanditState struct can be created with arms
	arms := []ArmState{
		{ID: "control", Successes: 1, Failures: 1},
		{ID: "variant-1", Successes: 2, Failures: 1},
	}
	state := &BanditState{
		Arms: arms,
	}

	if state == nil {
		t.Fatalf("BanditState should not be nil")
	}
	if len(state.Arms) != 2 {
		t.Fatalf("Expected 2 arms, got %d", len(state.Arms))
	}
	if state.Arms[0].ID != "control" {
		t.Fatalf("Expected first arm ID to be 'control', got %q", state.Arms[0].ID)
	}
}
