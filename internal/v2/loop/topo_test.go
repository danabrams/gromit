package loop

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestTopologicalSort_NilInput(t *testing.T) {
	t.Parallel()

	got, err := TopologicalSort(nil)
	if err != nil {
		t.Fatalf("TopologicalSort(nil): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d beads, want 0", len(got))
	}
}

func TestTopologicalSort_EmptyInput(t *testing.T) {
	t.Parallel()

	got, err := TopologicalSort([]*bead.Bead{})
	if err != nil {
		t.Fatalf("TopologicalSort(empty): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d beads, want 0", len(got))
	}
}

func TestTopologicalSort_BlockedByHonored(t *testing.T) {
	t.Parallel()

	// "consumer" is blocked_by "provider" — provider must come first.
	beads := []*bead.Bead{
		{ID: "consumer", BlockedBy: []bead.Dependency{{ID: "provider"}}},
		{ID: "provider"},
	}

	got, err := TopologicalSort(beads)
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d beads, want 2", len(got))
	}
	if got[0].ID != "provider" || got[1].ID != "consumer" {
		t.Fatalf("order = [%s %s], want [provider consumer]", got[0].ID, got[1].ID)
	}
}

func TestTopologicalSort_StableTieBreaking(t *testing.T) {
	t.Parallel()

	// Three independent beads — output must preserve input order.
	beads := []*bead.Bead{
		{ID: "c"},
		{ID: "a"},
		{ID: "b"},
	}

	got, err := TopologicalSort(beads)
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d beads, want 3", len(got))
	}
	want := []string{"c", "a", "b"}
	for i, w := range want {
		if got[i].ID != w {
			t.Fatalf("position %d = %q, want %q", i, got[i].ID, w)
		}
	}
}

func TestTopologicalSort_CycleReturnsError(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "a", DependsOn: []bead.Dependency{{ID: "b"}}},
		{ID: "b", DependsOn: []bead.Dependency{{ID: "a"}}},
	}

	_, err := TopologicalSort(beads)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
}

func TestTopologicalSort_DependencyBeforeDependent(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "child", DependsOn: []bead.Dependency{{ID: "root"}}},
		{ID: "root"},
	}

	got, err := TopologicalSort(beads)
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d beads, want 2", len(got))
	}
	if got[0].ID != "root" || got[1].ID != "child" {
		t.Fatalf("order = [%s %s], want [root child]", got[0].ID, got[1].ID)
	}
}

func TestTopologicalSort_DependenciesFieldHonored(t *testing.T) {
	t.Parallel()

	// Uses the Dependencies field (not DependsOn or BlockedBy) — all three must be collected.
	beads := []*bead.Bead{
		{ID: "child", Dependencies: []bead.Dependency{{ID: "root"}}},
		{ID: "root"},
	}

	got, err := TopologicalSort(beads)
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d beads, want 2", len(got))
	}
	if got[0].ID != "root" || got[1].ID != "child" {
		t.Fatalf("order = [%s %s], want [root child]", got[0].ID, got[1].ID)
	}
}

func TestTopologicalSort_ExternalDepTreatedAsSatisfied(t *testing.T) {
	t.Parallel()

	// "b" depends on "external" which is not in the input set.
	// The sort must not error and must include both beads.
	beads := []*bead.Bead{
		{ID: "b", DependsOn: []bead.Dependency{{ID: "external"}}},
		{ID: "a"},
	}

	got, err := TopologicalSort(beads)
	if err != nil {
		t.Fatalf("TopologicalSort: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d beads, want 2", len(got))
	}
}

func TestTopologicalSort_DuplicateIDReturnsError(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "dup"},
		{ID: "dup"},
	}

	_, err := TopologicalSort(beads)
	if err == nil {
		t.Fatal("expected error for duplicate bead ID, got nil")
	}
	if got := err.Error(); got != `topological sort: duplicate bead ID "dup"` {
		t.Fatalf("unexpected error message: %s", got)
	}
}

func TestTopologicalSort_ChainOrderingThreeLevels(t *testing.T) {
	t.Parallel()

	// a depends on b, b depends on c → order must be c, b, a.
	beads := []*bead.Bead{
		{ID: "a", DependsOn: []bead.Dependency{{ID: "b"}}},
		{ID: "b", DependsOn: []bead.Dependency{{ID: "c"}}},
		{ID: "c"},
	}

	got, err := TopologicalSort(beads)
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	want := []string{"c", "b", "a"}
	if len(got) != len(want) {
		t.Fatalf("got %d beads, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].ID != w {
			t.Fatalf("position %d = %q, want %q", i, got[i].ID, w)
		}
	}
}
