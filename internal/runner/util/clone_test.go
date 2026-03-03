package util

import "testing"

func TestCloneStringSliceReturnsNilForNilInput(t *testing.T) {
	if got := CloneStringSlice(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestCloneStringSliceCreatesIndependentCopy(t *testing.T) {
	src := []string{"alpha", "beta"}
	clone := CloneStringSlice(src)

	if len(clone) != len(src) {
		t.Fatalf("expected clone length %d, got %d", len(src), len(clone))
	}

	for i := range src {
		if clone[i] != src[i] {
			t.Fatalf("expected clone[%d] == %q, got %q", i, src[i], clone[i])
		}
	}

	clone[0] = "changed"
	if src[0] == "changed" {
		t.Fatalf("modifying clone should not mutate source")
	}
}

func TestCloneStringSliceEmptyNonNil(t *testing.T) {
	empty := make([]string, 0)
	if CloneStringSlice(empty) == nil {
		t.Fatalf("expected non-nil clone for non-nil empty slice")
	}
}
