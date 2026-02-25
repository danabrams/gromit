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

func TestRecordOutcomeIncrementsSuccesses(t *testing.T) {
	// Verify that RecordOutcome increments successes when outcome is true
	state := &BanditState{
		Arms: []ArmState{
			{ID: "control", Successes: 5, Failures: 2},
			{ID: "variant-1", Successes: 3, Failures: 4},
		},
	}

	state.RecordOutcome("control", true)

	if state.Arms[0].Successes != 6 {
		t.Fatalf("Expected control successes to be 6, got %d", state.Arms[0].Successes)
	}
	if state.Arms[0].Failures != 2 {
		t.Fatalf("Expected control failures to remain 2, got %d", state.Arms[0].Failures)
	}
}

func TestRecordOutcomeIncrementsFailures(t *testing.T) {
	// Verify that RecordOutcome increments failures when outcome is false
	state := &BanditState{
		Arms: []ArmState{
			{ID: "control", Successes: 5, Failures: 2},
			{ID: "variant-1", Successes: 3, Failures: 4},
		},
	}

	state.RecordOutcome("variant-1", false)

	if state.Arms[1].Successes != 3 {
		t.Fatalf("Expected variant-1 successes to remain 3, got %d", state.Arms[1].Successes)
	}
	if state.Arms[1].Failures != 5 {
		t.Fatalf("Expected variant-1 failures to be 5, got %d", state.Arms[1].Failures)
	}
}

func TestIsConvergedDetectsHighConfidenceWinner(t *testing.T) {
	// When one arm is clearly better, IsConverged should return true
	state := &BanditState{
		Arms: []ArmState{
			{ID: "control", Successes: 10, Failures: 10},
			{ID: "variant-1", Successes: 100, Failures: 5},
		},
	}

	converged := state.IsConverged(0.95)
	if !converged {
		t.Fatalf("Expected IsConverged to be true when one arm is clearly better")
	}
}

func TestIsConvergedDetectsInconsistentData(t *testing.T) {
	// When arms are similar, IsConverged should return false
	state := &BanditState{
		Arms: []ArmState{
			{ID: "control", Successes: 10, Failures: 10},
			{ID: "variant-1", Successes: 11, Failures: 9},
		},
	}

	converged := state.IsConverged(0.95)
	if converged {
		t.Fatalf("Expected IsConverged to be false when arms are similar")
	}
}
