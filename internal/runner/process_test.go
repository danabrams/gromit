package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestSetupBeadContext_WithSpecLabel verifies that setupBeadContext
// correctly extracts the spec name from bead labels and sets bc.Result.SpecID.
func TestSetupBeadContext_WithSpecLabel(t *testing.T) {
	b := &bead.Bead{
		ID:     "bead-1",
		Title:  "Test bead",
		Labels: []string{"spec:authentication"},
	}
	bc := &runtypes.BeadContext{
		Bead:   b,
		Result: &runtypes.IterationResult{},
	}

	SetupBeadContext(bc)

	if bc.Result.SpecID != "authentication" {
		t.Errorf("SpecID = %q, want %q", bc.Result.SpecID, "authentication")
	}
}

// TestSetupBeadContext_WithoutSpecLabel verifies that setupBeadContext
// correctly handles beads without a spec label (empty string).
func TestSetupBeadContext_WithoutSpecLabel(t *testing.T) {
	b := &bead.Bead{
		ID:     "bead-2",
		Title:  "Test bead without spec",
		Labels: []string{"some-other-label"},
	}
	bc := &runtypes.BeadContext{
		Bead:   b,
		Result: &runtypes.IterationResult{},
	}

	SetupBeadContext(bc)

	if bc.Result.SpecID != "" {
		t.Errorf("SpecID = %q, want empty string", bc.Result.SpecID)
	}
}
