package dep

import (
	"reflect"
	"strings"
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

func TestNext_CycleErrorReportsCyclePath(t *testing.T) {
	r := NewResolver()
	r.Add("bead1", []string{"bead2"})
	r.Add("bead2", []string{"bead3"})
	r.Add("bead3", []string{"bead1"})

	_, err := r.Next(nil)
	if err == nil {
		t.Fatalf("expected cycle detection error, got nil")
	}

	cycleErr, ok := err.(*CycleError)
	if !ok {
		t.Fatalf("expected *CycleError, got %T", err)
	}

	wantPath := []string{"bead1", "bead2", "bead3", "bead1"}
	if !reflect.DeepEqual(cycleErr.Path, wantPath) {
		t.Fatalf("expected cycle path %v, got %v", wantPath, cycleErr.Path)
	}

	if !strings.Contains(err.Error(), "cycle detected") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestNext_DeterministicOrderingWhenMultipleBeadsEligible(t *testing.T) {
	r := NewResolver()
	// Add beads in random order to verify deterministic selection.
	r.Add("zebra", nil)
	r.Add("apple", nil)
	r.Add("middle", nil)

	// All three have no dependencies; the resolver should pick the alphabetically
	// smallest eligible bead first.
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

	want := []string{"apple", "middle", "zebra"}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("expected deterministic alphabetical order %v, got %v", want, results)
	}

	// Repeat multiple times to make sure the ordering stays consistent.
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

		if !reflect.DeepEqual(results2, want) {
			t.Fatalf("Trial %d: expected ordering %v, got %v", trial, want, results2)
		}
	}
}

func TestNext_UnregisteredDependencyReturnsError(t *testing.T) {
	r := NewResolver()
	r.Add("bead1", []string{"phantom"})

	// "phantom" was never Add()-ed, so bead1's dependency can never be satisfied
	_, err := r.Next(nil)
	if err == nil {
		t.Fatal("expected error for unregistered dependency, got nil")
	}
	if !strings.Contains(err.Error(), "unsatisfiable") {
		t.Fatalf("error should mention unsatisfiable, got: %v", err)
	}
	if !strings.Contains(err.Error(), "phantom") {
		t.Fatalf("error should mention the unregistered dep 'phantom', got: %v", err)
	}
}

func TestNext_BeadDependingOnNeverAddedBeadReturnsError(t *testing.T) {
	r := NewResolver()
	r.Add("bead1", nil)
	r.Add("bead2", []string{"bead1", "never-added"})

	// Complete bead1 so bead2 becomes the only pending bead
	_, err := r.Next([]string{"bead1"})
	if err == nil {
		t.Fatal("expected error for dependency on never-added bead, got nil")
	}
	if !strings.Contains(err.Error(), "unsatisfiable") {
		t.Fatalf("error should mention unsatisfiable, got: %v", err)
	}
	if !strings.Contains(err.Error(), "never-added") {
		t.Fatalf("error should mention 'never-added', got: %v", err)
	}
}

func TestNext_TieBreakingUsesAlphabeticalOrder(t *testing.T) {
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

	want := []string{"apple", "middle", "zebra"}
	if !reflect.DeepEqual(collected, want) {
		t.Fatalf("expected alphabetical ordering %v, got %v", want, collected)
	}
}

func TestReplaceDependency_RewiresDownstreamBeads(t *testing.T) {
	r := NewResolver()
	r.Add("a", nil)
	r.Add("b", []string{"a"})
	r.Add("c", []string{"b"})

	// Decompose "b" into "b1" and "b2"
	r.Add("b1", []string{"a"})
	r.Add("b2", []string{"a"})
	r.ReplaceDependency("b", []string{"b1", "b2"})

	// Complete "a", then "b" (it was decomposed, marked completed)
	completed := []string{"a", "b"}

	// b1 and b2 should be available (both depend on completed "a")
	next, err := r.Next(completed)
	if err != nil {
		t.Fatal(err)
	}
	if next != "b1" && next != "b2" {
		t.Fatalf("expected b1 or b2, got %s", next)
	}
	completed = append(completed, next)

	next2, err := r.Next(completed)
	if err != nil {
		t.Fatal(err)
	}
	if next2 == next || (next2 != "b1" && next2 != "b2") {
		t.Fatalf("expected the other sub-bead, got %s", next2)
	}
	completed = append(completed, next2)

	// Now c should be available (depends on b1 and b2, both completed)
	next3, err := r.Next(completed)
	if err != nil {
		t.Fatal(err)
	}
	if next3 != "c" {
		t.Fatalf("expected c after sub-beads complete, got %s", next3)
	}
}

func TestReplaceDependency_NoOpForEmptyArgs(t *testing.T) {
	r := NewResolver()
	r.Add("a", nil)
	r.Add("b", []string{"a"})

	// Empty oldID and empty newIDs should not modify anything
	r.ReplaceDependency("", []string{"x"})
	r.ReplaceDependency("a", nil)

	next, err := r.Next(nil)
	if err != nil {
		t.Fatal(err)
	}
	if next != "a" {
		t.Fatalf("expected a, got %s", next)
	}
}
