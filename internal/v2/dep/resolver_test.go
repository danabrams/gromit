package dep

import (
	"reflect"
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

func TestNext_DeterministicOrderingWhenMultipleBeadsEligible(t *testing.T) {
	r := NewResolver()
	// Add beads in random order
	r.Add("zebra", nil)
	r.Add("apple", nil)
	r.Add("middle", nil)

	// All three have no dependencies, should return in sorted order
	results := []string{}
	for len(results) < 3 {
		next, err := r.Next(results)
		if err != nil {
			t.Fatalf("Next() failed: %v", err)
		}
		if next == "" {
			t.Fatalf("expected bead, got empty string")
		}
		results = append(results, next)
	}

	// Run multiple times to ensure consistency
	for trial := 0; trial < 3; trial++ {
		r2 := NewResolver()
		r2.Add("zebra", nil)
		r2.Add("apple", nil)
		r2.Add("middle", nil)

		results2 := []string{}
		for len(results2) < 3 {
			next, err := r2.Next(results2)
			if err != nil {
				t.Fatalf("Trial %d: Next() failed: %v", trial, err)
			}
			results2 = append(results2, next)
		}

		// Should match first run
		if len(results) != len(results2) {
			t.Fatalf("Trial %d: length mismatch", trial)
		}
		for i, expected := range results {
			if results2[i] != expected {
				t.Errorf("Trial %d: expected %s at position %d, got %s", trial, expected, i, results2[i])
			}
		}
	}
}

func TestNext_RespectsAdditionOrderTieBreaking(t *testing.T) {
	r := NewResolver()
	r.Add("zebra", nil)
	r.Add("apple", nil)
	r.Add("middle", nil)

	collected := []string{}
	for len(collected) < 3 {
		next, err := r.Next(collected)
		if err != nil {
			t.Fatalf("Next() failed: %v", err)
		}
		if next == "" {
			t.Fatalf("expected bead, got empty string")
		}
		collected = append(collected, next)
	}

	want := []string{"zebra", "apple", "middle"}
	if !reflect.DeepEqual(collected, want) {
		t.Fatalf("expected %v, got %v", want, collected)
	}
}
