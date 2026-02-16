package andon

import "testing"

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
