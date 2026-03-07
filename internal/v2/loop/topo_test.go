package loop

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

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
