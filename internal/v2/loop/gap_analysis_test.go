package loop

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestFlagChangedBeads_FlagsBeadsWithOverlappingFiles(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "bead-a"},
		{ID: "bead-b"},
		{ID: "bead-c"},
	}
	changedFiles := []string{"internal/foo/foo.go", "internal/bar/bar.go"}
	beadFiles := map[string][]string{
		"bead-a": {"internal/foo/foo.go", "internal/foo/foo_test.go"},
		"bead-b": {"internal/qux/qux.go"},
		"bead-c": {"internal/bar/bar.go"},
	}

	flagged := FlagChangedBeads(beads, changedFiles, beadFiles)

	if len(flagged) != 2 {
		t.Fatalf("got %d flagged beads, want 2", len(flagged))
	}
	ids := make([]string, len(flagged))
	for i, b := range flagged {
		ids[i] = b.ID
	}
	wantIDs := map[string]bool{"bead-a": true, "bead-c": true}
	for _, id := range ids {
		if !wantIDs[id] {
			t.Errorf("unexpected flagged bead %q", id)
		}
	}
}

func TestFlagChangedBeads_NoOverlapReturnsEmpty(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "bead-x"},
		{ID: "bead-y"},
	}
	changedFiles := []string{"internal/other/other.go"}
	beadFiles := map[string][]string{
		"bead-x": {"internal/alpha/alpha.go"},
		"bead-y": {"internal/beta/beta.go"},
	}

	flagged := FlagChangedBeads(beads, changedFiles, beadFiles)

	if len(flagged) != 0 {
		t.Fatalf("got %d flagged beads, want 0", len(flagged))
	}
}
