package bead

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/tracker"
)

func TestTrackerItemToBead(t *testing.T) {
	labelsJSON, _ := json.Marshal([]string{"spec:alpha", "build_strategy:medium"})
	outputsJSON, _ := json.Marshal([]string{"file1.go", "file2.md"})
	depsJSON, _ := json.Marshal([]Dependency{{ID: "dep-1", Title: "Dependency"}})
	item := &tracker.Item{
		ID:          "bead-123",
		Title:       "Feature work",
		Description: "Description",
		Status:      "open",
		Metadata: map[string]string{
			"priority":           "2",
			"labels":             string(labelsJSON),
			"expected_outputs":   string(outputsJSON),
			"parent":             "parent-1",
			"type":               "task",
			"owner":              "alice",
			"close_reason":       "done",
			"acceptance_criteria": "criteria",
			"dependencies":       string(depsJSON),
			"dependency_count":   "5",
			"dependent_count":    "3",
		},
	}

	bead, err := TrackerItemToBead(item)
	if err != nil {
		t.Fatalf("TrackerItemToBead returned error: %v", err)
	}
	if bead == nil {
		t.Fatal("TrackerItemToBead returned nil")
	}
	if bead.ID != item.ID {
		t.Fatalf("bead ID = %q, want %q", bead.ID, item.ID)
	}
	if bead.Title != item.Title {
		t.Fatalf("title = %q, want %q", bead.Title, item.Title)
	}
	if bead.Priority != 2 {
		t.Fatalf("priority = %d, want 2", bead.Priority)
	}
	if !reflect.DeepEqual(bead.Labels, []string{"spec:alpha", "build_strategy:medium"}) {
		t.Fatalf("labels = %v", bead.Labels)
	}
	if !reflect.DeepEqual(bead.ExpectedOutputs, []string{"file1.go", "file2.md"}) {
		t.Fatalf("expected outputs = %v", bead.ExpectedOutputs)
	}
	if bead.Parent != "parent-1" || bead.Type != "task" || bead.Owner != "alice" {
		t.Fatalf("metadata mismatch: %+v", bead)
	}
	if bead.CloseReason != "done" {
		t.Fatalf("close reason = %q, want done", bead.CloseReason)
	}
	if !reflect.DeepEqual(bead.Dependencies, []Dependency{{ID: "dep-1", Title: "Dependency"}}) {
		t.Fatalf("dependencies = %+v", bead.Dependencies)
	}
	if bead.DependencyCount == nil || *bead.DependencyCount != 5 {
		t.Fatalf("dependency count = %v, want 5", bead.DependencyCount)
	}
	if bead.DependentCount == nil || *bead.DependentCount != 3 {
		t.Fatalf("dependent count = %v, want 3", bead.DependentCount)
	}
}
