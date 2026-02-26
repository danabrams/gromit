package bead

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/danabrams/gromit/internal/tracker"
)

func TestBDAdapterReadyReturnsNilWhenNoReadyBead(t *testing.T) {
	t.Parallel()

	client := &Client{
		RunFn: func(args ...string) (string, error) {
			return "[]", nil
		},
	}

	adapter := BDAdapter{client: client}

	item, err := adapter.Ready(context.Background())
	if err != nil {
		t.Fatalf("Ready() returned unexpected error: %v", err)
	}
	if item != nil {
		t.Fatalf("Ready() returned item %v, expected nil", item)
	}
}

func TestBeadToItemPreservesFields(t *testing.T) {
	t.Parallel()

	depCount := 1
	dependerCount := 2
	input := &Bead{
		ID:                 "bead-123",
		Title:              "Sample",
		Description:        "desc",
		Status:             "open",
		Priority:           2,
		Owner:              "owner",
		Parent:             "parent-456",
		Type:               "task",
		CloseReason:        "done",
		AcceptanceCriteria: "criteria",
		Labels:             []string{"spec:alpha", "priority:high"},
		ExpectedOutputs:    []string{"output-1", "output-2"},
		Dependencies: []Dependency{
			{ID: "dep-1", Title: "Dep 1", Status: "open"},
		},
		BlockedBy: []Dependency{
			{ID: "block-1", Title: "Block 1", Status: "blocked"},
		},
		DependsOn: []Dependency{
			{ID: "depends-1", Title: "Depends 1", Status: "open"},
		},
		DependencyCount: &depCount,
		DependentCount:  &dependerCount,
	}

	item := beadToItem(input)
	if item == nil {
		t.Fatal("expected beadToItem to return non-nil item")
	}

	if item.ID != input.ID {
		t.Fatalf("ID = %q, want %q", item.ID, input.ID)
	}
	if item.Title != input.Title {
		t.Fatalf("Title = %q, want %q", item.Title, input.Title)
	}
	if item.Description != input.Description {
		t.Fatalf("Description = %q, want %q", item.Description, input.Description)
	}
	if item.Status != input.Status {
		t.Fatalf("Status = %q, want %q", item.Status, input.Status)
	}

	meta := item.Metadata
	if got := meta["status"]; got != input.Status {
		t.Fatalf("metadata[\"status\"] = %q, want %q", got, input.Status)
	}
	if got := meta["priority"]; got != "2" {
		t.Fatalf("metadata[\"priority\"] = %q, want %q", got, "2")
	}
	if got := meta["owner"]; got != input.Owner {
		t.Fatalf("metadata[\"owner\"] = %q, want %q", got, input.Owner)
	}
	if got := meta["parent"]; got != input.Parent {
		t.Fatalf("metadata[\"parent\"] = %q, want %q", got, input.Parent)
	}

	assertJSONListEqual(t, meta["labels"], input.Labels)
	assertJSONListEqual(t, meta["expected_outputs"], input.ExpectedOutputs)

	var deps []Dependency
	if err := json.Unmarshal([]byte(meta["dependencies"]), &deps); err != nil {
		t.Fatalf("unmarshal dependencies metadata: %v", err)
	}
	if len(deps) != len(input.Dependencies) {
		t.Fatalf("dependencies count = %d, want %d", len(deps), len(input.Dependencies))
	}

	if got := meta["dependency_count"]; got != "1" {
		t.Fatalf("metadata[\"dependency_count\"] = %q, want %q", got, "1")
	}
	if got := meta["dependent_count"]; got != "2" {
		t.Fatalf("metadata[\"dependent_count\"] = %q, want %q", got, "2")
	}
}

func assertJSONListEqual(t *testing.T, encoded string, expected []string) {
	t.Helper()
	var decoded []string
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("unmarshal list metadata: %v", err)
	}
	if len(decoded) != len(expected) {
		t.Fatalf("decoded list length = %d, want %d", len(decoded), len(expected))
	}
	for i, v := range decoded {
		if v != expected[i] {
			t.Fatalf("entry[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestBDAdapterListFiltersItemsByLabels(t *testing.T) {
	t.Parallel()

	// Create beads: one with spec:foo, one with spec:bar, one with both
	specFooBead := &Bead{
		ID:     "bead-1",
		Title:  "Spec Foo Task",
		Status: "open",
		Labels: []string{"spec:foo", "priority:high"},
	}

	specBarBead := &Bead{
		ID:     "bead-2",
		Title:  "Spec Bar Task",
		Status: "open",
		Labels: []string{"spec:bar"},
	}

	specBothBead := &Bead{
		ID:     "bead-3",
		Title:  "Both Spec Task",
		Status: "open",
		Labels: []string{"spec:foo", "spec:bar"},
	}

	client := &Client{
		RunFn: func(args ...string) (string, error) {
			// Return all beads when querying by status
			beads := []*Bead{specFooBead, specBarBead, specBothBead}
			data, _ := json.Marshal(beads)
			return string(data), nil
		},
	}

	adapter := NewBDAdapter(client)

	// Query for items with spec:foo label
	query := newTrackerQuery()
	query.Filter.Labels = []string{"spec:foo"}

	items, err := adapter.List(context.Background(), query)
	if err != nil {
		t.Fatalf("List() returned unexpected error: %v", err)
	}

	// Should return bead-1 and bead-3 (both have spec:foo label)
	if len(items) != 2 {
		t.Fatalf("List() returned %d items, expected 2", len(items))
	}

	foundBead1 := false
	foundBead3 := false
	for _, item := range items {
		if item.ID == "bead-1" {
			foundBead1 = true
		}
		if item.ID == "bead-3" {
			foundBead3 = true
		}
	}

	if !foundBead1 {
		t.Fatalf("List() did not return bead-1 which has label spec:foo")
	}
	if !foundBead3 {
		t.Fatalf("List() did not return bead-3 which has label spec:foo")
	}
}

// Helper to create a minimal tracker.Query
func newTrackerQuery() tracker.Query {
	return tracker.Query{
		Filter: tracker.Filter{
			Statuses: []string{"open"},
		},
	}
}

func TestBDAdapterCreateWithParentPassesParent(t *testing.T) {
	t.Parallel()

	const expectedParent = "parent-123"
	var recordedArgs []string

	client := &Client{
		RunFn: func(args ...string) (string, error) {
			recordedArgs = append([]string{}, args...)
			bead := Bead{ID: "child", Title: "child", Parent: expectedParent}
			data, _ := json.Marshal(bead)
			return string(data), nil
		},
	}

	adapter := NewBDAdapter(client)
	req := tracker.CreateRequest{Title: "child"}

	if _, err := adapter.CreateWithParent(context.Background(), req, expectedParent); err != nil {
		t.Fatalf("CreateWithParent() unexpected error: %v", err)
	}

	found := false
	for i, arg := range recordedArgs {
		if arg == "--parent" && i+1 < len(recordedArgs) && recordedArgs[i+1] == expectedParent {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("CreateWithParent() did not pass --parent %q to bd create, args: %v", expectedParent, recordedArgs)
	}
}
