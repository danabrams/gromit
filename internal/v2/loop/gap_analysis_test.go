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

func TestFlagChangedBeads_NilBeadInSlice(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "bead-a"},
		nil,
		{ID: "bead-c"},
	}
	changedFiles := []string{"internal/foo/foo.go"}
	beadFiles := map[string][]string{
		"bead-a": {"internal/foo/foo.go"},
		"bead-c": {"internal/bar/bar.go"},
	}

	flagged := FlagChangedBeads(beads, changedFiles, beadFiles)

	if len(flagged) != 1 {
		t.Fatalf("got %d flagged beads, want 1", len(flagged))
	}
	if flagged[0].ID != "bead-a" {
		t.Errorf("got flagged bead %q, want %q", flagged[0].ID, "bead-a")
	}
}

func TestFlagChangedBeads_EmptyChangedFiles(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "bead-a"},
		{ID: "bead-b"},
	}
	changedFiles := []string{}
	beadFiles := map[string][]string{
		"bead-a": {"internal/foo/foo.go"},
		"bead-b": {"internal/bar/bar.go"},
	}

	flagged := FlagChangedBeads(beads, changedFiles, beadFiles)

	if len(flagged) != 0 {
		t.Fatalf("got %d flagged beads, want 0", len(flagged))
	}
}

func TestFlagChangedBeads_EmptyBeadFilesMap(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "bead-a"},
		{ID: "bead-b"},
	}
	changedFiles := []string{"internal/foo/foo.go"}
	beadFiles := map[string][]string{}

	flagged := FlagChangedBeads(beads, changedFiles, beadFiles)

	if len(flagged) != 0 {
		t.Fatalf("got %d flagged beads, want 0", len(flagged))
	}
}

func TestFlagChangedBeads_BeadIDAbsentFromMap(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "bead-a"},
		{ID: "bead-missing"},
	}
	changedFiles := []string{"internal/foo/foo.go"}
	beadFiles := map[string][]string{
		"bead-a": {"internal/foo/foo.go"},
	}

	flagged := FlagChangedBeads(beads, changedFiles, beadFiles)

	if len(flagged) != 1 {
		t.Fatalf("got %d flagged beads, want 1", len(flagged))
	}
	if flagged[0].ID != "bead-a" {
		t.Errorf("got flagged bead %q, want %q", flagged[0].ID, "bead-a")
	}
}
