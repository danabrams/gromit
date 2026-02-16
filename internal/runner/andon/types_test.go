package andon

import (
	"testing"
	"time"
)

func TestAllFailureClasses_CanonicalOrderAndLabels(t *testing.T) {
	got := AllFailureClasses()
	want := []FailureClass{
		FailureClassTransient,
		FailureClassWorkflow,
		FailureClassQuality,
		FailureClassIntent,
		FailureClassData,
	}

	if len(got) != len(want) {
		t.Fatalf("len(AllFailureClasses()) = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllFailureClasses()[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	wantLabels := []string{"Transient", "Workflow", "Quality", "Intent", "Data"}
	for i := range got {
		if string(got[i]) != wantLabels[i] {
			t.Fatalf("string(AllFailureClasses()[%d]) = %q, want %q", i, string(got[i]), wantLabels[i])
		}
	}
}

func TestAllAndonLevels_CanonicalOrder(t *testing.T) {
	got := AllAndonLevels()
	want := []AndonLevel{LevelL1, LevelL2, LevelL3, LevelL4}

	if len(got) != len(want) {
		t.Fatalf("len(AllAndonLevels()) = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllAndonLevels()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDefaultThresholdDefinition_IsPureAndPolicyConsumable(t *testing.T) {
	first := DefaultThresholdDefinition()
	first.L1MaxRetries = 99
	first.L1MaxDuration = 99 * time.Minute

	second := DefaultThresholdDefinition()
	if second.L1MaxRetries == 99 || second.L1MaxDuration == 99*time.Minute {
		t.Fatalf("DefaultThresholdDefinition should return fresh defaults, got %+v", second)
	}

	now := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)
	state := RecoveryState{
		Class:      FailureClassTransient,
		Level:      LevelL1,
		L1Attempts: second.L1MaxRetries,
		L1Started:  now,
	}

	decision := ChooseNextAction(state, second, now)
	if decision.NextLevel != LevelL2 || decision.Action != DecisionEscalate {
		t.Fatalf("ChooseNextAction(..., DefaultThresholdDefinition(), ...) = %+v, want L2/escalate", decision)
	}
}
