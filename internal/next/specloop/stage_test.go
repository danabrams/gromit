package specloop

import "testing"

func TestNextAction_ContinueIsDefault(t *testing.T) {
	na := NextAction{}
	if na.Kind != Continue {
		t.Fatalf("want Continue, got %v", na.Kind)
	}
}

func TestActionKind_String(t *testing.T) {
	cases := map[ActionKind]string{
		Continue:   "continue",
		ReplanFrom: "replan_from",
		NeedsHuman: "needs_human",
		Blocked:    "blocked",
	}
	for k, want := range cases {
		if k.String() != want {
			t.Fatalf("want %s, got %s", want, k.String())
		}
	}
}

func TestFailureContext_NormalizeNilFields(t *testing.T) {
	fc := FailureContext{Cycle: 1}
	if fc.Failures != nil {
		t.Fatal("precondition: Failures should be nil before normalization")
	}
	if fc.EscalatedFailures != nil {
		t.Fatal("precondition: EscalatedFailures should be nil before normalization")
	}
	fc.NormalizeNilFields()
	if fc.Failures == nil {
		t.Fatal("Failures should be non-nil after normalization")
	}
	if len(fc.Failures) != 0 {
		t.Fatalf("Failures should be empty, got %d elements", len(fc.Failures))
	}
	if fc.EscalatedFailures == nil {
		t.Fatal("EscalatedFailures should be non-nil after normalization")
	}
	if len(fc.EscalatedFailures) != 0 {
		t.Fatalf("EscalatedFailures should be empty, got %d elements", len(fc.EscalatedFailures))
	}
}

func TestFailureContext_NormalizeNilFields_PreservesExisting(t *testing.T) {
	fc := FailureContext{
		Failures:          []string{"test failed"},
		EscalatedFailures: []string{"placeholder"},
		Cycle:             2,
	}
	fc.NormalizeNilFields()
	if len(fc.Failures) != 1 {
		t.Fatalf("expected 1 failure preserved, got %d", len(fc.Failures))
	}
	if fc.Failures[0] != "test failed" {
		t.Fatalf("expected preserved failure, got %q", fc.Failures[0])
	}
	if len(fc.EscalatedFailures) != 1 {
		t.Fatalf("expected 1 escalated failure preserved, got %d", len(fc.EscalatedFailures))
	}
	if fc.EscalatedFailures[0] != "placeholder" {
		t.Fatalf("expected preserved escalated failure, got %q", fc.EscalatedFailures[0])
	}
}
