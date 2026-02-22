package runner

import (
	"testing"
)

func TestExtractRequirementsFromDescription_NumberedList(t *testing.T) {
	input := "Some preamble\n1. do this\n2. do that\n3. do the other thing"
	got := extractRequirementsFromDescription(input)
	want := []string{"do this", "do that", "do the other thing"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
