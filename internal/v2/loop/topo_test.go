package loop

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

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
