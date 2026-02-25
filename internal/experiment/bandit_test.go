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

func TestSelectVariantRespectsForceVariant(t *testing.T) {
	// Verify that SelectVariant returns forced variant when provided
	state := &BanditState{
		Arms: []ArmState{
			{ID: "control", Successes: 10, Failures: 2},
			{ID: "variant-1", Successes: 5, Failures: 15},
		},
	}

	selected := state.SelectVariant("variant-1")
	if selected != "variant-1" {
		t.Fatalf("Expected selected variant to be 'variant-1', got %q", selected)
	}
}

func TestSelectVariantWithoutForceSelectsArm(t *testing.T) {
	// Verify that SelectVariant selects an arm when no force is provided
	state := &BanditState{
		Arms: []ArmState{
			{ID: "control", Successes: 1, Failures: 1},
			{ID: "variant-1", Successes: 1, Failures: 1},
		},
	}

	selected := state.SelectVariant("")
	if selected == "" {
		t.Fatalf("Expected selected variant to be non-empty")
	}

	// Should be one of the arms
	validIDs := map[string]bool{"control": true, "variant-1": true}
	if !validIDs[selected] {
		t.Fatalf("Expected selected variant to be 'control' or 'variant-1', got %q", selected)
	}
}
