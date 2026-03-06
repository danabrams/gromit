package tasktracker

import (
	"context"
	"testing"
)

func TestNextBead_ReturnsOpenBeadWithDependencyInfo(t *testing.T) {
	// This test verifies that NextBead retrieves the next open bead from bd
	// with dependency information included
	ctx := context.Background()
	adapter := NewBDAdapter(nil) // Will fail - but that's the point of RED phase

	bead, err := adapter.NextBead(ctx)
	if err != nil {
		t.Fatalf("NextBead failed: %v", err)
	}
	if bead == nil {
		t.Fatal("NextBead returned nil bead")
	}
}

func TestCreateBead_CreatesBeadWithTitleDescriptionAndPriority(t *testing.T) {
	ctx := context.Background()
	adapter := NewBDAdapter(nil)

	bead, err := adapter.CreateBead(ctx, "Test Title", "Test Description", 1, nil, []string{"dep1"})
	if err != nil {
		t.Fatalf("CreateBead failed: %v", err)
	}
	if bead == nil {
		t.Fatal("CreateBead returned nil bead")
	}
	if bead.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %q", bead.Title)
	}
	if bead.Description != "Test Description" {
		t.Errorf("expected description 'Test Description', got %q", bead.Description)
	}
	if bead.Priority != 1 {
		t.Errorf("expected priority 1, got %d", bead.Priority)
	}
}

func TestCloseBead_MarksBeadAsClosed(t *testing.T) {
	ctx := context.Background()
	adapter := NewBDAdapter(nil)

	err := adapter.CloseBead(ctx, "test-bead-id")
	if err != nil {
		t.Fatalf("CloseBead failed: %v", err)
	}
}

func TestQueryBeads_FiltersBeadsByLabelsAndStatus(t *testing.T) {
	ctx := context.Background()
	adapter := NewBDAdapter(nil)

	beads, err := adapter.QueryBeads(ctx, []string{"gen:1"}, "open", "")
	if err != nil {
		t.Fatalf("QueryBeads failed: %v", err)
	}
	if beads == nil {
		t.Fatal("QueryBeads returned nil beads slice")
	}
}
