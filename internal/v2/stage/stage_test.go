package stage

import "testing"

func TestStageRequestConstruction(t *testing.T) {
	req := StageRequest{
		Bead: BeadInfo{ID: "spec"},
	}

	if req.Bead.ID != "spec" {
		t.Fatalf("expected bead ID to be spec, got %s", req.Bead.ID)
	}
}

func TestDecisionStrings(t *testing.T) {
	expected := map[Decision]string{
		DecisionProceed: "proceed",
		DecisionSkip:    "skip",
		DecisionBlock:   "block",
		DecisionFail:    "fail",
	}

	for dec, want := range expected {
		got, ok := decisionStrings[dec]
		if !ok {
			t.Fatalf("decision strings missing entry for %v", dec)
		}
		if got != want {
			t.Fatalf("expected %v string to be %q, got %q", dec, want, got)
		}
		if dec.String() != want {
			t.Fatalf("expected %v.String() to be %q, got %q", dec, want, dec.String())
		}
	}
}
