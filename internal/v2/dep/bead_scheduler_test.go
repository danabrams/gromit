package dep

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestBeadScheduler_SingleBeadNoDeps(t *testing.T) {
	beads := []*bead.Bead{
		{ID: "solo"},
	}
	s := NewBeadScheduler(beads)

	got := s.Next()
	if got == nil || got.ID != "solo" {
		t.Fatalf("Next() = %v, want bead with ID 'solo'", got)
	}
}

func TestBeadScheduler_MultipleDeps_CorrectOrdering(t *testing.T) {
	beads := []*bead.Bead{
		{ID: "leaf"},
		{ID: "mid", DependsOn: []bead.Dependency{{ID: "leaf"}}},
		{ID: "root", DependsOn: []bead.Dependency{{ID: "mid"}}},
	}
	s := NewBeadScheduler(beads)

	want := []string{"leaf", "mid", "root"}
	for i, wantID := range want {
		got := s.Next()
		if got == nil {
			t.Fatalf("step %d: Next() = nil, want %q", i, wantID)
		}
		if got.ID != wantID {
			t.Fatalf("step %d: Next().ID = %q, want %q", i, got.ID, wantID)
		}
		s.MarkComplete(got.ID)
	}
}

func TestBeadScheduler_MarkComplete_UnblocksDependent(t *testing.T) {
	beads := []*bead.Bead{
		{ID: "blocker"},
		{ID: "blocked", DependsOn: []bead.Dependency{{ID: "blocker"}}},
	}
	s := NewBeadScheduler(beads)

	// Before completing blocker, Next should return blocker.
	got := s.Next()
	if got == nil || got.ID != "blocker" {
		t.Fatalf("Next() = %v, want blocker", got)
	}

	// Without MarkComplete, blocked should not be returned.
	// Next should return blocker again since it's not completed.
	got = s.Next()
	if got == nil || got.ID != "blocker" {
		t.Fatalf("Next() before MarkComplete = %v, want blocker again", got)
	}

	s.MarkComplete("blocker")

	got = s.Next()
	if got == nil || got.ID != "blocked" {
		t.Fatalf("Next() after MarkComplete = %v, want blocked", got)
	}
}

func TestBeadScheduler_Next_ReturnsNilWhenAllCompleted(t *testing.T) {
	beads := []*bead.Bead{
		{ID: "a"},
		{ID: "b"},
	}
	s := NewBeadScheduler(beads)

	s.MarkComplete("a")
	s.MarkComplete("b")

	if got := s.Next(); got != nil {
		t.Fatalf("Next() = %v, want nil when all beads completed", got)
	}
}

func TestBeadScheduler_Next_ReturnsNilWhenAllBlocked(t *testing.T) {
	beads := []*bead.Bead{
		{ID: "a", DependsOn: []bead.Dependency{{ID: "external"}}},
		{ID: "b", DependsOn: []bead.Dependency{{ID: "external"}}},
	}
	s := NewBeadScheduler(beads)

	// Both beads depend on "external" which is never completed.
	if got := s.Next(); got != nil {
		t.Fatalf("Next() = %v, want nil when all beads are blocked", got)
	}
}

func TestBeadScheduler_EmptyBeadList(t *testing.T) {
	s := NewBeadScheduler([]*bead.Bead{})

	if got := s.Next(); got != nil {
		t.Fatalf("Next() = %v, want nil for empty bead list", got)
	}
}

func TestBeadScheduler_NilBeadsInList(t *testing.T) {
	beads := []*bead.Bead{
		nil,
		{ID: "valid"},
		nil,
	}
	s := NewBeadScheduler(beads)

	got := s.Next()
	if got == nil || got.ID != "valid" {
		t.Fatalf("Next() = %v, want bead with ID 'valid'", got)
	}

	s.MarkComplete("valid")

	if got := s.Next(); got != nil {
		t.Fatalf("Next() after completing valid = %v, want nil", got)
	}
}

