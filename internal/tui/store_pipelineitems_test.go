package tui

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
)

func TestStorePipelineItemsGetNormalizesNilFields(t *testing.T) {
	store := &Store{}
	store.PipelineItems = PipelineItems{
		UnplannedSpecs: []string{"spec-alpha"},
	}

	items := store.GetPipelineItems()

	if items.BacklogIdeas == nil {
		t.Fatal("expected BacklogIdeas to be a slice, got nil")
	}
	if items.UndecomposedPlans == nil {
		t.Fatal("expected UndecomposedPlans to be a slice, got nil")
	}
	if items.Beads == nil {
		t.Fatal("expected Beads to be a slice, got nil")
	}
	if len(items.UnplannedSpecs) != 1 || items.UnplannedSpecs[0] != "spec-alpha" {
		t.Fatalf("unexpected UnplannedSpecs = %v", items.UnplannedSpecs)
	}
}

func TestStorePipelineItemsGetHandlesNilStore(t *testing.T) {
	var store *Store

	items := store.GetPipelineItems()
	want := normalizePipelineItems(PipelineItems{})

	if !reflect.DeepEqual(items, want) {
		t.Fatalf("pipeline items = %+v, want %+v", items, want)
	}
}

func TestStorePipelineItemsSetStoresIndependentCopies(t *testing.T) {
	store := &Store{}

	specs := make([]string, 1, 2)
	specs[0] = "spec-alpha"
	plans := make([]string, 1, 2)
	plans[0] = "plan-alpha"
	beads := make([]bead.Bead, 1, 2)
	beads[0] = bead.Bead{ID: "bead-alpha"}
	ideas := make([]backlog.Idea, 1, 2)
	ideas[0] = backlog.Idea{ID: "idea-alpha"}

	items := PipelineItems{
		BacklogIdeas:      ideas,
		UnplannedSpecs:    specs,
		UndecomposedPlans: plans,
		Beads:             beads,
	}

	store.SetPipelineItems(items)

	items.UnplannedSpecs[0] = "spec-beta"
	items.UnplannedSpecs = append(items.UnplannedSpecs, "spec-gamma")

	items.UndecomposedPlans[0] = "plan-beta"
	items.UndecomposedPlans = append(items.UndecomposedPlans, "plan-gamma")

	items.Beads[0].ID = "bead-beta"
	items.Beads = append(items.Beads, bead.Bead{ID: "bead-gamma"})

	items.BacklogIdeas[0].ID = "idea-beta"
	items.BacklogIdeas = append(items.BacklogIdeas, backlog.Idea{ID: "idea-gamma"})

	gots := store.GetPipelineItems()

	if len(gots.UnplannedSpecs) != 1 || gots.UnplannedSpecs[0] != "spec-alpha" {
		t.Fatalf("UnplannedSpecs mutated: %v", gots.UnplannedSpecs)
	}
	if len(gots.UndecomposedPlans) != 1 || gots.UndecomposedPlans[0] != "plan-alpha" {
		t.Fatalf("UndecomposedPlans mutated: %v", gots.UndecomposedPlans)
	}
	if len(gots.Beads) != 1 || gots.Beads[0].ID != "bead-alpha" {
		t.Fatalf("Beads mutated: %v", gots.Beads)
	}
	if len(gots.BacklogIdeas) != 1 || gots.BacklogIdeas[0].ID != "idea-alpha" {
		t.Fatalf("BacklogIdeas mutated: %v", gots.BacklogIdeas)
	}
}

func TestStorePipelineItemsGetReturnsIndependentCopy(t *testing.T) {
	store := &Store{}
	store.PipelineItems = PipelineItems{
		BacklogIdeas:      []backlog.Idea{{ID: "idea-alpha"}},
		UnplannedSpecs:    []string{"spec-alpha"},
		UndecomposedPlans: []string{"plan-alpha"},
		Beads:             []bead.Bead{{ID: "bead-alpha"}},
	}

	first := store.GetPipelineItems()
	first.BacklogIdeas[0].ID = "idea-beta"
	first.BacklogIdeas = append(first.BacklogIdeas, backlog.Idea{ID: "idea-gamma"})

	first.UnplannedSpecs[0] = "spec-beta"
	first.UnplannedSpecs = append(first.UnplannedSpecs, "spec-gamma")

	first.UndecomposedPlans[0] = "plan-beta"
	first.UndecomposedPlans = append(first.UndecomposedPlans, "plan-gamma")

	first.Beads[0].ID = "bead-beta"
	first.Beads = append(first.Beads, bead.Bead{ID: "bead-gamma"})

	second := store.GetPipelineItems()

	if len(second.BacklogIdeas) != 1 || second.BacklogIdeas[0].ID != "idea-alpha" {
		t.Fatalf("BacklogIdeas shared while copying: %v", second.BacklogIdeas)
	}
	if len(second.UnplannedSpecs) != 1 || second.UnplannedSpecs[0] != "spec-alpha" {
		t.Fatalf("UnplannedSpecs shared while copying: %v", second.UnplannedSpecs)
	}
	if len(second.UndecomposedPlans) != 1 || second.UndecomposedPlans[0] != "plan-alpha" {
		t.Fatalf("UndecomposedPlans shared while copying: %v", second.UndecomposedPlans)
	}
	if len(second.Beads) != 1 || second.Beads[0].ID != "bead-alpha" {
		t.Fatalf("Beads shared while copying: %v", second.Beads)
	}
}
