package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestProjectBeadCompletionOrder_RespectsDependenciesAndReadiness(t *testing.T) {
	all := []*bead.Bead{
		{ID: "d", Status: "in_progress", Priority: 4, Labels: []string{"spec:gamma"}, Title: "in progress"},
		{ID: "a", Status: "open", Priority: 1, Labels: []string{"spec:alpha"}, Title: "alpha"},
		{ID: "b", Status: "open", Priority: 0, Labels: []string{"spec:alpha"}, Title: "depends on a", DependsOn: []bead.Dependency{{ID: "a"}}},
		{ID: "c", Status: "open", Priority: 2, Labels: []string{"spec:beta"}, Title: "ready"},
	}
	ready := []*bead.Bead{
		{ID: "c"},
	}

	order := projectBeadCompletionOrder(ready, all)
	if len(order) != 4 {
		t.Fatalf("len(order) = %d, want 4", len(order))
	}
	got := []string{order[0].ID, order[1].ID, order[2].ID, order[3].ID}
	want := []string{"d", "c", "a", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full order=%v)", i, got[i], want[i], got)
		}
	}
}

func TestProjectSpecCompletionOrder_SortsByCompletionPoint(t *testing.T) {
	all := []*bead.Bead{
		{ID: "a1", Labels: []string{"spec:alpha"}},
		{ID: "a2", Labels: []string{"spec:alpha"}},
		{ID: "b1", Labels: []string{"spec:beta"}},
	}
	order := []*bead.Bead{
		{ID: "a1", Labels: []string{"spec:alpha"}},
		{ID: "b1", Labels: []string{"spec:beta"}},
		{ID: "a2", Labels: []string{"spec:alpha"}},
	}

	completions := projectSpecCompletionOrder(order, all)
	if len(completions) != 2 {
		t.Fatalf("len(completions) = %d, want 2", len(completions))
	}
	if completions[0].Name != "beta" || completions[0].CompletionIndex != 2 {
		t.Fatalf("completions[0] = %+v, want beta@2", completions[0])
	}
	if completions[1].Name != "alpha" || completions[1].CompletionIndex != 3 {
		t.Fatalf("completions[1] = %+v, want alpha@3", completions[1])
	}
}
