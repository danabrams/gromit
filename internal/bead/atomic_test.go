package bead

import "testing"

func TestIsAtomic_AtomicTrueLabel(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID:     "test-bead",
		Labels: []string{"atomic:true"},
	}

	got := IsAtomic(bead, 0, 5)
	if !got {
		t.Errorf("IsAtomic() = %v, want true for atomic:true label", got)
	}
}
