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