func TestBeadScheduler_CircularDependencies(t *testing.T) {
	// The scheduler does not detect cycles — it simply returns nil
	// when all remaining beads are blocked (which is the effect of a cycle).
	beads := []*bead.Bead{
		{ID: "a", DependsOn: []bead.Dependency{{ID: "b"}}},
		{ID: "b", DependsOn: []bead.Dependency{{ID: "a"}}},
	}
	s := NewBeadScheduler(beads)

	if got := s.Next(); got != nil {
		t.Fatalf("Next() = %v, want nil for circular dependencies", got)
	}
}

func TestBeadScheduler_DependenciesField(t *testing.T) {
	// Dependencies (not DependsOn) should also block scheduling.
	beads := []*bead.Bead{
		{ID: "dep"},
		{ID: "main", Dependencies: []bead.Dependency{{ID: "dep"}}},
	}
	s := NewBeadScheduler(beads)

	got := s.Next()
	if got == nil || got.ID != "dep" {
		t.Fatalf("Next() = %v, want dep", got)
	}

	s.MarkComplete("dep")

	got = s.Next()
	if got == nil || got.ID != "main" {
		t.Fatalf("Next() after dep complete = %v, want main", got)
	}
}

func TestBeadScheduler_BlockedByField(t *testing.T) {
	// BlockedBy should also block scheduling.
	beads := []*bead.Bead{
		{ID: "blocker"},
		{ID: "blocked", BlockedBy: []bead.Dependency{{ID: "blocker"}}},
	}
	s := NewBeadScheduler(beads)

	got := s.Next()
	if got == nil || got.ID != "blocker" {
		t.Fatalf("Next() = %v, want blocker", got)
	}

	s.MarkComplete("blocker")

	got = s.Next()
	if got == nil || got.ID != "blocked" {
		t.Fatalf("Next() after blocker complete = %v, want blocked", got)
	}
}

func TestBeadScheduler_MarkComplete_EmptyID(t *testing.T) {
	beads := []*bead.Bead{
		{ID: "a"},
	}
	s := NewBeadScheduler(beads)

	// MarkComplete with empty ID should be a no-op.
	s.MarkComplete("")

	got := s.Next()
	if got == nil || got.ID != "a" {
		t.Fatalf("Next() = %v, want a (empty MarkComplete should be no-op)", got)
	}
}

func TestBeadScheduler_DoesNotMutateInput(t *testing.T) {
	original := []*bead.Bead{
		{ID: "a"},
		{ID: "b"},
	}
	s := NewBeadScheduler(original)

	s.MarkComplete("a")
	s.MarkComplete("b")

	// The original slice should be unchanged and still contain both beads.
	if len(original) != 2 {
		t.Fatalf("original slice length = %d, want 2", len(original))
	}
	if original[0].ID != "a" || original[1].ID != "b" {
		t.Fatalf("original slice was mutated")
	}
}

func TestBeadScheduler_DiamondDependency(t *testing.T) {
	// Diamond: D depends on B and C, both depend on A.
	beads := []*bead.Bead{
		{ID: "A"},
		{ID: "B", DependsOn: []bead.Dependency{{ID: "A"}}},
		{ID: "C", DependsOn: []bead.Dependency{{ID: "A"}}},
		{ID: "D", DependsOn: []bead.Dependency{{ID: "B"}, {ID: "C"}}},
	}
	s := NewBeadScheduler(beads)

	// Step 1: A is the only one without deps.
	got := s.Next()
	if got == nil || got.ID != "A" {
		t.Fatalf("step 1: Next() = %v, want A", got)
	}
	s.MarkComplete("A")

	// Step 2: B and C are both ready; scheduler returns first in list order.
	got = s.Next()
	if got == nil || got.ID != "B" {
		t.Fatalf("step 2: Next() = %v, want B", got)
	}
	s.MarkComplete("B")

	// Step 3: C is ready, D still blocked on C.
	got = s.Next()
	if got == nil || got.ID != "C" {
		t.Fatalf("step 3: Next() = %v, want C", got)
	}
	s.MarkComplete("C")

	// Step 4: D is now unblocked.
	got = s.Next()
	if got == nil || got.ID != "D" {
		t.Fatalf("step 4: Next() = %v, want D", got)
	}
	s.MarkComplete("D")

	if got := s.Next(); got != nil {
		t.Fatalf("after all complete: Next() = %v, want nil", got)
	}
}
