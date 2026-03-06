package dep

import (
	"testing"
)

func TestAdd_RegistersSingleBead(t *testing.T) {
	r := NewResolver()
	r.Add("bead1", nil)

	// After adding a bead with no dependencies, it should be selectable
	next, err := r.Next(nil)
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if next != "bead1" {
		t.Errorf("expected bead1, got %s", next)
	}
}
