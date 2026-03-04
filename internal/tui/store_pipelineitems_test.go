package tui

import "testing"

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
