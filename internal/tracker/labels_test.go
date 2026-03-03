package tracker

import "testing"

func TestSpecLabelFor(t *testing.T) {
	specName := "auth"
	want := SpecLabelPrefix + specName
	if got := SpecLabelFor(specName); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSpecLabelPrefixConstant(t *testing.T) {
	if specLabelPrefix != "spec:" {
		t.Fatalf("expected specLabelPrefix to equal \"spec:\", got %q", specLabelPrefix)
	}
	if SpecLabelPrefix != specLabelPrefix {
		t.Fatalf("expected exported SpecLabelPrefix to re-use specLabelPrefix, got %q", SpecLabelPrefix)
	}
}
