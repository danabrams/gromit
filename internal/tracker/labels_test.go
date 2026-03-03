package tracker

import "testing"

func TestSpecLabelFor(t *testing.T) {
	specName := "auth"
	want := SpecLabelPrefix + specName
	if got := SpecLabelFor(specName); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
