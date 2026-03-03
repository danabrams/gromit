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
	expectedPrefix := "spec:"
	if specLabelPrefix != expectedPrefix {
		t.Fatalf("expected specLabelPrefix to equal %q, got %q", expectedPrefix, specLabelPrefix)
	}
	if SpecLabelPrefix != expectedPrefix {
		t.Fatalf("expected exported SpecLabelPrefix to equal %q, got %q", expectedPrefix, SpecLabelPrefix)
	}
}
