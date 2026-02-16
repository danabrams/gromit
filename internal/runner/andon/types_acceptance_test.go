//go:build acceptance

package andon

import (
	"reflect"
	"testing"
	"time"
)

// smoke-matrix: move | rationale: Catalog ordering/labels are deterministic data checks already covered in unit tests without end-to-end wiring. | destination: internal/runner/andon/types_test.go:TestAllFailureClasses_CanonicalOrderAndLabels
func TestFailureClasses_CanonicalCatalog(t *testing.T) {
	got := AllFailureClasses()
	want := []FailureClass{
		FailureClassTransient,
		FailureClassWorkflow,
		FailureClassQuality,
		FailureClassIntent,
		FailureClassData,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllFailureClasses() = %v, want %v", got, want)
	}

	wantLabels := map[FailureClass]string{
		FailureClassTransient: "Transient",
		FailureClassWorkflow:  "Workflow",
		FailureClassQuality:   "Quality",
		FailureClassIntent:    "Intent",
		FailureClassData:      "Data",
	}
	for _, class := range got {
		if string(class) != wantLabels[class] {
			t.Fatalf("failure class %v label = %q, want %q", class, string(class), wantLabels[class])
		}
	}
}

// smoke-matrix: move | rationale: Level catalog validation is a pure data invariant that belongs in unit coverage, not acceptance flow. | destination: internal/runner/andon/types_test.go:TestAllAndonLevels_CanonicalOrder
func TestLevels_CanonicalCatalog(t *testing.T) {
	got := AllAndonLevels()
	want := []AndonLevel{LevelL1, LevelL2, LevelL3, LevelL4}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllAndonLevels() = %v, want %v", got, want)
	}
}

// smoke-matrix: move | rationale: Default threshold purity and policy consumption are deterministic and exercised via unit tests. | destination: internal/runner/andon/types_test.go:TestDefaultThresholdDefinition_IsPureAndPolicyConsumable
func TestDefaultThresholdDefinition_IsPureAndPolicyConsumable_Acceptance(t *testing.T) {
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
