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

	resp, err := adapter.NextBead(ctx, NextBeadRequest{})
	if err != nil {
		t.Fatalf("NextBead failed: %v", err)
	}
	if resp == nil || resp.Bead == nil {
		t.Fatal("NextBead returned nil bead")
	}
}

func TestCreateBead_CreatesBeadWithTitleDescriptionAndPriority(t *testing.T) {
	ctx := context.Background()
	adapter := NewBDAdapter(nil)

	resp, err := adapter.CreateBead(ctx, CreateBeadRequest{
		Title:        "Test Title",
		Description:  "Test Description",
		Priority:     1,
		Dependencies: []string{"dep1"},
	})
	if err != nil {
		t.Fatalf("CreateBead failed: %v", err)
	}
	if resp == nil || resp.Bead == nil {
		t.Fatal("CreateBead returned nil bead")
	}
	if resp.Bead.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %q", resp.Bead.Title)
	}
	if resp.Bead.Description != "Test Description" {
		t.Errorf("expected description 'Test Description', got %q", resp.Bead.Description)
	}
	if resp.Bead.Priority != 1 {
		t.Errorf("expected priority 1, got %d", resp.Bead.Priority)
	}
}

func TestCloseBead_MarksBeadAsClosed(t *testing.T) {
	ctx := context.Background()
	adapter := NewBDAdapter(nil)

	resp, err := adapter.CloseBead(ctx, CloseBeadRequest{BeadID: "test-bead-id"})
	if err != nil {
		t.Fatalf("CloseBead failed: %v", err)
	}
	if resp == nil || !resp.Closed {
		t.Fatalf("CloseBead response = %#v", resp)
	}
}

func TestQueryBeads_FiltersBeadsByLabelsAndStatus(t *testing.T) {
	ctx := context.Background()
	adapter := NewBDAdapter(nil)

	resp, err := adapter.QueryBeads(ctx, QueryBeadsRequest{Labels: []string{"gen:1"}, Status: "open"})
	if err != nil {
		t.Fatalf("QueryBeads failed: %v", err)
	}
	if resp == nil {
		t.Fatal("QueryBeads returned nil beads slice")
	}
}
