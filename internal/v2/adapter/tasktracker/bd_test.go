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
