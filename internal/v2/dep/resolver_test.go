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

func TestNext_WaitsForDependencies(t *testing.T) {
	r := NewResolver()
	r.Add("bead1", nil)
	r.Add("bead2", []string{"bead1"})

	// bead1 has no dependencies, should be returned first
	next, err := r.Next(nil)
	if err != nil {
		t.Fatalf("First Next() failed: %v", err)
	}
	if next != "bead1" {
		t.Errorf("expected bead1 first, got %s", next)
	}

	// bead2 depends on bead1, should be returned after bead1 is completed
	next, err = r.Next([]string{"bead1"})
	if err != nil {
		t.Fatalf("Second Next() failed: %v", err)
	}
	if next != "bead2" {
		t.Errorf("expected bead2 after bead1 completed, got %s", next)
	}
}

func TestNext_DetectsCycles(t *testing.T) {
	r := NewResolver()
	r.Add("bead1", []string{"bead2"})
	r.Add("bead2", []string{"bead1"})

	// When there's a cycle, Next should return an error
	_, err := r.Next(nil)
	if err == nil {
		t.Errorf("expected cycle detection error, got nil")
	}
	if err.Error() == "" {
		t.Errorf("error message should not be empty")
	}
}
